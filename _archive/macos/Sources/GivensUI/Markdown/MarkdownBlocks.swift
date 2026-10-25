import Foundation

/// One block of a Markdown text, as the app draws it: a run note, or a capability's about.md.
/// Inline parts (bold, code, links) stay in the text; AttributedString(markdown:) reads those.
/// Ref: https://spec.commonmark.org/0.31.2/#blocks-and-inlines
public enum MarkdownBlock: Equatable, Sendable {
    /// `## Retry button`: its level, 1 to 6, and its text.
    case heading(level: Int, text: String)
    /// Lines of text up to a blank line, joined with spaces.
    case paragraph(String)
    /// `- item` or `* item` lines, each item's text.
    case bullets([String])
    /// `1. item` lines, each item's text.
    case numbered([String])
    /// `> quoted` lines, joined with spaces.
    case quote(String)
    /// A fenced code block: its language, which may be "", and its code.
    case code(language: String, code: String)
    /// `![alt](path)` on a line of its own.
    case image(alt: String, path: String)
    /// `---` or `***` on a line of its own.
    case rule
}

/// Reads Markdown into blocks, a line at a time. It covers what notes and about.md files use,
/// not all of CommonMark: no nested lists, tables or HTML.
public enum Markdown {
    /// The most lines read; the rest are left out. A note is 64 KB at most.
    public static let linesMax = 10_000

    /// The blocks of `text`, in order.
    public static func blocks(_ text: String) -> [MarkdownBlock] {
        var reader = BlockReader()
        let lines = text.components(separatedBy: "\n").prefix(linesMax)
        for line in lines { reader.read(line) }
        reader.finish()
        precondition(reader.fence == nil, "a fence is closed at the end")
        return reader.blocks
    }

    /// `![alt](path)` alone on a line gives its alt text and path.
    static func image(_ line: String) -> (alt: String, path: String)? {
        guard line.hasPrefix("!["), line.hasSuffix(")"),
            let close = line.range(of: "](")
        else { return nil }
        let alt = String(line[line.index(line.startIndex, offsetBy: 2)..<close.lowerBound])
        let path = String(line[close.upperBound..<line.index(before: line.endIndex)])
        return path.isEmpty || path.contains(" ") ? nil : (alt, path)
    }

    /// `## text` gives level 2 and "text"; a line that isn't a heading gives nil.
    static func heading(_ line: String) -> (level: Int, text: String)? {
        let marks = line.prefix { $0 == "#" }.count
        guard (1...6).contains(marks) else { return nil }
        let rest = line.dropFirst(marks)
        guard rest.isEmpty || rest.hasPrefix(" ") else { return nil }
        return (marks, rest.trimmingCharacters(in: .whitespaces))
    }

    /// The item text of a `- item`, `* item` or `+ item` line.
    static func bullet(_ line: String) -> String? {
        for mark in ["- ", "* ", "+ "] where line.hasPrefix(mark) {
            return String(line.dropFirst(2)).trimmingCharacters(in: .whitespaces)
        }
        return nil
    }

    /// The item text of a `1. item` or `1) item` line.
    static func numbered(_ line: String) -> String? {
        let digits = line.prefix { $0.isASCII && $0.isNumber }
        guard !digits.isEmpty, digits.count <= 9 else { return nil }
        let rest = line.dropFirst(digits.count)
        guard rest.hasPrefix(". ") || rest.hasPrefix(") ") else { return nil }
        return String(rest.dropFirst(2)).trimmingCharacters(in: .whitespaces)
    }
}

/// What Markdown.blocks keeps between lines: the blocks so far and the one being built.
struct BlockReader {
    var blocks: [MarkdownBlock] = []
    /// Lines of the paragraph, quote or list being built, and which it is.
    var pending: [String] = []
    var pendingKind = ""
    /// The open fence's language, while inside a fenced code block.
    var fence: String?
    var code: [String] = []

    mutating func read(_ raw: String) {
        let line = raw.trimmingCharacters(in: .whitespaces)
        if let language = fence {
            if line.hasPrefix("```") {
                blocks.append(.code(language: language, code: code.joined(separator: "\n")))
                fence = nil
                code = []
            } else {
                code.append(raw)
            }
            return
        }
        if line.hasPrefix("```") {
            flush()
            fence = String(line.dropFirst(3)).trimmingCharacters(in: .whitespaces)
            return
        }
        readOutsideFence(line)
    }

    private mutating func readOutsideFence(_ line: String) {
        if line.isEmpty {
            flush()
        } else if let heading = Markdown.heading(line) {
            flush()
            blocks.append(.heading(level: heading.level, text: heading.text))
        } else if line == "---" || line == "***" || line == "___" {
            flush()
            blocks.append(.rule)
        } else if let image = Markdown.image(line) {
            flush()
            blocks.append(.image(alt: image.alt, path: image.path))
        } else if let item = Markdown.bullet(line) {
            add(item, as: "bullets")
        } else if let item = Markdown.numbered(line) {
            add(item, as: "numbered")
        } else if line.hasPrefix(">") {
            add(String(line.dropFirst()).trimmingCharacters(in: .whitespaces), as: "quote")
        } else if pendingKind == "bullets" || pendingKind == "numbered" {
            // A line under a list item carries on that item.
            pending[pending.count - 1] += " " + line
        } else {
            add(line, as: pendingKind == "quote" ? "quote" : "paragraph")
        }
    }

    /// Adds a line to the block being built, finishing it first if it's of another kind.
    private mutating func add(_ text: String, as kind: String) {
        if kind != pendingKind { flush() }
        pendingKind = kind
        pending.append(text)
    }

    /// Ends the block being built, if any.
    mutating func flush() {
        guard !pending.isEmpty else { return }
        switch pendingKind {
        case "bullets": blocks.append(.bullets(pending))
        case "numbered": blocks.append(.numbered(pending))
        case "quote": blocks.append(.quote(pending.joined(separator: " ")))
        default: blocks.append(.paragraph(pending.joined(separator: " ")))
        }
        pending = []
        pendingKind = ""
    }

    /// Ends the text: a fence never closed keeps what it holds as code.
    mutating func finish() {
        if let language = fence {
            blocks.append(.code(language: language, code: code.joined(separator: "\n")))
            fence = nil
        }
        flush()
    }
}
