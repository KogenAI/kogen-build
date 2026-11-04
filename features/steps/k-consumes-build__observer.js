'use strict';
// The Go source of the k-consumes-build observer (k-consumes-build__report.js builds it outside
// the checkout). It lists what kogen-build's own non-test code uses of each Kogen dependency and
// judges each consumes/<producer>/ suite and the consumes/README.md inventory against it. Data
// only: no step definition.
module.exports = {
  source: String.raw`// The observer of the k-consumes-build acceptance: it lists what kogen-build's own non-test
// code uses of each Kogen dependency and judges each consumes/<producer>/ suite and the
// inventory of consumes/README.md against it.
//
//	go run . [-list] <root> <suite>=<module> ...
//
// A use is an exported package-level identifier of a package of the module ('pkg.Name'), or a
// method or field of one of its named types ('pkg.Type.Member', by the type that declares the
// member), named in a non-test Go file of a package of kogen-build outside contract/, consumes/
// and features/. With -list it prints the uses of each module; otherwise it prints "ok" and
// exits 0, or prints one problem per line and exits 1.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const own = "github.com/KogenAI/kogen-build"

// readme is the file whose "## Inventory" section maps every use to the tests asserting it.
const readme = "consumes/README.md"

// bq is the backquote that encloses an inventory row's name.
const bq = "\x60"

type listed struct {
	ImportPath   string
	Dir          string
	Name         string
	Export       string
	Standard     bool
	ForTest      string
	GoFiles      []string
	TestGoFiles  []string
	XTestGoFiles []string
	Imports      []string
	TestImports  []string
	XTestImports []string
	Module       *struct{ Path string }
	Error        *struct{ Err string }
}

type suite struct{ name, module string }

// found is what one type-checked package uses: the module of each use, and the tests (top-level
// Test functions) whose own body names it.
type found struct {
	module map[string]string
	tests  map[string]map[string]bool
	funcs  map[string]bool
}

func main() {
	list := flag.Bool("list", false, "print the uses of each module")
	flag.Parse()
	args := flag.Args()
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: observer [-list] <root> <suite>=<module> ...")
		os.Exit(2)
	}
	root, err := filepath.Abs(args[0])
	if err != nil {
		fail(err)
	}
	var suites []suite
	for _, a := range args[1:] {
		name, module, ok := strings.Cut(a, "=")
		if !ok {
			fail(fmt.Errorf("argument %q is not <suite>=<module>", a))
		}
		suites = append(suites, suite{name, module})
	}
	pkgs := goList(root)
	exports := map[string]string{}
	modules := map[string]string{}
	standard := map[string]bool{}
	for _, p := range pkgs {
		if p.ForTest != "" || strings.Contains(p.ImportPath, " [") || strings.HasSuffix(p.ImportPath, ".test") {
			continue
		}
		exports[p.ImportPath] = p.Export
		standard[p.ImportPath] = p.Standard
		if p.Module != nil {
			modules[p.ImportPath] = p.Module.Path
		}
	}
	fset := token.NewFileSet()
	imp := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		if exports[path] == "" {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(exports[path])
	})
	moduleOf := func(pkgPath string) string { return modules[pkgPath] }

	uses := map[string]map[string]bool{} // module → uses
	var problems []string
	for _, p := range pkgs {
		if !ownPlain(p) || under(root, p.Dir, "contract", "consumes", "features") {
			continue
		}
		for id, module := range usesOf(fset, imp, moduleOf, p.ImportPath, p.Dir, p.GoFiles).module {
			if module == own || module == "" || standard[pkgOfID(id)] {
				continue
			}
			if uses[module] == nil {
				uses[module] = map[string]bool{}
			}
			uses[module][id] = true
		}
	}
	known := map[string]bool{}
	for _, s := range suites {
		known[s.module] = true
	}
	for _, m := range sorted(uses) {
		if !known[m] {
			problems = append(problems, fmt.Sprintf("kogen-build's code uses the module %s, which has no consumes suite", m))
		}
	}
	if *list {
		for _, s := range suites {
			fmt.Printf("%s (%s):\n", s.name, s.module)
			for _, id := range sorted(uses[s.module]) {
				fmt.Printf("  %s\n", short(id))
			}
		}
		return
	}
	rows, err := inventory(filepath.Join(root, readme), suites)
	if err != nil {
		problems = append(problems, fmt.Sprintf("%s: %v", readme, err))
	}
	for _, s := range suites {
		problems = append(problems, judge(fset, imp, moduleOf, standard, root, pkgs, s, suites, uses[s.module], rows, err == nil)...)
	}
	if err == nil {
		problems = append(problems, rows.problems...)
	}
	if len(problems) == 0 {
		fmt.Println("ok")
		return
	}
	for _, p := range problems {
		fmt.Println(p)
	}
	os.Exit(1)
}

// judge lists the problems of the consumes suite s and of its inventory rows.
func judge(fset *token.FileSet, imp types.Importer, moduleOf func(string) string, standard map[string]bool, root string, pkgs []listed, s suite, suites []suite, want map[string]bool, rows table, haveRows bool) []string {
	dir := "consumes/" + s.name
	var problems []string
	tested := map[string]bool{}
	byTest := map[string]map[string]bool{} // short id → tests whose body uses it
	tests := map[string]bool{}
	seenPkg := false
	for _, p := range pkgs {
		if !ownPlain(p) || !under(root, p.Dir, dir) {
			continue
		}
		seenPkg = true
		rel, _ := filepath.Rel(root, p.Dir)
		rel = filepath.ToSlash(rel)
		for _, f := range p.GoFiles {
			problems = append(problems, fmt.Sprintf("%s: %s/%s is not a test file", dir, rel, f))
		}
		for _, path := range uniq(append(append(append([]string{}, p.Imports...), p.TestImports...), p.XTestImports...)) {
			switch {
			case standard[path]:
			case moduleOf(path) == own:
				problems = append(problems, fmt.Sprintf("%s: %s imports %s, a package of kogen-build", dir, rel, path))
			case !slices.ContainsFunc(suites, func(o suite) bool { return o.module == moduleOf(path) }):
				problems = append(problems, fmt.Sprintf("%s: %s imports %s, which is neither the standard library nor a Kogen dependency", dir, rel, path))
			}
		}
		for i, files := range [][]string{p.TestGoFiles, p.XTestGoFiles} {
			if len(files) == 0 {
				continue
			}
			got := usesOf(fset, imp, moduleOf, fmt.Sprintf("%s_consumes%d", p.ImportPath, i), p.Dir, files)
			for name := range got.funcs {
				tests[name] = true
			}
			for id, module := range got.module {
				if module != s.module {
					continue
				}
				tested[id] = true
				for name := range got.tests[id] {
					if byTest[short(id)] == nil {
						byTest[short(id)] = map[string]bool{}
					}
					byTest[short(id)][name] = true
				}
			}
		}
	}
	if !seenPkg {
		return []string{fmt.Sprintf("%s: no Go test package", dir)}
	}
	for _, id := range sorted(want) {
		if !tested[id] {
			problems = append(problems, fmt.Sprintf("%s: no test uses %s", dir, short(id)))
		}
	}
	if !haveRows {
		return problems
	}
	where := fmt.Sprintf("%s: %s", readme, s.name)
	if !rows.headed[s.name] {
		return append(problems, fmt.Sprintf("%s: no \"### %s\" inventory heading", where, s.name))
	}
	shortWant := map[string]bool{}
	for id := range want {
		shortWant[short(id)] = true
	}
	seen := map[string]bool{}
	for _, r := range rows.bySuite[s.name] {
		if seen[r.name] {
			problems = append(problems, fmt.Sprintf("%s: inventory lists %s twice", where, r.name))
			continue
		}
		seen[r.name] = true
		if !shortWant[r.name] {
			problems = append(problems, fmt.Sprintf("%s: inventory row %s is not a use of kogen-build's code", where, r.name))
			continue
		}
		if len(r.tests) == 0 {
			problems = append(problems, fmt.Sprintf("%s: inventory row %s names no test", where, r.name))
			continue
		}
		using := false
		for _, t := range r.tests {
			if !tests[t] {
				problems = append(problems, fmt.Sprintf("%s: inventory row %s names %s, which is not a test of %s", where, r.name, t, dir))
			}
			using = using || byTest[r.name][t]
		}
		if !using && tested[idByShort(want, r.name)] {
			problems = append(problems, fmt.Sprintf("%s: inventory row %s names no test whose body uses it", where, r.name))
		}
	}
	for _, id := range sorted(shortWant) {
		if !seen[id] {
			problems = append(problems, fmt.Sprintf("%s: inventory has no row for %s", where, id))
		}
	}
	return problems
}

func idByShort(want map[string]bool, name string) string {
	for id := range want {
		if short(id) == name {
			return id
		}
	}
	return ""
}

type row struct {
	name  string
	tests []string
}

// table is the inventory of the README: its rows under each "### <suite>" heading of the
// "## Inventory" section, and the problems of rows outside a suite's heading.
type table struct {
	bySuite  map[string][]row
	headed   map[string]bool
	problems []string
}

var testName = regexp.MustCompile("\\bTest[A-Za-z0-9_]*\\b")

// inventory reads the table rows under the README's "## Inventory" heading whose first cell is
// one backquoted name, by the "### <suite>" heading they are under; the other cells of a row name
// its tests.
func inventory(path string, suites []suite) (table, error) {
	t := table{bySuite: map[string][]row{}, headed: map[string]bool{}}
	b, err := os.ReadFile(path)
	if err != nil {
		return t, fmt.Errorf("unreadable: %v", errText(err))
	}
	in, heading := false, false
	current := ""
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "## ") {
			in = strings.TrimSpace(line) == "## Inventory"
			heading = heading || in
			current = ""
			continue
		}
		if !in {
			continue
		}
		if strings.HasPrefix(line, "### ") {
			current = strings.TrimSpace(strings.TrimPrefix(line, "### "))
			if !slices.ContainsFunc(suites, func(s suite) bool { return s.name == current }) {
				t.problems = append(t.problems, fmt.Sprintf("%s: inventory heading %q names no consumes suite", readme, line))
				current = "?"
			} else {
				t.headed[current] = true
			}
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		cells := strings.Split(strings.TrimSpace(line), "|")
		if len(cells) < 3 {
			continue
		}
		cell := strings.TrimSpace(cells[1])
		if !(len(cell) > 2 && strings.HasPrefix(cell, bq) && strings.HasSuffix(cell, bq) && !strings.Contains(cell[1:len(cell)-1], bq)) {
			continue
		}
		name := cell[1 : len(cell)-1]
		if current == "" {
			t.problems = append(t.problems, fmt.Sprintf("%s: inventory row %s is under no suite heading", readme, name))
			continue
		}
		if current == "?" {
			continue
		}
		t.bySuite[current] = append(t.bySuite[current], row{name, uniq(testName.FindAllString(strings.Join(cells[2:], "|"), -1))})
	}
	if !heading {
		return t, fmt.Errorf("no \"## Inventory\" heading")
	}
	return t, nil
}

func errText(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// usesOf type-checks the files of one package and returns each use of another package's
// exported identifiers (by full id '<import path>\x00<member>') with the module it belongs to,
// the top-level Test functions of the files, and for each use the tests whose body names it.
func usesOf(fset *token.FileSet, imp types.Importer, moduleOf func(string) string, path, dir string, names []string) found {
	var files []*ast.File
	for _, n := range names {
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, parser.SkipObjectResolution)
		if err != nil {
			fail(err)
		}
		files = append(files, f)
	}
	type span struct {
		name     string
		pos, end token.Pos
	}
	var spans []span
	out := found{module: map[string]string{}, tests: map[string]map[string]bool{}, funcs: map[string]bool{}}
	for _, f := range files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || !strings.HasPrefix(fd.Name.Name, "Test") || fd.Body == nil {
				continue
			}
			out.funcs[fd.Name.Name] = true
			spans = append(spans, span{fd.Name.Name, fd.Body.Pos(), fd.Body.End()})
		}
	}
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}}
	pkg, err := (&types.Config{Importer: imp}).Check(path, fset, files, info)
	if err != nil {
		fail(fmt.Errorf("type-check %s: %v", dir, err))
	}
	add := func(obj types.Object, at token.Pos) {
		if obj == nil || obj.Pkg() == nil || obj.Pkg() == pkg {
			return
		}
		id := idOf(obj)
		if id == "" {
			return
		}
		out.module[id] = moduleOf(obj.Pkg().Path())
		for _, s := range spans {
			if at >= s.pos && at < s.end {
				if out.tests[id] == nil {
					out.tests[id] = map[string]bool{}
				}
				out.tests[id][s.name] = true
			}
		}
	}
	for ident, obj := range info.Uses {
		add(obj, ident.Pos())
	}
	for sel, s := range info.Selections {
		add(s.Obj(), sel.Sel.Pos())
	}
	return out
}

// idOf names an object of another package: '<import path>\x00Name' for a package-level one,
// '<import path>\x00Type.Member' for a method or field ("" for anything else).
func idOf(obj types.Object) string {
	p := obj.Pkg()
	switch o := obj.(type) {
	case *types.Func:
		sig := o.Origin().Type().(*types.Signature)
		if sig.Recv() == nil {
			break
		}
		t := sig.Recv().Type()
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		}
		if n, ok := t.(*types.Named); ok {
			return p.Path() + "\x00" + n.Origin().Obj().Name() + "." + o.Name()
		}
		return p.Path() + "\x00?." + o.Name()
	case *types.Var:
		if !o.IsField() {
			break
		}
		if owner := fieldOwner(p, o.Origin()); owner != "" {
			return p.Path() + "\x00" + owner
		}
		return p.Path() + "\x00?." + o.Name()
	}
	if obj.Parent() != p.Scope() || !obj.Exported() {
		return ""
	}
	return p.Path() + "\x00" + obj.Name()
}

// fieldOwner is 'Type.Field' (or 'Type.Outer.Field' for a field of an anonymous struct field)
// for the field v declared by a named type of package p.
func fieldOwner(p *types.Package, v *types.Var) string {
	var find func(prefix string, s *types.Struct) string
	find = func(prefix string, s *types.Struct) string {
		for i := 0; i < s.NumFields(); i++ {
			f := s.Field(i)
			if f == v {
				return prefix + "." + f.Name()
			}
			if inner, ok := f.Type().(*types.Struct); ok {
				if got := find(prefix+"."+f.Name(), inner); got != "" {
					return got
				}
			}
		}
		return ""
	}
	for _, name := range p.Scope().Names() {
		tn, ok := p.Scope().Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		if s, ok := tn.Type().Underlying().(*types.Struct); ok {
			if got := find(name, s); got != "" {
				return got
			}
		}
	}
	return ""
}

// short is the README form of an id: the package's last path element, a dot and the member.
func short(id string) string {
	path, member, _ := strings.Cut(id, "\x00")
	return path[strings.LastIndex(path, "/")+1:] + "." + member
}

func pkgOfID(id string) string {
	path, _, _ := strings.Cut(id, "\x00")
	return path
}

func ownPlain(p listed) bool {
	return p.Module != nil && p.Module.Path == own && p.ForTest == "" && !strings.Contains(p.ImportPath, " [") && !strings.HasSuffix(p.ImportPath, ".test")
}

func under(root, dir string, rels ...string) bool {
	for _, rel := range rels {
		base := filepath.Join(root, rel)
		if dir == base || strings.HasPrefix(dir, base+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func goList(root string) []listed {
	cmd := exec.Command("go", "list", "-e", "-export", "-deps", "-test", "-json", "./...")
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		fail(fmt.Errorf("go list -e -export -deps -test -json ./... (in %s): %v", root, err))
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	var pkgs []listed
	for {
		var p listed
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			fail(err)
		}
		if p.Error != nil {
			fail(fmt.Errorf("go list: %s: %s", p.ImportPath, p.Error.Err))
		}
		pkgs = append(pkgs, p)
	}
	return pkgs
}

func sorted[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func uniq(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return slices.Compact(out)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "observer:", err)
	os.Exit(3)
}
`,
};
