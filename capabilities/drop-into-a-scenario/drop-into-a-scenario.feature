Feature: Drop into a scenario
  As a developer who needs to see a behaviour for myself
  I want Givens to set up a scenario's starting state and walk me through its steps
  So that I can try it without working out how to reach that state by hand

  Drop in runs the scenario's setup script with `GIVENS_QUIET=1`, so the script's own "your
  turn" lines are left out and the scenario's When and Then steps become a checklist instead.
  Recording a run appends a line to `runs/`, which is where "Verified today" comes from.

  Before running anything, Givens reads the `need` lines in the script and the helpers it calls,
  so a missing command is caught before any state changes.

  Scenario: Drop in
    Given a developer has the sample shop project open in Givens
    And the "Card declined" setup prints a line, then waits before printing the next
    And "Card declined" is selected
    When they choose "Drop in"
    Then the setup script's output appears line by line as it runs
    And when it finishes, any manual Givens, then the When steps, are shown as a checklist
    And each Then step can be marked "Pass" or "Fail"

  @error
  Scenario: The setup script fails
    Given a developer has the sample shop project open in Givens
    And the "Card declined" setup script stops with an error part way through
    When they drop in to "Card declined"
    Then the output is shown up to the point it stopped
    And they see "Setup failed (exit 3)"
    And no checklist is shown
    And they're offered "Edit script" and "Try again"

  @error
  Scenario: A command the setup needs isn't installed
    Given a developer has the sample shop project open in Givens
    And the "Card declined" setup script needs a command called shopctl, which isn't installed
    When they drop in to "Card declined"
    Then nothing in the script runs
    And they see "This scenario needs shopctl"
    And the install hint "brew install shop/tap/shopctl" is shown with a copy button

  @error
  Scenario: Setup is still running after two minutes
    Given a developer has the sample shop project open in Givens
    And the "Card declined" setup script takes more than two minutes
    And "Card declined" isn't tagged @timeout
    When they drop in to "Card declined"
    Then after two minutes they see "Still running after 2 minutes"
    And they can choose "Keep waiting" or "Stop"
    And choosing "Stop" ends the script and keeps its output so far

  # A scenario can set its own limit with a tag, like `@timeout:5m`.
  Scenario: A longer timeout
    Given "Card declined" is tagged @timeout:5m
    And its setup takes 3 minutes
    When they drop in to "Card declined"
    Then no "Still running" message is shown
    And setup finishes as normal

  @error
  Scenario: A timeout Givens can't read
    Given "Card declined" is tagged @timeout:soon
    When they drop in to "Card declined"
    Then setup runs with the usual 2-minute timeout
    And "Card declined" is marked "@timeout must look like @timeout:90s or @timeout:5m"

  Scenario: Record a run
    Given a developer has dropped in to "Card declined" and marked every Then step "Pass"
    When they add the note "Retry button is below the fold on small screens"
    And they choose "Record run"
    Then a line for checkout/card-declined is added to today's file in runs/, with the result and their note
    And "Card declined" shows "Verified today"

  @error
  Scenario: Record a run before marking every check
    Given a developer has dropped in to "Card declined" and marked only the first Then step
    When they choose "Record run"
    Then they see "Mark each check first"
    And nothing is added to runs/

  # Each run line records who ran it, from git's `user.name`. Runs are recorded against the
  # scenario's path, so renaming a scenario starts its runs over. New capabilities folders get a
  # `.gitattributes` line, `runs/** merge=union`, which tells git to keep both sides' lines in a
  # merge.
  Scenario: A run records who ran it
    Given a developer whose git user.name is "Sam" has dropped in to "Card declined" and marked every Then step "Pass"
    When they choose "Record run"
    Then the run line includes "Sam"
    And "Card declined" shows "Verified today by Sam"

  Scenario: Runs on two branches merge cleanly
    Given Sam and Ali each recorded a run for "Card declined" today, on different branches
    When the branches are merged
    Then git merges runs/ with no conflict
    And both run lines are kept

  Scenario: Reset state part way through
    Given a developer has dropped in to "Card declined" and ticked "they place the order"
    When they choose "Reset state"
    Then the setup script runs again from the start
    And the checklist is cleared

  # Setup should be idempotent: running it twice leaves the same state as running it once.
  # GIVENS.md states this as the rule. For the cases where it can't be, a scenario can have a
  # teardown script, `scripts/<slug>.teardown.sh`, which sources the kit like any setup script.
  # Givens runs it before every setup run, so it must also cope when there's nothing to clean
  # up.
  Scenario: Running setup twice gives the same state
    Given "Card declined" has no teardown script
    And a developer has dropped in to "Card declined" once
    When they choose "Reset state"
    Then setup finishes without errors
    And the shop has one shopper "sam@example.com", not two

  Scenario: Teardown runs before setup
    Given "Card declined" has a script called card-declined.teardown.sh
    When a developer drops in, or chooses "Reset state" or "Try again"
    Then the output shows "→ teardown" followed by the teardown's output
    And then setup runs from the start

  Scenario: Add a teardown
    Given "Card declined" has no teardown script
    When a developer chooses "Add teardown"
    Then checkout/scripts/card-declined.teardown.sh is created, starting with the line that sources givens.sh
    And it opens in the editor

  @error
  Scenario: The teardown fails
    Given the "Card declined" teardown stops with an error
    When a developer drops in to "Card declined"
    Then they see "Teardown failed (exit 2)"
    And setup doesn't run
    And they're offered "Edit teardown" and "Run setup anyway"

  Scenario: A scenario that builds on another
    Given a developer has the sample shop project open in Givens
    And "Pay with saved card" has a setup that prints "✓ saved card added"
    And the "3-D Secure challenge" setup starts with "from checkout/pay-with-saved-card"
    When they drop in to "3-D Secure challenge"
    Then the output shows "→ from checkout/pay-with-saved-card" followed by that scenario's setup
    And the checklist has only the steps of "3-D Secure challenge"

  # A `from <capability>/<slug>` line runs that scenario's setup script first, with its "your
  # turn" lines hidden. The rest of the script then adds only what's different. The whole of 3-D
  # Secure challenge's setup is:
  #
  #   from checkout/pay-with-saved-card
  #   mock_payments --challenge 3ds
  #
  # Both setups run, but the checklist shows only this scenario's steps. Chains can go deeper:
  # 3-D Secure failed builds on 3-D Secure challenge, which builds on Pay with saved card.
  #
  # When a `from` scenario has a teardown, it runs just before that scenario's setup, so each
  # level cleans up its own state.
  Scenario: Each level tears down its own state
    Given the "3-D Secure challenge" setup starts with "from checkout/pay-with-saved-card"
    And both scenarios have a teardown script
    When a developer drops in to "3-D Secure challenge"
    Then the output shows 3-D Secure challenge's teardown first
    And then "→ from checkout/pay-with-saved-card", followed by Pay with saved card's teardown and setup
    And then the rest of 3-D Secure challenge's setup

  # A missing `from` scenario or a loop is caught before anything runs, the same way a missing
  # command is. The kit guards against loops too, so a script run in a terminal can't loop
  # forever.
  @error
  Scenario: Builds on a scenario that doesn't exist
    Given a developer has the sample shop project open in Givens
    And the "3-D Secure challenge" setup starts with "from checkout/pay-with-card"
    When they drop in to "3-D Secure challenge"
    Then nothing in the script runs
    And they see "from checkout/pay-with-card: there's no scenario with that name"
    And they're offered "Edit script"

  @error
  Scenario: Setups that build on each other in a loop
    Given a developer has the sample shop project open in Givens
    And "Pay with saved card" starts with "from checkout/3-d-secure-challenge"
    And "3-D Secure challenge" starts with "from checkout/pay-with-saved-card"
    When they drop in to "3-D Secure challenge"
    Then nothing in the script runs
    And they see "These setups build on each other in a loop: 3-d-secure-challenge → pay-with-saved-card → 3-d-secure-challenge"

  @error
  Scenario: A loop run from the terminal
    Given the same loop as from-loop
    When a developer runs "bash capabilities/checkout/scripts/3-d-secure-challenge.sh" from the project's root folder
    Then the kit prints "✗ from loop: 3-d-secure-challenge → pay-with-saved-card → 3-d-secure-challenge"
    And the script stops with a non-zero exit code

  # `first_you` lines are hidden by `GIVENS_QUIET=1` like the other "your turn" lines, so Givens
  # shows them at the top of the checklist instead.
  Scenario: Steps done by hand come first
    Given the "Card declined" setup ends with first_you 'select "Checkout"'
    When a developer drops in to "Card declined"
    Then the checklist starts with "First, select "Checkout""
    And the When steps come after it
    And the Then steps are still the only ones marked "Pass" or "Fail"

  # Some setup steps can't run on this Mac, like a locked-down server console, a browser page or
  # a phone. `by_hand` stops setup at that step and shows it as a card: where to do it, the
  # snippet or a link, and "Done" or "Failed". `ask` stops setup until the user types a value
  # back. Both show even with `GIVENS_QUIET=1`, since they're part of setup, not the scenario's
  # steps. The 2-minute timer pauses while a card waits. Automate a step when you can, with
  # `ssh` or a browser driver checked by `need`, and use these only when you can't.
  Scenario: A step done by hand
    Given the "Card declined" setup has a by_hand step for "DigitalOcean console" with the snippet "shopctl flags set new_checkout on"
    When a developer drops in to "Card declined"
    Then the output pauses at a card showing "DigitalOcean console" and the snippet
    And they can choose "Copy" to copy the snippet
    And choosing "Done" carries on with the rest of setup

  @error
  Scenario: A step done by hand fails
    Given a developer has dropped in to "Card declined" and setup is paused at its by_hand step
    When they choose "Failed"
    Then setup stops and they see "Setup stopped at a step done by hand"
    And no checklist is shown
    And they're offered "Edit script" and "Try again"

  Scenario: A step in the browser
    Given the "Card declined" setup has by_hand --url "http://localhost:3000/admin" for "Browser"
    When a developer drops in and setup pauses at that step
    Then the card offers "Open"
    And choosing it opens the page in their default browser

  Scenario: A step on another device
    Given the "Magic link" setup has by_hand --qr "http://192.168.1.20:3000/magic/abc" for "Your phone"
    When a developer drops in and setup pauses at that step
    Then the card shows the link as a QR code
    And the link is shown as text beneath it

  Scenario: A value from elsewhere
    Given the "Accept invite" setup has ask INVITE_CODE "Paste the invite code shown on the phone"
    When a developer drops in, types "K7Q2" into the card and chooses "Continue"
    Then the rest of setup runs with INVITE_CODE set to "K7Q2"

  @error
  Scenario: Continue with nothing typed
    Given setup is paused at an ask card
    When a developer chooses "Continue" with the field empty
    Then they see "Enter a value first"
    And setup stays paused

  Scenario: A step by hand in a scenario it builds on
    Given the "Pay with saved card" setup has a by_hand step
    And the "3-D Secure challenge" setup starts with "from checkout/pay-with-saved-card"
    When a developer drops in to "3-D Secure challenge"
    Then the card for Pay with saved card's step is shown
    And its other "your turn" lines stay hidden

  Scenario: Waiting doesn't count as slow
    Given a developer has dropped in to "Card declined" and setup is paused at its by_hand step
    When 5 minutes pass before they choose "Done"
    Then no "Still running" message is shown

  # An outline's Drop in page lists every Examples row one after another, so rows can be
  # compared. Only one row is live at a time, since the app can only be in one row's state. The
  # row's values fill the `<placeholders>` in the steps, and reach the setup script as
  # `GIVENS_EX_<column>`. Doc strings and data tables reach it as files, through
  # `GIVENS_ARG_<n>`.
  Scenario: See every row on one page
    Given checkout.feature has the outline "Card rejected by the bank"
    And a developer has the sample shop project open in Givens
    When they select "Card rejected by the bank"
    Then its Drop in page lists the "expired" and "stolen" rows one after another
    And each row shows its steps with the placeholders filled in, like "Then they see "Your card has expired""
    And each row shows its status and its own "Drop in" button

  Scenario: Drop in to one row
    Given a developer has "Card rejected by the bank" open
    When they choose "Drop in" on the "expired" row
    Then the setup script runs with GIVENS_EX_card set to "expired"
    And that row's checklist opens beneath it
    And the "stolen" row stays readable but greyed out, with its "Drop in" disabled

  Scenario: Switch to another row
    Given a developer has dropped in to the "expired" row and marked one Then step
    When they choose "Switch to this row" on the "stolen" row
    Then they're asked "Leave the expired row? Its marks aren't recorded"
    And choosing "Leave" closes the expired checklist and runs setup for "stolen"

  Scenario: Record a run for one row
    Given a developer has dropped in to "Card rejected by the bank" with the "expired" row and marked every Then step "Pass"
    When they choose "Record run"
    Then a line for checkout/card-rejected-by-the-bank/expired-and-stolen-cards is added to today's file in runs/, with card=expired
    And the scenario shows "1 of 2 rows verified"
    And the Unverified view lists the "stolen" row as "Never run"

  Scenario: Step data reaches the script
    Given the "Pay with saved card" Given step has a data table of two cart items
    When a developer drops in to "Pay with saved card"
    Then GIVENS_ARG_1 holds the path to a CSV file with a header row and both items
    And the setup script can seed the cart from it

  Scenario: Build on one row of an outline
    Given checkout.feature has the outline "Card rejected by the bank" and the scenario "Retry after rejection"
    And the "Retry after rejection" setup starts with "from checkout/card-rejected-by-the-bank card=stolen"
    When a developer drops in to "Retry after rejection"
    Then the output shows "→ from checkout/card-rejected-by-the-bank card=stolen"
    And that setup runs with GIVENS_EX_card set to "stolen"

  @error
  Scenario: Build on an outline without a row
    Given checkout.feature has the outline "Card rejected by the bank" and the scenario "Retry after rejection"
    And the "Retry after rejection" setup starts with "from checkout/card-rejected-by-the-bank"
    When a developer drops in to "Retry after rejection"
    Then nothing in the script runs
    And they see "card-rejected-by-the-bank is an outline: pick a row, like card=expired"

  @error
  Scenario: Several unnamed Examples blocks
    Given "Card rejected by the bank" has two Examples blocks with no names
    When a developer opens the sample shop in Givens
    Then the scenario is listed with the warning "Name each Examples block so runs can tell them apart"
    And "Record run" is disabled on it until they're named

  # When a scenario's setup is all done by hand, in a browser, an app or a locked-down console,
  # it lives in `scripts/<slug>.md`, not a bash file. Each top-level numbered item becomes one
  # card in Drop in. Anything before the first item is shown above the cards. Nothing runs, so
  # there's no trust prompt.
  #
  #   You'll need the TestFlight build of the shop app.
  #
  #   1. Install it on your phone: [Shop beta](https://testflight.apple.com/join/abc)
  #   2. Sign in as `sam@example.com`
  #   3. Add the test card `4242 4242 4242 4242` under Wallet
  #
  #      ![Wallet screen](../images/wallet.png)
  Scenario: Setup steps shown as cards
    Given "Pay in app" has scripts/pay-in-app.md with 3 numbered steps, and no .sh script
    When a developer drops in to "Pay in app"
    Then they aren't asked to trust the project
    And they see "You'll need the TestFlight build of the shop app" above 3 cards, with the link and the wallet screenshot shown
    And choosing "Done" on the last card shows the checklist

  Scenario: Cards after a script
    Given "Pay in app" has both scripts/pay-in-app.sh and scripts/pay-in-app.md
    When a developer drops in to "Pay in app"
    Then the script's output appears first
    And when it finishes, the cards from pay-in-app.md are shown

  @error
  Scenario: A card marked failed
    Given a developer has dropped in to "Pay in app" and marked the first card "Done"
    When they choose "Failed" on the second card
    Then they see "Setup stopped at step 2 of 3"
    And no checklist is shown
    And they're offered "Edit setup" and "Try again"

  # A card often holds something to paste elsewhere: a command for a terminal, or a snippet for
  # a browser console. Each fenced code block gets its own Copy button, and copies exactly its
  # code: no fences, no list numbers, no indent from the list it sits in. The same goes for code
  # blocks in `by_hand` text.
  #
  #   2. Turn on the new checkout in the server console:
  #
  #      ```bash
  #      sudo -u shop shopctl flags set new_checkout on
  #      ```
  #
  #   3. Clear the cart in the browser console:
  #
  #      ```js
  #      localStorage.removeItem("cart")
  #      ```
  Scenario: Copy a code block
    Given step 2 of pay-in-app.md holds a bash code block, indented under its list item
    When a developer chooses "Copy" on that code block in Drop in
    Then the clipboard holds "sudo -u shop shopctl flags set new_checkout on"
    And it holds no fences, list number or leading spaces

  Scenario: Several code blocks
    Given step 2 holds a bash code block and step 3 holds a js code block
    When a developer looks at those cards in Drop in
    Then each code block has its own Copy button, labelled "bash" and "js"
    And copying one never includes the other

  Scenario: Copy inline code
    Given a card says "Sign in as sam@example.com"
    When a developer clicks sam@example.com
    Then the clipboard holds "sam@example.com"

  @error
  Scenario: A code block that never closes
    Given step 2 of pay-in-app.md opens a code fence that never closes
    When a developer drops in to "Pay in app"
    Then step 2's card shows "The code block starting on line 4 never closes"
    And it has no Copy button until the fence is closed
