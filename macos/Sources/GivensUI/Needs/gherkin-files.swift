import Foundation

// Calls capabilities/gherkin-files needs that the window's snapshot and its select, choose,
// type and press can't express: a capability's Rule: groups, editing a .feature file's text
// exactly, and renaming a scenario. Each mirrors a call in internal/app (search.go, edit.go).

/// A `Rule:` block in a capability, as the window groups its scenarios. Mirrors app.RuleGroup.
/// Ref: https://cucumber.io/docs/gherkin/reference/#rule
public struct RuleGroupRecord: Decodable, Sendable, Equatable {
    /// The text after "Rule:".
    public let name: String
    /// The names of the scenarios under it, in file order.
    public let scenarios: [String]
}

extension WindowModel {
    /// Editor text bytes, matching internal/app's EditorTextBytesMax.
    public static let editorTextBytesMax = 4 << 20

    /// A capability's Rule: blocks, in file order. Window.RuleGroups.
    /// Needed by "Rules group scenarios".
    public func ruleGroups(_ capability: String) async throws -> [RuleGroupRecord] {
        precondition(!capability.isEmpty, "rule groups name a capability")
        let reply = try await app.serve.call(
            "window.ruleGroups", ["window": id, "name": capability],
            returning: [RuleGroupRecord].self)
        try app.apply(reply)
        guard let groups = reply.value else {
            throw GivensError.badReply("window.ruleGroups sent nothing")
        }
        return groups
    }

    /// Opens the .feature file holding a scenario or capability in the editor and returns
    /// what it shows. Window.Edit.
    /// Needed by "Comments are kept".
    @discardableResult
    public func edit(_ target: String) async throws -> EditorRecord {
        precondition(!target.isEmpty, "edit names a scenario or capability")
        try await call("window.edit", ["target": target])
        guard let editor = snapshot?.editor else {
            throw GivensError.badReply("window.edit opened no editor")
        }
        return editor
    }

    /// Replaces the open editor's whole text, unsaved. Editor.SetText.
    /// Needed by "Comments are kept".
    public func setEditorText(_ text: String) async throws {
        precondition(text.utf8.count <= Self.editorTextBytesMax, "text fits editorTextBytesMax")
        try await call("window.setEditorText", ["text": text])
    }

    /// Saves the open editor's text to its file. Editor.Save.
    /// Needed by "Comments are kept".
    public func saveEditor() async throws {
        precondition(snapshot?.editor != nil, "save needs a file open in the editor")
        try await call("window.saveEditor")
    }

    /// Renames a scenario or capability in its .feature file. Window.Rename.
    /// Needed by "Two scenario names with the same slug".
    public func rename(_ name: String, to newName: String) async throws {
        precondition(!name.isEmpty, "rename names what to rename")
        precondition(!newName.isEmpty, "a new name isn't empty")
        try await call("window.rename", ["name": name, "newName": newName])
    }
}
