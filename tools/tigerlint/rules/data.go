package rules

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"givens/internal/assert"
)

// noGlobals: Power of Ten rule 6 "declare all data objects at the smallest possible level of
// scope". Package-level variables are banned, except `_`, sentinel errors named Err*, and a
// spec carrying a `tiger:allow-global` comment that says why.
func runNoGlobals(pass *analysis.Pass) (any, error) {
	assert.That(pass != nil, "pass is set")
	assert.That(pass.Files != nil, "pass has files")
	for _, file := range sourceFiles(pass) {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if ok && gen.Tok == token.VAR {
				reportGlobals(pass, gen)
			}
		}
	}
	return nil, nil
}

// reportGlobals reports each package-level variable in one var declaration.
func reportGlobals(pass *analysis.Pass, gen *ast.GenDecl) {
	assert.That(pass != nil, "pass is set")
	assert.That(gen.Tok == token.VAR, "gen declares variables")
	for _, spec := range gen.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok || allowsGlobal(gen.Doc) || allowsGlobal(value.Doc) || allowsGlobal(value.Comment) {
			continue
		}
		for _, name := range value.Names {
			if name.Name != "_" && !strings.HasPrefix(name.Name, "Err") {
				pass.Reportf(name.Pos(), "package-level variable %s; pass it in or make it a const",
					name.Name)
			}
		}
	}
}

// allowsGlobal reports whether a comment group carries `tiger:allow-global`.
func allowsGlobal(group *ast.CommentGroup) bool {
	if group == nil {
		return false
	}
	assert.That(len(group.List) > 0, "a comment group holds at least one comment")
	for _, comment := range group.List {
		assert.That(strings.HasPrefix(comment.Text, "/"), "a comment starts with a slash")
		if strings.Contains(comment.Text, "tiger:allow-global") {
			return true
		}
	}
	return false
}

// noDoublePointers: Power of Ten rule 9 "no more than one level of dereferencing".
func runNoDoublePointers(pass *analysis.Pass) (any, error) {
	assert.That(pass != nil, "pass is set")
	assert.That(pass.Files != nil, "pass has files")
	for _, file := range sourceFiles(pass) {
		ast.Inspect(file, func(n ast.Node) bool {
			star, ok := n.(*ast.StarExpr)
			if !ok {
				return true
			}
			if _, double := star.X.(*ast.StarExpr); double {
				pass.Reportf(star.Pos(), "two levels of pointer; keep it to one")
			}
			return true
		})
	}
	return nil, nil
}

// sizedMake: Power of Ten rule 3 and Tiger Style "static memory allocation", adapted to Go's
// garbage collector: when a slice or map is made, say how big it will get.
func runSizedMake(pass *analysis.Pass) (any, error) {
	assert.That(pass != nil, "pass is set")
	assert.That(pass.TypesInfo != nil, "pass has type information")
	for _, file := range sourceFiles(pass) {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if ok && len(call.Args) == 1 && isBuiltin(pass, call.Fun, "make") {
				switch pass.TypesInfo.TypeOf(call.Args[0]).Underlying().(type) {
				case *types.Slice, *types.Map:
					pass.Reportf(call.Pos(), "make without a size; give a length or capacity")
				}
			}
			return true
		})
	}
	return nil, nil
}

// isBuiltin reports whether expr names the builtin called name.
func isBuiltin(pass *analysis.Pass, expr ast.Expr, name string) bool {
	assert.That(pass != nil, "pass is set")
	assert.That(name != "", "name is set")
	ident, ok := expr.(*ast.Ident)
	if !ok || ident.Name != name {
		return false
	}
	_, builtin := pass.TypesInfo.Uses[ident].(*types.Builtin)
	return builtin
}

// forbiddenImports: Power of Ten rule 8 (limit the preprocessor) and rule 9 (restrict pointers),
// adapted to Go: no unsafe, no reflect, no cgo. Each one hides what the code does.
func runForbiddenImports(pass *analysis.Pass) (any, error) {
	assert.That(pass != nil, "pass is set")
	assert.That(pass.Files != nil, "pass has files")
	for _, file := range sourceFiles(pass) {
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return nil, err
			}
			if path == "unsafe" || path == "reflect" || path == "C" {
				pass.Reportf(spec.Pos(), "import %q is banned", path)
			}
		}
	}
	return nil, nil
}
