// The wire format between the app and `givens serve`: one JSON object per line, both ways,
// over the serve process's stdin and stdout. Each method mirrors a call in internal/app, so a
// scenario that passes in Go passes here once Swift renders what the core returns.
// Ref: https://jsonlines.org
//
//   → {"id":1,"method":"open","params":{"path":"/code/shop/capabilities"}}
//   ← {"id":1,"snapshot":{…},"effects":[{"kind":"clipboard","text":"bash …"}],"value":…}
//   ← {"id":1,"error":{"code":"not_built","message":"not built yet: see todo.txt"}}
//   ← {"event":"snapshot","snapshot":{…}}      pushed when files change or setup prints
//
// Every reply carries the whole app snapshot, so the app never patches state by hand. Methods
// named "test.*" are test seams: serve refuses them unless started with GIVENS_TEST_SEAMS=1,
// which only the acceptance suites set.

/// Method names, one per internal/app call.
public enum ServeMethod {
    /// App.Launch.
    public static let launch = "launch"
    /// App.Open.
    public static let open = "open"
    /// App.Quit.
    public static let quit = "quit"
    /// App.OpenDropped.
    public static let openDropped = "openDropped"
    /// App.InstallCommand.
    public static let installCommand = "installCommand"
    /// Window.Select.
    public static let select = "window.select"
    /// Window.Choose.
    public static let choose = "window.choose"
    /// Window.Type.
    public static let type = "window.type"
    /// Window.Press.
    public static let press = "window.press"
    /// Window.Capability.
    public static let capability = "window.capability"
    /// Window.Scenario.
    public static let scenario = "window.scenario"
    /// Window.DropIn.
    public static let dropIn = "window.dropIn"
    /// Window.OpenRecent.
    public static let openRecent = "window.openRecent"
    /// Window.CloseWindow.
    public static let close = "window.close"
    /// Session.Wait.
    public static let wait = "session.wait"
    /// Session.Mark.
    public static let mark = "session.mark"
    /// Session.RecordRun.
    public static let recordRun = "session.recordRun"
    /// App.ShipKits; a test seam.
    public static let shipKits = "test.shipKits"
    /// Moves the core's clock; a test seam.
    public static let advanceClock = "test.advanceClock"
}

/// One request line.
struct ServeRequest<Params: Encodable & Sendable>: Encodable, Sendable {
    let id: Int
    let method: String
    let params: Params
}

/// One reply line. `Value` is what the method returns besides the snapshot.
public struct ServeReply<Value: Decodable & Sendable>: Decodable, Sendable {
    /// The request this answers.
    public let id: Int
    /// Set when the call failed; then nothing else is.
    public let error: CoreError?
    /// The whole app after the call.
    public let snapshot: AppSnapshot?
    /// Things only the app can do: write the clipboard, move to the Trash, show a picker.
    public let effects: [Effect]?
    /// The call's own result, such as a Capability.
    public let value: Value?
}

/// A reply with no value of its own.
public struct NoValue: Decodable, Sendable {}

/// A line serve pushes without being asked.
struct ServeEvent: Decodable, Sendable {
    let event: String
    let snapshot: AppSnapshot?
}

/// The first fields of any line, read to route it.
struct ServeLineHeader: Decodable, Sendable {
    let id: Int?
    let event: String?
}

/// An error the core returned: an operating error, not a bug.
public struct CoreError: Decodable, Sendable, Equatable, CustomStringConvertible {
    /// A stable code, like "not_built" or "no_folder".
    public let code: String
    /// What the core says, in the words a person would read.
    public let message: String

    /// The message, for test failures.
    public var description: String { "\(code): \(message)" }
}

/// Something serve asks the app to do, because only the app can.
public struct Effect: Decodable, Sendable, Equatable {
    /// "clipboard", "trash", "folderPicker", "openURL" or "bringToFront".
    public let kind: String
    /// The clipboard text, for "clipboard", or the link, for "openURL".
    public let text: String?
    /// The file to trash, or the window to raise.
    public let path: String?
}

/// Every window, front first, and the welcome window's recent list.
public struct AppSnapshot: Decodable, Sendable, Equatable {
    /// Open windows, front first.
    public let windows: [WindowSnapshot]
    /// Recent projects, newest first.
    public let recents: [String]
}
