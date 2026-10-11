import Foundation
import GivensUI
import Testing

/// The line helpers the Mac editor uses to offer steps while typing.
@MainActor @Suite struct EditorLineTests {
    @Test func findsTheLineJustTypedOn() {
        let old = "Scenario: x\n    Given a sh\n    Then y"
        #expect(WindowModel.changedLine(from: old, to: old) == nil)
        #expect(
            WindowModel.changedLine(
                from: old,
                to: old.replacingOccurrences(
                    of: "a sh", with: "a sho")) == 1)
        #expect(WindowModel.changedLine(from: old, to: old + "\n    And") == 3)
        #expect(WindowModel.line(1, of: old) == "    Given a sh")
        #expect(WindowModel.line(9, of: old) == "")
    }

    @Test func swapsInAStepKeepingIndentAndKeyword() {
        let text = "Scenario: x\n    Given a sh\n    Then y"
        let swapped = WindowModel.withSuggestion(text, line: 1, step: "a shopper has an account")
        #expect(swapped == "Scenario: x\n    Given a shopper has an account\n    Then y")
        #expect(WindowModel.withSuggestion(text, line: 7, step: "z") == text)
    }
}

/// "Copy steps" text: tags, the Scenario: line, then the steps.
@MainActor @Suite struct CopyStepsTests {
    @Test func writesTheScenarioAsGherkin() throws {
        let json = """
            {"name":"Card declined","slug":"card-declined","capability":"Checkout","rule":"",
             "error":true,"tags":["@error"],"steps":["Given a shopper","When they pay"],
             "setup":"","status":"Never run","warning":""}
            """
        let scenario = try JSONDecoder().decode(ScenarioRecord.self, from: Data(json.utf8))
        #expect(
            WindowModel.gherkin(scenario)
                == "@error\nScenario: Card declined\n  Given a shopper\n  When they pay\n")
    }
}
