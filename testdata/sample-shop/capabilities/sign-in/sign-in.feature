Feature: Sign in
  As a shopper
  I want to sign in with a link sent to my email
  So that I don't need a password

  Scenario: Magic link
    Given a shopper has an account
    When they ask for a sign-in link and open it
    Then they're signed in

  @error
  Scenario: Expired link
    Given a shopper has a sign-in link older than 15 minutes
    When they open it
    Then they see "This link has expired"
