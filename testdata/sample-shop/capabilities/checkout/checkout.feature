Feature: Checkout
  As a returning shopper
  I want to pay with my saved card
  So that checkout takes seconds

  Scenario: Pay with saved card
    Given a signed-in shopper with one item in the cart
    When they place the order
    Then they see "Order placed"

  @error @timeout:5m
  Scenario: Card declined
    Given a signed-in shopper with one item in the cart
    When they place the order
    Then they see "Your card was declined"
    And the order stays pending

  Scenario: 3-D Secure challenge
    Given a signed-in shopper with one item in the cart
    And their bank asks for a 3-D Secure check
    When they place the order
    Then they see the bank's challenge

  @error
  Scenario: 3-D Secure failed
    Given a signed-in shopper is shown the bank's challenge
    When they fail the challenge
    Then they see "Your card was declined"

  @error
  Scenario: Empty cart
    Given a signed-in shopper with an empty cart
    When they open checkout
    Then they see "Your cart is empty"
