Feature: Write stories and scenarios
  As a developer, or an agent working for one
  I want to write stories, scenarios and scripts in Givens and have them saved as plain files
  So that anyone can read and change them with whatever tool they like

  The editor edits the real .feature file, tags and comments and all. Saving writes back only
  what changed, so the git diff of an edit made in Givens looks like one made by hand. Step
  autocomplete draws on steps already in the project, which keeps wording identical and step
  search useful.

  @wip
  Scenario: Write a new story
    Given a developer has the sample shop project open in Givens
    When they choose "New story" and fill in "Refunds", "a shop owner", "to refund an order" and "the shopper gets their money back without emailing support"
    Then refunds/refunds.feature is created, with "Feature: Refunds" and those lines as its description
    And "Refunds" appears in the sidebar

  @wip
  Scenario: Add a scenario to a capability
    Given a developer has the sample shop project open in Givens
    When they choose "New scenario" in Checkout, name it "Expired card" and mark it as an error state
    Then a "Scenario: Expired card" block tagged @error is added to checkout.feature
    And checkout/scripts/expired-card.sh is created, starting with the line that sources givens.sh
    And the new scenario opens in the editor

  @wip
  Scenario: Reuse an existing step
    Given a developer is writing a new scenario in Checkout
    When they type "Given a signed-in sh"
    Then they're offered "a signed-in shopper with one item in the cart" with "used in 4"
    And "a signed-in shopper with an empty cart" with "used in 1"
    And choosing one inserts it word for word

  @wip
  Scenario: Saving changes only what was changed
    Given a developer has the sample shop project in git and open in Givens
    When they change "the order stays pending" to "the order is marked failed" in "Card declined"
    And they save
    Then git diff shows that one line changed and nothing else

  @wip @error
  Scenario: The file changes on disk while it's being edited
    Given a developer has unsaved changes to "Card declined"
    When another tool saves a different change to checkout.feature
    Then they see "This file changed on disk"
    And they can choose "Keep mine", "Use theirs" or "Compare"

  @wip @error
  Scenario: A line that isn't a step
    Given a developer is editing "Card declined"
    When they add the line "Check the order is pending" among its steps
    And they save
    Then that line is marked "Expected a step, a tag, a comment or a new scenario"
    And the file is saved anyway

  @wip
  Scenario: Rename a scenario
    Given a developer has the sample shop project open in Givens
    When they rename "Pay with saved card" to "Pay with a saved card"
    Then its Scenario: line is updated, and its setup script is renamed to pay-with-a-saved-card.sh
    And the from line in 3-D Secure challenge's setup is updated to match
    And it shows "Never run", since its runs were recorded under the old name

  @wip
  Scenario: Rename a capability
    Given a developer has the sample shop project open in Givens
    When they rename "Sign in" to "Log in"
    Then the sidebar lists "Log in" in place of "Sign in"
    And its folder and .feature file are renamed to log-in and log-in.feature, with its scripts and helpers inside
    And its scenarios show "Never run"

  @wip
  Scenario: Renaming a capability updates from lines
    Given a developer has the sample shop project open in Givens
    And the "Accept invite" setup starts with "from sign-in/magic-link"
    When they rename "Sign in" to "Log in"
    Then that line becomes "from log-in/magic-link"
    And "Magic link" and "Expired link" show "Never run"

  # Runs are recorded against a scenario's path, so renaming a scenario or its capability starts
  # its runs over. Old lines stay in `runs/` as history; Givens never rewrites them.
  @wip
  Scenario: Warned before a rename
    Given "Card declined" has 3 runs recorded
    When a developer renames it to "Card refused"
    Then they see "Renaming starts Card declined's runs over"
    And they're offered "Rename" and "Cancel"

  @wip
  Scenario: A renamed scenario starts at Never run
    Given "Card declined" has a passing run recorded today
    When a developer renames it to "Card refused" and confirms
    Then "Card refused" shows "Never run"
    And it's listed in the Unverified view
    And nothing in runs/ changes

  @wip
  Scenario: Scenarios built on it are unverified too
    Given "3-D Secure challenge" has a passing run recorded
    When a developer renames "Pay with saved card" to "Pay with a saved card" and confirms
    Then the from line in 3-D Secure challenge's setup is updated
    And "3-D Secure challenge" shows "Unverified: setup changed since the last run"

  @wip
  Scenario: Delete a scenario
    Given a developer has the sample shop project open in Givens
    When they delete "Empty cart"
    And they confirm
    Then its block is removed from checkout.feature, and its setup script is moved to the Trash
    And Checkout shows 4 scenarios

  @wip @error
  Scenario: Delete a scenario another builds on
    Given a developer has the sample shop project open in Givens
    When they delete "Pay with saved card"
    Then they see "3-D Secure challenge builds on this scenario"
    And nothing is deleted unless they choose "Delete anyway"

  @wip
  Scenario: Delete a capability
    Given a developer has the sample shop project open in Givens
    When they delete "Invites"
    And they confirm
    Then the invites folder is moved to the Trash
    And the sidebar lists "Checkout" and "Sign in"

  @wip @error
  Scenario: Delete a capability another builds on
    Given a developer has the sample shop project open in Givens
    And the "Accept invite" setup starts with "from sign-in/magic-link"
    When they delete "Sign in"
    Then they see "Accept invite builds on a scenario in Sign in"
    And nothing is deleted unless they choose "Delete anyway"

  # Deleted scripts and folders go to the Trash, so they can be brought back in Finder. A
  # deleted scenario's block comes back with Undo or git.
  @wip
  Scenario: Preview a scenario while editing
    Given a developer is editing "Card declined"
    When they choose "Preview"
    Then the scenario is shown as it reads in the library, with its keywords lined up
    And their unsaved changes are included

  @wip
  Scenario: Preview a story with an image
    Given a developer is editing Checkout's about.md
    When they choose "Preview"
    Then the checkout flow diagram is shown as it will look in the library

  @wip @error
  Scenario: Preview a line that isn't a step
    Given a developer is editing "Card declined"
    And one line among its steps doesn't start with Given, When, Then, And, But or *
    When they choose "Preview"
    Then that line is shown marked with the parser's message
    And the steps around it are shown as normal

  @wip
  Scenario: Write setup steps
    Given "Pay in app" has no setup yet
    When a developer chooses "Write setup steps"
    Then scripts/pay-in-app.md is created with an empty numbered list
    And it opens in the editor
