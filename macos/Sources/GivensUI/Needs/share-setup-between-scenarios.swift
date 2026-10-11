import Foundation

// What capabilities/share-setup-between-scenarios needs from the core that the window's
// snapshot doesn't carry: the Shared list's full entries, a scenario's required commands, and
// the helper editor. Each mirrors a call in internal/app/helpers.go, with lower-camel JSON keys.

/// One documented helper in the sidebar's "Shared" section. Mirrors internal/app's Helper.
public struct SharedHelperRecord: Decodable, Sendable, Equatable {
    /// "seed_shopper".
    public let name: String
    /// "seed_shopper [email]", from the `# @helper` line.
    public let signature: String
    /// The comment lines under the `# @helper` line.
    public let description: String
    /// The capability whose _shared folder holds it, or "" for the project's _shared.
    public let capability: String
    /// Relative to the capabilities folder: "_shared/users.sh".
    public let definedIn: String
    /// The line of its `name() {`, counting from 1.
    public let line: Int
    /// Setup scripts that call it, relative to the capabilities folder.
    public let callers: [String]
    /// "Overrides _shared/users.sh" when it shadows a shared helper, or "".
    public let overrides: String
    /// Why it's flagged, such as a second definition at the same level, or "".
    public let warning: String
}

/// One command a scenario's setup needs. Mirrors internal/app's RequiredCommand.
public struct RequiredCommandRecord: Decodable, Sendable, Equatable {
    /// "bb".
    public let command: String
    /// The install hint from the `need` line.
    public let hint: String
    /// Whether the command is on PATH.
    public let installed: Bool
}

/// One line the helper editor marks, from `bash -n`. Mirrors internal/app's HelperProblem.
/// Ref: https://www.gnu.org/software/bash/manual/bash.html#The-Set-Builtin
public struct HelperProblemRecord: Decodable, Sendable, Equatable {
    /// As bash reports it, counting from 1.
    public let line: Int
    /// Bash's own words, such as "syntax error: unexpected end of file".
    public let message: String
}

/// The helper file open in the editor. Mirrors internal/app's HelperEdit.
public struct HelperEditRecord: Decodable, Sendable, Equatable {
    /// The helper file, absolute, or "" when no helper is open.
    public let path: String
    /// The line the editor opened at: the helper's `name() {` line.
    public let line: Int
    /// Scripts listed beside the editor, relative to the capabilities folder.
    public let callers: [String]
    /// Lines marked with a syntax error since the last save.
    public let problems: [HelperProblemRecord]
}

extension WindowModel {
    /// The sidebar's "Shared" section, with where each helper is defined, what it overrides
    /// and why it's flagged. Needed by "Two helpers with the same name", "Two shared files
    /// define the same helper" and "Add a helper".
    public func helpers() async throws -> [SharedHelperRecord] {
        try await shareSetupValue("window.helpers", [:], [SharedHelperRecord].self)
    }

    /// The commands a scenario's setup needs, whether each is installed, and its install hint.
    /// Needed by "See what a scenario needs installed".
    public func requiredCommands(_ scenario: String) async throws -> [RequiredCommandRecord] {
        precondition(!scenario.isEmpty, "required commands name a scenario")
        return try await shareSetupValue(
            "window.requiredCommands", ["scenario": scenario], [RequiredCommandRecord].self)
    }

    /// The helper file open in the editor, where it opened, its callers and its marked lines.
    /// Needed by "Edit a helper" and "Save a helper with a syntax error".
    public func editingHelper() async throws -> HelperEditRecord {
        try await shareSetupValue("window.editingHelper", [:], HelperEditRecord.self)
    }

    /// Creates `<capability>/_shared/<name>.sh` with a `# @helper` comment and an empty
    /// function, and opens it in the editor. Needed by "Add a helper".
    public func newHelper(capability: String, name: String) async throws {
        precondition(!capability.isEmpty, "a helper belongs to a capability")
        precondition(!name.isEmpty && !name.contains("/"), "a helper's name is a function name")
        try await call("window.newHelper", ["capability": capability, "name": name])
    }

    /// Saves `text` as the helper file open in the editor, even when bash can't parse it, then
    /// marks the lines `bash -n` reports. Needed by "Save a helper with a syntax error".
    public func saveHelper(_ text: String) async throws {
        precondition(text.utf8.count <= 1 << 22, "a helper file stays bounded")
        try await call("window.saveHelper", ["text": text])
    }

    /// Sends a window call that returns a value, applies its snapshot, and returns the value.
    private func shareSetupValue<Value: Decodable & Sendable>(
        _ method: String, _ params: [String: String], _ type: Value.Type
    ) async throws -> Value {
        precondition(params["window"] == nil, "the window is named by the model")
        let reply = try await app.serve.call(
            method, params.merging(["window": id]) { $1 }, returning: type)
        try app.apply(reply)
        guard let value = reply.value else { throw GivensError.badReply("\(method) sent nothing") }
        return value
    }
}
