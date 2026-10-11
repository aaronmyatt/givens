import GivensUI
import SwiftUI

/// The few colours and pieces every page shares, after the mockups: calm greys, blue Gherkin
/// keywords, green for a happy path or a pass, red for an error state or a fail.
/// Ref: https://developer.apple.com/design/human-interface-guidelines/color
enum Theme {
    /// Given, When, Then and their kin.
    static let keyword = Color.blue
    /// A happy path, a pass, a verified scenario.
    static let happy = Color.green
    /// An error state, a fail.
    static let error = Color.red
    /// The fill behind code: setup scripts and their output.
    static let codeFill = Color.primary.opacity(0.04)
    /// The line around code blocks and cards.
    static let hairline = Color.primary.opacity(0.12)

    /// Splits a step into its keyword and the rest: "Given a shopper" → ("Given", "a shopper").
    static func split(_ step: String) -> (keyword: String, text: String) {
        let trimmed = step.trimmingCharacters(in: .whitespaces)
        let parts = trimmed.split(separator: " ", maxSplits: 1)
        guard let first = parts.first else { return ("", trimmed) }
        return (String(first), parts.count > 1 ? String(parts[1]) : "")
    }
}

/// A scenario's status as a small symbol, its words in the tooltip and for VoiceOver, so a
/// long list stays quiet while "Never run" and the like are still there on hover.
/// Ref: https://developer.apple.com/documentation/swiftui/view/help(_:)-6d5vd
struct StatusIcon: View {
    let status: String

    var body: some View {
        Image(systemName: symbol)
            .foregroundStyle(tint)
            .help(status)
            .accessibilityLabel(status)
    }

    private var symbol: String {
        if status.hasPrefix("Verified") { return "checkmark.circle" }
        if status.hasPrefix("Last run failed") { return "xmark.circle" }
        if status.hasPrefix("Unverified") { return "exclamationmark.circle" }
        return "circle.dashed"
    }

    private var tint: Color {
        if status.hasPrefix("Verified") { return Theme.happy }
        if status.hasPrefix("Last run failed") { return Theme.error }
        if status.hasPrefix("Unverified") { return .orange }
        return .secondary
    }
}

/// The status in words, with its symbol: "✓ Verified 3 days ago".
struct StatusLine: View {
    let status: String

    var body: some View {
        if !status.isEmpty {
            HStack(spacing: 6) {
                StatusIcon(status: status)
                Text(status).foregroundStyle(.secondary)
            }
        }
    }
}

/// A small rounded tag, like "Error state" next to a scenario's name.
struct Badge: View {
    let text: String
    var tint: Color = .secondary

    var body: some View {
        Text(text)
            .font(.callout)
            .padding(.horizontal, 10)
            .padding(.vertical, 3)
            .foregroundStyle(tint)
            .background(tint.opacity(0.12), in: Capsule())
    }
}

/// A happy path or error state dot, as the scenario list marks them.
struct KindDot: View {
    let error: Bool

    var body: some View {
        Circle()
            .fill(error ? Theme.error : Theme.happy)
            .frame(width: 8, height: 8)
            .accessibilityLabel(error ? "Error state" : "Happy path")
    }
}

/// Monospaced text in a rounded, lightly filled box: a setup script or its output.
struct CodeBlock<Content: View>: View {
    @ViewBuilder let content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: 4) { content }
            .font(.system(.body, design: .monospaced))
            .textSelection(.enabled)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(14)
            .background(Theme.codeFill, in: RoundedRectangle(cornerRadius: 10))
            .overlay(RoundedRectangle(cornerRadius: 10).stroke(Theme.hairline))
    }
}

/// A scenario's steps lined up: keywords in blue in their own column, then the text.
/// Ref: https://developer.apple.com/documentation/swiftui/grid
struct StepsGrid: View {
    let steps: [String]

    var body: some View {
        Grid(alignment: .leadingFirstTextBaseline, horizontalSpacing: 16, verticalSpacing: 10) {
            ForEach(Array(steps.enumerated()), id: \.offset) { _, step in
                let parts = Theme.split(step)
                GridRow {
                    Text(parts.keyword)
                        .font(.system(.body, design: .monospaced))
                        .foregroundStyle(Theme.keyword)
                    Text(parts.text).textSelection(.enabled)
                }
            }
        }
    }
}

/// A small grey heading over a part of a page: "Scenarios", "Setup script".
struct SectionLabel: View {
    let text: String

    var body: some View {
        Text(text).font(.callout).foregroundStyle(.secondary)
    }
}

/// A warning line, or nothing when there's none.
struct WarningText: View {
    let warning: String

    var body: some View {
        if !warning.isEmpty {
            Label(warning, systemImage: "exclamationmark.triangle").foregroundStyle(.orange)
        }
    }
}

/// Buttons for whichever of `labels` the window offers, in the window's order.
struct OfferButtons: View {
    let window: WindowModel
    let labels: [String]
    var prominent: Set<String> = []

    var body: some View {
        ForEach(window.offers.filter(labels.contains), id: \.self) { offer in
            if prominent.contains(offer) {
                Button(offer) {
                    window.app.run(Feedback.after(offer)) { _ in try await window.choose(offer) }
                }
                .buttonStyle(.borderedProminent)
            } else {
                Button(offer) {
                    window.app.run(Feedback.after(offer)) { _ in try await window.choose(offer) }
                }
            }
        }
    }
}
