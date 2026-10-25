// What capabilities/views needs from the core beyond WindowSnapshot: the helpers the Shared
// view lists, which don't fit the snapshot's single selected capability. The scenarios a view
// lists come from listedScenarios() in Needs/tags.swift, which capabilities/tags needs too.
// Field names match internal/app's ViewHelper with lower-camel JSON keys.

/// One helper listed in the sidebar's Shared view.
public struct ViewHelperRecord: Decodable, Sendable, Equatable {
    /// "seed_shopper".
    public let name: String
    /// The text after "# @helper": "seed_shopper [email]".
    public let signature: String
    /// The comment lines under it, joined with spaces.
    public let description: String
    /// Where it's defined, relative to the capabilities folder: "_shared/users.sh".
    public let file: String
    /// How many setup scripts call it.
    public let callers: Int
}

extension WindowModel {
    /// The helpers the Shared view lists while it's selected: every helper documented with a
    /// "# @helper" comment. Mirrors internal/app's Window.ViewHelpers.
    /// Needed by "Browse helpers".
    public func viewHelpers() async throws -> [ViewHelperRecord] {
        let reply = try await app.serve.call(
            "window.viewHelpers", ["window": id], returning: [ViewHelperRecord].self)
        try app.apply(reply)
        guard let helpers = reply.value else {
            throw GivensError.badReply("window.viewHelpers sent nothing")
        }
        return helpers
    }
}
