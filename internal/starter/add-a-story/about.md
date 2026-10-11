# Add a story

This folder is a worked example of the format Givens reads. Each piece:

- `add-a-story.feature`: the story (As a / I want / So that) and one scenario per behaviour, in
  standard Gherkin, so any Gherkin test runner reads it too.
- `about.md`: this file. Markdown about the capability as a whole: diagrams, screenshots,
  context. Runners never read it.
- `scripts/<slug>.sh`: the setup for one scenario, named by the slug of its name. It sources
  the kit, `../../_shared/givens.sh`, on its first line.
- `scripts/<slug>.md`: setup done only by hand, as a numbered list. Drop in shows each item as
  a card.

The rest of the conventions are in `../GIVENS.md`.
