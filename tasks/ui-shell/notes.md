# macOS UI shell

## Goal
Pick what draws the windows. Every scenario passes through `internal/app` first, so this only
decides how that core is shown.

## Options
- A: Wails v3 (Go + a web view). One language, so tigerlint covers everything. Multi-window
  support is still in alpha; Dock drop, window restore and Trash need native glue.
  https://v3alpha.wails.io/
- B: SwiftUI app shell over the Go core, talking to a `givens serve` process over stdio
  (JSON lines). Window restore, recent projects, Dock drop and Trash come from AppKit for free;
  costs a second language outside tigerlint.
  https://developer.apple.com/documentation/swiftui/documentgroup

I'd take B: Open a project leans on behaviours AppKit already has, and the rules that matter
most stay in Go.

## Decided (2026-10-11): B, SwiftUI over `givens serve`
- Open a project leans on what AppKit already does: window restore, recent projects, Dock
  drop, a folder as a document, and Trash for deletes. Wails would need native glue for each,
  and its multi-window support is still alpha.
- The rules that matter stay in Go. The Swift side only draws `internal/app`'s snapshots and
  sends commands, so tigerlint still covers the core.
- The scaffold is already B: `macos/` builds, and every Swift suite's steps are defined.
- Cost: a second language outside tigerlint. swift-format and the Swift Gherkin suites in
  `scripts/check.sh` cover it instead.
- Wails would win if Givens had to run on Linux or Windows. It doesn't; it's a macOS app.

## Scaffold (2026-10-11)
Took B to scaffold the Swift side.
- `macos/`: SwiftPM package. `GivensUI` (models, serve client, AppKit services), `Givens` (the
  SwiftUI app), `GherkinKit` (a small strict runner), and `GivensAcceptanceTests`.
- The Swift suites run the same `.feature` files, compiled by `tools/pickles` with the Go
  parser, so godog and Swift never disagree on what a scenario is.
- Steps drive `AppModel`/`WindowModel`/`SessionModel` over a real `givens serve`; every
  scenario fails with "givens serve isn't available" until serve is built.
- `WindowModel.visible` is built from the same snapshot fields the views render, so "they see"
  checks the Swift side, not just the core.
- `the-kit` and `use-givens-from-the-terminal` have no Swift suite: no window is involved.
  `CoverageTests` fails if any other capability lacks one.
