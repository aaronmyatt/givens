// A step definition pairs a regular expression over a step's text with the code that runs it,
// the way godog's ctx.Step does. Patterns are written in the Go suite's RE2 dialect, which
// Swift's Regex reads unchanged for everything the suites use.
// Ref: https://developer.apple.com/documentation/swift/regex
// Ref: https://pkg.go.dev/github.com/cucumber/godog#ScenarioContext.Step

/// What a step's code receives: the captured groups and the step's doc string or table.
public struct StepCall: Sendable {
    /// Captured groups, left to right; a group that didn't take part is "".
    public let captures: [String]
    /// The step's `"""` block, or nil.
    public let docString: String?
    /// The step's data table, or nil.
    public let table: [[String]]?
    /// The step's full text, for messages.
    public let text: String

    /// The capture at `index`, "" when the pattern has fewer groups.
    public subscript(index: Int) -> String {
        precondition(index >= 0, "capture indexes start at 0")
        return index < captures.count ? captures[index] : ""
    }
}

/// A step definition: an anchored pattern and the code that runs a matching step.
public struct StepDefinition {
    /// The pattern as written, for reports.
    public let pattern: String
    /// Runs the step. A thrown error fails the step and skips the rest of the scenario.
    public let body: @MainActor @Sendable (StepCall) async throws -> Void
    /// The compiled pattern.
    let regex: Regex<AnyRegexOutput>

    /// Makes a definition. A pattern must be anchored with ^ and $ and must compile: a bad
    /// pattern is a bug in the suite, so it crashes.
    public init(
        _ pattern: String, _ body: @escaping @MainActor @Sendable (StepCall) async throws -> Void
    ) {
        precondition(pattern.hasPrefix("^"), "a pattern is anchored at the start: \(pattern)")
        precondition(pattern.hasSuffix("$"), "a pattern is anchored at the end: \(pattern)")
        do {
            regex = try Regex(pattern)
        } catch {
            preconditionFailure("\(pattern) doesn't compile: \(error)")
        }
        self.pattern = pattern
        self.body = body
    }

    /// The captured groups when `text` matches, or nil.
    public func captures(in text: String) -> [String]? {
        precondition(!text.contains("\n"), "a step is one line")
        guard let match = try? regex.wholeMatch(in: text) else { return nil }
        let groups = match.output.dropFirst().map { $0.substring.map(String.init) ?? "" }
        precondition(groups.count == match.output.count - 1, "every group is reported")
        return groups
    }
}
