package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Repo manages a local clone of a Git repository
type Repo struct {
	URL       string
	Branch    string
	LocalPath string
	repo      *gogit.Repository
}

// NewRepo creates a new Repo instance
func NewRepo(url, branch, localPath string) *Repo {
	return &Repo{
		URL:       url,
		Branch:    branch,
		LocalPath: localPath,
	}
}

// Clone clones the repository if it doesn't exist locally
func (r *Repo) Clone() error {
	if _, err := os.Stat(r.LocalPath); err == nil {
		// Already exists — open it
		repo, err := gogit.PlainOpen(r.LocalPath)
		if err != nil {
			return fmt.Errorf("failed to open existing repo: %w", err)
		}
		r.repo = repo
		return nil
	}

	repo, err := gogit.PlainClone(r.LocalPath, false, &gogit.CloneOptions{
		URL:           r.URL,
		ReferenceName: plumbing.NewBranchReferenceName(r.Branch),
		SingleBranch:  true,
		Depth:         1,
	})
	if err != nil {
		return fmt.Errorf("failed to clone repo %s: %w", r.URL, err)
	}

	r.repo = repo
	return nil
}

// Pull fetches the latest changes from the remote
func (r *Repo) Pull() error {
	if r.repo == nil {
		return fmt.Errorf("repository not initialized, call Clone() first")
	}

	w, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	err = w.Pull(&gogit.PullOptions{
		ReferenceName: plumbing.NewBranchReferenceName(r.Branch),
		Force:         true,
	})
	if err != nil && err != gogit.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to pull: %w", err)
	}

	return nil
}

// GetManifests returns all YAML/JSON manifest file paths in the repo
func (r *Repo) GetManifests() ([]string, error) {
	var manifests []string

	err := filepath.Walk(r.LocalPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		// Only include YAML files
		if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			manifests = append(manifests, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk repo: %w", err)
	}

	return manifests, nil
}

// GetCurrentCommit returns the current HEAD commit hash
func (r *Repo) GetCurrentCommit() (string, error) {
	if r.repo == nil {
		return "", fmt.Errorf("repository not initialized")
	}

	ref, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	return ref.Hash().String(), nil
}