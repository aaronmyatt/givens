import Foundation
import GherkinKit
import GivensUI
import Testing

/// Runs capabilities/keep-runs-off-main against the Swift app. Shared steps are in World/.
/// It mirrors acceptance/keep-runs-off-main in Go.
@Suite("Keep runs off main") @MainActor struct KeepRunsOffMainTests {
    static let capability = "keep-runs-off-main"

    @Test func scenarios() async throws {
        try await runCapability(Self.capability, steps: keepRunsOffMainSteps)
    }

    @Test func stepsDefined() throws {
        try checkStepsDefined(Self.capability, steps: keepRunsOffMainSteps)
    }
}

@MainActor
func keepRunsOffMainSteps(_ w: World) -> [StepDefinition] {
    let s = KeepRunsState(w)
    return keepRunsArrangeSteps(s) + keepRunsCheckSteps(s)
}

/// The Given and When steps: a repository with a remote, runs kept off main, another device.
@MainActor
private func keepRunsArrangeSteps(_ s: KeepRunsState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^the sample shop is a git repository with a remote$"#) { _ in
            try await s.gitWithRemote()
        },
        def(#"^the sample shop is a git repository without a remote$"#) { _ in
            try s.gitWithoutRemote()
        },
        def(#"^"([^"]*)" has a passing run recorded in capabilities/runs$"#) { call in
            try await w.recordPassingRun(call[0])
        },
        def(#"^a developer chooses "Move runs off main" and confirms$"#) { _ in
            try await s.moveRunsOffMain()
        },
        def(
            #"^the sample shop is a git repository with a remote, with a passing run of "#
                + #""([^"]*)"$"#
        ) { call in
            try await s.gitWithRemote()
            try await w.recordPassingRun(call[0])
        },
        def(#"^the sample shop is a git repository with a remote, cloned on another device$"#) {
            _ in
            try await s.gitWithRemote()
            try await w.openSampleShop()
            try await s.cloneOther()
        },
        def(
            #"^the sample shop is a git repository with a remote, with a limit of (\d+) runs$"#
        ) { call in
            try await s.gitWithRemote()
            try await s.writeAndShare {
                try Data("runs \(call[0])\nmegabytes 250\n".utf8)
                    .write(to: s.tree.appendingPathComponent("limits"))
            }
        },
        def(#"^a developer records a passing run of "([^"]*)"$"#) { call in
            try await w.recordPassingRun(call[0])
        },
        def(#"^a developer clones it on another device and opens it in Givens$"#) { _ in
            try await s.cloneOther()
            guard let other = s.other else { throw StepFailure("no other device") }
            w.app?.stop()
            (w.app, w.window, w.session) = (other.app, other.window, nil)
        },
        def(#"^a passing run of "([^"]*)" is recorded on the other device$"#) { call in
            try await s.onOther { try await w.recordPassingRun(call[0]) }
        },
        def(#"^its remote can't be reached$"#) { _ in try keepRunsAway(s) },
        def(
            #"^a passing run of "([^"]*)" was recorded while its remote couldn't be reached$"#
        ) { call in
            try keepRunsAway(s)
            try await w.recordPassingRun(call[0])
        },
        def(#"^the remote is back and a developer chooses "([^"]*)"$"#) { call in
            guard let remote = s.remote else { throw StepFailure("the shop has no remote") }
            try FileManager.default.moveItem(
                at: remote.appendingPathExtension("away"), to: remote)
            try await w.front().choose(call[0])
        },
        def(#"^"([^"]*)" has (\d+) runs recorded, each with a screenshot$"#) { call in
            try await s.recordedRuns(call[0], Int(call[1]) ?? 0)
        },
    ]
}

/// Moves the remote away, so it can't be reached.
@MainActor
private func keepRunsAway(_ s: KeepRunsState) throws {
    guard let remote = s.remote else { throw StepFailure("the shop has no remote") }
    try FileManager.default.moveItem(at: remote, to: remote.appendingPathExtension("away"))
}

/// The Then steps: what the remote's ref and the working tree hold.
@MainActor
private func keepRunsCheckSteps(_ s: KeepRunsState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^the remote's refs/givens/runs holds the run of "([^"]*)"$"#) { call in
            try keepRunsRemoteHolds(s, [call[0]])
        },
        def(#"^the remote's refs/givens/runs holds the runs of "([^"]*)" and "([^"]*)"$"#) {
            call in try keepRunsRemoteHolds(s, [call[0], call[1]])
        },
        def(#"^capabilities/runs holds the run of "([^"]*)"$"#) { call in
            let key = try s.runKey(call[0])
            let folder = w.capabilities.appendingPathComponent("runs")
            let logs = try FileManager.default.contentsOfDirectory(atPath: folder.path)
                .filter { $0.hasSuffix(".log") }
            let lines = try logs.flatMap {
                try String(contentsOf: folder.appendingPathComponent($0), encoding: .utf8)
                    .components(separatedBy: "\n")
            }
            guard
                lines.contains(where: { $0.components(separatedBy: "\t").dropFirst().first == key })
            else { throw StepFailure("capabilities/runs holds no run of \(key): \(lines)") }
        },
        def(#"^capabilities/runs holds only off-main\.md$"#) { _ in
            let folder = w.capabilities.appendingPathComponent("runs")
            let names = try FileManager.default.contentsOfDirectory(atPath: folder.path)
            guard names == ["off-main.md"] else {
                throw StepFailure("capabilities/runs holds \(names)")
            }
        },
        def(#"^git status shows nothing changed$"#) { _ in
            guard let project = w.project else { throw StepFailure("no sample shop copied") }
            let status = try keepRunsGit(["status", "--porcelain"], in: project)
            guard status.isEmpty else { throw StepFailure("git status shows:\n\(status)") }
        },
        def(#"^they don't see "([^"]*)"$"#) { call in
            try expectNot(try await w.front().visible, call[0], "the window")
        },
        def(#"^the remote's refs/givens/runs holds the newest (\d+) runs of "([^"]*)"$"#) {
            call in try keepRunsHoldsNewest(s, Int(call[0]) ?? 0, call[1])
        },
        def(#"^its history holds none of the older runs' screenshots$"#) { _ in
            guard let remote = s.remote, !s.dropped.isEmpty else {
                throw StepFailure("no step said which runs were dropped")
            }
            let objects = try keepRunsGit(
                ["rev-list", "--objects", "refs/givens/runs"], in: remote)
            for time in s.dropped {
                guard let date = ISO8601DateFormatter().date(from: time) else { continue }
                let folder = KeepRunsState.folderName(date)
                guard !objects.contains(folder) else {
                    throw StepFailure("the ref's history still holds \(folder)")
                }
            }
        },
    ]
}

/// Expects a run of each named scenario in the remote's ref.
@MainActor
private func keepRunsRemoteHolds(_ s: KeepRunsState, _ names: [String]) throws {
    let held = try s.remoteRuns()
    for name in names {
        let key = try s.runKey(name)
        guard held.contains(where: { $0.key == key }) else {
            throw StepFailure("the remote's refs/givens/runs holds no run of \(key): \(held)")
        }
    }
}

/// Expects `count` runs of the scenario in the remote's ref, none of them the oldest a Given
/// step recorded: the When step's run is newer than each, so those kept are its count-1.
@MainActor
private func keepRunsHoldsNewest(_ s: KeepRunsState, _ count: Int, _ name: String) throws {
    let key = try s.runKey(name)
    let mine = try s.remoteRuns().filter { $0.key == key }
    guard mine.count == count else {
        throw StepFailure("the remote holds \(mine.count) runs of \(key), not \(count)")
    }
    s.dropped = Array(s.times.prefix(max(s.times.count - (count - 1), 0)))
    for time in s.dropped where mine.contains(where: { $0.time == time }) {
        throw StepFailure("the remote still holds the run at \(time)")
    }
}
