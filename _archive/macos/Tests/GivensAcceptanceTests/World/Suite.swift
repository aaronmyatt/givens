import Foundation
import GherkinKit
import Testing

/// A capability's own step definitions for one scenario's world.
typealias CapabilitySteps = @MainActor (World) -> [StepDefinition]

/// Runs one capability's scenarios strictly against the Swift app. GIVENS_TAGS picks
/// scenarios by tag, shared with the Go suite; the default, "~@wip", skips work in progress.
///
///     GIVENS_TAGS=@wip swift test --package-path macos     # watch the unbuilt ones fail
///
/// Each failed scenario is its own test issue, with the step and why.
/// Ref: https://developer.apple.com/documentation/testing/issue/record(_:sourcelocation:)
@MainActor
func runCapability(_ capability: String, steps: @escaping CapabilitySteps) async throws {
    let pickles = try Toolchain.pickles(of: Toolchain.feature(capability))
    let expression = ProcessInfo.processInfo.environment["GIVENS_TAGS"] ?? ""
    let tags = TagExpression(expression.isEmpty ? "~@wip" : expression)
    let report = await runSuite(pickles, tags: tags) { _ in
        let world = try World()
        return ScenarioBinding(steps: commonSteps(world) + steps(world)) { world.tearDown() }
    }
    print(
        "\(capability): \(report.ran.count) ran, \(report.skipped) skipped, "
            + "\(report.failures.count) failed")
    for failure in report.failures {
        Issue.record(Comment(rawValue: failure.description))
    }
}

/// Fails for every step in the capability's .feature file that no definition matches, or
/// that more than one does. It runs nothing, so it covers @wip scenarios too.
@MainActor
func checkStepsDefined(_ capability: String, steps: @escaping CapabilitySteps) throws {
    let pickles = try Toolchain.pickles(of: Toolchain.feature(capability))
    let world = try World()
    defer { world.tearDown() }
    for problem in undefinedSteps(pickles, commonSteps(world) + steps(world)) {
        Issue.record(Comment(rawValue: problem))
    }
}
