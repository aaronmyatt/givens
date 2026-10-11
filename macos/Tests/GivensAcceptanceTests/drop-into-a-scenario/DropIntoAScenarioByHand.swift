import Foundation
import GherkinKit
import GivensUI

// first_you lines, and by_hand cards from a setup script.

/// first_you lines and by_hand cards.
@MainActor
func dropInByHandSteps(_ s: DropIntoAScenarioState) -> [StepDefinition] {
    [
        def(#"^the "Card declined" setup ends with first_you '([^']*)'$"#) { call in
            try s.writeSetup(
                "Card declined",
                DropIntoAScenarioState.script(
                    #"ok "seeded shopper""#, "first_you " + Shell.quote(call[0])))
            try await s.prepare()
        },
        def(#"^the checklist starts with "(.*)"$"#) { call in
            let items = try await s.settleTo(SessionModel.ready).checklist
            guard let first = items.first, first.text == call[0], first.kind == "given" else {
                throw StepFailure("the checklist doesn't start with the Given \(call[0]): \(items)")
            }
        },
        def(#"^the When steps come after it$"#) { _ in try await s.whensAfter() },
        def(#"^the Then steps are still the only ones marked "Pass" or "Fail"$"#) { _ in
            try await s.onlyThensPassFail()
        },
        def(#"^the "([^"]*)" setup has a by_hand step for "([^"]*)" with the snippet "([^"]*)"$"#) {
            call in try await s.byHandSnippet(call[0], call[1], call[2])
        },
        def(#"^the output pauses at a card showing "([^"]*)" and the snippet$"#) { call in
            let (session, card) = try await s.byHandCard(call[0])
            try expect([card.text], s.snippet, "the \(call[0]) card")
            try expectNot(session.output, DropIntoAScenarioState.afterCard, "the paused output")
        },
        def(#"^they can choose "Copy" to copy the snippet$"#) { _ in
            try await s.session().chooseOnCard(s.cardIndex, "Copy")
            let copied = s.w.clipboard.readText()
            guard copied.trimmingCharacters(in: .newlines) == s.snippet else {
                throw StepFailure("Copy put \"\(copied)\" on the clipboard, not the snippet")
            }
        },
        def(#"^choosing "Done" carries on with the rest of setup$"#) { _ in
            try await s.session().chooseOnCard(s.cardIndex, "Done")
            let output = try await s.settleTo(SessionModel.ready).output
            try expect(output, DropIntoAScenarioState.afterCard, "the output after Done")
        },
        def(#"^a developer has dropped in to "([^"]*)" and setup is paused at its by_hand step$"#) {
            call in try await s.pausedAtByHand(call[0])
        },
        def(#"^setup stops and they see "([^"]*)"$"#) { call in
            let session = try await s.settleTo(SessionModel.failed)
            try expect(try await s.w.front().visible, call[0], "the window")
            try expectNot(session.output, DropIntoAScenarioState.afterCard, "the output")
        },
    ]
}

/// by_hand cards with a --url or --qr link.
@MainActor
func dropInLinkSteps(_ s: DropIntoAScenarioState) -> [StepDefinition] {
    [
        def(#"^the "([^"]*)" setup has by_hand --(url|qr) "([^"]*)" for "([^"]*)"$"#) { call in
            try await s.byHandLink(call[0], call[1], call[2], call[3])
        },
        def(#"^a developer drops in and setup pauses at that step$"#) { _ in
            try await s.w.dropIn(s.scenario)
            _ = try await s.linkCard()
        },
        def(#"^the card offers "([^"]*)"$"#) { call in
            try expect(try await s.linkCard().offers, call[0], "the card's actions")
        },
        def(#"^choosing it opens the page in their default browser$"#) { _ in
            _ = try await s.linkCard()
            let opened = try await s.session().chooseOnCard(s.cardIndex, "Open")
            guard opened == [s.link] else {
                throw StepFailure("the default browser was asked to open \(opened), not \(s.link)")
            }
        },
        def(#"^the card shows the link as a QR code$"#) { _ in
            let card = try await s.linkCard()
            guard card.qrCode else { throw StepFailure("the card shows no QR code: \(card)") }
        },
        def(#"^the link is shown as text beneath it$"#) { _ in
            try expect([try await s.linkCard().text], s.link, "the card's text")
        },
    ]
}

extension DropIntoAScenarioState {
    static let byHandWhere = "DigitalOcean console"
    static let byHandText = "shopctl flags set new_checkout on"

    /// A setup that stops at a by_hand step, then prints a line once it's done.
    static func byHandScript(_ place: String, _ args: [String] = []) -> String {
        let quoted = args + [Shell.quote(place), Shell.quote(byHandText)]
        return script(
            #"ok "seeded shopper""#, "by_hand " + quoted.joined(separator: " "),
            #"ok "rest of setup ran""#)
    }

    func whensAfter() async throws {
        let items = try await session().checklist
        let (whens, thens) = try expectedChecks("Card declined")
        guard items.count >= 1 + whens.count, items[1].kind == "when" else {
            throw StepFailure("the When steps don't follow the first Given: \(items)")
        }
        try dropInExpectKinds(items, ["when": whens, "then": thens])
    }

    /// Expects a Given or When marked "pass" or "fail" to be refused, while "done" and a
    /// Then's "pass" are taken.
    func onlyThensPassFail() async throws {
        let session = try await session()
        for kind in ["given", "when"] {
            let item = try dropInFirst(session.checklist, kind)
            for mark in ["pass", "fail"]
            where try await !dropInRefused({ try await session.mark(item.text, mark) }) {
                throw StepFailure("the \(kind) step \"\(item.text)\" took the mark \(mark)")
            }
            try await session.mark(item.text, "done")
        }
        try await session.mark(try dropInFirst(session.checklist, "then").text, "pass")
    }

    func byHandSnippet(_ name: String, _ place: String, _ text: String) async throws {
        guard text == Self.byHandText else {
            throw StepFailure("the by_hand snippet is \"\(text)\", not \"\(Self.byHandText)\"")
        }
        scenario = name
        snippet = text
        try writeSetup(name, Self.byHandScript(place))
        try await prepare()
    }

    /// Waits for setup to pause, and finds the card for a by_hand step done at `place`.
    func byHandCard(_ place: String) async throws -> (SessionModel, SetupCardRecord) {
        let session = try await settleTo(SessionModel.waiting)
        let cards = try await session.setupCards()
        guard let index = cards.firstIndex(where: { $0.kind == "by_hand" && $0.where == place })
        else { throw StepFailure("no by_hand card for \"\(place)\": \(cards)") }
        cardIndex = index
        return (session, cards[index])
    }

    /// Drops in to a setup that stops at a by_hand step in the DigitalOcean console.
    func pausedAtByHand(_ name: String) async throws {
        snippet = Self.byHandText
        try writeSetup(name, Self.byHandScript(Self.byHandWhere))
        try await prepare()
        try await w.dropIn(name)
        _ = try await byHandCard(Self.byHandWhere)
    }

    /// Gives the scenario a by_hand step with --url or --qr.
    func byHandLink(_ name: String, _ flag: String, _ url: String, _ place: String) async throws {
        scenario = name
        link = url
        snippet = Self.byHandText
        try writeSetup(name, Self.byHandScript(place, ["--" + flag, Shell.quote(url)]))
        try await prepare()
    }

    /// The paused setup's by_hand card that holds the step's link.
    func linkCard() async throws -> SetupCardRecord {
        let cards = try await settleTo(SessionModel.waiting).setupCards()
        guard let index = cards.firstIndex(where: { $0.kind == "by_hand" && $0.link == link })
        else { throw StepFailure("no card links to \(link): \(cards)") }
        cardIndex = index
        return cards[index]
    }
}
