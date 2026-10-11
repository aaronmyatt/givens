import Foundation
import GherkinKit
import GivensUI

/// One scenario's state: a fresh copy of the sample shop, an AppModel on a real `givens serve`
/// with fake clipboard, Trash and folder picker, and what the last action left behind.
/// It mirrors acceptance/world in Go, so the two suites read the same.
@MainActor
final class World {
    /// How long a step waits for a Drop in's setup.
    static let setupWaitSeconds = 10

    let dir: URL
    let appData: URL
    let clipboard = FakeClipboard()
    let trash: FakeTrash
    let folderPicker = FakeFolderPicker()
    let urlOpener = FakeURLOpener()

    /// The sample shop's copy, inside dir; nil until a step copies it.
    var project: URL?
    /// Where commands run; nil means the sample shop's folder.
    var cwd: URL?
    /// Folders put on PATH ahead of the built givens, such as an installed command or a fake
    /// `open` that records what it was asked to launch.
    var pathPrepend: [URL] = []

    var app: AppModel?
    var window: WindowModel?
    var session: SessionModel?

    /// The last action's error, for steps that expect one.
    var error: Error?
    /// The last command's output, stdout and stderr together.
    var output = ""
    /// The last command's exit code.
    var exitCode: Int32 = 0

    /// Keeps openSampleShop from trusting the shop. Scenarios about trust set it; every other
    /// scenario starts trusted, as if the developer chose "Trust" earlier. Mirrors Go's World.
    var untrusted = false

    /// The file the scenario last edited, for "the file is saved anyway".
    var editedPath: URL?
    var editedText = ""

    init() throws {
        dir = FileManager.default.temporaryDirectory
            .appendingPathComponent("givens-scenario-\(UUID().uuidString)", isDirectory: true)
        appData = dir.appendingPathComponent("app-data", isDirectory: true)
        trash = FakeTrash(directory: dir.appendingPathComponent("Trash", isDirectory: true))
        try FileManager.default.createDirectory(at: appData, withIntermediateDirectories: true)
    }

    /// Stops serve and removes the scenario's folder.
    func tearDown() {
        precondition(dir.lastPathComponent.hasPrefix("givens-scenario-"), "only a scenario folder")
        app?.stop()
        try? FileManager.default.removeItem(at: dir)
    }

    /// The sample shop's capabilities folder.
    var capabilities: URL {
        guard let project else { preconditionFailure("a step copied the sample shop first") }
        return project.appendingPathComponent("capabilities", isDirectory: true)
    }

    /// Copies testdata/sample-shop into the scenario's folder, with internal/kit/givens.sh
    /// when it exists.
    func copySampleShop() throws {
        if project != nil { return }
        let copy = dir.appendingPathComponent("sample-shop", isDirectory: true)
        let fixture = Toolchain.root.appendingPathComponent("testdata/sample-shop")
        try FileManager.default.copyItem(at: fixture, to: copy)
        project = copy
        // The sample shop's cucumber-js, installed by scripts/runner.sh, linked in so npx
        // finds it without the network. acceptance/world does the same.
        // Ref: https://developer.apple.com/documentation/foundation/filemanager/createsymboliclink(at:withdestinationurl:)
        let modules = Toolchain.root.appendingPathComponent(".cache/runner/node_modules")
        if FileManager.default.fileExists(atPath: modules.path) {
            try FileManager.default.createSymbolicLink(
                at: copy.appendingPathComponent("node_modules"), withDestinationURL: modules)
        }
        let kit = Toolchain.root.appendingPathComponent("internal/kit/givens.sh")
        if let text = try? String(contentsOf: kit, encoding: .utf8) {
            try writeFile("capabilities/_shared/givens.sh", text)
        }
    }

    /// Builds the app on fakes, with test seams on. It doesn't launch it.
    func startApp() throws {
        if app != nil { return }
        let platform = Platform(
            clipboard: clipboard, trash: trash, folderPicker: folderPicker, urlOpener: urlOpener)
        let serve = ServeClient.Configuration(
            executable: try Toolchain.givens(), appDataDirectory: appData, testSeams: true)
        app = AppModel(serve: serve, platform: platform)
    }

    /// Copies the sample shop, trusts it unless `untrusted`, and opens its capabilities folder.
    func openSampleShop() async throws {
        try copySampleShop()
        try startApp()
        if !untrusted { try await app?.markTrusted(capabilities.path) }
        window = try await app?.open(capabilities.path)
    }

    /// The window under test, opening the sample shop when nothing is open yet.
    func front() async throws -> WindowModel {
        if let window { return window }
        if app == nil && project != nil {
            try await openSampleShop()
        } else if let front = app?.front {
            window = front
        }
        guard let window else { throw StepFailure("no Givens window is open") }
        return window
    }

    /// Writes `content` to `relative`, inside the sample shop, making folders as needed.
    func writeFile(_ relative: String, _ content: String) throws {
        precondition(!relative.hasPrefix("/"), "relative is inside the sample shop")
        guard let project else { throw StepFailure("no sample shop to write \(relative) in") }
        let url = project.appendingPathComponent(relative)
        try FileManager.default.createDirectory(
            at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try content.write(to: url, atomically: true, encoding: .utf8)
    }

    /// Reads `relative`, inside the sample shop.
    func readFile(_ relative: String) throws -> String {
        precondition(!relative.hasPrefix("/"), "relative is inside the sample shop")
        guard let project else { throw StepFailure("no sample shop to read \(relative) from") }
        return try String(contentsOf: project.appendingPathComponent(relative), encoding: .utf8)
    }

    /// Runs `command` with bash in cwd (or the sample shop), with pathPrepend and then the
    /// built givens first on PATH.
    func run(_ command: String) throws {
        let directory = cwd ?? project ?? dir
        let path = pathPrepend + [try Toolchain.binaries()]
        let result = try Shell.run(command, in: directory, path: path)
        output = result.output
        exitCode = result.status
    }

    /// Drops in to a scenario in the front window and waits for setup to finish or pause.
    func dropIn(_ scenario: String) async throws {
        let session = try await front().dropIn(scenario)
        self.session = session
        try await session.wait(seconds: Self.setupWaitSeconds)
    }

    /// Opens search with ⌘K and types `query`.
    func search(_ query: String) async throws {
        let window = try await front()
        try await window.press("⌘K")
        try await window.type(query)
    }

    /// Drops in, marks every check, and records a passing run.
    func recordPassingRun(_ scenario: String) async throws {
        if window == nil { try await openSampleShop() }
        try await dropIn(scenario)
        guard let session, session.status == SessionModel.ready else {
            throw StepFailure("drop in to \(scenario) ended \(session?.status ?? "nil"), not ready")
        }
        for item in session.checklist {
            try await session.mark(item.text, item.kind == "then" ? "pass" : "done")
        }
        try await session.recordRun()
    }
}

/// True when two paths name the same file, after resolving symlinks like /var → /private/var.
func samePath(_ first: String, _ second: String) -> Bool {
    precondition(!first.isEmpty, "a path is given")
    let resolve = { (path: String) in URL(fileURLWithPath: path).resolvingSymlinksInPath().path }
    return resolve(first) == resolve(second)
}

/// Fails unless one of `got` contains `want`, naming what was seen.
func expect(_ got: [String], _ want: String, _ what: String) throws {
    precondition(!want.isEmpty, "an expectation names text")
    guard got.contains(where: { $0.contains(want) }) else {
        throw StepFailure("\(what) doesn't include \"\(want)\"; it has \(got)")
    }
}

/// Fails if any of `got` contains `unwanted`.
func expectNot(_ got: [String], _ unwanted: String, _ what: String) throws {
    precondition(!unwanted.isEmpty, "an expectation names text")
    if let line = got.first(where: { $0.contains(unwanted) }) {
        throw StepFailure("\(what) includes \"\(unwanted)\", and shouldn't: \(line)")
    }
}

/// A name in lower case with hyphens, by the rule in capabilities/GIVENS.md: letters and
/// digits in any language are kept; every run of other characters becomes one hyphen.
func slug(_ name: String) -> String {
    var result = ""
    for character in name.lowercased() {
        if character.isLetter || character.isNumber {
            result.append(character)
        } else if !result.hasSuffix("-") {
            result.append("-")
        }
    }
    let trimmed = result.trimmingCharacters(in: CharacterSet(charactersIn: "-"))
    precondition(!trimmed.hasPrefix("-") && !trimmed.hasSuffix("-"), "hyphens are trimmed")
    return trimmed
}
