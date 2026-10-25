package app

// Drop in page limits. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	SetupCardsMax = 256 // cards one Drop in can show: by_hand, ask and markdown steps
	CodeBlocksMax = 64  // code blocks in one card
)

// Setup card kinds.
const (
	CardByHand   = "by_hand"  // a by_hand step in a setup script
	CardAsk      = "ask"      // an ask step waiting for a typed value
	CardMarkdown = "markdown" // one numbered item of scripts/<slug>.md
)

// URLOpener opens a link in the default browser. The Swift UI passes one that calls
// NSWorkspace; tests pass a fake that records the links.
// Ref: https://developer.apple.com/documentation/appkit/nsworkspace/open(_:)
type URLOpener interface {
	OpenURL(url string) error
}

// CodeBlock is one fenced code block in a setup card.
type CodeBlock struct {
	Language string `json:"language"` // the fence's info string; it labels the Copy button
	Code     string `json:"code"`     // exactly the code: no fences, no list number or indent
	Copyable bool   `json:"copyable"` // false while the fence never closes
	Run      bool   `json:"run"`      // "bash run" or "js run": Givens runs it by itself
}

// SetupCard is one card on the Drop in page: a by_hand or ask step, or a markdown step.
type SetupCard struct {
	Kind   string `json:"kind"`   // CardByHand, CardAsk or CardMarkdown
	Where  string `json:"where"`  // by_hand's <where>, like "DigitalOcean console"
	Prompt string `json:"prompt"` // ask's prompt; "" otherwise
	Text   string `json:"text"`   // the card's text as shown, code included
	Link   string `json:"link"`   // the --url link, or ""
	// Link targets and image paths in a markdown card's text.
	Links  []string    `json:"links"`
	Images []string    `json:"images"`
	Blocks []CodeBlock `json:"blocks"` // fenced code blocks, in order
	// Actions on this card: "Done", "Failed", "Copy", "Open", "Continue".
	Offers  []string `json:"offers"`
	Mark    string   `json:"mark"`    // "", "done" or "failed"
	Warning string   `json:"warning"` // why the card is marked, like an unclosed fence
	Output  []string `json:"output"`  // what its run blocks printed, in order

	inline    []string // inline code spans, which copy when clicked
	runBlocks []int    // its run blocks, as indices into Blocks; see cards_run.go
}

// DropInRow is one Examples row on an outline's Drop in page.
type DropInRow struct {
	Examples string            `json:"examples"` // the Examples block's name, or ""
	Values   map[string]string `json:"values"`   // column to value, like card=expired
	Steps    []string          `json:"steps"`    // the steps with this row's values filled in
	Status   string            `json:"status"`   // like "Never run" or "Verified today"
	// Actions enabled on this row, and those shown but disabled, like a greyed "Drop in".
	Offers   []string `json:"offers"`
	Disabled []string `json:"disabled"`
	Live     bool     `json:"live"`   // this row's checklist is open beneath it
	Greyed   bool     `json:"greyed"` // another row is live, so this one is greyed out
	// The live row's checklist; empty otherwise.
	Checklist []CheckItem `json:"checklist"`
}
