package app

import (
	"strings"

	"givens/internal/assert"
)

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
	Language string // the fence's info string, "bash" or "js"; it labels the Copy button
	Code     string // exactly the code: no fences, no list number, no list indent
	Copyable bool   // false while the fence never closes
}

// SetupCard is one card on the Drop in page: a by_hand or ask step, or a markdown step.
type SetupCard struct {
	Kind    string      // CardByHand, CardAsk or CardMarkdown
	Where   string      // by_hand's <where>, like "DigitalOcean console"; "" otherwise
	Prompt  string      // ask's prompt; "" otherwise
	Text    string      // the card's text as shown, code included
	Link    string      // the --url or --qr link, or ""
	QRCode  bool        // the link is shown as a QR code, with the link as text beneath it
	Links   []string    // link targets in a markdown card's text
	Images  []string    // image paths shown in a markdown card
	Blocks  []CodeBlock // fenced code blocks, in order
	Offers  []string    // actions on this card: "Done", "Failed", "Copy", "Open", "Continue"
	Mark    string      // "", "done" or "failed"
	Warning string      // why the card is marked, like an unclosed code fence; or ""
}

// DropInRow is one Examples row on an outline's Drop in page.
type DropInRow struct {
	Examples  string            // the Examples block's name, or ""
	Values    map[string]string // column to value, like card=expired
	Steps     []string          // the outline's steps with this row's placeholders filled in
	Status    string            // like "Never run" or "Verified today"
	Offers    []string          // actions enabled on this row: "Drop in", "Switch to this row"
	Disabled  []string          // actions shown but disabled, like a greyed "Drop in"
	Live      bool              // this row's checklist is open beneath it
	Greyed    bool              // another row is live, so this one is greyed out
	Checklist []CheckItem       // the live row's checklist; nil otherwise
}

// UseURLOpener sets what opens links, like a by_hand --url card's "Open". Needed by
// "A step in the browser".
func (a *App) UseURLOpener(opener URLOpener) error {
	assert.That(a != nil, "use url opener runs on an app")
	assert.That(opener != nil, "an opener is given")
	return ErrNotImplemented
}

// Cards lists the Drop in page's setup cards in the order setup reached them: by_hand and ask
// cards from the script, then the markdown setup's cards. Needed by every card scenario.
func (s *Session) Cards() []SetupCard {
	assert.That(s != nil, "cards runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	return nil
}

// ChooseOnCard takes an action offered on one card, by the card's index in Cards: "Done",
// "Failed", "Copy", "Open" or "Continue". Needed by "A step done by hand" and the markdown
// card scenarios.
func (s *Session) ChooseOnCard(card int, action string) error {
	assert.That(s != nil, "choose on card runs on a session")
	assert.That(card >= 0, "a card index isn't negative")
	assert.That(card < SetupCardsMax, "a card index fits SetupCardsMax")
	assert.That(action != "", "an action has a label")
	return ErrNotImplemented
}

// CopyCodeBlock copies one code block of one card to the clipboard: exactly its code, with no
// fences, list number or list indent. Needed by "Copy a code block" and "Several code blocks".
func (s *Session) CopyCodeBlock(card int, block int) error {
	assert.That(s != nil, "copy code block runs on a session")
	assert.That(card >= 0 && card < SetupCardsMax, "a card index fits SetupCardsMax")
	assert.That(block >= 0 && block < CodeBlocksMax, "a block index fits CodeBlocksMax")
	return ErrNotImplemented
}

// ClickInlineCode copies a card's inline code span, like `sam@example.com`, to the clipboard.
// Needed by "Copy inline code".
func (s *Session) ClickInlineCode(card int, code string) error {
	assert.That(s != nil, "click inline code runs on a session")
	assert.That(card >= 0 && card < SetupCardsMax, "a card index fits SetupCardsMax")
	assert.That(code != "", "inline code has text")
	assert.That(!strings.Contains(code, "`"), "inline code is given without its backticks")
	return ErrNotImplemented
}

// DropInRows lists the Examples rows of the selected outline's Drop in page, one after
// another, in file order. Needed by the outline scenarios.
func (w *Window) DropInRows() []DropInRow {
	assert.That(w != nil, "drop in rows runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	return nil
}

// ChooseOnRow takes an action on one Examples row of the selected outline: "Drop in" or
// "Switch to this row". row is one of the row's values, like "expired". Needed by "Drop in to
// one row" and "Switch to another row".
func (w *Window) ChooseOnRow(row string, action string) error {
	assert.That(w != nil, "choose on row runs on a window")
	assert.That(row != "", "a row is named by one of its values")
	assert.That(action != "", "an action has a label")
	return ErrNotImplemented
}
