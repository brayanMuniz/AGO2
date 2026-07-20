package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/brayanMuniz/AGO2/internal/models"
	"github.com/brayanMuniz/AGO2/internal/storage"
)

// GET /api/stats
func (a *App) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse optional query params
	tagLimit := 0
	if v := r.URL.Query().Get("tag_limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			tagLimit = parsed
		}
	}

	predictiveLimit := 0
	if v := r.URL.Query().Get("predictive_limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			predictiveLimit = parsed
		}
	}

	minCount := 5
	if v := r.URL.Query().Get("min_count"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			minCount = parsed
		}
	}

	artistLimit := 15
	if v := r.URL.Query().Get("artist_limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			artistLimit = parsed
		}
	}

	// Gather all stats
	library, err := storage.GetLibraryStats(a.DB)
	if err != nil {
		sendJSONError(w, "Failed to get library stats", http.StatusInternalServerError)
		fmt.Printf("Error getting library stats: %v\n", err)
		return
	}

	categories := []string{"artist", "character", "copyright", "general"}
	leaderboards := make(map[string][]models.TagLeaderboardEntry)
	leaderboardsFav := make(map[string][]models.TagLeaderboardEntry)
	for _, cat := range categories {
		entries, err := storage.GetTagLeaderboard(a.DB, cat, tagLimit)
		if err != nil {
			sendJSONError(w, "Failed to get tag leaderboard", http.StatusInternalServerError)
			fmt.Printf("Error getting %s leaderboard: %v\n", cat, err)
			return
		}
		leaderboards[cat] = entries

		favEntries, err := storage.GetTagLeaderboardByFavorites(a.DB, cat, tagLimit)
		if err != nil {
			sendJSONError(w, "Failed to get favorites tag leaderboard", http.StatusInternalServerError)
			fmt.Printf("Error getting %s favorites leaderboard: %v\n", cat, err)
			return
		}
		leaderboardsFav[cat] = favEntries
	}

	ratingDist, err := storage.GetRatingDistribution(a.DB)
	if err != nil {
		sendJSONError(w, "Failed to get rating distribution", http.StatusInternalServerError)
		fmt.Printf("Error getting rating distribution: %v\n", err)
		return
	}

	predictiveByRating, err := storage.GetPredictiveTagAnalytics(a.DB, minCount, predictiveLimit)
	if err != nil {
		sendJSONError(w, "Failed to get predictive analytics", http.StatusInternalServerError)
		fmt.Printf("Error getting predictive analytics: %v\n", err)
		return
	}

	artistProfiles, err := storage.GetArtistProfiles(a.DB, artistLimit)
	if err != nil {
		sendJSONError(w, "Failed to get artist profiles", http.StatusInternalServerError)
		fmt.Printf("Error getting artist profiles: %v\n", err)
		return
	}

	payload := models.StatsPayload{
		Library:            library,
		TagLeaderboards:    leaderboards,
		TagLeaderboardsFav: leaderboardsFav,
		RatingDist:         ratingDist,
		PredictiveByRating: predictiveByRating,
		ArtistProfiles:     artistProfiles,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}
