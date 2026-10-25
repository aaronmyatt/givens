import Foundation
import GherkinKit
import GivensUI

// Steps kept in the order they're written: When, Then, When, Then.

/// A scenario that goes back and forth, and the Drop in page's parts for it.
@MainActor
func dropInSequenceSteps(_ s: DropIntoAScenarioState) -> [StepDefinition] {
    [
        def(#"^"([^"]*)" goes When, Then, When, Then$"#) { call in
            try await s.goesBackAndForth(call[0])
        },
        def(#"^the checklist is in the order the steps are written$"#) { _ in
            try await s.checklistInOrder()
        },
        def(#"^the Drop in page shows a When part, a Then part, a When part and a Then part$"#) {
            _ in
            let kinds = try await s.session().parts.map(\.kind)
            guard kinds == ["when", "then", "when", "then"] else {
                throw StepFailure("the Drop in page's parts are \(kinds)")
            }
        },
    ]
}

extension DropIntoAScenarioState {
    /// What "goes When, Then, When, Then" adds to the end of a scenario: a second When and
    /// the Then that checks it.
    static let backAndForth = [
        #"    When they choose "Try another card""#, "    Then the card form opens again",
    ]

    /// Adds a second When and Then to the end of the named checkout scenario, and gives it a
    /// setup that finishes at once.
    func goesBackAndForth(_ name: String) async throws {
        try w.copySampleShop()
        let path = "capabilities/checkout/checkout.feature"
        var lines = try w.readFile(path).components(separatedBy: "\n")
        guard let start = lines.firstIndex(of: "  Scenario: \(name)") else {
            throw StepFailure("checkout.feature has no scenario \"\(name)\"")
        }
        var end = start + 1
        while end < lines.count && !lines[end].trimmingCharacters(in: .whitespaces).isEmpty {
            end += 1
        }
        lines.insert(contentsOf: Self.backAndForth, at: end)
        try w.writeFile(path, lines.joined(separator: "\n"))
        scenario = name
        try writeSetup(name, Self.quickSetup)
        try await prepare()
    }

    /// The checklist against Card declined's steps as written, the added When and Then last.
    func checklistInOrder() async throws {
        let trimmed = Self.backAndForth.map { $0.trimmingCharacters(in: .whitespaces) }
        let want = [
            "when: When they place the order",
            #"then: Then they see "Your card was declined""#,
            "then: And the order stays pending",
            "when: \(trimmed[0])", "then: \(trimmed[1])",
        ]
        let got = try await session().checklist.map { "\($0.kind): \($0.text)" }
        guard got == want else {
            throw StepFailure("the checklist is \(got), not \(want)")
        }
    }
}
