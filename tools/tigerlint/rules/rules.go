// Package rules holds tigerlint's analyzers: one per constraint in CONSTRAINTS.md.
//
// Each analyzer is a go/analysis pass, run by `go run ./tools/tigerlint ./...`.
// Ref: https://pkg.go.dev/golang.org/x/tools/go/analysis
//
// Sources for the rules:
// Tiger Style: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md
// Power of Ten:
// https://en.wikipedia.org/wiki/The_Power_of_10:_Rules_for_Developing_Safety-Critical_Code
package rules

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"givens/internal/assert"
)

// Limits. Each one is named where it's enforced, so a change here changes the rule.
const (
	// FunctionLinesMax: Tiger Style "restrict the length of function bodies to 70 lines".
	FunctionLinesMax = 70
	// LineColumnsMax: Tiger Style "hard limit all line lengths to at most 100 columns".
	LineColumnsMax = 100
	// TabColumns: a tab counts as 4 columns when measuring a line.
	TabColumns = 4
	// NestingDepthMax: nested if/for/switch/select/func literals inside one function.
	NestingDepthMax = 4
	// AssertsPerFunctionMin: Power of Ten rule 5, averaged over a package.
	AssertsPerFunctionMin = 2

	assertPackagePath = "givens/internal/assert"
	analyzerCount     = 13
)

// All returns every analyzer. They are built here, not held in package-level variables.
func All() []*analysis.Analyzer {
	analyzers := []*analysis.Analyzer{
		{Name: "functionlength", Run: runFunctionLength,
			Doc: "function bodies are at most 70 lines (Tiger Style; Power of Ten rule 4)"},
		{Name: "nestingdepth", Run: runNestingDepth,
			Doc: "control flow nests at most 4 levels inside a function (Tiger Style)"},
		{Name: "linelength", Run: runLineLength,
			Doc: "lines are at most 100 columns, tabs counted as 4 (Tiger Style)"},
		{Name: "debtmarkers", Run: runDebtMarkers,
			Doc: "comments never hold debt markers; track the work in todo.txt (Tiger Style)"},
		{Name: "norecursion", Run: runNoRecursion,
			Doc: "functions never call themselves (Power of Ten rule 1; Tiger Style)"},
		{Name: "boundedloops", Run: runBoundedLoops,
			Doc: "a loop without range or 3 clauses has a `// bound:` comment (Power of Ten 2)"},
		{Name: "simpleflow", Run: runSimpleFlow,
			Doc: "no goto, init, recover, or panic outside internal/assert (Power of Ten rule 1)"},
		{Name: "noglobals", Run: runNoGlobals,
			Doc: "no package-level variables except Err* sentinels (Power of Ten rule 6)"},
		{Name: "nodoublepointers", Run: runNoDoublePointers,
			Doc: "no pointer-to-pointer types or double dereferences (Power of Ten rule 9)"},
		{Name: "sizedmake", Run: runSizedMake,
			Doc: "make of a slice or map states a size (Power of Ten rule 3, adapted)"},
		{Name: "forbiddenimports", Run: runForbiddenImports,
			Doc: "no unsafe, reflect or cgo (Power of Ten rules 8 and 9, adapted)"},
		{Name: "checkederrors", Run: runCheckedErrors,
			Doc: "no call drops an error result (Power of Ten rule 7; Tiger Style)"},
		{Name: "assertdensity", Run: runAssertDensity,
			Doc: "each package averages 2 or more assert calls per function (Power of Ten rule 5)"},
	}
	assert.That(len(analyzers) == analyzerCount, "every rule is listed once")
	for _, analyzer := range analyzers {
		assert.That(analyzer.Name != "" && analyzer.Run != nil, "each rule has a name and a run")
	}
	return analyzers
}

// sourceFiles lists the package's hand-written files, skipping generated ones such as the main
// package go test writes. Ref: https://pkg.go.dev/go/ast#IsGenerated
func sourceFiles(pass *analysis.Pass) []*ast.File {
	assert.That(pass != nil, "pass is set")
	files := make([]*ast.File, 0, len(pass.Files))
	for _, file := range pass.Files {
		if !ast.IsGenerated(file) {
			files = append(files, file)
		}
	}
	assert.That(len(files) <= len(pass.Files), "files only shrink")
	return files
}

// isTestFile reports whether file is a _test.go file.
func isTestFile(pass *analysis.Pass, file *ast.File) bool {
	assert.That(pass != nil, "pass is set")
	assert.That(file != nil, "file is set")
	return strings.HasSuffix(pass.Fset.Position(file.Package).Filename, "_test.go")
}

// funcDecls lists every function and method with a body, in file order.
func funcDecls(pass *analysis.Pass, includeTests bool) []*ast.FuncDecl {
	assert.That(pass != nil, "pass is set")
	decls := make([]*ast.FuncDecl, 0, len(pass.Files)*8)
	for _, file := range sourceFiles(pass) {
		if !includeTests && isTestFile(pass, file) {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Body != nil {
				assert.That(fn.Name != nil, "a function declaration has a name")
				decls = append(decls, fn)
			}
		}
	}
	return decls
}
