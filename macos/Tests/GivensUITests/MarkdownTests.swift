// Unit tests for the Markdown block reader that run notes and about.md are drawn from.
// Ref: https://developer.apple.com/documentation/testing
import Testing

@testable import GivensUI

@Suite struct MarkdownTests {
    @Test func readsANote() {
        let note = """
            ## Retry button

            On small screens
            it's hidden:

            - below the fold
            * under the **banner**
            1. scroll
            2) tap

            > looked off
            ---
            ![The declined card](declined.png)
            ```bash
            shopctl cards
            ```
            """
        #expect(
            Markdown.blocks(note) == [
                .heading(level: 2, text: "Retry button"),
                .paragraph("On small screens it's hidden:"),
                .bullets(["below the fold", "under the **banner**"]),
                .numbered(["scroll", "tap"]),
                .quote("looked off"),
                .rule,
                .image(alt: "The declined card", path: "declined.png"),
                .code(language: "bash", code: "shopctl cards"),
            ])
    }

    @Test func keepsAnUnclosedFence() {
        #expect(Markdown.blocks("```\nnever closed") == [.code(language: "", code: "never closed")])
    }

    @Test func leavesNonHeadingsAlone() {
        #expect(Markdown.blocks("#hashtag") == [.paragraph("#hashtag")])
        #expect(Markdown.blocks("![x](a b.png)") == [.paragraph("![x](a b.png)")])
    }
}
