package main

import (
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
	if env_dir := os.Getenv("REPOSITORY_DIR"); env_dir != "" && isValidPath(env_dir) {
		dir = env_dir
	}
	log.Info("Starting HadesCloneContainer")
	log.Info("Cloning repositories to ", dir)

	repos := getReposFromEnv()
	for _, repo := range repos {
		repodir := dir
		// URL is mandatory
		if repo["URL"] == "" {
			log.Warn("Skipping repository without URL")
			continue
		}
		clone_options := &git.CloneOptions{
			URL: repo["URL"],
		}
		// Check if username and password are set and use them for authentication
		if repo["USERNAME"] != "" && repo["PASSWORD"] != "" {
			clone_options.Auth = &http.BasicAuth{
				Username: repo["USERNAME"],
				Password: repo["PASSWORD"],
			}
		}
		// Check if a branch is specified
		if repo["BRANCH"] != "" {
			clone_options.ReferenceName = plumbing.ReferenceName("refs/heads/" + repo["BRANCH"])
		}

		if repo["PATH"] != "" {
			repodir = path.Join(repodir, repo["PATH"])
		} else {
			parts := strings.Split(repo["URL"], "/")
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
		log.Infof("Cloned repository %s to %s", repo["URL"], repodir)
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
