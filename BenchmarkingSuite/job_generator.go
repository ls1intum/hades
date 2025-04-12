package BenchmarkingSuite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net/http"
	"sync"
	"time"

	"github.com/ls1intum/hades/shared/payload"
)

type SubmittedJob struct {
	JobID       string
	SubmittedAt time.Time
}

type JobFactory func(idx int) payload.RESTPayload

func GenerateUUID() uuid.UUID {
	return uuid.New()
}

// SubmitJobs submits multiple job requests to the Hades gateway concurrently.
// It generates jobs based on a provided payload template, assigns unique IDs and timestamps,
// and sends HTTP POST requests to the /build endpoint.
//
// Parameters:
//   - host: the hostname (and port) of the Hades gateway (e.g., "localhost:8080")
//   - jobCount: the total number of jobs to submit
//   - concurrency: the number of concurrent workers (goroutines) to use for job submission
//   - template: a payload.RESTPayload object used as the base template for each job request
//
// Returns:
//   - A slice of SubmittedJob, each containing the submitted job's ID and submission timestamp
//   - An error, if any issues occur during submission
func SubmitJobs(host string, jobCount int, concurrency int, factory JobFactory) ([]SubmittedJob, error) {
	var wg sync.WaitGroup
	jobs := make([]SubmittedJob, jobCount)
	jobChan := make(chan int, jobCount)
	var mu sync.Mutex
	client := &http.Client{Timeout: 15 * time.Second}

	for i := 0; i < jobCount; i++ {
		jobChan <- i
	}
	close(jobChan)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobChan {
				jobResult, err := submitJob(client, host, idx, factory)
				if err != nil {
					fmt.Printf("Job %d submission failed: %v\n", idx, err)
					continue
				}

				mu.Lock()
				jobs[idx] = jobResult
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return jobs, nil
}

func submitJob(client *http.Client, host string, idx int, factory JobFactory) (SubmittedJob, error) {
	job := factory(idx)
	job.ID = GenerateUUID()
	job.Name = fmt.Sprintf("benchmark-job-%d", idx)
	job.Timestamp = time.Now()

	body, err := json.Marshal(job)
	if err != nil {
		return SubmittedJob{}, fmt.Errorf("failed to marshal job %d: %w", idx, err)
	}

	submittedAt := time.Now()
	resp, err := client.Post(fmt.Sprintf("http://%s/build", host), "application/json", bytes.NewBuffer(body))
	if err != nil {
		return SubmittedJob{}, fmt.Errorf("job %d HTTP error: %w", idx, err)
	}
	defer resp.Body.Close()

	jobID := fmt.Sprintf("unknown-%d", idx)
	if resp.StatusCode == http.StatusOK {
		jobID = job.ID.String()
	} else {
		return SubmittedJob{}, fmt.Errorf("job %d got non-200 response: %d", idx, resp.StatusCode)
	}

	return SubmittedJob{
		JobID:       jobID,
		SubmittedAt: submittedAt,
	}, nil
}
