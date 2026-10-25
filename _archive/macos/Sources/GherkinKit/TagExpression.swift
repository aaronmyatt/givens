// GIVENS_TAGS is shared with the Go suite, so this reads godog's tag syntax:
// `&&` joins groups that must all hold, `,` joins tags of which one must hold, `~` negates.
// "~@wip" (the default) skips work in progress; "@wip" runs only it.
// Ref: https://github.com/cucumber/godog#tags

/// A filter over a pickle's tags, in godog's syntax.
public struct TagExpression: Sendable, Equatable {
    /// One tag, maybe negated.
    struct Term: Sendable, Equatable {
        let tag: String
        let negated: Bool
    }

    /// Groups that must all hold; each group holds when any of its terms does.
    let groups: [[Term]]

    /// Limits. Tiger Style: put a limit on everything.
    static let lengthMax = 1024

    /// Parses `expression`; an empty expression matches every pickle.
    public init(_ expression: String) {
        precondition(expression.count <= Self.lengthMax, "a tag expression fits lengthMax")
        let groups = expression.split(separator: "&&").map { group in
            group.split(separator: ",").map { raw in
                // A tag never holds whitespace, so dropping all of it trims the term.
                let text = String(raw.filter { !$0.isWhitespace })
                let negated = text.hasPrefix("~")
                return Term(tag: negated ? String(text.dropFirst()) : text, negated: negated)
            }
            .filter { !$0.tag.isEmpty }
        }
        self.groups = groups.filter { !$0.isEmpty }
        precondition(
            self.groups.allSatisfy { $0.allSatisfy { $0.tag.hasPrefix("@") } },
            "every tag in a tag expression starts with @")
    }

    /// True when a pickle with `tags` should run. Tags compare without case, like godog.
    public func matches(_ tags: [String]) -> Bool {
        let lowered = Set(tags.map { $0.lowercased() })
        precondition(lowered.count <= tags.count, "lowering never adds tags")
        return groups.allSatisfy { group in
            group.contains { term in lowered.contains(term.tag.lowercased()) != term.negated }
        }
    }
}
