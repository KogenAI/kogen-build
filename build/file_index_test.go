package build

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// A repository with thousands of top-level directories cannot name each of them within the
// bound: the index still stays bounded and its counts stay true.
func TestFileIndexVeryWideRepositoryStaysBounded(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	total := 0
	for d := 0; d < 3000; d++ {
		for f := 0; f < 20; f++ {
			fmt.Fprintf(&sb, "dir-%05d-with-a-long-name/file-%02d.txt\x00", d, f)
			total++
		}
	}
	out := fileIndex(sb.String(), []string{"dir-00001-with-a-long-name/file-01.txt"})
	if len(out) > fileIndexBytes {
		t.Fatalf("index is %d bytes, want at most %d", len(out), fileIndexBytes)
	}
	if !strings.Contains(out, fmt.Sprintf("total paths: %d\n", total)) || !strings.Contains(out, "dir-00001-with-a-long-name/file-01.txt\n") {
		t.Fatalf("index lost the true total or a required path:\n%.600s", out)
	}
}

func TestFileIndexOmissionAccountingAndInputOrder(t *testing.T) {
	t.Parallel()
	var paths []string
	for i := 0; i < 1800; i++ {
		paths = append(paths,
			fmt.Sprintf("root-file-%04d.txt", i),
			fmt.Sprintf("source/direct-file-%04d.txt", i),
			fmt.Sprintf("source/nested/deeper/file-%04d.txt", i),
			fmt.Sprintf("other/nested/file-%04d.txt", i))
	}
	required := []string{"root-file-1799.txt", "source/direct-file-1799.txt", "source/nested/deeper/file-1799.txt"}
	out := fileIndex(strings.Join(paths, "\x00")+"\x00", required)
	if len(out) > fileIndexBytes {
		t.Fatalf("index is %d bytes, want at most %d", len(out), fileIndexBytes)
	}
	reversed := make([]string, len(paths))
	for i, p := range paths {
		reversed[len(paths)-1-i] = p
	}
	if reordered := fileIndex(strings.Join(reversed, "\x00")+"\x00", required); reordered != out {
		t.Fatal("reordering the same listing changed the complete index")
	}

	_, body, ok := strings.Cut(out, "## Listed paths\n")
	if !ok {
		t.Fatal("fixture did not produce a summarized index")
	}
	listedText, omissionText, ok := strings.Cut(body, "\n## Omitted directories\n")
	if !ok {
		t.Fatal("fixture did not produce omitted directories")
	}
	listed := map[string]bool{}
	for _, p := range strings.Split(strings.TrimSuffix(listedText, "\n"), "\n") {
		listed[p] = true
	}
	for _, p := range required {
		if !listed[p] {
			t.Errorf("required path %q is not individually listed", p)
		}
	}
	omitted := map[string]int{}
	var root, direct, nested int
	for _, p := range paths {
		if listed[p] {
			continue
		}
		omitted[p] = 0
		switch {
		case !strings.Contains(p, "/"):
			root++
		case strings.HasPrefix(p, "source/nested/"):
			nested++
		case strings.HasPrefix(p, "source/"):
			direct++
		}
	}
	if root == 0 || direct == 0 || nested == 0 {
		t.Fatalf("fixture must omit root, direct and descendant files; got %d, %d, %d", root, direct, nested)
	}
	sum := 0
	for _, line := range strings.Split(strings.TrimSuffix(omissionText, "\n"), "\n") {
		dir, countText, ok := strings.Cut(line, ": ")
		if !ok || !strings.HasSuffix(dir, "/") || !strings.HasSuffix(countText, " omitted paths") {
			t.Fatalf("invalid omission entry %q", line)
		}
		count, err := strconv.Atoi(strings.TrimSuffix(countText, " omitted paths"))
		if err != nil {
			t.Fatalf("invalid omission count in %q: %v", line, err)
		}
		covered := 0
		for p := range omitted {
			if (dir == "./" && !strings.Contains(p, "/")) || (dir != "./" && strings.HasPrefix(p, dir)) {
				omitted[p]++
				covered++
			}
		}
		if count != covered || covered == 0 {
			t.Errorf("entry %q reports %d paths, independently counted %d", dir, count, covered)
		}
		sum += count
	}
	for p, coverage := range omitted {
		if coverage != 1 {
			t.Errorf("omitted path %q is covered %d times, want exactly once", p, coverage)
		}
	}
	if sum != len(omitted) {
		t.Errorf("omission entries sum to %d, want %d", sum, len(omitted))
	}
	if !strings.Contains(out, fmt.Sprintf("omitted paths: %d\n", len(omitted))) {
		t.Errorf("reported omitted total does not match %d independently derived paths", len(omitted))
	}
}

func TestFileIndexControlCharacters(t *testing.T) {
	path := "docs/new\nline/tab\tend/return\rend/slash\\end/unit\x1fend.txt"
	ordinary := "docs/café.txt"
	out := fileIndex(path+"\x00"+ordinary+"\x00", nil)

	cases := []struct{ name, escaped, raw string }{
		{"newline", `new\nline`, "new\nline"},
		{"tab", `tab\tend`, "tab\tend"},
		{"carriage return", `return\rend`, "return\rend"},
		{"backslash", `slash\\end`, `slash\end`},
		{"unit separator", `unit\x1fend`, "unit\x1fend"},
	}
	for _, tc := range cases {
		if !strings.Contains(out, tc.escaped) {
			t.Errorf("%s: missing escape %q in %q", tc.name, tc.escaped, out)
		}
		if strings.Contains(out, tc.raw) {
			t.Errorf("%s: raw character present in %q", tc.name, out)
		}
	}
	line := `docs/new\nline/tab\tend/return\rend/slash\\end/unit\x1fend.txt`
	if !strings.Contains(out, "\n"+line+"\n") {
		t.Errorf("escaped path is not a complete line in %q", out)
	}
	if !strings.Contains(out, "\n"+ordinary+"\n") {
		t.Errorf("ordinary path is not an unchanged complete line in %q", out)
	}
}
