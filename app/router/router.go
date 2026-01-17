package router

import (
	"net/http"

	"github.com/ashanmugaraja/cronzee/app/handler"
	"github.com/ashanmugaraja/cronzee/app/models"
	"github.com/ashanmugaraja/cronzee/app/structs"
	"github.com/ashanmugaraja/cronzee/app/views"
	"github.com/ashanmugaraja/cronzee/app/worker"
)

// Router handles HTTP routing
type Router struct {
	mux           *http.ServeMux
	healthHandler *handler.HealthHandler
}

// NewRouter creates a new router
func NewRouter(monitor *worker.Monitor, db *models.Database, config *structs.Config) *Router {
	router := &Router{
		mux:           http.NewServeMux(),
		healthHandler: handler.NewHealthHandler(monitor, db, config),
	}

	router.setupRoutes()
	return router
}

// setupRoutes configures all application routes
func (r *Router) setupRoutes() {
	// API endpoints matching original server.go
	r.mux.HandleFunc("/api/status", r.healthHandler.GetStatus)
	r.mux.HandleFunc("/api/endpoints", r.healthHandler.GetEndpoints)
	r.mux.HandleFunc("/api/endpoints/add", r.healthHandler.AddEndpoint)
	r.mux.HandleFunc("/api/endpoints/delete", r.healthHandler.DeleteEndpoint)
	r.mux.HandleFunc("/api/endpoints/enable", r.healthHandler.EnableEndpoint)
	r.mux.HandleFunc("/api/endpoints/disable", r.healthHandler.DisableEndpoint)
	r.mux.HandleFunc("/api/endpoints/suppress", r.healthHandler.SuppressAlerts)
	r.mux.HandleFunc("/api/endpoints/unsuppress", r.healthHandler.UnsuppressAlerts)
	r.mux.HandleFunc("/api/history", r.healthHandler.GetHistory)
	r.mux.HandleFunc("/api/endpoints/update", r.healthHandler.UpdateEndpoint)
	r.mux.HandleFunc("/api/expiring-certs", r.healthHandler.GetExpiringCerts)
	r.mux.HandleFunc("/api/config", r.healthHandler.GetConfig)
	r.mux.HandleFunc("/api/verify-passkey", r.healthHandler.VerifyPasskey)
	r.mux.HandleFunc("/api/endpoints/enable-health", r.healthHandler.EnableHealthMonitoring)

	// ✅ NEW: Manual SSL recheck
	r.mux.HandleFunc("/api/ssl/recheck", r.healthHandler.ReRunSSLCheck)

	// Static files
	r.mux.HandleFunc("/static/app.js", r.serveJS)

	// Root endpoint serves the dashboard
	r.mux.HandleFunc("/", r.serveDashboard)
}

// serveDashboard serves the main dashboard HTML
func (r *Router) serveDashboard(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(views.DashboardHTML))
}

// serveJS serves the JavaScript file
func (r *Router) serveJS(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(views.AppJS))
}

// ServeHTTP implements http.Handler interface
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
