package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Addr        string   `json:"server_address"`
	MaxTasks    int      `json:"max_tasks"`
	MaxFiles    int      `json:"max_files"`
	PrTimeout   string   `json:"processing_timeout"`
	DwnTimeout  string   `json:"download_timeout"`
	TempDir     string   `json:"temp_dir"`
	ArchiveDir  string   `json:"archive_dir"`
	AllowedExts []string `json:"allowed_exts"`
}

func (c *Config) MakeTimePr() (time.Duration, error) {
	if c.PrTimeout == "" {
		return 5 * time.Minute, nil
	}
	return time.ParseDuration(c.PrTimeout)
}

func (c *Config) MakeTimeDwn() (time.Duration, error) {
	if c.DwnTimeout == "" {
		return 30 * time.Millisecond, nil
	}
	return time.ParseDuration(c.DwnTimeout)
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Addr:        ":8080",
		MaxTasks:    3,
		MaxFiles:    3,
		PrTimeout:   "5m",
		DwnTimeout:  "30s",
		TempDir:     filepath.Join(os.TempDir(), "archive-service", "temp"),
		ArchiveDir:  filepath.Join(os.TempDir(), "archive-service", "archives"),
		AllowedExts: []string{".pdf", ".jpeg"},
	}

	if err = json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if err = os.MkdirAll(cfg.TempDir, 0755); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(cfg.ArchiveDir, 0755); err != nil {
		return nil, err
	}

	return cfg, nil
}
