package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"givens/internal/app"
	"givens/internal/assert"
	"givens/internal/termedit"
)

// The app's other panels as plain commands: Shared, a scenario's runs, ⌘K search, trust and
// sharing runs. --json prints what the app's snapshot holds, for scripts and agents.
// See capabilities/work-in-the-terminal.

// runHelpers lists the project's helpers: where each is defined and how many scripts call it.
func runHelpers(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	asJSON, args := withFlag(args, "--json")
	if len(args) != 0 {
		fmt.Println("usage: givens helpers [--json]")
		return exitUsage
	}
	return onProject(func(window *app.Window) error {
		helpers := window.Helpers()
		if asJSON {
			return printJSON(helpers)
		}
		for _, helper := range helpers {
			fmt.Printf("%s\t%s:%d\tcalled by %d %s\n", helper.Name, helper.DefinedIn,
				helper.Line, len(helper.Callers), plural(len(helper.Callers), "script"))
		}
		return nil
	})
}

// runHelper makes a new helper and opens it in the editor, or opens one: givens helper new
// <capability or _shared> <name>, givens helper edit <name>.
func runHelper(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	usageOK := (len(args) == 3 && args[0] == "new") || (len(args) == 2 && args[0] == "edit")
	if !usageOK {
		fmt.Println("usage: givens helper new <capability|_shared> <name> | " +
			"givens helper edit <name>")
		return exitUsage
	}
	name := args[len(args)-1]
	return onProject(func(window *app.Window) error {
		if args[0] == "new" {
			capability := args[1]
			if capability == "_shared" {
				capability = ""
			}
			if err := window.NewHelper(capability, name); err != nil {
				return err
			}
		}
		for _, helper := range window.Helpers() {
			if helper.Name == name {
				return editFile(window, helper.DefinedIn)
			}
		}
		return fmt.Errorf("there's no helper called %s", name)
	})
}

// editFile edits one file under capabilities/ in the developer's editor, and says how it went.
func editFile(window *app.Window, file string) error {
	assert.That(window != nil, "a file is edited in a window")
	assert.That(file != "", "a file is named")
	editor, err := window.Edit(file)
	if err != nil {
		return err
	}
	outcome, edit, err := termedit.Run(editor)
	if err != nil {
		return err
	}
	if reportEdit(outcome, edit, file) != exitOK {
		return errors.New("nothing was saved")
	}
	return nil
}

// runHistory lists a scenario's runs, newest first.
func runHistory(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	asJSON, args := withFlag(args, "--json")
	if len(args) != 1 {
		fmt.Println("usage: givens history <capability>/<scenario> [--json]")
		return exitUsage
	}
	return onProject(func(window *app.Window) error {
		if err := window.Select(args[0]); err != nil {
			return err
		}
		lines := window.Snapshot().Runs
		if asJSON {
			return printJSON(lines)
		}
		if len(lines) == 0 {
			fmt.Println("Never run")
		}
		for _, line := range lines {
			fmt.Println(strings.TrimSpace(line.Text + "  " + line.Note))
		}
		return nil
	})
}

// runSearch searches everything, as ⌘K does: stories, scenarios, steps, scripts, helpers.
func runSearch(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	asJSON, args := withFlag(args, "--json")
	kind := ""
	if len(args) >= 2 && args[0] == "--kind" {
		kind, args = args[1], args[2:]
	}
	if len(args) == 0 {
		fmt.Println("usage: givens search [--kind Stories|Scenarios|Steps|Scripts|Helpers] " +
			"<words> [--json]")
		return exitUsage
	}
	return onProject(func(window *app.Window) error {
		if err := window.Press("⌘K"); err != nil {
			return err
		}
		if err := window.Type(strings.Join(args, " ")); err != nil {
			return err
		}
		groups := make([]app.SearchGroup, 0, 5)
		for _, group := range window.SearchResults().Groups {
			if kind == "" || strings.EqualFold(kind, group.Kind) {
				groups = append(groups, group)
			}
		}
		if asJSON {
			return printJSON(groups)
		}
		for _, group := range groups {
			for _, result := range group.Results {
				fmt.Println(strings.TrimRight(group.Kind+"\t"+result.Name+"\t"+result.Detail,
					"\t"))
			}
		}
		return nil
	})
}

// runTrust trusts the project, as choosing "Trust" does, so its setups run.
func runTrust(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(appDataEnv != "", "trust is kept in Givens's data")
	if len(args) != 0 {
		fmt.Println("usage: givens trust")
		return exitUsage
	}
	core, window, err := openProject()
	if err != nil {
		complain(err)
		return exitProblems
	}
	if err := core.MarkTrusted(window.Path()); err != nil {
		complain(err)
		return exitProblems
	}
	fmt.Println("Trusted " + window.Path())
	return exitOK
}

// runSync shares runs kept off main and fetches other devices', as the app does every few
// minutes, and says how many wait.
func runSync(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	if len(args) != 0 {
		fmt.Println("usage: givens sync")
		return exitUsage
	}
	return onProject(func(window *app.Window) error {
		if !window.KeepsRunsOffMain() {
			fmt.Println("Runs here go in capabilities/runs: there's no git remote to share " +
				"them through")
			return nil
		}
		if err := window.SyncRuns(); err != nil {
			return err
		}
		for _, message := range window.Snapshot().Messages {
			if strings.Contains(message, "not shared yet") {
				fmt.Println(message)
				return nil
			}
		}
		fmt.Println("Runs are shared")
		return nil
	})
}

// onProject runs call in the project's window, printing an error and exiting 1 if it fails.
func onProject(call func(*app.Window) error) int {
	assert.That(call != nil, "there's something to run")
	assert.That(effectTimeout > 0, "the command's effects have a limit")
	_, window, err := openProject()
	if err == nil {
		err = call(window)
	}
	if err != nil {
		complain(err)
		return exitProblems
	}
	return exitOK
}

// withFlag takes flag out of args, saying whether it was there.
func withFlag(args []string, flag string) (bool, []string) {
	assert.That(args != nil, "args is a list")
	assert.That(strings.HasPrefix(flag, "--"), "a flag starts with --")
	kept := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != flag {
			kept = append(kept, arg)
		}
	}
	return len(kept) < len(args), kept
}

// printJSON prints value as indented JSON. Ref: https://pkg.go.dev/encoding/json#MarshalIndent
func printJSON(value any) error {
	assert.That(value != nil, "there's a value to print")
	text, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	assert.That(len(text) > 0, "JSON has text")
	fmt.Println(string(text))
	return nil
}

// plural is noun for one, or noun+s.
func plural(count int, noun string) string {
	assert.That(count >= 0, "a count isn't negative")
	assert.That(noun != "", "a noun is given")
	if count == 1 {
		return noun
	}
	return noun + "s"
}
