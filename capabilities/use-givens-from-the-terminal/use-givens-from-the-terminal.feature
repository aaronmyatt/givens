Feature: Use Givens from the terminal
  As a developer in a terminal, or an agent working for one
  I want to list, run and check scenarios without opening the app
  So that agents and CI (automated checks on each push) can use the same files

  `list` and `run` stay plain bash, like the rest of the `givens` command. `check` needs the
  app's parser, so it runs the app headless, with no window.

  @wip
  Scenario: List scenarios
    Given a developer's terminal is in the sample shop's folder
    When they run "givens list"
    Then each scenario is printed as its path, like "checkout/card-declined", with its kind and status
    And "givens list --json" prints the same as JSON

  @wip
  Scenario: Run a setup by name
    Given a developer's terminal is anywhere inside the sample shop's folder
    When they run "givens run checkout/card-declined"
    Then the setup runs, with its "your turn" lines

  @wip
  Scenario: Check passes
    Given every file in the sample shop is valid
    When a developer runs "givens check"
    Then it prints "9 scenarios, 0 problems"
    And it exits with 0

  @wip @error
  Scenario: Check finds a problem
    Given checkout.feature has the line "Check the order is pending" among the steps of "Card declined"
    When a developer runs "givens check"
    Then it prints "checkout/checkout.feature line 17: expected a step, a tag, a comment or a new scenario"
    And it exits with 1

  @wip @error
  Scenario: Run a scenario that doesn't exist
    Given a developer's terminal is in the sample shop's folder
    When they run "givens run checkout/nope"
    Then it prints "There's no scenario checkout/nope"
    And it exits with 1

  @wip
  Scenario: List by tag
    Given a developer's terminal is in the sample shop's folder
    When they run "givens list --tags @error"
    Then it prints the same scenarios the Error states view lists

  @wip
  Scenario: Steps done by hand in a terminal
    Given "Pay in app" has only scripts/pay-in-app.md
    When a developer runs "givens run checkout/pay-in-app"
    Then it prints each step as plain text
    And it waits for Enter after each one, or f if it failed

  @wip
  Scenario: Code blocks in a terminal
    Given "Pay in app" has only scripts/pay-in-app.md
    And scripts/pay-in-app.md has a fenced code block indented under step 2
    When a developer runs "givens run checkout/pay-in-app"
    Then each code block is printed on its own lines, with no fences and no indent
    And it can be selected and pasted as it is
