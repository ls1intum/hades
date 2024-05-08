package main

import (
	"container/heap"
	"os"
	"strconv"
	"strings"
)

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
