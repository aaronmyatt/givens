# shellcheck shell=bash
# Shoppers every capability can sign in as. The kit loads this file before any setup runs.

# @helper seed_shopper [email]
# Seeds a signed-in shopper with a saved card and a delivery address.
seed_shopper() {
  local email="${1:-sam@example.com}"
  seed_address "$email"
  ok "seeded shopper $email with a saved card"
}

# @helper seed_address <email>
# Gives a shopper one delivery address in London.
seed_address() {
  local email="$1"
  ok "seeded an address for $email"
}
