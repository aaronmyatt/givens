source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
seed_shopper
ok "made a sign-in link for sam@example.com, sent 16 minutes ago"

when_you 'open the sign-in link'
then_check 'you see "This link has expired"'
