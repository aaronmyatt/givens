Feature: Update the kit
  As a developer on a project started with an older Givens
  I want to update the kit from inside Givens
  So that I get fixes without copying files by hand

  givens.sh starts with a version line, for example `# givens kit 2`. Updating the kit doesn't
  unverify any scenario.

  @wip
  Scenario: An update is available
    Given the sample shop's givens.sh is version 1
    And Givens ships version 2
    When a developer opens the sample shop
    Then they see "Kit update available (1 → 2)"
    And they're offered "Update kit"

  @wip
  Scenario: Update the kit
    Given the sample shop's givens.sh is version 1 and hasn't been edited
    When a developer chooses "Update kit"
    Then givens.sh is replaced with version 2
    And git diff shows only givens.sh changed

  @wip @error
  Scenario: The kit was edited by hand
    Given the sample shop's givens.sh has local edits
    When a developer chooses "Update kit"
    Then they see "givens.sh has changes of its own"
    And they're offered "Compare" and "Replace anyway"

  @wip @error
  Scenario: The project's kit is newer than Givens
    Given the sample shop's givens.sh is version 3
    And Givens ships version 2
    When a developer opens the sample shop
    Then they see "This project uses a newer kit. Update Givens to edit its scripts."
    And reading, searching and copying still work
