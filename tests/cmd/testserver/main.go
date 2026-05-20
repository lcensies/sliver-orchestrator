// testserver is a minimal HTTP server for E2E testing.
// It uses a stub SliverRPC client so no real C2 connection is required.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bishopfox/sliver/scenario/api"
	"github.com/bishopfox/sliver/scenario/atomic"
	"github.com/bishopfox/sliver/scenario/store"
)

func main() {
	listen   := flag.String("listen", ":18765", "listen address")
	dbPath   := flag.String("db", "", "sqlite db path (default: temp file)")
	atomicsD := flag.String("atomics", "", "atomics dir (optional)")
	flag.Parse()

	db := *dbPath
	if db == "" {
		f, err := os.CreateTemp("", "testserver-*.db")
		if err != nil {
			log.Fatalf("temp db: %v", err)
		}
		f.Close()
		db = f.Name()
		defer os.Remove(db)
	}

	st, err := store.Open(db)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	lib := atomic.NewLibrary()
	if *atomicsD != "" {
		if err := lib.LoadDir(*atomicsD); err != nil {
			log.Printf("WARNING: atomics: %v", err)
		}
	}

	srv := api.NewServer(st, lib, &stubRPC{}, "", "", "*")
	httpSrv := &http.Server{
		Addr:        *listen,
		Handler:     srv.Handler(),
		ReadTimeout: 30 * time.Second,
	}
	log.Printf("testserver listening on %s", *listen)
	if err := httpSrv.ListenAndServe(); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
