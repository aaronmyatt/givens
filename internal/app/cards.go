package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/markdown"
)

// Card actions.
const (
	OfferDone      = "Done"
	OfferFailed    = "Failed"
	OfferOpen      = "Open"
	OfferContinue  = "Continue"
	OfferEditSetup = "Edit setup"
)

// Messages on the Drop in page about cards.
const (
	MessageEnterValue      = "Enter a value first"
	MessageStoppedAtByHand = "Setup stopped at a step done by hand"
	// MessageNoStepsYet follows the markdown setup's file name, as in "pay-in-app.md has no
	// steps yet".
	MessageNoStepsYet = " has no steps yet"
)

// cardHeader starts the line the kit prints for a card under GIVENS_UI=1: the record
// separator, then "givens-card". See _givens_card in internal/kit/givens.sh.
const cardHeader = "\x1egivens-card\t"

// UseURLOpener sets what opens links, like a by_hand --url card's "Open". Needed by
// "A step in the browser".
func (a *App) UseURLOpener(opener URLOpener) error {
	assert.That(a != nil, "use url opener runs on an app")
	assert.That(opener != nil, "an opener is given")
	a.opener = opener
	return nil
}

// Cards lists the Drop in page's setup cards in the order setup reached them: by_hand and ask
// cards from the script, then the markdown setup's cards. Needed by every card scenario.
func (s *Session) Cards() []SetupCard {
	assert.That(s != nil, "cards runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	s.poll()
	return append(make([]SetupCard, 0, len(s.cards)), s.cards...)
}

// readOutput moves the current script's new output into display, taking card lines out and
// turning them into cards.
func (s *Session) readOutput() {
	assert.That(s != nil, "read output runs on a session")
	assert.That(s.process != nil, "a script is running")
	lines := s.process.Output()
	for _, line := range lines[s.parsed:] {
		switch {
		case s.reading > 0:
			last := &s.cards[len(s.cards)-1]
			last.Text = strings.TrimPrefix(last.Text+"\n"+line, "\n")
			s.snippets[len(s.snippets)-1] = last.Text
			s.reading--
		case strings.HasPrefix(line, cardHeader):
			s.addScriptCard(strings.Split(strings.TrimPrefix(line, cardHeader), "\t"))
		default:
			s.display = append(s.display, line)
		}
	}
	s.parsed = len(lines)
	if s.reading == 0 && s.answering() && s.status == SessionRunning {
		s.status, s.waitingSince = SessionWaiting, s.window.app.config.Clock.Now()
	}
}

// addScriptCard adds a by_hand or ask card from the kit's fields: kind, where, link, and how
// many lines of text follow.
func (s *Session) addScriptCard(fields []string) {
	assert.That(s != nil, "add card runs on a session")
	assert.That(len(s.cards) <= SetupCardsMax, "cards stay bounded")
	if len(fields) != 4 || len(s.cards) >= SetupCardsMax {
		return
	}
	lines, err := strconv.Atoi(fields[3])
	if err != nil || lines < 0 {
		lines = 0
	}
	card := SetupCard{
		Kind: fields[0], Link: fields[2], Links: []string{}, Images: []string{},
		Blocks: []CodeBlock{}, Output: []string{},
	}
	if card.Kind == CardAsk {
		card.Offers = []string{OfferContinue}
	} else {
		card.Kind, card.Where = CardByHand, fields[1]
		card.Offers = []string{OfferDone, OfferFailed}
		if lines > 0 {
			card.Offers = append(card.Offers, OfferCopy)
		}
		if card.Link != "" {
			card.Offers = append(card.Offers, OfferOpen)
		}
	}
	s.cards, s.snippets, s.reading = append(s.cards, card), append(s.snippets, ""), lines
}

// answering is true while the last script card waits for an answer.
func (s *Session) answering() bool {
	assert.That(s != nil, "answering runs on a session")
	assert.That(len(s.cards) == len(s.snippets), "each card has a snippet slot")
	if len(s.cards) == 0 {
		return false
	}
	last := s.cards[len(s.cards)-1]
	return last.Kind != CardMarkdown && last.Mark == ""
}

// finishCardText finishes a script card once its text is read: an ask card's prompt.
func (s *Session) finishCardText() {
	assert.That(s != nil, "finish card runs on a session")
	assert.That(s.reading == 0, "the card's text is all read")
	if len(s.cards) == 0 {
		return
	}
	last := &s.cards[len(s.cards)-1]
	if last.Kind == CardAsk && last.Prompt == "" {
		last.Prompt = last.Text
	}
}

// addMarkdownCards adds one card per numbered step of scripts/<slug>.md, once its scripts
// have run, and waits for each to be marked.
func (s *Session) addMarkdownCards() error {
	assert.That(s.scenario.SetupCards != "", "the scenario has a markdown setup")
	assert.That(s.markdownFrom < 0, "markdown cards are added once")
	text, err := helpers.ReadScript(s.scenario.SetupCards)
	if err != nil {
		return err
	}
	parsed, err := markdown.Parse(text)
	if err != nil {
		return err
	}
	// With no step there's no card to mark, so waiting could never end. "Write setup steps"
	// starts the file as "1." alone, which is an empty list item, not a step.
	// Ref: https://spec.commonmark.org/0.31.2/#list-items
	if len(parsed.Steps) == 0 {
		s.fail(filepath.Base(s.scenario.SetupCards) + MessageNoStepsYet)
		s.markdownFailed = true
		return nil
	}
	s.intro, s.markdownFrom = parsed.Intro, len(s.cards)
	for _, step := range parsed.Steps {
		card := SetupCard{
			Kind: CardMarkdown, Text: markdown.Rendered(step.Text),
			Links:  append([]string{}, step.Links...),
			Images: append([]string{}, step.Images...), Blocks: make([]CodeBlock, 0, 2),
			Offers: []string{OfferDone, OfferFailed}, inline: step.Inline,
			Output: []string{},
		}
		for index, block := range step.Blocks {
			runs := runLanguage(block) != ""
			card.Blocks = append(card.Blocks, CodeBlock{
				Language: block.Language, Code: block.Code, Copyable: block.Closed, Run: runs,
			})
			if runs {
				card.runBlocks = append(card.runBlocks, index)
			}
			if !block.Closed {
				card.Warning = fmt.Sprintf("The code block starting on line %d never closes",
					block.Line)
			}
		}
		// A card that runs marks itself, so it has nothing to choose.
		if len(card.runBlocks) > 0 {
			card.Offers = []string{}
		}
		s.cards, s.snippets = append(s.cards, card), append(s.snippets, "")
	}
	return s.runNextCard()
}

// ChooseOnCard takes an action offered on one card, by the card's index in Cards: "Done",
// "Failed", "Copy", "Open" or "Continue".
func (s *Session) ChooseOnCard(card int, action string) error {
	assert.That(s != nil, "choose on card runs on a session")
	assert.That(action != "", "an action has a label")
	s.poll()
	if card < 0 || card >= len(s.cards) {
		return fmt.Errorf("there's no card %d", card)
	}
	chosen := &s.cards[card]
	switch action {
	case OfferCopy:
		return s.window.app.config.Clipboard.WriteText(s.snippets[card])
	case OfferOpen:
		return s.window.openLink(chosen.Link)
	}
	if chosen.Mark != "" {
		return fmt.Errorf("card %d is already marked %q", card, chosen.Mark)
	}
	if chosen.Kind == CardMarkdown {
		return s.markMarkdownCard(card, action)
	}
	return s.answerScriptCard(chosen, action)
}

// answerScriptCard answers the waiting by_hand or ask card on the script's stdin.
func (s *Session) answerScriptCard(chosen *SetupCard, action string) error {
	assert.That(chosen != nil, "a card is chosen")
	assert.That(chosen.Kind != CardMarkdown, "script cards are answered on stdin")
	if s.process == nil || s.status != SessionWaiting {
		return errors.New("setup isn't waiting at this card")
	}
	answer, mark := "done", MarkDone
	switch {
	case action == OfferContinue && chosen.Kind == CardAsk:
		if s.cardInput == "" {
			s.message = MessageEnterValue
			return nil
		}
		answer = s.cardInput
	case action == OfferFailed && chosen.Kind == CardByHand:
		answer, mark, s.cardFailed = "failed", "failed", true
	case action != OfferDone || chosen.Kind != CardByHand:
		return fmt.Errorf("%q isn't offered on this card", action)
	}
	if err := s.process.Answer(answer); err != nil {
		return err
	}
	chosen.Mark, s.cardInput, s.message = mark, "", ""
	s.resume()
	return nil
}

// markMarkdownCard marks one markdown card; the last Done shows the checklist, and a Failed
// stops setup there.
func (s *Session) markMarkdownCard(card int, action string) error {
	assert.That(s.cards[card].Kind == CardMarkdown, "the card is a markdown step")
	assert.That(s.markdownFrom >= 0, "markdown cards were added")
	steps := len(s.cards) - s.markdownFrom
	switch action {
	case OfferDone:
		s.cards[card].Mark = MarkDone
	case OfferFailed:
		s.cards[card].Mark = "failed"
		s.resume()
		s.fail(fmt.Sprintf("Setup stopped at step %d of %d", card-s.markdownFrom+1, steps))
		s.markdownFailed = true
		return nil
	default:
		return fmt.Errorf("%q isn't offered on a markdown card", action)
	}
	return s.runNextCard()
}

// resume ends a wait at a card, adding the time waited so it doesn't count as slow.
func (s *Session) resume() {
	assert.That(s != nil, "resume runs on a session")
	assert.That(!s.started.IsZero(), "the session started")
	if s.status == SessionWaiting {
		s.waited += s.window.app.config.Clock.Now().Sub(s.waitingSince)
		s.status = SessionRunning
	}
}

// CopyCodeBlock copies one code block of one card to the clipboard: exactly its code, with no
// fences, list number or indent. Needed by "Copy a code block" and "Several code blocks".
func (s *Session) CopyCodeBlock(card int, block int) error {
	assert.That(s != nil, "copy code block runs on a session")
	assert.That(block >= 0 && block < CodeBlocksMax, "a block index is in range")
	if card < 0 || card >= len(s.cards) || block >= len(s.cards[card].Blocks) {
		return fmt.Errorf("card %d has no code block %d", card, block)
	}
	if !s.cards[card].Blocks[block].Copyable {
		return errors.New("the code block never closes, so it has no Copy button")
	}
	return s.window.app.config.Clipboard.WriteText(s.cards[card].Blocks[block].Code)
}

// ClickInlineCode copies a card's inline code span, like `sam@example.com`, to the clipboard.
// Needed by "Copy inline code".
func (s *Session) ClickInlineCode(card int, code string) error {
	assert.That(s != nil, "click inline code runs on a session")
	assert.That(code != "", "inline code has text")
	if card < 0 || card >= len(s.cards) {
		return fmt.Errorf("there's no card %d", card)
	}
	for _, span := range s.cards[card].inline {
		if span == code {
			return s.window.app.config.Clipboard.WriteText(code)
		}
	}
	return fmt.Errorf("card %d has no inline code %q", card, code)
}

// pendingCard is the card the window's own buttons answer: the script card waiting for an
// answer, or the first markdown card not yet marked; -1 when none waits.
func (s *Session) pendingCard() int {
	assert.That(s != nil, "pending card runs on a session")
	assert.That(len(s.cards) <= SetupCardsMax, "cards stay bounded")
	if s.status != SessionWaiting {
		return -1
	}
	for index, card := range s.cards {
		if card.Mark == "" {
			return index
		}
	}
	return -1
}

// CopyCard copies a card's code: its first closed code block, or else its first inline code
// span. The givens command's TUI does it on c, where the app has a button per block and span.
func (s *Session) CopyCard(card int) error {
	assert.That(s != nil, "copy card runs on a session")
	assert.That(len(s.cards) <= SetupCardsMax, "cards stay bounded")
	if card < 0 || card >= len(s.cards) {
		return fmt.Errorf("there's no card %d", card)
	}
	for index, block := range s.cards[card].Blocks {
		if block.Copyable {
			return s.CopyCodeBlock(card, index)
		}
	}
	if len(s.cards[card].inline) > 0 {
		return s.ClickInlineCode(card, s.cards[card].inline[0])
	}
	return errors.New("this card has no code to copy")
}

// PendingCard is the card waiting on the developer, as an index into Cards, or -1.
func (s *Session) PendingCard() int {
	assert.That(s != nil, "pending card runs on a session")
	assert.That(s.window != nil, "a session belongs to a window")
	s.poll()
	return s.pendingCard()
}
