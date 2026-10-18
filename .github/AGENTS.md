# GitHub automation

| Path | Responsibility |
|---|---|
| `workflows/test.yml` | macOS, Windows, and Linux headless validation on main pushes, pull requests, manual runs, and release calls |
| `workflows/release.yml` | Version validation, gated six-target packaging, artifact uploads, checksums, and complete GitHub releases |

Use the root Makefile. Download Go modules before its patch preparer runs on a clean runner. Preserve MyGo's isolated dependency copy and centralized test overlay; bare go test does not discover the repository's tests. Go follows go.mod; Playground uses Node 24. Linux tests install WebKitGTK 4.1 and fonts. Windows tests install Make. Hosted tests do not run desktop, notification, or physical-media suites.

Release tags must be stable vMAJOR.MINOR.PATCH versions matching mygo.json. Tag pushes validate and test before building macOS arm64/amd64 DMGs, Windows amd64/arm64 NSIS installers, and Linux amd64/arm64 Debian packages and tar archives. Manual Release runs build the same artifacts without publishing. Build jobs receive read-only permissions; only the final publish job receives contents: write through GitHub's built-in token. No repository secrets are required.

Collect each target's expected package types explicitly so a missing installer fails. Replace package filename spaces with hyphens before uploading artifacts or calculating checksums, since GitHub renames asset filenames containing spaces. Upload only distribution packages, never source trees, caches, or user configuration. All builds must pass before creating a draft release. Upload packages and SHA256SUMS before making it public. Failed uploads leave a draft that can be retried; never overwrite an already published version. Run concurrency is serialized per ref.

Keep macOS ad-hoc signing and unsigned Windows packaging: custom appearance requires rewriting writable app resources. Developer ID/notarization and signed automatic updates need separate product decisions and are not enabled here. Linux custom-appearance limitations remain as documented in the root AGENTS.md.

After committing an intended version, explicitly push its matching tag to release it. Do not add scripts that automatically commit or push. Changes to commands, artifact types, or trigger behavior require corresponding Makefile, root AGENTS.md, README.md, and tests/AGENTS.md updates. Validate workflow syntax with actionlint and execute changed shell behavior; do not write source-string contract tests.
