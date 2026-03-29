package engine

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/site"
)

const growthSchema = `
CREATE TABLE IF NOT EXISTS growth_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,
    category TEXT NOT NULL,
    metric TEXT NOT NULL,
    value REAL NOT NULL DEFAULT 0,
    source TEXT NOT NULL DEFAULT 'manual',
    notes TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(date, category, metric)
);
CREATE INDEX IF NOT EXISTS idx_growth_date ON growth_metrics(date);
CREATE INDEX IF NOT EXISTS idx_growth_cat ON growth_metrics(category);
`

// MigrateGrowth creates the growth_metrics table.
func MigrateGrowth(db *sql.DB) {
	for _, stmt := range strings.Split(growthSchema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			db.Exec(stmt)
		}
	}
	log.Println("[growth] metrics table ready")
}

// RegisterGrowthRoutes mounts the growth dashboard API.
func RegisterGrowthRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("GET /api/growth", func(w http.ResponseWriter, r *http.Request) {
		handleGrowthDashboard(w, r, db)
	})
	mux.HandleFunc("GET /api/growth/metrics", func(w http.ResponseWriter, r *http.Request) {
		handleGrowthMetrics(w, r, db)
	})
	mux.HandleFunc("POST /api/growth/metrics", func(w http.ResponseWriter, r *http.Request) {
		handleGrowthMetricUpsert(w, r, db)
	})
	mux.HandleFunc("DELETE /api/growth/metrics/{id}", func(w http.ResponseWriter, r *http.Request) {
		handleGrowthMetricDelete(w, r, db)
	})
	log.Println("[growth] routes: /api/growth, /api/growth/metrics")
}

// handleGrowthDashboard returns the full aggregated growth dashboard data.
func handleGrowthDashboard(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}
	daysBack := 30
	switch period {
	case "today", "1d":
		daysBack = 1
	case "7d":
		daysBack = 7
	case "90d":
		daysBack = 90
	}
	since := time.Now().AddDate(0, 0, -daysBack).Format("2006-01-02")

	result := map[string]any{
		"period":    period,
		"since":     since,
		"generated": time.Now().UTC().Format(time.RFC3339),
	}

	// ── 1. INSTALLS (auto-tracked) ──
	installs := map[string]any{}
	installStats := site.DownloadStats(db)
	installs["total"] = installStats["total"]
	installs["unique"] = installStats["unique"]
	installs["last_24h"] = installStats["last_24h"]
	installs["last_7d"] = installStats["last_7d"]
	installs["last_30d"] = installStats["last_30d"]
	installs["by_day"] = installStats["by_day"]
	installs["top_agents"] = installStats["top_agents"]
	installs["source"] = "auto"
	result["installs"] = installs

	// ── 2. USERS (auto-tracked) ──
	users := map[string]any{"source": "auto"}
	var totalUsers int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
	users["total"] = totalUsers

	var recentUsers int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE created_at >= ?", since).Scan(&recentUsers)
	users["period"] = recentUsers

	// Users by day
	uRows, _ := db.Query(`SELECT date(created_at) as d, COUNT(*) FROM users WHERE created_at >= ? GROUP BY d ORDER BY d`, since)
	if uRows != nil {
		defer uRows.Close()
		var uDaily []map[string]any
		for uRows.Next() {
			var d string
			var c int
			uRows.Scan(&d, &c)
			uDaily = append(uDaily, map[string]any{"date": d, "count": c})
		}
		users["by_day"] = uDaily
	}
	result["users"] = users

	// ── 3. TEAMS (auto-tracked) ──
	var totalTeams int
	db.QueryRow("SELECT COUNT(*) FROM teams").Scan(&totalTeams)
	result["teams"] = map[string]any{"total": totalTeams, "source": "auto"}

	// ── 4. NURTURE (auto-tracked) ──
	nurture := map[string]any{"source": "auto"}
	var nSent, nFailed int
	db.QueryRow("SELECT COUNT(*) FROM nurture_log WHERE status='sent'").Scan(&nSent)
	db.QueryRow("SELECT COUNT(*) FROM nurture_log WHERE status='failed'").Scan(&nFailed)
	var nCaptures int
	db.QueryRow("SELECT COUNT(DISTINCT email) FROM exchange_gate_captures").Scan(&nCaptures)
	nurture["emails_sent"] = nSent
	nurture["emails_failed"] = nFailed
	nurture["leads"] = nCaptures
	result["nurture"] = nurture

	// ── 5. PROXY USAGE (auto-tracked) ──
	proxy := map[string]any{"source": "auto"}
	var totalReqs int64
	var totalCost float64
	db.QueryRow("SELECT COALESCE(COUNT(*),0), COALESCE(SUM(cost_usd),0) FROM requests").Scan(&totalReqs, &totalCost)
	proxy["total_requests"] = totalReqs
	proxy["total_cost_usd"] = totalCost

	var periodReqs int64
	var periodCost float64
	db.QueryRow("SELECT COALESCE(COUNT(*),0), COALESCE(SUM(cost_usd),0) FROM requests WHERE timestamp >= ?", since).Scan(&periodReqs, &periodCost)
	proxy["period_requests"] = periodReqs
	proxy["period_cost_usd"] = periodCost
	result["proxy"] = proxy

	// ── 6. MANUAL/IMPORTED METRICS (from growth_metrics table) ──
	// Group by category
	manual := map[string][]map[string]any{}
	mRows, err := db.Query(`
		SELECT date, category, metric, value, source, notes
		FROM growth_metrics WHERE date >= ?
		ORDER BY category, date DESC`, since)
	if err == nil {
		defer mRows.Close()
		for mRows.Next() {
			var date, cat, metric, src, notes string
			var val float64
			mRows.Scan(&date, &cat, &metric, &val, &src, &notes)
			manual[cat] = append(manual[cat], map[string]any{
				"date": date, "metric": metric, "value": val,
				"source": src, "notes": notes,
			})
		}
	}

	// Build ad sections from manual data
	result["google_ads"] = buildAdSection(manual["google_ads"], "manual")
	result["reddit_ads"] = buildAdSection(manual["reddit_ads"], "manual")
	result["github"] = buildGitHubSection(manual["github"], "manual")
	result["revenue"] = buildRevenueSection(manual["revenue"], db)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func buildAdSection(entries []map[string]any, src string) map[string]any {
	section := map[string]any{"source": src, "entries": entries}
	if entries == nil {
		section["entries"] = []map[string]any{}
	}
	// Aggregate totals
	var spend, impressions, clicks float64
	for _, e := range entries {
		m, _ := e["metric"].(string)
		v, _ := e["value"].(float64)
		switch m {
		case "spend":
			spend += v
		case "impressions":
			impressions += v
		case "clicks":
			clicks += v
		}
	}
	section["total_spend"] = spend
	section["total_impressions"] = impressions
	section["total_clicks"] = clicks
	if impressions > 0 {
		section["ctr"] = clicks / impressions * 100
	}
	if clicks > 0 {
		section["cpc"] = spend / clicks
	}
	return section
}

func buildGitHubSection(entries []map[string]any, src string) map[string]any {
	section := map[string]any{"source": src}
	if entries == nil {
		section["entries"] = []map[string]any{}
		return section
	}
	section["entries"] = entries
	// Find latest values
	for _, e := range entries {
		m, _ := e["metric"].(string)
		v, _ := e["value"].(float64)
		switch m {
		case "stars":
			section["stars"] = v
		case "clones":
			section["clones"] = v
		case "visitors":
			section["visitors"] = v
		case "release_downloads":
			section["release_downloads"] = v
		}
	}
	return section
}

func buildRevenueSection(entries []map[string]any, db *sql.DB) map[string]any {
	section := map[string]any{"source": "mixed"}

	// Auto: count users by tier
	tierCounts := map[string]int{}
	rows, _ := db.Query("SELECT tier, COUNT(*) FROM users GROUP BY tier")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var tier string
			var c int
			rows.Scan(&tier, &c)
			tierCounts[tier] = c
		}
	}
	section["users_by_tier"] = tierCounts

	// Manual revenue data from growth_metrics
	if entries != nil {
		for _, e := range entries {
			m, _ := e["metric"].(string)
			v, _ := e["value"].(float64)
			switch m {
			case "mrr":
				section["mrr"] = v
			case "total_revenue":
				section["total_revenue"] = v
			case "new_customers":
				section["new_customers"] = v
			}
		}
	}
	section["entries"] = entries
	if entries == nil {
		section["entries"] = []map[string]any{}
	}
	return section
}

// handleGrowthMetrics lists all manual metrics.
func handleGrowthMetrics(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	rows, err := db.Query(`SELECT id, date, category, metric, value, source, notes, updated_at FROM growth_metrics ORDER BY date DESC, category, metric LIMIT 500`)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"metrics": []any{}, "error": err.Error()})
		return
	}
	defer rows.Close()

	var metrics []map[string]any
	for rows.Next() {
		var id int
		var date, cat, metric, src, notes, updated string
		var val float64
		rows.Scan(&id, &date, &cat, &metric, &val, &src, &notes, &updated)
		metrics = append(metrics, map[string]any{
			"id": id, "date": date, "category": cat, "metric": metric,
			"value": val, "source": src, "notes": notes, "updated_at": updated,
		})
	}
	if metrics == nil {
		metrics = []map[string]any{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"metrics": metrics, "count": len(metrics)})
}

// handleGrowthMetricUpsert creates or updates a manual metric.
func handleGrowthMetricUpsert(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var req struct {
		Date     string  `json:"date"`
		Category string  `json:"category"`
		Metric   string  `json:"metric"`
		Value    float64 `json:"value"`
		Source   string  `json:"source"`
		Notes    string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}
	if req.Category == "" || req.Metric == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "category and metric are required"})
		return
	}
	if req.Source == "" {
		req.Source = "manual"
	}

	_, err := db.Exec(`
		INSERT INTO growth_metrics (date, category, metric, value, source, notes, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(date, category, metric) DO UPDATE SET
			value = excluded.value, source = excluded.source,
			notes = excluded.notes, updated_at = datetime('now')`,
		req.Date, req.Category, req.Metric, req.Value, req.Source, req.Notes)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

// handleGrowthMetricDelete removes a manual metric.
func handleGrowthMetricDelete(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	id := r.PathValue("id")
	res, err := db.Exec("DELETE FROM growth_metrics WHERE id = ?", id)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
