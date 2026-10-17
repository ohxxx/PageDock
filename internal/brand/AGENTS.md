# Default branding

`icon.png` is the single source for the default logo: a browser window with three dots nested in a tray with uniform stroke weight. Keep it neutral, monochrome, and text-free. The mark fills most of the square canvas width for clear rendering at small sizes. imagegen generated and refined the artwork from user feedback.

`icon.go` embeds/decodes the PNG for native settings previews and the Windows title bar. Show it only when no custom image exists; never persist the default image into user config. `mygo.json` references the same file for platform icon generation. README references it directly; Playground serves it through the explicit `/icon.png` allowlist entry.

When replacing the logo, check native previews, custom/default restoration, README, Playground, and platform builds together. Record validation in tests/AGENTS.md; generation prompts and local evidence live in `.scratch/minimal-design/`.
