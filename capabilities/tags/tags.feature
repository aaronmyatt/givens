Feature: Tags
  As a developer, or an agent, marking scenarios
  I want kinds, timeouts and groups written as Gherkin tags
  So that a test runner can filter on the same tags Givens uses

  Search and `givens list` take the same tag expressions as Cucumber runners, so a filter that
  works in one works in the other. Cucumber tag expressions

  Scenario: An error state
    Given "Card declined" is tagged @error in checkout.feature
    When a developer selects the "Error states" view
    Then "Card declined" is listed, marked as an error state

  Scenario: A tag view
    Given "Card declined" and "Pay with saved card" are both tagged @payments
    When a developer selects "payments" under Tags in the sidebar
    Then only those two scenarios are listed

  Scenario: A tag on the Feature
    Given the Feature: Checkout line is tagged @payments
    When a developer selects "payments" under Tags
    Then every Checkout scenario is listed

  Scenario: A runner filters on the same tags
    Given a developer has the sample shop project
    When they run "npx cucumber-js --dry-run --tags @error capabilities/**/*.feature"
    Then cucumber-js finds the same scenarios the Error states view lists

  Scenario: Search by tag
    Given "Card declined" and "Pay with saved card" are both tagged @payments
    When a developer presses ⌘K and types "@payments"
    Then only those two scenarios are listed under Scenarios
    And a scenario that gets @payments from its Feature line is listed too

  Scenario: Search with a tag expression
    Given "Card declined" is tagged @error and "3-D Secure failed" is tagged @error and @slow
    When a developer presses ⌘K and types "@error and not @slow"
    Then "Card declined" is listed
    And "3-D Secure failed" isn't

  Scenario: A tag and words together
    Given "Card declined" and "Pay with saved card" are both tagged @payments
    When a developer presses ⌘K and types "@payments declined"
    Then only "Card declined" is listed, with "declined" highlighted

  @error
  Scenario: A tag expression that doesn't parse
    Given a developer has the sample shop project open in Givens
    When they press ⌘K and type "@error and"
    Then they see "Not a tag expression: expected a tag after 'and'"
    And no results are listed until the expression is complete
