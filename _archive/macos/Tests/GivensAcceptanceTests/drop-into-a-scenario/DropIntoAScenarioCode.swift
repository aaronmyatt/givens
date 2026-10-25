import Foundation
import GherkinKit
import GivensUI

// Copying code from markdown cards: fenced blocks, inline code, and fences that never close.

/// Code blocks and inline code in markdown cards.
@MainActor
func dropInCodeSteps(_ s: DropIntoAScenarioState) -> [StepDefinition] {
    [
        def(
            #"^step 2 of pay-in-app\.md holds a bash code block, indented under its list item$"#
        ) { _ in try s.markdownSetup(DropIntoAScenarioState.payInApp, s.codeBlocksSetup) },
        def(#"^a developer chooses "Copy" on that code block in Drop in$"#) { _ in
            let (session, cards) = try await s.openCards()
            guard cards.count >= 2 else { throw StepFailure("there's no step 2 card") }
            try await session.copyCodeBlock(card: cards[1].index, block: 0)
        },
        def(#"^it holds no fences, list number or leading spaces$"#) { _ in
            let copied = s.w.clipboard.readText()
            guard !copied.contains("```") && !copied.hasPrefix("2.") else {
                throw StepFailure("the copy holds a fence or list number: \(copied)")
            }
            let lines = copied.components(separatedBy: "\n")
            guard !lines.contains(where: { $0.hasPrefix(" ") || $0.hasPrefix("\t") }) else {
                throw StepFailure("the copy holds a leading space: \(copied)")
            }
        },
        def(#"^step 2 holds a bash code block and step 3 holds a js code block$"#) { _ in
            try s.markdownSetup(DropIntoAScenarioState.payInApp, s.codeBlocksSetup)
        },
        def(#"^a developer looks at those cards in Drop in$"#) { _ in _ = try await s.openCards() },
        def(#"^each code block has its own Copy button, labelled "([^"]*)" and "([^"]*)"$"#) {
            call in try await s.labelledCopyButtons(call[0], call[1])
        },
        def(#"^copying one never includes the other$"#) { _ in try await s.copiesApart() },
        def(#"^a card says "([^"]*)"$"#) { call in try await s.cardSays(call[0]) },
        def(#"^a developer clicks (\S+)$"#) { call in
            try await s.session().clickInlineCode(card: s.cardIndex, code: call[0])
        },
        def(#"^step 2 of pay-in-app\.md opens a code fence that never closes$"#) { _ in
            try s.markdownSetup(DropIntoAScenarioState.payInApp, s.unclosedSetup)
        },
        def(#"^step (\d+)'s card shows "([^"]*)"$"#) { call in
            let (_, cards) = try await s.markdownCards()
            guard let step = Int(call[0]), step >= 1, step <= cards.count else {
                throw StepFailure("there's no card for step \(call[0])")
            }
            let card = cards[step - 1].card
            try expect([card.warning, card.text], call[1], "step \(step)'s card")
        },
        def(#"^it has no Copy button until the fence is closed$"#) { _ in
            try await s.noCopyUntilClosed()
        },
    ]
}

extension DropIntoAScenarioState {
    static let bashCode = "sudo -u shop shopctl flags set new_checkout on"
    static let jsCode = #"localStorage.removeItem("cart")"#

    /// pay-in-app.md with a bash block under step 2 and a js block under step 3, each
    /// indented under its list item, as the spec's comment shows.
    var codeBlocksSetup: String {
        """
        You'll need the TestFlight build of the shop app.

        1. Install it on your phone: [Shop beta](\(Self.betaLink))
        2. Turn on the new checkout in the server console:

           ```bash
           \(Self.bashCode)
           ```

        3. Clear the cart in the browser console:

           ```js
           \(Self.jsCode)
           ```

        """
    }

    /// pay-in-app.md with step 2's fence opened on line 4 and never closed.
    var unclosedSetup: String {
        """
        1. Install it on your phone: [Shop beta](\(Self.betaLink))
        2. Turn on the new checkout in the server console:

           ```bash
           \(Self.bashCode)

        3. Clear the cart in the browser console.

        """
    }

    /// Drops in to the scenario's markdown setup, unless a Drop in is already open.
    func openCards() async throws -> (SessionModel, [DropInMarkdownCard]) {
        if w.session == nil { try await w.dropIn(scenario) }
        return try await markdownCards()
    }

    func labelledCopyButtons(_ first: String, _ second: String) async throws {
        let (_, cards) = try await markdownCards()
        guard cards.count == 3 else { throw StepFailure("there are \(cards.count) cards, not 3") }
        for (at, language) in [(1, first), (2, second)] {
            let blocks = cards[at].card.blocks
            guard blocks.count == 1, blocks[0].language == language, blocks[0].copyable else {
                throw StepFailure(
                    "step \(at + 1) has no one \(language) block with Copy: \(blocks)")
            }
        }
    }

    /// Copies each block and expects exactly its own code on the clipboard.
    func copiesApart() async throws {
        let (session, cards) = try await markdownCards()
        guard cards.count == 3 else { throw StepFailure("there are \(cards.count) cards, not 3") }
        for (at, code) in [(1, Self.bashCode), (2, Self.jsCode)] {
            try await session.copyCodeBlock(card: cards[at].index, block: 0)
            let copied = w.clipboard.readText().trimmingCharacters(in: .newlines)
            guard copied == code else {
                throw StepFailure("copying step \(at + 1)'s block gave \"\(copied)\", not \(code)")
            }
        }
    }

    /// Drops in to the spec's pay-in-app.md and finds the card showing `text`.
    func cardSays(_ text: String) async throws {
        try markdownSetup(Self.payInApp, payInAppSetup)
        let (_, cards) = try await openCards()
        guard let card = cards.first(where: { $0.card.text.contains(text) }) else {
            throw StepFailure("no card says \"\(text)\": \(cards.map(\.card.text))")
        }
        cardIndex = card.index
    }

    /// Expects step 2's block not to be copyable, then closes the fence, drops in again, and
    /// expects it to be.
    func noCopyUntilClosed() async throws {
        let (_, open) = try await markdownCards()
        guard open.count >= 2, !dropInHasCopy(open[1].card) else {
            throw StepFailure("step 2's card offers Copy with its fence open: \(open.map(\.card))")
        }
        let closed = unclosedSetup.replacingOccurrences(
            of: Self.bashCode + "\n", with: Self.bashCode + "\n   ```\n")
        try markdownSetup(Self.payInApp, closed)
        try await w.dropIn(Self.payInApp)
        let (_, fixed) = try await markdownCards()
        guard fixed.count >= 2, dropInHasCopy(fixed[1].card) else {
            throw StepFailure(
                "step 2's card has no Copy once its fence closed: \(fixed.map(\.card))")
        }
    }
}

/// True when a card offers Copy on a code block.
func dropInHasCopy(_ card: SetupCardRecord) -> Bool {
    card.blocks.contains(where: \.copyable) || card.offers.contains("Copy")
}
