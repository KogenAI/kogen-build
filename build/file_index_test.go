package build

import (
	"fmt"
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
			fmt.Fprintf(&sb, "dir-%05d-with-a-long-name/file-%02d.txt\n", d, f)
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
