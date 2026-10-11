source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
seed_shopper
seed_catalogue
seed_cart 1
mock_payments

when_you 'place the order'
then_check 'you see "Order placed"'
