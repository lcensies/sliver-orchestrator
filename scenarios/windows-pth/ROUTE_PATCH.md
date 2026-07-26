# Register the Windows implant route

In `api/server.go`, find this line (around line 110):

```go
	// Implant delivery — generates a Sliver beacon on demand and serves the binary
	mux.HandleFunc("GET /api/v1/implant/linux", s.handleGetImplantLinux)
```

Add the Windows route directly underneath it:

```go
	// Implant delivery — generates a Sliver beacon on demand and serves the binary
	mux.HandleFunc("GET /api/v1/implant/linux", s.handleGetImplantLinux)
	mux.HandleFunc("GET /api/v1/implant/windows", s.handleGetImplantWindows)
```

Then copy `implant_windows.go` into the repo's `api/` directory and rebuild:

```bash
cp implant_windows.go /path/to/sliver-orchestrator/api/
make scenario-server     # or: make scenario
```

That's the only wiring needed — `implant_windows.go` reuses the existing
`ensureHTTPListener`, `serveImplant`, `globalImplantCache`, `defaultListenerPort`
and `buildTimeout` from `implant.go`, and adds its own `findMatchingBuildOS`
(OS-parameterized) so there is no clash with the existing `findMatchingBuild`.
