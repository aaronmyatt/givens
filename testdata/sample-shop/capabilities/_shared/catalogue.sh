# shellcheck shell=bash
# The shop's products. Babashka reads the catalogue as Clojure data.
# Ref: https://book.babashka.org/

# @helper seed_catalogue
# Seeds the products in _shared/catalogue.clj. Needs bb.
seed_catalogue() {
  need bb "brew install borkdude/brew/babashka"
  local products
  products="$(bb "$GIVENS_ROOT/_shared/catalogue.clj")"
  ok "seeded the catalogue with $products products"
}
