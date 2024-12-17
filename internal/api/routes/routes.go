// internal/api/routes/routes.go
package routes

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/fawad-mazhar/naxos/internal/api/handlers"
	"github.com/fawad-mazhar/naxos/internal/config"
	"github.com/fawad-mazhar/naxos/internal/queue"
	"github.com/fawad-mazhar/naxos/internal/storage/postgres"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func SetupRouter(cfg *config.Config, db *postgres.Client, queue *queue.RabbitMQ) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})

	// Initialize handlers
	jobHandler := handlers.NewJobHandler(db, queue)
	statusHandler := handlers.NewStatusHandler(db)

	// Routes
	r.Route("/api/v1", func(r chi.Router) {
		// Job Definition endpoints
		r.Route("/job-definitions", func(r chi.Router) {
			r.Post("/", jobHandler.CreateJobDefinition)
			r.Get("/{id}", jobHandler.GetJobDefinition)
		})

		// Job Execution endpoints
		r.Route("/jobs", func(r chi.Router) {
			r.Post("/{id}/execute", jobHandler.ExecuteJob)
			r.Get("/{id}/status", jobHandler.GetJobStatus)
		})

		// System Status endpoint
		r.Get("/system/status", statusHandler.GetSystemStatus)
	})

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	return r
}
