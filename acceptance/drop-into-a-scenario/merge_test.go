package dropintoascenario

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// unionMerge is the line GIVENS.md gives a capabilities folder's .gitattributes, so git keeps
// both sides' run lines. Ref: https://git-scm.com/docs/gitattributes#_built_in_merge_drivers
const unionMerge = "runs/** merge=union\n"

// runsOnTwoBranches records a passing run as Sam on branch sam, and as Ali on branch ali, both
// branched from main, through Givens each time.
func (s *state) runsOnTwoBranches(name string) error {
	if err := s.writeSetup(name, quickSetup()); err != nil {
		return err
	}
	if err := s.ensureUnionMerge(); err != nil {
		return err
	}
	if err := s.gitInit("Sam"); err != nil {
		return err
	}
	if err := s.recordOnBranch(name, "sam", "Sam"); err != nil {
		return err
	}
	if _, err := s.git("checkout", "--quiet", "main"); err != nil {
		return err
	}
	// A later run on the same day, from a fresh Givens, as on Ali's machine.
	s.Clock.Advance(time.Hour)
	s.App, s.Window, s.Session = nil, nil, nil
	return s.recordOnBranch(name, "ali", "Ali")
}

// ensureUnionMerge gives the sample shop's capabilities folder the .gitattributes line a new
// capabilities folder gets, if the fixture lacks it.
func (s *state) ensureUnionMerge() error {
	path := filepath.Join("capabilities", ".gitattributes")
	existing, err := s.ReadFile(path)
	if err == nil && strings.Contains(existing, strings.TrimSpace(unionMerge)) {
		return nil
	}
	return s.WriteFile(path, existing+unionMerge)
}

// recordOnBranch starts branch, records a passing run as user, and commits it.
// Ref: https://git-scm.com/docs/git-checkout and https://git-scm.com/docs/git-commit
func (s *state) recordOnBranch(name string, branch string, user string) error {
	if _, err := s.git("checkout", "--quiet", "-b", branch); err != nil {
		return err
	}
	if _, err := s.git("config", "user.name", user); err != nil {
		return err
	}
	if err := s.droppedInAllPassed(name); err != nil {
		return err
	}
	session, err := s.session()
	if err != nil {
		return err
	}
	if err := session.RecordRun(""); err != nil {
		return err
	}
	if _, err := s.git("add", "--all"); err != nil {
		return err
	}
	_, err = s.git("commit", "--quiet", "--message", user+"'s run")
	return err
}

// mergeBranches merges ali into sam, keeping git's output and exit code for the Then steps.
// Ref: https://git-scm.com/docs/git-merge
func (s *state) mergeBranches() error {
	if _, err := s.git("checkout", "--quiet", "sam"); err != nil {
		return err
	}
	output, err := s.git("merge", "--no-edit", "ali")
	s.mergeOutput = output
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		s.mergeExit = exit.ExitCode()
		return nil
	}
	return err
}

func (s *state) mergedCleanly() error {
	if s.mergeExit != 0 || strings.Contains(s.mergeOutput, "CONFLICT") {
		return fmt.Errorf("git merge exited %d:\n%s", s.mergeExit, s.mergeOutput)
	}
	return nil
}

func (s *state) bothLinesKept() error {
	lines, err := s.runLines(s.runKey)
	if err != nil {
		return err
	}
	for _, user := range []string{"Sam", "Ali"} {
		found := false
		for _, fields := range lines {
			found = found || hasField(fields, user)
		}
		if !found {
			return fmt.Errorf("after the merge, no run line for %s is by %s: %q", s.runKey,
				user, lines)
		}
	}
	if len(lines) != 2 {
		return fmt.Errorf("after the merge there are %d run lines, not 2: %q", len(lines), lines)
	}
	return nil
}
