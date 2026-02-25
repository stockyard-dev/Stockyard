// Package platform provides the shared App interface and registration pattern
// for the 6 Stockyard flagship apps (Proxy, Observe, Trust, Studio, Forge, Exchange).
package platform

import (
	"database/sql"
	"net/http"
)

// App is the interface every Stockyard app implements.
type App interface {
	// Name returns the app identifier (proxy, observe, trust, studio, forge, exchange).
	Name() string

	// Description returns a short tagline for the app.
	Description() string

	// Migrate runs app-specific database migrations.
	Migrate(conn *sql.DB) error

	// RegisterRoutes mounts the app's HTTP handlers on the shared mux.
	// API routes go under /api/{name}/, UI data under /app/{name}/.
	RegisterRoutes(mux *http.ServeMux)
}

// Registry holds all registered apps.
type Registry struct {
	apps []App
}

// NewRegistry creates an empty app registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds an app to the registry.
func (r *Registry) Register(app App) {
	r.apps = append(r.apps, app)
}

// MigrateAll runs all app migrations.
func (r *Registry) MigrateAll(conn *sql.DB) error {
	for _, app := range r.apps {
		if err := app.Migrate(conn); err != nil {
			return err
		}
	}
	return nil
}

// RegisterAllRoutes mounts all app routes on the mux.
func (r *Registry) RegisterAllRoutes(mux *http.ServeMux) {
	for _, app := range r.apps {
		app.RegisterRoutes(mux)
	}
}

// Apps returns all registered apps.
func (r *Registry) Apps() []App {
	return r.apps
}

// AppList returns a summary of registered apps for the /api/apps endpoint.
func (r *Registry) AppList() []map[string]string {
	var list []map[string]string
	for _, app := range r.apps {
		list = append(list, map[string]string{
			"name":        app.Name(),
			"description": app.Description(),
			"api":         "/api/" + app.Name(),
		})
	}
	return list
}
