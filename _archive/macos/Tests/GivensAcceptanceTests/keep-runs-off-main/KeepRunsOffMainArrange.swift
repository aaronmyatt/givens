import Foundation
import GivensUI

/// Runs git in `directory` and returns its output, the final newline trimmed. A run line ends
/// in tabs when it has no note, so nothing else is trimmed. Steps set the repository's own
/// user and signing, so the developer's global git config can't change what they do.
/// Ref: https://developer.apple.com/documentation/foundation/process
@discardableResult
func keepRunsGit(_ arguments: [String], in directory: URL) throws -> String {
    precondition(!arguments.isEmpty, "git has a command")
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
    process.arguments = ["git"] + arguments
    process.currentDirectoryURL = directory
    let pipe = Pipe()
    process.standardOutput = pipe
    process.standardError = pipe
    try process.run()
    let data = pipe.fileHandleForReading.readDataToEndOfFile()
    process.waitUntilExit()
    let output = String(decoding: data, as: UTF8.self)
    guard process.terminationStatus == 0 else {
        throw StepFailure("git \(arguments.joined(separator: " ")): \(output)")
    }
    return output.hasSuffix("\n") ? String(output.dropLast()) : output
}

/// One run line, as the steps read it from the remote's ref: its run key and its time.
struct KeepRunsLine: Equatable {
    let key: String
    let time: String
}

/// The other device a scenario clones the shop onto: its own app, on its own serve.
@MainActor
struct KeepRunsDevice {
    let app: AppModel
    let window: WindowModel
}

/// What this capability's steps keep between steps of one scenario. Mirrors
/// acceptance/keep-runs-off-main in Go.
@MainActor
final class KeepRunsState {
    let w: World
    /// The bare remote the shop pushes to.
    var remote: URL?
    /// The other device, once cloned.
    var other: KeepRunsDevice?
    /// The runs a Given step recorded, oldest first, as RFC 3339 times.
    var times: [String] = []
    /// Those a trim was expected to drop.
    var dropped: [String] = []
    /// How long the last timed open took.
    var opening: Duration = .zero

    init(_ w: World) { self.w = w }

    /// The project's copy of refs/givens/runs, where Givens keeps the runs it shares.
    var tree: URL {
        guard let project = w.project else { preconditionFailure("the shop was copied") }
        return project.appendingPathComponent(".git/givens/tree", isDirectory: true)
    }

    /// Makes the sample shop a git repository, committed on main.
    func gitWithoutRemote() throws {
        try w.copySampleShop()
        guard let project = w.project else { throw StepFailure("no sample shop copied") }
        for arguments in [
            ["init", "--quiet", "--initial-branch=main"], ["config", "user.name", "Sam"],
            ["config", "user.email", "sam@example.com"], ["config", "commit.gpgsign", "false"],
            ["add", "."], ["commit", "--quiet", "-m", "Sample shop"],
        ] {
            try keepRunsGit(arguments, in: project)
        }
    }

    /// Makes the sample shop a git repository, committed on main and pushed to a bare remote.
    /// A window already open is opened again, as a developer who just added a remote would,
    /// so Givens sees the repository.
    func gitWithRemote() async throws {
        try gitWithoutRemote()
        guard let project = w.project else { throw StepFailure("no sample shop copied") }
        let bare = w.dir.appendingPathComponent("remote.git", isDirectory: true)
        try keepRunsGit(
            ["init", "--quiet", "--bare", "--initial-branch=main", bare.path], in: w.dir)
        remote = bare
        try keepRunsGit(["remote", "add", "origin", bare.path], in: project)
        try keepRunsGit(["push", "--quiet", "origin", "main"], in: project)
        guard let window = w.window, let app = w.app else { return }
        try await window.close()
        (w.window, w.session) = (nil, nil)
        w.window = try await app.open(w.capabilities.path)
        try await w.waitForRunsSync(w.window)
    }

    /// Makes the shop a git repository whose remote takes the connection and never answers:
    /// git's ext transport runs a command that only sleeps.
    /// Ref: https://git-scm.com/docs/git-remote-ext
    func remoteSilent() async throws {
        try await gitWithRemote()
        guard let project = w.project else { throw StepFailure("no sample shop copied") }
        try keepRunsGit(["config", "protocol.ext.allow", "always"], in: project)
        try keepRunsGit(["remote", "set-url", "origin", "ext::sh -c sleep% 30"], in: project)
    }

    /// Opens the shop as a developer would, timing it, and doesn't wait for the sync.
    func openTimed() async throws {
        try w.startApp()
        guard let app = w.app else { throw StepFailure("no app started") }
        try await app.markTrusted(w.capabilities.path)
        let clock = ContinuousClock()
        let start = clock.now
        w.window = try await app.open(w.capabilities.path)
        opening = clock.now - start
    }

    /// Chooses "Move runs off main" and confirms it, from the project: a Drop in still open is
    /// left first, as the developer would close it.
    func moveRunsOffMain() async throws {
        let window = try await w.front()
        if window.session != nil {
            try await window.select("Checkout")
            w.session = nil
        }
        try await window.choose("Move runs off main")
        try await window.choose("Move runs off main")
    }

    /// Writes into the ref's copy, then has Givens share it.
    func writeAndShare(_ write: () throws -> Void) async throws {
        try FileManager.default.createDirectory(at: tree, withIntermediateDirectories: true)
        try write()
        try await w.front().syncRuns()
    }

    /// Clones the remote as another device, with its own app on its own serve, and opens it.
    func cloneOther() async throws {
        guard let remote else { throw StepFailure("the shop has no remote") }
        let folder = w.dir.appendingPathComponent("other-device", isDirectory: true)
        try keepRunsGit(["clone", "--quiet", remote.path, folder.path], in: w.dir)
        let appData = w.dir.appendingPathComponent("other-app-data", isDirectory: true)
        try FileManager.default.createDirectory(at: appData, withIntermediateDirectories: true)
        let serve = ServeClient.Configuration(
            executable: try Toolchain.givens(), appDataDirectory: appData, testSeams: true,
            callTimeout: .seconds(World.setupWaitSeconds + World.callMarginSeconds))
        let platform = Platform(
            clipboard: w.clipboard, trash: w.trash, folderPicker: w.folderPicker,
            urlOpener: w.urlOpener)
        let app = AppModel(serve: serve, platform: platform)
        let capabilities = folder.appendingPathComponent("capabilities").path
        try await app.markTrusted(capabilities)
        let window = try await app.open(capabilities)
        try await w.waitForRunsSync(window)
        other = KeepRunsDevice(app: app, window: window)
    }

    /// Runs a step against the other device's app and window, then stops that app.
    func onOther(_ step: () async throws -> Void) async throws {
        guard let other else { throw StepFailure("no other device was cloned") }
        let (app, window, session) = (w.app, w.window, w.session)
        (w.app, w.window, w.session) = (other.app, other.window, nil)
        defer {
            other.app.stop()
            (w.app, w.window, w.session) = (app, window, session)
        }
        try await step()
    }

    /// The run key of a sample shop scenario: <capability>/<slug>.
    func runKey(_ name: String) throws -> String {
        "\(try w.scenarioCapability(name))/\(slug(name))"
    }

    /// The runs the remote's refs/givens/runs holds.
    func remoteRuns() throws -> [KeepRunsLine] {
        guard let remote else { throw StepFailure("the shop has no remote") }
        let names = try keepRunsGit(
            ["ls-tree", "-r", "--name-only", "refs/givens/runs"], in: remote)
        var found: [KeepRunsLine] = []
        for name in names.components(separatedBy: "\n")
        where name.hasPrefix("runs/") && name.hasSuffix(".log")
            && name.split(separator: "/").count == 2
        {
            let text = try keepRunsGit(["show", "refs/givens/runs:\(name)"], in: remote)
            for line in text.components(separatedBy: "\n") {
                let fields = line.components(separatedBy: "\t")
                if fields.count >= 5 { found.append(KeepRunsLine(key: fields[1], time: fields[0])) }
            }
        }
        return found
    }

    /// Writes `count` passing runs of a scenario into the ref's copy, each with a screenshot,
    /// a minute apart and before now, and has Givens share them.
    func recordedRuns(_ name: String, _ count: Int) async throws {
        let key = try runKey(name)
        let now = Date(timeIntervalSince1970: Date().timeIntervalSince1970.rounded(.down))
        let stamps = (0..<count).map { now.addingTimeInterval(-60 * Double(count - $0)) }
        times = stamps.map(Self.rfc3339)
        try await writeAndShare {
            for stamp in stamps { try writeShot(key, stamp) }
        }
    }

    /// One passing run of key at stamp, with a screenshot, in the ref's copy.
    private func writeShot(_ key: String, _ stamp: Date) throws {
        let folder = tree.appendingPathComponent(
            "runs/files/\(key)/\(Self.folderName(stamp))", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        try Data(Self.rfc3339(stamp).utf8).write(to: folder.appendingPathComponent("shot.png"))
        let log = tree.appendingPathComponent("runs/\(Self.rfc3339(stamp).prefix(10)).log")
        let line = "\(Self.rfc3339(stamp))\t\(key)\tpass\t\t\n"
        if let handle = try? FileHandle(forWritingTo: log) {
            try handle.seekToEnd()
            try handle.write(contentsOf: Data(line.utf8))
            try handle.close()
        } else {
            try Data(line.utf8).write(to: log)
        }
    }

    /// A time as a run line writes it: RFC 3339 in UTC. Ref: https://www.rfc-editor.org/rfc/rfc3339
    static func rfc3339(_ date: Date) -> String {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime]
        return formatter.string(from: date)
    }

    /// A run's files folder name: its time in UTC, as internal/runs names it.
    static func folderName(_ date: Date) -> String {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = TimeZone(identifier: "UTC")
        formatter.dateFormat = "yyyy-MM-dd'T'HHmmss'Z'"
        return formatter.string(from: date)
    }
}
