// scenario-server: Sliver Scenario Orchestrator
//
// Connects to a running Sliver server using an operator config file,
// loads the MITRE ATT&CK atomic library from a directory of YAML files,
// and exposes a REST API for building and executing attack chains.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bishopfox/sliver/scenario/api"
	"github.com/bishopfox/sliver/scenario/atomic"
	"github.com/bishopfox/sliver/scenario/config"
	slvr "github.com/bishopfox/sliver/scenario/sliver"
	"github.com/bishopfox/sliver/scenario/store"
)

func main() {
	var (
		configFile  = flag.String("config-file", "", "Path to YAML config file (optional)")
		sliverCfg   = flag.String("config", "", "Path to Sliver operator .cfg file (required)")
		atomicsDir  = flag.String("atomics", "", "Path to atomics YAML directory")
		dbPath      = flag.String("db", "", "Path to SQLite database file")
		listen      = flag.String("listen", "", "HTTP listen address (default :8080)")
		allowOrigin = flag.String("allow-origin", "", "CORS Allow-Origin header value")
	)
	flag.Parse()

	// Load config: file → env → flags
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// CLI flags override everything else
	if *sliverCfg != "" {
		cfg.SliverConfig = *sliverCfg
	}
	if *atomicsDir != "" {
		cfg.AtomicsDir = *atomicsDir
	}
	if *dbPath != "" {
		cfg.DBPath = *dbPath
	}
	if *listen != "" {
		cfg.ListenAddr = *listen
	}
	if *allowOrigin != "" {
		cfg.AllowOrigin = *allowOrigin
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		fmt.Fprintln(os.Stderr, "Run with --help for usage.")
		os.Exit(1)
	}

	log.Printf("scenario-server starting  %s", cfg)

	// ── Atomic library ─────────────────────────────────────────────────────
	lib := atomic.NewLibrary()
	if cfg.AtomicsDir != "" {
		if err := lib.LoadDir(cfg.AtomicsDir); err != nil {
			log.Printf("WARNING: loading atomics from %q: %v", cfg.AtomicsDir, err)
		} else {
			log.Printf("Loaded %d techniques from %s", lib.Count(), cfg.AtomicsDir)
		}
	}

	// ── SQLite store ────────────────────────────────────────────────────────
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	log.Printf("Database: %s", cfg.DBPath)

	// ── Sliver gRPC connection ──────────────────────────────────────────────
	operatorCfg, err := slvr.LoadConfig(cfg.SliverConfig)
	if err != nil {
		log.Fatalf("loading Sliver operator config: %v", err)
	}

	log.Printf("Connecting to Sliver server %s:%d as %q...", operatorCfg.LHost, operatorCfg.LPort, operatorCfg.Operator)
	rpc, conn, err := slvr.Connect(operatorCfg)
	if err != nil {
		log.Fatalf("connecting to Sliver: %v", err)
	}
	defer conn.Close()
	log.Printf("Connected to Sliver server")

	// ── HTTP server ─────────────────────────────────────────────────────────
	srv := api.NewServer(st, lib, rpc, cfg.SliverConfig, cfg.C2Host, cfg.AllowOrigin)
	httpSrv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      srv.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // SSE endpoints need unlimited write time
		IdleTimeout:  120 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Listening on %s", cfg.ListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down...")
}
