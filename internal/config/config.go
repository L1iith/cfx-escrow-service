package config

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddress    string
	APISecret        string
	DataDirectory    string
	Repository       string
	RepositoryURL    string
	Branch           string
	ResourceRoot     string
	UploaderBinary   string
	UploaderArgs     []string
	MirrorRepository string
	MirrorBranch     string
	MirrorToken      string
	CFXForumCookie   string
	GitAuthorName    string
	GitAuthorEmail   string
	JobTimeout       time.Duration
	MaxBodyBytes     int64
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddress:    value("LISTEN_ADDRESS", "127.0.0.1:8080"),
		DataDirectory:    value("DATA_DIRECTORY", "/var/lib/cfx-escrow-service"),
		Branch:           value("SOURCE_BRANCH", "main"),
		ResourceRoot:     value("RESOURCE_ROOT", "server-files/resources"),
		UploaderBinary:   value("UPLOADER_BINARY", "node"),
		MirrorBranch:     value("MIRROR_BRANCH", "main"),
		GitAuthorName:    value("GIT_AUTHOR_NAME", "cfx-escrow-service"),
		GitAuthorEmail:   value("GIT_AUTHOR_EMAIL", "cfx-escrow-service@users.noreply.github.com"),
		JobTimeout:       duration("JOB_TIMEOUT", 3*time.Hour),
		MaxBodyBytes:     int64Value("MAX_BODY_BYTES", 1<<20),
		APISecret:        strings.TrimSpace(os.Getenv("API_SECRET")),
		Repository:       strings.TrimSpace(os.Getenv("SOURCE_REPOSITORY")),
		RepositoryURL:    strings.TrimSpace(os.Getenv("SOURCE_REPOSITORY_URL")),
		MirrorRepository: strings.TrimSpace(os.Getenv("MIRROR_REPOSITORY")),
		MirrorToken:      strings.TrimSpace(os.Getenv("MIRROR_TOKEN")),
		CFXForumCookie:   strings.TrimSpace(os.Getenv("CFX_FORUM_COOKIE")),
	}

	rawArgs := value("UPLOADER_ARGS_JSON", `["/opt/cfx-escrow-bot/src/cli-escrow.js"]`)
	if err := json.Unmarshal([]byte(rawArgs), &cfg.UploaderArgs); err != nil {
		return Config{}, errors.New("UPLOADER_ARGS_JSON must be a JSON string array")
	}

	required := map[string]string{
		"API_SECRET":            cfg.APISecret,
		"SOURCE_REPOSITORY":     cfg.Repository,
		"SOURCE_REPOSITORY_URL": cfg.RepositoryURL,
		"CFX_FORUM_COOKIE":      cfg.CFXForumCookie,
	}
	for name, current := range required {
		if current == "" {
			return Config{}, errors.New(name + " is required")
		}
	}
	if len(cfg.UploaderArgs) == 0 {
		return Config{}, errors.New("UPLOADER_ARGS_JSON cannot be empty")
	}
	return cfg, nil
}

func value(name, fallback string) string {
	current := strings.TrimSpace(os.Getenv(name))
	if current == "" {
		return fallback
	}
	return current
}

func duration(name string, fallback time.Duration) time.Duration {
	current := strings.TrimSpace(os.Getenv(name))
	if current == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(current)
	if err != nil {
		return fallback
	}
	return parsed
}

func int64Value(name string, fallback int64) int64 {
	current := strings.TrimSpace(os.Getenv(name))
	if current == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(current, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
