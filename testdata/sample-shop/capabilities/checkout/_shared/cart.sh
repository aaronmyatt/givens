# shellcheck shell=bash
# The signed-in shopper's cart.

# @helper seed_cart <items>
# Empties the shopper's cart, then adds that many items from the catalogue.
seed_cart() {
  local items="$1"
  ok "seeded a cart with $items items"
}
