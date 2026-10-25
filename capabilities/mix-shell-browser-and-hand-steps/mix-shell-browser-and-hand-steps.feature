Feature: Mix shell, browser and hand steps
  As a developer whose app runs in a browser
  I want one scenario's setup to mix shell commands, JavaScript run in the browser and steps done by hand
  So that I can reach any starting state, then carry on in that same browser myself

  The kit drives a browser through the Chrome DevTools Protocol (CDP: the WebSocket protocol
  Playwright and Puppeteer use to drive Chrome). The givens command does the talking, with
  chromedp, a Go CDP client; the kit's browser functions call it. See about.md in The kit.

  `browser_open <url>` opens a page in the Givens browser: Chrome, started with a profile of
  Givens's own for this project, so the developer's own Chrome and its sign-ins are left alone.
  `in_browser <js>` runs JavaScript in that page, waits for a promise to settle, and prints what
  it returns. `app_action <name> [json]` calls `window.app.<name>`, which the app sets in its dev
  builds (the "app actions" way of driving an app through its own functions, not its UI). The
  Givens browser stays open after setup, so the developer does the When steps in it.

  A setup card in `scripts/<slug>.md` whose code block's info string is `bash run` or `js run`
  runs by itself. Cards with no such block wait for the developer, as before.

  # In a setup script. Shell lines, browser lines and by_hand steps run in the order written.

  Scenario: Open a page from setup
    Given the "Card declined" setup has browser_open "http://localhost:3000/cart"
    When a developer drops in to "Card declined"
    Then the output shows "✓ opened http://localhost:3000/cart"
    And the Givens browser shows the shop's cart page

  Scenario: Shell, browser and hand steps in one script
    Given the "Card declined" setup seeds a shopper with a shell command
    And then signs in with app_action signIn '{"email":"sam@example.com"}'
    And then has a by_hand step for "Phone" with the text "Approve the sign-in"
    When a developer drops in to "Card declined"
    Then the output shows the seed line, then the app action's line, then the by_hand card
    And choosing "Done" on the card shows the checklist

  Scenario: An app action that returns a promise
    Given the shop's window.app.addToCart returns a promise that settles after 2 seconds
    And the "Card declined" setup has app_action addToCart '{"sku":"tee"}', then ok "cart filled"
    When a developer drops in to "Card declined"
    Then "✓ cart filled" is printed only after the promise settles

  Scenario: Use what the JavaScript returns
    Given the shop's page returns "ord_42" from window.app.lastOrderId()
    And the "Card declined" setup has ORDER="$(in_browser 'window.app.lastOrderId()')"
    When a developer drops in to "Card declined"
    Then the rest of setup sees ORDER as "ord_42"

  @error
  Scenario: The JavaScript throws
    Given the "Card declined" setup has in_browser 'window.app.nope()'
    When a developer drops in to "Card declined"
    Then the output shows "✗ in_browser: window.app.nope is not a function"
    And they see "Setup failed (exit 1)"
    And no checklist is shown

  @error
  Scenario: The page has no such app action
    Given the shop's page has no window.app.refund
    And the "Card declined" setup has app_action refund '{"order":"ord_42"}'
    When a developer drops in to "Card declined"
    Then the output shows "✗ app_action refund: the page has no window.app.refund"
    And no checklist is shown

  @error
  Scenario: JavaScript that never answers
    Given the "Card declined" setup has in_browser with a promise that never settles
    When a developer drops in to "Card declined"
    Then after 30 seconds the output shows "✗ in_browser: no answer after 30s"
    And no checklist is shown

  # Givens reads the browser lines before running anything, as it reads need lines, so a
  # missing Chrome is caught before any state changes.
  @error
  Scenario: No Chrome installed
    Given the "Card declined" setup has browser_open "http://localhost:3000/cart"
    And Chrome isn't installed
    When a developer drops in to "Card declined"
    Then nothing in the script runs
    And they see "This scenario needs Chrome"
    And the install hint "brew install --cask google-chrome" is shown, and Copy copies it

  # The Givens browser outlives setup, so the developer takes over where setup left off.

  Scenario: Carry on in the same browser
    Given the "Card declined" setup signs in as "sam@example.com" with app_action
    When a developer drops in to "Card declined" and setup finishes
    Then the Givens browser stays open on the shop, signed in as "sam@example.com"
    And the developer's own Chrome profile is left as it was

  Scenario: Reset state in the same browser
    Given a developer has dropped in to "Card declined" and the Givens browser is open
    When they choose "Reset state"
    Then setup runs again in the same Givens browser window
    And no second browser window opens

  Scenario: A step by hand in the Givens browser
    Given the "Card declined" setup opens the Givens browser
    And then has by_hand --url "http://localhost:3000/admin" for "Browser"
    When a developer drops in and chooses "Open" on the card
    Then the admin page opens in the Givens browser, not the default browser

  # In setup cards. A card's `bash run` block runs with the kit loaded, so ok, from and the
  # browser functions work in it. A `js run` block runs in the Givens browser.

  Scenario: A card that runs a shell command
    Given step 1 of pay-in-app.md has a "bash run" block that calls ok "seeded shopper"
    And step 2 has no run block
    When a developer drops in to "Pay in app"
    Then step 1 runs by itself and its card shows "✓ seeded shopper"
    And step 1 is marked done when its block exits with 0
    And step 2 waits for "Done"

  Scenario: A card that runs JavaScript
    Given step 1 of pay-in-app.md has a "bash run" block with browser_open "http://localhost:3000/cart"
    And step 2 has a "js run" block holding window.app.lastOrderId()
    When a developer drops in to "Pay in app"
    Then step 2 runs in the Givens browser by itself
    And its card shows "ord_42"

  @error
  Scenario: A card's run block fails
    Given step 1 of pay-in-app.md has a "bash run" block that exits with 2
    When a developer drops in to "Pay in app"
    Then they see "Setup stopped at step 1 of 3 (exit 2)"
    And step 1's card shows the block's output so far
    And they're offered "Edit setup" and "Try again"

  # Cards with no run blocks run nothing, so they skip the trust prompt. A run block is code.
  Scenario: Cards that run code ask for trust
    Given a developer has never run anything from the sample shop
    And step 1 of pay-in-app.md has a "bash run" block
    When they drop in to "Pay in app"
    Then they see "Run scripts from this project?" before any card runs

  # Outside a Drop in. The same files act the same run straight from a terminal, or by
  # `givens run`, which prints the cards and waits at the ones done by hand.

  Scenario: Browser lines in a terminal
    Given the "Card declined" setup has browser_open and app_action lines
    When a developer runs "bash capabilities/checkout/scripts/card-declined.sh" in a terminal
    Then the Givens browser opens and the app action runs, as in Drop in
    And the browser stays open after the script exits

  Scenario: Run cards in a terminal
    Given step 1 of pay-in-app.md has a "bash run" block and step 2 has none
    When a developer runs "givens run checkout/pay-in-app"
    Then step 1 runs and prints its output
    And step 2 is printed as plain text and waits for Enter, or f if it failed

  @error
  Scenario: The givens command isn't installed
    Given the "Card declined" setup has browser_open "http://localhost:3000/cart"
    And the givens command isn't on the PATH
    When a developer runs the script in a terminal
    Then it prints "✗ needs givens: run scripts/install.sh in a Givens checkout"
    And nothing after that line runs

  Scenario: A terminal run and Drop in share the browser
    Given a developer ran the "Card declined" setup in a terminal and the Givens browser is open
    When they drop in to "Card declined"
    Then setup uses that same Givens browser window
    And no second browser window opens
