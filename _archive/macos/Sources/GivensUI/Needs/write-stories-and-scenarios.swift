import Foundation

// Calls capabilities/write-stories-and-scenarios needs that choose, type and press can't
// express: dialogs with several fields, and what the editor knows beyond its text (marks by
// line, step suggestions, the preview). Each mirrors a call in internal/app/edit.go, so the
// serve method does what the Go suite already checks. edit, setEditorText, saveEditor and
// rename come from gherkin-files.swift; listedScenarios from tags.swift.

/// One problem the editor marks, by line. Mirrors internal/app's EditorMark.
public struct EditorMarkRecord: Decodable, Sendable, Equatable {
    /// 1-based line in the editor's text.
    public let line: Int
    /// "Expected a step, a tag, a comment or a new scenario".
    public let message: String
}

/// One step offered while typing. Mirrors internal/app's StepSuggestion.
public struct StepSuggestionRecord: Decodable, Sendable, Equatable {
    /// The step without its keyword.
    public let text: String
    /// How widely it's used: "used in 4".
    public let label: String
}

/// One line of the editor's preview. Mirrors internal/app's PreviewLine.
public struct PreviewLineRecord: Decodable, Sendable, Equatable {
    /// The line as the library shows it, keywords lined up for steps.
    public let text: String
    /// The parser's message when the line doesn't parse, or "".
    public let mark: String
}

extension WindowModel {
    /// Fills in the New story dialog and creates the capability.
    /// Needed by "Write a new story".
    public func newStory(name: String, asA: String, iWant: String, soThat: String) async throws {
        precondition(!name.isEmpty, "a story has a name")
        let fields = ["name": name, "asA": asA, "iWant": iWant, "soThat": soThat]
        try await call("window.newStory", fields)
    }

    /// Fills in the New scenario dialog for the selected capability.
    /// Needed by "Add a scenario to a capability" and "Reuse an existing step".
    public func newScenario(_ name: String, error: Bool) async throws {
        precondition(!name.isEmpty, "a scenario has a name")
        try await call("window.newScenario", ["name": name, "error": error ? "true" : "false"])
    }

    /// The problems marked in the editor's text, by line.
    /// Needed by "A line that isn't a step".
    public func editorMarks() async throws -> [EditorMarkRecord] {
        try await writeStoriesValue("window.editorMarks", [:], [EditorMarkRecord].self)
    }

    /// The steps offered for the step being typed. Needed by "Reuse an existing step".
    public func stepSuggestions() async throws -> [StepSuggestionRecord] {
        try await writeStoriesValue("window.stepSuggestions", [:], [StepSuggestionRecord].self)
    }

    /// The steps offered for one step line as the Mac editor holds it, like "Given a sh".
    /// None for a line that isn't a step.
    public func stepSuggestions(forLine line: String) async throws -> [StepSuggestionRecord] {
        precondition(!line.contains("\n"), "suggestions are for one line")
        guard !line.trimmingCharacters(in: .whitespaces).isEmpty else { return [] }
        return try await writeStoriesValue(
            "window.stepSuggestions", ["text": line], [StepSuggestionRecord].self)
    }

    /// Replaces the step being typed with an offered one, word for word.
    /// Needed by "Reuse an existing step".
    public func chooseStepSuggestion(_ text: String) async throws {
        precondition(!text.isEmpty && !text.contains("\n"), "a suggestion is one step")
        try await call("window.chooseStepSuggestion", ["text": text])
    }

    /// What "Preview" shows for the editor's text, unsaved changes included.
    /// Needed by "Preview a scenario while editing" and the other preview scenarios.
    public func editorPreview() async throws -> [PreviewLineRecord] {
        try await writeStoriesValue("window.editorPreview", [:], [PreviewLineRecord].self)
    }

    /// Asks to delete a scenario or capability; the window then offers how to go on.
    /// Needed by "Delete a scenario" and the other delete scenarios.
    public func delete(_ name: String) async throws {
        precondition(!name.isEmpty, "delete names what to delete")
        try await call("window.delete", ["name": name])
    }

    /// Sends a window call that returns a value, and applies its reply.
    private func writeStoriesValue<Value: Decodable & Sendable>(
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
