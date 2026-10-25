Feature: Work in the terminal
  As a developer who lives in a terminal
  I want to browse, drop in to and edit a project's scenarios from the givens command
  So that I never leave the terminal to work on them

  Bare `givens`, in a terminal, opens a TUI (a text UI: full screen, driven by the keyboard)
  over the project the terminal is in; `givens <folder>` opens it over that folder's. It's
  Givens's one window: the list, a capability's story, a scenario, and the Drop in page. Text is edited in the developer's own editor, `$VISUAL` or
  `$EDITOR`, as `git commit` does: Givens writes the text to a temporary file, waits for the
  editor to close, checks it, and saves it only when it's valid. `list`, `run` and `check`
  stay plain text, for agents and CI. See use-givens-from-the-terminal.

  Background:
    Given a developer's terminal is in the sample shop's folder

  # Browsing. The TUI opens on the project's capabilities, each folded to its heading until
  # it's unfolded.

  Scenario: Browse a project
    When they run "givens" in a terminal
    Then they see each capability with its scenario count, folded
    And unfolding "Checkout" shows each scenario with its status, like "Card declined" with "Never run"

  Scenario: Start a capabilities folder from the TUI
    Given a developer's terminal is in a project with no capabilities folder
    When they run "givens" there
    Then the TUI says "No capabilities here yet" and offers "c Create capabilities folder"
    And pressing "c" makes the capabilities folder, and the TUI lists "Add a story (6)"

  Scenario: Read a scenario
    Given a developer has the sample shop open in the TUI
    When they move to "Card declined"
    Then its steps show beside the list, with its setup script's path

  Scenario: Read a capability's story
    Given a developer has the sample shop open in the TUI
    When they open the "Checkout" story
    Then they see its As a, I want and So that lines, and its about.md
    And pressing Esc goes back to the list

  Scenario: A long list scrolls
    Given a developer has the sample shop open in a terminal 12 lines tall
    When they press "z", then Down until the cursor is on the last scenario
    Then "Expired link" is on show, with the cursor on it
    And "Checkout (5)" has scrolled off the top

  Scenario: Views and tags
    Given a developer has the sample shop open in the TUI
    When they press Tab until "Views" is shown and choose "Error states"
    Then the list holds the scenarios tagged @error

  # Keys on the list. Capabilities, Views and Tags are each a list of headings over
  # scenarios, and every key acts the same on each. The tag payments is added for the Tags tab:
  # the sample shop has no tags of its own.

  Scenario Outline: Fold and unfold on every tab
    Given "Pay with saved card" and "Card declined" are tagged @payments
    And a developer has the sample shop open in the TUI
    When they press Tab until "<tab>" is shown
    Then "<heading>" is folded, with "Card declined" not listed
    And Space on "<heading>" lists "Card declined", and Space again folds it away
    And "l" on "<heading>" lists "Card declined", and "h" folds it away
    And "z" unfolds every heading, and "z" again folds them all

    Examples:
      | tab          | heading      |
      | Capabilities | Checkout     |
      | Views        | Error states |
      | Tags         | payments     |

  Scenario Outline: Move through the list on every tab
    Given "Pay with saved card" and "Card declined" are tagged @payments
    And a developer has the sample shop open in the TUI
    When they press Tab until "<tab>" is shown and press "l" on "<heading>"
    Then pressing "l" again puts the cursor on "<first>"
    And pressing "j" puts the cursor on "<second>", with "<first>" still listed
    And pressing "h" puts the cursor back on "<top>"
    And pressing "k" wraps the cursor round to the last row, and "j" back to "<top>"
    And pressing "G" puts the cursor on the last row, and "g" back on "<top>"

    Examples:
      | tab          | heading      | first               | second            | top              |
      | Capabilities | Checkout     | Pay with saved card | Card declined     | Checkout (5)     |
      | Views        | Error states | Card declined       | 3-D Secure failed | Error states (5) |
      | Tags         | payments     | Pay with saved card | Card declined     | payments (2)     |

  Scenario Outline: Scenario keys under every tab
    Given "Pay with saved card" and "Card declined" are tagged @payments
    And a developer's editor is vim, and changes nothing
    And a developer has the sample shop open in the TUI
    When they press Tab until "<tab>" is shown, open "<heading>" and move to "Card declined"
    Then pressing "e" opens checkout.feature in vim at the line of "Scenario: Card declined"
    And pressing "s" opens card-declined.sh in vim
    And pressing "r" shows its runs: "Never run"
    And pressing "l" drops in to "Card declined"

    Examples:
      | tab          | heading      |
      | Capabilities | Checkout     |
      | Views        | Error states |
      | Tags         | payments     |

  Scenario: Search from the TUI
    Given a developer has the sample shop open in the TUI
    When they press "/" and type "declined"
    Then "Card declined" is among the results
    And pressing Enter on it shows "Card declined"

  # Drop in. The core's Session runs setup, shows its cards and the checklist, and records
  # the run; the page gives each of its offers a key.

  Scenario: Drop in from the TUI
    Given a developer has the sample shop open in the TUI
    When they move to "Card declined" and press Enter
    Then setup's output scrolls by as it runs
    And once setup is done, the checklist shows "When they place the order" and "Then they see "Your card was declined""

  Scenario: Read the whole scenario in a Drop in
    Given a developer has dropped in to "Card declined" in the TUI and setup is done
    Then "Given a signed-in shopper with one item in the cart" shows just above "When they place the order"

  Scenario: Edit the scenario from a Drop in
    Given a developer's editor is vim
    And a developer has dropped in to "Card declined" in the TUI and setup is done
    When they press "e"
    Then vim opens checkout.feature's text at the line of "Scenario: Card declined"
    And once they save a change and quit, checkout.feature holds it

  Scenario: Edit the setup from a Drop in
    Given a developer has dropped in to "Card declined" in the TUI and setup is done
    Then pressing "s" opens card-declined.sh in their editor

  # Setup scripts and helpers. s on a scenario edits its setup; the Helpers tab lists the
  # shell functions setup scripts share.

  Scenario: Edit a scenario's setup from the list
    Given a developer has the sample shop open in the TUI
    When they move to "Card declined"
    Then pressing "s" opens card-declined.sh in their editor

  Scenario: Start a setup script from the list
    Given a developer's editor adds a line to the file it opens
    And "Empty cart" has no setup yet
    And a developer has the sample shop open in the TUI
    When they move to "Empty cart" and press "s"
    Then checkout/scripts/empty-cart.sh is written with the kit's source line, and opens in their editor

  Scenario: Helpers
    Given a developer's editor is vim
    And a developer has the sample shop open in the TUI
    When they press Tab until "Helpers" is shown
    Then each helper is listed with where it's defined, like "seed_shopper" with "_shared/users.sh"
    And pressing Enter on "seed_shopper" opens users.sh in vim at its function

  Scenario: Add a helper from the TUI
    Given a developer's editor names a new helper "seed_gift_card" in _shared
    And a developer has the sample shop open in the TUI
    When they press Tab until "Helpers" is shown and press "n"
    Then capabilities/_shared/seed_gift_card.sh defines seed_gift_card
    And "seed_gift_card" is listed

  Scenario: Drop in by name
    When they run "givens drop-in checkout/card-declined" in a terminal
    Then setup runs and the checklist shows

  Scenario: A step done by hand in the TUI
    Given the "Card declined" setup has a by_hand step for "Phone" with the text "Approve the sign-in"
    When a developer drops in to "Card declined" in the TUI
    Then they see "By hand, in Phone" and "Approve the sign-in"
    And pressing Enter marks it done, and setup carries on

  Scenario: Answer an ask in the TUI
    Given the "Card declined" setup asks for INVITE_CODE with "Paste the invite code"
    When a developer drops in to "Card declined" in the TUI
    And they type "K7Q2" and press Enter
    Then setup carries on with INVITE_CODE set to "K7Q2"

  Scenario: Steps done by hand as cards in the TUI
    Given "Pay in app" has only scripts/pay-in-app.md
    When a developer drops in to "Pay in app" in the TUI
    Then step 1 is shown as a card
    And pressing "c" on a card copies its inline code or code block
    And pressing Enter on each card in turn shows the checklist

  Scenario: Setup that runs long in the TUI
    Given the "Card declined" setup takes longer than its limit of 1 second
    When a developer drops in to "Card declined" in the TUI
    Then they see "Still running after 1 second"
    And they can press "k" to keep waiting or "s" to stop

  Scenario: Record a run from the TUI
    Given a developer has dropped in to "Card declined" in the TUI and setup is done
    When they mark each step with "p" or Enter, write a note in their editor with "n", and press "r"
    Then the run is recorded with the note
    And "Card declined" shows "Verified today"

  Scenario: See how a Drop in went once every step is marked
    Given a developer has dropped in to "Card declined" in the TUI and setup is done
    When they mark every step done or passed but the last, which they mark "f"
    Then the Drop in page says "1 step failed · r to record"
    And marking that step "p" instead says "All steps passed · r to record"
    And pressing "r" records the run and says "✓ Run recorded: all steps passed"

  Scenario: Attach a file to a run in the TUI
    Given a developer has dropped in to "Card declined" in the TUI and setup is done
    When they press "a" and give the path of a screenshot
    And they record the run
    Then the screenshot is kept with the run

  Scenario: Pick an outline's row in the TUI
    Given checkout.feature has the outline "Card rejected by the bank"
    And a developer has the sample shop open in the TUI
    When they press Enter on "Card rejected by the bank"
    Then they pick one of its Examples rows before setup runs

  @error
  Scenario: Setup fails in the TUI
    Given the "Card declined" setup fails with exit 3
    When a developer drops in to "Card declined" in the TUI
    Then they see "Setup failed (exit 3)"
    And pressing "s" opens card-declined.sh in their editor

  # Editing, in the developer's own editor.

  Scenario: Edit a scenario in your editor
    Given a developer's editor is vim
    When they run "givens edit checkout/card-declined"
    Then vim opens checkout.feature's text at the line of "Scenario: Card declined"
    And once they save a change and quit, checkout.feature holds it

  Scenario: Edit a setup script or about.md
    Given a developer's editor adds a line to the file it opens
    When a developer runs "givens edit checkout/scripts/card-declined.sh"
    Then their editor opens the script's text, in a file ending in .sh
    And once they save a change and quit, the script holds it

  @error
  Scenario: An edit that breaks the file
    Given a developer is editing "Card declined" with "givens edit"
    When they add the line "Check the order is pending" among its steps and quit
    Then their editor opens again with "# givens: line 17: Expected a step, a tag, a comment or a new scenario" at the top
    And checkout.feature is left as it was until they fix it or quit without saving

  @error
  Scenario: A setup script edit that doesn't parse
    Given a developer's editor adds the line "if then" to the end of the file it opens
    When they run "givens edit checkout/scripts/card-declined.sh"
    Then their editor opens again with "# givens: line 9: Syntax error near unexpected token `then'" at the top
    And card-declined.sh is left as it was until they fix it or quit without saving

  Scenario: Quit without changes
    When a developer runs "givens edit checkout/card-declined" and quits without saving
    Then it prints "No changes"
    And nothing is written

  @error
  Scenario: The file changes while it's in your editor
    Given a developer is editing "Card declined" with "givens edit"
    When another tool saves a change to checkout.feature
    And they save and quit
    Then it prints "checkout.feature changed on disk while you edited it"
    And their edit is kept in a file whose path it prints

  # Writing, renaming and deleting, as plain commands too.

  Scenario: New story from the terminal
    When a developer runs "givens new story gift-cards" and fills in the template
    Then capabilities/gift-cards/gift-cards.feature holds the story they wrote

  Scenario: New scenario from the terminal
    When a developer runs "givens new scenario checkout" and names it "Gift card declined"
    Then checkout.feature gets "Scenario: Gift card declined" tagged @todo
    And checkout/scripts/gift-card-declined.sh is created

  Scenario: Rename from the terminal
    When a developer runs "givens rename checkout/card-declined "Card refused""
    Then checkout.feature has "Scenario: Card refused"
    And its setup script is now checkout/scripts/card-refused.sh

  @error
  Scenario: Delete a scenario another builds on, from the terminal
    When a developer runs "givens delete checkout/pay-with-saved-card"
    Then it prints "3-D Secure challenge builds on this scenario"
    And nothing is deleted unless they run it again with --yes

  Scenario: Flag a scenario to do from the terminal
    When a developer runs "givens todo checkout/card-declined"
    Then "Card declined" is tagged @todo
    And the TUI flags "Card declined" with ⚑ in the list
    And "givens done checkout/card-declined" takes the tag off

  # Helpers, history, search, trust and sync, as plain commands.

  Scenario: Helpers, history and search
    Given "Card declined" has a passing run recorded
    When a developer runs "givens helpers", "givens history checkout/card-declined" and "givens search declined"
    Then the first lists seed_shopper with where it's defined and how many scripts call it
    And the second lists the passing run, newest first
    And the third lists "Card declined"

  Scenario: Trust a project from the terminal
    Given a developer has never run anything from the sample shop
    When they drop in to "Card declined" in the TUI
    Then they see "Run scripts from this project?" with "t" to trust and "d" not to
    And "givens trust" trusts it from the command line

  Scenario: Share runs from the terminal
    Given the sample shop is a git repository with a remote, with a run recorded while it couldn't be reached
    When a developer runs "givens sync"
    Then the remote's refs/givens/runs holds the run

  @error
  Scenario: A TUI command with no terminal
    When a developer runs "givens drop-in checkout/card-declined" with its output piped
    Then it prints "givens drop-in needs a terminal; use givens run checkout/card-declined"
    And it exits with 2

