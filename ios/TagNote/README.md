# TagNote iPhone App

The iPhone app is a native SwiftUI app. It cannot run inside Docker because iOS
Simulator and app signing require Xcode on macOS.

For local development, run the existing TagNote backend in Docker and connect to
it from iOS Simulator.

## Run The Backend

From the repository root:

```bash
TAGNOTE_TEST_MODE=1 docker compose build
TAGNOTE_TEST_MODE=1 docker compose up -d
```

The API is available at:

```text
http://localhost:3777
```

When `TAGNOTE_TEST_MODE=1`, use:

```text
Email: test@test.com
Password: testpass123
```

## Run The iPhone App

1. Open `ios/TagNote/TagNote.xcodeproj` in Xcode.
2. Select an iPhone simulator.
3. Press Run.
4. On the first screen, enter:

```text
http://localhost:3777
```

iOS Simulator resolves `localhost` to the host Mac, so it can reach the Docker
Compose port mapping directly.

For a physical iPhone, use the Mac's LAN address instead, for example:

```text
http://192.168.1.20:3777
```

Production use should be served over HTTPS.

## Run iOS Tests

Unit tests do not need the TagNote backend. The helper script automatically
selects the first available iPhone simulator:

```bash
ios/TagNote/scripts/test-ios.sh unit
```

E2E UI tests need the Docker backend running and test credentials configured.
The backend **must** run with `TAGNOTE_TEST_MODE=1`: it creates the test account
and relaxes the auth rate limiter, which the suite would otherwise trip (it
performs several logins). The UI suite also runs on an iPhone (compact,
slide-over drawer) and an iPad (regular, persistent sidebar) by default to cover
both adaptive layouts.

```bash
TAGNOTE_TEST_MODE=1 docker compose up -d

TAGNOTE_E2E_SERVER_URL=http://localhost:3777 \
TAGNOTE_E2E_EMAIL=test@test.com \
TAGNOTE_E2E_PASSWORD=testpass123 \
ios/TagNote/scripts/test-ios.sh e2e
```

The script forwards these `TAGNOTE_E2E_*` variables to the on-simulator test
runner via the `TEST_RUNNER_` prefix that `xcodebuild` requires; setting them
when invoking `xcodebuild` directly (without that prefix) has no effect, and the
suite falls back to the `http://localhost:3777` default.

App Store screenshots are produced by `testScreenshotsForAppStoreListing`,
attached to the result bundle (`-resultBundlePath`) with `.keepAlways`. Running
the suite on both an iPhone and an iPad yields both required screenshot sets;
export them with `xcrun xcresulttool export attachments`.

If you are running the backend on a remote server, expose it through an SSH
tunnel first:

```bash
ssh -L 3777:localhost:3777 user@server
```

To force a specific simulator, set `IOS_TEST_DESTINATION`:

```bash
IOS_TEST_DESTINATION='platform=iOS Simulator,name=iPhone 15 Pro' \
ios/TagNote/scripts/test-ios.sh unit
```

## Visual verification without accessibility permissions

Driving the Simulator with `osascript`/AppleScript or Python `Quartz` (taps,
drags, scrolls) requires the controlling terminal/IDE to hold macOS
**Accessibility** and **Automation** permissions. Those are SIP-protected and
cannot be granted programmatically — an agent usually can't enable them, and
they fail with `osascript is not allowed assistive access (-1719)`.

Prefer the **launch-argument + `simctl screenshot`** harness instead. It is
fully deterministic and needs **no** accessibility permission, because the app
puts itself into the state you want to inspect at launch, and `xcrun simctl`
talks to the simulator daemon (no Accessibility/Automation grant required):

| Launch env var | Effect |
| --- | --- |
| `TAGNOTE_UI_CREATE_NOTE=1` | Opens the editor on launch. |
| `TAGNOTE_UI_OPEN_SIDEBAR=1` | Opens the compact drawer on launch. |
| `TAGNOTE_UI_SCROLL_BOTTOM=1` | Starts the sidebar scrolled to the bottom (for safe-area / clipping checks). |
| `TAGNOTE_UI_PRESELECT_TAGS=a,b` | Pre-applies the given comma-separated tag filters on launch (for filter-chip checks). |

Pass them to the app with the `SIMCTL_CHILD_` prefix. Example — capture the
drawer scrolled to the bottom (e.g. to verify content does not slide under the
status bar / Dynamic Island):

```bash
DEV=<booted-simulator-udid>
xcodebuild build -project TagNote.xcodeproj -scheme TagNote \
  -destination "platform=iOS Simulator,name=iPhone 17 Pro" \
  -derivedDataPath /tmp/tagnote-dd
xcrun simctl install "$DEV" \
  /tmp/tagnote-dd/Build/Products/Debug-iphonesimulator/TagNote.app
xcrun simctl terminate "$DEV" com.tagnote.ios 2>/dev/null
SIMCTL_CHILD_TAGNOTE_UI_OPEN_SIDEBAR=1 SIMCTL_CHILD_TAGNOTE_UI_SCROLL_BOTTOM=1 \
  xcrun simctl launch "$DEV" com.tagnote.ios
xcrun simctl io "$DEV" screenshot /tmp/drawer.png
```

For an objective pass/fail (instead of eyeballing the screenshot), sample
pixels with Python + Pillow. To confirm nothing renders under the status bar,
count dark (text) pixels in the strip *above* the clock — it must be ~0:

```python
from PIL import Image
im = Image.open("/tmp/drawer.png").convert("RGB"); px = im.load()
# iPhone 17 Pro @3x is 1206x2622; the clock sits ~y60+, so y in [5,55] is above it.
dark = sum(
    1
    for y in range(5, 55, 2)
    for x in range(60, 420, 2)            # drawer width, left of the Dynamic Island
    if sum(px[x, y]) / 3 < 150
)
print("FAIL: content under status bar" if dark > 5 else "PASS: strip clear")
```

This harness caught and verified the sidebar safe-area fix (commit `76951c7`):
the strip held ~747 dark pixels while scrolled until the drawer's `ScrollView`
was pinned to the safe area with fixed inset spacers and `.clipped()`, after
which it measured 0.

### Keep screenshots small when reading them back

`xcrun simctl io ... screenshot` captures at full device resolution — an iPad
Pro 13" frame is 2064×2752 (~5.7 MP, 1–3 MB per PNG). Large image payloads are
slow and unreliable to load back into an agent: in a long, tool-heavy session
they can arrive late or come back blank, which looks like "intermittent image
rendering" (the image is fine — its delivery is just delayed by size). Downscale
before reading, and read one image per turn:

```bash
sips -Z 1000 /tmp/drawer.png --out /tmp/drawer_small.png   # ~6x smaller payload
```

Best of all, don't rely on *seeing* the image: run the Pillow pixel check above
and read its small text result instead. A one-line PASS/FAIL is fully reliable
regardless of image size or session length, and is what the safe-area /
status-bar fixes were ultimately verified with.

## Headless Mac Mini CI

iOS Simulator tests can run without a visible monitor, but they still need a
logged-in macOS Aqua user session. Do not run them from a system LaunchDaemon or
an SSH-only bootstrap context.

Recommended setup:

1. Keep the CI user logged in on the Mac mini.
2. Enable Screen Sharing for maintenance, but the screen can be locked.
3. Run the test worker as a per-user LaunchAgent under `~/Library/LaunchAgents`.
4. Run `ios/TagNote/scripts/test-ios.sh unit` or `e2e` from that agent.

If `xcrun simctl list devices available` fails with `CoreSimulatorService
connection became invalid`, the process is not running with usable access to the
GUI user's CoreSimulator services. Start the job from the logged-in user session
or reconnect with a tool that preserves that session.
