Feature: Update the kit
  As a developer on a project started with an older Givens
  I want Givens to update the kit for me
  So that I get fixes without copying files by hand

  givens.sh starts with a version line, for example `# givens kit 2`. Updating the kit doesn't
  unverify any scenario. What Givens says about the kit shows under the TUI's list.

  Scenario: An update is available
    Given the sample shop's givens.sh is version 1
    And Givens ships version 2
    When a developer opens the sample shop
    Then they see "Kit update available (1 → 2)"
    And they're offered "Update kit"

  Scenario: Update the kit
    Given the sample shop's givens.sh is version 1 and hasn't been edited
    When a developer chooses "Update kit"
    Then givens.sh is replaced with version 2
    And git diff shows only givens.sh changed

  # Every setup script's first line sources _shared/givens.sh, so a capabilities folder without
  # it can't run anything. Givens writes the kit it ships whenever it finds the file missing:
  # when a project opens, and when the file goes while the project is open.
  @error
  Scenario: A missing kit is put back
    Given the sample shop has no givens.sh
    And Givens ships version 2
    When a developer opens the sample shop
    Then givens.sh is written at version 2
    And deleting givens.sh while the sample shop is open puts version 2 back

  @error
  Scenario: The kit was edited by hand
    Given the sample shop's givens.sh has local edits
    When a developer chooses "Update kit"
    Then they see "givens.sh has changes of its own"
    And they're offered "Compare" and "Replace anyway"

  @error
  Scenario: The project's kit is newer than Givens
    Given the sample shop's givens.sh is version 3
    And Givens ships version 2
    When a developer opens the sample shop
    Then they see "This project uses a newer kit. Update Givens to edit its scripts."
    And reading, searching and copying still work
