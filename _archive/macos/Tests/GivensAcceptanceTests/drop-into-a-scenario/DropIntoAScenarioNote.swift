import Foundation
import GherkinKit
import GivensUI

// A run note written in Markdown, kept whole as note.md in the run's folder.

/// Writing a Markdown note, and where it's kept.
@MainActor
func dropInNoteSteps(_ s: DropIntoAScenarioState) -> [StepDefinition] {
    [
        def(
            #"^they write a note in Markdown, with a heading, a list and the image "([^"]*)"$"#
        ) { call in
            s.note =
                "## \(DropIntoAScenarioState.noteHeading)\n\nOn small screens:\n\n"
                + "- it's below the fold\n- the banner covers it\n\n"
                + "![The declined card](\(call[0]))\n"
            try await s.session().setRunNote(s.note)
        },
        def(#"^the run line keeps the note's heading as its note$"#) { _ in
            let fields = try s.runLines(s.runKey).last ?? []
            guard fields.last == DropIntoAScenarioState.noteHeading else {
                throw StepFailure("the run line's note is \"\(fields.last ?? "")\"")
            }
        },
        def(#"^the whole note is kept as note\.md in the run's folder$"#) { _ in
            let note = try s.newestRunFolder().appendingPathComponent("note.md")
            let text = (try? String(contentsOf: note, encoding: .utf8)) ?? ""
            guard text == s.note else { throw StepFailure("note.md holds \"\(text)\"") }
        },
        def(#"^the run, in the scenario's run history, shows the whole note$"#) { _ in
            let window = try await s.w.front()
            try await window.select(s.scenario)
            guard let newest = window.snapshot?.runs.first else {
                throw StepFailure("the scenario's page lists no runs")
            }
            let folder = try s.newestRunFolder().path
            guard newest.noteMarkdown == s.note, samePath(newest.folder, folder) else {
                throw StepFailure(
                    "the newest run shows \"\(newest.noteMarkdown)\" in \(newest.folder)")
            }
        },
    ]
}

extension DropIntoAScenarioState {
    /// The heading the Markdown note starts with.
    static let noteHeading = "Retry button"

    /// The newest run's folder under runs/files/<run key>/: folders are named by time, so the
    /// newest sorts last.
    func newestRunFolder() throws -> URL {
        let folder = w.capabilities.appendingPathComponent("runs/files/\(runKey)")
        let runs = (try? FileManager.default.contentsOfDirectory(atPath: folder.path)) ?? []
        guard let newest = runs.max() else {
            throw StepFailure("runs/files/\(runKey) has no run folder")
        }
        return folder.appendingPathComponent(newest)
    }
}
