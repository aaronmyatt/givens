import Foundation
import GherkinKit

/// The repository, the Go-built commands the suites need, and the pickles they run.
/// Built once per test process: `go build` takes seconds, scenarios take milliseconds.
@MainActor
enum Toolchain {
    /// The repository root: the folder above macos/ holding go.mod.
    static let root: URL = {
        // #filePath is this file; walk up to the folder holding go.mod.
        // Ref: https://developer.apple.com/documentation/swift/filepath()
        var dir = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        for _ in 0..<32 where !FileManager.default.fileExists(atPath: dir.path + "/go.mod") {
            dir = dir.deletingLastPathComponent()
        }
        precondition(FileManager.default.fileExists(atPath: dir.path + "/go.mod"), "found go.mod")
        return dir
    }()

    private static var built: Result<URL, Error>?

    /// The folder holding the built `givens` and `pickles` commands.
    static func binaries() throws -> URL {
        if let built { return try built.get() }
        let result = Result { try build() }
        built = result
        return try result.get()
    }

    /// The `givens` command, for ServeClient and for steps that run it in a shell.
    static func givens() throws -> URL { try binaries().appendingPathComponent("givens") }

    /// Builds cmd/givens and tools/pickles into a temporary folder.
    /// Ref: https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies
    private static func build() throws -> URL {
        let bin = FileManager.default.temporaryDirectory
            .appendingPathComponent("givens-swift-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: bin, withIntermediateDirectories: true)
        let (status, output) = try Shell.run(
            "go build -o \(Shell.quote(bin.path))/ ./cmd/givens ./tools/pickles", in: root)
        guard status == 0 else { throw StepFailure("go build failed:\n\(output)") }
        precondition(FileManager.default.fileExists(atPath: bin.path + "/givens"), "built givens")
        return bin
    }

    /// The pickles of any .feature file, compiled by tools/pickles.
    static func pickles(of feature: URL) throws -> [Pickle] {
        precondition(feature.pathExtension == "feature", "pickles come from a .feature file")
        let pickles = try binaries().appendingPathComponent("pickles")
        let (status, output) = try Shell.run(
            "\(Shell.quote(pickles.path)) \(Shell.quote(feature.path))", in: root, stderr: false)
        guard status == 0 else {
            throw StepFailure("pickles failed on \(feature.path):\n\(output)")
        }
        return try JSONDecoder().decode([Pickle].self, from: Data(output.utf8))
    }

    /// A capability's .feature file in Givens's own spec.
    static func feature(_ capability: String) -> URL {
        precondition(!capability.contains("/"), "a capability is a slug")
        return root.appendingPathComponent("capabilities/\(capability)/\(capability).feature")
    }
}

/// Runs bash commands. Ref: https://developer.apple.com/documentation/foundation/process
enum Shell {
    /// Runs `command` with bash in `directory`, with the folders in `path` first on PATH.
    /// A non-zero exit isn't an error; callers check the status.
    static func run(
        _ command: String, in directory: URL, path: [URL] = [], stderr: Bool = true
    ) throws -> (status: Int32, output: String) {
        precondition(!command.isEmpty, "a command is given")
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/bin/bash")
        process.arguments = ["-c", command]
        process.currentDirectoryURL = directory
        var environment = ProcessInfo.processInfo.environment
        let prefix = path.map(\.path) + [environment["PATH"] ?? "/usr/bin:/bin"]
        environment["PATH"] = prefix.joined(separator: ":")
        process.environment = environment
        let pipe = Pipe()
        process.standardOutput = pipe
        process.standardError = stderr ? pipe : FileHandle.nullDevice
        try process.run()
        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        return (process.terminationStatus, String(decoding: data, as: UTF8.self))
    }

    /// Single-quotes `text` for bash. Ref: https://www.gnu.org/software/bash/manual/bash.html#Single-Quotes
    static func quote(_ text: String) -> String {
        "'" + text.replacingOccurrences(of: "'", with: #"'\''"#) + "'"
    }
}

/// A step's check didn't hold. Its message is the test failure.
struct StepFailure: Error, CustomStringConvertible {
    let description: String

    init(_ description: String) {
        precondition(!description.isEmpty, "a failure says what went wrong")
        self.description = description
    }
}
