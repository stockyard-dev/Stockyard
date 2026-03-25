// Stockyard Cortex — standalone binary.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/stockyard-dev/stockyard/internal/cortex/server"
	"github.com/stockyard-dev/stockyard/internal/cortex/store"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("cortex %s\n", version)
		os.Exit(0)
	}

	dataDir := os.Getenv("CORTEX_DATA_DIR")
	if dataDir == "" {
		dataDir = "/tmp/cortex"
	}
	os.MkdirAll(dataDir, 0o755)

	db, err := store.Open(filepath.Join(dataDir, "cortex.db"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	port := 9707
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}

	srv := server.New(server.Config{Port: port, Store: db})
	fmt.Printf("Cortex listening on :%d\n", port)
	srv.ListenAndServe()
}
