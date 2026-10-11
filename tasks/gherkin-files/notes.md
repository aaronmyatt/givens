# Gherkin files

## Goal
Make the 7 scenarios in `capabilities/gherkin-files/gherkin-files.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/gherkin-files/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/spec (parse, check), internal/project (load a folder).
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
- Runner (2026-10-11): testdata/sample-shop has a package.json pinning @cucumber/cucumber
  13.2.1 (13.3.0 was under two weeks old) and its package-lock.json. `scripts/runner.sh`
  (check.sh's `runner` step) runs `npm ci --ignore-scripts` into gitignored `.cache/runner`
  once; both Worlds link its node_modules into each sample-shop copy, so npx never fetches.
  - Known gap: a fresh clone needs the network, or a warm npm cache, for that first install.
- A test runner reads the same files; Step definitions are left alone: pass in Go and Swift
  with no core change beyond Foundation (folders without `<name>.feature` were never
  capabilities).
- Rules group scenarios: `project.Scenario.Rule`; `Window.Select` (capability or scenario,
  exact name before slug) and `RuleGroups`.
- Comments are kept: the editor holds the whole file and `Save` writes it as typed, so git diff
  shows only the edited line. Save refuses with "This file changed on disk" when the file moved
  under it (Write stories' scenario for that still has to pass its own steps).
- A file that isn't valid Gherkin: `spec.Explain` turns the parser's token list into words
  ("expected a step, a tag, a comment or a new scenario"); the capability keeps its Feature:
  name, read from the text, and offers "Edit checkout.feature".
- Same slug: both scenarios warn; their page offers only "Copy". Scenario `Rename` landed
  here: rewrites the Scenario: line, moves setup files and `from` lines unless the other
  scenario still makes the old slug, and asks first ("Renaming starts X's runs over") when runs
  exist. Capability rename is still Write stories'.
- Two .feature files: `project.Capability.Ignored`; warning names the first; "Move to its own
  capability" moves it to `<slug>/<slug>.feature`.
- Fixes for broken capabilities are offered with nothing selected: the sidebar warning carries
  them. The selected capability's come first.
- Prompts (`ask`/`answer`): one question at a time, its message across the top and only its
  two buttons offered. Rename uses it; Delete and Trust will.
- UI: RootView draws a warning icon in the sidebar, warnings on pages, Rule: headings over
  their scenarios, and an editor pane with Save (⌘S). Both visible lists include the Rule:
  headings. Known gap: not looked at on screen yet; the app can't open a folder until Open a
  project builds Launch and "Open folder…".