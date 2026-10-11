import Foundation
import GherkinKit
import GivensUI
import Testing

/// Runs capabilities/drop-into-a-scenario against the Swift app. Shared steps are in World/.
///
/// Each Given writes the setup scripts it needs into the scenario's copy of the sample shop, so
/// a scenario never depends on what the fixture's own scripts print. Scripts leave marker files
/// in the shop's shop-data/ folder; Then steps read them to see what ran, and with what.
/// Timers run on the core's test clock: a step moves it on instead of waiting two minutes.
@Suite("Drop into a scenario") @MainActor struct DropIntoAScenarioTests {
    static let capability = "drop-into-a-scenario"

    @Test func scenarios() async throws {
        try await runCapability(Self.capability, steps: dropIntoAScenarioSteps)
    }

    @Test func stepsDefined() throws {
        try checkStepsDefined(Self.capability, steps: dropIntoAScenarioSteps)
    }
}

/// What this capability's steps keep between steps of one scenario.
@MainActor
final class DropIntoAScenarioState {
    let w: World
    /// The scenario a Given arranged, for steps that don't name it.
    var scenario = ""
    /// shop-data files a script writes only if it runs.
    var markers: [String] = []
    /// How long the blocked setup "takes", in seconds of the core's clock.
    var slowForSeconds = 0
    /// "Still running" appeared while a step watched for it.
    var sawSlow = false
    /// The note added before "Record run", and the run key recorded against.
    var note = ""
    var runKey = ""
    /// The by_hand card's snippet and its --url link.
    var snippet = ""
    var link = ""
    /// The card a step picked, by its index in setupCards().
    var cardIndex = 0
    /// Setup output after each way in, for the teardown checks.
    var outputs: [String: [String]] = [:]
    /// git merge's output and exit code.
    var mergeOutput = ""
    var mergeExit: Int32 = 0

    init(_ w: World) { self.w = w }

    // Lines the test setups print through the kit's ok, which prefixes "✓ ".
    static let firstLine = "✓ seeded shopper"
    static let secondLine = "✓ card set to decline"
    static let afterCard = "✓ rest of setup ran"
    static let teardownLine = "✓ cleared orders"
    static let stillRunning = "Still running"
    static let payWithSavedCard = "Pay with saved card"
    static let threeDSChallenge = "3-D Secure challenge"
    static let shopDataFolder = "shop-data"
    /// One tick past the default two-minute timeout.
    static let slowSetupSeconds = 121

    // Lines test scripts use. Each script makes shop-data/ first, so a marker can always be
    // written. shopData opens a double quote that the line using it closes.
    static let makeShopData = #"mkdir -p "$GIVENS_ROOT/../shop-data""#
    static let shopData = #""$GIVENS_ROOT/../shop-data"#
    /// Blocks setup until a step writes shop-data/release, for at most a minute, so a setup
    /// that's never released can't outlive the test run by long.
    static let waitForRelease =
        #"for _ in $(seq 1 600); do [ -f "$GIVENS_ROOT/../shop-data/release" ] && break; "#
        + "sleep 0.1; done"
    static let writePID = #"echo "$$" > "$GIVENS_ROOT/../shop-data/setup.pid""#

    /// A setup script that sources the kit, makes shop-data/, then runs lines.
    static func script(_ lines: String...) -> String {
        ([kitSource, makeShopData] + lines).joined(separator: "\n") + "\n"
    }

    /// A setup script whose first line after the kit's is a `from` line.
    static func scriptFrom(_ fromLine: String, _ lines: String...) -> String {
        ([kitSource, fromLine, makeShopData] + lines).joined(separator: "\n") + "\n"
    }

    /// A script line that writes a marker file in shop-data/.
    static func touch(_ name: String) -> String { "touch " + shopData + "/" + name + "\"" }

    /// A quick setup, for scenarios about what happens after it.
    static var quickSetup: String { script(#"ok "seeded shopper""#) }
}

/// Every step of this capability, grouped by area across this folder's files.
@MainActor
func dropIntoAScenarioSteps(_ w: World) -> [StepDefinition] {
    let s = DropIntoAScenarioState(w)
    return dropInRunSteps(s) + dropInTimeoutSteps(s) + dropInRecordSteps(s)
        + dropInTeardownSteps(s) + dropInFromSteps(s) + dropInByHandSteps(s) + dropInLinkSteps(s)
        + dropInAskSteps(s) + dropInRowSteps(s) + dropInTableSteps(s)
        + dropInMarkdownSteps(s) + dropInCodeSteps(s)
}

/// Running setup: output, the checklist, failures and missing commands.
@MainActor
private func dropInRunSteps(_ s: DropIntoAScenarioState) -> [StepDefinition] {
    [
        def(#"^the "Card declined" setup prints a line, then waits before printing the next$"#) {
            _ in try await s.printsThenWaits()
        },
        def(#"^the setup script's output appears line by line as it runs$"#) { _ in
            try await s.outputStreams()
        },
        def(
            #"^when it finishes, any manual Givens, then the When steps, are shown as a checklist$"#
        ) { _ in try await s.checklistShown() },
        def(#"^each Then step can be marked "Pass" or "Fail"$"#) { _ in
            try await s.thenStepsMarkable()
        },
        def(#"^the "Card declined" setup script stops with an error part way through$"#) { _ in
            try await s.setupStopsPartWay()
        },
        def(#"^the output is shown up to the point it stopped$"#) { _ in
            try await s.outputUpToStop()
        },
        def(#"^no checklist is shown$"#) { _ in try await s.noChecklist() },
        def(
            #"^the "Card declined" setup script needs a command called shopctl, which isn't installed$"#
        ) { _ in try await s.needsShopctl() },
        def(#"^nothing in the script runs$"#) { _ in try await s.nothingRuns() },
        def(#"^the install hint "([^"]*)" is shown with a copy button$"#) { call in
            try await s.installHint(call[0])
        },
    ]
}

/// Timeouts: "Still running", @timeout tags and Stop.
@MainActor
private func dropInTimeoutSteps(_ s: DropIntoAScenarioState) -> [StepDefinition] {
    [
        def(#"^the "Card declined" setup script takes more than two minutes$"#) { _ in
            try await s.setupTakesTooLong()
        },
        def(#"^"([^"]*)" isn't tagged @timeout$"#) { call in
            try s.setTimeoutTag(call[0], "")
        },
        def(#"^after two minutes they see "([^"]*)"$"#) { call in
            try await s.afterTwoMinutes(call[0])
        },
        def(#"^choosing "Stop" ends the script and keeps its output so far$"#) { _ in
            try await s.stopKeepsOutput()
        },
        def(#"^"([^"]*)" is tagged (@timeout:\S+)$"#) { call in
            try await s.tagTimeout(call[0], call[1])
        },
        def(#"^its setup takes (\d+) minutes$"#) { call in
            guard let minutes = Int(call[0]) else { throw StepFailure("\(call[0]) isn't minutes") }
            s.slowForSeconds = minutes * 60
        },
        def(#"^no "Still running" message is shown$"#) { _ in try await s.noStillRunning() },
        def(#"^setup finishes as normal$"#) { _ in try await s.finishesAsNormal() },
        def(#"^setup runs with the usual 2-minute timeout$"#) { _ in try await s.usualTimeout() },
        def(#"^"([^"]*)" is marked "([^"]*)"$"#) { call in
            let scenario = try await s.w.front().scenario(call[0])
            try expect([scenario.warning], call[1], "\(call[0])'s warning")
        },
    ]
}

// MARK: - Shared helpers

extension DropIntoAScenarioState {
    /// Opens the sample shop, unless it's open, and trusts it, so a Drop in runs setup without
    /// the trust prompt. Scenarios about trust live in capabilities/trust-a-project.
    func prepare() async throws {
        if w.window == nil { try await w.openSampleShop() }
        guard let app = w.app else { throw StepFailure("Givens isn't running") }
        try await app.markTrusted(w.capabilities.path)
    }

    /// The window's current Drop in: the one a choose may have replaced, or the one the last
    /// drop in step started.
    func session() async throws -> SessionModel {
        let window = try await w.front()
        if let current = window.session { return current }
        if let session = w.session { return session }
        throw StepFailure("no Drop in is open; the window shows \(window.visible)")
    }

    /// Waits for the current Drop in to finish or pause, and fails unless it ends in `want`.
    @discardableResult
    func settleTo(_ want: String) async throws -> SessionModel {
        let session = try await session()
        let status = try await session.wait(seconds: World.setupWaitSeconds)
        guard status == want else {
            throw StepFailure(
                "setup ended \"\(status)\", not \"\(want)\"; output: \(session.output)")
        }
        return session
    }

    /// Drops in to `name` and waits for setup to finish or pause in `want`.
    @discardableResult
    func dropInTo(_ name: String, _ want: String) async throws -> SessionModel {
        try await w.dropIn(name)
        return try await settleTo(want)
    }

    /// Replaces the named scenario's setup script.
    func writeSetup(_ name: String, _ content: String) throws {
        try w.copySampleShop()
        try w.writeFile(try w.scriptPath(name), content)
    }

    /// The named scenario's teardown script, relative to the sample shop.
    func teardownPath(_ name: String) throws -> String {
        let path = try w.scriptPath(name)
        return String(path.dropLast(".sh".count)) + ".teardown.sh"
    }

    /// Replaces the named scenario's teardown script.
    func writeTeardown(_ name: String, _ content: String) throws {
        try w.copySampleShop()
        try w.writeFile(try teardownPath(name), content)
    }

    /// A marker file a script wrote in shop-data/, or nil when there's none.
    func shopFile(_ name: String) -> String? {
        try? w.readFile(Self.shopDataFolder + "/" + name)
    }

    /// Lets a setup blocked on waitForRelease carry on.
    func release() throws { try w.writeFile(Self.shopDataFolder + "/release", "") }

    /// Moves the core's clock on.
    func advance(seconds: Int) async throws {
        guard let app = w.app else { throw StepFailure("Givens isn't running") }
        try await app.advanceClock(seconds: seconds)
    }

    /// The run key of a scenario with no Examples: capability/scenario-slug.
    func runKeyOf(_ name: String) throws -> String {
        "\(try w.scenarioCapability(name))/\(slug(name))"
    }

    /// True when the front window shows a line containing `text`.
    func shows(_ text: String) async throws -> Bool {
        try await w.front().visible.contains { $0.contains(text) }
    }
}
