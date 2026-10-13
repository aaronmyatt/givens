import GivensUI
import SwiftUI
import UniformTypeIdentifiers

/// The Drop in page, in three numbered parts as in the mockup: the Given (setup's output and
/// any cards it's paused at), the When steps to tick off, and the Then checks to pass or
/// fail, with a note and "Record run" at the foot.
struct DropInPage: View {
    let window: WindowModel
    let session: SessionModel
    @State private var answer = ""
    @State private var note = ""
    @State private var pickingFiles = false
    @State private var previewingNote = false

    var body: some View {
        VStack(spacing: 0) {
            header
            Divider()
            ScrollView {
                VStack(alignment: .leading, spacing: 0) {
                    given
                    Divider()
                    // One part per run of steps, in the order they're written, so a Then
                    // comes right after the When it checks.
                    ForEach(Array(session.parts.enumerated()), id: \.offset) { index, part in
                        if part.kind == "then" {
                            then(part.items, number: index + 2)
                        } else {
                            when(part, number: index + 2)
                        }
                        Divider()
                    }
                    if !checks.isEmpty {
                        foot
                    }
                }
                .frame(maxWidth: 860, alignment: .leading)
            }
        }
    }

    /// Then items, passed or failed, from every Then part.
    private var checks: [CheckItem] { session.checklist.filter { $0.kind == "then" } }

    private var header: some View {
        HStack(spacing: 10) {
            // Selecting anything leaves the Drop in; the core asks first if marks would be
            // lost. Back goes to the scenario's own page.
            Button {
                window.app.run { _ in try await window.select(session.scenario) }
            } label: {
                Label("Back to the scenario", systemImage: "chevron.left")
            }
            .labelStyle(.iconOnly)
            .buttonStyle(.borderless)
            .help("Back to the scenario")
            .keyboardShortcut("[", modifiers: .command)
            // Every crumb leads back to the scenario's page, the screen before this one.
            if let capability = window.snapshot?.capability?.name {
                Text(capability).foregroundStyle(.secondary)
                Image(systemName: "chevron.right").foregroundStyle(.tertiary)
            }
            Button(session.scenario) {
                window.app.run { _ in try await window.select(session.scenario) }
            }
            .buttonStyle(.borderless)
            .font(.title3.bold())
            .help("Back to the scenario")
            Badge(text: "Drop in", tint: Theme.keyword)
            Spacer()
            OfferButtons(window: window, labels: ["Leave", "Reset state"])
        }
        .padding(.horizontal, 24)
        .padding(.vertical, 14)
        .background(.bar)
    }

    private var given: some View {
        VStack(alignment: .leading, spacing: 12) {
            StepHeading(number: "1", title: "Given", detail: givenDetail, done: isReady)
            CodeBlock {
                ForEach(Array(session.output.enumerated()), id: \.offset) { _, line in
                    Text(line).foregroundStyle(line.hasPrefix("✗") ? Theme.error : .primary)
                }
                if session.output.isEmpty {
                    Text(running ? "Starting setup…" : "No setup ran for this scenario.")
                        .foregroundStyle(.secondary)
                }
            }
            if !session.cards.isEmpty { card }
            HStack {
                OfferButtons(
                    window: window,
                    labels: [
                        "Stop", "Keep waiting", "Try again", "Edit script", "Edit setup",
                        "Run setup anyway", "Add teardown", "Edit teardown",
                    ])
            }
        }
        .padding(24)
    }

    private var givenDetail: String {
        switch session.status {
        case SessionModel.running: "setting up…"
        case SessionModel.waiting: "waiting on you"
        case SessionModel.failed: "setup failed"
        case SessionModel.stopped: "stopped"
        default: "state ready"
        }
    }

    private var isReady: Bool { session.status == SessionModel.ready }
    private var running: Bool { session.status == SessionModel.running }

    /// The card setup is paused at: a step done by hand, or a value to type.
    private var card: some View {
        VStack(alignment: .leading, spacing: 10) {
            ForEach(Array(session.cards.enumerated()), id: \.offset) { _, line in
                Text(line).textSelection(.enabled)
            }
            if session.status == SessionModel.waiting {
                TextField("Type the value", text: $answer)
                    .textFieldStyle(.roundedBorder)
                    .onSubmit { window.app.run { _ in try await window.type(answer) } }
            }
            HStack {
                OfferButtons(
                    window: window, labels: ["Done", "Failed", "Continue", "Open", "Copy"],
                    prominent: ["Done", "Continue"])
            }
        }
        .padding(16)
        .background(Theme.keyword.opacity(0.06), in: RoundedRectangle(cornerRadius: 10))
        .overlay(RoundedRectangle(cornerRadius: 10).stroke(Theme.keyword.opacity(0.3)))
    }

    /// Manual Givens or When steps, ticked off with Done.
    private func when(_ part: ChecklistPart, number: Int) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            StepHeading(
                number: String(number), title: part.kind == "given" ? "Given" : "When",
                detail: part.kind == "given" ? "by hand" : "your turn",
                done: part.items.allSatisfy { $0.mark == "done" })
            ForEach(part.items, id: \.text) { item in
                Toggle(
                    isOn: Binding(
                        get: { item.mark == "done" },
                        set: { on in
                            let mark = on ? "done" : "clear"
                            window.app.run { _ in try await session.mark(item.text, mark) }
                        })
                ) {
                    Text(item.text)
                }
                .toggleStyle(.checkbox)
            }
        }
        .padding(24)
    }

    /// Then steps, each passed or failed.
    private func then(_ items: [CheckItem], number: Int) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            StepHeading(
                number: String(number), title: "Then", detail: "what should be true",
                done: items.allSatisfy { !$0.mark.isEmpty })
            ForEach(items, id: \.text) { item in
                HStack {
                    Text(item.text)
                    Spacer()
                    // A second click on the chosen mark takes it off.
                    MarkButton(label: "Pass", chosen: item.mark == "pass", tint: Theme.happy) {
                        let mark = item.mark == "pass" ? "clear" : "pass"
                        window.app.run { _ in try await session.mark(item.text, mark) }
                    }
                    MarkButton(label: "Fail", chosen: item.mark == "fail", tint: Theme.error) {
                        let mark = item.mark == "fail" ? "clear" : "fail"
                        window.app.run { _ in try await session.mark(item.text, mark) }
                    }
                }
            }
        }
        .padding(24)
    }

    /// The foot of the page, below the last part: a note, files to keep, and "Record run".
    private var foot: some View {
        VStack(alignment: .leading, spacing: 12) {
            noteEditor
            attachments
            HStack {
                if checks.contains(where: { $0.mark.isEmpty }) {
                    Text("Mark each check first").foregroundStyle(Theme.error)
                }
                Spacer()
                if window.offers.contains("Record run") {
                    Button("Record run") {
                        window.app.run { _ in
                            if !note.isEmpty { try await session.setRunNote(note) }
                            try await window.choose("Record run")
                            // Once recorded, back to the scenario, whose status now shows the
                            // run. Still unmarked checks keep the page, with its message.
                            if !checks.contains(where: { $0.mark.isEmpty }) {
                                let failed = checks.contains { $0.mark == "fail" }
                                try await window.select(session.scenario)
                                Feedback.shared.say(
                                    failed ? "Run recorded: failed" : "Run recorded: passed",
                                    symbol: failed ? "xmark.seal.fill" : "checkmark.seal.fill")
                            }
                        }
                    }
                    .buttonStyle(.borderedProminent)
                }
            }
        }
        .padding(24)
        // Files dropped anywhere on the foot are attached, as "Add attachment…" does.
        // Ref: https://developer.apple.com/documentation/swiftui/view/dropdestination(for:action:istargeted:)
        .dropDestination(for: URL.self) { urls, _ in
            attach(urls)
            return !urls.isEmpty
        }
    }

    /// The note, in Markdown: written in one tab, read in the other. An image dropped on it
    /// is attached and linked at the end of the note.
    private var noteEditor: some View {
        VStack(alignment: .leading, spacing: 6) {
            // Ref: https://developer.apple.com/documentation/swiftui/segmentedpickerstyle
            Picker("Note", selection: $previewingNote) {
                Text("Write").tag(false)
                Text("Preview").tag(true)
            }
            .pickerStyle(.segmented)
            .labelsHidden()
            .fixedSize()
            Group {
                if previewingNote {
                    ScrollView {
                        if note.isEmpty {
                            Text("Nothing to preview").foregroundStyle(.secondary)
                                .frame(maxWidth: .infinity, alignment: .leading)
                        } else {
                            MarkdownView(text: note, resolve: attachedFile)
                        }
                    }
                    .padding(8)
                } else {
                    // Ref: https://developer.apple.com/documentation/swiftui/texteditor
                    TextEditor(text: $note)
                        .font(.system(.body, design: .monospaced))
                        .scrollContentBackground(.hidden)
                        .padding(4)
                        .dropDestination(for: URL.self) { urls, _ in
                            attach(urls)
                            return !urls.isEmpty
                        }
                }
            }
            .frame(minHeight: 120, maxHeight: 320)
            .background(Theme.codeFill, in: RoundedRectangle(cornerRadius: 8))
            .overlay(RoundedRectangle(cornerRadius: 8).stroke(Theme.hairline))
            Text("Markdown. Drop an image here to attach it and show it in the note.")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
    }

    /// An attached file by the name the note links it with, for the Preview tab: before
    /// "Record run", images are still where the tester keeps them.
    private func attachedFile(_ name: String) -> URL? {
        session.attachments.first { URL(fileURLWithPath: $0).lastPathComponent == name }
            .map { URL(fileURLWithPath: $0) }
    }

    /// True for a file whose extension names an image type, like .png or .heic.
    static func isImage(_ url: URL) -> Bool {
        UTType(filenameExtension: url.pathExtension)?.conforms(to: .image) ?? false
    }

    /// Files to keep with the run, like a screenshot, each with a button to take it off.
    /// "Record run" copies them into the run's own folder in runs/.
    private var attachments: some View {
        VStack(alignment: .leading, spacing: 6) {
            ForEach(session.attachments, id: \.self) { path in
                let name = URL(fileURLWithPath: path).lastPathComponent
                HStack(spacing: 6) {
                    Image(systemName: "paperclip").foregroundStyle(.secondary)
                    Text(name).help(path)
                    Button {
                        window.app.run { _ in try await session.detach(name) }
                    } label: {
                        Image(systemName: "xmark.circle.fill")
                    }
                    .buttonStyle(.borderless)
                    .foregroundStyle(.secondary)
                    .help("Remove \(name)")
                }
            }
            Button("Add attachment…", systemImage: "paperclip") { pickingFiles = true }
                .buttonStyle(.borderless)
        }
        // Ref: https://developer.apple.com/documentation/swiftui/view/fileimporter(ispresented:allowedcontenttypes:allowsmultipleselection:oncompletion:)
        .fileImporter(
            isPresented: $pickingFiles, allowedContentTypes: [.item],
            allowsMultipleSelection: true
        ) { picked in
            if case .success(let urls) = picked { attach(urls) }
        }
    }

    /// Attaches each file URL in turn; the core turns down a folder or a file that's too big.
    /// Each image is also linked at the end of the note, by name, which resolves against the
    /// run's folder once recorded.
    /// Ref: https://developer.apple.com/documentation/uniformtypeidentifiers/uttype-swift.struct/image
    private func attach(_ urls: [URL]) {
        let files = urls.filter(\.isFileURL)
        for url in files where Self.isImage(url) {
            let alt = url.deletingPathExtension().lastPathComponent
            let link = "![\(alt)](\(url.lastPathComponent))"
            note += (note.isEmpty || note.hasSuffix("\n") ? "" : "\n\n") + link + "\n"
        }
        window.app.run { _ in
            for url in files { try await session.attach(url.path) }
        }
    }
}

/// "② When · your turn": a numbered part of the Drop in page, ticked when it's done.
struct StepHeading: View {
    let number: String
    let title: String
    let detail: String
    let done: Bool

    var body: some View {
        HStack(spacing: 10) {
            ZStack {
                Circle().fill(done ? Theme.happy.opacity(0.15) : Theme.codeFill)
                if done {
                    Image(systemName: "checkmark").foregroundStyle(Theme.happy)
                } else {
                    Text(number).foregroundStyle(.secondary)
                }
            }
            .frame(width: 26, height: 26)
            Text("\(title) · \(detail)").foregroundStyle(.secondary)
        }
    }
}

/// Pass or Fail: filled with its colour, with a tick or cross, once chosen. A custom style,
/// since a bordered button on macOS ignores .tint.
/// Ref: https://developer.apple.com/documentation/swiftui/buttonstyle
struct MarkButton: View {
    let label: String
    let chosen: Bool
    let tint: Color
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Label(label, systemImage: label == "Pass" ? "checkmark" : "xmark")
                .labelStyle(MarkLabelStyle(showsIcon: chosen))
                .frame(minWidth: 56)
        }
        .buttonStyle(MarkButtonStyle(chosen: chosen, tint: tint))
        .accessibilityValue(chosen ? "chosen" : "")
        .help(chosen ? "Click again to clear" : label)
    }
}

/// The icon only once a mark is chosen, so unmarked rows stay quiet.
struct MarkLabelStyle: LabelStyle {
    let showsIcon: Bool

    func makeBody(configuration: Configuration) -> some View {
        HStack(spacing: 4) {
            if showsIcon { configuration.icon.font(.caption.bold()) }
            configuration.title
        }
    }
}

/// A capsule-ish button: outlined when not chosen, tinted fill and text when chosen.
struct MarkButtonStyle: ButtonStyle {
    let chosen: Bool
    let tint: Color

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .padding(.horizontal, 12)
            .padding(.vertical, 5)
            .foregroundStyle(chosen ? tint : Color.primary)
            .fontWeight(chosen ? .semibold : .regular)
            .background(
                RoundedRectangle(cornerRadius: 7)
                    .fill(chosen ? tint.opacity(0.16) : Color.clear)
            )
            .overlay(
                RoundedRectangle(cornerRadius: 7)
                    .stroke(chosen ? tint.opacity(0.7) : Theme.hairline, lineWidth: 1)
            )
            .opacity(configuration.isPressed ? 0.6 : 1)
            .contentShape(RoundedRectangle(cornerRadius: 7))
    }
}
