package danbooru

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
)

// GetCredentials retrieves the Danbooru username and API key,
// checking the database first and falling back to environment variables.
func GetCredentials(db *sql.DB) (string, string) {
	var username, apiKey string
	if db != nil {
		_ = db.QueryRow("SELECT value FROM app_settings WHERE key = 'danbooru_username'").Scan(&username)
		_ = db.QueryRow("SELECT value FROM app_settings WHERE key = 'danbooru_api_key'").Scan(&apiKey)
	}
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	if apiKey == "" {
		apiKey = os.Getenv("DANBOORU_KEY")
	}
	return username, apiKey
}

// SaveCredentials persists the Danbooru username and API key to both
// the database and the .env file.
func SaveCredentials(db *sql.DB, username, apiKey string) error {
	_, err := db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES ('danbooru_username', ?, CURRENT_TIMESTAMP) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP", username)
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES ('danbooru_api_key', ?, CURRENT_TIMESTAMP) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP", apiKey)
	if err != nil {
		return err
	}

	os.Setenv("USERNAME", username)
	os.Setenv("DANBOORU_KEY", apiKey)
	updateEnvFile("USERNAME", username)
	updateEnvFile("DANBOORU_KEY", apiKey)
	return nil
}

func updateEnvFile(key, value string) {
	envPath := "./.env"
	content, err := os.ReadFile(envPath)
	lines := []string{}
	if err == nil {
		lines = strings.Split(string(content), "\n")
	}
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
			lines[i] = fmt.Sprintf("%s=%s", key, value)
			found = true
			break
		}
	}
	if !found {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines[len(lines)-1] = fmt.Sprintf("%s=%s", key, value)
			lines = append(lines, "")
		} else {
			lines = append(lines, fmt.Sprintf("%s=%s", key, value))
		}
	}
	_ = os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0644)
}
