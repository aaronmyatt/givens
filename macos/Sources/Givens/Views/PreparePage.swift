import GivensUI
import SwiftUI

/// A Drop in waiting for its Run: the scenario or row chosen, nothing run yet.
struct StagedDropIn: Equatable {
    /// The scenario dropped in to.
    let scenario: String
    /// The outline row, as ChooseOnRow names it, or nil for a plain scenario.
    let row: String?
}

/// The Drop in page before setup runs. Nothing runs until "Run setup" is pressed, so visiting
/// is safe; the When and Then parts stay greyed out until setup is done.
struct PreparePage: View {
    let window: WindowModel
    let scenario: ScenarioRecord
    let staged: StagedDropIn
    let onBack: () -> Void

    var body: some View {
        VStack(spacing: 0) {
            header
            Divider()
            ScrollView {
                VStack(alignment: .leading, spacing: 0) {
                    given
                    Divider()
                    waiting(number: "2", title: "When · your turn", kinds: ["When"])
                    Divider()
                    waiting(number: "3", title: "Then · what should be true", kinds: ["Then"])
                }
                .frame(maxWidth: 860, alignment: .leading)
            }
        }
    }

    private var header: some View {
        HStack(spacing: 10) {
            Button(action: onBack) {
                Label("Back to the scenario", systemImage: "chevron.left")
            }
            .labelStyle(.iconOnly)
            .buttonStyle(.borderless)
            .help("Back to the scenario")
            .keyboardShortcut("[", modifiers: .command)
            Text(scenario.capability).foregroundStyle(.secondary)
            Image(systemName: "chevron.right").foregroundStyle(.tertiary)
            Button(scenario.name, action: onBack).buttonStyle(.borderless).font(.title3.bold())
            Badge(text: "Drop in", tint: Theme.keyword)
            if let row = staged.row { Badge(text: row) }
            Spacer()
        }
        .padding(.horizontal, 24)
        .padding(.vertical, 14)
        .background(.bar)
    }

    private var given: some View {
        VStack(alignment: .leading, spacing: 14) {
            StepHeading(number: "1", title: "Given", detail: "not set up yet", done: false)
            StepsGrid(steps: Self.steps(scenario.steps, kinds: ["Given"]))
            if scenario.setup.isEmpty {
                Text("This scenario has no setup, so there's nothing to run first.")
                    .foregroundStyle(.secondary)
            } else {
                CodeBlock { Text(scenario.setup) }
            }
            HStack {
                Button {
                    window.app.run { _ in
                        if let row = staged.row {
                            try await window.chooseOnRow(row, "Drop in")
                        } else {
                            try await window.choose("Drop in")
                        }
                    }
                } label: {
                    Label(
                        scenario.setup.isEmpty ? "Start" : "Run setup", systemImage: "play.fill")
                }
                .buttonStyle(.borderedProminent)
                .controlSize(.large)
                .keyboardShortcut(.return, modifiers: .command)
                Text("⌘↵").foregroundStyle(.secondary)
            }
        }
        .padding(24)
    }

    /// One later part, shown but greyed out until setup has run.
    private func waiting(number: String, title: String, kinds: Set<String>) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            StepHeading(number: number, title: title, detail: "after setup", done: false)
            StepsGrid(steps: Self.steps(scenario.steps, kinds: kinds))
        }
        .padding(24)
        .opacity(0.4)
        .help("Run setup first")
    }

    /// The steps of the given kinds, an And or But counting as the kind before it.
    static func steps(_ steps: [String], kinds: Set<String>) -> [String] {
        var kind = "Given"
        return steps.filter { step in
            let keyword = Theme.split(step).keyword
            if ["Given", "When", "Then"].contains(keyword) { kind = keyword }
            return kinds.contains(kind)
        }
    }
}
