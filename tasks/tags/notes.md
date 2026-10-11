# Tags

## Goal
Make the 8 scenarios in `capabilities/tags/tags.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/tags/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/tags (Cucumber tag expressions), internal/search.
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
- All 8 (2026-10-11), Go and Swift. Search everything's 6 pass with them: ⌘K search was
  built once for both. See tasks/search-everything/notes.md.
- Views: sidebar lists Error states, Unverified and Shared under the capabilities, then one
  view per tag (any tag but @error and @timeout:…), each with a count. Selecting one fills
  `ListedScenarios`, now also in the snapshot as `listed`, in both visible lists.
- Search: the box's text splits into a tag expression (tags, and, or, not, brackets) and
  words. A tag expression narrows to Scenarios; a broken one shows "Not a tag expression:
  expected a tag after 'and'" and no results.
- UI: sidebar sections (Capabilities, Views, Tags), the listed scenarios, and a `.searchable`
  field that sends ⌘K then the whole query on each change. Not eyeballed.
- Known cost: the Unverified count fingerprints every scenario on each snapshot. Fine for the
  sample shop; a big project may need statuses cached. Added to todo.txt.