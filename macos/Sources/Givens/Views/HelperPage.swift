import GivensUI
import SwiftUI

/// Loads the Shared list's full entries, with each helper's code, and hands them to `content`.
/// They're loaded again whenever the sidebar changes, which it does when files change.
struct SharedHelpers<Content: View>: View {
    let window: WindowModel
    @ViewBuilder let content: ([SharedHelperRecord]) -> Content
    @State private var helpers: [SharedHelperRecord] = []

    var body: some View {
        content(helpers)
            // Ref: https://developer.apple.com/documentation/swiftui/view/task(id:priority:_:)
            .task(id: window.sidebar) {
                helpers = (try? await window.helpers()) ?? []
            }
    }
}

/// The right column for a helper: how it's called, what it does, where it lives, who calls
/// it, and its code, with Edit to change it.
struct HelperPage: View {
    let window: WindowModel
    let helper: SharedHelperRecord

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                HStack(alignment: .firstTextBaseline) {
                    Text(helper.name).font(.system(.title, design: .monospaced)).bold()
                    Badge(text: helper.capability.isEmpty ? "Every capability" : helper.capability)
                    Spacer()
                    if window.offers.contains("Edit") {
                        Button {
                            window.app.run { _ in try await window.choose("Edit") }
                        } label: {
                            Label("Edit", systemImage: "pencil")
                        }
                        .buttonStyle(.bordered)
                        .keyboardShortcut("e")
                    }
                }
                Text(helper.signature).font(.system(.body, design: .monospaced))
                    .foregroundStyle(Theme.keyword)
                if !helper.description.isEmpty { Text(helper.description) }
                WarningText(warning: helper.warning)
                if !helper.overrides.isEmpty {
                    Text(helper.overrides).foregroundStyle(.secondary)
                }
                VStack(alignment: .leading, spacing: 8) {
                    SectionLabel(text: "\(helper.definedIn), line \(helper.line)")
                    CodeBlock { Text(helper.code.isEmpty ? "Couldn't read it" : helper.code) }
                }
                VStack(alignment: .leading, spacing: 6) {
                    SectionLabel(
                        text: helper.callers.isEmpty
                            ? "No setup script calls it" : "Called by \(helper.callers.count)")
                    ForEach(helper.callers, id: \.self) { caller in
                        Label(caller, systemImage: "apple.terminal")
                            .font(.system(.callout, design: .monospaced))
                    }
                }
            }
            .frame(maxWidth: 820, alignment: .leading)
            .padding(28)
        }
    }
}
