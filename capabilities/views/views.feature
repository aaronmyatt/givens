Feature: Views
  As a developer looking across a whole project
  I want views that collect scenarios and helpers from every capability
  So that I can find every error state, or everything that needs checking again, without opening each capability

  The views are Error states, Unverified and Shared, plus one view per tag. In the TUI, the
  Views tab lists the first three and the Tags tab the tags, each with how many it holds;
  Enter on Error states, Unverified or a tag lists its scenarios. The Helpers tab lists
  Shared's helpers, and `givens helpers` prints them. Views are worked out from the same files
  as everything else, so there's nothing extra to keep up to date.

  Scenario: See every error state
    Given a developer has the sample shop project open in Givens
    When they select the "Error states" view
    Then only error states are listed, from every capability
    And each one shows which capability it belongs to

  Scenario: See what needs checking again
    Given a developer has the sample shop project open in Givens
    And "Card declined" has a passing run recorded
    And "Empty cart" has a failing run recorded
    When they select the "Unverified" view
    Then "Card declined" isn't listed
    And "Empty cart" is listed with "Last run failed"
    And the other 7 scenarios are listed with "Never run"

  # A scenario counts as verified while its latest run passed and its setup hasn't changed
  # since. "Changing a helper unverifies the scenarios that call it" covers the second half.
  Scenario: Browse helpers
    Given a developer has the sample shop project open in Givens
    When they select the "Shared" view
    Then every documented helper is listed with its signature and description
    And each one shows the file it's defined in and how many scripts call it

  @error
  Scenario: A view with nothing in it
    Given a developer has a project open in Givens whose scenarios are all happy paths
    When they select the "Error states" view
    Then they see "No error states yet"
    And they're offered "New scenario"
