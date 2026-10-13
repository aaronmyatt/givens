import Foundation

// What capabilities/open-a-project needs from the core beyond WindowSnapshot: another
// branch's capabilities, read without checking that branch out.

extension WindowModel {
    /// The branches of the git repository holding this window's project: local ones first,
    /// then remote ones like "origin/gift-cards". Mirrors Window.Branches.
    /// Needed by "Read capabilities from another branch".
    public func branches() async throws -> [String] {
        let reply = try await app.serve.call(
            "window.branches", ["window": id], returning: [String].self)
        try app.apply(reply)
        return reply.value ?? []
    }

    /// Opens this project's capabilities as they are on `name`, read only, in a new window
    /// in front. A branch that doesn't exist is said in this window instead. Mirrors
    /// Window.OpenBranch. Needed by "Read capabilities from another branch" and "Open a
    /// branch that doesn't exist".
    public func openBranch(_ name: String) async throws {
        precondition(!name.isEmpty && !name.hasPrefix("-"), "a branch name isn't an option")
        try await call("window.openBranch", ["name": name])
    }
}
