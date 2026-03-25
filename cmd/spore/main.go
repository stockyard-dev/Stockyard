// Stockyard Spore — standalone binary.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/stockyard-dev/stockyard/internal/spore/server"
	"github.com/stockyard-dev/stockyard/internal/spore/store"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("spore %s\n", version)
		os.Exit(0)
	}

	dataDir := os.Getenv("SPORE_DATA_DIR")
	if dataDir == "" {
		dataDir = "/tmp/spore"
	}
	os.MkdirAll(dataDir, 0o755)

	db, err := store.Open(filepath.Join(dataDir, "spore.db"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	port := 9709
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}

	srv := server.New(server.Config{Port: port, Store: db})
	fmt.Printf("Spore listening on :%d\n", port)
	srv.ListenAndServe()
}
