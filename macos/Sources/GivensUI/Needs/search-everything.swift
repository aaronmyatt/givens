// What capabilities/search-everything needs from the core beyond WindowSnapshot. The grouped
// ⌘K results it reads (searchResults(), SearchResultsRecord) are in Needs/tags.swift, which
// capabilities/tags needs too; this file adds the kind filter.

extension WindowModel {
    /// Shows only one kind of result, by its heading ("Scripts"), or every kind for "".
    /// Mirrors internal/app's Window.FilterSearch.
    /// Needed by "Show only one kind of result".
    public func filterSearch(_ kind: String) async throws {
        precondition(kind.count <= 1024, "the kind fits NameLengthMax")
        try await call("window.filterSearch", ["kind": kind])
    }
}
