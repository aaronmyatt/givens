package rules

import (
	"go/ast"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"

	"givens/internal/assert"
)

// functionLength: Tiger Style "restrict the length of function bodies to 70 lines";
// Power of Ten rule 4 ("no function longer than a printed page").
func runFunctionLength(pass *analysis.Pass) (any, error) {
	assert.That(pass != nil, "pass is set")
	assert.That(pass.Fset != nil, "pass has a file set")
	for _, fn := range funcDecls(pass, true) {
		open, end := pass.Fset.Position(fn.Body.Lbrace), pass.Fset.Position(fn.Body.Rbrace)
		lines := end.Line - open.Line - 1
		if lines > FunctionLinesMax {
			pass.Reportf(fn.Pos(), "%s has %d lines; split it to %d or fewer",
				fn.Name.Name, lines, FunctionLinesMax)
		}
	}
	return nil, nil
}

// nestingDepth: Tiger Style "centralize control flow" and "push ifs up and fors down".
// An `else if` chain counts as one level, not one per branch.
func runNestingDepth(pass *analysis.Pass) (any, error) {
	assert.That(pass != nil, "pass is set")
	assert.That(NestingDepthMax > 0, "the limit is positive")
	for _, fn := range funcDecls(pass, true) {
		if pos, deep := deepestNesting(fn.Body); deep {
			pass.Reportf(pos.Pos(), "%s nests deeper than %d levels; extract a function",
				fn.Name.Name, NestingDepthMax)
		}
	}
	return nil, nil
}

// deepestNesting walks body with an explicit stack (no recursion) and returns the first node
// that goes past NestingDepthMax. ast.Inspect calls f(nil) after each node's children.
// Ref: https://pkg.go.dev/go/ast#Inspect
func deepestNesting(body *ast.BlockStmt) (ast.Node, bool) {
	assert.That(body != nil, "body is set")
	nodes := make([]ast.Node, 0, 64)
	nests := make([]bool, 0, 64)
	depth := 0
	var found ast.Node
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			if nests[len(nests)-1] {
				depth--
			}
			nodes, nests = nodes[:len(nodes)-1], nests[:len(nests)-1]
			return true
		}
		nesting := isNesting(n, nodes)
		if nesting {
			depth++
		}
		if nesting && depth > NestingDepthMax && found == nil {
			found = n
		}
		nodes, nests = append(nodes, n), append(nests, nesting)
		return true
	})
	assert.That(depth == 0, "every nesting level opened is closed")
	return found, found != nil
}

// isNesting reports whether n opens a nesting level, given the nodes above it.
func isNesting(n ast.Node, parents []ast.Node) bool {
	assert.That(n != nil, "n is set")
	assert.That(len(parents) < 1<<16, "the node stack stays bounded")
	switch node := n.(type) {
	case *ast.IfStmt:
		if len(parents) == 0 {
			return true
		}
		parent, ok := parents[len(parents)-1].(*ast.IfStmt)
		return !ok || parent.Else != node
	case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt,
		*ast.FuncLit:
		return true
	}
	return false
}

// lineLength: Tiger Style "hard limit all line lengths, without exception, to at most 100
// columns". A tab counts as TabColumns.
func runLineLength(pass *analysis.Pass) (any, error) {
	assert.That(pass != nil, "pass is set")
	assert.That(pass.ReadFile != nil, "the driver can read files")
	for _, file := range sourceFiles(pass) {
		name := pass.Fset.Position(file.Package).Filename
		content, err := pass.ReadFile(name)
		if err != nil {
			return nil, err
		}
		tokenFile := pass.Fset.File(file.Package)
		for index, line := range strings.Split(string(content), "\n") {
			if columns := lineColumns(line); columns > LineColumnsMax {
				pass.Reportf(tokenFile.LineStart(index+1), "line is %d columns; the limit is %d",
					columns, LineColumnsMax)
			}
		}
	}
	return nil, nil
}

// lineColumns counts runes, with each tab as TabColumns.
func lineColumns(line string) int {
	assert.That(!strings.Contains(line, "\n"), "a line holds no newline")
	columns := utf8.RuneCountInString(line) + strings.Count(line, "\t")*(TabColumns-1)
	assert.That(columns >= utf8.RuneCountInString(line), "tabs only add columns")
	return columns
}

// debtMarkers: Tiger Style "zero technical debt". Work left to do goes in todo.txt (txtodo),
// never in a comment where nobody tracks it.
func runDebtMarkers(pass *analysis.Pass) (any, error) {
	assert.That(pass != nil, "pass is set")
	// Built per run, not held in a package-level variable (see noglobals).
	marker := regexp.MustCompile(`\b(TODO|FIXME|XXX|HACK)\b`)
	assert.That(marker.NumSubexp() == 1, "the pattern has one group")
	for _, file := range sourceFiles(pass) {
		for _, group := range file.Comments {
			if marker.MatchString(group.Text()) {
				pass.Reportf(group.Pos(), "debt marker in a comment; add it to todo.txt instead")
			}
		}
	}
	return nil, nil
}
