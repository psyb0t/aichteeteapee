# Testing

Prefix: TEST. How to run the suites is in `AGENTS.md`.

- **TEST1:** Tests that start a real server, hub or socket are named `*_integration_test.go` and run in the normal suite; there are no build tags.
- **TEST3:** A test that starts goroutines (hub, client, connection pumps, the Timeout middleware) shuts them down through the code's own `Close` or `Stop` before it returns. The older WebSocket tests still use fixed `time.Sleep` calls; new tests wait on done channels or `assert.Eventually` instead.
- **TEST4:** Listeners bind `127.0.0.1:0` and read the real address back (`GetHTTPListenerAddr`). Unix socket and upload tests write under `t.TempDir()`. Static file tests read `serbewr/.fixtures/`; add fixtures there, not elsewhere.
- **TEST5:** The coverage floor is 90% over the whole module (`MIN_TEST_COVERAGE` in the `Makefile`, enforced by `scripts/test-coverage.sh`).
- **TEST6:** A fake test secret that trips gitleaks is allowed as exactly that value in that file: the `recovery_test.go` entry in `.gitleaks.toml`, or an inline `//gitleaks:allow`.
