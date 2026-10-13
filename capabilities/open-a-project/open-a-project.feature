Feature: Open a project
  As a developer joining or coming back to a project
  I want to open its capabilities folder in Givens
  So that I can see what the app does without digging through raw files

  Givens reads the folder directly and keeps no copy or database of its own. Anything that
  writes these files, including agents, editors and `git pull`, shows up in Givens as soon as
  it's saved.

  Scenario: Open a documented project
    Given a developer has the sample shop project, with checkout/checkout.feature, invites/invites.feature and sign-in/sign-in.feature
    When they open its capabilities folder in Givens
    Then the sidebar lists "Checkout", "Invites" and "Sign in", from each Feature: line
    And each capability shows how many scenarios it has
    And the Checkout story is shown

  Scenario: Open the project folder instead
    Given a developer has the sample shop project
    When they open the project's root folder in Givens
    Then Givens finds the capabilities folder inside it
    And the window title shows the path to that capabilities folder

  @error
  Scenario: Open a folder with no capabilities
    Given a developer has a project with no capabilities folder
    When they open it in Givens
    Then they see "No capabilities here yet"
    And they're offered "Create capabilities folder"
    And nothing is written to the project

  Scenario: Create a capabilities folder
    Given a developer has a project with no capabilities folder open in Givens
    When they choose "Create capabilities folder"
    Then the project gets a capabilities folder containing GIVENS.md and _shared/givens.sh
    And it contains the starter capability "Add a story", with add-a-story.feature, its about.md and setup scripts
    And it contains .gitattributes marking runs/ as merge=union
    And the sidebar lists "Add a story"

  Scenario: Files written by other tools appear
    Given a developer has the sample shop project open in Givens
    And "Checkout" is selected
    When another tool adds a scenario to checkout.feature
    Then "Pay with a gift card" appears in the Checkout scenarios within two seconds
    And "Checkout" stays selected

  # This is what lets an agent write scenarios while you watch them arrive.
  #
  # With nothing to reopen, Givens shows a welcome window with "Open folder…" and recent
  # projects. From a terminal, `givens <folder>` opens a project, like `code .`. The command is
  # a small script inside the app that runs `open -a Givens`, so `open -a Givens .` also works
  # without installing anything.
  Scenario: Start with nothing open
    Given a developer has never opened a project in Givens
    When they open Givens
    Then they see a welcome window with "Open folder…"
    And choosing "Open folder…" shows a folder picker

  Scenario: Reopen a recent project
    Given a developer has opened the sample shop project in Givens before
    And no project was open when they last quit Givens
    When they open Givens
    Then the welcome window lists the sample shop under "Recent"
    And choosing it opens the project

  Scenario: Drop a folder on Givens
    Given a developer has Givens in their Dock
    When they drag the sample shop's folder onto it
    Then the sample shop project opens in Givens

  Scenario: Install the givens command
    Given a developer has Givens open
    When they choose "Install givens command" from the Givens menu
    Then givens works in new terminal windows
    And Givens says where it installed it

  Scenario: Open a project from the terminal
    Given a developer has installed the givens command
    When they run "givens ." in the sample shop's folder
    Then the sample shop project opens in Givens

  @error
  Scenario: Open a folder that doesn't exist from the terminal
    Given a developer has installed the givens command
    When they run "givens ~/code/nope"
    Then the command prints "There's no folder at ~/code/nope"
    And Givens doesn't open a window

  Scenario: Open a second project
    Given a developer has the sample shop project open in Givens
    When they open another project's capabilities folder
    Then it opens in a second window
    And each window's title shows its own project's path

  Scenario: Open a project that's already open
    Given a developer has the sample shop project open in Givens
    When they open the sample shop's capabilities folder again
    Then its existing window comes to the front
    And no second window opens

  Scenario: Reopen windows after quitting
    Given a developer has two projects open in Givens
    When they quit Givens and open it again
    Then both projects reopen in their own windows

  # Another branch's capabilities can be read without checking that branch out. Givens copies
  # its capabilities folder out of git into Givens's own data folder, so the working tree is
  # never touched. The window is read only: its setup scripts belong to code that isn't
  # checked out.
  Scenario: Read capabilities from another branch
    Given the sample shop is a git repository with a branch "gift-cards" that adds the scenario "Pay with a gift card" to Checkout
    And a developer has the sample shop project open in Givens
    When they open the branch "gift-cards"
    Then a second window opens, its title ending "@ gift-cards"
    And it lists "Pay with a gift card" under "Checkout"
    And the sample shop's own checkout.feature doesn't have "Pay with a gift card"
    And "Drop in" isn't offered for "Pay with a gift card"

  @error
  Scenario: Open a branch that doesn't exist
    Given the sample shop is a git repository with a branch "gift-cards" that adds the scenario "Pay with a gift card" to Checkout
    And a developer has the sample shop project open in Givens
    When they open the branch "no-such-branch"
    Then they see "No branch called no-such-branch"
    And no second window opens
