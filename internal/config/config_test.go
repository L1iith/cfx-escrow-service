package config

import "testing"

func TestAdditionalRepositories(t *testing.T) {
	t.Setenv("API_SECRET", "secret")
	t.Setenv("SOURCE_REPOSITORY", "owner/server")
	t.Setenv("SOURCE_REPOSITORY_URL", "https://github.com/owner/server.git")
	t.Setenv("CFX_FORUM_COOKIE", "cookie")
	t.Setenv("ADDITIONAL_REPOSITORIES_JSON", `{"owner/assets":{"url":"https://github.com/owner/assets.git","resource_root":".","mirror_repository":"owner/assets-escrowed"}}`)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	repository, exists := cfg.RepositoryConfig("owner/assets")
	if !exists {
		t.Fatal("expected additional repository")
	}
	if repository.Branch != "main" || repository.ResourceRoot != "." || repository.MirrorBranch != "main" {
		t.Fatal("unexpected repository defaults")
	}
}
