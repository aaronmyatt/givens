source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
seed_owner
seed_invite sam@example.com

when_you 'decline the invite as Sam'
then_check 'the owner sees "Sam declined your invite"'
