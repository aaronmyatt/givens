import Foundation
import GherkinKit
import GivensUI
import Testing

/// Runs capabilities/trust-a-project against the Swift app. Shared steps are in World/.
///
/// Card declined's setup touches a marker file outside the project, so a step can tell
/// whether any of the project's scripts ran.
@Suite("Trust a project") @MainActor struct TrustAProjectTests {
    static let capability = "trust-a-project"

    @Test func scenarios() async throws {
        try await runCapability(Self.capability, steps: trustAProjectSteps)
    }

    @Test func stepsDefined() throws {
        try checkStepsDefined(Self.capability, steps: trustAProjectSteps)
    }
}

/// What this capability's steps keep between steps of one scenario.
@MainActor
final class TrustAProjectState {
    /// The scenario every step drops in to, and the prompt Givens asks before running scripts.
    static let scenarioName = "Card declined"
    static let trustPrompt = "Run scripts from this project?"
    /// Polling: a fixed number of looks, a fixed pause apart.
    static let pollIntervalMilliseconds = 100
    /// How many looks "nothing ran" is watched for.
    static let quietAttempts = 5
    /// How many looks a run may take to start: World's setup wait.
    static let setupPollAttempts = World.setupWaitSeconds * 1000 / pollIntervalMilliseconds

    let w: World
    /// The file Card declined's setup touches when it runs.
    var marker: URL { w.dir.appendingPathComponent("card-declined-ran") }
    /// The line in the setup that touches it.
    var script: String { "touch \(Shell.quote(marker.path))" }

    init(_ w: World) { self.w = w }

    /// Copies the sample shop and makes Card declined's setup touch the marker.
    func arrangeShop() throws {
        try w.copySampleShop()
        try w.setupStartsWith(Self.scenarioName, script)
    }

    var ran: Bool { FileManager.default.fileExists(atPath: marker.path) }

    /// Waits one poll interval.
    /// Ref: https://developer.apple.com/documentation/swift/task/sleep(for:tolerance:clock:)
    func pause() async throws {
        try await Task.sleep(for: .milliseconds(Self.pollIntervalMilliseconds))
    }

    /// Watches the marker for a short, fixed time and fails if the setup ran.
    func stayedQuiet() async throws {
        for _ in 0..<Self.quietAttempts {
            if ran { throw StepFailure("the setup script ran") }
            try await pause()
        }
    }

    /// Waits up to World's setup wait for the setup to touch the marker.
    func startedRunning() async throws {
        for _ in 0..<Self.setupPollAttempts {
            if ran { return }
            try await pause()
        }
        throw StepFailure("the setup didn't run within \(World.setupWaitSeconds) seconds")
    }

    /// Drops in to Card declined in `window`, answers the prompt with `choice`, and waits for
    /// any setup that choice started.
    func chooseOnPrompt(_ window: WindowModel, _ choice: String) async throws {
        try await window.dropIn(Self.scenarioName)
        try await window.choose(choice)
        if let session = window.session {
            try await session.wait(seconds: World.setupWaitSeconds)
        }
    }

    /// Answers the prompt in an earlier run of Givens on this scenario's app data, then quits
    /// it, so only what Givens stored outside the project carries over.
    func choseInEarlierApp(_ choice: String) async throws {
        try w.startApp()
        guard let earlier = w.app else { throw StepFailure("no app started") }
        try await chooseOnPrompt(try await earlier.open(w.capabilities.path), choice)
        try await earlier.quit()
        w.app = nil
        w.window = nil
        w.session = nil
        // Forget the earlier run, so later steps see only new runs.
        if ran { try FileManager.default.removeItem(at: marker) }
    }

    /// Expects `text`, and a line showing the project's path as given or with its links
    /// resolved: macOS's temporary folder /var is a link to /private/var.
    /// Ref: https://developer.apple.com/documentation/foundation/url/resolvingsymlinksinpath()
    func seeWithPath(_ text: String) async throws {
        let visible = try await w.front().visible
        try expect(visible, text, "the window")
        guard let project = w.project else { throw StepFailure("no project copied") }
        let forms = [project.path, project.resolvingSymlinksInPath().path]
        guard visible.contains(where: { line in forms.contains(where: line.contains) }) else {
            throw StepFailure("the window doesn't show the project's path \(forms[0]): \(visible)")
        }
    }
}

@MainActor
func trustAProjectSteps(_ w: World) -> [StepDefinition] {
    let s = TrustAProjectState(w)
    return trustAProjectAskSteps(s) + trustAProjectChoiceSteps(s) + trustAProjectCloneSteps(s)
}

@MainActor
private func trustAProjectAskSteps(_ s: TrustAProjectState) -> [StepDefinition] {
    [
        def(#"^a developer has never run anything from the sample shop$"#) { _ in
            let entries = try FileManager.default.contentsOfDirectory(atPath: s.w.appData.path)
            guard entries.isEmpty else {
                throw StepFailure("the app data folder isn't empty: \(entries)")
            }
            try s.arrangeShop()
        },
        def(#"^they see "([^"]*)" with the project's path$"#) { call in
            try await s.seeWithPath(call[0])
        },
        // Checks the setup waits on the question, then that choosing "Trust" lets it run, so
        // the wait was for the choice.
        def(#"^nothing runs until they choose$"#) { _ in
            do {
                try await s.stayedQuiet()
            } catch {
                throw StepFailure("before a choice was made, \(error)")
            }
            try await s.w.front().choose("Trust")
            try await s.startedRunning()
        },
        def(#"^a developer chose "([^"]*)" for the sample shop before$"#) { call in
            try s.arrangeShop()
            try await s.choseInEarlierApp(call[0])
        },
        def(#"^setup starts without asking$"#) { _ in
            let window = try await s.w.front()
            try expectNot(window.visible, TrustAProjectState.trustPrompt, "the window")
            try expectNot(window.offers, "Don't trust", "the offered actions")
            try await s.startedRunning()
        },
    ]
}

@MainActor
private func trustAProjectChoiceSteps(_ s: TrustAProjectState) -> [StepDefinition] {
    [
        // Answers the prompt in this scenario's own app, which stays open.
        def(#"^a developer chose "([^"]*)" for the sample shop$"#) { call in
            try s.arrangeShop()
            try await s.w.openSampleShop()
            try await s.chooseOnPrompt(try await s.w.front(), call[0])
            if s.ran { throw StepFailure("choosing \"\(call[0])\" ran the setup") }
        },
        def(
            #"^the Drop in screen opens with the scenario's steps and setup script, but nothing in the script runs$"#
        ) { _ in
            guard s.w.session != nil else {
                throw StepFailure("dropping in opened no Drop in screen")
            }
            let visible = try await s.w.front().visible
            let name = TrustAProjectState.scenarioName
            let feature = s.w.capabilities.appendingPathComponent("checkout/checkout.feature")
            guard let pickle = try Toolchain.pickles(of: feature).first(where: { $0.name == name })
            else { throw StepFailure("checkout.feature has no \(name)") }
            for step in pickle.steps {
                try expect(visible, step.text, "the Drop in screen")
            }
            try expect(visible, s.script, "the Drop in screen")
            try await s.stayedQuiet()
        },
        // Expects the message, then copies the setup command with scripts off.
        def(#"^they see "([^"]*)", and "Copy" still works$"#) { call in
            let window = try await s.w.front()
            try expect(window.visible, call[0], "the window")
            try await window.choose("Copy")
            let copied = s.w.clipboard.readText()
            guard copied.contains("card-declined") else {
                throw StepFailure(
                    "\"Copy\" put \"\(copied)\" on the clipboard, not Card declined's setup")
            }
            try await s.stayedQuiet()
        },
    ]
}

@MainActor
private func trustAProjectCloneSteps(_ s: TrustAProjectState) -> [StepDefinition] {
    [
        // Moves the copied shop to <scenario>/home/code/shop, standing in for ~/code/shop,
        // and trusts it in an earlier run of Givens.
        def(#"^a developer trusted the sample shop at ~/code/shop$"#) { _ in
            try s.arrangeShop()
            guard let project = s.w.project else { throw StepFailure("no project copied") }
            let shop = s.w.dir.appendingPathComponent("home/code/shop", isDirectory: true)
            try FileManager.default.createDirectory(
                at: shop.deletingLastPathComponent(), withIntermediateDirectories: true)
            try FileManager.default.moveItem(at: project, to: shop)
            s.w.project = shop
            try await s.choseInEarlierApp("Trust")
        },
        // Copies the trusted shop to shop-2, as a fresh clone would be, and drops in there.
        def(#"^they open a fresh clone of it at ~/code/shop-2 and drop in$"#) { _ in
            guard let project = s.w.project else { throw StepFailure("no project copied") }
            let clone = project.deletingLastPathComponent().appendingPathComponent("shop-2")
            try FileManager.default.copyItem(at: project, to: clone)
            s.w.project = clone
            try s.w.startApp()
            guard let app = s.w.app else { throw StepFailure("no app started") }
            s.w.window = try await app.open(s.w.capabilities.path)
            try await s.w.dropIn(TrustAProjectState.scenarioName)
        },
        def(#"^they're asked "([^"]*)" again$"#) { call in
            try await s.seeWithPath(call[0])
            try await s.stayedQuiet()
        },
    ]
}
