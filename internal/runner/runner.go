package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/L1iith/cfx-escrow-service/internal/config"
	"github.com/L1iith/cfx-escrow-service/internal/model"
)

var commitPattern = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

type Runner struct {
	cfg config.Config
}

func New(cfg config.Config) *Runner {
	return &Runner{cfg: cfg}
}

func (r *Runner) Process(ctx context.Context, job *model.Job, logf func(string)) ([]model.ResourceResult, error) {
	repository, exists := r.cfg.RepositoryConfig(job.Request.Repository)
	if !exists {
		return nil, errors.New("repository is not allowed")
	}
	if job.Request.Branch == "" {
		job.Request.Branch = repository.Branch
	}
	if err := r.validate(job.Request, repository); err != nil {
		return nil, err
	}
	workRoot := filepath.Join(r.cfg.DataDirectory, "work")
	if err := os.MkdirAll(workRoot, 0o750); err != nil {
		return nil, err
	}
	workDir, err := os.MkdirTemp(workRoot, "job-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workDir)

	if err := r.command(ctx, "", logf, "git", "clone", "--no-checkout", "--branch", job.Request.Branch, "--single-branch", repository.URL, workDir); err != nil {
		return nil, err
	}
	target := "origin/" + job.Request.Branch
	if job.Request.Commit != "" {
		target = job.Request.Commit
	}
	if err := r.command(ctx, "", logf, "git", "-C", workDir, "checkout", "--detach", target); err != nil {
		return nil, err
	}

	resourceDirs := make([]string, 0, len(job.Request.Resources))
	resourceMarkers := make([]string, 0, len(job.Request.Resources))
	for _, resource := range job.Request.Resources {
		relative, err := r.resourcePath(repository.ResourceRoot, resource)
		if err != nil {
			return nil, err
		}
		resourceDir := filepath.Join(workDir, relative)
		marker := filepath.Join(resourceDir, ".escrow")
		if stat, err := os.Stat(resourceDir); err != nil || !stat.IsDir() {
			return nil, fmt.Errorf("resource directory not found: %s", resource)
		}
		if _, err := os.Stat(marker); err != nil {
			return nil, fmt.Errorf("resource marker not found: %s", resource)
		}
		resourceDirs = append(resourceDirs, resourceDir)
		resourceMarkers = append(resourceMarkers, filepath.ToSlash(filepath.Join(relative, ".escrow")))
	}

	args := append([]string(nil), r.cfg.UploaderArgs...)
	if job.Request.Operation != model.OperationMirror {
		for _, resourceDir := range resourceDirs {
			args = append(args, "--resource", resourceDir)
		}
	}
	if job.Request.Operation != model.OperationUpload {
		if repository.MirrorRepository == "" || r.cfg.MirrorToken == "" {
			return nil, errors.New("mirror operation requested but mirror configuration is incomplete")
		}
		args = append(args,
			"--mirror-repo", repository.MirrorRepository,
			"--mirror-token", r.cfg.MirrorToken,
			"--mirror-branch", repository.MirrorBranch,
			"--workspace", workDir,
		)
	}

	env := append(os.Environ(), "CFX_FORUM_COOKIE="+r.cfg.CFXForumCookie)
	if err := r.commandEnv(ctx, workDir, env, logf, r.cfg.UploaderBinary, args...); err != nil {
		return nil, err
	}

	results := make([]model.ResourceResult, 0, len(resourceMarkers))
	for index, marker := range resourceMarkers {
		assetID, err := readAssetID(filepath.Join(workDir, filepath.FromSlash(marker)))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", job.Request.Resources[index], err)
		}
		results = append(results, model.ResourceResult{Path: job.Request.Resources[index], AssetID: assetID})
	}
	if len(resourceMarkers) > 0 {
		if err := r.commitMarkers(ctx, workDir, job.Request.Branch, resourceMarkers, logf); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (r *Runner) validate(request model.JobRequest, repository config.Repository) error {
	if request.Branch != repository.Branch {
		return errors.New("branch is not allowed")
	}
	if request.Commit != "" && !commitPattern.MatchString(request.Commit) {
		return errors.New("commit must be a full hexadecimal SHA")
	}
	switch request.Operation {
	case model.OperationUpload, model.OperationMirror, model.OperationUploadAndMirror:
	default:
		return errors.New("invalid operation")
	}
	if request.Operation != model.OperationMirror && len(request.Resources) == 0 {
		return errors.New("at least one resource is required")
	}
	return nil
}

func (r *Runner) resourcePath(resourceRoot, resource string) (string, error) {
	resource = filepath.Clean(filepath.FromSlash(strings.TrimSpace(resource)))
	if resource == "." || filepath.IsAbs(resource) || resource == ".." || strings.HasPrefix(resource, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid resource path")
	}
	root := filepath.Clean(filepath.FromSlash(resourceRoot))
	relative, err := filepath.Rel(root, resource)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("resource path must be inside %s", resourceRoot)
	}
	return resource, nil
}

func (r *Runner) commitMarkers(ctx context.Context, workDir, branch string, markers []string, logf func(string)) error {
	args := append([]string{"-C", workDir, "add", "--"}, markers...)
	if err := r.command(ctx, "", logf, "git", args...); err != nil {
		return err
	}
	check := exec.CommandContext(ctx, "git", "-C", workDir, "diff", "--cached", "--quiet")
	if err := check.Run(); err == nil {
		logf("no marker changes to commit")
		return nil
	}
	if err := r.command(ctx, "", logf, "git", "-C", workDir, "config", "user.name", r.cfg.GitAuthorName); err != nil {
		return err
	}
	if err := r.command(ctx, "", logf, "git", "-C", workDir, "config", "user.email", r.cfg.GitAuthorEmail); err != nil {
		return err
	}
	if err := r.command(ctx, "", logf, "git", "-C", workDir, "commit", "-m", "chore(escrow): update asset ids [skip ci]"); err != nil {
		return err
	}
	if err := r.command(ctx, "", logf, "git", "-C", workDir, "fetch", "origin", branch); err != nil {
		return err
	}
	if err := r.command(ctx, "", logf, "git", "-C", workDir, "rebase", "origin/"+branch); err != nil {
		return err
	}
	return r.command(ctx, "", logf, "git", "-C", workDir, "push", "origin", "HEAD:"+branch)
}

func (r *Runner) command(ctx context.Context, directory string, logf func(string), name string, args ...string) error {
	return r.commandEnv(ctx, directory, nil, logf, name, args...)
}

func (r *Runner) commandEnv(ctx context.Context, directory string, env []string, logf func(string), name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = directory
	if env != nil {
		cmd.Env = env
	}
	writer := &lineWriter{logf: logf}
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Run(); err != nil {
		writer.Flush()
		return fmt.Errorf("%s failed: %w", name, err)
	}
	writer.Flush()
	return nil
}

type lineWriter struct {
	mu   sync.Mutex
	data []byte
	logf func(string)
}

func (w *lineWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.data = append(w.data, data...)
	for {
		index := strings.IndexByte(string(w.data), '\n')
		if index < 0 {
			break
		}
		line := strings.TrimSpace(string(w.data[:index]))
		w.data = w.data[index+1:]
		if line != "" {
			w.logf(line)
		}
	}
	return len(data), nil
}

func (w *lineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	line := strings.TrimSpace(string(w.data))
	if line != "" {
		w.logf(line)
	}
	w.data = nil
}

func readAssetID(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 64<<10))
	if err != nil {
		return 0, err
	}
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return 0, errors.New("marker is empty after upload")
	}
	if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
		return id, nil
	}
	var marker struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(data, &marker); err != nil || marker.ID <= 0 {
		return 0, errors.New("marker does not contain a valid asset id")
	}
	return marker.ID, nil
}

var _ io.Writer = (*lineWriter)(nil)
