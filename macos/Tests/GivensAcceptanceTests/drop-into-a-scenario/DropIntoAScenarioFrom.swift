import Foundation
import GherkinKit
import GivensUI

// Setups that build on other scenarios with `from`, and from loops run in a terminal.

/// Setups that build on other scenarios with `from`.
@MainActor
func dropInFromSteps(_ s: DropIntoAScenarioState) -> [StepDefinition] {
    [
        def(#"^"([^"]*)" has a setup that prints "([^"]*)"$"#) { call in
            try await s.setupPrints(call[0], call[1])
        },
        def(#"^the output shows "([^"]*)" followed by that scenario's setup$"#) { call in
            try await s.fromFollowedBySetup(call[0])
        },
        def(#"^the checklist has only the steps of "([^"]*)"$"#) { call in
            let session = try await s.settleTo(SessionModel.ready)
            let (whens, thens) = try s.expectedChecks(call[0])
            try dropInExpectKinds(session.checklist, ["when": whens, "then": thens])
        },
        def(#"^both scenarios have a teardown script$"#) { _ in try await s.bothHaveTeardowns() },
        def(#"^the output shows 3-D Secure challenge's teardown first$"#) { _ in
            try await s.inOrder(
                "→ teardown", "✓ 3-d-secure-challenge torn down",
                "→ from checkout/pay-with-saved-card")
        },
        def(#"^then "([^"]*)", followed by Pay with saved card's teardown and setup$"#) { call in
            try await s.inOrder(
                "✓ 3-d-secure-challenge torn down", call[0], "✓ pay-with-saved-card torn down",
                "✓ saved card added")
        },
        def(#"^then the rest of 3-D Secure challenge's setup$"#) { _ in
            try await s.inOrder("✓ saved card added", "✓ challenge mocked")
        },
        def(#"^"([^"]*)" starts with "([^"]*)"$"#) { call in
            let marker = "ran-" + slug(call[0])
            s.markers.append(marker)
            try s.writeSetup(
                call[0],
                DropIntoAScenarioState.scriptFrom(call[1], DropIntoAScenarioState.touch(marker)))
            try await s.prepare()
        },
        def(#"^the same loop as from-loop$"#) { _ in try s.sameLoop() },
        def(#"^a developer runs "([^"]*)" from the project's root folder$"#) { call in
            try await s.runFromRoot(call[0])
        },
        def(#"^the kit prints "([^"]*)"$"#) { call in
            try expect(s.w.output.components(separatedBy: "\n"), call[0], "the script's output")
        },
        def(#"^the script stops with a non-zero exit code$"#) { _ in
            guard s.w.exitCode != 0 else {
                throw StepFailure("the script exited 0:\n\(s.w.output)")
            }
        },
        def(#"^the output shows "([^"]*)"$"#) { call in
            try expect(try await s.session().output, call[0], "the setup's output")
        },
    ]
}

extension DropIntoAScenarioState {
    /// Gives the named scenario a setup printing one line through the kit's ok, which adds
    /// the "✓ " the step quotes.
    func setupPrints(_ name: String, _ line: String) async throws {
        guard line.hasPrefix("✓ ") else {
            throw StepFailure("\"\(line)\" isn't a line the kit's ok prints")
        }
        try writeSetup(name, Self.script("ok " + Shell.quote(String(line.dropFirst(2)))))
        try await prepare()
    }

    /// Expects the from line, then the ok lines of the setup it names, in order. It reads that
    /// setup from the sample shop, so it checks whatever that script prints.
    func fromFollowedBySetup(_ line: String) async throws {
        let output = try await session().output
        guard let at = dropInIndex(output, line) else {
            throw StepFailure("the output has no \"\(line)\": \(output)")
        }
        let target =
            line.replacingOccurrences(of: "→ from ", with: "")
            .split(separator: " ").first.map(String.init) ?? ""
        let parts = target.split(separator: "/").map(String.init)
        guard parts.count == 2 else { throw StepFailure("\"\(line)\" names no scenario") }
        let path = "capabilities/\(parts[0])/scripts/\(parts[1]).sh"
        let text = try w.readFile(path)
        // The messages a script prints with ok, like ok "seeded shopper".
        // Ref: https://developer.apple.com/documentation/swift/regex
        let okLine = try Regex(#"(?m)^\s*ok\s+["']([^"'$]+)["']"#)
        let messages = text.matches(of: okLine).compactMap { $0.output[1].substring }
        guard !messages.isEmpty else { throw StepFailure("\(path) prints no ok line") }
        var next = at + 1
        for message in messages {
            guard let found = dropInIndex(output, "✓ " + message, from: next) else {
                throw StepFailure("\"\(message)\" doesn't follow \"\(line)\": \(output)")
            }
            next = found + 1
        }
    }

    /// Gives Pay with saved card and 3-D Secure challenge a teardown and a setup each printing
    /// one line named after its scenario.
    func bothHaveTeardowns() async throws {
        for name in [Self.payWithSavedCard, Self.threeDSChallenge] {
            try writeTeardown(name, Self.script("ok \"\(slug(name)) torn down\""))
        }
        try writeSetup(Self.payWithSavedCard, Self.script(#"ok "saved card added""#))
        try writeSetup(
            Self.threeDSChallenge,
            Self.scriptFrom("from checkout/pay-with-saved-card", #"ok "challenge mocked""#))
        try await prepare()
    }

    /// Fails unless each line appears in the finished Drop in's output, in the order given.
    func inOrder(_ lines: String...) async throws {
        let output = try await settleTo(SessionModel.ready).output
        var previous = -1
        for line in lines {
            guard let at = dropInIndex(output, line) else {
                throw StepFailure("the output has no \"\(line)\": \(output)")
            }
            guard at > previous else {
                throw StepFailure("\"\(line)\" comes too early in the output: \(output)")
            }
            previous = at
        }
    }

    /// Writes the loop from "Setups that build on each other in a loop", without Givens.
    func sameLoop() throws {
        let loop = [
            Self.payWithSavedCard: "from checkout/3-d-secure-challenge",
            Self.threeDSChallenge: "from checkout/pay-with-saved-card",
        ]
        for (name, line) in loop {
            try writeSetup(name, Self.scriptFrom(line, "ok \"\(slug(name)) ran\""))
        }
    }

    /// Runs `command` with bash from the sample shop's root, for at most 20 seconds, so a loop
    /// the kit doesn't catch fails the step instead of hanging the suite. Output goes to a
    /// file, so a script that prints forever can't block on a full pipe.
    /// Ref: https://developer.apple.com/documentation/foundation/process
    func runFromRoot(_ command: String) async throws {
        guard let project = w.project else { throw StepFailure("no sample shop") }
        let log = w.dir.appendingPathComponent("terminal-run.log")
        FileManager.default.createFile(atPath: log.path, contents: nil)
        let handle = try FileHandle(forWritingTo: log)
        defer { try? handle.close() }
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/bin/bash")
        process.arguments = ["-c", command]
        process.currentDirectoryURL = project
        process.standardOutput = handle
        process.standardError = handle
        try process.run()
        for _ in 0..<200 where process.isRunning { try await Task.sleep(for: .milliseconds(100)) }
        let stillRunning = process.isRunning
        if stillRunning { process.terminate() }
        process.waitUntilExit()
        w.output = (try? String(contentsOf: log, encoding: .utf8)) ?? ""
        w.exitCode = process.terminationStatus
        if stillRunning {
            throw StepFailure("\"\(command)\" was still running after 20 seconds:\n\(w.output)")
        }
    }
}
