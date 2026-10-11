import Foundation
import GherkinKit
import GivensUI

// ask cards, by_hand cards in a scenario a setup builds on, and the timer pausing at a card.

/// ask cards, cards from a `from` scenario, and waiting at a card.
@MainActor
func dropInAskSteps(_ s: DropIntoAScenarioState) -> [StepDefinition] {
    [
        def(#"^the "([^"]*)" setup has ask ([A-Z_]+) "([^"]*)"$"#) { call in
            try await s.setupAsks(call[0], call[1], call[2])
        },
        def(#"^a developer drops in, types "([^"]*)" into the card and chooses "([^"]*)"$"#) {
            call in
            try await s.w.dropIn(s.scenario)
            let session = try await s.askCard()
            try await s.w.front().type(call[0])
            try await session.chooseOnCard(s.cardIndex, call[1])
        },
        def(#"^the rest of setup runs with ([A-Z_]+) set to "([^"]*)"$"#) { call in
            try await s.settleTo(SessionModel.ready)
            let answer = s.shopFile("answer") ?? ""
            guard answer == call[1] else {
                throw StepFailure("setup ran on with \(call[0])=\"\(answer)\", not \"\(call[1])\"")
            }
        },
        def(#"^setup is paused at an ask card$"#) { _ in
            try await s.setupAsks(
                "Accept invite", "INVITE_CODE", "Paste the invite code shown on the phone")
            try await s.w.dropIn("Accept invite")
            _ = try await s.askCard()
        },
        def(#"^a developer chooses "Continue" with the field empty$"#) { _ in
            try await s.session().chooseOnCard(s.cardIndex, "Continue")
        },
        def(#"^setup stays paused$"#) { _ in try await s.staysPaused() },
        def(#"^the "Pay with saved card" setup has a by_hand step$"#) { _ in
            try await s.fromScenarioByHand()
        },
        def(#"^the card for Pay with saved card's step is shown$"#) { _ in
            _ = try await s.byHandCard("Bank portal")
        },
        def(#"^its other "your turn" lines stay hidden$"#) { _ in try await s.yourTurnHidden() },
        def(#"^(\d+) minutes pass before they choose "Done"$"#) { call in
            try await s.minutesPassThenDone(call[0])
        },
    ]
}

extension DropIntoAScenarioState {
    /// Gives the scenario a setup that asks for a value, then saves it in shop-data/.
    func setupAsks(_ name: String, _ variable: String, _ prompt: String) async throws {
        scenario = name
        markers.append("answer")
        try writeSetup(
            name,
            Self.script(
                "ask \(variable) \(Shell.quote(prompt))",
                "printf '%s' \"$\(variable)\" > " + Self.shopData + "/answer\"",
                #"ok "rest of setup ran""#))
        try await prepare()
    }

    /// Waits for setup to pause at an ask card, and finds it.
    func askCard() async throws -> SessionModel {
        let session = try await settleTo(SessionModel.waiting)
        let cards = try await session.setupCards()
        guard let index = cards.firstIndex(where: { $0.kind == "ask" }) else {
            throw StepFailure("setup paused with no ask card: \(cards)")
        }
        cardIndex = index
        return session
    }

    func staysPaused() async throws {
        let session = try await session()
        guard session.status == SessionModel.waiting else {
            throw StepFailure("setup is \"\(session.status)\", not still paused")
        }
        if shopFile("answer") != nil { throw StepFailure("setup carried on past the ask card") }
        try expectNot(session.output, Self.afterCard, "the output")
    }

    /// Gives Pay with saved card a setup with "your turn" lines, which `from` hides, then a
    /// by_hand step in a bank portal.
    func fromScenarioByHand() async throws {
        try writeSetup(
            Self.payWithSavedCard,
            Self.script(
                "first_you 'open the shop app'", #"when_you 'tap "Pay"'"#,
                #"then_check 'you see "Order placed"'"#, #"ok "saved card added""#,
                "by_hand 'Bank portal' 'Approve the test card'", #"ok "card approved""#))
        try await prepare()
    }

    func yourTurnHidden() async throws {
        let output = try await session().output
        for hidden in ["Your turn", "open the shop app", #"tap "Pay""#, "Order placed"] {
            try expectNot(output, hidden, "the output")
        }
        try expect(output, "✓ saved card added", "the output")
    }

    /// Moves the clock on while setup waits at a card, watches for "Still running", then
    /// chooses "Done".
    func minutesPassThenDone(_ minutes: String) async throws {
        guard let count = Int(minutes) else { throw StepFailure("\(minutes) isn't minutes") }
        try await advance(seconds: count * 60)
        sawSlow = try await !dropInStaysFalse({ try await shows(Self.stillRunning) })
        try await session().chooseOnCard(cardIndex, "Done")
        do {
            try await settleTo(SessionModel.ready)
        } catch let failure as StepFailure {
            throw StepFailure("setup didn't finish after Done: \(failure.description)")
        }
    }
}
