import Foundation
import GherkinKit
import GivensUI

// Merging runs from two branches, Reset state, idempotent setup and teardown scripts.

/// Reset state, idempotent setup and teardown scripts.
@MainActor
func dropInTeardownSteps(_ s: DropIntoAScenarioState) -> [StepDefinition] {
    [
        def(#"^a developer has dropped in to "([^"]*)" and ticked "([^"]*)"$"#) { call in
            try await s.droppedInTicked(call[0], call[1])
        },
        def(#"^the setup script runs again from the start$"#) { _ in try await s.setupRanAgain() },
        def(#"^the checklist is cleared$"#) { _ in try await s.checklistCleared() },
        def(#"^"([^"]*)" has no teardown script$"#) { call in try await s.noTeardown(call[0]) },
        def(#"^a developer has dropped in to "([^"]*)" once$"#) { call in
            try await s.dropInTo(call[0], SessionModel.ready)
        },
        def(#"^setup finishes without errors$"#) { _ in
            try expectNot(try await s.settleTo(SessionModel.ready).output, "✗", "the output")
        },
        def(#"^the shop has one shopper "([^"]*)", not two$"#) { call in
            let shoppers = s.shopFile("shoppers.txt") ?? ""
            let count = shoppers.components(separatedBy: "\n").filter { $0 == call[0] }.count
            guard count == 1 else { throw StepFailure("the shop has \(count) \(call[0]), not 1") }
        },
        def(#"^"([^"]*)" has a script called ([a-z0-9-]+\.teardown\.sh)$"#) { call in
            try await s.hasTeardown(call[0], call[1])
        },
        def(#"^a developer drops in, or chooses "Reset state" or "Try again"$"#) { _ in
            try await s.everyWayIn()
        },
        def(#"^the output shows "→ teardown" followed by the teardown's output$"#) { _ in
            try s.teardownFirst()
        },
        def(#"^then setup runs from the start$"#) { _ in try s.setupAfterTeardown() },
        def(
            #"^checkout/scripts/card-declined\.teardown\.sh is created, starting with the line that sources givens\.sh$"#
        ) { _ in
            let text = try s.w.readFile("capabilities/checkout/scripts/card-declined.teardown.sh")
            let first = text.components(separatedBy: "\n").first ?? ""
            guard first == kitSource else {
                throw StepFailure("the teardown starts with \"\(first)\", not the kit's line")
            }
        },
        def(#"^the "Card declined" teardown stops with an error$"#) { _ in
            try await s.teardownFails()
        },
        def(#"^setup doesn't run$"#) { _ in try await s.setupDidNotRun() },
    ]
}

extension DropIntoAScenarioState {
    /// Records a passing run as Sam on branch sam, and as Ali on branch ali, both branched
    /// from main, through Givens each time.
    func runsOnTwoBranches(_ name: String) async throws {
        try writeSetup(name, Self.quickSetup)
        let attributes = (try? w.readFile("capabilities/.gitattributes")) ?? ""
        if !attributes.contains("runs/** merge=union") {
            // A built-in merge driver. Ref: https://git-scm.com/docs/gitattributes#_built_in_merge_drivers
            try w.writeFile("capabilities/.gitattributes", attributes + "runs/** merge=union\n")
        }
        try gitInit("Sam")
        try await recordOnBranch(name, "sam", "Sam")
        try git(["checkout", "--quiet", "main"])
        // A later run on the same day, from a fresh Givens, as on Ali's Mac.
        try await advance(seconds: 3600)
        w.app?.stop()
        w.app = nil
        w.window = nil
        w.session = nil
        try await recordOnBranch(name, "ali", "Ali")
    }

    /// Starts `branch`, records a passing run as `user`, and commits it.
    /// Ref: https://git-scm.com/docs/git-checkout and https://git-scm.com/docs/git-commit
    func recordOnBranch(_ name: String, _ branch: String, _ user: String) async throws {
        try git(["checkout", "--quiet", "-b", branch])
        try git(["config", "user.name", user])
        try await droppedInAllPassed(name)
        try await session().recordRun()
        try git(["add", "--all"])
        try git(["commit", "--quiet", "--message", "\(user)'s run"])
    }

    /// Merges ali into sam, keeping git's output and exit code for the Then steps.
    /// Ref: https://git-scm.com/docs/git-merge
    func mergeBranches() throws {
        try git(["checkout", "--quiet", "sam"])
        (mergeExit, mergeOutput) = try git(["merge", "--no-edit", "ali"], allowFailure: true)
    }

    func bothLinesKept() throws {
        let lines = try runLines(runKey)
        for user in ["Sam", "Ali"] where !lines.contains(where: { dropInHasField($0, user) }) {
            throw StepFailure("after the merge, no run line for \(runKey) is by \(user): \(lines)")
        }
        guard lines.count == 2 else {
            throw StepFailure("after the merge there are \(lines.count) run lines, not 2: \(lines)")
        }
    }

    /// Drops in to a setup that counts its runs, and ticks the step containing `text`.
    func droppedInTicked(_ name: String, _ text: String) async throws {
        try writeSetup(
            name,
            Self.script("echo run >> " + Self.shopData + "/setup-runs\"", #"ok "seeded shopper""#))
        try await prepare()
        let session = try await dropInTo(name, SessionModel.ready)
        guard let item = session.checklist.first(where: { $0.text.contains(text) }) else {
            throw StepFailure("the checklist has no step \"\(text)\": \(session.checklist)")
        }
        try await session.mark(item.text, "done")
    }

    func setupRanAgain() async throws {
        let session = try await settleTo(SessionModel.ready)
        let runs = (shopFile("setup-runs") ?? "").components(separatedBy: "\n")
        let count = runs.filter { $0 == "run" }.count
        guard count == 2 else { throw StepFailure("setup ran \(count) times, not twice") }
        try expect(session.output, Self.firstLine, "the output after Reset state")
    }

    func checklistCleared() async throws {
        let items = try await session().checklist
        guard !items.isEmpty else { throw StepFailure("there's no checklist after Reset state") }
        if let marked = items.first(where: { !$0.mark.isEmpty }) {
            throw StepFailure("\"\(marked.text)\" is still marked \(marked.mark) after Reset state")
        }
    }

    /// Removes the scenario's teardown and gives it an idempotent setup: it seeds the shopper
    /// only if the shop doesn't have them yet. It selects the scenario, so its actions, such
    /// as "Add teardown", are offered.
    func noTeardown(_ name: String) async throws {
        try w.copySampleShop()
        guard let project = w.project else { throw StepFailure("no sample shop") }
        let teardown = project.appendingPathComponent(try teardownPath(name))
        if FileManager.default.fileExists(atPath: teardown.path) {
            try FileManager.default.removeItem(at: teardown)
        }
        let shoppers = Self.shopData + "/shoppers.txt\""
        try writeSetup(
            name,
            Self.script(
                "touch " + shoppers,
                "grep -qx 'sam@example.com' \(shoppers) || echo 'sam@example.com' >> \(shoppers)",
                #"ok "seeded shopper""#))
        try await prepare()
        try await w.front().select(name)
    }
}
