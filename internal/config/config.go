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
	Repositories     map[string]Repository
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

type Repository struct {
	URL              string `json:"url"`
	Branch           string `json:"branch"`
	ResourceRoot     string `json:"resource_root"`
	MirrorRepository string `json:"mirror_repository"`
	MirrorBranch     string `json:"mirror_branch"`
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
	cfg.Repositories = map[string]Repository{
		cfg.Repository: {
			URL:              cfg.RepositoryURL,
			Branch:           cfg.Branch,
			ResourceRoot:     cfg.ResourceRoot,
			MirrorRepository: cfg.MirrorRepository,
			MirrorBranch:     cfg.MirrorBranch,
		},
	}
	if err := loadAdditionalRepositories(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadAdditionalRepositories(cfg *Config) error {
	raw := strings.TrimSpace(os.Getenv("ADDITIONAL_REPOSITORIES_JSON"))
	if raw == "" {
		return nil
	}
	var repositories map[string]Repository
	if err := json.Unmarshal([]byte(raw), &repositories); err != nil {
		return errors.New("ADDITIONAL_REPOSITORIES_JSON must be a JSON object")
	}
	for name, repository := range repositories {
		name = strings.TrimSpace(name)
		repository.URL = strings.TrimSpace(repository.URL)
		repository.Branch = strings.TrimSpace(repository.Branch)
		repository.ResourceRoot = strings.TrimSpace(repository.ResourceRoot)
		repository.MirrorRepository = strings.TrimSpace(repository.MirrorRepository)
		repository.MirrorBranch = strings.TrimSpace(repository.MirrorBranch)
		if name == "" || repository.URL == "" {
			return errors.New("additional repository names and URLs are required")
		}
		if _, exists := cfg.Repositories[name]; exists {
			return errors.New("additional repository duplicates the primary repository")
		}
		if repository.Branch == "" {
			repository.Branch = "main"
		}
		if repository.ResourceRoot == "" {
			repository.ResourceRoot = "."
		}
		if repository.MirrorBranch == "" {
			repository.MirrorBranch = "main"
		}
		cfg.Repositories[name] = repository
	}
	return nil
}

func (c Config) RepositoryConfig(name string) (Repository, bool) {
	if repository, exists := c.Repositories[name]; exists {
		return repository, true
	}
	if name != c.Repository {
		return Repository{}, false
	}
	return Repository{
		URL:              c.RepositoryURL,
		Branch:           c.Branch,
		ResourceRoot:     c.ResourceRoot,
		MirrorRepository: c.MirrorRepository,
		MirrorBranch:     c.MirrorBranch,
	}, true
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
