import Foundation

// What capabilities/keep-runs-off-main needs from the core beyond WindowSnapshot and the
// offers ("Move runs off main", "Share runs", "Change run limits"): a sync the app runs every
// few minutes, so runs recorded on another device show up without reopening the project, and
// the ref's limits, which the "Change run limits" sheet reads and sets.

/// How many runs, and MB of them, refs/givens/runs keeps. Serve's runLimitsValue.
public struct RunLimits: Decodable, Sendable {
    /// The most runs the ref keeps.
    public let runs: Int
    /// The most MB of runs the ref keeps.
    public let megabytes: Int
}

extension WindowModel {
    /// Shares the project's runs kept off main, and fetches other devices'. A remote that
    /// can't be reached is said in the window's messages, not thrown. Mirrors Window.SyncRuns.
    public func syncRuns() async throws {
        try await call("window.syncRuns")
    }

    /// The ref's limits: its limits file, or the defaults until one is written. Mirrors
    /// Window.RunLimits. Needed by "Change the limits".
    public func runLimits() async throws -> RunLimits {
        let reply = try await app.serve.call(
            "window.runLimits", ["window": id], returning: RunLimits.self)
        try app.apply(reply)
        guard let value = reply.value else { throw GivensError.badReply("no run limits") }
        return value
    }

    /// Writes the ref's limits and shares them; lower limits trim the ref at once. Mirrors
    /// Window.SetRunLimits. Needed by "Change the limits".
    public func setRunLimits(runs: Int, megabytes: Int) async throws {
        try await call("window.setRunLimits", ["runs": "\(runs)", "megabytes": "\(megabytes)"])
    }
}
