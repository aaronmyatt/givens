import AppKit
import GivensUI
import SwiftUI

/// Draws Markdown as the app reads it: headings sized, lists with their marks, quotes with a
/// bar, code in a code block, images from disk. Blocks come from GivensUI's Markdown reader;
/// inline parts (bold, code, links) from AttributedString(markdown:).
/// Ref: https://developer.apple.com/documentation/foundation/attributedstring/init(markdown:options:baseurl:)
struct MarkdownView: View {
    let text: String
    /// Finds an image's file from the path written in the text, or nil when there's none.
    let resolve: (String) -> URL?

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            ForEach(Array(Markdown.blocks(text).enumerated()), id: \.offset) { _, block in
                view(of: block)
            }
        }
        .textSelection(.enabled)
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    @ViewBuilder private func view(of block: MarkdownBlock) -> some View {
        switch block {
        case .heading(let level, let text):
            Self.inline(text).font(Self.headingFont(level)).bold().padding(.top, 4)
        case .paragraph(let text):
            Self.inline(text)
        case .bullets(let items):
            list(items) { _ in "•" }
        case .numbered(let items):
            list(items) { "\($0 + 1)." }
        case .quote(let text):
            Self.inline(text)
                .foregroundStyle(.secondary)
                .padding(.leading, 10)
                .overlay(alignment: .leading) {
                    Rectangle().fill(Theme.hairline).frame(width: 3)
                }
        case .code(_, let code):
            CodeBlock { Text(code) }
        case .image(let alt, let path):
            image(alt: alt, path: path)
        case .rule:
            Divider()
        }
    }

    private func list(_ items: [String], mark: @escaping (Int) -> String) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            ForEach(Array(items.enumerated()), id: \.offset) { index, item in
                HStack(alignment: .firstTextBaseline, spacing: 6) {
                    Text(mark(index)).foregroundStyle(.secondary)
                    Self.inline(item)
                }
            }
        }
    }

    /// An image read from disk, as wide as it is up to the column; a missing one is named.
    @ViewBuilder private func image(alt: String, path: String) -> some View {
        if let url = resolve(path), let loaded = NSImage(contentsOf: url) {
            Image(nsImage: loaded)
                .resizable()
                .scaledToFit()
                .frame(maxWidth: min(loaded.size.width, 560), alignment: .leading)
                .accessibilityLabel(alt)
        } else {
            Label("Image not found: \(path)", systemImage: "photo")
                .foregroundStyle(.secondary)
        }
    }

    /// Inline Markdown in one Text; text that doesn't parse is shown as written.
    static func inline(_ text: String) -> Text {
        let options = AttributedString.MarkdownParsingOptions(
            interpretedSyntax: .inlineOnlyPreservingWhitespace)
        let parsed = (try? AttributedString(markdown: text, options: options))
        return Text(parsed ?? AttributedString(text))
    }

    static func headingFont(_ level: Int) -> Font {
        switch level {
        case 1: .title
        case 2: .title2
        case 3: .title3
        default: .headline
        }
    }

    /// Resolves image paths against `folder`; links to the web aren't loaded.
    static func relative(to folder: String) -> (String) -> URL? {
        { path in
            guard !path.contains("://"), !folder.isEmpty else { return nil }
            return URL(fileURLWithPath: folder).appendingPathComponent(path)
        }
    }
}
