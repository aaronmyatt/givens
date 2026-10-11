import Foundation
import GherkinKit
import GivensUI

// Recording runs in runs/, and merging them in git.

/// Recording runs, who ran them, and merging runs from two branches.
@MainActor
func dropInRecordSteps(_ s: DropIntoAScenarioState) -> [StepDefinition] {
    [
        def(#"^a developer has dropped in to "([^"]*)" and marked every Then step "Pass"$"#) {
            call in try await s.droppedInAllPassed(call[0])
        },
        def(#"^they add the note "([^"]*)"$"#) { call in
            s.note = call[0]
            try await s.session().setRunNote(call[0])
        },
        def(
            #"^a line for (\S+) is added to today's file in runs/, with the result and their note$"#
        ) { call in try s.runLineWithNote(call[0]) },
        def(#"^a developer has dropped in to "([^"]*)" and marked only the first Then step$"#) {
            call in try await s.droppedInFirstMarked(call[0])
        },
        def(#"^nothing is added to runs/$"#) { _ in try s.nothingInRuns() },
        def(
            #"^a developer whose git user\.name is "([^"]*)" has dropped in to "([^"]*)" and marked every Then step "Pass"$"#
        ) { call in
            try s.writeSetup(call[1], DropIntoAScenarioState.quickSetup)
            try s.gitInit(call[0])
            try await s.droppedInAllPassed(call[1])
        },
        def(#"^the run line includes "([^"]*)"$"#) { call in
            let fields = try s.runLines(s.runKey).last ?? []
            guard dropInHasField(fields, call[0]) else {
                throw StepFailure("the run line for \(s.runKey) lacks \"\(call[0])\": \(fields)")
            }
        },
        def(#"^Sam and Ali each recorded a run for "([^"]*)" today, on different branches$"#) {
            call in try await s.runsOnTwoBranches(call[0])
        },
        def(#"^the branches are merged$"#) { _ in try s.mergeBranches() },
        def(#"^git merges runs/ with no conflict$"#) { _ in
            guard s.mergeExit == 0 && !s.mergeOutput.contains("CONFLICT") else {
                throw StepFailure("git merge exited \(s.mergeExit):\n\(s.mergeOutput)")
            }
        },
        def(#"^both run lines are kept$"#) { _ in try s.bothLinesKept() },
    ]
}

/// True when one of `fields` is exactly `want`, ignoring spaces around it.
func dropInHasField(_ fields: [String], _ want: String) -> Bool {
    fields.contains { $0.trimmingCharacters(in: .whitespaces) == want }
}

extension DropIntoAScenarioState {
    /// Drops in to a quick setup and marks every check, Thens "pass".
    func droppedInAllPassed(_ name: String) async throws {
        try writeSetup(name, Self.quickSetup)
        try await prepare()
        runKey = try runKeyOf(name)
        try await dropInMarkAll(try await dropInTo(name, SessionModel.ready))
    }

    /// Drops in to a quick setup and marks only the first Then step "pass".
    func droppedInFirstMarked(_ name: String) async throws {
        try writeSetup(name, Self.quickSetup)
        try await prepare()
        let session = try await dropInTo(name, SessionModel.ready)
        try await session.mark(try dropInFirst(session.checklist, "then").text, "pass")
    }

    /// Today's run file: the core's clock names it YYYY-MM-DD.log, and the newest is today's.
    func todaysRuns() throws -> String {
        let folder = w.capabilities.appendingPathComponent("runs")
        let names = (try? FileManager.default.contentsOfDirectory(atPath: folder.path)) ?? []
        let logs = names.filter {
            $0.range(of: #"^\d{4}-\d{2}-\d{2}\.log$"#, options: .regularExpression) != nil
        }
        guard let newest = logs.max() else {
            throw StepFailure("runs/ has no YYYY-MM-DD.log file: \(names)")
        }
        return "capabilities/runs/" + newest
    }

    /// The tab-separated fields of each line in today's run file for `key`.
    func runLines(_ key: String) throws -> [[String]] {
        let path = try todaysRuns()
        let text = try w.readFile(path)
        let lines = text.split(separator: "\n").map { $0.components(separatedBy: "\t") }
        let found = lines.filter { dropInHasField($0, key) }
        guard !found.isEmpty else { throw StepFailure("\(path) has no line for \(key):\n\(text)") }
        return found
    }

    func runLineWithNote(_ key: String) throws {
        let fields = try runLines(key).last ?? []
        guard dropInHasField(fields, "pass") || dropInHasField(fields, "passed") else {
            throw StepFailure("the run line for \(key) doesn't record a pass: \(fields)")
        }
        guard dropInHasField(fields, note) else {
            throw StepFailure(
                "the run line for \(key) doesn't hold the note \"\(note)\": \(fields)")
        }
    }

    func nothingInRuns() throws {
        let folder = w.capabilities.appendingPathComponent("runs")
        let names = (try? FileManager.default.contentsOfDirectory(atPath: folder.path)) ?? []
        for name in names {
            let text = try String(
                contentsOf: folder.appendingPathComponent(name), encoding: .utf8)
            if !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                throw StepFailure("runs/\(name) was written:\n\(text)")
            }
        }
    }

    /// Runs git in the sample shop, away from this Mac's own git settings, and returns its
    /// output. A non-zero exit throws unless `allowFailure`.
    /// Ref: https://git-scm.com/docs/git#Documentation/git.txt-codeGITCONFIGGLOBALcode
    @discardableResult
    func git(_ args: [String], allowFailure: Bool = false) throws -> (Int32, String) {
        guard let project = w.project else { throw StepFailure("no sample shop for git") }
        let command =
            "GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 git "
            + args.map(Shell.quote).joined(separator: " ")
        let result = try Shell.run(command, in: project)
        guard allowFailure || result.status == 0 else {
            throw StepFailure("git \(args.joined(separator: " ")) failed:\n\(result.output)")
        }
        return (result.status, result.output)
    }

    /// Makes the sample shop a repository whose user.name is `user`, all committed on main.
    /// Ref: https://git-scm.com/docs/git-init and https://git-scm.com/docs/git-config
    func gitInit(_ user: String) throws {
        try w.copySampleShop()
        try git(["init", "--quiet", "--initial-branch=main"])
        try git(["config", "user.name", user])
        try git(["config", "user.email", user.lowercased() + "@example.com"])
        try git(["config", "commit.gpgsign", "false"])
        try git(["add", "--all"])
        try git(["commit", "--quiet", "--message", "Sample shop"])
    }
}
