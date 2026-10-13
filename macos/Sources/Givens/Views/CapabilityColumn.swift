import GivensUI
import SwiftUI

/// The middle column for a capability: its story above its scenarios, grouped under their
/// Rule: blocks, then its about.md, rendered, so a long one doesn't push the scenarios off the
/// screen. Each scenario shows a happy or error dot and its status as an
/// icon; the status's words are in its tooltip, so the list stays short and calm.
/// Ref: https://cucumber.io/docs/gherkin/reference/#rule
struct CapabilityColumn: View {
    let window: WindowModel
    let capability: CapabilityRecord

    var body: some View {
        List(selection: selection) {
            Section {
                StoryHeader(window: window, capability: capability)
                    .listRowSeparator(.hidden)
                    .selectionDisabled()
            }
            ForEach(Array(groups.enumerated()), id: \.offset) { index, group in
                Section {
                    ForEach(group.scenarios, id: \.name) { scenario in
                        ScenarioListRow(window: window, scenario: scenario)
                            .tag(scenario.name)
                    }
                } header: {
                    if index == 0 && group.rule.isEmpty {
                        Text("Scenarios")
                    } else {
                        Text(group.rule)
                    }
                }
            }
            if !capability.about.isEmpty {
                Section {
                    // Images are read from the capability's own folder, as about.md links them.
                    MarkdownView(
                        text: capability.about,
                        resolve: MarkdownView.relative(
                            to: URL(fileURLWithPath: window.path)
                                .appendingPathComponent(capability.slug).path)
                    )
                    .padding(.vertical, 8)
                    .listRowSeparator(.hidden)
                    .selectionDisabled()
                } header: {
                    Text("About")
                }
            }
            Section {
                Legend().listRowSeparator(.hidden).selectionDisabled()
            }
        }
        .listStyle(.inset)
    }

    /// The scenario on show, as the list's selection.
    private var selection: Binding<String?> {
        Binding(
            get: { window.snapshot?.scenario?.name },
            set: { name in
                guard let name, name != window.snapshot?.scenario?.name else { return }
                window.app.run { _ in try await window.select(name) }
            })
    }

    /// Runs of scenarios under the same Rule: block, in file order.
    private var groups: [(rule: String, scenarios: [ScenarioRecord])] {
        var groups: [(rule: String, scenarios: [ScenarioRecord])] = []
        for scenario in capability.scenarios {
            if let last = groups.last, last.rule == scenario.rule {
                groups[groups.count - 1].scenarios.append(scenario)
            } else {
                groups.append((scenario.rule, [scenario]))
            }
        }
        return groups
    }
}

/// A capability's name, with "Edit about.md" beside it, and its story with the As a, I want
/// and So that words greyed.
struct StoryHeader: View {
    let window: WindowModel
    let capability: CapabilityRecord

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text(capability.name).font(.title2).bold()
                Spacer()
                // "Edit about.md", or "Add about.md" when there's none: opens the editor.
                ForEach(window.offers.filter(Placement.story.contains), id: \.self) { offer in
                    Button(offer, systemImage: "doc.richtext") {
                        window.app.run { _ in try await window.choose(offer) }
                    }
                    .buttonStyle(.borderless)
                }
            }
            WarningText(warning: capability.warning)
            VStack(alignment: .leading, spacing: 4) {
                ForEach(capability.story, id: \.self) { line in
                    Self.storyLine(line).font(.title3)
                }
            }
        }
        .padding(.vertical, 8)
    }

    /// "As a returning shopper" with "As a" in grey.
    /// Ref: https://developer.apple.com/documentation/swiftui/text/foregroundstyle(_:)
    static func storyLine(_ line: String) -> Text {
        for lead in ["As a", "As an", "I want", "So that"] where line.hasPrefix(lead + " ") {
            let rest = String(line.dropFirst(lead.count))
            return Text(lead).foregroundStyle(.secondary) + Text(rest)
        }
        return Text(line)
    }
}

/// One scenario in the middle column: its kind dot, name and status icon.
struct ScenarioListRow: View {
    let window: WindowModel
    let scenario: ScenarioRecord

    var body: some View {
        HStack(spacing: 10) {
            KindDot(error: scenario.error)
            Text(scenario.name).lineLimit(2)
            Spacer(minLength: 8)
            if !scenario.warning.isEmpty {
                Image(systemName: "exclamationmark.triangle")
                    .foregroundStyle(.orange)
                    .help(scenario.warning)
                    .accessibilityLabel(scenario.warning)
            }
            StatusIcon(status: scenario.status)
        }
        .padding(.vertical, 3)
        .contextMenu {
            Button("Delete…") {
                window.app.run { _ in try await window.delete(scenario.name) }
            }
        }
    }
}

/// What the dots mean.
struct Legend: View {
    var body: some View {
        HStack(spacing: 6) {
            KindDot(error: false)
            Text("Happy path")
            KindDot(error: true).padding(.leading, 8)
            Text("Error state")
        }
        .font(.caption)
        .foregroundStyle(.secondary)
    }
}

/// The middle column for a view, tag or step: the scenarios it lists, with their capability.
struct ListedColumn: View {
    let window: WindowModel
    let title: String

    var body: some View {
        List(selection: selection) {
            Section {
                Text(title).font(.title2).bold().selectionDisabled()
            }
            Section {
                ForEach(window.snapshot?.listed ?? [], id: \.name) { scenario in
                    VStack(alignment: .leading, spacing: 2) {
                        ScenarioListRow(window: window, scenario: scenario)
                        Text(scenario.capability)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .padding(.leading, 18)
                    }
                    .tag(scenario.name)
                }
            }
        }
        .listStyle(.inset)
    }

    private var selection: Binding<String?> {
        Binding(
            get: { window.snapshot?.scenario?.name },
            set: { name in
                guard let name else { return }
                window.app.run { _ in try await window.select(name) }
            })
    }
}

/// The middle column for Shared: every documented helper, by name, with its signature.
struct HelpersColumn: View {
    let window: WindowModel
    let helpers: [SharedHelperRecord]

    var body: some View {
        List(selection: selection) {
            Section {
                Text("Shared").font(.title2).bold().selectionDisabled()
            }
            Section("Helpers") {
                ForEach(helpers, id: \.name) { helper in
                    VStack(alignment: .leading, spacing: 2) {
                        HStack {
                            Image(systemName: "function").foregroundStyle(.secondary)
                            Text(helper.name).font(.system(.body, design: .monospaced))
                            Spacer()
                            if !helper.warning.isEmpty {
                                Image(systemName: "exclamationmark.triangle")
                                    .foregroundStyle(.orange)
                                    .help(helper.warning)
                            }
                        }
                        Text(helper.capability.isEmpty ? "Every capability" : helper.capability)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                    .tag(helper.name)
                }
            }
        }
        .listStyle(.inset)
    }

    private var selection: Binding<String?> {
        Binding(
            get: { window.selected },
            set: { name in
                guard let name, name != window.selected else { return }
                window.app.run { _ in try await window.select(name) }
            })
    }
}
