#!/usr/bin/env zsh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../../.." && pwd)"
PROJECT="$ROOT_DIR/ios/TagNote/TagNote.xcodeproj"
SCHEME="TagNote"
MODE="${1:-unit}"

case "$MODE" in
  unit)
    ONLY_TESTING="TagNoteTests"
    ;;
  e2e|ui)
    ONLY_TESTING="TagNoteUITests"
    ;;
  all)
    ONLY_TESTING=""
    ;;
  *)
    echo "usage: $0 [unit|e2e|all]" >&2
    exit 64
    ;;
esac

# Print the name of the first available simulator whose line matches the given
# awk pattern (e.g. "iPhone" or "iPad"), or nothing if none is found.
first_sim_named() {
  local pattern="$1" devices="$2"
  printf '%s\n' "$devices" | awk -v pat="$pattern" '
    $0 ~ ("^[[:space:]]+" pat) {
      line=$0
      sub(/^[[:space:]]+/, "", line)
      sub(/[[:space:]]+\([0-9A-Fa-f-]+\).*/, "", line)
      print line
      exit
    }
  '
}

# Echoes one destination per line. For e2e/ui mode it returns both an iPhone
# (compact layout) and an iPad (persistent-sidebar layout) so the adaptive UI is
# covered on both. Other modes use a single iPhone.
pick_destinations() {
  if [[ -n "${IOS_TEST_DESTINATION:-}" ]]; then
    echo "$IOS_TEST_DESTINATION"
    return 0
  fi

  local devices
  if ! devices="$(xcrun simctl list devices available 2>/tmp/tagnote-simctl.err)"; then
    echo "CoreSimulator is not available in this session." >&2
    cat /tmp/tagnote-simctl.err >&2 || true
    echo >&2
    echo "Open Xcode once, install an iOS Simulator runtime, or run this from a GUI macOS session." >&2
    exit 69
  fi

  local iphone
  iphone="$(first_sim_named iPhone "$devices")"
  if [[ -z "$iphone" ]]; then
    echo "No available iPhone simulator devices were found." >&2
    echo "Installed Xcode destinations:" >&2
    xcodebuild -showdestinations -project "$PROJECT" -scheme "$SCHEME" >&2 || true
    echo >&2
    echo "Create one in Xcode: Window > Devices and Simulators > Simulators." >&2
    exit 70
  fi
  echo "platform=iOS Simulator,name=$iphone"

  # For UI/e2e tests also run on an iPad to exercise the persistent-sidebar
  # (regular width) layout. Skipped silently if no iPad simulator is installed.
  if [[ "$MODE" == "e2e" || "$MODE" == "ui" ]]; then
    local ipad
    ipad="$(first_sim_named iPad "$devices")"
    if [[ -n "$ipad" ]]; then
      echo "platform=iOS Simulator,name=$ipad"
    else
      echo "No iPad simulator found; skipping iPad (persistent-sidebar) coverage." >&2
    fi
  fi
}

# Forward the E2E configuration to the on-simulator test runner. xcodebuild only
# propagates host env vars prefixed with TEST_RUNNER_ into the test process, so a
# bare TAGNOTE_E2E_SERVER_URL set by the caller would otherwise be ignored and
# the suite would silently fall back to the http://localhost:3777 default.
for e2e_var in TAGNOTE_E2E_SERVER_URL TAGNOTE_E2E_EMAIL TAGNOTE_E2E_PASSWORD; do
  if [[ -n "${(P)e2e_var:-}" ]]; then
    export "TEST_RUNNER_${e2e_var}=${(P)e2e_var}"
  fi
done

# Run each destination in its own xcodebuild invocation (sequentially) rather
# than passing several -destination flags to one invocation. Parallel
# destinations share the host and the same backend IP, so simultaneous
# simulators contend for resources and burst the auth rate limiter; running them
# one at a time keeps the UI suite deterministic.
typeset -a destinations
while IFS= read -r destination; do
  [[ -z "$destination" ]] && continue
  destinations+=("$destination")
done < <(pick_destinations)

overall_rc=0
for destination in "${destinations[@]}"; do
  echo "==> Testing on: $destination"
  cmd=(
    xcodebuild test
    -project "$PROJECT"
    -scheme "$SCHEME"
    -destination "$destination"
  )
  if [[ -n "$ONLY_TESTING" ]]; then
    cmd+=("-only-testing:$ONLY_TESTING")
  fi
  if ! "${cmd[@]}"; then
    overall_rc=1
    echo "FAILED on destination: $destination" >&2
  fi
done

exit $overall_rc
