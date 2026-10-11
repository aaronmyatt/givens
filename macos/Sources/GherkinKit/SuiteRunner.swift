// Runs pickles strictly, like the Go suite's godog options (Strict: true): an undefined step,
// a step more than one definition matches, or a step that throws fails its scenario.
// Ref: https://pkg.go.dev/github.com/cucumber/godog#Options

/// One scenario's step definitions and how to clean up after it. A suite makes a fresh one
/// per scenario, so no state leaks between scenarios.
public struct ScenarioBinding {
    /// Every definition this scenario can use: shared steps and the capability's own.
    public let steps: [StepDefinition]
    /// Runs after the scenario, pass or fail.
    public let tearDown: @MainActor @Sendable () async -> Void

    /// Binds steps and a tear-down.
    public init(
        steps: [StepDefinition], tearDown: @escaping @MainActor @Sendable () async -> Void
    ) {
        precondition(!steps.isEmpty, "a scenario has step definitions")
        self.steps = steps
        self.tearDown = tearDown
    }
}

/// Why one scenario failed.
public struct ScenarioFailure: Sendable, CustomStringConvertible {
    /// The scenario's name.
    public let scenario: String
    /// Where it starts: "capabilities/x/x.feature:12".
    public let location: String
    /// The step that failed, without its keyword.
    public let step: String
    /// What went wrong.
    public let message: String

    /// A report line, the way godog prints a failure.
    public var description: String { "\(location) \(scenario)\n    step: \(step)\n    \(message)" }
}

/// What a suite run did.
public struct SuiteReport: Sendable {
    /// Names of the scenarios that ran, in order.
    public var ran: [String] = []
    /// Scenarios the tag expression skipped.
    public var skipped = 0
    /// Every failed scenario.
    public var failures: [ScenarioFailure] = []
}

/// Limits. Tiger Style: put a limit on everything.
public enum SuiteLimits {
    /// Pickles one suite may run.
    public static let picklesMax = 4096
    /// Steps one scenario may have.
    public static let stepsPerScenarioMax = 256
}

/// Runs every pickle `tags` matches, binding each to a fresh scenario from `start`.
@MainActor
public func runSuite(
    _ pickles: [Pickle], tags: TagExpression,
    start: @MainActor (Pickle) async throws -> ScenarioBinding
) async -> SuiteReport {
    precondition(pickles.count <= SuiteLimits.picklesMax, "pickles fit picklesMax")
    var report = SuiteReport()
    for pickle in pickles {
        guard tags.matches(pickle.tagNames) else {
            report.skipped += 1
            continue
        }
        report.ran.append(pickle.name)
        if let failure = await runScenario(pickle, start: start) {
            report.failures.append(failure)
        }
    }
    precondition(report.ran.count + report.skipped == pickles.count, "every pickle is counted")
    return report
}

/// Runs one scenario and returns its failure, or nil when every step passed.
@MainActor
func runScenario(
    _ pickle: Pickle, start: @MainActor (Pickle) async throws -> ScenarioBinding
) async -> ScenarioFailure? {
    precondition(pickle.steps.count <= SuiteLimits.stepsPerScenarioMax, "steps fit the limit")
    let location = "\(pickle.uri):\(pickle.location?.line ?? 0)"
    func failure(_ step: String, _ message: String) -> ScenarioFailure {
        ScenarioFailure(scenario: pickle.name, location: location, step: step, message: message)
    }
    let binding: ScenarioBinding
    do {
        binding = try await start(pickle)
    } catch {
        return failure("(set up)", "\(error)")
    }
    var result: ScenarioFailure?
    for step in pickle.steps where result == nil {
        let matched = matching(binding.steps, step.text)
        guard matched.count == 1, let (definition, captures) = matched.first else {
            result = failure(
                step.text,
                "\(matched.count) definitions match: "
                    + matched.map(\.0.pattern).joined(separator: " | "))
            continue
        }
        let call = StepCall(
            captures: captures, docString: step.argument?.docString?.content,
            table: step.argument?.dataTable?.cells, text: step.text)
        do {
            try await definition.body(call)
        } catch {
            result = failure(step.text, "\(error)")
        }
    }
    await binding.tearDown()
    return result
}

/// The definitions whose pattern matches `text`, with their captures.
func matching(_ definitions: [StepDefinition], _ text: String) -> [(StepDefinition, [String])] {
    precondition(!definitions.isEmpty, "there are definitions to match")
    let matched = definitions.compactMap { definition in
        definition.captures(in: text).map { (definition, $0) }
    }
    precondition(matched.count <= definitions.count, "a definition matches at most once")
    return matched
}

/// Lists every step in `pickles` that no definition matches, or more than one does. It runs
/// nothing, so it checks @wip scenarios too: every step has exactly one home.
public func undefinedSteps(_ pickles: [Pickle], _ definitions: [StepDefinition]) -> [String] {
    precondition(!definitions.isEmpty, "there are definitions to check against")
    var problems: [String] = []
    for pickle in pickles {
        for step in pickle.steps {
            let matched = matching(definitions, step.text)
            if matched.count != 1 {
                problems.append(
                    "\(pickle.name): \(matched.count) definitions match \"\(step.text)\""
                        + matched.map { " | \($0.0.pattern)" }.joined())
            }
        }
    }
    precondition(problems.count <= pickles.count * SuiteLimits.stepsPerScenarioMax, "bounded")
    return problems
}
