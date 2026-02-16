package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/ls1intum/hades/shared/buildlogs"
)

// ArtemisAdapter handles the collection and synchronization of build logs and test results
// before sending them to Artemis. It uses sync.Map for thread-safe storage and per-job
// mutexes to ensure exactly-once delivery of combined results.
type ArtemisAdapter struct {
	logs       sync.Map // jobID (string) -> []buildlogs.LogEntry (only Step 2:execution logs)
	results    sync.Map // jobID (string) -> results
	jobLocks   sync.Map // jobID (string) -> *sync.Mutex
	httpClient *http.Client
	cfg        AdapterConfig
}

// ResultDTO is the complete data transfer object sent to Artemis.
// This struct is adapted from Artemis.
// It combines metadata, test results, and build logs into a single payload.
type ResultDTO struct {
	ResultMetadata
	Results   []TestSuiteDTO       `json:"results"`
	BuildLogs []buildlogs.LogEntry `json:"logs"`
}

// ResultMetadata contains the metadata about a build job's execution and outcome.
type ResultMetadata struct {
	JobName                  string `json:"jobName"`
	UUID                     string `json:"uuid"`
	AssignmentRepoBranchName string `json:"assignmentRepoBranchName"`
	IsBuildSuccessful        bool   `json:"isBuildSuccessful"`
	AssignmentRepoCommitHash string `json:"assignmentRepoCommitHash"`
	TestsRepoCommitHash      string `json:"testsRepoCommitHash"`
	BuildCompletionTime      string `json:"buildCompletionTime"`
	Passed                   int    `json:"passed"`
}

// TestSuiteDTO represents a collection of test cases that were executed together.
// It aggregates statistics about the test run including total tests, failures, and execution time.
type TestSuiteDTO struct {
	Name      string        `json:"name"`
	Time      float64       `json:"time"`
	Errors    int           `json:"errors"`
	Skipped   int           `json:"skipped"`
	Failures  int           `json:"failures"`
	Tests     int           `json:"tests"`
	TestCases []TestCaseDTO `json:"testCases"`
}

// TestCaseDTO represents a single test case execution result.
// It contains either failure/error details for failed tests or success information for passed tests.
type TestCaseDTO struct {
	Name      string                     `json:"name"`
	Classname string                     `json:"classname"`
	Time      float64                    `json:"time"`
	Failures  []TestCaseDetailMessageDTO `json:"failures"`     // empty for passing tests
	Errors    []TestCaseDetailMessageDTO `json:"errors"`       // empty for passing tests
	Successes []TestCaseDetailMessageDTO `json:"successInfos"` // empty for failing tests
}

// TestCaseDetailMessageDTO provides detailed information about a test case result,
// including error messages, stack traces, and result types.
type TestCaseDetailMessageDTO struct {
	Message               string `json:"message"`
	Type                  string `json:"type"`
	MessageWithStackTrace string `json:"messageWithStackTrace"`
}

const requestTimeout = 10 * time.Second

func NewAdapter(ctx context.Context, cfg AdapterConfig) *ArtemisAdapter {
	aa := ArtemisAdapter{
		httpClient: &http.Client{Timeout: requestTimeout},
		cfg:        cfg,
	}

	return &aa
}

// StoreLogs stores execution logs (Step 2) for the given job ID and triggers a check
// to see if results are also ready. If both logs and results are available, they are
// combined and sent to Artemis.
//
// If logs array has fewer than 2 elements, an empty log array is stored and a warning is logged.
func (aa *ArtemisAdapter) StoreLogs(jobID string, logs []buildlogs.Log) error {
	executionLogs := []buildlogs.LogEntry{}
	if len(logs) < 2 {
		slog.Warn("Execution logs missing", "jobID", jobID)
		executionLogs = []buildlogs.LogEntry{}
	} else {
		executionLogs = logs[1].Logs
	}

	aa.logs.Store(jobID, executionLogs)
	return aa.checkAndSendIfReady(jobID)
}

// StoreResults stores test results for the given job ID and triggers a check
// to see if logs are also ready. If both logs and results are available, they are
// combined and sent to Artemis.
func (aa *ArtemisAdapter) StoreResults(jobID string, results ResultDTO) error {
	aa.results.Store(jobID, results)
	slog.Debug("Stored results", "jobID", jobID, "results", results.Results)
	return aa.checkAndSendIfReady(jobID)
}

// checkAndSendIfReady checks if both logs and results exist for the given jobID.
// If both are present, it combines them into a single ResultDTO and sends it to Artemis.
// After a successful send, all data for the job (logs, results, and mutex) is cleaned up.
//
// This method uses a per-job mutex to ensure that even if StoreLogs and StoreResults
// are called concurrently for the same job, the check-combine-send-delete sequence
// is atomic, preventing duplicate sends.
func (aa *ArtemisAdapter) checkAndSendIfReady(jobID string) error {
	// Get or create mutex for this specific job
	lockVal, _ := aa.jobLocks.LoadOrStore(jobID, &sync.Mutex{})
	lock := lockVal.(*sync.Mutex)

	lock.Lock()
	defer lock.Unlock()

	logs, logsExist := aa.logs.Load(jobID)
	results, resultsExist := aa.results.Load(jobID)

	// If both logs and results exist, combine and send
	if logsExist && resultsExist {
		logs := logs.([]buildlogs.LogEntry)
		results := results.(ResultDTO)

		results.BuildLogs = logs
		slog.Debug("Combined logs and results", "jobID", jobID)

		// Send to Artemis
		if err := aa.sendToArtemis(results); err != nil {
			slog.Error("Failed to send results with logs to Artemis", "jobID", jobID, "error", err)
			return err
		}
		slog.Info("Successfully sent results with logs to Artemis", "jobID", jobID)

		// Clean up after successful send
		aa.logs.Delete(jobID)
		aa.results.Delete(jobID)
		aa.jobLocks.Delete(jobID)
	} else {
		slog.Debug("Waiting for complete data", "jobID", jobID, "hasLogs", logsExist, "hasResults", resultsExist)
	}

	return nil
}

// sendToArtemis sends the combined ResultDTO to the configured Artemis endpoint.
// It constructs the full endpoint URL using the job name, marshals the DTO to JSON,
// and sends an authenticated POST request.
//
// Returns an error if marshaling fails, request creation fails, the HTTP request fails,
// or if Artemis responds with a status code >= 400.
func (aa *ArtemisAdapter) sendToArtemis(dto ResultDTO) error {
	endpoint := fmt.Sprintf("%s/%s/%s", aa.cfg.ArtemisBaseURL, aa.cfg.NewResultEndpoint, dto.JobName)

	jsonData, err := json.Marshal(dto)
	if err != nil {
		return fmt.Errorf("marshaling DTO to JSON: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", aa.cfg.ArtemisAuthToken)

	resp, err := aa.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request to Artemis: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("adapter returned HTTP %d for job %s", resp.StatusCode, dto.UUID)
	}

	slog.Info("Request sent", "status", resp.Status)

	return nil
}
