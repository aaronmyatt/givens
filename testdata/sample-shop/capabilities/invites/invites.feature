Feature: Invites
  As a shop owner
  I want to invite my team
  So that they can help run the shop

  Scenario: Accept invite
    Given a shop owner has invited sam@example.com
    When Sam opens the invite and accepts it
    Then Sam can see the shop's orders

  @error
  Scenario: Invite declined
    Given a shop owner has invited sam@example.com
    When Sam declines the invite
    Then the owner sees "Sam declined your invite"
