package rules

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"

	"givens/internal/assert"
)

// checkedErrors: Power of Ten rule 7 "check the return value of all non-void functions";
// Tiger Style "all errors must be handled". A call used as a statement may not drop an error.
// To drop one on purpose, say so: `_ = file.Close()`. Printing to stdout and writing to an
// in-memory builder can't fail in a way worth handling, so they're exempt.
func runCheckedErrors(pass *analysis.Pass) (any, error) {
	assert.That(pass != nil, "pass is set")
	assert.That(pass.TypesInfo != nil, "pass has type information")
	for _, file := range sourceFiles(pass) {
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.ExprStmt)
			if !ok {
				return true
			}
			call, ok := stmt.X.(*ast.CallExpr)
			if ok && returnsError(pass, call) && !droppableError(pass, call) {
				pass.Reportf(call.Pos(), "error dropped; handle it, or write `_ =` to drop it")
			}
			return true
		})
	}
	return nil, nil
}

// returnsError reports whether call returns an error among its results.
func returnsError(pass *analysis.Pass, call *ast.CallExpr) bool {
	assert.That(pass != nil, "pass is set")
	errorType := types.Universe.Lookup("error").Type()
	assert.That(errorType != nil, "the universe has an error type")
	result := pass.TypesInfo.TypeOf(call)
	tuple, ok := result.(*types.Tuple)
	if !ok {
		return result != nil && types.Identical(result, errorType)
	}
	for index := range tuple.Len() {
		if types.Identical(tuple.At(index).Type(), errorType) {
			return true
		}
	}
	return false
}

// droppableError reports whether call is one whose error is safe to drop.
func droppableError(pass *analysis.Pass, call *ast.CallExpr) bool {
	assert.That(pass != nil, "pass is set")
	assert.That(call != nil, "call is set")
	callee := typeutil.StaticCallee(pass.TypesInfo, call)
	if callee == nil {
		return false
	}
	switch callee.FullName() {
	case "fmt.Print", "fmt.Println", "fmt.Printf",
		"(*strings.Builder).Write", "(*strings.Builder).WriteByte",
		"(*strings.Builder).WriteRune", "(*strings.Builder).WriteString",
		"(*bytes.Buffer).Write", "(*bytes.Buffer).WriteByte",
		"(*bytes.Buffer).WriteRune", "(*bytes.Buffer).WriteString":
		return true
	}
	return false
}

// assertDensity: Power of Ten rule 5 "the assertion density of the code should average to a
// minimum of two assertions per function"; Tiger Style says the same. Counted over each
// package's non-test functions; internal/assert itself is exempt.
func runAssertDensity(pass *analysis.Pass) (any, error) {
	assert.That(pass != nil, "pass is set")
	assert.That(pass.Pkg != nil, "pass has a package")
	if pass.Pkg.Path() == assertPackagePath {
		return nil, nil
	}
	functions := funcDecls(pass, false)
	if len(functions) == 0 {
		return nil, nil
	}
	asserts := 0
	for _, fn := range functions {
		asserts += countAsserts(pass, fn.Body)
	}
	if asserts < AssertsPerFunctionMin*len(functions) {
		pass.Reportf(functions[0].Pos(), "package %s has %d asserts across %d functions; "+
			"it needs %d or more", pass.Pkg.Name(), asserts, len(functions),
			AssertsPerFunctionMin*len(functions))
	}
	return nil, nil
}

// countAsserts counts calls into internal/assert inside body.
func countAsserts(pass *analysis.Pass, body *ast.BlockStmt) int {
	assert.That(pass != nil, "pass is set")
	assert.That(body != nil, "body is set")
	count := 0
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee := typeutil.StaticCallee(pass.TypesInfo, call)
		if callee != nil && callee.Pkg() != nil && callee.Pkg().Path() == assertPackagePath {
			count++
		}
		return true
	})
	return count
}
