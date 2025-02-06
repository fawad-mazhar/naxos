// internal/api/handlers/job_handler.go
package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/fawad-mazhar/naxos/internal/models"
	"github.com/fawad-mazhar/naxos/internal/queue"
	"github.com/fawad-mazhar/naxos/internal/storage/postgres"
	"github.com/go-chi/chi/v5"
)

type JobHandler struct {
	db    *postgres.Client
	queue *queue.RabbitMQ
}

func NewJobHandler(db *postgres.Client, queue *queue.RabbitMQ) *JobHandler {
	return &JobHandler{
		db:    db,
		queue: queue,
	}
}

func (h *JobHandler) CreateJobDefinition(w http.ResponseWriter, r *http.Request) {
	var jobDef models.JobDefinition
	if err := json.NewDecoder(r.Body).Decode(&jobDef); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.db.StoreJobDefinition(r.Context(), &jobDef); err != nil {
		http.Error(w, "failed to store job definition", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Job definition created successfully",
		"id":      jobDef.ID,
	})
}

func (h *JobHandler) ExecuteJob(w http.ResponseWriter, r *http.Request) {
	definitionID := chi.URLParam(r, "id")

	// Parse optional execution data
	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		data = make(map[string]interface{})
	}

	// Create new job execution
	job := models.NewJobExecution(definitionID, data)

	// Store job execution
	if err := h.db.CreateJobExecution(r.Context(), job); err != nil {
		http.Error(w, "failed to create job execution", http.StatusInternalServerError)
		return
	}

	// Publish job to queue
	if err := h.queue.PublishJob(r.Context(), job); err != nil {
		http.Error(w, "failed to queue job", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"message":     "Job queued successfully",
		"executionId": job.ID,
	})
}

func (h *JobHandler) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	// Add debug logging
	executionID := chi.URLParam(r, "id")
	log.Printf("Getting status for job: %s", executionID)

	// Get job execution from database
	jobExec, taskExecs, err := h.db.GetJobExecutionDetails(r.Context(), executionID)
	if err != nil {
		log.Printf("Error getting job execution details: %v", err)
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	// Build response
	response := struct {
		*models.JobExecution
		Tasks []models.TaskExecution `json:"tasks"`
	}{
		JobExecution: jobExec,
		Tasks:        taskExecs,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *JobHandler) GetJobDefinition(w http.ResponseWriter, r *http.Request) {
	definitionID := chi.URLParam(r, "id")

	jobDef, err := h.db.GetJobDefinition(r.Context(), definitionID)
	if err != nil {
		http.Error(w, "job definition not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(jobDef)
}
