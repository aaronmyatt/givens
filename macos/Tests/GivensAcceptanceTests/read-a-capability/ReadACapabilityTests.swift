import Foundation
import GherkinKit
import GivensUI
import Testing

/// Runs capabilities/read-a-capability against the Swift app. Shared steps are in World/.
@Suite("Read a capability") @MainActor struct ReadACapabilityTests {
    static let capability = "read-a-capability"

    @Test func scenarios() async throws {
        try await runCapability(Self.capability, steps: readACapabilitySteps)
    }

    @Test func stepsDefined() throws {
        try checkStepsDefined(Self.capability, steps: readACapabilitySteps)
    }
}

/// A Checkout scenario as checkout.feature defines it, read without Givens.
private struct ReadACapabilityDiskScenario {
    let name: String
    let error: Bool
    let tags: [String]
    /// Keyword and text: "Given a signed-in shopper with one item in the cart".
    let steps: [String]
}

/// The about.md "A story image that isn't on disk" writes: a paragraph on each side of an
/// image whose file is gone, so the placeholder's place can be checked.
private enum ReadACapabilityMissingImage {
    static let textBefore = "Checkout takes the shopper from their cart to a placed order."
    static let textAfter = "A declined card leaves the order pending."
    static let about =
        "# How checkout works\n\n\(textBefore)\n\n![Checkout flow](images/flow.svg)\n\n"
        + "\(textAfter)\n"
}

/// What this capability's steps keep between steps of one scenario.
@MainActor
final class ReadACapabilityState {
    let w: World
    /// The scenario a step named, for steps that say "its".
    var scenario = ""
    /// The about.md paragraphs written around a missing image, for the placeholder's place.
    var textBeforeImage = ""
    var textAfterImage = ""

    init(_ w: World) { self.w = w }

    var checkoutFeature: String { "capabilities/checkout/checkout.feature" }

    /// Checkout's scenarios in file order: names and tags from the compiled pickles, steps
    /// with their keywords from the file, since pickles drop keywords.
    fileprivate func checkoutScenarios() throws -> [ReadACapabilityDiskScenario] {
        let pickles = try Toolchain.pickles(
            of: w.capabilities.appendingPathComponent(
                "checkout/checkout.feature"))
        let lines = try w.readFile(checkoutFeature).components(separatedBy: "\n")
        return try pickles.map { pickle in
            ReadACapabilityDiskScenario(
                name: pickle.name, error: pickle.tagNames.contains("@error"),
                tags: pickle.tagNames, steps: try readACapabilityKeywordSteps(pickle, lines))
        }
    }

    fileprivate func checkoutScenario(_ name: String) throws -> ReadACapabilityDiskScenario {
        guard let found = try checkoutScenarios().first(where: { $0.name == name }) else {
            throw StepFailure("checkout.feature has no scenario \"\(name)\"")
        }
        return found
    }

    /// The selected capability as the window shows it.
    func selectedCapability() async throws -> (WindowModel, CapabilityRecord) {
        let window = try await w.front()
        guard !window.selected.isEmpty else { throw StepFailure("nothing is selected") }
        return (window, try await window.capability(window.selected))
    }

    /// The scenario a step named (or the selected one), as shown and as on disk.
    fileprivate func shownScenario() async throws -> (
        WindowModel, ScenarioRecord, ReadACapabilityDiskScenario
    ) {
        let window = try await w.front()
        let name = scenario.isEmpty ? window.selected : scenario
        let onDisk = try checkoutScenario(name)
        return (window, try await window.scenario(name), onDisk)
    }

    /// Gives a Checkout scenario a setup script when it has none, so there's a setup to show
    /// next to its steps.
    func ensureSetup(_ name: String) throws {
        let scripts = w.capabilities.appendingPathComponent("checkout/scripts")
        for fileExtension in [".sh", ".md"]
        where FileManager.default.fileExists(
            atPath: scripts.appendingPathComponent(slug(name) + fileExtension).path)
        {
            return
        }
        let setup = kitSource + "\nseed_shopper\nseed_cart 1\nmock_payments --decline\n"
        try w.writeFile("capabilities/checkout/scripts/\(slug(name)).sh", setup)
    }

    /// checkout.feature's As a, I want and So that lines.
    func storyOnDisk() throws -> [String] {
        let story = try storyLines(
            of: w.capabilities.appendingPathComponent(
                "checkout/checkout.feature"))
        guard story.count == 3 else { throw StepFailure("checkout.feature's story is \(story)") }
        return story
    }
}

/// The pickle's steps with their keywords, read from the file from the scenario's line on.
private func readACapabilityKeywordSteps(_ pickle: Pickle, _ lines: [String]) throws
    -> [String]
{
    guard let start = pickle.location?.line, start <= lines.count else {
        throw StepFailure("\(pickle.name) has no location in its file")
    }
    let keywords = ["Given ", "When ", "Then ", "And ", "But ", "* "]
    let stepLines = lines[start...].map { $0.trimmingCharacters(in: .whitespaces) }
        .filter { line in keywords.contains(where: line.hasPrefix) }
    return try pickle.steps.map { step in
        guard let line = stepLines.first(where: { $0.hasSuffix(" " + step.text) }) else {
            throw StepFailure("no line in the file holds the step \"\(step.text)\"")
        }
        return line
    }
}

/// How a scenario's steps read in the library: keywords right-aligned to the widest one, so
/// every step's text starts in the same column.
private func readACapabilityLinedUp(_ steps: [String]) -> [String] {
    let keywords = steps.map { String($0.prefix { $0 != " " }) }
    let width = keywords.map(\.count).max() ?? 0
    return zip(keywords, steps).map { keyword, step in
        String(repeating: " ", count: width - keyword.count) + step
    }
}

/// Everything the window shows, one line per entry, so a block of text shown as one piece
/// can still be searched line by line.
@MainActor
private func readACapabilityLines(_ window: WindowModel) -> [String] {
    window.visible.flatMap { $0.components(separatedBy: "\n") }
}

/// The first line at or after `from` that contains `text`, or nil.
private func readACapabilityIndex(_ lines: [String], _ text: String, from: Int = 0) -> Int? {
    guard from < lines.count else { return nil }
    return lines[from...].firstIndex { $0.contains(text) }
}

/// Fails unless the story's last line is shown, with `text` (when given) below it.
private func readACapabilityStoryThenText(_ lines: [String], _ story: [String], _ text: String)
    throws
{
    guard story.count == 3 else { throw StepFailure("the story shown is \(story)") }
    guard let last = readACapabilityIndex(lines, story[2]) else {
        throw StepFailure("the window doesn't show \"\(story[2])\": \(lines)")
    }
    if !text.isEmpty && readACapabilityIndex(lines, text, from: last + 1) == nil {
        throw StepFailure("the window doesn't show \"\(text)\" below the story: \(lines)")
    }
}

/// about.md's first line of text without markdown marks, which reads the same once
/// rendered; "" when there's none.
private func readACapabilityPlainLine(_ markdown: String) -> String {
    let marks = CharacterSet(charactersIn: "*_`[]!<>|")
    for line in markdown.components(separatedBy: "\n") {
        let text = line.trimmingCharacters(in: CharacterSet(charactersIn: "# "))
        if !text.isEmpty && text.rangeOfCharacter(from: marks) == nil { return text }
    }
    return ""
}

/// The paths of the markdown images in `markdown`.
/// Ref: https://spec.commonmark.org/0.31.2/#images
/// Ref: https://developer.apple.com/documentation/swift/regex
private func readACapabilityImages(_ markdown: String) throws -> [String] {
    let image = try Regex(#"!\[[^\]]*\]\(([^)\s]+)\)"#)
    return markdown.matches(of: image).compactMap { match in
        match.output[1].substring.map(String.init)
    }
}

@MainActor
func readACapabilitySteps(_ w: World) -> [StepDefinition] {
    let s = ReadACapabilityState(w)
    return readACapabilityStorySteps(s) + readACapabilityScenarioSteps(s)
        + readACapabilityImageSteps(s)
}

@MainActor
private func readACapabilityStorySteps(_ s: ReadACapabilityState) -> [StepDefinition] {
    [
        def(#"^they see "([^"]*)", "([^"]*)" and "([^"]*)"$"#) { call in
            let (window, capability) = try await s.selectedCapability()
            guard capability.story == call.captures else {
                throw StepFailure(
                    "\(capability.name)'s story is \(capability.story), not \(call.captures)")
            }
            for line in call.captures {
                try expect(readACapabilityLines(window), line, "the window")
            }
        },
        def(#"^below the scenarios, about\.md is shown with the checkout flow diagram$"#) { _ in
            let about = try s.w.readFile("capabilities/checkout/about.md")
            let images = try readACapabilityImages(about)
            guard !images.isEmpty else {
                throw StepFailure("checkout/about.md shows no image, so no flow diagram")
            }
            let (window, capability) = try await s.selectedCapability()
            for image in images {
                let path = s.w.capabilities.appendingPathComponent("checkout/\(image)").path
                guard FileManager.default.fileExists(atPath: path) else {
                    throw StepFailure("checkout/about.md's image \(image) isn't on disk")
                }
                try expect([capability.about], image, "the about.md shown")
            }
            // Below the last scenario, so a long about.md doesn't push the list off screen.
            let lines = readACapabilityLines(window)
            let last = capability.scenarios.last?.name ?? ""
            let text = readACapabilityPlainLine(about)
            guard let at = readACapabilityIndex(lines, last),
                text.isEmpty || readACapabilityIndex(lines, text, from: at + 1) != nil
            else {
                throw StepFailure("the window doesn't show \"\(text)\" below \(last): \(lines)")
            }
        },
        def(
            #"^its (\d+) scenarios are listed, with happy paths and error states marked differently$"#
        ) { call in
            let (_, capability) = try await s.selectedCapability()
            let onDisk = try s.checkoutScenarios()
            let want = Int(call[0]) ?? -1
            guard onDisk.count == want && capability.scenarios.count == want else {
                throw StepFailure(
                    "\(capability.name) lists \(capability.scenarios.count) scenarios and its "
                        + "file has \(onDisk.count), not \(want)")
            }
            for (shown, disk) in zip(capability.scenarios, onDisk)
            where shown.name != disk.name || shown.error != disk.error {
                throw StepFailure(
                    "\"\(shown.name)\" (error state: \(shown.error)) is listed where "
                        + "\"\(disk.name)\" (error state: \(disk.error)) is in the file")
            }
            guard Set(onDisk.map(\.error)).count == 2 else {
                throw StepFailure("checkout.feature should hold both kinds of scenario")
            }
        },
    ]
}

@MainActor
private func readACapabilityScenarioSteps(_ s: ReadACapabilityState) -> [StepDefinition] {
    [
        def(#"^they select the "([^"]*)" scenario$"#) { call in
            s.scenario = call[0]
            try s.w.copySampleShop()
            try s.ensureSetup(call[0])
            try await s.w.front().select(call[0])
        },
        def(#"^its steps are shown (?:with their keywords lined up|as normal)$"#) { _ in
            let (window, shown, onDisk) = try await s.shownScenario()
            let steps = shown.steps.map { $0.trimmingCharacters(in: .whitespaces) }
            guard steps == onDisk.steps else {
                throw StepFailure("\"\(onDisk.name)\" shows \(shown.steps), not \(onDisk.steps)")
            }
            let lines = readACapabilityLines(window)
            var from = 0
            for want in readACapabilityLinedUp(onDisk.steps) {
                guard let found = lines[from...].firstIndex(of: want) else {
                    throw StepFailure(
                        "the window doesn't show \"\(want)\" after line \(from): \(lines)")
                }
                from = found + 1
            }
        },
        def(#"^its tags, (\S+) and (\S+), are shown above its steps$"#) { call in
            let (window, shown, onDisk) = try await s.shownScenario()
            let lines = readACapabilityLines(window)
            guard let first = readACapabilityLinedUp(onDisk.steps).first,
                let firstStep = lines.firstIndex(of: first)
            else { throw StepFailure("the window doesn't show \(onDisk.name)'s steps: \(lines)") }
            for tag in call.captures {
                guard onDisk.tags.contains(tag) else {
                    throw StepFailure("checkout.feature doesn't tag \(onDisk.name) with \(tag)")
                }
                guard shown.tags.contains(tag) else {
                    throw StepFailure("\(onDisk.name) has the tags \(shown.tags), without \(tag)")
                }
                let words = lines[..<firstStep].map { $0.split(separator: " ").map(String.init) }
                guard words.contains(where: { $0.contains(tag) }) else {
                    throw StepFailure("\(tag) isn't shown above \(onDisk.name)'s steps: \(lines)")
                }
            }
        },
        def(#"^its setup script is shown next to the steps$"#) { _ in
            let (_, shown, onDisk) = try await s.shownScenario()
            let script = "capabilities/checkout/scripts/\(slug(onDisk.name)).sh"
            let want = try s.w.readFile(script)
            guard shown.setup == want else {
                throw StepFailure(
                    "\(onDisk.name) shows the setup \"\(shown.setup)\", not \(script)")
            }
        },
        def(#"^the "([^"]*)" scenario has no setup yet$"#) { call in
            s.scenario = call[0]
            try s.w.copySampleShop()
            _ = try s.checkoutScenario(call[0])
            let scripts = s.w.capabilities.appendingPathComponent("checkout/scripts")
            for fileExtension in [".sh", ".md", ".teardown.sh"] {
                let file = scripts.appendingPathComponent(slug(call[0]) + fileExtension)
                if FileManager.default.fileExists(atPath: file.path) {
                    try FileManager.default.removeItem(at: file)
                }
            }
        },
        def(#"^in place of a setup they see "([^"]*)"$"#) { call in
            let (window, shown, onDisk) = try await s.shownScenario()
            guard shown.setup.isEmpty else {
                throw StepFailure(
                    "\(onDisk.name) has no setup files, yet shows the setup \"\(shown.setup)\"")
            }
            try expect(readACapabilityLines(window), call[0], "the window")
        },
    ]
}

@MainActor
private func readACapabilityImageSteps(_ s: ReadACapabilityState) -> [StepDefinition] {
    [
        def(#"^Checkout's about\.md shows an image that isn't on disk$"#) { _ in
            try s.w.copySampleShop()
            try s.w.writeFile("capabilities/checkout/about.md", ReadACapabilityMissingImage.about)
            let image = s.w.capabilities.appendingPathComponent("checkout/images/flow.svg")
            if FileManager.default.fileExists(atPath: image.path) {
                try FileManager.default.removeItem(at: image)
            }
            s.textBeforeImage = ReadACapabilityMissingImage.textBefore
            s.textAfterImage = ReadACapabilityMissingImage.textAfter
        },
        def(#"^the rest of the story is shown as normal$"#) { _ in
            let (window, capability) = try await s.selectedCapability()
            let story = try s.storyOnDisk()
            guard capability.story == story else {
                throw StepFailure("\(capability.name)'s story is \(capability.story), not \(story)")
            }
            for text in [s.textBeforeImage, s.textAfterImage] {
                try expect([capability.about], text, "the about.md shown")
            }
            let lines = readACapabilityLines(window)
            try readACapabilityStoryThenText(lines, story, s.textBeforeImage)
            try expect(lines, s.textAfterImage, "the window")
        },
        def(#"^where the image would be there's a placeholder naming "([^"]*)"$"#) { call in
            let lines = readACapabilityLines(try await s.w.front())
            guard let before = readACapabilityIndex(lines, s.textBeforeImage) else {
                throw StepFailure("the window doesn't show \"\(s.textBeforeImage)\": \(lines)")
            }
            // The placeholder names the image in words, not as the markdown or HTML for it.
            guard
                let placeholder = lines[(before + 1)...].firstIndex(where: { line in
                    line.contains(call[0]) && !line.contains("](") && !line.contains("<img")
                })
            else {
                throw StepFailure("no placeholder names \(call[0]) after the text before it")
            }
            guard readACapabilityIndex(lines, s.textAfterImage, from: placeholder + 1) != nil
            else {
                throw StepFailure(
                    "the placeholder for \(call[0]) isn't followed by \"\(s.textAfterImage)\"")
            }
        },
    ]
}
