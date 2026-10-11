import Foundation
import Observation

/// One running Givens, as the SwiftUI scenes see it. It mirrors internal/app's App: every
/// action is a serve call, and every reply replaces the windows with the core's snapshot.
/// Ref: https://developer.apple.com/documentation/observation/observable()
@MainActor
@Observable
public final class AppModel {
    /// Limits, matching internal/app.
    public static let windowsMax = 64
    /// Folders one Dock drop may open.
    public static let droppedPathsMax = windowsMax

    /// Open windows, front first.
    public private(set) var windows: [WindowModel] = []
    /// Recent projects, newest first.
    public private(set) var recents: [String] = []

    @ObservationIgnored let serve: ServeClient
    @ObservationIgnored let platform: Platform

    /// Makes an app that hasn't launched yet.
    public init(serve: ServeClient.Configuration, platform: Platform) {
        self.serve = ServeClient(serve)
        self.platform = platform
        self.serve.onSnapshot = { [weak self] snapshot in self?.apply(snapshot) }
    }

    /// The front window, or nil.
    public var front: WindowModel? { windows.first }

    /// Starts Givens: reopens the windows open at the last quit, or shows the welcome window.
    public func launch() async throws {
        try await send(ServeMethod.launch)
        precondition(!windows.isEmpty, "a launched app shows at least one window")
    }

    /// Opens a capabilities folder, or a project folder holding one, and returns its window.
    @discardableResult
    public func open(_ path: String) async throws -> WindowModel {
        precondition(path.hasPrefix("/"), "open takes an absolute path")
        try await send(ServeMethod.open, ["path": path])
        guard let window = front else { throw GivensError.badReply("open left no window") }
        return window
    }

    /// Opens folders dropped on the Dock icon. Called from application(_:open:).
    /// Ref: https://developer.apple.com/documentation/appkit/nsapplicationdelegate/application(_:open:)
    public func openDropped(_ paths: [String]) async throws {
        precondition(!paths.isEmpty, "a drop holds at least one folder")
        precondition(paths.count <= Self.droppedPathsMax, "a drop fits droppedPathsMax")
        try await send(ServeMethod.openDropped, ["paths": paths])
    }

    /// "Install givens command" in the Givens menu; returns what Givens says.
    public func installCommand(into binDirectory: String) async throws -> String {
        precondition(binDirectory.hasPrefix("/"), "the install folder is absolute")
        let reply = try await serve.call(
            ServeMethod.installCommand, ["binDir": binDirectory], returning: String.self)
        try apply(reply)
        return reply.value ?? ""
    }

    /// Closes every window, remembering which projects were open, and stops serve.
    public func quit() async throws {
        try await send(ServeMethod.quit)
        serve.stop()
        windows = []
        precondition(front == nil, "a quit app has no windows")
    }

    /// Stops serve without remembering windows, as a crash would. Tests tear down with it.
    public func stop() {
        serve.stop()
        windows = []
        precondition(front == nil, "a stopped app has no windows")
    }

    /// Sets the kit versions the core ships, oldest first. A test seam.
    public func shipKits(_ releases: [KitRelease]) async throws {
        precondition(!releases.isEmpty, "Givens ships at least one kit")
        try await send(ServeMethod.shipKits, ["releases": releases])
    }

    /// Moves the core's clock forward. A test seam, for "a day later".
    public func advanceClock(seconds: Int) async throws {
        precondition(seconds > 0, "time only moves forward")
        try await send(ServeMethod.advanceClock, ["seconds": seconds])
    }

    /// Sends a call with no value of its own and applies its reply.
    func send(_ method: String, _ params: some Encodable & Sendable = [String: String]())
        async throws
    {
        try apply(try await serve.call(method, params))
    }

    /// Applies a reply's snapshot, then its effects.
    func apply<Value>(_ reply: ServeReply<Value>) throws {
        if let snapshot = reply.snapshot { apply(snapshot) }
        for effect in reply.effects ?? [] { try perform(effect) }
    }

    /// Replaces the windows with the core's, keeping each WindowModel whose id survives so
    /// SwiftUI keeps its scene.
    func apply(_ snapshot: AppSnapshot) {
        precondition(snapshot.windows.count <= Self.windowsMax, "windows fit windowsMax")
        let existing = Dictionary(uniqueKeysWithValues: windows.map { ($0.id, $0) })
        windows = snapshot.windows.map { window in
            let model = existing[window.id] ?? WindowModel(app: self, id: window.id)
            model.snapshot = window
            return model
        }
        recents = snapshot.recents
    }

    /// Does what the core asked for and couldn't do itself.
    func perform(_ effect: Effect) throws {
        switch effect.kind {
        case "clipboard": platform.clipboard.writeText(effect.text ?? "")
        case "trash": try platform.trash.move(effect.path ?? "")
        case "folderPicker": platform.folderPicker.present()
        case "openURL": platform.urlOpener.open(effect.text ?? "")
        case "bringToFront": break  // the snapshot's order already says which window is front
        default: throw GivensError.badReply("unknown effect \(effect.kind)")
        }
    }
}
