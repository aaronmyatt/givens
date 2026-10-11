// Package runs keeps the run log: one line per recorded Drop in, in
// <capabilities>/runs/YYYY-MM-DD.log.
//
// Lines are only ever appended, never rewritten, so capabilities/.gitattributes can mark the
// folder `runs/** merge=union` and git keeps both branches' lines in a merge.
// Ref: https://git-scm.com/docs/gitattributes#_built_in_merge_drivers
//
// A line is tab-separated: time (RFC 3339), run key, result, author, then one field per row
// value (`card=expired`) and per fingerprint part (`fp:steps=1a2b3c4d`), and the note last.
package runs

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"givens/internal/assert"
)

// Limits. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	// RunFilesMax is about 27 years of daily files.
	RunFilesMax = 10000
	// LineLengthBytesMax bounds one run line, note included.
	LineLengthBytesMax = 64 << 10
	// FieldsMax bounds the row values and fingerprint parts on one line.
	FieldsMax = 256
)

// Results.
const (
	ResultPass = "pass"
	ResultFail = "fail"
)

// fingerprintPrefix starts a fingerprint part's field, so it can't be read as a row value.
const fingerprintPrefix = "fp:"

// ErrBadLine is returned for a run line that can't be read.
var ErrBadLine = errors.New("not a run line")

// Pair is a name and a value: a row value (card, expired) or a fingerprint part (steps, 1a2b).
type Pair struct {
	Name  string
	Value string
}

// Run is one recorded Drop in.
type Run struct {
	Time        time.Time
	Key         string // see Key
	Result      string // ResultPass or ResultFail
	Author      string // git's user.name, or ""
	Values      []Pair // the outline row's values, in column order
	Fingerprint []Pair // what the setup was made of when it ran
	Note        string
}

// Key is the name a run is recorded against: <capability>/<scenario slug>, plus
// /<examples slug> for a row of a named Examples block. examplesSlug is "" otherwise.
func Key(scenarioKey string, examplesSlug string) string {
	assert.That(strings.Count(scenarioKey, "/") == 1, "a scenario key is capability/slug")
	assert.That(!strings.Contains(examplesSlug, "/"), "an examples slug is one slug")
	if examplesSlug == "" {
		return scenarioKey
	}
	return scenarioKey + "/" + examplesSlug
}

// Dir is the run log's folder inside a capabilities folder.
func Dir(capabilities string) string {
	assert.That(capabilities != "", "capabilities is a path")
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	return filepath.Join(capabilities, "runs")
}

// Append adds run to its day's file, making runs/ if needed.
func Append(capabilities string, run Run) error {
	assert.That(!run.Time.IsZero(), "a run has a time")
	assert.That(run.Result == ResultPass || run.Result == ResultFail, "a run has a result")
	line := format(run)
	if len(line) > LineLengthBytesMax {
		return fmt.Errorf("the run line for %s is %d bytes, more than %d", run.Key, len(line),
			LineLengthBytesMax)
	}
	if err := os.MkdirAll(Dir(capabilities), 0o755); err != nil {
		return err
	}
	path := filepath.Join(Dir(capabilities), run.Time.Format(time.DateOnly)+".log")
	// O_APPEND makes each write land at the end of the file, so the log is never rewritten.
	// Ref: https://pkg.go.dev/os#OpenFile
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(line + "\n"); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// format writes one run as a line. Tabs and line breaks inside fields become spaces.
func format(run Run) string {
	assert.That(run.Key != "", "a run has a key")
	assert.That(len(run.Values)+len(run.Fingerprint) <= FieldsMax, "fields stay bounded")
	fields := make([]string, 0, 5+len(run.Values)+len(run.Fingerprint))
	fields = append(fields, run.Time.Format(time.RFC3339), run.Key, run.Result, run.Author)
	for _, value := range run.Values {
		fields = append(fields, value.Name+"="+value.Value)
	}
	for _, part := range run.Fingerprint {
		fields = append(fields, fingerprintPrefix+part.Name+"="+part.Value)
	}
	fields = append(fields, run.Note)
	for index, field := range fields {
		fields[index] = strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(field)
	}
	return strings.Join(fields, "\t")
}

// Parse reads one run line.
func Parse(line string) (Run, error) {
	assert.That(len(line) <= LineLengthBytesMax, "a line fits LineLengthBytesMax")
	assert.That(!strings.Contains(line, "\n"), "a line is one line")
	fields := strings.Split(line, "\t")
	if len(fields) < 5 || len(fields) > 5+FieldsMax {
		return Run{}, fmt.Errorf("%w: %d fields", ErrBadLine, len(fields))
	}
	at, err := time.Parse(time.RFC3339, fields[0])
	if err != nil {
		return Run{}, fmt.Errorf("%w: %v", ErrBadLine, err)
	}
	run := Run{
		Time: at, Key: fields[1], Result: fields[2], Author: fields[3],
		Note: fields[len(fields)-1],
	}
	for _, field := range fields[4 : len(fields)-1] {
		name, value, ok := strings.Cut(field, "=")
		if !ok {
			return Run{}, fmt.Errorf("%w: %q is neither a row value nor a fingerprint part",
				ErrBadLine, field)
		}
		if part, isPart := strings.CutPrefix(name, fingerprintPrefix); isPart {
			run.Fingerprint = append(run.Fingerprint, Pair{Name: part, Value: value})
		} else {
			run.Values = append(run.Values, Pair{Name: name, Value: value})
		}
	}
	return run, nil
}

// Read reads every run in runs/, oldest file first and in file order within a day. No runs/
// folder means no runs. A line that can't be read is skipped: one bad merge shouldn't hide
// every run.
func Read(capabilities string) ([]Run, error) {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	// Glob returns names in lexical order, which for YYYY-MM-DD is date order.
	// Ref: https://pkg.go.dev/path/filepath#Glob
	paths, err := filepath.Glob(filepath.Join(Dir(capabilities), "*.log"))
	if err != nil {
		return nil, err
	}
	if len(paths) > RunFilesMax {
		return nil, fmt.Errorf("runs/ holds %d files, more than %d", len(paths), RunFilesMax)
	}
	all := make([]Run, 0, len(paths)*4)
	for _, path := range paths {
		found, err := readFile(path)
		if err != nil {
			return nil, err
		}
		all = append(all, found...)
	}
	assert.That(len(paths) <= RunFilesMax, "files read stay bounded")
	return all, nil
}

// readFile reads one day's runs.
func readFile(path string) ([]Run, error) {
	assert.That(strings.HasSuffix(path, ".log"), "path is a run file")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	// Scanner splits on lines; Buffer raises its limit to a whole run line.
	// Ref: https://pkg.go.dev/bufio#Scanner.Buffer
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4096), LineLengthBytesMax+1)
	found := make([]Run, 0, 16)
	// bound: Scan stops at the end of the file, which Read's callers keep to one day of runs.
	for scanner.Scan() {
		if run, err := Parse(scanner.Text()); err == nil {
			found = append(found, run)
		}
	}
	if err := scanner.Err(); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	assert.That(scanner.Err() == nil, "the file was read to its end")
	return found, file.Close()
}

// Latest returns the newest run for key whose row values are values, and false when there's
// none. Runs are compared by time, so a merged file's order doesn't matter.
func Latest(all []Run, key string, values []Pair) (Run, bool) {
	assert.That(key != "", "a key names a scenario")
	assert.That(len(values) <= FieldsMax, "values stay bounded")
	var latest Run
	found := false
	for _, run := range all {
		if run.Key != key || !samePairs(run.Values, values) {
			continue
		}
		if !found || !run.Time.Before(latest.Time) {
			latest, found = run, true
		}
	}
	return latest, found
}

// samePairs is true when a and b hold the same pairs in the same order.
func samePairs(a []Pair, b []Pair) bool {
	assert.That(len(a) <= FieldsMax, "a stays bounded")
	assert.That(len(b) <= FieldsMax, "b stays bounded")
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}
