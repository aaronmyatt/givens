Feature: Open a project
  As a developer joining or coming back to a project
  I want to open its capabilities folder with givens
  So that I can see what the app does without digging through raw files

  Givens reads the folder directly and keeps no copy or database of its own. Anything that
  writes these files, including agents, editors and `git pull`, shows up in the TUI and the
  givens commands as soon as it's saved. Bare `givens` opens the TUI on the project the
  terminal is in, and `givens <folder>` on the one in that folder, like `code .`; either finds
  the capabilities folder in the folder or a folder above it.

  Scenario: Open a documented project
    Given a developer has the sample shop project, with checkout/checkout.feature, invites/invites.feature and sign-in/sign-in.feature
    When they open its capabilities folder
    Then Givens lists "Checkout", "Invites" and "Sign in", from each Feature: line
    And each capability shows how many scenarios it has
    And the Checkout story is shown beside the list

  Scenario: Open the project folder instead
    Given a developer has the sample shop project
    When they open the project's root folder
    Then Givens finds the capabilities folder inside it
    And the top line of the TUI shows the path to that capabilities folder

  @error
  Scenario: Open a folder with no capabilities
    Given a developer has a project with no capabilities folder
    When Givens opens it
    Then they see "No capabilities here yet"
    And they're offered "Create capabilities folder"
    And nothing is written to the project

  Scenario: Create a capabilities folder
    Given a developer has opened a project with no capabilities folder
    When they choose "Create capabilities folder"
    Then the project gets a capabilities folder containing GIVENS.md and _shared/givens.sh
    And it contains the starter capability "Add a story", with add-a-story.feature, its about.md and setup scripts
    And it contains .gitattributes marking runs/ as merge=union
    And Givens lists "Add a story"

  # This is what lets an agent write scenarios while you watch them arrive.
  Scenario: Files written by other tools appear
    Given a developer has the sample shop project open
    And "Checkout" is selected
    When another tool adds a scenario to checkout.feature
    Then "Pay with a gift card" appears in the Checkout scenarios within two seconds
    And "Checkout" stays selected

  # With no terminal, as when its output is piped, `givens <folder>` prints its usage and exits
  # 2; see use-givens-from-the-terminal for the plain commands.
  Scenario: Open a project from the terminal
    Given a developer has the sample shop project
    When they run "givens ." in a terminal in the sample shop's folder
    Then the TUI opens on the sample shop's capabilities folder
    And its list holds "Checkout", "Invites" and "Sign in", folded

  @error
  Scenario: Open a folder that doesn't exist from the terminal
    Given a developer has no folder at ~/code/nope
    When they run "givens ~/code/nope" in a terminal
    Then the command prints "There's no folder at ~/code/nope"
    And the TUI doesn't open

  # Another branch's capabilities can be read without checking that branch out. Givens copies
  # its capabilities folder out of git into Givens's own data folder, so the working tree is
  # never touched. The copy is read only: its setup scripts belong to code that isn't checked
  # out.
  Scenario: Read capabilities from another branch
    Given the sample shop is a git repository with a branch "gift-cards" that adds the scenario "Pay with a gift card" to Checkout
    And a developer has the sample shop project open
    When they open the branch "gift-cards"
    Then Givens opens a read-only copy of it, its title ending "@ gift-cards"
    And it lists "Pay with a gift card" under "Checkout"
    And the sample shop's own checkout.feature doesn't have "Pay with a gift card"
    And "Drop in" isn't offered for "Pay with a gift card"

  @error
  Scenario: Open a branch that doesn't exist
    Given the sample shop is a git repository with a branch "gift-cards" that adds the scenario "Pay with a gift card" to Checkout
    And a developer has the sample shop project open
    When they open the branch "no-such-branch"
    Then they see "No branch called no-such-branch"
    And no copy of a branch opens
