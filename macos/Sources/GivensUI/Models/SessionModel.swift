import Foundation

/// One Drop in: a setup run and the checklist that follows. It mirrors internal/app's Session.
/// It's a view of the window's latest snapshot, so it never goes stale.
@MainActor
public struct SessionModel {
    /// Session states, matching internal/app.
    public static let running = "running"
    /// Setup is paused at a card.
    public static let waiting = "waiting"
    /// Setup finished; the checklist is shown.
    public static let ready = "ready"
    /// Setup or a card failed.
    public static let failed = "failed"
    /// The developer chose "Stop".
    public static let stopped = "stopped"

    let window: WindowModel
    let record: SessionRecord

    /// The scenario dropped in to.
    public var scenario: String { record.scenario }
    /// One of the states above.
    public var status: String { current.status }
    /// Setup output so far.
    public var output: [String] { current.output }
    /// Cards setup is paused at.
    public var cards: [String] { current.cards }
    /// The checklist: manual Givens, then Whens, then Thens.
    public var checklist: [CheckItem] { current.checklist }

    /// The window's latest session, or the one this was made from if it has closed.
    var current: SessionRecord { window.snapshot?.session ?? record }

    /// Waits up to `seconds` for setup to finish or pause, then returns the status.
    @discardableResult
    public func wait(seconds: Int) async throws -> String {
        precondition(seconds > 0, "a wait has a limit")
        try await window.call(ServeMethod.wait, ["seconds": String(seconds)])
        return status
    }

    /// Marks a checklist item: "done" for Givens and Whens, "pass" or "fail" for Thens.
    public func mark(_ item: String, _ mark: String) async throws {
        precondition(["done", "pass", "fail"].contains(mark), "a mark is done, pass or fail")
        try await window.call(ServeMethod.mark, ["item": item, "mark": mark])
    }

    /// Records the run with a note, which may be "".
    public func recordRun(note: String = "") async throws {
        precondition(note.count <= 1024, "a note fits NameLengthMax")
        try await window.call(ServeMethod.recordRun, ["note": note])
    }
}
