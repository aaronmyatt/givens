import Foundation
import GherkinKit
import GivensUI

// Step bodies for running setup: output, the checklist, failures, missing commands, timeouts.

/// Polls `check` every 50ms until it's true, for at most 5 seconds: long enough for a setup
/// that's already finished, short enough that a scenario fails in seconds.
/// Ref: https://developer.apple.com/documentation/swift/task/sleep(for:tolerance:clock:)
@MainActor
func dropInEventually(_ check: @MainActor () async throws -> Bool) async throws -> Bool {
    for _ in 0..<100 {
        if try await check() { return true }
        try await Task.sleep(for: .milliseconds(50))
    }
    return try await check()
}

/// Watches `check` for about a second and reports whether it never became true.
@MainActor
func dropInStaysFalse(_ check: @MainActor () async throws -> Bool) async throws -> Bool {
    for _ in 0..<20 {
        if try await check() { return false }
        try await Task.sleep(for: .milliseconds(50))
    }
    return try await !check()
}

/// The index of the first line containing `text` at or after `from`, or nil.
func dropInIndex(_ lines: [String], _ text: String, from: Int = 0) -> Int? {
    guard from < lines.count else { return nil }
    return lines[from...].firstIndex { $0.contains(text) }
}

/// True when the core refused a call with an operating error. Any other error, like serve not
/// running, is rethrown, so a refusal is never mistaken for a broken connection.
@MainActor
func dropInRefused(_ action: @MainActor () async throws -> Void) async throws -> Bool {
    do {
        try await action()
        return false
    } catch GivensError.core {
        return true
    }
}

extension DropIntoAScenarioState {
    /// Gives Card declined a setup that prints one line, then waits for a step to release it
    /// before printing the next, so a step can see output arrive while setup runs.
    func printsThenWaits() async throws {
        try writeSetup(
            "Card declined",
            Self.script(#"ok "seeded shopper""#, Self.waitForRelease, #"ok "card set to decline""#))
        try await prepare()
    }

    /// Expects the first line while setup is still running, then the rest, in order.
    func outputStreams() async throws {
        let session = try await session()
        guard try await dropInEventually({ dropInIndex(session.output, Self.firstLine) != nil })
        else {
            throw StepFailure("the first line never appeared while setup ran: \(session.output)")
        }
        guard session.status == SessionModel.running else {
            throw StepFailure("setup is \"\(session.status)\" while it waits, not running")
        }
        try expectNot(session.output, Self.secondLine, "the output before setup carried on")
        try release()
        try await settleTo(SessionModel.ready)
        let output = session.output
        guard let first = dropInIndex(output, Self.firstLine),
            let second = dropInIndex(output, Self.secondLine), first < second
        else { throw StepFailure("the output is out of order or incomplete: \(output)") }
        if let joined = output.first(where: { $0.contains("\n") }) {
            throw StepFailure("an output entry holds more than one line: \(joined)")
        }
    }

    /// Expects the checklist to be any manual Givens, then the When steps, then the Thens.
    func checklistShown() async throws {
        let session = try await settleTo(SessionModel.ready)
        let (whens, thens) = try expectedChecks("Card declined")
        let order = ["given": 0, "when": 1, "then": 2]
        var last = 0
        for item in session.checklist {
            guard let rank = order[item.kind], rank >= last else {
                throw StepFailure("the checklist isn't Givens, Whens, Thens: \(session.checklist)")
            }
            last = rank
        }
        try dropInExpectKinds(session.checklist, ["when": whens, "then": thens])
    }

    /// Marks each Then step "pass", then "fail", and expects a When step to refuse "pass".
    func thenStepsMarkable() async throws {
        let session = try await session()
        let when = try dropInFirst(session.checklist, "when")
        if try await !dropInRefused({ try await session.mark(when.text, "pass") }) {
            throw StepFailure("the When step \"\(when.text)\" was marked \"pass\"")
        }
        for item in session.checklist where item.kind == "then" {
            for mark in ["pass", "fail"] {
                try await session.mark(item.text, mark)
                let got = session.checklist.first { $0.text == item.text }?.mark ?? ""
                guard got == mark else {
                    throw StepFailure("\"\(item.text)\" shows \"\(got)\" after marking \(mark)")
                }
            }
        }
    }

    /// Gives Card declined a setup that exits 3 after two lines.
    func setupStopsPartWay() async throws {
        try writeSetup(
            "Card declined",
            Self.script(
                #"ok "seeded shopper""#, #"ok "cart filled""#, "exit 3",
                #"ok "card set to decline""#))
        try await prepare()
    }

    func outputUpToStop() async throws {
        let output = try await settleTo(SessionModel.failed).output
        try expect(output, Self.firstLine, "the output")
        try expect(output, "✓ cart filled", "the output")
        try expectNot(output, Self.secondLine, "the output")
    }

    func noChecklist() async throws {
        let session = try await settleTo(SessionModel.failed)
        guard session.checklist.isEmpty else {
            throw StepFailure("a checklist is shown: \(session.checklist)")
        }
    }

    /// Gives Card declined a setup whose first line leaves a marker, then needs shopctl.
    /// Givens reads need lines before running anything, so the marker must never appear.
    func needsShopctl() async throws {
        try w.copySampleShop()
        let found = try Shell.run("command -v shopctl", in: w.dir)
        guard found.status != 0 else {
            throw StepFailure("shopctl is installed at \(found.output), so this can't be set up")
        }
        markers.append("ran-card-declined")
        try writeSetup(
            "Card declined",
            Self.script(
                Self.touch("ran-card-declined"), #"need shopctl "brew install shop/tap/shopctl""#,
                #"ok "seeded shopper""#))
        try await prepare()
    }

    /// Expects setup to have stopped before running a line: no kit output, and no marker a
    /// script line would have written.
    func nothingRuns() async throws {
        let session = try await settleTo(SessionModel.failed)
        if let line = session.output.first(where: { $0.hasPrefix("✓") || $0.hasPrefix("✗") }) {
            throw StepFailure("the script ran: its output has \"\(line)\"")
        }
        for marker in markers where shopFile(marker) != nil {
            throw StepFailure("the script ran: it wrote shop-data/\(marker)")
        }
    }

    func installHint(_ hint: String) async throws {
        let window = try await w.front()
        try expect(window.visible, hint, "the window")
        try await window.choose("Copy")
        let copied = w.clipboard.readText().trimmingCharacters(in: .whitespacesAndNewlines)
        guard copied == hint else { throw StepFailure("Copy put \"\(copied)\", not \"\(hint)\"") }
    }

    /// Gives Card declined a setup that blocks until released, writing its process id so a
    /// step can check "Stop" ended it.
    func setupTakesTooLong() async throws {
        try writeSetup(
            "Card declined",
            Self.script(Self.writePID, #"ok "started""#, Self.waitForRelease, #"ok "finished""#))
        try await prepare()
    }

    func afterTwoMinutes(_ text: String) async throws {
        if try await shows(Self.stillRunning) {
            throw StepFailure("\"Still running\" shows before two minutes have passed")
        }
        try await advance(seconds: Self.slowSetupSeconds)
        guard try await dropInEventually({ try await shows(text) }) else {
            throw StepFailure("two minutes on, the window doesn't show \"\(text)\"")
        }
    }
}
