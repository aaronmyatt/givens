source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
seed_shopper
seed_cart 1
mock_payments --decline

when_you 'place the order'
then_check 'you see "Your card was declined"'
then_check 'the order stays pending'
