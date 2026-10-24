# Releasing

1. Bump the version in `mapricot.go` (`version`), following [semantic versioning](https://semver.org/), and merge it to `main`.
2. Publish a GitHub release whose tag is that version with a `v`, such as `v0.2.0`: `gh release create v0.2.0 --generate-notes`.
3. The tag is the release. The release workflow checks the version constant and asks the Go proxy for the new version, so pkg.go.dev lists it right away.

## Keeping up with the API

CI runs every Monday as well as on each push. Its `endpoints` job fails when [the API's OpenAPI description](https://api.mapricot.com/openapi.json) lists an endpoint this client has no method for, and its tests fail when a live response changes. Add or fix the method and its test, and release a new minor version. A new endpoint is added to every Mapricot SDK at the same time.

## One-time setup

- A repository secret or organization secret `MAPRICOT_KEY`: a server key with the `places`, `routing`, and `ip-geolocation` scopes and a monthly cap, used by the live tests.
- Nothing else: pkg.go.dev reads public repositories through the Go proxy.
