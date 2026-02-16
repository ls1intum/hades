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

type ArtemisAdapter struct {
	logs       sync.Map // jobID (string) -> []buildlogs.LogEntry (only Step 2:execution logs)
	results    sync.Map // jobID (string) -> results
	jobLocks   sync.Map // jobID (string) -> *sync.Mutex
	httpClient *http.Client
	cfg        AdapterConfig
}

// Result DTOs used by Artemis
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

type TestSuiteDTO struct {
	Name      string        `json:"name"`
	Time      float64       `json:"time"`
	Errors    int           `json:"errors"`
	Skipped   int           `json:"skipped"`
	Failures  int           `json:"failures"`
	Tests     int           `json:"tests"`
	TestCases []TestCaseDTO `json:"testCases"`
}

type TestCaseDTO struct {
	Name      string                     `json:"name"`
	Classname string                     `json:"classname"`
	Time      float64                    `json:"time"`
	Failures  []TestCaseDetailMessageDTO `json:"failures"`     // empty for passing tests
	Errors    []TestCaseDetailMessageDTO `json:"errors"`       // empty for passing tests
	Successes []TestCaseDetailMessageDTO `json:"successInfos"` // empty for failing tests
}

type TestCaseDetailMessageDTO struct {
	Message               string `json:"message"`
	Type                  string `json:"type"`
	MessageWithStackTrace string `json:"messageWithStackTrace"`
}

type ResultDTO struct {
	ResultMetadata
	Results   []TestSuiteDTO       `json:"results"`
	BuildLogs []buildlogs.LogEntry `json:"logs"`
}

const requestTimeout = 10 * time.Second

func NewAdapter(ctx context.Context, cfg AdapterConfig) *ArtemisAdapter {
	aa := ArtemisAdapter{
		httpClient: &http.Client{Timeout: requestTimeout},
		cfg:        cfg,
	}

	return &aa
}

// StoreLogs only stores Step 2 (execution) logs for a job ID and checks if results are ready
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

// StoreResults stores results for a job ID and checks if logs are ready
func (aa *ArtemisAdapter) StoreResults(jobID string, results ResultDTO) error {
	aa.results.Store(jobID, results)
	slog.Debug("Stored results", "jobID", jobID, "results", results.Results)
	return aa.checkAndSendIfReady(jobID)
}

// checkAndSendIfReady checks if both logs and results exist for a jobID
// If both are present, combines them and sends to Artemis
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

// sendToArtemis sends the combined result DTO to the Artemis endpoint
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
