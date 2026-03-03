package models

import "path/filepath"

type Repo struct {
	Path          string `json:"path" yaml:"path"`
	DefaultBranch string `json:"defaultBranch" yaml:"default_branch"`
	Name          string `json:"name" yaml:"name"`
}

// DisplayName returns the repo name, falling back to the directory name.
func (r *Repo) DisplayName() string {
	if r.Name != "" {
		return r.Name
	}
	return filepath.Base(r.Path)
}
