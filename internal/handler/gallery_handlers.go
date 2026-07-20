package handler

import (
	"encoding/json"
	"net/http"

	"github.com/brayanMuniz/AGO2/internal/danbooru"
	"github.com/brayanMuniz/AGO2/internal/gallery"
)

// POST /api/process-gallery
func (a *App) handleProcessGallery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userName, apiKey := danbooru.GetCredentials(a.DB)

	job := gallery.CreateJob()

	go gallery.RunGalleryWorker(a.DB, apiKey, userName, a.Cfg.GalleryDir, a.Cfg.ThumbnailDir, job)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted is standard for "started processing"
	json.NewEncoder(w).Encode(map[string]string{
		"job_id":  job.ID,
		"message": "Processing started in the background",
	})
}

// GET /api/process-gallery/status?job_id=xyz
func (a *App) handleGetJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		sendJSONError(w, "job_id is required", http.StatusBadRequest)
		return
	}

	job, exists := gallery.GetJob(jobID)

	if !exists {
		sendJSONError(w, "Job not found", http.StatusNotFound)
		return
	}

	job.RLock()
	defer job.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}
