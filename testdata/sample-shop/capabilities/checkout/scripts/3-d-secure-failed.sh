source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
from checkout/3-d-secure-challenge

first_you "place the order, so the bank's challenge is shown"
when_you 'fail the challenge'
then_check 'you see "Your card was declined"'
