// internal/api/handlers/status_handler.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/fawad-mazhar/naxos/internal/storage/postgres"
)

type StatusHandler struct {
	db *postgres.Client
}

func NewStatusHandler(db *postgres.Client) *StatusHandler {
	return &StatusHandler{
		db: db,
	}
}

func (h *StatusHandler) GetSystemStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.db.GetSystemStatus(r.Context())
	if err != nil {
		http.Error(w, "failed to get system status", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(status)
}
