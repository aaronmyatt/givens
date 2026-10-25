package main

import (
	"fmt"
	"slices"
	"strings"

	"givens/internal/app"
	"givens/internal/assert"
)

// rename, delete, todo and done: the app's own calls, run from a terminal. A question the app
// would ask, like "3-D Secure challenge builds on this scenario", is printed instead, and
// --yes answers it. See capabilities/work-in-the-terminal.

// runRename renames a scenario or a capability.
func runRename(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	yes, args := withoutYes(args)
	if len(args) != 2 || args[0] == "" || args[1] == "" {
		fmt.Println("usage: givens rename <capability>/<scenario> | <capability> <name> [--yes]")
		return exitUsage
	}
	return onWindow(func(window *app.Window) error {
		return window.Rename(args[0], args[1])
	}, yes, "Renamed "+args[0]+" to "+args[1])
}

// runDelete deletes a scenario, its setup files going to the Trash, or a capability.
func runDelete(args []string) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	yes, args := withoutYes(args)
	if len(args) != 1 || args[0] == "" {
		fmt.Println("usage: givens delete <capability>/<scenario> | <capability> [--yes]")
		return exitUsage
	}
	return onWindow(func(window *app.Window) error {
		return window.Delete(args[0])
	}, yes, "Deleted "+args[0])
}

// runFlag flags a scenario to do, or takes the flag off.
func runFlag(args []string, flag bool) int {
	assert.That(args != nil, "args is a list")
	assert.That(len(args) < 1<<16, "a command line stays bounded")
	if len(args) != 1 || args[0] == "" {
		fmt.Println("usage: givens todo <capability>/<scenario> | givens done <…>")
		return exitUsage
	}
	action, done := app.OfferRemoveTodo, "Took the to do flag off "+args[0]
	if flag {
		action, done = app.OfferFlagTodo, "Flagged "+args[0]+" to do"
	}
	return onWindow(func(window *app.Window) error {
		if err := window.Select(args[0]); err != nil {
			return err
		}
		if !slices.Contains(window.Offers(), action) {
			if flag {
				return fmt.Errorf("%s is already flagged to do", args[0])
			}
			return fmt.Errorf("%s isn't flagged to do", args[0])
		}
		return window.Choose(action)
	}, false, done)
}

// withoutYes takes --yes out of args.
func withoutYes(args []string) (bool, []string) {
	assert.That(args != nil, "args is a list")
	kept := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "--yes" {
			kept = append(kept, arg)
		}
	}
	assert.That(len(kept) <= len(args), "only --yes is taken out")
	return len(kept) < len(args), kept
}

// onWindow runs call in the project's window. When it leaves a question up, the question is
// printed and, with yes, answered with its first choice; without, nothing changes. done is
// printed once it has.
func onWindow(call func(*app.Window) error, yes bool, done string) int {
	assert.That(call != nil, "there's something to run")
	assert.That(done != "", "there's something to say once it's done")
	_, window, err := openProject()
	if err != nil {
		complain(err)
		return exitProblems
	}
	if err := call(window); err != nil {
		complain(err)
		return exitProblems
	}
	if question, confirm := window.Question(); question != "" {
		fmt.Println(question)
		if !yes {
			fmt.Printf("Nothing changed. Run it again with --yes to %s.\n",
				strings.ToLower(confirm))
			return exitProblems
		}
		if err := window.Choose(confirm); err != nil {
			complain(err)
			return exitProblems
		}
	}
	fmt.Println(done)
	return exitOK
}
