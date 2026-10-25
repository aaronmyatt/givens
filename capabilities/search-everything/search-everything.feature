Feature: Search everything
  As a developer looking for where a behaviour is described
  I want to search stories, scenarios, steps, scripts and helpers from one place
  So that I find every scenario that touches it, not just the first file that mentions it

  In the TUI, "/" opens search: typing narrows the results, ↑ and ↓ move, Enter on a scenario
  shows it in the list, and Esc closes. `givens search <words>` prints the same results, one
  per line, and `--kind Scripts` keeps one kind. Results are grouped by kind, and steps are
  searchable on their own: a step used word for word in several scenarios appears once, with
  its number of uses.

  Scenario: Search for a phrase
    Given a developer has the sample shop project open in Givens
    When they search for "declined"
    Then the results are grouped under Scenarios, Steps and Scripts
    And "Card declined" and "Invite declined" are listed under Scenarios
    And every match is marked in its result

  Scenario: Find a capability by its story
    Given a developer has the sample shop project open in Givens
    When they search for "saved card"
    Then "Checkout" is listed under Stories, matching "I want to pay with my saved card"
    And "Pay with saved card" is listed under Scenarios

  Scenario: Find every use of a step
    Given a developer has the sample shop project open in Givens
    When they search for "your card was declined"
    Then the step "Then they see "Your card was declined"" is listed with "2 uses"
    And selecting it lists "Card declined" and "3-D Secure failed"

  Scenario: Show only one kind of result
    Given a developer has searched for "declined"
    When they narrow the results to "Scripts"
    Then only scripts are listed
    And scripts with "declined" in their name come before scripts that only mention it

  Scenario: Find a helper
    Given a developer has the sample shop project open in Givens
    When they search for "seed_shopper"
    Then seed_shopper is listed under Helpers with its description
    And selecting it shows its signature and the 5 scripts that call it

  @error
  Scenario: Nothing matches
    Given a developer has the sample shop project open in Givens
    When they search for "gift card"
    Then they see "Nothing matches "gift card""
    And they're offered "New scenario"
