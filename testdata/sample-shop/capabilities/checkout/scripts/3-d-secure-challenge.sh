source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
from checkout/pay-with-saved-card
mock_payments --challenge 3ds

when_you 'place the order'
then_check "you see the bank's challenge"
