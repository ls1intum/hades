package main

import (
	"container/heap"
	"os"
	"path"
	"strconv"
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

// Returns a map of all repositories and their respective metadata
// The metadata is a map of key-value pairs
func getReposFromEnv() map[string]map[string]string {
	// Map to hold the environment variables
	repoVars := make(map[string]map[string]string)

	// Iterate over all environment variables
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		key := pair[0]
		value := pair[1]

		// Check if the environment variable starts with "HADES_"
		if strings.HasPrefix(key, "HADES_") {
			// Should be in the form HADES_<group>_<type>
			parts := strings.SplitN(key, "_", 3)
			if len(parts) < 3 {
				continue
			}
			group := parts[1]
			varType := parts[2]

			// Group the variables
			if _, ok := repoVars[group]; !ok {
				repoVars[group] = make(map[string]string)
			}
			repoVars[group][varType] = value
		}
	}

	return repoVars
}

func getReposFromMap(repo_map map[string]map[string]string) PriorityQueue {
	repos := make(PriorityQueue, 0)
	heap.Init(&repos)
	for _, repo := range repo_map {
		tmp := FromMap(repo)
		order, _ := strconv.Atoi(repo["ORDER"])
		item := &Item{Repository: tmp, order: order}
		heap.Push(&repos, item)
	}
	return repos
}
