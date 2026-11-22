package build

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/KogenAI/kogen-coordinator/durable"
	"github.com/KogenAI/kogen-coordinator/project"
)

// Every Build runs hundreds of git commands, and on macOS /usr/bin/git is the xcode-select
// shim, which resolves the developer directory on each call and costs about twice what the
// real binary does. Each verification pass also starts every toolchain its PATH finds to record
// its version, and every fixture project used to write a new acceptance script, whose first run
// waits for the operating system's check of a new executable.
//
// The suite therefore keeps one private directory for the whole run of the test binary. Its bin
// directory holds only wrappers for the tools the tests use (git, go and node, each found once
// here) and goes first on PATH, followed by the system directories only: every session, git
// command and standalone binary of the tests then sees that PATH, and a Build's check identity
// records only those tools. A tool not found here gets no wrapper. The wrappers exec the real path
// rather than being symlinks to it: git reached through a symlink cannot compute its runtime
// prefix under the Command Line Tools and falls back to a compiled-in Xcode.app path, which
// breaks templates and info/exclude. git itself is a copy of the real binary (installGit), so no
// shell stands between a test and it, and a git script on the PATH (a host shim) is looked
// through to the binary it runs. Its accept directory holds the one read-only acceptance
// script, with its two reports, that every fixture project links to. The directory is removed
// after the run; a test binary re-executed by a test finds the PATH of its parent and shares
// that directory.
func TestMain(m *testing.M) {
	// No global or system Git configuration (hooks, signing, templates, identity rules) reaches
	// the fixture repositories; a re-executed child inherits this through its environment.
	os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_TERMINAL_PROMPT", "0")
	if inheritPrivate() {
		// A test re-executed this binary (the baseline tests do) under its PATH: the child
		// shares the parent's private directory, and its environment stays the parent's.
		durable.SkipSyncInTests()
		os.Exit(m.Run())
	}
	private, err := os.MkdirTemp("", "kogen-build-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := setupPrivate(private); err != nil {
		os.RemoveAll(private)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	durable.SkipSyncInTests()
	code, sharded := runShards(m)
	if !sharded {
		code = m.Run()
	}
	os.RemoveAll(private)
	os.Exit(code)
}

// The tests run in several processes. Every Build starts many git processes, and one process
// running all of the suite's Builds in parallel spends most of its time waiting for its own
// forks. Measured on a 12-CPU host with go test -count=1: 122 s in one process (-run .), 77 s
// split as below. runShards splits the tests by name over runtime.NumCPU()/3
// (at most four) re-executions of this binary, each running its tests like any run of the
// package, prints their output in order and takes the first failing exit code.
//
// A run that asks for anything but the plain forms (a -run pattern, a profile, a benchmark, a
// shuffle, -update, ...) is not split. A shard run writes the test log that `go test` caches
// results by the same way a single run does: the tests of the shards log into their own files and
// the parent appends them to its log. One cheap test stays with the parent, so that the parent's
// own run produces the log, the result line and the exit status.
func runShards(m *testing.M) (int, bool) {
	shards := min(4, runtime.NumCPU()/3)
	if shards < 2 {
		return 0, false
	}
	if !flag.Parsed() {
		flag.Parse()
	}
	forwarded := map[string]string{}
	plain := true
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "test.v", "test.count", "test.timeout", "test.paniconexit0", "test.testlogfile", "test.parallel":
			forwarded[f.Name] = f.Value.String()
		default:
			plain = false
		}
	})
	if !plain {
		return 0, false
	}
	out, err := exec.Command(os.Args[0], "-test.list=.").Output()
	if err != nil {
		return 0, false
	}
	const reserved = "TestUnknownBuilderToolNamesPanic"
	var names []string
	for _, line := range strings.Fields(string(out)) {
		if (strings.HasPrefix(line, "Test") || strings.HasPrefix(line, "Example")) && line != reserved {
			names = append(names, line)
		}
	}
	if len(names) < 2*shards || !strings.Contains(string(out), reserved) {
		return 0, false
	}
	// Chunks of tests are handed to the shards as they become free, so that a shard that drew
	// the slow tests does not decide the end of the run. The tests are dealt over the chunks in
	// turn, so that tests next to each other in a file (often alike in cost) land in different ones.
	chunks := make([][]string, min(len(names)/2, shards*6))
	for i, name := range names {
		chunks[i%len(chunks)] = append(chunks[i%len(chunks)], name)
	}
	logs := make([]string, len(chunks))
	outputs := make([][]byte, len(chunks))
	codes := make([]int, len(chunks))
	var wg sync.WaitGroup
	next := make(chan int, len(chunks))
	for i := range chunks {
		next <- i
	}
	close(next)
	for range shards {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				args := []string{"-test.run=^(" + strings.Join(chunks[i], "|") + ")$"}
				for _, name := range []string{"test.v", "test.count", "test.timeout", "test.paniconexit0"} {
					if v, ok := forwarded[name]; ok {
						args = append(args, "-"+name+"="+v)
					}
				}
				if v, ok := forwarded["test.parallel"]; ok {
					args = append(args, "-test.parallel="+v)
				} else {
					args = append(args, "-test.parallel=3")
				}
				if forwarded["test.testlogfile"] != "" {
					f, err := os.CreateTemp("", "kogen-build-testlog-")
					if err != nil {
						codes[i] = 1
						outputs[i] = []byte(err.Error() + "\n")
						continue
					}
					f.Close()
					logs[i] = f.Name()
					args = append(args, "-test.testlogfile="+f.Name())
				}
				var err error
				outputs[i], err = exec.Command(os.Args[0], args...).CombinedOutput()
				var ee *exec.ExitError
				switch {
				case err == nil:
				case errors.As(err, &ee) && ee.ExitCode() > 0:
					codes[i] = ee.ExitCode()
				default:
					codes[i] = 1
				}
			}
		}()
	}
	wg.Wait()
	defer func() {
		for _, log := range logs {
			if log != "" {
				os.Remove(log)
			}
		}
	}()
	code := 0
	for i := range outputs {
		text := outputs[i]
		if codes[i] == 0 {
			text = bytes.TrimSuffix(text, []byte("PASS\n"))
		} else if code == 0 {
			code = codes[i]
		}
		os.Stdout.Write(text)
	}
	if code != 0 {
		fmt.Println("FAIL")
		return code, true
	}
	flag.Set("test.run", "^"+reserved+"$")
	code = m.Run()
	if path := forwarded["test.testlogfile"]; path != "" {
		if f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0); err == nil {
			for _, log := range logs {
				if data, err := os.ReadFile(log); err == nil {
					_, data, _ = bytes.Cut(data, []byte("\n")) // the log's version line
					f.Write(data)
				}
			}
			f.Close()
		}
	}
	return code, true
}

// sharedAccept is the acceptance script every fixture project links to; set by TestMain.
var sharedAccept string

const systemPath = "/usr/bin:/bin:/usr/sbin:/sbin"

// inheritPrivate reports whether PATH is already exactly what a parent run of this binary set up
// (its private bin directory, then the system directories) and, if so, takes that run's shared
// acceptance script.
func inheritPrivate() bool {
	bin, rest, ok := strings.Cut(os.Getenv("PATH"), string(os.PathListSeparator))
	if !ok || rest != systemPath || filepath.Base(bin) != "bin" || !strings.HasPrefix(filepath.Base(filepath.Dir(bin)), "kogen-build-test-") {
		return false
	}
	script := filepath.Join(filepath.Dir(bin), "accept", "accept.sh")
	if st, err := os.Stat(script); err != nil || st.IsDir() {
		return false
	}
	sharedAccept = script
	return true
}

func setupPrivate(private string) error {
	bin := filepath.Join(private, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		return err
	}
	for name, target := range wrapperTargets() {
		if name == "git" && installGit(private, bin, target) {
			continue
		}
		if err := writeWrapper(bin, name, target); err != nil {
			return err
		}
	}
	os.Setenv("PATH", bin+string(os.PathListSeparator)+systemPath)

	dir := filepath.Join(private, "accept")
	if err := os.Mkdir(dir, 0o700); err != nil {
		return err
	}
	feature := project.FeaturePath(testSlug)
	green, red := filepath.Join(dir, "green.ndjson"), filepath.Join(dir, "red.ndjson")
	for path, status := range map[string]string{green: "PASSED", red: "FAILED"} {
		text, err := reportText(feature, testFeature, status)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			return err
		}
	}
	script := "#!/bin/sh\nif grep -qx red app.txt; then cp '" + red + "' \"$KOGEN_ACCEPTANCE_REPORT\"; echo actual-case-red; exit 1; fi\ncp '" + green + "' \"$KOGEN_ACCEPTANCE_REPORT\"\n"
	sharedAccept = filepath.Join(dir, "accept.sh")
	return os.WriteFile(sharedAccept, []byte(script), 0o555)
}

// wrapperTargets maps each tool the tests use to the real executable found on the host's
// PATH, before it is replaced.
func wrapperTargets() map[string]string {
	targets := map[string]string{}
	if dir := realGitDir(); dir != "" {
		targets["git"] = filepath.Join(dir, "git")
	} else if p, err := exec.LookPath("git"); err == nil {
		if real := behindShim(p); real != "" {
			p = real
		}
		targets["git"] = p
	}
	if _, err := exec.LookPath("go"); err == nil {
		if out, err := exec.Command("go", "env", "GOROOT").Output(); err == nil {
			if root := strings.TrimSpace(string(out)); root != "" {
				targets["go"] = filepath.Join(root, "bin", "go")
			}
		}
	}
	if _, err := exec.LookPath("node"); err == nil {
		if out, err := exec.Command("node", "-p", "process.execPath").Output(); err == nil {
			if p := strings.TrimSpace(string(out)); p != "" {
				targets["node"] = p
			}
		}
	}
	return targets
}

func writeWrapper(dir, name, target string) error {
	quoted := "'" + strings.ReplaceAll(target, "'", `'\''`) + "'"
	return os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexec "+quoted+" \"$@\"\n"), 0o700)
}

// isScript reports whether the file at path is an interpreter script rather than a binary.
func isScript(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 2)
	n, _ := f.Read(head)
	return string(head[:n]) == "#!"
}

// behindShim is the git binary a git script on the PATH (a host shim that execs the real
// one) stands for, found from the exec path that git reports: <prefix>/libexec/git-core
// belongs to <prefix>/bin/git. It is "" when path is a binary or the real one is not found.
// Every session, git command and Build step of the suite runs git, and each script in front
// of it costs one more process start.
func behindShim(path string) string {
	if !isScript(path) {
		return ""
	}
	out, err := exec.Command(path, "--exec-path").Output()
	if err != nil {
		return ""
	}
	real := filepath.Join(strings.TrimSpace(string(out)), "..", "..", "bin", "git")
	if st, err := os.Stat(real); err != nil || st.IsDir() || st.Mode()&0o111 == 0 || isScript(real) {
		return ""
	}
	return real
}

// installGit puts git in bin as a copy of the real executable, so that a test starts git
// itself and no shell stands in between, and reports whether it did. Git finds its helper
// programs and templates relative to its own path under some builds (the Command Line
// Tools one among them): the private directory gets links to the real prefix's libexec and
// share directories for that. A copy that does not report the real git's exec path is
// removed and the caller falls back to a wrapper.
func installGit(private, bin, real string) bool {
	data, err := os.ReadFile(real)
	if err != nil {
		return false
	}
	want, err := exec.Command(real, "--exec-path").Output()
	if err != nil {
		return false
	}
	path := filepath.Join(bin, "git")
	if err := os.WriteFile(path, data, 0o700); err != nil {
		return false
	}
	var links []string
	prefix := filepath.Dir(filepath.Dir(real))
	for _, dir := range []string{"libexec", "share"} {
		if st, err := os.Stat(filepath.Join(prefix, dir)); err == nil && st.IsDir() {
			link := filepath.Join(private, dir)
			if os.Symlink(filepath.Join(prefix, dir), link) == nil {
				links = append(links, link)
			}
		}
	}
	got, err := exec.Command(path, "--exec-path").Output()
	if err == nil && sameFile(strings.TrimSpace(string(got)), strings.TrimSpace(string(want))) {
		return true
	}
	os.Remove(path)
	for _, link := range links {
		os.Remove(link)
	}
	return false
}

func sameFile(a, b string) bool {
	pa, err1 := filepath.EvalSymlinks(a)
	pb, err2 := filepath.EvalSymlinks(b)
	return err1 == nil && err2 == nil && pa == pb
}

func realGitDir() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	if p, err := exec.LookPath("git"); err != nil || p != "/usr/bin/git" {
		return ""
	}
	out, err := exec.Command("xcode-select", "-p").Output()
	if err != nil {
		return ""
	}
	dir := filepath.Join(strings.TrimSpace(string(out)), "usr", "bin")
	if st, err := os.Stat(filepath.Join(dir, "git")); err != nil || st.IsDir() {
		return ""
	}
	return dir
}
