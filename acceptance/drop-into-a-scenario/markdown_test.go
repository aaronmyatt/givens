package dropintoascenario

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"givens/acceptance/world"
	"givens/internal/app"
	"givens/internal/slug"
)

const (
	payInApp    = "Pay in app"
	bashCode    = "sudo -u shop shopctl flags set new_checkout on"
	jsCode      = `localStorage.removeItem("cart")`
	betaLink    = "https://testflight.apple.com/join/abc"
	trustPrompt = "Run scripts from this project?"
)

// codeBlocksSetup is pay-in-app.md with a bash block under step 2 and a js block under step 3,
// each indented under its list item, as the spec's comment shows.
const codeBlocksSetup = "You'll need the TestFlight build of the shop app.\n" +
	"\n" +
	"1. Install it on your phone: [Shop beta](" + betaLink + ")\n" +
	"2. Turn on the new checkout in the server console:\n" +
	"\n" +
	"   ```bash\n" +
	"   " + bashCode + "\n" +
	"   ```\n" +
	"\n" +
	"3. Clear the cart in the browser console:\n" +
	"\n" +
	"   ```js\n" +
	"   " + jsCode + "\n" +
	"   ```\n"

// unclosedSetup is pay-in-app.md with step 2's fence opened on line 4 and never closed.
const unclosedSetup = "1. Install it on your phone: [Shop beta](" + betaLink + ")\n" +
	"2. Turn on the new checkout in the server console:\n" +
	"\n" +
	"   ```bash\n" +
	"   " + bashCode + "\n" +
	"\n" +
	"3. Clear the cart in the browser console.\n"

// markdownSetup makes name a scenario whose setup is only scripts/<slug>.md, holding content.
// It doesn't trust the project: a markdown setup runs nothing, so it needs no trust.
func (s *state) markdownSetup(name string, content string) error {
	s.Untrusted = true
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	if err := s.EnsureScenario("checkout", name); err != nil {
		return err
	}
	s.scenario = name
	scripts := filepath.Join("capabilities", "checkout", "scripts")
	shell := filepath.Join(s.Project, scripts, slug.Of(name)+".sh")
	if err := os.Remove(shell); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.WriteFile(filepath.Join(scripts, slug.Of(name)+".md"), content)
}

func (s *state) markdownOnly(name string, scriptSlug string) error {
	if slug.Of(name) != scriptSlug {
		return fmt.Errorf("%q's setup is %s.md, not %s.md", name, slug.Of(name), scriptSlug)
	}
	return s.markdownSetup(name, world.PayInAppSetup)
}

// markdownCards waits for setup to pause at its cards, and returns them.
func (s *state) markdownCards() (*app.Session, []app.SetupCard, error) {
	session, err := s.settleTo(app.SessionWaiting)
	if err != nil {
		return nil, nil, err
	}
	cards := make([]app.SetupCard, 0, 4)
	for _, card := range session.Cards() {
		if card.Kind == app.CardMarkdown {
			cards = append(cards, card)
		}
	}
	if len(cards) == 0 {
		return nil, nil, fmt.Errorf("setup paused with no markdown cards: %+v", session.Cards())
	}
	return session, cards, nil
}

func (s *state) notAskedToTrust() error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := world.ExpectNot(window.Visible(), trustPrompt, "the window"); err != nil {
		return err
	}
	if slices.Contains(window.Offers(), "Trust") {
		return fmt.Errorf("the window offers \"Trust\": %q", window.Offers())
	}
	_, _, err = s.markdownCards()
	return err
}

func (s *state) introAboveCards(intro string, count string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := world.Expect(window.Visible(), intro, "the window"); err != nil {
		return err
	}
	_, cards, err := s.markdownCards()
	if err != nil {
		return err
	}
	if strconv.Itoa(len(cards)) != count {
		return fmt.Errorf("there are %d cards, not %s: %+v", len(cards), count, cards)
	}
	if !slices.Contains(cards[0].Links, betaLink) {
		return fmt.Errorf("the first card doesn't link to %s: %+v", betaLink, cards[0])
	}
	last := cards[len(cards)-1]
	for _, image := range last.Images {
		if strings.HasSuffix(image, "images/wallet.png") {
			return nil
		}
	}
	return fmt.Errorf("the last card doesn't show the wallet screenshot: %+v", last)
}

// doneOnLastCard marks each card "Done" in turn, expecting the checklist only after the last.
func (s *state) doneOnLastCard() error {
	session, cards, err := s.markdownCards()
	if err != nil {
		return err
	}
	for index := range cards {
		if len(session.Checklist()) > 0 {
			return fmt.Errorf("the checklist showed before card %d was done", index+1)
		}
		if err := session.ChooseOnCard(index, "Done"); err != nil {
			return err
		}
	}
	if _, err := s.settleTo(app.SessionReady); err != nil {
		return err
	}
	if len(session.Checklist()) == 0 {
		return errors.New("no checklist after the last card was done")
	}
	return nil
}

// scriptAndMarkdown gives name a script that prints one line, and the spec's markdown cards.
func (s *state) scriptAndMarkdown(name string, shell string, markdown string) error {
	if shell != slug.Of(name) || markdown != slug.Of(name) {
		return fmt.Errorf("%q's setup files are named %s, not %s/%s", name, slug.Of(name),
			shell, markdown)
	}
	if err := s.markdownSetup(name, world.PayInAppSetup); err != nil {
		return err
	}
	if err := s.writeSetup(name, script(`ok "app build installed"`)); err != nil {
		return err
	}
	return s.prepare()
}

func (s *state) scriptOutputFirst() error {
	session, _, err := s.markdownCards()
	if err != nil {
		return err
	}
	return world.Expect(session.Output(), "✓ app build installed", "the output")
}

func (s *state) markdownCardsShown(markdown string) error {
	_, cards, err := s.markdownCards()
	if err != nil {
		return err
	}
	if len(cards) != 3 {
		return fmt.Errorf("%s.md gave %d cards, not 3: %+v", markdown, len(cards), cards)
	}
	return nil
}

func (s *state) firstCardDone(name string) error {
	if err := s.markdownSetup(name, world.PayInAppSetup); err != nil {
		return err
	}
	if err := s.DropIn(name); err != nil {
		return err
	}
	session, _, err := s.markdownCards()
	if err != nil {
		return err
	}
	return session.ChooseOnCard(0, "Done")
}

func (s *state) chooseOnCard(action string, ordinal string) error {
	session, cards, err := s.markdownCards()
	if err != nil {
		return err
	}
	index := map[string]int{"first": 0, "second": 1, "third": 2, "last": len(cards) - 1}
	return session.ChooseOnCard(index[ordinal], action)
}

func (s *state) codeBlocksSetup() error {
	return s.markdownSetup(payInApp, codeBlocksSetup)
}

// openCards drops in to the scenario's markdown setup, unless a Drop in is already open.
func (s *state) openCards() (*app.Session, []app.SetupCard, error) {
	if s.Session == nil {
		if err := s.DropIn(s.scenario); err != nil {
			return nil, nil, err
		}
	}
	return s.markdownCards()
}

func (s *state) copyBashBlock() error {
	session, _, err := s.openCards()
	if err != nil {
		return err
	}
	return session.CopyCodeBlock(1, 0)
}

func (s *state) cleanCopy() error {
	copied, err := s.Clipboard.ReadText()
	if err != nil {
		return err
	}
	if strings.Contains(copied, "```") || strings.HasPrefix(copied, "2.") {
		return fmt.Errorf("the copy holds a fence or list number: %q", copied)
	}
	for _, line := range strings.Split(copied, "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			return fmt.Errorf("the copy holds a leading space: %q", copied)
		}
	}
	return nil
}

func (s *state) lookAtCards() error {
	_, _, err := s.openCards()
	return err
}

func (s *state) labelledCopyButtons(first string, second string) error {
	_, cards, err := s.markdownCards()
	if err != nil {
		return err
	}
	if len(cards) != 3 {
		return fmt.Errorf("there are %d cards, not 3: %+v", len(cards), cards)
	}
	for index, language := range map[int]string{1: first, 2: second} {
		blocks := cards[index].Blocks
		if len(blocks) != 1 || blocks[0].Language != language || !blocks[0].Copyable {
			return fmt.Errorf("step %d doesn't have one %s block with Copy: %+v", index+1,
				language, blocks)
		}
	}
	return nil
}

// copiesApart copies each block and expects exactly its own code on the clipboard.
func (s *state) copiesApart() error {
	session, _, err := s.markdownCards()
	if err != nil {
		return err
	}
	for card, code := range map[int]string{1: bashCode, 2: jsCode} {
		if err := session.CopyCodeBlock(card, 0); err != nil {
			return err
		}
		copied, err := s.Clipboard.ReadText()
		if err != nil {
			return err
		}
		if strings.TrimRight(copied, "\n") != code {
			return fmt.Errorf("copying step %d's block gave %q, not %q", card+1, copied, code)
		}
	}
	return nil
}

// cardSays drops in to the spec's pay-in-app.md and finds the card showing text.
func (s *state) cardSays(text string) error {
	if err := s.markdownSetup(payInApp, world.PayInAppSetup); err != nil {
		return err
	}
	_, cards, err := s.openCards()
	if err != nil {
		return err
	}
	for index, card := range cards {
		if strings.Contains(card.Text, text) {
			s.cardIndex = index
			return nil
		}
	}
	return fmt.Errorf("no card says %q: %+v", text, cards)
}

func (s *state) clickInlineCode(code string) error {
	session, err := s.session()
	if err != nil {
		return err
	}
	return session.ClickInlineCode(s.cardIndex, code)
}

func (s *state) unclosedFence() error {
	return s.markdownSetup(payInApp, unclosedSetup)
}

func (s *state) stepCardShows(step string, text string) error {
	_, cards, err := s.markdownCards()
	if err != nil {
		return err
	}
	index, err := strconv.Atoi(step)
	if err != nil || index < 1 || index > len(cards) {
		return fmt.Errorf("there's no card for step %s: %+v", step, cards)
	}
	card := cards[index-1]
	return world.Expect([]string{card.Warning, card.Text}, text, "step "+step+"'s card")
}

// noCopyUntilClosed expects step 2's block not to be copyable, then closes the fence, drops in
// again, and expects it to be.
func (s *state) noCopyUntilClosed() error {
	_, cards, err := s.markdownCards()
	if err != nil {
		return err
	}
	if len(cards) < 2 || hasCopy(cards[1]) {
		return fmt.Errorf("step 2's card offers Copy with its fence open: %+v", cards)
	}
	closed := strings.Replace(unclosedSetup, bashCode+"\n", bashCode+"\n   ```\n", 1)
	if err := s.markdownSetup(payInApp, closed); err != nil {
		return err
	}
	if err := s.DropIn(payInApp); err != nil {
		return err
	}
	_, cards, err = s.markdownCards()
	if err != nil {
		return err
	}
	if len(cards) < 2 || !hasCopy(cards[1]) {
		return fmt.Errorf("step 2's card has no Copy once its fence is closed: %+v", cards)
	}
	return nil
}

// hasCopy reports whether a card offers Copy on a code block.
func hasCopy(card app.SetupCard) bool {
	for _, block := range card.Blocks {
		if block.Copyable {
			return true
		}
	}
	return slices.Contains(card.Offers, "Copy")
}

// markdownHolding makes name's setup only scripts/<slug>.md, holding exactly content, as
// "Write setup steps" leaves it before a step is written.
func (s *state) markdownHolding(name string, scriptSlug string, content string) error {
	if slug.Of(name) != scriptSlug {
		return fmt.Errorf("%q's setup is %s.md, not %s.md", name, slug.Of(name), scriptSlug)
	}
	return s.markdownSetup(name, content+"\n")
}

// editorAfterDropIn chooses action on the Drop in, then expects the Drop in gone, since the
// Mac page draws a Drop in over the editor, and file open in the editor.
func (s *state) editorAfterDropIn(action string, file string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := window.Choose(action); err != nil {
		return err
	}
	if window.Session() != nil {
		return fmt.Errorf("the Drop in is still open after %q", action)
	}
	want := filepath.Join(s.Capabilities(), "checkout", "scripts", file)
	if window.Editing() != want {
		return fmt.Errorf("the editor holds %q, not %q", window.Editing(), want)
	}
	return nil
}
