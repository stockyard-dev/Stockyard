// Package main implements stockyard-manifest-validate per docs/VALIDATOR-SPEC.md
// in stockyard-desktop. Advisory-only CI check; not enforced at runtime.
//
// This first pass covers: E001-E004 (structural), E010-E014 (type-level),
// E030-E033 (catalog equality). Cross-tool event graph (E040-E042) is
// deferred until more than one tool ships a real manifest.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Manifest mirrors stockyard.json per TOOL-SCHEMA-DESIGN.md §4.
// Only fields the validator needs are strongly typed; the rest are
// carried through as map[string]any so unknown fields don't error.
type Manifest struct {
	SchemaVersion string                 `json:"schema_version"`
	Tool          *ToolIdentity          `json:"tool"`
	DataModel     *DataModel             `json:"data_model"`
	Config        *ConfigBlock           `json:"config"`
	Events        map[string]any         `json:"events"`
	Raw           map[string]any         `json:"-"`
}

type ToolIdentity struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

type DataModel struct {
	Tables []Table `json:"tables"`
}

type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
	Sync    string   `json:"sync"`
	Backup  string   `json:"backup"`
}

type Column struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	PrimaryKey bool     `json:"primary_key"`
	Required   bool     `json:"required"`
	Editable   *bool    `json:"editable"`
	PII        bool     `json:"pii"`
	Enum       []string `json:"enum"`
	Display    string   `json:"display"`
}

type ConfigBlock struct {
	Fields []Column `json:"fields"`
}

// CatalogEntry is the subset of site/tools/catalog.json we care about.
// Note: catalog uses "name" where the manifest uses "display_name".
type CatalogEntry struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// Finding is one check result. Severity is "error" or "warning".
type Finding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

const currentSchemaVersion = "1"

var (
	allowedColumnTypes = map[string]bool{
		"text": true, "integer": true, "real": true,
		"blob": true, "timestamp": true, "json": true,
	}
	allowedSync   = map[string]bool{"replicated": true, "local_only": true, "ephemeral": true}
	allowedBackup = map[string]bool{"encrypted": true, "skip": true}
)

func main() {
	var (
		catalogPath = flag.String("catalog", "", "path to site/tools/catalog.json (enables E030-E033)")
		strict      = flag.Bool("strict", false, "promote warnings to errors")
		asJSON      = flag.Bool("json", false, "machine-readable JSON output")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] <manifest-or-dir>...\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	manifests, err := collectManifests(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	var catalog map[string]CatalogEntry
	if *catalogPath != "" {
		catalog, err = loadCatalog(*catalogPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading catalog: %v\n", err)
			os.Exit(2)
		}
	}

	var findings []Finding
	for _, p := range manifests {
		findings = append(findings, validateFile(p, catalog, *catalogPath != "")...)
	}

	hasError := emit(findings, *asJSON, *strict)
	if hasError {
		os.Exit(1)
	}
}

func collectManifests(paths []string) ([]string, error) {
	var out []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, p)
			continue
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				candidate := filepath.Join(p, e.Name(), "stockyard.json")
				if _, err := os.Stat(candidate); err == nil {
					out = append(out, candidate)
				}
			}
		}
		// also accept a stockyard.json directly in the dir
		direct := filepath.Join(p, "stockyard.json")
		if _, err := os.Stat(direct); err == nil {
			out = append(out, direct)
		}
	}
	sort.Strings(out)
	return out, nil
}

func loadCatalog(path string) (map[string]CatalogEntry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []CatalogEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	out := make(map[string]CatalogEntry, len(entries))
	for _, e := range entries {
		out[e.Slug] = e
	}
	return out, nil
}

func validateFile(path string, catalog map[string]CatalogEntry, haveCatalog bool) []Finding {
	b, err := os.ReadFile(path)
	if err != nil {
		return []Finding{{Code: "E001", Severity: "error", Path: path, Message: "cannot read file: " + err.Error()}}
	}

	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return []Finding{{Code: "E001", Severity: "error", Path: path, Message: "file does not parse as JSON: " + err.Error()}}
	}
	// Re-parse into Raw for unknown-field friendliness.
	_ = json.Unmarshal(b, &m.Raw)

	var f []Finding
	f = append(f, checkStructural(path, &m)...)
	f = append(f, checkTypes(path, &m)...)
	f = append(f, checkSemantic(path, &m)...)
	if haveCatalog {
		f = append(f, checkCatalog(path, &m, catalog)...)
	}
	return f
}

func checkStructural(path string, m *Manifest) []Finding {
	var f []Finding
	if m.SchemaVersion == "" {
		f = append(f, Finding{"E002", "error", path, "schema_version missing"})
	} else if m.SchemaVersion != currentSchemaVersion {
		f = append(f, Finding{"E002", "error", path, "schema_version " + m.SchemaVersion + " not understood (expected " + currentSchemaVersion + ")"})
	}
	if m.Tool == nil {
		f = append(f, Finding{"E003", "error", path, "required block 'tool' missing"})
	} else {
		if m.Tool.Slug == "" {
			f = append(f, Finding{"E004", "error", path, "tool.slug missing"})
		}
		if m.Tool.DisplayName == "" {
			f = append(f, Finding{"E004", "error", path, "tool.display_name missing"})
		}
		if m.Tool.Version == "" {
			f = append(f, Finding{"E004", "error", path, "tool.version missing"})
		}
	}
	if _, ok := m.Raw["data_model"]; !ok {
		f = append(f, Finding{"E003", "error", path, "required block 'data_model' missing"})
	}
	if _, ok := m.Raw["config"]; !ok {
		f = append(f, Finding{"E003", "error", path, "required block 'config' missing"})
	}
	if _, ok := m.Raw["events"]; !ok {
		f = append(f, Finding{"E003", "error", path, "required block 'events' missing"})
	}
	return f
}

func checkTypes(path string, m *Manifest) []Finding {
	var f []Finding
	if m.DataModel == nil {
		return f
	}
	for _, t := range m.DataModel.Tables {
		if t.Sync != "" && !allowedSync[t.Sync] {
			f = append(f, Finding{"E013", "error", path, "table " + t.Name + ": sync '" + t.Sync + "' not in (replicated,local_only,ephemeral)"})
		}
		if t.Backup != "" && !allowedBackup[t.Backup] {
			f = append(f, Finding{"E014", "error", path, "table " + t.Name + ": backup '" + t.Backup + "' not in (encrypted,skip)"})
		}
		for _, c := range t.Columns {
			if !allowedColumnTypes[c.Type] {
				f = append(f, Finding{"E010", "error", path, "table " + t.Name + " column " + c.Name + ": type '" + c.Type + "' not in allowed set"})
			}
			if c.PrimaryKey && c.Editable != nil && *c.Editable {
				f = append(f, Finding{"E011", "error", path, "table " + t.Name + " column " + c.Name + ": primary_key cannot be editable"})
			}
			if len(c.Enum) > 0 && c.Type != "text" {
				f = append(f, Finding{"E012", "error", path, "table " + t.Name + " column " + c.Name + ": enum requires type=text"})
			}
		}
	}
	return f
}

func checkSemantic(path string, m *Manifest) []Finding {
	var f []Finding
	if m.DataModel == nil {
		return f
	}
	for _, t := range m.DataModel.Tables {
		sync := t.Sync
		if sync == "" {
			sync = "replicated"
		}
		backup := t.Backup
		if backup == "" {
			backup = "encrypted"
		}
		for _, c := range t.Columns {
			if c.PII && c.Display == "" {
				f = append(f, Finding{"W020", "warning", path, "table " + t.Name + " column " + c.Name + ": pii:true has no display"})
			}
			if c.PII && sync == "replicated" && backup == "skip" {
				f = append(f, Finding{"W021", "warning", path, "table " + t.Name + " column " + c.Name + ": replicated PII with backup:skip is a policy smell"})
			}
		}
	}
	return f
}

func checkCatalog(path string, m *Manifest, catalog map[string]CatalogEntry) []Finding {
	var f []Finding
	if m.Tool == nil || m.Tool.Slug == "" {
		return f // structural check already flagged
	}
	entry, ok := catalog[m.Tool.Slug]
	if !ok {
		return []Finding{{"E030", "error", path, "tool.slug '" + m.Tool.Slug + "' not present in catalog.json"}}
	}
	// Catalog's "name" maps to manifest's "display_name" — see schema doc §4a.
	if m.Tool.DisplayName != entry.Name {
		f = append(f, Finding{"E031", "error", path,
			"tool.display_name '" + m.Tool.DisplayName + "' != catalog name '" + entry.Name + "'"})
	}
	if m.Tool.Description != "" && entry.Description != "" && m.Tool.Description != entry.Description {
		f = append(f, Finding{"E032", "error", path,
			"tool.description disagrees with catalog.json (fix the manifest, not the catalog)"})
	}
	if m.Tool.Category != "" && entry.Category != "" && m.Tool.Category != entry.Category {
		f = append(f, Finding{"E033", "error", path,
			"tool.category '" + m.Tool.Category + "' != catalog category '" + entry.Category + "'"})
	}
	return f
}

func emit(findings []Finding, asJSON, strict bool) bool {
	hasErr := false
	for i := range findings {
		if strict && findings[i].Severity == "warning" {
			findings[i].Severity = "error"
		}
		if findings[i].Severity == "error" {
			hasErr = true
		}
	}
	if asJSON {
		b, _ := json.MarshalIndent(findings, "", "  ")
		fmt.Println(string(b))
		return hasErr
	}
	if len(findings) == 0 {
		fmt.Println("OK: all checks passed")
		return false
	}
	for _, x := range findings {
		fmt.Printf("%s  %s  %s: %s\n", x.Severity, x.Code, x.Path, x.Message)
	}
	return hasErr
}
