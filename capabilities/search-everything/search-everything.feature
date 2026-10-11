Feature: Search everything
  As a developer looking for where a behaviour is described
  I want to search stories, scenarios, steps, scripts and helpers from one place
  So that I find every scenario that touches it, not just the first file that mentions it

  Search opens with ⌘K. Results are grouped by kind, and steps are searchable on their own: a
  step used word for word in several scenarios appears once, with its number of uses. ↵ opens a
  result, ⇧⌘C copies its setup command, ⌘↵ drops in and ⌘E edits it.

  Scenario: Search for a phrase
    Given a developer has the sample shop project open in Givens
    When they press ⌘K and type "declined"
    Then the results are grouped under Scenarios, Steps and Scripts
    And "Card declined" and "Invite declined" are listed under Scenarios
    And every match is highlighted

  Scenario: Find a capability by its story
    Given a developer has the sample shop project open in Givens
    When they press ⌘K and type "saved card"
    Then "Checkout" is listed under Stories, matching "I want to pay with my saved card"
    And "Pay with saved card" is listed under Scenarios

  Scenario: Find every use of a step
    Given a developer has the sample shop project open in Givens
    When they press ⌘K and type "your card was declined"
    Then the step "Then they see "Your card was declined"" is listed with "2 uses"
    And selecting it lists "Card declined" and "3-D Secure failed"

  Scenario: Show only one kind of result
    Given a developer has searched for "declined"
    When they choose the "Scripts" filter
    Then only scripts are listed
    And scripts with "declined" in their name come before scripts that only mention it

  Scenario: Find a helper
    Given a developer has the sample shop project open in Givens
    When they press ⌘K and type "seed_shopper"
    Then seed_shopper is listed under Helpers with its description
    And selecting it shows its signature and the 5 scripts that call it

  @error
  Scenario: Nothing matches
    Given a developer has the sample shop project open in Givens
    When they press ⌘K and type "gift card"
    Then they see "Nothing matches "gift card""
    And they're offered "New scenario"
