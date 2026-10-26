---
name: givens
description: Read, run and check this project's Gherkin scenarios in capabilities/ with the givens CLI. Use when working on a feature, a scenario, a setup script, or anything tagged @todo.
---

# Givens

This project keeps its behaviour as Gherkin scenarios in `capabilities/`, one `.feature` file
per capability, each with setup scripts that put the app into a scenario's starting state.

## Start here

1. Read `capabilities/GIVENS.md`. It says how the folder is laid out and what each tag means.
2. Run `givens list` to see every scenario as `<capability>/<scenario>`, with its kind and
   status. `givens list --json` prints the same as JSON; `givens list --tags @todo` lists the
   ones that need work.

## Commands

Use these, not the TUI (the full-screen `givens` with no arguments): they print plain text.

- `givens list [--json] [--tags <expression>]`: the scenarios.
- `givens run <capability>/<scenario>`: run a scenario's setup.
- `givens check`: check every file; exits 1 and prints each problem if there are any.
- `givens todo <capability>/<scenario>`: flag a scenario as needing work (`@todo`).
- `givens done <capability>/<scenario>`: take the flag off.

## Rules

- Run `givens check` after editing a `.feature` file or a setup script.
- Flag a scenario with `givens todo` rather than writing the tag by hand.
- Setup scripts should be idempotent: running one twice leaves the same state as once.
