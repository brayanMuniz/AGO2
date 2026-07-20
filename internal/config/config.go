package config

type Config struct {
	GalleryDir          string
	ThumbnailDir        string
	AlbumsDir           string
	DBPath              string
	TagCategoryJSONPath string
}

func Default() *Config {
	return &Config{
		GalleryDir:          "./Gallery/",
		ThumbnailDir:        "thumbnails",
		AlbumsDir:           "./Albums/",
		DBPath:              "./gallery.db",
		TagCategoryJSONPath: "./ui/tag_to_category.json",
	}
}
