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
        def(#"^they attach the file "([^"]*)"$"#) { call in try await s.attachFile(call[0]) },
        def(#"^"([^"]*)" is copied into the run's folder in runs/$"#) { call in
            try s.copiedIntoRunFolder(call[0])
        },
        def(#"^the run, in the scenario's run history, lists "([^"]*)"$"#) { call in
            try await s.historyListsFile(call[0])
        },
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
        scenario = name
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

    /// Writes a stand-in screenshot outside the project, as a tester's own file would be, and
    /// attaches it to the Drop in.
    func attachFile(_ name: String) async throws {
        let desktop = w.dir.appendingPathComponent("desktop")
        try FileManager.default.createDirectory(at: desktop, withIntermediateDirectories: true)
        let file = desktop.appendingPathComponent(name)
        try Data("proof of \(name)".utf8).write(to: file)
        attached = file.path
        try await session().attach(attached)
    }

    /// The newest run folder under runs/files/<run key>/ holds a copy of the attached file,
    /// and the tester's own file is still where it was. Folders are named by time, so the
    /// newest sorts last.
    func copiedIntoRunFolder(_ name: String) throws {
        let folder = w.capabilities.appendingPathComponent("runs/files/\(runKey)")
        let runs = (try? FileManager.default.contentsOfDirectory(atPath: folder.path)) ?? []
        guard let newest = runs.max() else {
            throw StepFailure("runs/files/\(runKey) has no run folder")
        }
        let copied = folder.appendingPathComponent(newest).appendingPathComponent(name)
        let content = (try? String(contentsOf: copied, encoding: .utf8)) ?? ""
        guard content == "proof of \(name)" else {
            throw StepFailure("\(copied.path) holds \"\(content)\", not the attached file")
        }
        guard FileManager.default.fileExists(atPath: attached) else {
            throw StepFailure("the tester's own \(name) was moved")
        }
    }

    /// Selects the scenario and checks its newest run lists the file.
    func historyListsFile(_ name: String) async throws {
        let window = try await w.front()
        try await window.select(scenario)
        guard let newest = window.snapshot?.runs.first else {
            throw StepFailure("the scenario's page lists no runs")
        }
        guard newest.files.contains(where: { URL(fileURLWithPath: $0).lastPathComponent == name })
        else { throw StepFailure("the newest run lists \(newest.files), not \(name)") }
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
