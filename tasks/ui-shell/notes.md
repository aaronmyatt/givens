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

## Rule
- Take `@wip` off a scenario only when both the Go suite and the Swift suite
  (`swift test --package-path macos`) pass it. (Was a todo line; it's a standing rule.)

## As built
- `givens serve --app-data <folder>` (2026-10-11): `internal/serve` answers every method in
  ServeProtocol.swift over JSON lines. Each reply carries `App.Snapshot()`; errors carry a code:
  `not_built`, `not_found`, `bad_request`, `unknown_method`, `no_test_seams`, `failed`.
  - Requests are checked before they reach internal/app, so a bad request is an error reply,
    never a failed assertion.
  - The core's clipboard and Trash are a recorder: writes become `effects` for the app to do.
    Known gap: the core can't see a Trash move until the app does it, so a delete followed by a
    reload in the same call still sees the file. Write stories' delete scenarios settle it.
  - Methods in `macos/Sources/GivensUI/Needs/*.swift` aren't in the table yet; each
    capability's "Wire … into the UI shell" line adds its own.
- Snapshot types (`internal/app/snapshot.go`) carry Records.swift's JSON keys, and lists are
  never null. `Window.Visible()` is built from the snapshot in the same order as
  `WindowModel.visible`, so Go's "they see" and Swift's read the same text.
- Spike: `macos/Tests/GivensAcceptanceTests/ServeTests.swift` opens the sample shop through a
  real serve and checks the window lists Checkout, Invites and Sign in. No screenshot: the real
  app can't open a folder yet (Launch and "Open folder…" are Open a project's).
- Pushed snapshots (2026-10-11): every 250 ms serve builds the snapshot and sends
  `{"event":"snapshot",…}` when it differs from the last one the app got. That covers files
  another tool wrote and, once Drop in exists, setup output, with no per-cause plumbing.
  - Windows now watch their folder: every read (`Sidebar`, `Capability`, `Scenario`, and so
    `Snapshot`) calls `refresh`, which reloads when `internal/watch` sees a change.
  - A mutex in Server keeps the request loop and the pusher off the app at the same time.