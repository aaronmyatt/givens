import GivensUI
import SwiftUI

/// The right column for a scenario: its name and kind, its tags, its steps lined up, its
/// setup script with Copy and Drop in beside it, and its status at the foot.
struct ScenarioPage: View {
    let window: WindowModel
    let scenario: ScenarioRecord
    /// Opens the Drop in page without running anything: for the scenario, or one outline row.
    let onDropIn: (_ row: String?) -> Void

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 22) {
                header
                StepsGrid(steps: scenario.steps)
                rows
                StatusLine(status: scenario.status)
                setup
                history
            }
            .frame(maxWidth: 820, alignment: .leading)
            .padding(28)
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(alignment: .firstTextBaseline, spacing: 12) {
                Text(scenario.name).font(.title).bold()
                if scenario.error { Badge(text: "Error state", tint: Theme.error) }
                Spacer()
                // Copies the scenario itself; "Copy" by the setup script copies how to run it.
                Button {
                    window.app.copyText(WindowModel.gherkin(scenario))
                } label: {
                    Label("Copy steps", systemImage: "doc.on.doc")
                }
                .buttonStyle(.bordered)
                .help("Copy this scenario as Gherkin text")
                .keyboardShortcut("c", modifiers: [.command, .option])
                if window.offers.contains("Edit") {
                    Button {
                        window.app.run { _ in try await window.choose("Edit") }
                    } label: {
                        Label("Edit", systemImage: "square.and.pencil")
                    }
                    .labelStyle(.iconOnly)
                    .buttonStyle(.borderless)
                    .help("Edit")
                    .keyboardShortcut("e")
                }
            }
            WarningText(warning: scenario.warning)
            let tags = scenario.tags.filter { $0 != "@error" }
            if !tags.isEmpty {
                HStack(spacing: 6) { ForEach(tags, id: \.self) { Badge(text: $0) } }
            }
            if !scenario.rule.isEmpty {
                Text("Rule: \(scenario.rule)").foregroundStyle(.secondary)
            }
        }
    }

    /// An outline's Examples rows, each droppable on its own.
    @ViewBuilder private var rows: some View {
        let rows = window.snapshot?.rows ?? []
        if !rows.isEmpty {
            VStack(alignment: .leading, spacing: 10) {
                SectionLabel(text: "Examples")
                ForEach(Array(rows.enumerated()), id: \.offset) { _, row in
                    RowView(window: window, row: row, onDropIn: onDropIn)
                }
            }
        }
    }

    private var setup: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                SectionLabel(text: "Setup script")
                Spacer()
                ForEach(window.offers.filter(Placement.scenario.contains), id: \.self) {
                    offer in
                    if offer != "Edit" {
                        Button {
                            if offer == "Drop in" {
                                onDropIn(nil)
                                return
                            }
                            window.app.run(Feedback.after(offer)) { _ in
                                try await window.choose(offer)
                            }
                        } label: {
                            Label(offer, systemImage: Self.symbol(offer))
                        }
                        .buttonStyle(.bordered)
                        // ⌘↵ drops in, as in the search mockup's key hints.
                        // Ref: https://developer.apple.com/documentation/swiftui/view/keyboardshortcut(_:)-8liec
                        .keyboardShortcut(offer == "Drop in" ? Self.dropInKey : nil)
                    }
                }
            }
            if scenario.setup.isEmpty {
                Text(Self.noSetup).foregroundStyle(.secondary)
            } else {
                CodeBlock { Text(scenario.setup) }
            }
        }
    }

    /// internal/app's MessageNoSetup, shown in place of the script.
    static let noSetup = "No setup yet"

    /// ⌘↵, the Drop in shortcut.
    static let dropInKey = KeyboardShortcut(.return, modifiers: .command)

    /// Every recorded run, newest first, so what's been tried shows before dropping in.
    @ViewBuilder private var history: some View {
        let runs = window.snapshot?.runs ?? []
        VStack(alignment: .leading, spacing: 8) {
            SectionLabel(text: runs.isEmpty ? "No runs yet" : "Runs")
            ForEach(Array(runs.enumerated()), id: \.offset) { _, run in
                HStack(alignment: .firstTextBaseline, spacing: 8) {
                    Image(systemName: run.result == "pass" ? "checkmark.circle" : "xmark.circle")
                        .foregroundStyle(run.result == "pass" ? Theme.happy : Theme.error)
                        .accessibilityLabel(run.result == "pass" ? "Passed" : "Failed")
                    VStack(alignment: .leading, spacing: 2) {
                        Text(run.text)
                        if !run.note.isEmpty {
                            Text(run.note).foregroundStyle(.secondary).textSelection(.enabled)
                        }
                    }
                }
            }
        }
    }

    /// An SF Symbol for a scenario action. Ref: https://developer.apple.com/sf-symbols/
    static func symbol(_ offer: String) -> String {
        switch offer {
        case "Copy": "doc.on.doc"
        case "Drop in": "play.rectangle"
        case "Create script", "Write setup steps", "Add teardown": "plus"
        case "Edit script", "Edit setup", "Edit teardown": "pencil.and.list.clipboard"
        default: "pencil"
        }
    }
}

/// One Examples row of the selected outline: its values and status, its steps, and its
/// buttons; a row greyed out while another is live.
struct RowView: View {
    let window: WindowModel
    let row: DropInRowRecord
    /// Opens the Drop in page for this row, before anything runs.
    let onDropIn: (_ row: String?) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text(values).font(.system(.body, design: .monospaced)).bold()
                Spacer()
                StatusIcon(status: row.status)
                ForEach(row.offers, id: \.self) { offer in
                    Button(offer) {
                        if offer == "Drop in" {
                            onDropIn(name)
                        } else {
                            window.app.run { _ in try await window.chooseOnRow(name, offer) }
                        }
                    }
                }
                ForEach(row.disabled, id: \.self) { Button($0) {}.disabled(true) }
            }
            StepsGrid(steps: row.steps).font(.callout)
        }
        .padding(12)
        .background(Theme.codeFill, in: RoundedRectangle(cornerRadius: 10))
        .overlay(
            RoundedRectangle(cornerRadius: 10)
                .stroke(row.live ? Theme.keyword : Theme.hairline)
        )
        .opacity(row.greyed ? 0.45 : 1)
        .help(WindowModel.rowLine(row))
    }

    /// The row's values in column order: "card=expired, amount=5".
    private var values: String {
        row.values.keys.sorted().map { "\($0)=\(row.values[$0] ?? "")" }
            .joined(separator: ", ")
    }

    /// The row as ChooseOnRow names it: its first column's value, like "card=expired".
    private var name: String {
        guard let column = row.values.keys.sorted().first else { return "" }
        return "\(column)=\(row.values[column] ?? "")"
    }
}
