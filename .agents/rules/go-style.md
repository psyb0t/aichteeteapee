# Go style

Prefix: GO.

- **GO3:** Imports outside the standard library must be on the `depguard` allow list in `.golangci.yml`. A new dependency means editing that list, `make dep` (tidy plus vendor) and committing `vendor/`.
- **GO5:** Concurrency follows the existing patterns: `sync.Once` for one-shot shutdown (`Server.Stop`, `Connection.Stop`), `atomic.Bool` for state flags read on hot paths, `sync.RWMutex` around maps with copy-on-read getters (`clientsMap.GetAll`), and `sync.WaitGroup.Go` for goroutines a shutdown has to wait for.
