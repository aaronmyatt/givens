package model

// Limits for editing. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	// EditorTextBytesMax bounds a file open in the editor; it matches the spec reader's limit
	// on a .feature file.
	EditorTextBytesMax = 4 << 20
)

// StoryDraft is what a developer fills in on the New story dialog: the capability's name and
// the three halves of its story, without their "As a", "I want" and "So that" prefixes.
type StoryDraft struct {
	Name   string // "Refunds"
	AsA    string // "a shop owner"
	IWant  string // "to refund an order"
	SoThat string // "the shopper gets their money back without emailing support"
}

// ScenarioDraft is what a developer fills in on the New scenario dialog.
type ScenarioDraft struct {
	Name  string // "Expired card"
	Error bool   // tag it @error
}

// EditorMark is a problem the editor marks on one line of the file it holds.
type EditorMark struct {
	Line    int    `json:"line"`    // 1-based line number in the editor's text
	Message string `json:"message"` // "Expected a step, a tag, a comment or a new scenario"
}

// StepSuggestion is one step offered while typing in the editor, drawn from steps already in
// the project.
type StepSuggestion struct {
	Text  string `json:"text"`  // the step without its keyword
	Label string `json:"label"` // how widely it's used: "used in 4"
}

// PreviewLine is one line of the editor's preview, as the library would show it.
type PreviewLine struct {
	Text string `json:"text"` // the line as shown, keywords lined up for steps
	Mark string `json:"mark"` // the parser's message when the line doesn't parse, or ""
}
