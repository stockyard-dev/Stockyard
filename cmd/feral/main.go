// Stockyard Feral — standalone binary.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/stockyard-dev/stockyard/internal/feral/server"
	"github.com/stockyard-dev/stockyard/internal/feral/store"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("feral %s\n", version)
		os.Exit(0)
	}

	dataDir := os.Getenv("FERAL_DATA_DIR")
	if dataDir == "" {
		dataDir = "/tmp/feral"
	}
	os.MkdirAll(dataDir, 0o755)

	db, err := store.Open(filepath.Join(dataDir, "feral.db"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	port := 9704
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}

	srv := server.New(server.Config{Port: port, Store: db})
	fmt.Printf("Feral listening on :%d\n", port)
	srv.ListenAndServe()
}
