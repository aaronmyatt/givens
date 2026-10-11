source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
seed_owner
seed_invite sam@example.com

when_you 'open the invite as Sam and accept it'
then_check "Sam can see the shop's orders"
