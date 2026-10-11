import GivensUI
import SwiftUI

/// Shows the front window's model: the welcome window or a project. Each view reads only the
/// model's snapshot fields, the same ones WindowModel.visible lists.
struct RootView: View {
    let app: AppModel

    var body: some View {
        if let window = app.front {
            switch window.kind {
            case "project": ProjectView(window: window)
            default: WelcomeView(app: app, window: window)
            }
        } else {
            ProgressView()
        }
    }
}

/// "Open folder…" and the recent projects.
struct WelcomeView: View {
    let app: AppModel
    let window: WindowModel

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            ForEach(window.snapshot?.messages ?? [], id: \.self) { Text($0) }
            ForEach(window.offers, id: \.self) { offer in
                Button(offer) { app.run { _ in try await window.choose(offer) } }
            }
            if !app.recents.isEmpty {
                Text("Recent").font(.headline)
                ForEach(app.recents, id: \.self) { path in
                    Button(path) { app.run { _ in try await window.openRecent(path) } }
                }
            }
        }
        .padding()
    }
}

/// A project: sidebar on the left, the selected capability or scenario on the right.
/// Ref: https://developer.apple.com/documentation/swiftui/navigationsplitview
struct ProjectView: View {
    let window: WindowModel

    var body: some View {
        NavigationSplitView {
            List(window.sidebar, id: \.name) { item in
                Button {
                    window.app.run { _ in try await window.select(item.name) }
                } label: {
                    HStack {
                        Text(item.name)
                        if !item.warning.isEmpty {
                            Image(systemName: "exclamationmark.triangle")
                                .foregroundStyle(.orange)
                                .accessibilityLabel(item.warning)
                        }
                        Spacer()
                        if item.count > 0 { Text("\(item.count)").foregroundStyle(.secondary) }
                    }
                }
                .help(item.warning)
            }
        } detail: {
            DetailView(window: window)
        }
        .navigationTitle(window.title)
    }
}

/// Messages, the shown capability or scenario, the editor, the Drop in page and the offered
/// actions. Each piece of text drawn here is one WindowModel.visible lists, in the same order.
struct DetailView: View {
    let window: WindowModel

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 8) {
                ForEach(window.snapshot?.messages ?? [], id: \.self) { Text($0).bold() }
                ForEach(window.snapshot?.labels ?? [], id: \.self) {
                    Text($0).foregroundStyle(.secondary)
                }
                if let capability = window.snapshot?.capability {
                    CapabilityView(capability: capability)
                }
                if let scenario = window.snapshot?.scenario {
                    ScenarioView(scenario: scenario)
                }
                if let editor = window.snapshot?.editor {
                    EditorView(window: window, editor: editor)
                }
                if let session = window.snapshot?.session {
                    ForEach(session.output, id: \.self) { Text($0).monospaced() }
                    ForEach(session.checklist, id: \.text) { Text("\($0.mark) \($0.text)") }
                }
                HStack {
                    ForEach(window.offers, id: \.self) { offer in
                        Button(offer) { window.app.run { _ in try await window.choose(offer) } }
                    }
                }
            }
            .padding()
        }
    }
}

/// A warning line, or nothing when there's none.
struct WarningText: View {
    let warning: String

    var body: some View {
        if !warning.isEmpty {
            Label(warning, systemImage: "exclamationmark.triangle").foregroundStyle(.orange)
        }
    }
}

/// The middle column: the story, about.md, and the scenarios grouped under their Rule: blocks.
/// Ref: https://cucumber.io/docs/gherkin/reference/#rule
struct CapabilityView: View {
    let capability: CapabilityRecord

    var body: some View {
        Text(capability.name).font(.title)
        WarningText(warning: capability.warning)
        ForEach(capability.story, id: \.self) { Text($0) }
        if !capability.about.isEmpty { Text(capability.about) }
        ForEach(Array(capability.scenarios.enumerated()), id: \.offset) { index, scenario in
            let previous = index == 0 ? "" : capability.scenarios[index - 1].rule
            if scenario.rule != previous && !scenario.rule.isEmpty {
                Text(scenario.rule).font(.headline).padding(.top, 4)
            }
            HStack {
                Image(systemName: scenario.error ? "xmark.octagon" : "checkmark.circle")
                Text(scenario.name)
                Text(scenario.status).foregroundStyle(.secondary)
            }
            WarningText(warning: scenario.warning)
        }
    }
}

/// The right-hand side: a scenario's tags above its steps.
struct ScenarioView: View {
    let scenario: ScenarioRecord

    var body: some View {
        Text(scenario.name).font(.title2)
        if !scenario.status.isEmpty { Text(scenario.status).foregroundStyle(.secondary) }
        WarningText(warning: scenario.warning)
        Text(scenario.tags.joined(separator: " ")).foregroundStyle(.secondary)
        ForEach(scenario.steps, id: \.self) { Text($0).monospaced() }
    }
}

/// The editor: the file's whole text, comments and all, saved as typed.
/// Ref: https://developer.apple.com/documentation/swiftui/texteditor
struct EditorView: View {
    let window: WindowModel
    let editor: EditorRecord
    @State private var text = ""

    var body: some View {
        Text(editor.path).font(.caption).foregroundStyle(.secondary)
        TextEditor(text: $text)
            .font(.body.monospaced())
            .frame(minHeight: 240)
            .onAppear { text = editor.text }
            .onChange(of: editor.text) { _, saved in text = saved }
        ForEach(editor.problems, id: \.self) { WarningText(warning: $0) }
        Button("Save") {
            window.app.run { _ in
                try await window.setEditorText(text)
                try await window.saveEditor()
            }
        }
        .keyboardShortcut("s")
    }
}
