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
