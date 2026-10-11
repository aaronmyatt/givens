#!/usr/bin/env bats
# The kit's own tests: each makes a small capabilities folder holding this givens.sh, runs a
# setup script with /bin/bash (3.2 on macOS), and checks what it printed and how it ended.
# The-kit's scenarios each get one test here as they're built.
# Ref: https://bats-core.readthedocs.io/en/stable/writing-tests.html
# Scripts under test are written in single quotes on purpose: they expand when they run.
# shellcheck disable=SC2016

setup() {
  root="$BATS_TEST_TMPDIR/capabilities"
  mkdir -p "$root/_shared" "$root/shop/_shared" "$root/shop/scripts"
  cp "$BATS_TEST_DIRNAME/givens.sh" "$root/_shared/givens.sh"
}

# script <name> <body>: writes shop/scripts/<name>.sh, sourcing the kit first.
script() {
  {
    printf '%s\n' 'source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"'
    printf '%s\n' "$2"
  } > "$root/shop/scripts/$1.sh"
}

@test "shared helpers load before the capability's own, which override them" {
  printf '%s\n' 'seed() { ok "shared"; }' 'only_shared() { ok "only shared"; }' \
    > "$root/_shared/a.sh"
  printf '%s\n' 'seed() { ok "shop"; }' > "$root/shop/_shared/a.sh"
  script both 'seed; only_shared'
  run /bin/bash "$root/shop/scripts/both.sh"
  [ "$status" -eq 0 ]
  [ "$output" = "$(printf '✓ shop\n✓ only shared')" ]
}

@test "GIVENS_ROOT is the capabilities folder, also for child processes" {
  script root 'bash -c "echo \$GIVENS_ROOT"'
  run /bin/bash "$root/shop/scripts/root.sh"
  [ "$status" -eq 0 ]
  [ "$output" = "$(cd "$root" && pwd)" ]
}

@test "a failing command stops setup" {
  script stops 'ok first; false; ok never'
  run /bin/bash "$root/shop/scripts/stops.sh"
  [ "$status" -ne 0 ]
  [ "$output" = "✓ first" ]
}

@test "ok, warn and fail print plain text when not in a terminal" {
  script marks 'ok "seeded shopper"; warn "Mailpit isn'"'"'t running"; fail "No database found"'
  run /bin/bash "$root/shop/scripts/marks.sh"
  [ "$status" -ne 0 ]
  [ "$output" = "$(printf "✓ seeded shopper\n! Mailpit isn't running\n✗ No database found")" ]
}

# in_terminal <answers> <script>: runs a script under a pseudo-terminal made by script(1),
# typing answers into it, so /dev/tty works as at a developer's keyboard. `command` skips
# the script helper above, which shares the name.
# Ref: https://man.freebsd.org/cgi/man.cgi?query=script&sektion=1
in_terminal() {
  # stdin stays open until the script is done: at the pipe's end, macOS script(1) types ^D,
  # which a waiting read takes as no answer at all.
  # The kit colours its output in a terminal; sed takes the colour codes back out.
  { sleep 0.3; printf '%b' "$1"; sleep 1; } | command script -q /dev/null /bin/bash "$2" |
    tr -d '\r' | sed $'s/\033\\[[0-9;]*m//g'
}

# The kit scenarios, one test each: capabilities/the-kit/the-kit.feature.

@test "Strict mode stops at the first failure" {
  script strict 'ok "seeded shopper"; ls missing-folder; ok "seeded the catalogue"'
  run /bin/bash "$root/shop/scripts/strict.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"✓ seeded shopper"* ]]
  [[ "$output" != *"seeded the catalogue"* ]]
}

@test "\"Your turn\" heading prints once" {
  script turn 'first_you x; when_you y; when_you z; then_check w'
  run /bin/bash "$root/shop/scripts/turn.sh"
  [ "$status" -eq 0 ]
  [ "$output" = "$(printf '\nYour turn\n\nFirst, x\nWhen you y\nWhen you z\nThen check that w')" ]
}

@test "Quiet mode" {
  script quiet 'ok a; first_you x; when_you y; then_check w; warn b'
  GIVENS_QUIET=1 run /bin/bash "$root/shop/scripts/quiet.sh"
  [ "$status" -eq 0 ]
  [ "$output" = "$(printf '✓ a\n! b')" ]
}

@test "No colour when not in a terminal" {
  script colour 'ok "seeded shopper"'
  [[ "$(printf '' | command script -q /dev/null /bin/bash "$root/shop/scripts/colour.sh")" == \
    *$'\033['* ]]
  /bin/bash "$root/shop/scripts/colour.sh" > "$BATS_TEST_TMPDIR/out.txt"
  [ "$(cat "$BATS_TEST_TMPDIR/out.txt")" = "✓ seeded shopper" ]
}

@test "A missing command" {
  script need 'need shopctl-not-installed "brew install shop/tap/shopctl"; ok after'
  run /bin/bash "$root/shop/scripts/need.sh"
  [ "$status" -ne 0 ]
  [ "$output" = "✗ needs shopctl-not-installed: brew install shop/tap/shopctl" ]
}

@test "fail stops setup" {
  script fails 'fail "No database found"; ok after'
  run /bin/bash "$root/shop/scripts/fails.sh"
  [ "$status" -ne 0 ]
  [ "$output" = "✗ No database found" ]
}

@test "warn carries on" {
  script warns 'warn "Mailpit is down"; ok "seeded shopper"'
  run /bin/bash "$root/shop/scripts/warns.sh"
  [ "$status" -eq 0 ]
  [ "$output" = "$(printf '! Mailpit is down\n✓ seeded shopper')" ]
}

@test "When mode" {
  script when 'if when_mode; then ok "placed the order"; exit 0; fi
ok "seeded shopper"
when_you "run this: $(when_command)"'
  cd "$BATS_TEST_TMPDIR"
  run /bin/bash capabilities/shop/scripts/when.sh
  [ "$status" -eq 0 ]
  [[ "$output" == *"When you run this: bash capabilities/shop/scripts/when.sh --when"* ]]
  run /bin/bash capabilities/shop/scripts/when.sh --when
  [ "$status" -eq 0 ]
  [ "$output" = "✓ placed the order" ]
}

@test "A step by hand in a terminal" {
  script byhand 'by_hand "DigitalOcean console" <<SNIPPET
systemctl restart shop
SNIPPET
ok "carried on"'
  output="$(in_terminal '\n' "$root/shop/scripts/byhand.sh")"
  [[ "$output" == *"By hand, in DigitalOcean console:"* ]]
  [[ "$output" == *"systemctl restart shop"* ]]
  [[ "$output" == *"Press Enter when done, or type f if it failed"* ]]
  [[ "$output" == *"✓ carried on"* ]]
}

@test "A step by hand fails in a terminal" {
  script byhandfails 'by_hand "DigitalOcean console" "restart it"; ok "carried on"'
  output="$(in_terminal 'f\n' "$root/shop/scripts/byhandfails.sh")"
  [[ "$output" == *"✗ Stopped at a step done by hand"* ]]
  [[ "$output" != *"carried on"* ]]
}

@test "Ask in a terminal" {
  script asks 'ask INVITE_CODE "Paste the invite code"; ok "code is $INVITE_CODE"'
  output="$(in_terminal 'K7Q2\n' "$root/shop/scripts/asks.sh")"
  [[ "$output" == *"Paste the invite code: "* ]]
  [[ "$output" == *"✓ code is K7Q2"* ]]
}

@test "Nobody at the keyboard" {
  script nobody 'by_hand "DigitalOcean console" "restart it"; ok "carried on"'
  # Opening /dev/tty is how the kit asks for a keyboard. With a terminal here, the Go suite,
  # which runs the script in a new session, covers this instead.
  if { : </dev/tty; } 2>/dev/null; then skip "this shell has a terminal"; fi
  run /bin/bash "$root/shop/scripts/nobody.sh" </dev/null
  [ "$status" -ne 0 ]
  [ "$output" = "✗ by_hand needs someone at the keyboard" ]
}
