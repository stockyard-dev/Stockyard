// sy-docs generates the Stockyard documentation site as static HTML.
//
// Usage:
//
//	sy-docs                     # Generate to ./docs-site
//	sy-docs -o /path/to/output  # Generate to custom directory
//	sy-docs --help              # Show help
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/stockyard-dev/stockyard/internal/apiserver"
	"github.com/stockyard-dev/stockyard/internal/docs"
)

func main() {
	log.SetFlags(0)

	output := flag.String("o", "./docs-site", "Output directory for generated docs")
	flag.Parse()

	if len(os.Args) > 1 && (os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println("sy-docs -- Stockyard documentation site generator")
		fmt.Println()
		fmt.Println("Generates static HTML documentation for all", apiserver.CatalogCount(), "products.")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  sy-docs              Generate to ./docs-site")
		fmt.Println("  sy-docs -o dist/docs  Generate to custom directory")
		fmt.Println()
		fmt.Println("The output directory can be deployed to any static hosting")
		fmt.Println("service (Cloudflare Pages, Vercel, Netlify, GitHub Pages).")
		os.Exit(0)
	}

	start := time.Now()

	products := apiserver.Catalog()
	log.Printf("Generating docs for %d products...", len(products))

	cfg := docs.GenerateConfig{
		OutputDir: *output,
	}

	if err := docs.Generate(cfg); err != nil {
		log.Fatalf("Error: %v", err)
	}

	elapsed := time.Since(start)

	// Count generated files
	var count int
	countFiles(*output, &count)

	log.Printf("Done. %d HTML files generated in %s", count, elapsed.Round(time.Millisecond))
	log.Printf("Output: %s", *output)
	log.Printf("")
	log.Printf("To preview locally:")
	log.Printf("  cd %s && python3 -m http.server 8080", *output)
	log.Printf("  open http://localhost:8080")
}

func countFiles(dir string, count *int) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			countFiles(dir+"/"+e.Name(), count)
		} else {
			*count++
		}
	}
}
