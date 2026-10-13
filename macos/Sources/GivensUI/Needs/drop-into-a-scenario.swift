import Foundation

// What capabilities/drop-into-a-scenario needs from the core that the models don't offer yet.
// Each call mirrors one in internal/app/dropin.go, under the same serve wire format as
// ServeProtocol.swift: one JSON request line, one reply line carrying the whole snapshot.

/// One fenced code block in a setup card. Mirrors internal/app's CodeBlock.
public struct CodeBlockRecord: Decodable, Sendable, Equatable {
    /// The fence's info string, like "bash" or "js"; it labels the Copy button.
    public let language: String
    /// Exactly the code: no fences, no list number, no list indent.
    public let code: String
    /// False while the fence never closes.
    public let copyable: Bool
}

/// One card on the Drop in page: a by_hand or ask step, or a markdown step. Mirrors
/// internal/app's SetupCard.
public struct SetupCardRecord: Decodable, Sendable, Equatable {
    /// "by_hand", "ask" or "markdown".
    public let kind: String
    /// by_hand's place, like "DigitalOcean console"; "" otherwise.
    public let `where`: String
    /// ask's prompt; "" otherwise.
    public let prompt: String
    /// The card's text as shown, code included.
    public let text: String
    /// The --url link, or "".
    public let link: String
    /// Link targets in a markdown card's text.
    public let links: [String]
    /// Image paths shown in a markdown card.
    public let images: [String]
    /// Fenced code blocks, in order.
    public let blocks: [CodeBlockRecord]
    /// Actions on this card: "Done", "Failed", "Copy", "Open", "Continue".
    public let offers: [String]
    /// "", "done" or "failed".
    public let mark: String
    /// Why the card is marked, like an unclosed code fence; or "".
    public let warning: String
}

/// One Examples row on an outline's Drop in page. Mirrors internal/app's DropInRow.
public struct DropInRowRecord: Decodable, Sendable, Equatable {
    /// The Examples block's name, or "".
    public let examples: String
    /// Column to value, like card=expired.
    public let values: [String: String]
    /// The outline's steps with this row's placeholders filled in.
    public let steps: [String]
    /// Like "Never run" or "Verified today".
    public let status: String
    /// Actions enabled on this row: "Drop in", "Switch to this row".
    public let offers: [String]
    /// Actions shown but disabled, like a greyed "Drop in".
    public let disabled: [String]
    /// True while this row's checklist is open beneath it.
    public let live: Bool
    /// True while another row is live, so this one is greyed out.
    public let greyed: Bool
    /// The live row's checklist; empty otherwise.
    public let checklist: [CheckItem]
}

extension AppModel {
    /// Records a project's folder as trusted, as choosing "Trust" on the "Run scripts from this
    /// project?" prompt does. Mirrors App.MarkTrusted. Needed by "Drop in" and every scenario
    /// of drop-into-a-scenario whose setup runs, so they start trusted; trust itself is
    /// covered by trust-a-project.
    public func markTrusted(_ path: String) async throws {
        precondition(path.hasPrefix("/"), "a trusted folder has an absolute path")
        try await send("test.markTrusted", ["path": path])
    }
}

extension SessionModel {
    /// The Drop in page's setup cards in the order setup reached them. Mirrors Session.Cards.
    /// Needed by "A step done by hand" and every other card scenario.
    public func setupCards() async throws -> [SetupCardRecord] {
        let reply = try await window.app.serve.call(
            "session.cards", ["window": window.id], returning: [SetupCardRecord].self)
        try window.app.apply(reply)
        return reply.value ?? []
    }

    /// Takes an action on one card, by its index in `setupCards()`: "Done", "Failed", "Copy",
    /// "Open" or "Continue". Mirrors Session.ChooseOnCard. Returns the links the core asked
    /// the default browser to open, as "openURL" effects; the app hands each to
    /// NSWorkspace.open. Needed by "A step done by hand" and "A step in the browser".
    /// Ref: https://developer.apple.com/documentation/appkit/nsworkspace/open(_:)
    @discardableResult
    public func chooseOnCard(_ card: Int, _ action: String) async throws -> [String] {
        precondition(card >= 0, "a card index isn't negative")
        precondition(!action.isEmpty, "an action has a label")
        let params = ["window": window.id, "card": String(card), "action": action]
        let reply = try await window.app.serve.call("session.chooseOnCard", params)
        if let snapshot = reply.snapshot { window.app.apply(snapshot) }
        var opened: [String] = []
        for effect in reply.effects ?? [] {
            if effect.kind == "openURL" { opened.append(effect.text ?? "") }
            try window.app.perform(effect)
        }
        return opened
    }

    /// Copies one code block of one card: exactly its code. Mirrors Session.CopyCodeBlock.
    /// Needed by "Copy a code block" and "Several code blocks".
    public func copyCodeBlock(card: Int, block: Int) async throws {
        precondition(card >= 0 && block >= 0, "card and block indexes aren't negative")
        try await window.call(
            "session.copyCodeBlock", ["card": String(card), "block": String(block)])
    }

    /// Copies a card's inline code span, given without its backticks. Mirrors
    /// Session.ClickInlineCode. Needed by "Copy inline code".
    public func clickInlineCode(card: Int, code: String) async throws {
        precondition(card >= 0, "a card index isn't negative")
        precondition(!code.isEmpty && !code.contains("`"), "inline code has text, no backticks")
        try await window.call("session.clickInlineCode", ["card": String(card), "code": code])
    }

    /// Sets the note "Record run" saves with the run, in Markdown. A note of more than one
    /// line is kept whole as note.md in the run's folder. Mirrors Session.SetRunNote.
    /// Needed by "Record a run" and "Record a run with a note in Markdown".
    public func setRunNote(_ note: String) async throws {
        precondition(note.utf8.count <= 64 << 10, "a note fits runs.NoteBytesMax")
        try await window.call("session.setRunNote", ["note": note])
    }

    /// Attaches the file at `path`; "Record run" copies it into the run's folder in runs/.
    /// Mirrors Session.Attach. Needed by "Attach proof to a run".
    public func attach(_ path: String) async throws {
        precondition(path.hasPrefix("/"), "an attached file has an absolute path")
        try await window.call("session.attach", ["path": path])
    }

    /// Takes the attached file called `name` off again. Mirrors Session.Detach.
    public func detach(_ name: String) async throws {
        precondition(!name.isEmpty && !name.contains("/"), "a file name is one name")
        try await window.call("session.detach", ["name": name])
    }
}

extension WindowModel {
    /// The Examples rows of the selected outline's Drop in page, in file order. Mirrors
    /// Window.DropInRows. Needed by "See every row on one page" and the other outline
    /// scenarios.
    public func dropInRows() async throws -> [DropInRowRecord] {
        let reply = try await app.serve.call(
            "window.dropInRows", ["window": id], returning: [DropInRowRecord].self)
        try app.apply(reply)
        return reply.value ?? []
    }

    /// Takes an action on one Examples row, named by one of its values like "expired":
    /// "Drop in" or "Switch to this row". Mirrors Window.ChooseOnRow. Needed by "Drop in to
    /// one row" and "Switch to another row".
    public func chooseOnRow(_ row: String, _ action: String) async throws {
        precondition(!row.isEmpty && !action.isEmpty, "a row and an action are named")
        try await call("window.chooseOnRow", ["row": row, "action": action])
    }
}
