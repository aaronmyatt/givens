Feature: Read a capability
  As a developer getting to know part of an app
  I want to see a capability's story, about.md and scenarios together
  So that I understand what it's for and how it behaves before I change it

  Capabilities are listed with their scenario counts. Selecting one shows its story, its
  scenarios and its about.md; selecting a scenario shows its steps with its setup script.
  Views, such as "Error states", list scenarios from every capability at once. In the TUI,
  what's under the cursor shows beside the list, and Enter on a capability opens its story
  and about.md to read. See work-in-the-terminal.

  Scenario: Read a story
    Given a developer has the sample shop project open in Givens
    When they select "Checkout"
    Then they see "As a returning shopper", "I want to pay with my saved card" and "So that checkout takes seconds"
    And its 5 scenarios are listed, each marked as a happy path or an error state
    And after the scenarios, about.md is shown with the checkout flow diagram

  Scenario: Read a scenario
    Given a developer has the sample shop project open in Givens
    When they select the "Card declined" scenario
    Then its steps are shown with their keywords lined up
    And its tags, @error and @timeout:5m, are shown above its steps
    And its setup script is shown with its steps
    And they can choose "Copy" or "Drop in"

  # s on a scenario in the TUI creates the script.
  @error
  Scenario: A scenario with no setup
    Given a developer has the sample shop project open in Givens
    And the "Empty cart" scenario has no setup yet
    When they select "Empty cart"
    Then its steps are shown as normal
    And in place of a setup they see "No setup yet"
    And they're offered "Create script" and "Write setup steps"

  @error
  Scenario: A story image that isn't on disk
    Given a developer has the sample shop project open in Givens
    And Checkout's about.md shows an image that isn't on disk
    When they select "Checkout"
    Then the rest of the story is shown as normal
    And where the image would be there's a placeholder naming "images/flow.svg"

  # A scenario's recorded runs are listed newest first, with who ran each and its note, so
  # what's been tried shows before dropping in. r on a scenario in the TUI shows them, and
  # `givens history <key>` prints them.
  Scenario: See a scenario's runs
    Given a developer has the sample shop project open in Givens
    And "Card declined" has a passing run recorded
    When they select the "Card declined" scenario
    Then one run is listed for it, "Passed today"

  # "Copy steps" copies the scenario as Gherkin, to paste into a ticket or a chat.
  Scenario: Copy a scenario's steps
    Given a developer has the sample shop project open in Givens
    When they select the "Card declined" scenario
    And they choose "Copy steps"
    Then the clipboard holds "Card declined" as Gherkin, its Scenario: line then its steps
