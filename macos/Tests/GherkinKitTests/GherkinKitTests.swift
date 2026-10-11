// Unit tests for the runner itself, so a green acceptance suite means the app passed, not that
// the runner skipped something.
// Ref: https://developer.apple.com/documentation/testing
import Testing

@testable import GherkinKit

@Suite struct TagExpressionTests {
    @Test func defaultSkipsWorkInProgress() {
        let tags = TagExpression("~@wip")
        #expect(tags.matches(["@error"]))
        #expect(!tags.matches(["@WIP", "@error"]))
    }

    @Test func andOfOrs() {
        let tags = TagExpression("@wip && @error,@payments")
        #expect(tags.matches(["@wip", "@payments"]))
        #expect(!tags.matches(["@wip"]))
        #expect(!tags.matches(["@error"]))
    }

    @Test func emptyMatchesEverything() {
        #expect(TagExpression("").matches([]))
    }
}

@Suite struct StepDefinitionTests {
    @Test func capturesGroupsAndBlanksMissingOptionals() {
        let definition = StepDefinition(#"^they're offered "([^"]*)"(?: and "([^"]*)")?$"#) { _ in }
        #expect(definition.captures(in: #"they're offered "Trust""#) == ["Trust", ""])
        #expect(definition.captures(in: #"they're offered "A" and "B""#) == ["A", "B"])
        #expect(definition.captures(in: "they see nothing") == nil)
    }

    @Test func readsUnicodeShortcuts() {
        let definition = StepDefinition(#"^they press ⌘K and type "([^"]*)"$"#) { _ in }
        #expect(definition.captures(in: #"they press ⌘K and type "card""#) == ["card"])
    }
}

@Suite struct RunnerTests {
    /// A one-step pickle for runner tests.
    func pickle(_ text: String, tags: [String] = []) -> Pickle {
        Pickle(
            id: "1", uri: "test.feature", name: "Sample",
            steps: [PickleStep(id: "2", text: text, type: "Action", argument: nil)],
            tags: tags.map(PickleTag.init(name:)), location: nil)
    }

    @Test @MainActor func undefinedStepFailsTheScenario() async {
        let binding = ScenarioBinding(steps: [StepDefinition("^known$") { _ in }]) {}
        let report = await runSuite([pickle("unknown")], tags: TagExpression("")) { _ in binding }
        #expect(report.failures.count == 1)
        #expect(report.failures.first?.message.hasPrefix("0 definitions match") == true)
    }

    @Test @MainActor func skipsByTag() async {
        let binding = ScenarioBinding(steps: [StepDefinition("^known$") { _ in }]) {}
        let report = await runSuite(
            [pickle("known", tags: ["@wip"]), pickle("known")], tags: TagExpression("~@wip")
        ) { _ in binding }
        #expect(report.ran.count == 1)
        #expect(report.skipped == 1)
        #expect(report.failures.isEmpty)
    }

    @Test func reportsAmbiguousSteps() {
        let definitions = [StepDefinition("^a.*$") { _ in }, StepDefinition("^ab$") { _ in }]
        #expect(undefinedSteps([pickle("ab")], definitions).count == 1)
        #expect(undefinedSteps([pickle("ax")], definitions).isEmpty)
    }
}
