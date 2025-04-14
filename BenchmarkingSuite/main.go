package main

import (
	"fmt"
	"log"
	"time"

	"github.com/ls1intum/hades/shared/payload"
)

func main() {
	// -----------------------------
	// Configuration
	// -----------------------------
	host := "localhost:8081" // Your Hades gateway address
	jobCount := 10           // Number of jobs to submit
	concurrency := 5         // Number of concurrent workers

	// -----------------------------
	// Job Factory
	// -----------------------------
	// TODO: specify different job types
	factory := func(i int) payload.RESTPayload {
		return payload.RESTPayload{
			Priority: 3,
			QueuePayload: payload.QueuePayload{
				Name:      fmt.Sprintf("benchmark-job-%d", i),
				Metadata:  map[string]string{"GLOBAL": fmt.Sprintf("test-%d", i)},
				Timestamp: time.Now(),
				Steps: []payload.Step{
					{
						ID:    1,
						Name:  "Clone",
						Image: "ghcr.io/ls1intum/hades/hades-clone-container:latest",
						Metadata: map[string]string{
							"REPOSITORY_DIR":            "/shared",
							"HADES_TEST_USERNAME":       "",
							"HADES_TEST_PASSWORD":       "",
							"HADES_TEST_URL":            "https://github.com/Mtze/Artemis-Java-Test.git",
							"HADES_TEST_PATH":           "./example",
							"HADES_TEST_ORDER":          "1",
							"HADES_ASSIGNMENT_USERNAME": "",
							"HADES_ASSIGNMENT_PASSWORD": "",
							"HADES_ASSIGNMENT_URL":      "https://github.com/Mtze/Artemis-Java-Solution.git",
							"HADES_ASSIGNMENT_PATH":     "./example/assignment",
							"HADES_ASSIGNMENT_ORDER":    "2",
						},
					},
					{
						ID:     2,
						Name:   "Execute",
						Image:  "ls1tum/artemis-maven-template:java17-18",
						Script: "set +e && cd ./shared/example || exit 0 && ./gradlew --status || exit 0 && ./gradlew clean test || exit 0",
					},
				},
			},
		}
	}

	// -----------------------------
	// Job Submission
	// -----------------------------
	fmt.Printf("Submitting %d jobs with %d workers to %s...\n", jobCount, concurrency, host)
	jobs, err := SubmitJobs(host, jobCount, concurrency, factory)
	if err != nil {
		log.Fatalf("Failed to submit jobs: %v", err)
	}

	// -----------------------------
	// Report
	// -----------------------------
	fmt.Println("\nSubmitted Jobs:")
	for _, job := range jobs {
		fmt.Printf("Job ID: %s | Submitted at: %s\n", job.JobID, job.SubmittedAt.Format(time.RFC3339))
	}
}
