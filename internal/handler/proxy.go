package handler

import (
	"io"
	"net/http"
	"time"
)

// proxyImageHandler proxies image requests to external URLs (e.g., Danbooru CDN).
func proxyImageHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "Missing URL parameter", http.StatusBadRequest)
		return
	}

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Danbooru requires a User-Agent, otherwise they block the request
	req.Header.Set("User-Agent", "MyGoApp/1.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to fetch image", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Pass the content type (e.g., image/jpeg) and the image bytes back to the frontend
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
