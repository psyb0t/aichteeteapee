# Go style

Prefix: GO. `.golangci.yml` is the authority; `make lint` must pass with zero issues.

- **GO1:** Lines stay within 80 columns (`lll`, tab width 4). Long signatures put one parameter per line, as every exported function in `serbewr/` does.
- **GO2:** The linter set is strict (`err113`, `wrapcheck`, `mnd`, `gochecknoglobals`, `ireturn`, `wsl_v5`, `nlreturn` and the rest). Fix the code rather than adding `//nolint`. When a `//nolint` is right, name the linter and give the reason on the same line, as `//nolint:funlen // Graceful shutdown logic requires length` in `serbewr/server.go` does.
- **GO3:** Imports outside the standard library must be on the `depguard` allow list in `.golangci.yml`. A new dependency means editing that list, `make dep` (tidy plus vendor) and committing `vendor/`.
- **GO4:** Magic numbers become named constants: in `defaults.go` when callers may care, as unexported package constants otherwise (`maxRequestIDLength`, `defaultFileCacheTTL`, `bufferSize`).
- **GO5:** Concurrency follows the existing patterns: `sync.Once` for one-shot shutdown (`Server.Stop`, `Connection.Stop`), `atomic.Bool` for state flags read on hot paths, `sync.RWMutex` around maps with copy-on-read getters (`clientsMap.GetAll`), and `sync.WaitGroup.Go` for goroutines a shutdown has to wait for.
- **GO6:** Comments say why, not what, and end with a period (`godot`). New doc comments on exported identifiers start with the identifier's name.
- **GO7:** `go fix -diff ./...` must print nothing; `make lint-fix` applies its rewrites.
