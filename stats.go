package main

import (
	"database/sql"
	"fmt"
	"sort"
)

// --- Structs ---

type LibraryStats struct {
	TotalImages      int   `json:"total_images"`
	TotalDuplicates  int   `json:"total_duplicates"`
	TotalFavorites   int   `json:"total_favorites"`
	TotalDiskSpace   int64 `json:"total_disk_space"`
	UnorganizedQueue int   `json:"unorganized_queue"`
}

type TagLeaderboardEntry struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Count    int    `json:"count"`
}

type RatingDistribution struct {
	Rating string `json:"rating"`
	Count  int    `json:"count"`
}

type PredictiveTagEntry struct {
	Name            string  `json:"name"`
	TotalCount      int     `json:"total_count"`
	GeneralPct      float64 `json:"general_pct"`
	SensitivePct    float64 `json:"sensitive_pct"`
	QuestionablePct float64 `json:"questionable_pct"`
	ExplicitPct     float64 `json:"explicit_pct"`
}

type ArtistProfile struct {
	Name            string                `json:"name"`
	TotalCount      int                   `json:"total_count"`
	FavoriteCount   int                   `json:"favorite_count"`
	RatingBreakdown map[string]int        `json:"rating_breakdown"`
	TopTags         []TagLeaderboardEntry `json:"top_tags"`
}

type StatsPayload struct {
	Library            LibraryStats                     `json:"library"`
	TagLeaderboards    map[string][]TagLeaderboardEntry `json:"tag_leaderboards"`
	TagLeaderboardsFav map[string][]TagLeaderboardEntry `json:"tag_leaderboards_favorites"`
	RatingDist         []RatingDistribution             `json:"rating_distribution"`
	PredictiveByRating map[string][]PredictiveTagEntry  `json:"predictive_by_rating"`
	ArtistProfiles     []ArtistProfile                  `json:"artist_profiles"`
}

// --- Query Functions ---

func GetLibraryStats(db *sql.DB) (LibraryStats, error) {
	var stats LibraryStats

	query := `
		SELECT
			COALESCE(SUM(CASE WHEN hasDuplicate IS NULL THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN hasDuplicate IS NOT NULL THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN isFavorite = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(file_size), 0),
			COALESCE(SUM(CASE WHEN active_metadata_id IS NULL THEN 1 ELSE 0 END), 0)
		FROM files
	`

	err := db.QueryRow(query).Scan(
		&stats.TotalImages,
		&stats.TotalDuplicates,
		&stats.TotalFavorites,
		&stats.TotalDiskSpace,
		&stats.UnorganizedQueue,
	)
	if err != nil {
		return stats, fmt.Errorf("failed to get library stats: %w", err)
	}

	return stats, nil
}

func GetTagLeaderboard(db *sql.DB, category string, limit int) ([]TagLeaderboardEntry, error) {
	query := `
		SELECT t.name, t.category, COUNT(*) as cnt
		FROM tags t
		JOIN record_tags rt ON t.id = rt.tag_id
		JOIN metadata_records m ON rt.metadata_id = m.id
		JOIN files f ON f.active_metadata_id = m.id
		WHERE t.category = ?
		GROUP BY t.id
		ORDER BY cnt DESC
	`

	var args []interface{}
	args = append(args, category)

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag leaderboard for %s: %w", category, err)
	}
	defer rows.Close()

	var entries []TagLeaderboardEntry
	for rows.Next() {
		var e TagLeaderboardEntry
		if err := rows.Scan(&e.Name, &e.Category, &e.Count); err != nil {
			return nil, fmt.Errorf("failed to scan tag leaderboard entry: %w", err)
		}
		entries = append(entries, e)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tag leaderboard rows: %w", err)
	}

	if entries == nil {
		entries = make([]TagLeaderboardEntry, 0)
	}

	return entries, nil
}

func GetTagLeaderboardByFavorites(db *sql.DB, category string, limit int) ([]TagLeaderboardEntry, error) {
	query := `
		SELECT t.name, t.category, COUNT(*) as cnt
		FROM tags t
		JOIN record_tags rt ON t.id = rt.tag_id
		JOIN metadata_records m ON rt.metadata_id = m.id
		JOIN files f ON f.active_metadata_id = m.id
		WHERE t.category = ? AND f.isFavorite = 1
		GROUP BY t.id
		ORDER BY cnt DESC
	`

	var args []interface{}
	args = append(args, category)

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get favorites tag leaderboard for %s: %w", category, err)
	}
	defer rows.Close()

	var entries []TagLeaderboardEntry
	for rows.Next() {
		var e TagLeaderboardEntry
		if err := rows.Scan(&e.Name, &e.Category, &e.Count); err != nil {
			return nil, fmt.Errorf("failed to scan favorites tag leaderboard entry: %w", err)
		}
		entries = append(entries, e)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating favorites tag leaderboard rows: %w", err)
	}

	if entries == nil {
		entries = make([]TagLeaderboardEntry, 0)
	}

	return entries, nil
}

func GetRatingDistribution(db *sql.DB) ([]RatingDistribution, error) {
	query := `
		SELECT COALESCE(m.rating, 'unknown') as rating, COUNT(*) as cnt
		FROM files f
		JOIN metadata_records m ON f.active_metadata_id = m.id
		GROUP BY m.rating
		ORDER BY cnt DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get rating distribution: %w", err)
	}
	defer rows.Close()

	var dist []RatingDistribution
	for rows.Next() {
		var d RatingDistribution
		if err := rows.Scan(&d.Rating, &d.Count); err != nil {
			return nil, fmt.Errorf("failed to scan rating distribution: %w", err)
		}
		dist = append(dist, d)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rating distribution rows: %w", err)
	}

	if dist == nil {
		dist = make([]RatingDistribution, 0)
	}

	return dist, nil
}

func GetPredictiveTagAnalytics(db *sql.DB, minCount int, limit int) (map[string][]PredictiveTagEntry, error) {
	query := `
		SELECT
			t.name,
			COUNT(*) as total,
			ROUND(100.0 * SUM(CASE WHEN m.rating = 'g' THEN 1 ELSE 0 END) / COUNT(*), 1) as general_pct,
			ROUND(100.0 * SUM(CASE WHEN m.rating = 's' THEN 1 ELSE 0 END) / COUNT(*), 1) as sensitive_pct,
			ROUND(100.0 * SUM(CASE WHEN m.rating = 'q' THEN 1 ELSE 0 END) / COUNT(*), 1) as questionable_pct,
			ROUND(100.0 * SUM(CASE WHEN m.rating = 'e' THEN 1 ELSE 0 END) / COUNT(*), 1) as explicit_pct
		FROM tags t
		JOIN record_tags rt ON t.id = rt.tag_id
		JOIN metadata_records m ON rt.metadata_id = m.id
		JOIN files f ON f.active_metadata_id = m.id
		WHERE t.category = 'general'
		GROUP BY t.id
		HAVING COUNT(*) >= ?
	`

	rows, err := db.Query(query, minCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get predictive tag analytics: %w", err)
	}
	defer rows.Close()

	var allTags []PredictiveTagEntry
	for rows.Next() {
		var e PredictiveTagEntry
		if err := rows.Scan(&e.Name, &e.TotalCount, &e.GeneralPct, &e.SensitivePct, &e.QuestionablePct, &e.ExplicitPct); err != nil {
			return nil, fmt.Errorf("failed to scan predictive tag entry: %w", err)
		}
		allTags = append(allTags, e)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating predictive tag rows: %w", err)
	}

	result := make(map[string][]PredictiveTagEntry)

	ratings := []struct {
		key  string
		less func(slice []PredictiveTagEntry, i, j int) bool
	}{
		{
			key: "g",
			less: func(slice []PredictiveTagEntry, i, j int) bool {
				if slice[i].GeneralPct == slice[j].GeneralPct {
					return slice[i].TotalCount > slice[j].TotalCount
				}
				return slice[i].GeneralPct > slice[j].GeneralPct
			},
		},
		{
			key: "s",
			less: func(slice []PredictiveTagEntry, i, j int) bool {
				if slice[i].SensitivePct == slice[j].SensitivePct {
					return slice[i].TotalCount > slice[j].TotalCount
				}
				return slice[i].SensitivePct > slice[j].SensitivePct
			},
		},
		{
			key: "q",
			less: func(slice []PredictiveTagEntry, i, j int) bool {
				if slice[i].QuestionablePct == slice[j].QuestionablePct {
					return slice[i].TotalCount > slice[j].TotalCount
				}
				return slice[i].QuestionablePct > slice[j].QuestionablePct
			},
		},
		{
			key: "e",
			less: func(slice []PredictiveTagEntry, i, j int) bool {
				if slice[i].ExplicitPct == slice[j].ExplicitPct {
					return slice[i].TotalCount > slice[j].TotalCount
				}
				return slice[i].ExplicitPct > slice[j].ExplicitPct
			},
		},
	}

	for _, r := range ratings {
		list := make([]PredictiveTagEntry, len(allTags))
		copy(list, allTags)
		sort.Slice(list, func(i, j int) bool {
			return r.less(list, i, j)
		})
		if limit > 0 && len(list) > limit {
			list = list[:limit]
		}
		result[r.key] = list
	}

	return result, nil
}

func GetArtistProfiles(db *sql.DB, limit int) ([]ArtistProfile, error) {
	// Step 1: Get top artists by total image count
	topArtists, err := GetTagLeaderboard(db, "artist", limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top artists: %w", err)
	}

	if len(topArtists) == 0 {
		return make([]ArtistProfile, 0), nil
	}

	profiles := make([]ArtistProfile, 0, len(topArtists))

	for _, artist := range topArtists {
		profile := ArtistProfile{
			Name:            artist.Name,
			TotalCount:      artist.Count,
			RatingBreakdown: make(map[string]int),
		}

		// Step 2: Rating breakdown for this artist
		ratingQuery := `
			SELECT COALESCE(m.rating, 'unknown') as rating, COUNT(*) as cnt
			FROM tags t
			JOIN record_tags rt ON t.id = rt.tag_id
			JOIN metadata_records m ON rt.metadata_id = m.id
			JOIN files f ON f.active_metadata_id = m.id
			WHERE t.name = ? AND t.category = 'artist'
			GROUP BY m.rating
		`
		ratingRows, err := db.Query(ratingQuery, artist.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get rating breakdown for artist %s: %w", artist.Name, err)
		}
		for ratingRows.Next() {
			var rating string
			var count int
			if err := ratingRows.Scan(&rating, &count); err != nil {
				ratingRows.Close()
				return nil, fmt.Errorf("failed to scan rating breakdown: %w", err)
			}
			profile.RatingBreakdown[rating] = count
		}
		ratingRows.Close()

		// Step 3: Favorite count for this artist
		favQuery := `
			SELECT COUNT(*)
			FROM tags t
			JOIN record_tags rt ON t.id = rt.tag_id
			JOIN metadata_records m ON rt.metadata_id = m.id
			JOIN files f ON f.active_metadata_id = m.id
			WHERE t.name = ? AND t.category = 'artist' AND f.isFavorite = 1
		`
		err = db.QueryRow(favQuery, artist.Name).Scan(&profile.FavoriteCount)
		if err != nil {
			profile.FavoriteCount = 0
		}

		// Step 4: Top 5 co-occurring general tags for this artist
		topTagsQuery := `
			SELECT t2.name, t2.category, COUNT(*) as cnt
			FROM tags t1
			JOIN record_tags rt1 ON t1.id = rt1.tag_id
			JOIN metadata_records m ON rt1.metadata_id = m.id
			JOIN files f ON f.active_metadata_id = m.id
			JOIN record_tags rt2 ON m.id = rt2.metadata_id
			JOIN tags t2 ON rt2.tag_id = t2.id
			WHERE t1.name = ? AND t1.category = 'artist' AND t2.category = 'general'
			GROUP BY t2.id
			ORDER BY cnt DESC
			LIMIT 5
		`
		tagRows, err := db.Query(topTagsQuery, artist.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get top tags for artist %s: %w", artist.Name, err)
		}
		var topTags []TagLeaderboardEntry
		for tagRows.Next() {
			var e TagLeaderboardEntry
			if err := tagRows.Scan(&e.Name, &e.Category, &e.Count); err != nil {
				tagRows.Close()
				return nil, fmt.Errorf("failed to scan top tag: %w", err)
			}
			topTags = append(topTags, e)
		}
		tagRows.Close()

		if topTags == nil {
			topTags = make([]TagLeaderboardEntry, 0)
		}
		profile.TopTags = topTags

		profiles = append(profiles, profile)
	}

	return profiles, nil
}
