package rules

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"

	"givens/internal/assert"
)

// noRecursion: Power of Ten rule 1 "no recursion"; Tiger Style "do not use recursion".
// Catches direct recursion: a function or method that calls itself.
func runNoRecursion(pass *analysis.Pass) (any, error) {
	assert.That(pass != nil, "pass is set")
	assert.That(pass.TypesInfo != nil, "pass has type information")
	for _, fn := range funcDecls(pass, true) {
		self := pass.TypesInfo.Defs[fn.Name]
		if self == nil {
			continue
		}
		// StaticCallee resolves a call to the function or method it names, if known statically.
		// Ref: https://pkg.go.dev/golang.org/x/tools/go/types/typeutil#StaticCallee
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := typeutil.StaticCallee(pass.TypesInfo, call)
			if callee != nil && callee == self {
				pass.Reportf(call.Pos(), "%s calls itself; use a bounded loop", fn.Name.Name)
			}
			return true
		})
	}
	return nil, nil
}

// boundedLoops: Power of Ten rule 2 "all loops must have a fixed upper bound"; Tiger Style
// "put a limit on everything". Range loops and three-clause loops state their bound. Any other
// loop needs a `// bound: <why it ends>` comment on its line or the line above.
func runBoundedLoops(pass *analysis.Pass) (any, error) {
	assert.That(pass != nil, "pass is set")
	assert.That(pass.Fset != nil, "pass has a file set")
	for _, file := range sourceFiles(pass) {
		bounds := boundLines(pass, file)
		ast.Inspect(file, func(n ast.Node) bool {
			loop, ok := n.(*ast.ForStmt)
			if !ok || (loop.Init != nil && loop.Cond != nil && loop.Post != nil) {
				return true
			}
			line := pass.Fset.Position(loop.For).Line
			if !bounds[line] && !bounds[line-1] {
				pass.Reportf(loop.For, "loop has no stated bound; add `// bound: <why it ends>`")
			}
			return true
		})
	}
	return nil, nil
}

// boundLines returns the lines of file that carry a `// bound:` comment.
func boundLines(pass *analysis.Pass, file *ast.File) map[int]bool {
	assert.That(pass != nil, "pass is set")
	assert.That(file != nil, "file is set")
	lines := make(map[int]bool, len(file.Comments))
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "// bound:") {
				lines[pass.Fset.Position(comment.Slash).Line] = true
			}
		}
	}
	return lines
}

// simpleFlow: Power of Ten rule 1 "simple control flow". No goto, no init (code that runs
// before main without being called), no recover (crashes must stay crashes), and panic only
// inside internal/assert.
func runSimpleFlow(pass *analysis.Pass) (any, error) {
	assert.That(pass != nil, "pass is set")
	assert.That(pass.Pkg != nil, "pass has a package")
	inAssert := pass.Pkg.Path() == assertPackagePath
	for _, file := range sourceFiles(pass) {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.BranchStmt:
				if node.Tok == token.GOTO {
					pass.Reportf(node.Pos(), "goto is banned; restructure the flow")
				}
			case *ast.FuncDecl:
				if node.Recv == nil && node.Name.Name == "init" {
					pass.Reportf(node.Pos(), "init is banned; call setup explicitly from main")
				}
			case *ast.CallExpr:
				checkBuiltinCall(pass, node, inAssert)
			}
			return true
		})
	}
	return nil, nil
}

// checkBuiltinCall reports recover anywhere, and panic outside internal/assert.
func checkBuiltinCall(pass *analysis.Pass, call *ast.CallExpr, inAssert bool) {
	assert.That(pass != nil, "pass is set")
	assert.That(call != nil, "call is set")
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return
	}
	if _, builtin := pass.TypesInfo.Uses[ident].(*types.Builtin); !builtin {
		return
	}
	if ident.Name == "recover" {
		pass.Reportf(call.Pos(), "recover is banned; let crashes crash")
	}
	if ident.Name == "panic" && !inAssert {
		pass.Reportf(call.Pos(), "panic is banned; use assert.That for bugs, errors for failures")
	}
}
