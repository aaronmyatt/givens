import Foundation

/// Why a call into the core didn't return a result.
public enum GivensError: Error, Equatable, CustomStringConvertible {
    /// `givens serve` couldn't start, or stopped; carries what it printed.
    case serveUnavailable(String)
    /// The core answered with an operating error.
    case core(CoreError)
    /// No reply within the call's limit.
    case timedOut(String)
    /// A reply that doesn't match ServeProtocol.swift.
    case badReply(String)

    /// The reason, for test failures and alerts.
    public var description: String {
        switch self {
        case .serveUnavailable(let detail): "givens serve isn't available: \(detail)"
        case .core(let error): error.message
        case .timedOut(let method): "givens serve didn't answer \(method) in time"
        case .badReply(let detail): "givens serve sent a reply Swift can't read: \(detail)"
        }
    }
}

/// Runs `givens serve` as a child process and talks to it in JSON lines over its stdin and
/// stdout. One client per running app; the app's models are its only callers.
/// Ref: https://developer.apple.com/documentation/foundation/process
@MainActor
public final class ServeClient {
    /// How to start serve.
    public struct Configuration: Sendable {
        /// The givens command, built from cmd/givens.
        public let executable: URL
        /// Givens's own data: trusted folders, recents, open windows. Never in a project.
        public let appDataDirectory: URL
        /// Lets "test.*" methods through. Only the acceptance suites set it.
        public let testSeams: Bool
        /// The longest one call may take.
        public let callTimeout: Duration

        /// Makes a configuration.
        public init(
            executable: URL, appDataDirectory: URL, testSeams: Bool = false,
            callTimeout: Duration = .seconds(10)
        ) {
            precondition(executable.isFileURL, "serve is a local command")
            precondition(appDataDirectory.isFileURL, "app data is a local folder")
            self.executable = executable
            self.appDataDirectory = appDataDirectory
            self.testSeams = testSeams
            self.callTimeout = callTimeout
        }
    }

    /// Limits. Tiger Style: put a limit on everything.
    static let lineLengthMax = 1 << 24
    static let pendingCallsMax = 64
    static let strayLinesMax = 32

    let configuration: Configuration
    /// Called on the main actor with each snapshot serve pushes unasked.
    public var onSnapshot: ((AppSnapshot) -> Void)?

    private var process: Process?
    private var nextID = 0
    private var pending: [Int: CheckedContinuation<Data, Error>] = [:]
    private var buffer = Data()
    /// Lines that weren't JSON, kept to explain a serve that failed to start.
    private var strayLines: [String] = []

    /// Makes a client; serve starts on the first call.
    public init(_ configuration: Configuration) {
        self.configuration = configuration
    }

    /// Sends one request and waits for its reply. An error reply throws `.core`.
    public func call<Value: Decodable & Sendable>(
        _ method: String, _ params: some Encodable & Sendable = [String: String](),
        returning: Value.Type = NoValue.self
    ) async throws -> ServeReply<Value> {
        precondition(!method.isEmpty, "a call names a method")
        precondition(pending.count < Self.pendingCallsMax, "pending calls fit pendingCallsMax")
        let input = try startIfNeeded()
        nextID += 1
        let id = nextID
        var line = try JSONEncoder().encode(ServeRequest(id: id, method: method, params: params))
        line.append(0x0A)
        let data = try await withCheckedThrowingContinuation { continuation in
            pending[id] = continuation
            do {
                try input.write(contentsOf: line)
            } catch {
                pending[id] = nil
                continuation.resume(throwing: GivensError.serveUnavailable("\(error)"))
                return
            }
            scheduleTimeout(id, method)
        }
        let reply: ServeReply<Value>
        do {
            reply = try JSONDecoder().decode(ServeReply<Value>.self, from: data)
        } catch {
            throw GivensError.badReply("\(method): \(error)")
        }
        if let error = reply.error { throw GivensError.core(error) }
        return reply
    }

    /// Stops serve. Pending calls fail.
    public func stop() {
        process?.terminate()
        process = nil
        failPending(.serveUnavailable("stopped"))
        precondition(pending.isEmpty, "stopping answers every pending call")
    }

    /// Starts serve once and returns the pipe to write requests to.
    private func startIfNeeded() throws -> FileHandle {
        if let input = process?.standardInput as? Pipe { return input.fileHandleForWriting }
        let child = Process()
        child.executableURL = configuration.executable
        child.arguments = ["serve", "--app-data", configuration.appDataDirectory.path]
        var environment = ProcessInfo.processInfo.environment
        environment["GIVENS_TEST_SEAMS"] = configuration.testSeams ? "1" : nil
        child.environment = environment
        let input = Pipe()
        let output = Pipe()
        child.standardInput = input
        child.standardOutput = output
        child.standardError = output
        // readabilityHandler runs on a background queue; hop to the main actor to parse.
        // Ref: https://developer.apple.com/documentation/foundation/filehandle/readabilityhandler
        output.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let chunk = handle.availableData
            Task { @MainActor in self?.receive(chunk) }
        }
        child.terminationHandler = { [weak self] ended in
            let status = ended.terminationStatus
            Task { @MainActor in self?.ended(status) }
        }
        do {
            try child.run()
        } catch {
            throw GivensError.serveUnavailable("\(configuration.executable.path): \(error)")
        }
        process = child
        return input.fileHandleForWriting
    }

    /// Splits output into lines and routes each one.
    private func receive(_ chunk: Data) {
        buffer.append(chunk)
        precondition(buffer.count <= Self.lineLengthMax, "a serve line fits lineLengthMax")
        // bound: each pass removes one line from buffer, which is finite.
        while let newline = buffer.firstIndex(of: 0x0A) {
            let line = buffer[buffer.startIndex..<newline]
            buffer.removeSubrange(buffer.startIndex...newline)
            route(Data(line))
        }
    }

    /// Hands a reply to its waiting call, or a pushed snapshot to `onSnapshot`.
    private func route(_ line: Data) {
        guard let header = try? JSONDecoder().decode(ServeLineHeader.self, from: line) else {
            if strayLines.count < Self.strayLinesMax {
                strayLines.append(String(decoding: line, as: UTF8.self))
            }
            return
        }
        if let id = header.id, let continuation = pending.removeValue(forKey: id) {
            continuation.resume(returning: line)
            return
        }
        if header.event != nil,
            let event = try? JSONDecoder().decode(ServeEvent.self, from: line),
            let snapshot = event.snapshot
        {
            onSnapshot?(snapshot)
        }
    }

    /// Fails every pending call when serve exits, after reading what it printed last.
    private func ended(_ status: Int32) {
        if let output = process?.standardOutput as? Pipe {
            output.fileHandleForReading.readabilityHandler = nil
            receive(output.fileHandleForReading.readDataToEndOfFile())
        }
        process = nil
        let said = strayLines.joined(separator: "\n")
        failPending(.serveUnavailable("givens serve exited \(status)\n\(said)"))
    }

    private func scheduleTimeout(_ id: Int, _ method: String) {
        let limit = configuration.callTimeout
        Task { @MainActor [weak self] in
            try? await Task.sleep(for: limit)
            self?.pending.removeValue(forKey: id)?.resume(throwing: GivensError.timedOut(method))
        }
    }

    private func failPending(_ error: GivensError) {
        let waiting = pending
        pending.removeAll()
        for continuation in waiting.values { continuation.resume(throwing: error) }
    }
}
