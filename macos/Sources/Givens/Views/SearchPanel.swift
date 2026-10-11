import GivensUI
import SwiftUI

/// The ⌘K results, floating over the detail column as in the mockup: kind chips, results
/// grouped by kind with the matched words highlighted, and the keys at the foot. The rows come
/// from window.searchResults(), the grouped form of the snapshot's result lines.
struct SearchPanel: View {
    let window: WindowModel
    let search: SearchRecord
    @State private var results: SearchResultsRecord?
    @State private var query = ""
    @FocusState private var focused: Bool

    static let kinds = ["Stories", "Scenarios", "Steps", "Scripts", "Helpers"]

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            field
            chips
            Divider()
            ScrollView {
                VStack(alignment: .leading, spacing: 4) {
                    ForEach(results?.groups ?? [], id: \.kind) { group in
                        SectionLabel(text: group.kind).padding(.top, 10).padding(.leading, 8)
                        ForEach(Array(group.results.enumerated()), id: \.offset) { _, result in
                            SearchRow(kind: group.kind, result: result) { open(result) }
                        }
                    }
                }
                .padding(10)
            }
            .frame(maxHeight: 420)
            Divider()
            footer
        }
        .frame(maxWidth: 720)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 14))
        .overlay(RoundedRectangle(cornerRadius: 14).stroke(Theme.hairline))
        .shadow(color: .black.opacity(0.15), radius: 20, y: 8)
        // The snapshot's lines change whenever the query or filter does; fetch the groups then.
        // Ref: https://developer.apple.com/documentation/swiftui/view/task(id:priority:_:)
        .task(id: search.results) {
            results = try? await window.searchResults()
        }
    }

    /// The query box. Each change sends the whole query again: ⌘K starts it empty, then the
    /// text is typed. Return opens the best result; Esc closes the panel.
    /// Ref: https://developer.apple.com/documentation/swiftui/focusstate
    private var field: some View {
        HStack(spacing: 10) {
            Image(systemName: "magnifyingglass").font(.title3).foregroundStyle(.secondary)
            TextField("Search stories, scenarios, steps, scripts and helpers", text: $query)
                .textFieldStyle(.plain)
                .font(.title3)
                .focused($focused)
                .onSubmit {
                    if let best = results?.groups.first?.results.first { open(best) }
                }
                .onExitCommand { window.app.run { _ in try await window.press("⎋") } }
        }
        .padding(.horizontal, 14)
        .padding(.top, 14)
        .onAppear {
            query = search.query
            focused = true
        }
        // Waits for a pause in typing, then sends only what changed: added letters are typed
        // on in one call; anything else starts the query again with ⌘K, which clears the kind
        // filter, so a chosen chip is put back.
        // Ref: https://developer.apple.com/documentation/swiftui/view/task(id:priority:_:)
        .task(id: query) {
            try? await Task.sleep(for: .milliseconds(120))
            let typed = query
            let sent = search.query
            guard !Task.isCancelled, typed != sent else { return }
            let filter = results?.filter ?? ""
            do {
                if !sent.isEmpty && typed.hasPrefix(sent) {
                    try await window.type(String(typed.dropFirst(sent.count)))
                } else {
                    try await window.press("⌘K")
                    if !typed.isEmpty { try await window.type(typed) }
                    if !filter.isEmpty { try await window.filterSearch(filter) }
                }
            } catch {
                Feedback.shared.fail(error)
            }
        }
    }

    private var chips: some View {
        HStack(spacing: 8) {
            ForEach(["All"] + Self.kinds, id: \.self) { kind in
                let chosen = (results?.filter ?? "") == (kind == "All" ? "" : kind)
                Button(kind) {
                    window.app.run { _ in try await window.filterSearch(kind == "All" ? "" : kind) }
                }
                .buttonStyle(.bordered)
                .buttonBorderShape(.capsule)
                .tint(chosen ? Theme.keyword : nil)
            }
            Spacer()
            let count = (results?.groups ?? []).reduce(0) { $0 + $1.results.count }
            Text("\(count) results").foregroundStyle(.secondary)
        }
        .padding(12)
    }

    private var footer: some View {
        HStack(spacing: 16) {
            Text("Click to open").foregroundStyle(.secondary)
            Button("Copy setup") { window.app.run { _ in try await window.press("⌘C") } }
                .buttonStyle(.borderless)
            Spacer()
            Button("Close") { window.app.run { _ in try await window.press("⎋") } }
                .buttonStyle(.borderless)
                .keyboardShortcut(.cancelAction)
        }
        .font(.callout)
        .padding(.horizontal, 14)
        .padding(.vertical, 8)
    }

    /// Opens a result by its name, then closes search.
    private func open(_ result: SearchResultRecord) {
        window.app.run { _ in
            try await window.select(result.name)
            try await window.press("⎋")
        }
    }
}

/// One search result: an icon for its kind, its name with the matches highlighted, and its
/// capability or how many scenarios use it.
struct SearchRow: View {
    let kind: String
    let result: SearchResultRecord
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 10) {
                Image(systemName: symbol).foregroundStyle(.secondary).frame(width: 18)
                VStack(alignment: .leading, spacing: 2) {
                    Text(Self.highlighted(result.name, result.matches))
                        .font(kind == "Scripts" ? .system(.body, design: .monospaced) : .body)
                    if !result.detail.isEmpty {
                        Text(Self.highlighted(result.detail, result.matches))
                            .font(.callout)
                            .foregroundStyle(.secondary)
                            .lineLimit(1)
                    }
                }
                Spacer()
                Text(result.uses > 0 ? "\(result.uses) uses" : result.capability)
                    .foregroundStyle(.secondary)
            }
            .contentShape(Rectangle())
            .padding(.horizontal, 8)
            .padding(.vertical, 6)
        }
        .buttonStyle(.plain)
    }

    private var symbol: String {
        switch kind {
        case "Stories": "book"
        case "Scenarios": "checklist"
        case "Steps": "quote.opening"
        case "Scripts": "apple.terminal"
        default: "function"
        }
    }

    /// The text with each match's first occurrence on a soft orange background.
    /// Ref: https://developer.apple.com/documentation/foundation/attributedstring
    static func highlighted(_ text: String, _ matches: [String]) -> AttributedString {
        var styled = AttributedString(text)
        for match in matches where !match.isEmpty {
            if let range = styled.range(of: match, options: .caseInsensitive) {
                styled[range].backgroundColor = .orange.opacity(0.25)
            }
        }
        return styled
    }
}
