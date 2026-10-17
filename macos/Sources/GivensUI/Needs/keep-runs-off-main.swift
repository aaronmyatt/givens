import Foundation

// What capabilities/keep-runs-off-main needs from the core beyond WindowSnapshot and the
// offers ("Move runs off main", "Share runs"): a sync the app runs every few minutes, so runs
// recorded on another device show up without reopening the project.

extension WindowModel {
    /// Shares the project's runs kept off main, and fetches other devices'. A remote that
    /// can't be reached is said in the window's messages, not thrown. Mirrors Window.SyncRuns.
    public func syncRuns() async throws {
        try await call("window.syncRuns")
    }
}
