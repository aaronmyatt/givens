import Foundation
import GherkinKit
import GivensUI

// Step data reaching the setup script, unnamed Examples blocks, and the outline fixture.

/// Data tables as files, and Examples blocks with no names.
@MainActor
func dropInTableSteps(_ s: DropIntoAScenarioState) -> [StepDefinition] {
    [
        def(#"^the "Pay with saved card" Given step has a data table of two cart items$"#) { _ in
            try await s.dataTable()
        },
        def(#"^GIVENS_ARG_1 holds the path to a CSV file with a header row and both items$"#) {
            _ in try await s.argIsCSV()
        },
        def(#"^the setup script can seed the cart from it$"#) { _ in
            let cart = s.shopFile("cart") ?? ""
            guard cart == "Mug\nPoster\n" else {
                throw StepFailure("the setup seeded the cart with \"\(cart)\", not Mug and Poster")
            }
        },
        def(#"^"([^"]*)" has two Examples blocks with no names$"#) { call in
            guard call[0] == DropIntoAScenarioState.outlineName else {
                throw StepFailure("only the outline has Examples blocks to unname, not \(call[0])")
            }
            try s.writeOutline(named: false)
            try await s.prepare()
        },
        def(#"^the scenario is listed with the warning "([^"]*)"$"#) { call in
            let scenario = try await s.w.front().scenario(DropIntoAScenarioState.outlineName)
            try expect([scenario.warning], call[0], "the outline's warning")
        },
        def(#"^"Record run" is disabled on it until they're named$"#) { _ in
            try await s.recordDisabledUntilNamed()
        },
    ]
}

extension DropIntoAScenarioState {
    static let checkoutFeature = "capabilities/checkout/checkout.feature"

    /// The outline's steps, with placeholders for each row.
    static let outlineSteps = """
          Scenario Outline: Card rejected by the bank
            Given a signed-in shopper whose card is <card>
            When they place the order
            Then they see "<message>"

        """

    /// One named Examples block holding both rows.
    static let namedExamples = """

            Examples: Expired and stolen cards
              | card    | message                       |
              | expired | Your card has expired         |
              | stolen  | Your card was reported stolen |

        """

    /// Two Examples blocks with no names, one row each.
    static let unnamedExamples = """

            Examples:
              | card    | message               |
              | expired | Your card has expired |

            Examples:
              | card   | message                       |
              | stolen | Your card was reported stolen |

        """

    /// The data table Pay with saved card's Given step gets.
    static let cartTable = """
            Given a signed-in shopper with these items in the cart:
              | item   | price |
              | Mug    | 12.00 |
              | Poster | 8.50  |
        """

    /// Puts "Card rejected by the bank" in checkout.feature, replacing any outline of that
    /// name, so the rows are the ones these steps check. Its setup records the row it ran for
    /// in shop-data/ex-card.
    func writeOutline(named: Bool) throws {
        try w.copySampleShop()
        var text = dropInRemoveScenario(try w.readFile(Self.checkoutFeature), Self.outlineName)
        while text.hasSuffix("\n") { text.removeLast() }  // bound: text's length
        text += "\n\n" + Self.outlineSteps + (named ? Self.namedExamples : Self.unnamedExamples)
        try w.writeFile(Self.checkoutFeature, text)
        markers.append("ex-card")
        try writeSetup(
            Self.outlineName,
            Self.script(
                #"printf '%s' "$GIVENS_EX_card" > "# + Self.shopData + #"/ex-card""#,
                #"ok "card set to $GIVENS_EX_card""#))
    }

    /// Gives Pay with saved card's Given step a table of two cart items, and a setup that
    /// keeps a copy of GIVENS_ARG_1 and seeds a cart from it.
    func dataTable() async throws {
        try w.copySampleShop()
        let text = try w.readFile(Self.checkoutFeature)
        let header = "  Scenario: \(Self.payWithSavedCard)\n"
        guard let range = text.range(of: header),
            let givenEnd = text[range.upperBound...].firstIndex(of: "\n"),
            text[range.upperBound..<givenEnd].trimmingCharacters(in: .whitespaces)
                .hasPrefix("Given")
        else { throw StepFailure("checkout.feature has no \(Self.payWithSavedCard) with a Given") }
        let edited =
            String(text[..<range.upperBound]) + Self.cartTable + String(text[givenEnd...])
        try w.writeFile(Self.checkoutFeature, edited)
        try writeSetup(
            Self.payWithSavedCard,
            Self.script(
                #"printf '%s' "$GIVENS_ARG_1" > "# + Self.shopData + #"/arg-1-path""#,
                #"cp "$GIVENS_ARG_1" "# + Self.shopData + #"/arg-1.csv""#,
                #"tail -n +2 "$GIVENS_ARG_1" | while IFS=, read -r item _ || [ -n "$item" ]; do "#
                    + #"echo "$item" >> "# + Self.shopData + #"/cart"; done"#,
                #"ok "cart seeded""#))
        try await prepare()
    }

    /// Expects GIVENS_ARG_1 to have named a CSV file holding the header and both items. The
    /// table has no quoted fields, so splitting on commas reads it as RFC 4180 would.
    /// Ref: https://www.rfc-editor.org/rfc/rfc4180
    func argIsCSV() async throws {
        try await settleTo(SessionModel.ready)
        guard let path = shopFile("arg-1-path"), !path.isEmpty else {
            throw StepFailure("GIVENS_ARG_1 was empty or unset")
        }
        let copied = shopFile("arg-1.csv") ?? ""
        let records = copied.split(whereSeparator: \.isNewline).map {
            $0.components(separatedBy: ",")
        }
        let want = [["item", "price"], ["Mug", "12.00"], ["Poster", "8.50"]]
        guard records == want else { throw StepFailure("\(path) holds \(records), not \(want)") }
    }

    /// Drops in to a row and marks every check, expecting no "Record run"; then names the
    /// Examples blocks and expects the warning to clear and "Record run" to return.
    func recordDisabledUntilNamed() async throws {
        // As in Go: the blocks stay unnamed for the first run. rowAllPassed would name them.
        let window = try await w.front()
        try await window.select(Self.outlineName)
        try await rowPassed("expired")
        guard !window.offers.contains("Record run") else {
            throw StepFailure("\"Record run\" is offered with the blocks unnamed: \(window.offers)")
        }
        try writeOutline(named: true)
        let named = try await dropInEventually {
            try await window.scenario(Self.outlineName).warning.isEmpty
        }
        guard named else { throw StepFailure("the warning stayed after the blocks were named") }
        try await rowPassed("expired")
        try expect(window.offers, "Record run", "the offered actions once named")
    }
}

/// Removes the named scenario or outline, with its tags, from a feature's text.
func dropInRemoveScenario(_ text: String, _ name: String) -> String {
    var lines = text.components(separatedBy: "\n")
    let trimmed = { (line: String) in line.trimmingCharacters(in: .whitespaces) }
    guard
        var start = lines.firstIndex(where: {
            trimmed($0) == "Scenario: \(name)" || trimmed($0) == "Scenario Outline: \(name)"
        })
    else { return text }
    var end = start + 1
    let blockStarts = ["Scenario", "@", "Rule:", "Background:"]
    while end < lines.count && !blockStarts.contains(where: trimmed(lines[end]).hasPrefix) {
        end += 1  // bound: lines.count
    }
    while start > 0 && trimmed(lines[start - 1]).hasPrefix("@") { start -= 1 }  // bound: start
    lines.removeSubrange(start..<end)
    return lines.joined(separator: "\n")
}
