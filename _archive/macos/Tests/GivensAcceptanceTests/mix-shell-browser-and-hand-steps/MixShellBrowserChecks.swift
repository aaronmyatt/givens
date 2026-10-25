import Foundation
import GherkinKit
import GivensUI

/// The When and Then steps about the Givens browser and setup's output.
@MainActor
func mixBrowserSteps(_ s: MixShellState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^the output shows "([^"]*)"$"#) { call in try s.outputShows(call[0]) },
        def(#"^the Givens browser shows the shop's cart page$"#) { _ in
            try mixPageIs(s, "/cart")
        },
        def(
            #"^the output shows the seed line, then the app action's line, then the by_hand card$"#
        ) { _ in try await mixSeedActionCard(s) },
        def(#"^choosing "Done" on the card shows the checklist$"#) { _ in
            let session = try s.session()
            _ = try await session.chooseOnCard(0, "Done")
            _ = try await session.wait(seconds: World.setupWaitSeconds)
            guard !session.checklist.isEmpty else {
                throw StepFailure("no checklist; setup is \(session.status): \(session.output)")
            }
        },
        def(#"^"([^"]*)" is printed only after the promise settles$"#) { call in
            let output = try s.session().output
            guard let at = output.firstIndex(of: call[0]), at > 0,
                output[at - 1] == #"{"cart":["tee"]}"#
            else { throw StepFailure("\(call[0]) doesn't follow the settled cart: \(output)") }
            guard let url = URL(string: try w.shopURL() + "/was-settled") else { return }
            let (data, _) = try await URLSession.shared.data(from: url)
            guard String(decoding: data, as: UTF8.self) == "yes" else {
                throw StepFailure("the shop never heard the promise settle")
            }
        },
        def(#"^the rest of setup sees ORDER as "([^"]*)"$"#) { call in
            try s.outputShows("✓ ORDER is \(call[0])")
        },
        def(#"^no checklist is shown$"#) { _ in
            let session = try s.session()
            guard session.checklist.isEmpty, session.status != SessionModel.ready else {
                throw StepFailure("setup is \(session.status) with a checklist")
            }
        },
        def(#"^after 30 seconds the output shows "([^"]*)"$"#) { call in
            // A call may take setupWaitSeconds, so the 30 s and Chrome's start are waited in turns.
            for _ in 0..<5 where try s.session().status == SessionModel.running {
                _ = try await s.session().wait(seconds: World.setupWaitSeconds)
            }
            try s.outputShows(call[0])
        },
        def(#"^nothing in the script runs$"#) { _ in
            let output = try s.session().output
            guard output.isEmpty else { throw StepFailure("setup printed \(output)") }
        },
        def(#"^the install hint "([^"]*)" is shown with a copy button$"#) { call in
            let window = try await w.front()
            try expect(window.visible, call[0], "the window")
            try await window.choose("Copy")
            guard w.clipboard.readText().trimmingCharacters(in: .newlines) == call[0] else {
                throw StepFailure("Copy put \(w.clipboard.readText()) on the clipboard")
            }
        },
        def(#"^a developer drops in to "([^"]*)" and setup finishes$"#) { call in
            try await mixDropInReady(s, call[0])
        },
        def(#"^the Givens browser stays open on the shop, signed in as "([^"]*)"$"#) { call in
            let who = try s.browser("eval 'window.app.whoami()'")
            guard who == call[0] else { throw StepFailure("signed in as \(who), not \(call[0])") }
        },
        def(#"^the developer's own Chrome profile is left as it was$"#) { _ in
            let browsers = w.dir.appendingPathComponent("browsers")
            let profiles = try FileManager.default.contentsOfDirectory(atPath: browsers.path)
            let port = browsers.appendingPathComponent("\(profiles.first ?? "")/DevToolsActivePort")
            guard profiles.count == 1, FileManager.default.fileExists(atPath: port.path) else {
                throw StepFailure("Chrome didn't start on Givens's own profile: \(profiles)")
            }
        },
        def(#"^a developer has dropped in to "([^"]*)" and the Givens browser is open$"#) { call in
            try s.withPage(call[0])
            try await mixDropInReady(s, call[0])
            s.page = try s.pageFile()
        },
        def(#"^setup runs again in the same Givens browser window$"#) { _ in
            try await mixSamePage(s)
        },
        def(#"^setup uses that same Givens browser window$"#) { _ in try await mixSamePage(s) },
        def(#"^no second window opens$"#) { _ in
            let pages = try await s.pages()
            guard pages.count == 1 else { throw StepFailure("\(pages.count) pages are open") }
        },
        def(#"^a developer drops in and chooses "Open" on the card$"#) { _ in
            try await w.dropIn("Card declined")
            _ = try await s.session().chooseOnCard(0, "Open")
        },
        def(#"^the admin page opens in the Givens browser, not the default browser$"#) { _ in
            try mixPageIs(s, "/admin")
            guard w.urlOpener.opened.isEmpty else {
                throw StepFailure("the default browser opened \(w.urlOpener.opened)")
            }
        },
    ]
}

/// The steps that run setup in a terminal.
@MainActor
func mixTerminalSteps(_ s: MixShellState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^a developer runs "([^"]*)" in a terminal$"#) { call in try w.run(call[0]) },
        def(#"^the Givens browser opens and the app action runs, as in Drop in$"#) { _ in
            let opened = try s.shop("✓ opened \(mixShopURL)/")
            guard w.exitCode == 0, w.output.contains(opened),
                w.output.contains("sam@example.com")
            else { throw StepFailure("the script exited \(w.exitCode):\n\(w.output)") }
            guard try s.browser("eval 'window.app.whoami()'") == "sam@example.com" else {
                throw StepFailure("the app action didn't sign in")
            }
        },
        def(#"^the browser stays open after the script exits$"#) { _ in
            _ = try s.browser("eval 1")
        },
        def(#"^the givens command isn't on the PATH$"#) { _ in },
        def(#"^a developer runs the script in a terminal$"#) { _ in
            guard let project = w.project else { throw StepFailure("no sample shop copied") }
            let script = try w.scriptPath("Card declined")
            let result = try Shell.run(
                "PATH=/usr/bin:/bin /bin/bash \(Shell.quote(script))", in: project,
                environment: w.environment)
            (w.output, w.exitCode) = (result.output, result.status)
        },
        // The common "it prints" takes no quotes inside; this line has some.
        def(#"^it prints "(✗ needs givens: .*)"$"#) { call in
            guard w.output.contains(call[0]) else { throw StepFailure("it printed \(w.output)") }
        },
        def(#"^nothing after that line runs$"#) { _ in
            guard w.exitCode != 0, !w.output.contains("✓") else {
                throw StepFailure("the script carried on (exit \(w.exitCode)):\n\(w.output)")
            }
        },
        def(#"^a developer ran the "([^"]*)" setup in a terminal and the Givens browser is open$"#)
        {
            call in
            try s.withPage(call[0])
            try w.run("bash \(Shell.quote(try w.scriptPath(call[0])))")
            guard w.exitCode == 0 else { throw StepFailure("the script failed:\n\(w.output)") }
            s.page = try s.pageFile()
        },
    ]
}

/// Expects the Givens browser at `path` on the shop.
@MainActor
private func mixPageIs(_ s: MixShellState, _ path: String) throws {
    let at = try s.browser("eval 'location.pathname'")
    guard at == path else { throw StepFailure("the Givens browser is at \(at), not \(path)") }
}

/// Expects the seed line, then signIn's result, then the by_hand card waiting.
@MainActor
private func mixSeedActionCard(_ s: MixShellState) async throws {
    let session = try s.session()
    let output = session.output
    guard let seed = output.firstIndex(of: "✓ seeded sam"),
        let action = output.firstIndex(of: "sam@example.com"), seed < action
    else { throw StepFailure("the output isn't the seed line, then the action's: \(output)") }
    let cards = try await session.setupCards()
    guard cards.count == 1, cards[0].where == "Phone", cards[0].text == "Approve the sign-in",
        session.status == SessionModel.waiting
    else { throw StepFailure("setup is \(session.status) with cards \(cards)") }
}

/// Drops in and expects setup to end ready.
@MainActor
private func mixDropInReady(_ s: MixShellState, _ name: String) async throws {
    try await s.w.dropIn(name)
    let session = try s.session()
    guard session.status == SessionModel.ready else {
        throw StepFailure("setup ended \(session.status): \(session.output)")
    }
}

/// Expects setup to have run again, on the page it had before.
@MainActor
private func mixSamePage(_ s: MixShellState) async throws {
    _ = try await s.session().wait(seconds: World.setupWaitSeconds)
    try s.outputShows("✓ opened \(mixShopURL)/")
    let now = try s.pageFile()
    guard now == s.page else { throw StepFailure("setup used page \(now), not \(s.page)") }
}
