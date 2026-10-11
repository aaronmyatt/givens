import Foundation
import GherkinKit
import GivensUI

// Setup done by hand in scripts/<slug>.md, shown as cards.

/// Markdown setups: cards, cards after a script, and a card marked failed.
@MainActor
func dropInMarkdownSteps(_ s: DropIntoAScenarioState) -> [StepDefinition] {
    [
        def(#"^"([^"]*)" has scripts/([a-z0-9-]+)\.md with 3 numbered steps, and no \.sh script$"#)
        {
            call in
            guard slug(call[0]) == call[1] else {
                throw StepFailure("\(call[0])'s setup is \(slug(call[0])).md, not \(call[1]).md")
            }
            try s.markdownSetup(call[0], payInAppSetup)
        },
        def(#"^they aren't asked to trust the project$"#) { _ in
            let window = try await s.w.front()
            try expectNot(window.visible, "Run scripts from this project?", "the window")
            guard !window.offers.contains("Trust") else {
                throw StepFailure("the window offers \"Trust\": \(window.offers)")
            }
            _ = try await s.markdownCards()
        },
        def(
            #"^they see "([^"]*)" above (\d+) cards, with the link and the wallet screenshot shown$"#
        ) { call in try await s.introAboveCards(call[0], call[1]) },
        def(#"^choosing "Done" on the last card shows the checklist$"#) { _ in
            try await s.doneOnLastCard()
        },
        def(#"^"([^"]*)" has both scripts/([a-z0-9-]+)\.sh and scripts/([a-z0-9-]+)\.md$"#) {
            call in try await s.scriptAndMarkdown(call[0], call[1], call[2])
        },
        def(#"^the script's output appears first$"#) { _ in
            let (session, _) = try await s.markdownCards()
            try expect(session.output, "✓ app build installed", "the output")
        },
        def(#"^when it finishes, the cards from ([a-z0-9-]+)\.md are shown$"#) { call in
            let (_, cards) = try await s.markdownCards()
            guard cards.count == 3 else {
                throw StepFailure("\(call[0]).md gave \(cards.count) cards, not 3: \(cards)")
            }
        },
        def(#"^a developer has dropped in to "([^"]*)" and marked the first card "Done"$"#) {
            call in
            try s.markdownSetup(call[0], payInAppSetup)
            try await s.w.dropIn(call[0])
            let (session, cards) = try await s.markdownCards()
            try await session.chooseOnCard(cards[0].index, "Done")
        },
        def(#"^they choose "([^"]*)" on the (first|second|third|last) card$"#) { call in
            let (session, cards) = try await s.markdownCards()
            let ordinal = ["first": 0, "second": 1, "third": 2, "last": cards.count - 1]
            let at = ordinal[call[1]] ?? 0
            guard at < cards.count else { throw StepFailure("there's no \(call[1]) card") }
            try await session.chooseOnCard(cards[at].index, call[0])
        },
    ]
}

/// A markdown card with its index among all the session's cards, for chooseOnCard.
struct DropInMarkdownCard {
    let index: Int
    let card: SetupCardRecord
}

extension DropIntoAScenarioState {
    static let payInApp = "Pay in app"
    static let betaLink = "https://testflight.apple.com/join/abc"

    /// Makes `name` a checkout scenario whose setup is only scripts/<slug>.md, holding
    /// `content`. It doesn't trust the project: a markdown setup runs nothing.
    func markdownSetup(_ name: String, _ content: String) throws {
        w.untrusted = true
        try w.copySampleShop()
        try w.ensureScenario("checkout", name)
        scenario = name
        guard let project = w.project else { throw StepFailure("no sample shop") }
        let shell = project.appendingPathComponent("capabilities/checkout/scripts/\(slug(name)).sh")
        if FileManager.default.fileExists(atPath: shell.path) {
            try FileManager.default.removeItem(at: shell)
        }
        try w.writeFile("capabilities/checkout/scripts/\(slug(name)).md", content)
    }

    /// Waits for setup to pause at its cards, and returns the markdown ones.
    func markdownCards() async throws -> (SessionModel, [DropInMarkdownCard]) {
        let session = try await settleTo(SessionModel.waiting)
        let all = try await session.setupCards()
        let cards = all.enumerated().filter { $0.element.kind == "markdown" }
            .map { DropInMarkdownCard(index: $0.offset, card: $0.element) }
        guard !cards.isEmpty else {
            throw StepFailure("setup paused with no markdown cards: \(all)")
        }
        return (session, cards)
    }

    func introAboveCards(_ intro: String, _ count: String) async throws {
        try expect(try await w.front().visible, intro, "the window")
        let (_, cards) = try await markdownCards()
        guard String(cards.count) == count else {
            throw StepFailure("there are \(cards.count) cards, not \(count): \(cards.map(\.card))")
        }
        guard cards[0].card.links.contains(Self.betaLink) else {
            throw StepFailure("the first card doesn't link to \(Self.betaLink): \(cards[0].card)")
        }
        let last = cards[cards.count - 1].card
        guard last.images.contains(where: { $0.hasSuffix("images/wallet.png") }) else {
            throw StepFailure("the last card doesn't show the wallet screenshot: \(last)")
        }
    }

    /// Marks each card "Done" in turn, expecting the checklist only after the last.
    func doneOnLastCard() async throws {
        let (session, cards) = try await markdownCards()
        for (number, card) in cards.enumerated() {
            guard session.checklist.isEmpty else {
                throw StepFailure("the checklist showed before card \(number + 1) was done")
            }
            try await session.chooseOnCard(card.index, "Done")
        }
        guard try await !settleTo(SessionModel.ready).checklist.isEmpty else {
            throw StepFailure("no checklist after the last card was done")
        }
    }

    /// Gives `name` a script that prints one line, and the spec's markdown cards.
    func scriptAndMarkdown(_ name: String, _ shell: String, _ markdown: String) async throws {
        guard shell == slug(name) && markdown == slug(name) else {
            throw StepFailure(
                "\(name)'s setup files are named \(slug(name)), not \(shell)/\(markdown)")
        }
        try markdownSetup(name, payInAppSetup)
        try writeSetup(name, Self.script(#"ok "app build installed""#))
        try await prepare()
    }
}
