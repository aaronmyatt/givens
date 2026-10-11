// A pickle is one compiled scenario: Background steps folded in, one per Examples row.
// The Go side compiles them (tools/pickles) so Swift and godog run identical scenarios.
// Ref: https://github.com/cucumber/messages/blob/main/messages.md#pickle

/// One compiled scenario, decoded from the Cucumber Pickle message.
public struct Pickle: Decodable, Sendable, Equatable {
    /// Unique within one compile run.
    public let id: String
    /// The .feature file it came from, as passed to tools/pickles.
    public let uri: String
    /// The scenario's name, Examples values filled in.
    public let name: String
    /// Steps in order, Background steps first.
    public let steps: [PickleStep]
    /// Every tag that applies: the Feature's, the Rule's, the scenario's and the Examples'.
    public let tags: [PickleTag]
    /// Where the scenario starts in its file; nil for very old compilers.
    public let location: PickleLocation?

    /// The tag names alone, like "@wip".
    public var tagNames: [String] { tags.map(\.name) }
}

/// One step of a pickle.
public struct PickleStep: Decodable, Sendable, Equatable {
    /// Unique within one compile run.
    public let id: String
    /// The step without its keyword: `they see "Card declined"`.
    public let text: String
    /// "Context", "Action", "Outcome" or "Unknown".
    public let type: String?
    /// A doc string or a data table, when the step has one.
    public let argument: PickleStepArgument?
}

/// The doc string or data table under a step.
public struct PickleStepArgument: Decodable, Sendable, Equatable {
    /// A `"""` block.
    public let docString: PickleDocString?
    /// A `| a | b |` table.
    public let dataTable: PickleTable?
}

/// A `"""` block under a step.
public struct PickleDocString: Decodable, Sendable, Equatable {
    /// The text between the fences, indentation removed.
    public let content: String
    /// The media type after the opening fence, like "markdown".
    public let mediaType: String?
}

/// A data table under a step.
public struct PickleTable: Decodable, Sendable, Equatable {
    /// Rows top to bottom, the header row included.
    public let rows: [PickleTableRow]

    /// The table as plain strings, row by row.
    public var cells: [[String]] { rows.map { $0.cells.map(\.value) } }
}

/// One row of a data table.
public struct PickleTableRow: Decodable, Sendable, Equatable {
    /// Cells left to right.
    public let cells: [PickleTableCell]
}

/// One cell of a data table.
public struct PickleTableCell: Decodable, Sendable, Equatable {
    /// The cell's text, trimmed.
    public let value: String
}

/// A tag on a pickle.
public struct PickleTag: Decodable, Sendable, Equatable {
    /// The tag with its @, like "@error".
    public let name: String
}

/// A line and column in a .feature file.
public struct PickleLocation: Decodable, Sendable, Equatable {
    /// 1-based line.
    public let line: Int
    /// 1-based column.
    public let column: Int?
}
