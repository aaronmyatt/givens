import Darwin
import Foundation
import GherkinKit
import GivensUI

// Step bodies for timeouts and Stop, plus the checklist and feature-file helpers they share
// with the other areas.

extension DropIntoAScenarioState {
    /// Chooses "Stop", then expects the script's process gone and its output kept.
    func stopKeepsOutput() async throws {
        try await w.front().choose("Stop")
        let session = try await settleTo(SessionModel.stopped)
        let text = shopFile("setup.pid")?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard let pid = pid_t(text) else {
            throw StepFailure("the setup never wrote its process id")
        }
        // Signal 0 checks a process exists without signalling it.
        // Ref: https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/kill.2.html
        guard try await dropInEventually({ kill(pid, 0) != 0 }) else {
            throw StepFailure("the setup script (process \(pid)) is still running after Stop")
        }
        try expect(session.output, "✓ started", "the output after Stop")
        try expectNot(session.output, "✓ finished", "the output after Stop")
    }

    /// Sets the scenario's @timeout tag and gives it a setup that blocks until released.
    func tagTimeout(_ name: String, _ tag: String) async throws {
        try setTimeoutTag(name, tag)
        scenario = name
        try await setupTakesTooLong()
    }

    /// Moves the clock on by however long setup takes, watches for "Still running", then lets
    /// a blocked setup finish.
    func noStillRunning() async throws {
        if slowForSeconds > 0 { try await advance(seconds: slowForSeconds) }
        if try await !dropInStaysFalse({ try await shows(Self.stillRunning) }) { sawSlow = true }
        if sawSlow {
            throw StepFailure("the window shows \"Still running\": \(try await w.front().visible)")
        }
        try release()
    }

    func finishesAsNormal() async throws {
        try release()
        let session = try await settleTo(SessionModel.ready)
        guard !session.checklist.isEmpty else {
            throw StepFailure("setup finished but no checklist is shown")
        }
        try expect(session.output, "✓ finished", "the output")
    }

    /// Expects no "Still running" one minute in, then the message just past two.
    func usualTimeout() async throws {
        try await advance(seconds: 60)
        guard try await dropInStaysFalse({ try await shows(Self.stillRunning) }) else {
            throw StepFailure("\"Still running\" showed after one minute")
        }
        try await advance(seconds: Self.slowSetupSeconds - 60)
        let want = "Still running after 2 minutes"
        guard try await dropInEventually({ try await shows(want) }) else {
            throw StepFailure("two minutes on, the window doesn't show \"\(want)\"")
        }
        try release()
        try await settleTo(SessionModel.ready)
    }

    /// Replaces the named checkout scenario's @timeout tag with `tag`, or removes it when
    /// `tag` is "".
    func setTimeoutTag(_ name: String, _ tag: String) throws {
        try w.copySampleShop()
        let path = "capabilities/checkout/checkout.feature"
        var lines = try w.readFile(path).components(separatedBy: "\n")
        let trimmed = { (line: String) in line.trimmingCharacters(in: .whitespaces) }
        guard var at = lines.lastIndex(where: { trimmed($0) == "Scenario: \(name)" }) else {
            throw StepFailure("checkout.feature has no scenario \"\(name)\"")
        }
        var tags: [String] = []
        if at > 0 && trimmed(lines[at - 1]).hasPrefix("@") {
            tags = lines[at - 1].split(separator: " ").map(String.init)
            lines.remove(at: at - 1)
            at -= 1
        }
        var kept = tags.filter { !$0.hasPrefix("@timeout") }
        if !tag.isEmpty { kept.append(tag) }
        if !kept.isEmpty { lines.insert("  " + kept.joined(separator: " "), at: at) }
        try w.writeFile(path, lines.joined(separator: "\n"))
    }

    /// The named scenario's When and Then steps, read from the sample shop's copy. Pickles
    /// give And and But steps the type of the step before them.
    /// Ref: https://github.com/cucumber/messages/blob/main/messages.md#picklesteptype
    func expectedChecks(_ name: String) throws -> (whens: [String], thens: [String]) {
        let capability = try w.scenarioCapability(name)
        let feature = w.capabilities.appendingPathComponent("\(capability)/\(capability).feature")
        guard let pickle = try Toolchain.pickles(of: feature).first(where: { $0.name == name })
        else { throw StepFailure("\(feature.path) has no scenario \"\(name)\"") }
        let whens = pickle.steps.filter { $0.type == "Action" }.map(\.text)
        let thens = pickle.steps.filter { $0.type == "Outcome" }.map(\.text)
        return (whens, thens)
    }
}

/// Fails unless the items of each kind are exactly the wanted steps, in order.
func dropInExpectKinds(_ items: [CheckItem], _ wanted: [String: [String]]) throws {
    for (kind, steps) in wanted {
        let got = items.filter { $0.kind == kind }.map(\.text)
        guard got.count == steps.count else {
            throw StepFailure(
                "the checklist has \(got.count) \(kind) items, not \(steps.count): \(got)")
        }
        for (index, step) in steps.enumerated() where !got[index].contains(step) {
            throw StepFailure("\(kind) item \(index + 1) is \"\(got[index])\", not \"\(step)\"")
        }
    }
}

/// The first checklist item of `kind`, like "then".
func dropInFirst(_ items: [CheckItem], _ kind: String) throws -> CheckItem {
    guard let item = items.first(where: { $0.kind == kind }) else {
        throw StepFailure("the checklist has no \"\(kind)\" item: \(items)")
    }
    return item
}

/// Marks every checklist item: Givens and Whens "done", Thens "pass".
@MainActor
func dropInMarkAll(_ session: SessionModel) async throws {
    guard !session.checklist.isEmpty else {
        throw StepFailure("the checklist is empty, so there's nothing to mark")
    }
    for item in session.checklist {
        try await session.mark(item.text, item.kind == "then" ? "pass" : "done")
    }
}
