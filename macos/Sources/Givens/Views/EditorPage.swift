import GivensUI
import SwiftUI

/// The editor: the file's whole text, comments and all, saved as typed. Steps already in the
/// project are offered under the text while a step line is typed, with "used in N"; the
/// preview sits beside the text once chosen; problems sit in the bar at the foot.
/// Ref: https://developer.apple.com/documentation/swiftui/texteditor
struct EditorPage: View {
    let window: WindowModel
    let editor: EditorRecord
    @State private var text = ""
    /// The text last sent to the core, so its echo in the next snapshot isn't taken for a
    /// change from elsewhere (a save, "Use theirs").
    @State private var sent = ""
    /// The line last typed on, and the project's steps that start like it.
    @State private var typing: Int?
    @State private var suggestions: [StepSuggestionRecord] = []

    var body: some View {
        VStack(spacing: 0) {
            header
            Divider()
            HStack(spacing: 0) {
                source
                if !editor.preview.isEmpty {
                    Divider()
                    preview
                }
            }
            Divider()
            footer
        }
    }

    private var header: some View {
        HStack(spacing: 10) {
            Text(editor.path).font(.headline).lineLimit(1).truncationMode(.head)
            if editor.dirty || text != sent { Badge(text: "Edited", tint: .orange) }
            Spacer()
            OfferButtons(window: window, labels: ["Keep mine", "Use theirs", "Compare"])
            if window.offers.contains("Preview") {
                // On while the preview shows; the core toggles it.
                Toggle(
                    isOn: Binding(
                        get: { !editor.preview.isEmpty },
                        set: { _ in
                            window.app.run { _ in
                                try await sync()
                                try await window.choose("Preview")
                            }
                        })
                ) {
                    Label("Preview", systemImage: "eye")
                }
                .toggleStyle(.button)
            }
            if window.offers.contains("Close") {
                // With unsaved changes the core asks first, in the banner above.
                Button {
                    window.app.run { _ in
                        try await sync()
                        try await window.choose("Close")
                    }
                } label: {
                    Label("Close", systemImage: "xmark")
                }
                .help("Close the editor")
            }
            Button("Save") {
                window.app.run { _ in
                    try await sync()
                    try await window.saveEditor()
                    // A clash with changes on disk stays unsaved, with its problem below.
                    let problems = window.snapshot?.editor?.problems ?? []
                    if problems.isEmpty {
                        let name = URL(fileURLWithPath: editor.path).lastPathComponent
                        Feedback.shared.say("Saved \(name)", symbol: "square.and.arrow.down.fill")
                    } else {
                        Feedback.shared.say(
                            "Not saved: \(problems[0])", symbol: "exclamationmark.triangle.fill",
                            failure: true)
                    }
                }
            }
            .buttonStyle(.borderedProminent)
            .keyboardShortcut("s")
        }
        .padding(.horizontal, 20)
        .padding(.vertical, 12)
        .background(.bar)
    }

    private var source: some View {
        VStack(alignment: .leading, spacing: 0) {
            TextEditor(text: $text)
                .font(.system(.body, design: .monospaced))
                .scrollContentBackground(.hidden)
                .padding(16)
                .onAppear {
                    text = editor.text
                    sent = editor.text
                }
                .onChange(of: editor.text) { _, held in
                    // Only a change made elsewhere replaces what's typed.
                    guard held != sent else { return }
                    text = held
                    sent = held
                }
                .onChange(of: text) { old, new in
                    typing = WindowModel.changedLine(from: old, to: new)
                }
                // Sends the text after a pause, so Preview and "unsaved changes" see it.
                .task(id: text) {
                    try? await Task.sleep(for: .milliseconds(300))
                    guard !Task.isCancelled, text != sent else { return }
                    try? await sync()
                }
                // .task(id:) cancels the last lookup when the line changes, so a pause in
                // typing is what asks.
                // Ref: https://developer.apple.com/documentation/swiftui/view/task(id:priority:_:)
                .task(id: typingLine) {
                    try? await Task.sleep(for: .milliseconds(150))
                    guard !Task.isCancelled else { return }
                    suggestions = (try? await window.stepSuggestions(forLine: typingLine)) ?? []
                }
            if !suggestions.isEmpty { suggestionList }
        }
    }

    /// The offered steps, like the mockup's popup: the step, then how widely it's used.
    private var suggestionList: some View {
        VStack(alignment: .leading, spacing: 0) {
            ForEach(suggestions, id: \.text) { suggestion in
                Button {
                    guard let typing else { return }
                    text = WindowModel.withSuggestion(text, line: typing, step: suggestion.text)
                    suggestions = []
                } label: {
                    HStack {
                        Text(suggestion.text).font(.system(.body, design: .monospaced))
                        Spacer()
                        Text(suggestion.label).foregroundStyle(.secondary)
                    }
                    .contentShape(Rectangle())
                    .padding(.horizontal, 12)
                    .padding(.vertical, 7)
                }
                .buttonStyle(.plain)
            }
        }
        .background(.background, in: RoundedRectangle(cornerRadius: 10))
        .overlay(RoundedRectangle(cornerRadius: 10).stroke(Theme.hairline))
        .shadow(color: .black.opacity(0.08), radius: 8, y: 2)
        .padding([.horizontal, .bottom], 16)
    }

    private var preview: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 6) {
                SectionLabel(text: "Preview")
                ForEach(Array(editor.preview.enumerated()), id: \.offset) { _, line in
                    Text(line).font(.system(.body, design: .monospaced))
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(16)
        }
        .frame(minWidth: 260)
    }

    private var footer: some View {
        HStack {
            if editor.problems.isEmpty {
                Label("No problems", systemImage: "checkmark.circle")
                    .foregroundStyle(Theme.happy)
            } else {
                VStack(alignment: .leading) {
                    ForEach(editor.problems, id: \.self) { WarningText(warning: $0) }
                }
            }
            Spacer()
        }
        .font(.callout)
        .padding(.horizontal, 20)
        .padding(.vertical, 8)
        .background(.bar)
    }

    /// Sends the text as typed to the core.
    private func sync() async throws {
        guard text != sent else { return }
        let typed = text
        sent = typed
        try await window.setEditorText(typed)
    }

    /// The text of the line last typed on, or "" before any typing.
    private var typingLine: String {
        typing.map { WindowModel.line($0, of: text) } ?? ""
    }
}
