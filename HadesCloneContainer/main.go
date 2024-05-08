package main

import (
	"container/heap"
	"os"
	"path"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	log "github.com/sirupsen/logrus"
)

var dir = "/opt/repositories/"

func isValidPath(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

func main() {
	if os.Getenv("DEBUG") == "true" {
		log.SetLevel(log.DebugLevel)
	}

	if env_dir := os.Getenv("REPOSITORY_DIR"); env_dir != "" && isValidPath(env_dir) {
		dir = env_dir
	}
	log.Info("Starting HadesCloneContainer")
	log.Info("Cloning repositories to ", dir)

	repo_map := getReposFromEnv()
	repos := getReposFromMap(repo_map)
	for repos.Len() > 0 {
		repo := heap.Pop(&repos).(*Item).Repository
		log.Debugf("Cloning repository: %+v", repo)
		repodir := dir
		// URL is mandatory
		if repo.URL == "" {
			log.Warn("Skipping repository without URL")
			continue
		}
		clone_options := &git.CloneOptions{
			URL: repo.URL,
		}
		// Check if username and password are set and use them for authentication
		if repo.Username != "" && repo.Password != "" {
			clone_options.Auth = &http.BasicAuth{
				Username: repo.Username,
				Password: repo.Password,
			}
		}
		// Check if a branch is specified
		if repo.Branch != "" {
			clone_options.ReferenceName = plumbing.ReferenceName("refs/heads/" + repo.Branch)
		}

		if repo.Path != "" {
			repodir = path.Join(repodir, repo.Path)
		} else {
			parts := strings.Split(repo.URL, "/")
			if len(parts) > 0 {
				repoName := strings.TrimSuffix(parts[len(parts)-1], ".git")
				repodir = path.Join(repodir, repoName)
			}
		}

		// Execute the clone operation
		_, err := git.PlainClone(repodir, false, clone_options)
		if err != nil {
			log.WithError(err).Error("Failed to clone repository")
			continue
		}
		log.Infof("Cloned repository %s to %s", repo.URL, repodir)
	}
}
