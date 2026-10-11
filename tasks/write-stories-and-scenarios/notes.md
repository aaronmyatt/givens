# Write stories and scenarios

## Goal
Make the 20 scenarios in `capabilities/write-stories-and-scenarios/write-stories-and-scenarios.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/write-stories-and-scenarios/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/edit (minimal-diff writes to .feature files, renames, deletes to Trash).
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built

## Choices the steps already make
- "Keywords lined up" means keywords right-aligned to the widest one: `  And the order stays pending`.
- "A developer is editing X" opens X and appends " until the bank replies" to its last step, unsaved.
- "and confirms" chooses "Rename" only when it's offered; "they confirm" chooses "Delete".
- The preview's parser message is exactly "Expected a step, a tag, a comment or a new scenario".
- "An empty numbered list" means the only non-blank line is `1.`.
