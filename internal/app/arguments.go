package app

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"givens/internal/assert"
	"givens/internal/helpers"
)

// writeArguments writes each doc string and data table in the scenario's steps, Background
// first, to a file of its own, and returns GIVENS_ARG_<n>=<path> for each, counting from 1.
// A data table is CSV with its header row first. The files live in a temporary folder the
// next Drop in in this window removes.
// Ref: https://pkg.go.dev/encoding/csv
func (s *Session) writeArguments() ([]string, error) {
	assert.That(s != nil, "arguments are written for a session")
	assert.That(s.window != nil, "a session belongs to a window")
	steps := append(append(s.scenario.Background[:0:0], s.scenario.Background...),
		s.scenario.Steps...)
	env := make([]string, 0, 2)
	for _, step := range steps {
		if step.DocString == "" && step.Table == nil {
			continue
		}
		if s.argumentsDir == "" {
			dir, err := os.MkdirTemp("", "givens-arguments-")
			if err != nil {
				return nil, err
			}
			s.argumentsDir = dir
		}
		number := strconv.Itoa(len(env) + 1)
		path := filepath.Join(s.argumentsDir, "argument-"+number)
		if err := writeArgument(path, step.DocString, step.Table); err != nil {
			return nil, err
		}
		env = append(env, "GIVENS_ARG_"+number+"="+path)
	}
	assert.That(len(env) <= len(steps), "each argument comes from a step")
	return env, nil
}

// writeArgument writes a doc string as it is, or a table as CSV.
func writeArgument(path string, docString string, table [][]string) error {
	assert.That(filepath.IsAbs(path), "the argument file is absolute")
	assert.That(len(docString) <= helpers.FileSizeBytesMax, "a doc string stays bounded")
	if table == nil {
		return os.WriteFile(path, []byte(docString), 0o644)
	}
	var text strings.Builder
	writer := csv.NewWriter(&text)
	if err := writer.WriteAll(table); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text.String()), 0o644)
}

// removeArguments removes the session's argument files, if it wrote any.
func (s *Session) removeArguments() {
	assert.That(s != nil, "arguments are removed for a session")
	assert.That(s.argumentsDir == "" || strings.Contains(s.argumentsDir, "givens-arguments-"),
		"only an arguments folder is removed")
	if s.argumentsDir != "" {
		_ = os.RemoveAll(s.argumentsDir)
	}
}
