# GIVENS.md

How this capabilities folder is laid out. Givens, test runners, people and agents all read the
same files, so these rules keep them agreeing.

## Layout

```text
capabilities/
  GIVENS.md                    # this file
  .gitattributes               # runs/** merge=union
  _shared/givens.sh            # the kit, sourced by every setup script
  _shared/*.sh                 # helpers every capability can call
  <capability>/
    <capability>.feature       # strict Gherkin: the story and every scenario
    about.md                   # optional: diagrams, images, context
    scripts/<slug>.sh          # setup that runs commands
    scripts/<slug>.teardown.sh # optional: runs before each setup
    scripts/<slug>.md          # setup done only by hand, shown as cards
    _shared/*.sh               # helpers only this capability's scripts can call
  runs/YYYY-MM-DD.log          # one line per recorded Drop in
```

## Rules

- A capability is a folder holding exactly one `.feature` file, named after the folder.
  A folder with no `.feature` file isn't a capability; Givens leaves it alone.
- The Feature's description holds the story: an `As a`, an `I want` and a `So that` line.
  Descriptions are plain text; a line starting with `#` is a Gherkin comment.
- Tags: `@error` marks an error state (untagged is a happy path). `@timeout:5m` sets when to
  show "Still running" (default 2 minutes). Any other tag is a view. A tag on `Feature:`
  applies to every scenario in the file.
- A slug is a name in lower case; letters and digits in any language are kept; every run of
  other characters becomes one hyphen; hyphens are trimmed from the ends.
  `Card declined` gives `card-declined`.
- Scenario names must make different slugs within a file, `Rule:` blocks included.
- A scenario's setup is `scripts/<slug>.sh`, `scripts/<slug>.md`, or both (script first).
  Setup should be idempotent: running it twice leaves the same state as once.
- A run is recorded against `<capability>/<scenario slug>`, plus `/<examples slug>` for a named
  Examples block. Renaming a scenario starts its runs over.
- Step definitions belong to the project's test runner, not to Givens. They may live in the
  capability folders (e.g. `checkout/steps/`).

## Glossary

| Term | Meaning |
| --- | --- |
| Capability | One thing the app does. A folder holding one `.feature` file. |
| Story | The As a, I want and So that lines in the Feature's description. |
| Scenario | One behaviour, written as Given, When and Then steps. |
| Happy path / error state | A scenario where things go as hoped / where something goes wrong (`@error`). |
| Outline, row | A `Scenario Outline:` runs once per row of its Examples table. |
| Slug | A name in lower case with hyphens. Names files and run keys. |
| Setup, teardown | Scripts that put the app into, or clear it out of, a scenario's starting state. |
| Helper | A shell function in a `_shared/` folder that setup scripts call. |
| Kit | `_shared/givens.sh`, sourced by every setup script. |
| Drop in | Run a scenario's setup, then walk its steps as a checklist. |
| Run, verified | A recorded Drop in; verified while its latest run passed and its setup is unchanged. |
| Fingerprint | A code worked out from a scenario's steps, setup and helpers. A change unverifies it. |

## This repository

These capabilities are Givens's own spec. Every scenario starts tagged `@wip`; the acceptance
suite in `acceptance/` skips `@wip` and runs the rest strictly. Removing `@wip` from a scenario
is how the backlog in `todo.txt` marks it done.
