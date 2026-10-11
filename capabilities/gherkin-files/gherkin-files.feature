Feature: Gherkin files
  As a developer whose project has a Gherkin test runner
  I want each capability's scenarios in a standard .feature file
  So that the runner and Givens read the same scenarios, and neither needs a copy

  Givens parses `.feature` files with the official Cucumber Gherkin parser, so it accepts
  exactly what runners accept. cucumber/gherkin

  Scenario: A test runner reads the same files
    Given a developer has the sample shop project
    When they run "npx cucumber-js --dry-run capabilities/**/*.feature" from its root folder
    Then cucumber-js finds all 9 scenarios
    And it reports no parse errors

  Scenario: Rules group scenarios
    Given checkout.feature puts "Card declined" and "Card rejected by the bank" under Rule: The bank can refuse a card
    When a developer selects "Checkout"
    Then those two scenarios are listed under "The bank can refuse a card"
    And their run keys stay checkout/card-declined and checkout/card-rejected-by-the-bank

  Scenario: Comments are kept
    Given checkout.feature has the comment "# TODO: cover partial refunds" above "Card declined"
    When a developer edits a step of "Card declined" in Givens and saves
    Then the comment is still there
    And git diff shows only the step that changed

  Scenario: Step definitions are left alone
    Given the sample shop has checkout/steps/checkout.steps.ts
    And a folder capabilities/steps/ with no .feature file
    When a developer opens the sample shop in Givens
    Then Checkout loads as normal, with no warning about the steps file
    And "steps" isn't listed as a capability

  @error
  Scenario: A file that isn't valid Gherkin
    Given checkout.feature has the line "Check the order is pending" among the steps of "Card declined"
    When a developer opens the sample shop in Givens
    Then "Invites" and "Sign in" load as normal
    And "Checkout" is listed with a warning
    And selecting it shows "checkout.feature line 17: expected a step, a tag, a comment or a new scenario"
    And they're offered "Edit checkout.feature"

  @error
  Scenario: Two scenario names with the same slug
    Given checkout.feature has scenarios named "Card declined" and "Card declined!"
    When a developer opens the sample shop in Givens
    Then both are listed with the warning "Both names make the slug card-declined"
    And "Drop in" and "Record run" are disabled on both until one is renamed

  @error
  Scenario: Two .feature files in one capability
    Given the checkout folder holds checkout.feature and refunds.feature
    When a developer opens the sample shop in Givens
    Then Checkout loads from checkout.feature
    And it shows the warning "refunds.feature isn't read: a capability has one .feature file, named after its folder"
    And they're offered "Move to its own capability"
