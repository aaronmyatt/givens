import Foundation
import GherkinKit
import GivensUI

/// The steps about pay-in-app.md's cards and their run blocks. Mirrors cards_test.go.
@MainActor
func mixCardSteps(_ s: MixShellState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^step (\d+) of pay-in-app\.md has a "(bash|js) run" block that calls ok "([^"]*)"$"#)
        { call in try mixRunBlock(s, call[0], call[1], #"ok "\#(call[2])""#) },
        def(
            #"^step (\d+) of pay-in-app\.md has a "(bash|js) run" block with browser_open "([^"]*)"$"#
        ) { call in
            try mixRunBlock(s, call[0], call[1], #"browser_open "\#(try s.shop(call[2]))""#)
        },
        def(#"^step (\d+) has a "(bash|js) run" block holding (.+)$"#) { call in
            try mixRunBlock(s, call[0], call[1], call[2])
        },
        def(#"^step (\d+) of pay-in-app\.md has a "(bash|js) run" block that exits with (\d+)$"#) {
            call in
            try mixRunBlock(s, call[0], call[1], "echo \"seeding shopper\"\nexit \(call[2])")
        },
        def(#"^step (\d+) of pay-in-app\.md has a "(bash|js) run" block$"#) { call in
            try mixRunBlock(s, call[0], call[1], #"ok "ran step \#(call[0])""#)
        },
        def(#"^step (\d+) of pay-in-app\.md has a "bash run" block and step (\d+) has none$"#) {
            call in try mixRunBlock(s, call[0], "bash", #"ok "seeded shopper""#)
        },
        def(#"^step (\d+) has no run block$"#) { call in
            let number = try mixCardStep(s, call[0])
            guard !s.cardSteps[number - 1].contains(" run\n") else {
                throw StepFailure("step \(number) has a run block")
            }
        },
        def(#"^a developer has never run anything from the sample shop$"#) { _ in
            w.untrusted = true
        },
        def(#"^step (\d+) runs by itself and its card shows "([^"]*)"$"#) { call in
            try expect(try await mixCard(s, call[0]).output, call[1], "step \(call[0])'s card")
        },
        def(#"^step (\d+) is marked done when its block exits with 0$"#) { call in
            try await mixCardDone(s, call[0])
        },
        def(#"^step (\d+) waits for "Done"$"#) { call in
            let card = try await mixCard(s, call[0])
            guard card.mark.isEmpty, card.offers.contains("Done"),
                try s.session().status == SessionModel.waiting
            else { throw StepFailure("step \(call[0]) is marked \(card.mark): \(card.offers)") }
        },
        def(#"^step (\d+) runs in the Givens browser by itself$"#) { call in
            try await mixCardDone(s, call[0])
        },
        def(#"^its card shows "([^"]*)"$"#) { call in
            let card = try await mixCard(s, String(s.cardSteps.count - 1))
            try expect(card.output, call[0], "the card")
        },
        def(#"^step (\d+)'s card shows the block's output so far$"#) { call in
            try expect(try await mixCard(s, call[0]).output, "seeding shopper", "the card")
        },
        def(#"^they see "([^"]*)" before any card runs$"#) { call in
            try await w.seesEventually(call[0])
            for card in try await s.session().setupCards()
            where !card.mark.isEmpty || !card.output.isEmpty {
                throw StepFailure("a card ran before trust: \(card)")
            }
        },
        def(#"^step (\d+) runs and prints its output$"#) { _ in
            guard w.output.contains("✓ seeded shopper") else {
                throw StepFailure("it printed:\n\(w.output)")
            }
        },
        def(#"^step (\d+) is printed as plain text and waits for Enter, or f if it failed$"#) {
            call in
            for text in [
                "\(call[0]). Sign in as sam@example.com",
                "Press Enter when done, or type f if it failed",
            ] where !w.output.contains(text) {
                throw StepFailure("the output doesn't hold \(text):\n\(w.output)")
            }
        },
    ]
}

/// Gives pay-in-app.md's step a run block, and makes the file the scenario's only setup.
@MainActor
private func mixRunBlock(_ s: MixShellState, _ step: String, _ language: String, _ code: String)
    throws
{
    let number = try mixCardStep(s, step)
    let indented = "   " + code.replacingOccurrences(of: "\n", with: "\n   ")
    s.cardSteps[number - 1] += "\n\n   ```\(language) run\n\(indented)\n   ```"
    let scripts = "capabilities/checkout/scripts"
    try? s.w.removeFile("\(scripts)/pay-in-app.sh")
    var text = "You'll need the TestFlight build of the shop app.\n\n"
    for (index, body) in s.cardSteps.enumerated() { text += "\(index + 1). \(body)\n" }
    try s.w.writeFile("\(scripts)/pay-in-app.md", text)
}

/// Checks a step number of pay-in-app.md, adding the scenario first if no step has.
@MainActor
private func mixCardStep(_ s: MixShellState, _ step: String) throws -> Int {
    if s.cardSteps.isEmpty {
        s.cardSteps = [
            "Install it on your phone: [Shop beta](https://testflight.apple.com/join/abc)",
            "Sign in as `sam@example.com`",
            "Add the test card `4242 4242 4242 4242` under Wallet",
        ]
        try s.w.copySampleShop()
        try s.w.ensureScenario("checkout", "Pay in app")
    }
    guard let number = Int(step), number >= 1, number <= s.cardSteps.count else {
        throw StepFailure("pay-in-app.md has no step \(step)")
    }
    return number
}

/// One of pay-in-app.md's cards, once setup has run as far as it goes.
@MainActor
private func mixCard(_ s: MixShellState, _ step: String) async throws -> SetupCardRecord {
    let session = try s.session()
    _ = try await session.wait(seconds: World.setupWaitSeconds)
    let number = try mixCardStep(s, step)
    let cards = try await session.setupCards()
    guard cards.count == s.cardSteps.count else {
        throw StepFailure("\(cards.count) cards; setup is \(session.status): \(session.output)")
    }
    return cards[number - 1]
}

/// Expects a card to have marked itself done.
@MainActor
private func mixCardDone(_ s: MixShellState, _ step: String) async throws {
    let card = try await mixCard(s, step)
    guard card.mark == "done" else {
        throw StepFailure("step \(step) is marked \(card.mark): \(card.output)")
    }
}
