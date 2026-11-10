# Testing

Prefix: TEST. How to run the suites is in `AGENTS.md`.

- **TEST1:** Tests sit next to the code, named after the file they cover (`server_test.go` for `server.go`), in the same package (white-box). Tests that start a real server, hub or socket are named `*_integration_test.go` and run in the normal suite; there are no build tags.
- **TEST2:** Use testify: `require` for preconditions that make the rest of the test meaningless, `assert` for the checks. Table-driven tests with `t.Run` are the default shape.
- **TEST3:** Everything runs with `-race`. A test that starts goroutines (hub, client, connection pumps, the Timeout middleware) shuts them down through the code's own `Close` or `Stop` before it returns. New tests wait on done channels or `assert.Eventually` rather than adding more fixed `time.Sleep` calls; the older WebSocket tests still have some.
- **TEST4:** Listeners bind `127.0.0.1:0` and read the real address back (`GetHTTPListenerAddr`). Unix socket and upload tests write under `t.TempDir()`. Static file tests read `serbewr/.fixtures/`; add fixtures there, not elsewhere.
- **TEST5:** The coverage floor is 90% over the whole module (`MIN_TEST_COVERAGE` in the `Makefile`, enforced by `scripts/test-coverage.sh`). Add tests for the code you add. Do not lower the floor or exclude files to pass.
- **TEST6:** Fake secrets in tests are obviously fake. When one trips gitleaks, allow exactly that value in that file (the `recovery_test.go` entry in `.gitleaks.toml`, an inline `//gitleaks:allow`), never a whole rule or directory.
