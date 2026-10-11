import Foundation
import GherkinKit
import GivensUI

// Outlines: one Drop in page for every Examples row.

/// Outline rows: listing them, dropping in to one, switching, and recording a row's run.
@MainActor
func dropInRowSteps(_ s: DropIntoAScenarioState) -> [StepDefinition] {
    [
        def(
            #"^checkout\.feature has the outline "Card rejected by the bank"(?: and the scenario "([^"]*)")?$"#
        ) { call in try await s.hasOutline(call[0]) },
        def(#"^its Drop in page lists the "([^"]*)" and "([^"]*)" rows one after another$"#) {
            call in
            let cards = try await s.rows().map { $0.values["card"] ?? "" }
            guard cards == [call[0], call[1]] else {
                throw StepFailure("the rows are \(cards), not \(call[0]) then \(call[1])")
            }
        },
        def(#"^each row shows its steps with the placeholders filled in, like "(.*)"$"#) { call in
            try await s.rowsFilledIn(call[0])
        },
        def(#"^each row shows its status and its own "Drop in" button$"#) { _ in
            for row in try await s.rows()
            where row.status.isEmpty || !row.offers.contains("Drop in") {
                throw StepFailure("the \(row.values) row has no status or no \"Drop in\": \(row)")
            }
        },
        def(#"^a developer has "([^"]*)" open$"#) { call in try await s.outlineOpen(call[0]) },
        def(#"^they choose "([^"]*)" on the "([^"]*)" row$"#) { call in
            try await s.chooseOnRow(call[0], call[1])
        },
        def(#"^(?:the setup script runs|that setup runs) with GIVENS_EX_card set to "([^"]*)"$"#) {
            call in try await s.exampleCardIs(call[0])
        },
        def(#"^that row's checklist opens beneath it$"#) { _ in
            try await s.rowChecklistLive("expired")
        },
        def(#"^the "([^"]*)" row stays readable but greyed out, with its "Drop in" disabled$"#) {
            call in try await s.rowGreyedOut(call[0])
        },
        def(#"^a developer has dropped in to the "([^"]*)" row and marked one Then step$"#) {
            call in try await s.rowMarkedOne(call[0])
        },
        def(#"^they're asked "([^"]*)"$"#) { call in
            let window = try await s.w.front()
            try expect(window.visible, call[0], "the window")
            try expect(window.offers, "Leave", "the offered actions")
        },
        def(#"^choosing "Leave" closes the ([a-z]+) checklist and runs setup for "([^"]*)"$"#) {
            call in try await s.leaveSwitches(call[0], call[1])
        },
        def(
            #"^a developer has dropped in to "([^"]*)" with the "([^"]*)" row and marked every Then step "Pass"$"#
        ) { call in try await s.rowAllPassed(call[0], call[1]) },
        def(#"^a line for (\S+) is added to today's file in runs/, with (\w+)=(\w+)$"#) { call in
            let fields = try s.runLines(call[0]).last ?? []
            guard dropInHasField(fields, "\(call[1])=\(call[2])") else {
                throw StepFailure(
                    "the run line for \(call[0]) lacks \(call[1])=\(call[2]): \(fields)")
            }
        },
        def(#"^the scenario shows "([^"]*)"$"#) { call in
            let scenario = try await s.w.front().scenario(DropIntoAScenarioState.outlineName)
            try expect([scenario.status], call[0], "the outline's status")
        },
        def(#"^the Unverified view lists the "([^"]*)" row as "([^"]*)"$"#) { call in
            let window = try await s.w.front()
            try await window.select("Unverified")
            guard window.visible.contains(where: { $0.contains(call[0]) && $0.contains(call[1]) })
            else { throw StepFailure("Unverified doesn't list \(call[0]) as \(call[1])") }
        },
    ]
}

extension DropIntoAScenarioState {
    static let outlineName = "Card rejected by the bank"
    static let rowMessages = [
        "expired": "Your card has expired", "stolen": "Your card was reported stolen",
    ]

    /// Writes the outline, and the named plain scenario if one is given, with setups.
    func hasOutline(_ other: String) async throws {
        try writeOutline(named: true)
        if !other.isEmpty {
            try w.ensureScenario("checkout", other)
            try writeSetup(other, Self.script(#"ok "second card added""#))
        }
        try await prepare()
    }

    /// The selected outline's Drop in rows.
    func rows() async throws -> [DropInRowRecord] {
        let window = try await w.front()
        let rows = try await window.dropInRows()
        guard !rows.isEmpty else {
            throw StepFailure("the Drop in page lists no rows; it shows \(window.visible)")
        }
        return rows
    }

    /// The row whose card column is `card`.
    func row(_ card: String) async throws -> DropInRowRecord {
        let rows = try await rows()
        guard let row = rows.first(where: { $0.values["card"] == card }) else {
            throw StepFailure("no row has card=\(card): \(rows)")
        }
        return row
    }

    func rowsFilledIn(_ example: String) async throws {
        for row in try await rows() {
            if let step = row.steps.first(where: {
                $0.contains("<card>") || $0.contains("<message>")
            }) {
                throw StepFailure("the \(row.values) row shows an unfilled step \"\(step)\"")
            }
            let message = Self.rowMessages[row.values["card"] ?? ""] ?? ""
            let want = "Then they see \"\(message)\""
            guard row.steps.contains(want) else {
                throw StepFailure("the \(row.values) row doesn't show \(want): \(row.steps)")
            }
        }
        try expect(try await row("expired").steps, example, "the expired row's steps")
    }

    func outlineOpen(_ name: String) async throws {
        try writeOutline(named: true)
        try await prepare()
        try await w.front().select(name)
    }

    func chooseOnRow(_ action: String, _ row: String) async throws {
        try await w.front().chooseOnRow(row, action)
        if action == "Drop in" { try await settleTo(SessionModel.ready) }
    }

    func exampleCardIs(_ card: String) async throws {
        try await settleTo(SessionModel.ready)
        let got = shopFile("ex-card")
        guard got == card else {
            throw StepFailure("setup ran with GIVENS_EX_card=\(got ?? "unset"), not \(card)")
        }
    }

    func rowGreyedOut(_ card: String) async throws {
        let row = try await row(card)
        guard row.greyed && !row.steps.isEmpty else {
            throw StepFailure("the \(card) row isn't greyed out with its steps shown: \(row)")
        }
        guard row.disabled.contains("Drop in") && !row.offers.contains("Drop in") else {
            throw StepFailure("the \(card) row's \"Drop in\" isn't disabled: \(row)")
        }
    }

    /// Drops in to one row of the outline and marks its first Then step "pass".
    func rowMarkedOne(_ card: String) async throws {
        try await outlineOpen(Self.outlineName)
        try await chooseOnRow("Drop in", card)
        let session = try await session()
        try await session.mark(try dropInFirst(session.checklist, "then").text, "pass")
    }

    func leaveSwitches(_ from: String, _ to: String) async throws {
        try await w.front().choose("Leave")
        try await exampleCardIs(to)
        let left = try await row(from)
        guard !left.live && left.checklist.isEmpty else {
            throw StepFailure("the \(from) checklist is still open: \(left)")
        }
        try await rowChecklistLive(to)
    }

    /// Expects the row to be live, with a checklist for its own values.
    func rowChecklistLive(_ card: String) async throws {
        let row = try await row(card)
        guard row.live else { throw StepFailure("the \(card) row isn't live: \(row)") }
        let message = Self.rowMessages[card] ?? card
        try expect(row.checklist.map(\.text), message, "the \(card) row's checklist")
    }

    func rowAllPassed(_ name: String, _ card: String) async throws {
        try await outlineOpen(name)
        try await chooseOnRow("Drop in", card)
        try await dropInMarkAll(try await session())
    }
}
