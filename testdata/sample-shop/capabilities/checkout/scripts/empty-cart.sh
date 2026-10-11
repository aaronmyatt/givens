source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
seed_shopper
seed_cart 0

when_you 'open checkout'
then_check 'you see "Your cart is empty"'
