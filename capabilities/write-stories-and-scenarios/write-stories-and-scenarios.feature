Feature: Write stories and scenarios
  As a developer, or an agent working for one
  I want to write stories, scenarios and scripts with Givens and have them saved as plain files
  So that anyone can read and change them with whatever tool they like

  Text is edited in the developer's own editor. Givens holds the file open to edit, checks
  it, and writes back the real .feature file, tags and comments and all: `givens edit`, and e
  or s in the TUI, hand that text to `$VISUAL` or `$EDITOR` and take it back when it closes.
  See work-in-the-terminal. Saving writes back only what changed, so the git diff of an edit
  made through Givens looks like one made by hand. Givens also suggests steps already in the
  project, which keeps wording identical and step search useful.

  # `givens new story` asks for these lines in a template in the developer's editor.
  Scenario: Write a new story
    Given a developer has the sample shop project open in Givens
    When they start a new story and fill in "Refunds", "a shop owner", "to refund an order" and "the shopper gets their money back without emailing support"
    Then refunds/refunds.feature is created, with "Feature: Refunds" and those lines as its description
    And "Refunds" is listed among the capabilities

  # `givens new scenario <capability>` asks for the name, and whether it's an error state, in
  # a template in the developer's editor; n on a scenario in the TUI asks for the name.
  Scenario: Add a scenario to a capability
    Given a developer has the sample shop project open in Givens
    When they add a scenario to Checkout named "Expired card", marked as an error state
    Then a "Scenario: Expired card" block tagged @error is added to checkout.feature
    And checkout/scripts/expired-card.sh is created, starting with the line that sources givens.sh
    And checkout.feature is left open to edit at the new scenario

  # A new scenario isn't built yet, so it starts flagged to do. The developer removes the flag
  # once it works, with `givens done`.
  Scenario: A new scenario starts flagged to do
    Given a developer has the sample shop project open in Givens
    When they add a scenario to Checkout named "Gift card"
    Then a "Scenario: Gift card" block tagged @todo is added to checkout.feature
    And "Gift card" is shown flagged to do

  # Givens suggests steps already in the project as a step is typed into the file open to
  # edit, most used first. The TUI and `givens edit` leave typing to the developer's editor,
  # so they don't show the suggestions.
  Scenario: Reuse an existing step
    Given a developer is writing a new scenario in Checkout
    When they type "Given a signed-in sh"
    Then the steps suggested include "a signed-in shopper with one item in the cart" with "used in 4"
    And "a signed-in shopper with an empty cart" with "used in 1"
    And choosing one puts it in word for word

  Scenario: Saving changes only what was changed
    Given a developer has the sample shop project in git and open in Givens
    When they change "the order stays pending" to "the order is marked failed" in "Card declined"
    And they save
    Then git diff shows that one line changed and nothing else

  # Givens offers three ways out. `givens edit`, and e in the TUI, write nothing and keep the
  # edit in a file whose path they print; see work-in-the-terminal.
  @error
  Scenario: The file changes on disk while it's being edited
    Given a developer has unsaved changes to "Card declined"
    When another tool saves a different change to checkout.feature
    Then they see "This file changed on disk"
    And they can choose "Keep mine", "Use theirs" or "Compare"

  # Saved as it is, a line the parser can't read is marked. `givens edit`, and e in the TUI,
  # check first: they open the developer's editor again with the problem at the top; see
  # work-in-the-terminal.
  @error
  Scenario: A line that isn't a step
    Given a developer is editing "Card declined"
    When they add the line "Check the order is pending" among its steps
    And they save
    Then that line is marked "Expected a step, a tag, a comment or a new scenario"
    And the file is saved anyway

  # `givens rename <key> <new name>` renames a scenario or a capability.
  Scenario: Rename a scenario
    Given a developer has the sample shop project open in Givens
    When they rename "Pay with saved card" to "Pay with a saved card"
    Then its Scenario: line is updated, and its setup script is renamed to pay-with-a-saved-card.sh
    And the from line in 3-D Secure challenge's setup is updated to match
    And it shows "Never run", since its runs were recorded under the old name

  Scenario: Rename a capability
    Given a developer has the sample shop project open in Givens
    When they rename "Sign in" to "Log in"
    Then "Log in" is listed among the capabilities in place of "Sign in"
    And its folder and .feature file are renamed to log-in and log-in.feature, with its scripts and helpers inside
    And its scenarios show "Never run"

  Scenario: Renaming a capability updates from lines
    Given a developer has the sample shop project open in Givens
    And the "Accept invite" setup starts with "from sign-in/magic-link"
    When they rename "Sign in" to "Log in"
    Then that line becomes "from log-in/magic-link"
    And "Magic link" and "Expired link" show "Never run"

  # Runs are recorded against a scenario's path, so renaming a scenario or its capability starts
  # its runs over. Old lines stay in `runs/` as history; Givens never rewrites them.
  # `givens rename` prints the warning, and renames only when it's run again with --yes.
  Scenario: Warned before a rename
    Given "Card declined" has 3 runs recorded
    When a developer renames it to "Card refused"
    Then they see "Renaming starts Card declined's runs over"
    And they're offered "Rename" and "Cancel"

  Scenario: A renamed scenario starts at Never run
    Given "Card declined" has a passing run recorded today
    When a developer renames it to "Card refused" and confirms
    Then "Card refused" shows "Never run"
    And it's listed in the Unverified view
    And nothing in runs/ changes

  Scenario: Scenarios built on it are unverified too
    Given "3-D Secure challenge" has a passing run recorded
    When a developer renames "Pay with saved card" to "Pay with a saved card" and confirms
    Then the from line in 3-D Secure challenge's setup is updated
    And "3-D Secure challenge" shows "Unverified: setup changed since the last run"

  # Deleted scripts and folders go to the Trash, so they can be brought back from it. A
  # deleted scenario's block comes back with git. `givens delete <key>` prints what Givens
  # asks first, and deletes only when it's run again with --yes.
  Scenario: Delete a scenario
    Given a developer has the sample shop project open in Givens
    When they delete "Empty cart"
    And they confirm
    Then its block is removed from checkout.feature, and its setup script is moved to the Trash
    And Checkout shows 4 scenarios

  @error
  Scenario: Delete a scenario another builds on
    Given a developer has the sample shop project open in Givens
    When they delete "Pay with saved card"
    Then they see "3-D Secure challenge builds on this scenario"
    And nothing is deleted unless they choose "Delete anyway"

  Scenario: Delete a capability
    Given a developer has the sample shop project open in Givens
    When they delete "Invites"
    And they confirm
    Then the invites folder is moved to the Trash
    And only "Checkout" and "Sign in" are listed among the capabilities

  @error
  Scenario: Delete a capability another builds on
    Given a developer has the sample shop project open in Givens
    And the "Accept invite" setup starts with "from sign-in/magic-link"
    When they delete "Sign in"
    Then they see "Accept invite builds on a scenario in Sign in"
    And nothing is deleted unless they choose "Delete anyway"

  # Givens previews the file open to edit, unsaved changes included, as it reads once saved.
  # The TUI and `givens edit` don't show it: the developer's editor shows the text.
  Scenario: Preview a scenario while editing
    Given a developer is editing "Card declined"
    When they choose "Preview"
    Then the preview shows the scenario with its keywords lined up
    And their unsaved changes are included

  # e on a scenario in the TUI, like `givens edit <key>`, opens its .feature file in the
  # developer's editor at its block. With nothing else open, Givens opens the story on show at
  # the top of its .feature file the same way; in the TUI, e on a story edits its about.md.
  Scenario: Press e to edit a scenario
    Given a developer has the sample shop project open in Givens
    And "Card declined" is selected
    When they press e
    Then checkout/checkout.feature opens to edit at the "Card declined" block

  Scenario: Edit the story on show
    Given a developer has the sample shop project open in Givens
    And "Checkout" is selected
    When they edit what's on show
    Then checkout/checkout.feature opens to edit

  # A capability's about.md holds diagrams and context below its story. Givens offers to edit
  # it, or to start one when there's none: e on a story in the TUI does whichever is offered.
  Scenario: Edit a capability's about.md
    Given a developer has the sample shop project open in Givens
    And "Checkout" is selected
    When they choose "Edit about.md"
    Then checkout/about.md opens to edit

  Scenario: Add an about.md
    Given Invites has no about.md
    And "Invites" is selected
    When they choose "Add about.md"
    Then invites/about.md is created, starting with the line "# Invites"
    And it opens to edit

  Scenario: Preview a story with an image
    Given a developer is editing Checkout's about.md
    When they choose "Preview"
    Then the checkout flow diagram is shown as Checkout's story shows it

  @error
  Scenario: Preview a line that isn't a step
    Given a developer is editing "Card declined"
    And one line among its steps doesn't start with Given, When, Then, And, But or *
    When they choose "Preview"
    Then that line is shown marked with the parser's message
    And the steps around it are shown as normal

  # A scenario with no setup offers a script or setup steps done by hand. s on a scenario in
  # the TUI creates the script; setup steps are started here.
  Scenario: Write setup steps
    Given "Pay in app" has no setup yet
    When a developer chooses "Write setup steps"
    Then scripts/pay-in-app.md is created with an empty numbered list
    And it opens to edit

  # A setup file open to edit can be deleted, like setup steps started by mistake. It goes to
  # the Trash, so it can be brought back from it. s on a scenario in the TUI opens its setup
  # in the developer's editor, as "Edit script" or "Edit setup" does here.
  Scenario: Delete a setup file while editing it
    Given "Pay in app" has only scripts/pay-in-app.md
    And a developer has chosen "Edit setup" on "Pay in app"
    When they choose "Delete file"
    And they confirm
    Then pay-in-app.md is moved to the Trash
    And nothing is left open to edit
    And they're offered "Create script" and "Write setup steps"

  # A setup script another scenario runs with a from line gets the same warning as deleting
  # the scenario does.
  @error
  Scenario: Delete a setup script another builds on
    Given a developer has the sample shop project open in Givens
    And a developer has chosen "Edit script" on "Pay with saved card"
    When they choose "Delete file"
    Then they see "3-D Secure challenge builds on this scenario"
    And nothing is deleted unless they choose "Delete anyway"
