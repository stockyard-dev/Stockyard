// Stockyard TidePool — standalone binary.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/stockyard-dev/stockyard/internal/tidepool/server"
	"github.com/stockyard-dev/stockyard/internal/tidepool/store"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("tidepool %s\n", version)
		os.Exit(0)
	}

	dataDir := os.Getenv("TIDEPOOL_DATA_DIR")
	if dataDir == "" {
		dataDir = "/tmp/tidepool"
	}
	os.MkdirAll(dataDir, 0o755)

	db, err := store.Open(filepath.Join(dataDir, "tidepool.db"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	port := 9705
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}

	srv := server.New(server.Config{Port: port, Store: db})
	fmt.Printf("TidePool listening on :%d\n", port)
	srv.ListenAndServe()
}
