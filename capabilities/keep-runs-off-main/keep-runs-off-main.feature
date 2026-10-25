Feature: Keep runs off main
  As a developer sharing runs across my devices and with my team
  I want runs kept in a git ref of their own, not on a branch
  So that main's history and pull requests hold code and specs, not run logs and screenshots

  In a git repository with an origin remote, Givens keeps runs off main by default: in
  refs/givens/runs on the remote, which it fetches and pushes itself. A plain git clone
  doesn't fetch it, and it's never in a pull request. Without a remote, runs go in
  capabilities/runs. `givens sync` shares runs waiting to go and fetches other devices'.

  Scenario: Runs are kept off main by default
    Given the sample shop is a git repository with a remote
    When a developer records a passing run of "Empty cart"
    Then the remote's refs/givens/runs holds the run of "Empty cart"
    And git status shows nothing changed

  Scenario: Without a remote, runs stay in capabilities/runs
    Given the sample shop is a git repository without a remote
    When a developer records a passing run of "Empty cart"
    Then capabilities/runs holds the run of "Empty cart"

  # Runs recorded before the project had a remote stay in capabilities/runs, and still count.
  # Moving them sends them to the Trash and leaves off-main.md, which says where runs are.
  Scenario: Move runs off main
    Given "Card declined" has a passing run recorded in capabilities/runs
    And the sample shop is a git repository with a remote
    When a developer chooses "Move runs off main" and confirms
    Then the remote's refs/givens/runs holds the run of "Card declined"
    And capabilities/runs holds only off-main.md
    And "Card declined" shows "Verified today"

  Scenario: Runs show up on another device
    Given the sample shop is a git repository with a remote, with a passing run of "Card declined"
    When a developer clones it on another device and opens it in Givens
    Then "Card declined" shows "Verified today"

  # Runs aren't kept per branch, so another branch, opened to read, reads the same ref.
  Scenario: Another branch shows runs kept off main
    Given the sample shop is a git repository with a remote, with a passing run of "Card declined"
    And it has a branch "gift-cards"
    When they open the branch "gift-cards"
    Then "Card declined" shows "Verified today"

  # Opening a project syncs its runs in the background, so a remote that doesn't answer
  # doesn't hold the project up. The sync gives up after 8 seconds; the next one tries again.
  @error
  Scenario: Open a project whose remote doesn't answer
    Given the sample shop is a git repository with a remote that doesn't answer
    When a developer opens it in Givens
    Then it opens within two seconds, showing "Syncing runs with origin"
    And once the sync gives up, they don't see "Syncing runs with origin"

  Scenario: Two devices record at the same time
    Given the sample shop is a git repository with a remote, cloned on another device
    And a passing run of "Card declined" is recorded on the other device
    When a developer records a passing run of "Empty cart"
    Then the remote's refs/givens/runs holds the runs of "Card declined" and "Empty cart"

  @error
  Scenario: Record a run while the remote can't be reached
    Given the sample shop is a git repository with a remote
    And its remote can't be reached
    When a developer records a passing run of "Empty cart"
    Then they see "1 run not shared yet"
    And they're offered "Share runs"

  Scenario: Share runs once the remote is back
    Given the sample shop is a git repository with a remote
    And a passing run of "Empty cart" was recorded while its remote couldn't be reached
    When the remote is back and a developer chooses "Share runs"
    Then the remote's refs/givens/runs holds the run of "Empty cart"
    And they don't see "1 run not shared yet"

  # The ref keeps the newest runs only, so it can't grow without end: 1000 runs or 250 MB by
  # default, whichever comes first, set in the ref's limits file. Past either, the oldest go
  # until both are at 80%, so it isn't trimmed on every run. Each scenario's newest run is
  # always kept, so none goes back to "Never run".
  Scenario: Old runs are dropped
    Given the sample shop is a git repository with a remote, with a limit of 5 runs
    And "Empty cart" has 5 runs recorded, each with a screenshot
    When a developer records a passing run of "Empty cart"
    Then the remote's refs/givens/runs holds the newest 4 runs of "Empty cart"
    And its history holds none of the older runs' screenshots

  # The limits are in the ref's limits file, so every device trims the same way. Lower limits
  # trim the ref at once, down to 80% as above.
  Scenario: Change the limits
    Given the sample shop is a git repository with a remote
    And "Empty cart" has 5 runs recorded, each with a screenshot
    When a developer sets the run limits to 3 runs and 100 MB
    Then the remote's refs/givens/runs has a limit of 3 runs and 100 MB
    And it holds 2 runs of "Empty cart"
