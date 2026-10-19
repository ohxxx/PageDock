<p align="center"><img src="internal/brand/icon.png" width="96" alt="PageDock logo"></p>

# PageDock

Your favorite website, in its own desktop window.

Enter a URL, save, and open. PageDock restores your website, sign-in session, and window position on the next launch.

## Features

- **Dedicated window** — System WebView without browser tabs or an address bar.
- **Custom appearance** — Set an app name and icon you can recognize at a glance.
- **Desktop notifications** — Receive website notifications and click to return to the relevant page.
- **Permission controls** — Manage notifications, camera, microphone, and available location permissions for the saved site.
- **Native settings** — System light/dark appearance, Simplified Chinese, and English.
- **Local data** — Settings stay on your device; clear sign-in data or restore defaults.

## Get started

Download the package for your system and architecture from [GitHub Releases](https://github.com/ohxxx/PageDock/releases). macOS packages use ad-hoc signing without Apple notarization; Windows installers are unsigned, so system security checks may require explicit approval.

To run from source, install Go 1.27.1 and start the development app:

```sh
make dev
```

On first launch, enter a website address and choose **Save and Apply**. HTTPS is the default; enable HTTP in settings for local or intranet sites.

Build the app with `make build`, or both Windows architectures with `make build-windows`.

## Releases

Set `version` in `mygo.json`, commit the intended changes, then explicitly push a matching `vMAJOR.MINOR.PATCH` tag. GitHub Actions runs tests on all three systems, builds both architectures, and publishes the packages with `SHA256SUMS`. A manual **Release** workflow run builds downloadable artifacts without publishing.

The workflow uses GitHub's built-in token and requires no additional secrets. See [GitHub automation](.github/AGENTS.md) for the release process.

## Platforms

Supports macOS, Windows, and Linux. Windows requires WebView2; Linux requires WebKitGTK 4.1.

Custom app icons are unavailable on Linux. Website location controls are unavailable on macOS. Website notifications require PageDock to remain running and follow system notification settings.

During each run, PageDock keeps up to 256 notifications for up to 24 hours, removing the oldest when the limit is reached.

## Playground

Run `make playground` to test notifications, click callbacks, microphone, and camera locally. Audio and video are never uploaded.

[Development guidelines](AGENTS.md) · [Testing and validation](tests/AGENTS.md)
