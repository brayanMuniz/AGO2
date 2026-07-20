package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/brayanMuniz/AGO2/internal/danbooru"
)

// GET /api/settings/danbooru
func (a *App) handleGetDanbooruSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	username, apiKey := danbooru.GetCredentials(a.DB)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"username": username,
		"api_key":  apiKey,
	})
}

// POST /api/settings/danbooru
func (a *App) handleSaveDanbooruSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		APIKey   string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := danbooru.SaveCredentials(a.DB, strings.TrimSpace(req.Username), strings.TrimSpace(req.APIKey)); err != nil {
		sendJSONError(w, fmt.Sprintf("Failed to save credentials: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Credentials saved successfully"})
}
