package facts

import (
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Scan parses every _test.go file in dir and returns the package facts.
//
// Only syntax is needed: no type checking, no go/packages. Files that fail to
// parse are skipped (a broken file is reported by `go build`, not by us).
//
// Files excluded by a build constraint are skipped. That is not pedantry: our
// own regression test carries `//go:build flakesmith` and full of t.Run calls,
// so counting it would mark the package as "uses subtests" and make every later
// hunt in that package skip itself — a silent, self-inflicted no-op.
//
// The package's non-test .go files are parsed too, but ONLY for env evidence
// (AppEnvReads + their comparison literals): a test can flip on a variable the
// application code reads. Every other fact stays a strict test-file signal so
// the order probe's candidates do not gain noise from app code.
func Scan(dir string) (PkgFacts, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return PkgFacts{}, err
	}

	f := PkgFacts{
		Dir:            dir,
		Clause:         map[string]string{},
		TestPos:        map[string]Pos{},
		GlobalsWritten: map[string][]string{},
		GlobalsRead:    map[string][]string{},
		EnvValues:      map[string][]string{},
		FileCalls:      map[string][]string{},
		TestSubtests:   map[string]bool{},
		TestParallel:   map[string]bool{},
		TestSynctest:   map[string]bool{},
	}

	fset := token.NewFileSet()
	var parsed []*ast.File

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		// "_" and "." prefixes are ignored by the go tool; mirror that.
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil || excludedByBuildTags(src) {
			continue
		}
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			continue
		}
		parsed = append(parsed, file)
		f.Files = append(f.Files, name)

		clause := file.Name.Name
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name == nil {
				continue
			}
			fnName := fn.Name.Name
			if strings.HasPrefix(fnName, "Test") {
				f.Clause[fnName] = clause
				// First declaration wins: a redefinition would not compile
				// anyway, and a map that changes mid-scan is a trap.
				if _, dup := f.TestPos[fnName]; !dup {
					f.TestPos[fnName] = Pos{File: path, Line: fset.Position(fn.Pos()).Line}
				}
			}
			if fnName == "TestMain" {
				f.HasTestMain = true
			}
		}
	}
	sort.Strings(f.Files)

	globals := collectGlobals(parsed)

	for _, file := range parsed {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name == nil {
				continue
			}
			name := fn.Name.Name
			u := analyseFunc(fn, globals)
			if len(u.writes) > 0 {
				f.GlobalsWritten[name] = u.writes
			}
			if len(u.reads) > 0 {
				f.GlobalsRead[name] = u.reads
			}
			if len(u.fsCalls) > 0 {
				f.FileCalls[name] = u.fsCalls
			}
			f.EnvReads = append(f.EnvReads, u.envs...)
			for k, vals := range u.envVals {
				f.EnvValues[k] = append(f.EnvValues[k], vals...)
			}
			if u.subtest {
				f.Subtests = true
				f.TestSubtests[name] = true
			}
			if u.parallel {
				f.TestParallel[name] = true
			}
			if u.synctest {
				f.TestSynctest[name] = true
			}
		}
	}

	f.EnvReads = dedupeSort(f.EnvReads)

	// Application files: an os.Getenv read that lives in the package's regular
	// code (the test exercises it through a helper or the API under test) is
	// invisible to the test-file scan, yet the env probe can reproduce it —
	// the witness runs the real `go test`, not our analysis. The keys go to a
	// SEPARATE field and the probe tries them after the test-local ones: app
	// code reads many env vars no test outcome depends on, and candidate order
	// is the probe's budget order.
	testKeys := map[string]bool{}
	for _, k := range f.EnvReads {
		testKeys[k] = true
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil || excludedByBuildTags(src) {
			continue
		}
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Body == nil {
				continue // asm stubs have no body; methods are wanted (env reads hide in them)
			}
			// nil globals: only the env evidence matters here; writes and
			// reads of package vars stay a strictly test-file signal so the
			// order probe's pollution candidates do not gain noise.
			u := analyseFunc(fn, nil)
			for _, k := range u.envs {
				if !testKeys[k] {
					f.AppEnvReads = append(f.AppEnvReads, k)
				}
			}
			for k, vals := range u.envVals {
				f.EnvValues[k] = append(f.EnvValues[k], vals...)
			}
		}
	}
	f.AppEnvReads = dedupeSort(f.AppEnvReads)

	for k, vals := range f.EnvValues {
		f.EnvValues[k] = dedupeSort(vals)
	}
	f.ExternalDependencies = externalDependencies(dir)
	f.ImportPath = importPath(dir)
	return f, nil
}

// excludedByBuildTags reports whether the file would be left out of a build
// that passes no tags — which is exactly what `go test` does by default.
//
// Only the header is inspected: build constraints are required to appear before
// the package clause, so the walk stops at the first line that is neither a
// comment nor a constraint.
func excludedByBuildTags(src []byte) bool {
	var exprs []constraint.Expr
header:
	for _, raw := range strings.Split(string(src), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
		case constraint.IsGoBuild(line) || constraint.IsPlusBuild(line):
			e, err := constraint.Parse(line)
			if err != nil {
				continue
			}
			exprs = append(exprs, e)
		case strings.HasPrefix(line, "//"), strings.HasPrefix(line, "/*"):
			// Other header comments (license, "Code generated", doc) are not a barrier.
		default:
			break header
		}
	}
	if len(exprs) == 0 {
		return false
	}
	tags := defaultBuildTags()
	for _, e := range exprs {
		if !e.Eval(func(tag string) bool { return tags[tag] }) {
			return true
		}
	}
	return false
}

// defaultBuildTags mirrors the tags the go tool sets when none are passed
// explicitly. Only tags that can appear in a test file matter here; the go1.x
// series is cumulative and included so version-gated files resolve correctly.
func defaultBuildTags() map[string]bool {
	tags := map[string]bool{"gc": true, runtime.GOOS: true, runtime.GOARCH: true}
	ver := strings.TrimPrefix(runtime.Version(), "go")
	parts := strings.Split(ver, ".")
	if len(parts) >= 2 {
		major, err1 := strconv.Atoi(parts[0])
		minor, err2 := strconv.Atoi(parts[1])
		if err1 == nil && err2 == nil {
			for m := 0; m <= minor; m++ {
				tags[strconv.Itoa(major)+"."+strconv.Itoa(m)] = true
				tags["go"+strconv.Itoa(major)+"."+strconv.Itoa(m)] = true
			}
		}
	}
	return tags
}

// collectGlobals returns the names of package-level vars and consts in all
// parsed files (const included: a test may read a package const that another
// test's TestMain mutates via a var alias — cheap to include).
func collectGlobals(files []*ast.File) map[string]bool {
	out := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || (gen.Tok != token.VAR && gen.Tok != token.CONST) {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, n := range vs.Names {
					out[n.Name] = true
				}
			}
		}
	}
	return out
}

// funcUsage is what one test function does to the world around it.
type funcUsage struct {
	writes   []string
	reads    []string
	envs     []string
	envVals  map[string][]string
	fsCalls  []string
	subtest  bool
	parallel bool
	synctest bool
}

// analyseFunc walks one function body and classifies its effects.
//
// Assignment (LHS of =, :=, ++, --) counts as a write; every other identifier
// reference counts as a read. Conservative on purpose: a write is also a read
// in most cases, and over-reporting a read is harmless while under-reporting a
// write loses the signal.
func analyseFunc(fn *ast.FuncDecl, globals map[string]bool) funcUsage {
	u := funcUsage{envVals: map[string][]string{}}
	writeSet := map[string]bool{}
	readSet := map[string]bool{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range x.Lhs {
				markWrites(lhs, globals, writeSet)
			}
		case *ast.IncDecStmt:
			markWrites(x.X, globals, writeSet)
		case *ast.BinaryExpr:
			// os.Getenv("X") == "literal": a value candidate. This is the only
			// realistic hint for the env probe — the value the user wrote in their code.
			if k, v, ok := envCompare(x); ok {
				u.envVals[k] = append(u.envVals[k], v)
			}
		case *ast.CallExpr:
			if key, ok := envKey(x); ok {
				u.envs = append(u.envs, key)
			}
			if name, ok := filesystemCall(x); ok {
				u.fsCalls = append(u.fsCalls, name)
			}
			if isTRun(x) {
				u.subtest = true
			}
			if isTParallel(x) {
				u.parallel = true
			}
			if isSynctestCall(x) {
				u.synctest = true
			}
		case *ast.Ident:
			if globals[x.Name] {
				readSet[x.Name] = true
			}
		}
		return true
	})

	for v := range writeSet {
		u.writes = append(u.writes, v)
		delete(readSet, v) // written vars also appear as idents; keep them in writes only
	}
	for v := range readSet {
		u.reads = append(u.reads, v)
	}
	sort.Strings(u.writes)
	sort.Strings(u.reads)
	sort.Strings(u.fsCalls)
	u.fsCalls = dedupeSort(u.fsCalls)
	return u
}

// envCompare matches `os.Getenv("K") == "v"` (and !=) in both operand orders.
func envCompare(b *ast.BinaryExpr) (key, value string, ok bool) {
	if b.Op != token.EQL && b.Op != token.NEQ {
		return "", "", false
	}
	if call, isCall := b.X.(*ast.CallExpr); isCall {
		if k, ok := envKey(call); ok {
			if v, ok := stringLit(b.Y); ok {
				return k, v, true
			}
		}
	}
	if call, isCall := b.Y.(*ast.CallExpr); isCall {
		if k, ok := envKey(call); ok {
			if v, ok := stringLit(b.X); ok {
				return k, v, true
			}
		}
	}
	return "", "", false
}

func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil || s == "" {
		return "", false
	}
	return s, true
}

// filesystemCall recognises the os calls that leave state behind on disk.
//
// Static only, and deliberately so: we are not claiming WHERE it writes (the
// path is usually computed), only that this test leaves something outside the
// process. That is enough for a human to recognise the bug in a report, and it
// keeps the tool away from running code to observe it.
func filesystemCall(call *ast.CallExpr) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || (pkg.Name != "os" && pkg.Name != "ioutil") {
		return "", false
	}
	switch sel.Sel.Name {
	case "WriteFile", "Create", "CreateTemp", "OpenFile", "Mkdir", "MkdirAll",
		"Remove", "RemoveAll", "Rename", "Symlink", "Chdir":
		return pkg.Name + "." + sel.Sel.Name, true
	}
	return "", false
}

// markWrites records globals assigned through the common shapes:
// x = ..., x := ..., x, y = ..., &x, pkg.x (selector on a package-level var is
// not a global here), x[i] = ..., x.f = ...
func markWrites(expr ast.Expr, globals map[string]bool, out map[string]bool) {
	switch e := expr.(type) {
	case *ast.Ident:
		if globals[e.Name] {
			out[e.Name] = true
		}
	case *ast.StarExpr:
		markWrites(e.X, globals, out)
	case *ast.IndexExpr:
		markWrites(e.X, globals, out)
	case *ast.SelectorExpr:
		markWrites(e.X, globals, out)
	case *ast.ParenExpr:
		markWrites(e.X, globals, out)
	}
}

// envKey extracts "KEY" from os.Getenv("KEY") / os.LookupEnv("KEY").
func envKey(call *ast.CallExpr) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "os" {
		return "", false
	}
	if sel.Sel.Name != "Getenv" && sel.Sel.Name != "LookupEnv" {
		return "", false
	}
	if len(call.Args) != 1 {
		return "", false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil || s == "" {
		return "", false
	}
	return s, true
}

func isTRun(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Run" {
		return false
	}
	recv, ok := sel.X.(*ast.Ident)
	return ok && (recv.Name == "t" || recv.Name == "b" || recv.Name == "tb")
}

// isTParallel matches t.Parallel() / b.Parallel().
func isTParallel(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Parallel" || len(call.Args) != 0 {
		return false
	}
	recv, ok := sel.X.(*ast.Ident)
	return ok && (recv.Name == "t" || recv.Name == "b" || recv.Name == "tb")
}

// isSynctestCall matches the testing/synctest bubble API across its whole
// lineage: synctest.Run (1.24/1.25 preview and the golang.org/x/sync/synctest
// preview, same selector name), synctest.Omega (the 1.24 stdlib preview
// spelling), synctest.Test (graduated spelling, Go 1.26), plus Wait and Go.
// The match is identifier-name only, no import resolution — same contract as
// filesystemCall: we read syntax and never run the type checker.
//
// Go is included though it opens no bubble itself: every legal synctest.Go
// sits inside a bubble body, so it names the same test.
func isSynctestCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "synctest" {
		return false
	}
	switch sel.Sel.Name {
	case "Run", "Test", "Wait", "Go", "Omega":
		return true
	}
	return false
}

func dedupeSort(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// importPath reads the module path from the nearest go.mod and appends the
// relative directory. Cheap and good enough for capsule identity; the accurate
// answer comes from `go list` (gotest.Discover already does that).
func importPath(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	cur := abs
	var rel []string
	for {
		if mod := filepath.Join(cur, "go.mod"); fileExists(mod) {
			return joinModule(mod, rel)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		rel = append([]string{filepath.Base(cur)}, rel...)
		cur = parent
	}
}

func joinModule(modPath string, rel []string) string {
	data, err := os.ReadFile(modPath)
	if err != nil {
		return ""
	}
	module := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			module = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			break
		}
	}
	if module == "" {
		return ""
	}
	if len(rel) == 0 {
		return module
	}
	return module + "/" + strings.Join(rel, "/")
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
