# Mix shell, browser and hand steps

Asked 2026-10-19: extend Givens and its kit with chromedp, so one scenario's setup can mix
shell, JavaScript run in the browser, and steps done by hand. The developer's own words:
"drive my apps via App Action scripts".

## Goal
- Make the 19 scenarios in `capabilities/mix-shell-browser-and-hand-steps/` pass: each loses
  `@wip`, its Go steps pass against a real Chrome, its Swift steps pass, `scripts/check.sh`
  passes.
- The kit gains `browser_open <url>`, `in_browser <js>` and `app_action <name> [json]`.
- Setup cards in `scripts/<slug>.md` run a code block whose info string is `bash run` or
  `js run` by themselves. Other cards wait, as now.
- The Givens browser stays open after setup, so the developer does the When steps in it.

## Design
- CDP through chromedp, behind a new `givens browser` command. The kit is bash 3.2 and
  can't speak WebSocket, so it shells out to `"${GIVENS_BIN:-givens}" browser …`. Drop in sets
  `GIVENS_BIN` to the app's own givens; a terminal finds it on PATH, checked by `need givens`.
  - Ref: https://chromedevtools.github.io/devtools-protocol/
  - Ref: https://pkg.go.dev/github.com/chromedp/chromedp
- One Chrome per project, with a profile of Givens's own: `<app data>/browsers/<project key>/`.
  Started with `--remote-debugging-port=0 --user-data-dir=<that folder>`. Chrome writes the
  port it picked to `DevToolsActivePort` in that folder, so the next call (from a terminal or
  from Drop in) finds the same browser and attaches with `chromedp.NewRemoteAllocator`. If
  the port is dead, start Chrome again.
  - Chrome 136+ ignores the debug port on the default profile, which also keeps the
    developer's own Chrome and sign-ins out of reach.
  - Ref: https://developer.chrome.com/blog/remote-debugging-port
  - Ref: https://pkg.go.dev/github.com/chromedp/chromedp#NewRemoteAllocator
- One page per project browser. `browser_open` goes to the URL and waits for load. The tab's
  target id goes in a file next to `DevToolsActivePort`; `in_browser` and `app_action` run
  in that tab.
  - Ref: https://pkg.go.dev/github.com/chromedp/chromedp#WithTargetID
- `in_browser` is `Runtime.evaluate` with `awaitPromise` and `returnByValue`. A string is
  printed as is and anything else as JSON, so `X="$(in_browser …)"` works. A thrown error
  prints `✗ in_browser: <message>` and exits 1.
  - Ref: https://chromedevtools.github.io/devtools-protocol/tot/Runtime/#method-evaluate
- `app_action name json` sends `window.app[name](json)`. First it checks `typeof
  window.app?.[name] === "function"`, so a missing action has its own message.
  - Ref: https://www.cypress.io/blog/stop-using-page-objects-and-start-using-app-actions
- Limits, named as constants: `browserEvalSecondsMax = 30`, `browserOpenSecondsMax = 30`,
  `browserResultBytesMax = 1 MiB`. The 2-minute "Still running" check is separate and still
  applies to the whole of setup.
- Pre-run check: a script, or a helper it calls, that has a browser line needs `givens` and
  Chrome. It's caught the same way as `need`: "This scenario needs Chrome". The hint is
  `brew install --cask google-chrome`. `GIVENS_CHROME` can point at another Chromium-based
  browser (Chromium, Brave, Edge).
- `by_hand --url` opens in the Givens browser when this project's browser is running, so a
  hand step shares the session setup signed in to. Otherwise it opens the default browser,
  as now.
- Run blocks in cards: the markdown parser (internal/markdown) gives each fenced block its
  info string. A card that has a `bash run` or `js run` block runs it when Drop in reaches
  that card. Bash runs as a temp script that sources the kit first, with the scenario's env.
  A card with a run block is marked done when every run block in it exits with 0.
  - Ref: https://spec.commonmark.org/0.31.2/#info-string
- Trust: a .md with any run block asks for trust, the same as a .sh. Plain cards still skip
  the trust prompt.
- Kit goes to version 2 (`# givens kit 2`). Update the kit already handles the bump, and
  updating the kit doesn't unverify anything.

## Rejected
- Raw CDP over the standard library: Go's standard library has no WebSocket, and the
  developer asked for chromedp.
- An in-page WebSocket bridge (a dev-only script in the app connects to a Givens daemon). It
  works in any browser, but every app would have to ship and guard that script. CDP needs
  only `window.app`.
- Firefox and Safari: CDP is Chromium only. WebDriver BiDi (the cross-browser W3C protocol)
  could be added later behind the same kit functions.
  - Ref: https://w3c.github.io/webdriver-bidi/
- Running JavaScript in the developer's own Chrome: it needs a debug port on their profile,
  which Chrome 136+ refuses, and it would put their sign-ins within setup's reach.
- Automating When or Then steps from the checklist: out of scope. Step definitions belong
  to the project's test runner (GIVENS.md). This work is setup only.

## Spec
- 19 scenarios in `capabilities/mix-shell-browser-and-hand-steps/`. 8 cover a setup script,
  3 the browser staying open, 4 setup cards, 4 the terminal.
- `macos/Tests/GivensAcceptanceTests/CoverageTests.swift` lists the capability as windowless
  until its Swift suite exists. The Mac task removes that line.

## Testing
- The Go world serves a fake shop page (`httptest`) with `window.app.signIn`, `addToCart`
  (settles after a delay), `lastOrderId` and nothing called `refund`.
  - Ref: https://pkg.go.dev/net/http/httptest
- Chrome runs headless in tests, with a temp profile, through the same `internal/browser`
  code. No Chrome means the suite fails with the install hint. It isn't skipped.
- "No Chrome on this Mac" sets `GIVENS_CHROME` to a path that doesn't exist.

## Deps (2026-10-24)
- chromedp v0.21.0 and cdproto v0.158.4, on the allowlist in scripts/check.sh with why.
  chromedp's only other dependency is go-json-experiment (indirect).
- v0.21 has a new generic API: `chromedp.Run(ctx, action)` returns the action's value, and
  `chromedp.Evaluate[T](js, opts...)`. Older examples (`Run(ctx, a, b, c)` with pointers)
  don't compile. Ref: https://pkg.go.dev/github.com/chromedp/chromedp@v0.21.0
- It has what the design needs built in: `chromedp.KeepOpen` starts Chrome detached, with
  `--remote-debugging-port=0`, reads `DevToolsActivePort`, and leaves it running when the Go
  program ends; `remote.NewAllocator(ctx, wsURL)` attaches to it later. So internal/browser
  needn't start Chrome by hand. KeepOpen needs `remote.WebSocket` as the dialer.
- Probe (a scratch main): 5.36 MB with chromedp, against 5.31 MB for a main using net/http,
  encoding/json and os/exec. Givens is 6.9 MB today; the real growth is measured once
  internal/browser links it (below).
- Chrome's start: 6.0 s the first time with a new profile, 0.43 s after; an evaluate is
  about 2 ms. So the first browser line of a project's first Drop in takes seconds; the 30 s
  limits leave room.

## internal/browser (2026-10-24)
- `Profile(capabilities)`: `<user cache>/Givens/browsers/<hash>`, or under `GIVENS_BROWSERS`.
  Not `<app data>/browsers` as planned: the givens command in a terminal doesn't know the
  app's data folder, and a terminal run and a Drop in must share one browser. The path is
  hashed after `EvalSymlinks`, so /var and /private/var give one profile.
- Finding Chrome again: the port in `DevToolsActivePort`, then the DevTools HTTP endpoints
  (`/json/version` for the websocket, `/json/list` for tabs, `PUT /json/new` when the
  developer closed every window). A port file whose Chrome is gone is `ErrNotRunning`.
- The project's page: its target id in `givens-page`. Each call attaches with
  `WithTargetID` and `WithDetachOnCancel`; without the latter chromedp closes the tab on
  cancel.
- `Close` sends `Browser.close` on a connection of its own (`remote.DialContext`): chromedp
  refuses that command and wants `chromedp.Cancel`, which doesn't close a kept-open browser.
- `Eval`: `Evaluate[[]byte]` with `EvalAwaitPromise`; a string prints as is, anything else as
  compact JSON, undefined as "". A throw gives the first line of its description, the
  `TypeError: ` class cut off. A deadline is `ErrNoAnswer` ("no answer after 30s").
- `Action`: the name must be a plain JS name, the argument JSON; `typeof window.app?.name`
  is checked first, so a missing one is `ErrNoAction` ("the page has no window.app.refund").
- `GIVENS_CHROME` names another browser; `GIVENS_HEADLESS=1` for the suites.
- Unit tests start headless Chrome against an httptest page: open, evaluate (string, JSON,
  undefined, promise), one page across calls, a throw, a missing action, a promise that never
  settles (1 s limit), symlinked profiles. Each test's Chrome is closed after; checked none
  are left running.

## CLI: givens browser (2026-10-24)
- `givens browser open <url> | eval <js> | action <name> [json] | close`. The project is
  `GIVENS_ROOT` (the kit exports it), else the capabilities folder found from the working
  folder.
- A result goes to stdout, one line, nothing for undefined. A problem goes to stderr as one
  plain line, with exit 1; usage is exit 2. Chosen over the CLI printing "✗ in_browser: …"
  itself: `X="$(in_browser …)"` would swallow a ✗ line on stdout, and the kit, not the CLI,
  knows its own function names. The kit's browser functions put `fail "in_browser: <line>"`
  on stderr.
- No Chrome: "this scenario needs Chrome: brew install --cask google-chrome, or set
  GIVENS_CHROME".
- Binary: 6.9 MB to 13.1 MB with chromedp and cdproto linked in. Start-up unchanged:
  `givens list` on the sample shop takes 31 to 38 ms either way.
- Tried by hand, headless: eval with no browser, open a data: URL, eval, action, a missing
  action, a throw, usage, close, eval after close.

## Kit 2 (2026-10-24)
- `browser_open`, `in_browser`, `app_action` in givens.sh, through `_givens_browser`: runs
  `"${GIVENS_BIN:-givens}" browser …` with stderr in a temp file; on failure,
  `fail "<kit function>: <line>" >&2`. On stderr so `X="$(in_browser …)"` doesn't swallow
  it; set -e stops the script at the assignment.
- `need givens 'choose "Install givens command" from the Givens menu'` runs only when
  `GIVENS_BIN` is unset, so a Drop in never depends on the PATH.
- Header is `# givens kit 2`; the three copies (internal/kit, capabilities/_shared,
  testdata/sample-shop) are the same file.
- bats: "Browser lines in a terminal", "The JavaScript throws", "The page has no such app
  action", "The givens command isn't installed", and GIVENS_BIN winning over the PATH. They
  use a fake givens that logs its arguments; real Chrome is the Go suite's job.
- Step changed: share-setup-between-scenarios' "the sample shop's kit is updated from version
  1 to 2" became "… updated to the next version", in the feature, Go and Swift. It hard-coded
  version 1, so bumping the kit broke it; what it checks (an update keeps scenarios verified)
  is unchanged.
- the-kit/about.md: three function rows, a paragraph on the Givens browser, GIVENS_BIN and
  GIVENS_CHROME rows.

## Drop in: GIVENS_BIN, Chrome in the pre-run check, by_hand --url (2026-10-24)
- `begin` adds `GIVENS_BIN=<givens>` to setup's env: `Config.Command`, else the running
  program (the givens binary under serve). `App.givensCommand` is shared with "Install
  givens command". The Go world now sets `Command` to the suite's built givens, as the
  running program there is the test binary.
- `helpers.UsesBrowser`: a line calling browser_open, in_browser or app_action, before any
  `#`. `RequiredCommands` then lists "Chrome" with `brew install --cask google-chrome`, so
  the pre-run check says "This scenario needs Chrome" before anything runs, and "See what a
  scenario needs installed" lists it. Helpers' bodies and from setups count, as for need.
- "Open" on a by_hand card goes through `Window.openLink`: `browser.Open` while this
  project's Givens browser is running, else the URL opener.
- Reset state: nothing to do. Setup runs again, `browser_open` finds the running Chrome and
  its page file, and navigates the same tab. The Go steps will check no second window opens.

## Run blocks in setup cards (2026-10-24)
- markdown: the fence's info string is now read whole; `Block.Language` is its first word
  and `Block.Run` is true when the next word is "run". Before, "bash run" didn't match the
  fence pattern at all.
- `setup.BlockLanguage` and `setup.WriteBlock` are shared by Drop in and `givens run`: a
  bash block runs as `source <kit>` plus its code; a js block's code goes in a .js file and
  runs as `in_browser "$(cat <file>)"`, so it uses GIVENS_BIN and the Givens browser like a
  script line.
- The kit takes `GIVENS_SCRIPT_AS` (Givens sets it to the setup's .sh path, there or not)
  as GIVENS_SCRIPT, then unsets it, so the block loads its capability's helpers and from
  chains get the right key, and scripts the block starts don't inherit it.
- Drop in (cards_run.go): cards are worked top to bottom. `runNextCard` finds the first
  unmarked markdown card; if it has run blocks they run one at a time (`Session.block`), the
  card's output goes in `SetupCard.Output`, and exit 0 marks it done and moves on. A manual
  card waits as before. A run card offers nothing: it marks itself. A block exiting non-zero
  marks the card failed with "Setup stopped at step N of M (exit X)", offering "Edit setup"
  and "Try again". Stop and leaving stop a running block.
- `Session.Wait` keeps waiting while a block runs, even with no script process.
- Trust: a markdown setup with any run block asks, as a script does. Run blocks join
  `setupTexts`, so a js block or a bash block with browser lines needs Chrome before
  anything runs.
- givens run: a step with run blocks runs them and moves on without asking; a failure prints
  "✗ Stopped at step N of M (exit X)". Setup scripts still make it exit 1.
- Not yet exercised by a scenario; the Go steps are next.

## Go steps (2026-10-24)
- acceptance/mix-shell-browser-and-hand-steps: steps_test.go (writing setups), browser_test.go
  (the browser and the terminal), cards_test.go (pay-in-app.md's run blocks). 19 of 19 pass
  with GIVENS_TAGS=@wip, in about 42 s; "JavaScript that never answers" is 30 of those.
- The world: `GIVENS_HEADLESS=1` for every suite; `GIVENS_BROWSERS` points at each
  scenario's folder and `GIVENS_CHROME` is cleared in setUp (process-wide, so the scripts a
  Drop in starts see them; scenarios run one at a time); tearDown closes every Givens browser
  in the folder, stops the fake shop, and removes the folder with retries while Chrome's
  helpers finish. Checked: no Chrome left running after the suite.
- world.Shop (shop.go): an httptest page on every path, with window.app.signIn, whoami,
  addToCart (settles after 2 s, then fetches /settled so the step knows), lastOrderId, and no
  refund. Steps swap the spec's http://localhost:3000 for its address, so port 3000 needn't be
  free.
- Lines are added to a setup after the kit's line, in step order, ahead of the fixture's own
  lines. A step that needs a page (app_action, in_browser) adds `browser_open` to the shop
  first unless the setup has one: the spec's "then signs in with app_action" names no URL.
- browser.Pages and browser.Page let the steps check Reset and a terminal run reuse the
  one page and open no second.
- "the givens command isn't on the PATH" runs the script with PATH=/usr/bin:/bin.
- The common `it prints "…"` can't hold quotes, so this suite has its own pattern for the
  needs givens line.

## Mac (2026-10-24)
- Run cards show through the core's card lines, which DropInPage already draws: a run card's
  heading reads "Step N · runs by itself", then "· running", "· ran" or "· failed", and its
  output follows its text. No new SwiftUI view. Not eyeballed: drop in to a scenario whose
  .md has a `bash run` block and check the card's heading and output lines.
- Records: `CodeBlockRecord.run`, `SetupCardRecord.output`.
- `ServeClient.Configuration.environment`: extra variables for serve. The Swift suites run
  side by side, so each scenario's GIVENS_BROWSERS (and GIVENS_HEADLESS, GIVENS_CHROME) goes
  to its own serve, not through setenv. `Shell.run` takes the same.
- The fake shop moved to acceptance/shop, served in process by acceptance/world and as
  tools/fakeshop (printing its address on its first line) for Swift; Toolchain builds it.
  GET /was-settled tells another process the promise settled.
- World.swift's tearDown closes the scenario's Givens browser with `givens browser close`,
  stops fakeshop, and removes the folder with retries.
- Swift suite: macos/Tests/GivensAcceptanceTests/mix-shell-browser-and-hand-steps, three
  files mirroring Go's. 19 of 19 with GIVENS_TAGS=@wip, 46 s. Out of CoverageTests'
  windowless list.

## As built (2026-10-24)
- All 19 scenarios lost @wip and pass in Go and Swift; scripts/check.sh passes.
- GIVENS.md (capabilities/ and internal/starter/, which must match): the layout line for
  .md setups names "bash run" blocks; two rules, on run blocks and on the Givens browser.
- Left for a human: Chrome must be on every machine that runs check.sh, CI included. The
  browser suites fail without it, on purpose, with the install hint.

## Known gaps
- Not eyeballed: the Mac app's run cards, and a visible (not headless) Givens browser. To
  check: in a project with a dev server, add `browser_open "http://localhost:3000"` to a
  setup and drop in; a Chrome window opens on its own profile and stays open. Drop in again:
  the same window is reused.
- `browser.Open` and a `js run` card hold serve while the page loads (up to 30 s), as
  by_hand's "Open" calls it directly.
- The Givens browser's profile lives in ~/Library/Caches/Givens/browsers; nothing removes a
  project's profile when the project goes away.
- GIVENS_BROWSERS is set process-wide in the Go world: fine as scenarios run one at a time,
  but Go suites must not use t.Parallel.
