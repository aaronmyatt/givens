import Foundation

// Calls capabilities/tags needs that the window's snapshot doesn't carry: the scenarios a
// view or tag lists, and ⌘K search results grouped by kind with their highlighted parts.
// Each mirrors a call in internal/app/search.go.

/// One search result row. Mirrors app.SearchResult.
public struct SearchResultRecord: Decodable, Sendable, Equatable {
    /// The row's title: a capability, scenario, step, script file or helper.
    public let name: String
    /// The row's second line, or "".
    public let detail: String
    /// The capability the result belongs to, or "" for a shared helper.
    public let capability: String
    /// For a step, how many scenarios use it word for word; 0 otherwise.
    public let uses: Int
    /// Each highlighted part of name and detail, in reading order.
    public let matches: [String]
}

/// The results of one kind. Mirrors app.SearchGroup.
public struct SearchGroupRecord: Decodable, Sendable, Equatable {
    /// "Stories", "Scenarios", "Steps", "Scripts" or "Helpers".
    public let kind: String
    /// Best first.
    public let results: [SearchResultRecord]
}

/// The ⌘K search's results, grouped. Mirrors app.SearchResults.
public struct SearchResultsRecord: Decodable, Sendable, Equatable {
    /// The text in the search box.
    public let query: String
    /// The chosen kind filter, or "" for every kind.
    public let filter: String
    /// Only kinds with results, in kind order.
    public let groups: [SearchGroupRecord]

    /// The results of one kind, or none.
    public func results(_ kind: String) -> [SearchResultRecord] {
        precondition(!kind.isEmpty, "a group has a kind")
        return groups.first { $0.kind == kind }?.results ?? []
    }
}

extension WindowModel {
    /// The ⌘K search's results, grouped by kind. Window.SearchResults.
    /// Needed by "Search by tag", "Search with a tag expression", "A tag and words together"
    /// and "A tag expression that doesn't parse".
    public func searchResults() async throws -> SearchResultsRecord {
        let reply = try await app.serve.call(
            "window.searchResults", ["window": id], returning: SearchResultsRecord.self)
        try app.apply(reply)
        guard let results = reply.value else {
            throw GivensError.badReply("window.searchResults sent nothing")
        }
        return results
    }

    /// The scenarios the selected view or tag lists. Window.ListedScenarios.
    /// Needed by "An error state", "A tag view", "A tag on the Feature" and "A runner filters
    /// on the same tags".
    public func listedScenarios() async throws -> [ScenarioRecord] {
        let reply = try await app.serve.call(
            "window.listedScenarios", ["window": id], returning: [ScenarioRecord].self)
        try app.apply(reply)
        guard let scenarios = reply.value else {
            throw GivensError.badReply("window.listedScenarios sent nothing")
        }
        return scenarios
    }
}
