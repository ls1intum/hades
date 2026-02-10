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

	"github.com/joshdk/go-junit"
	"github.com/ls1intum/hades/shared/buildlogs"
)

type ArtemisAdapter struct {
	logs       sync.Map // jobID (string) -> []buildlogs.Log
	results    sync.Map // jobID (string) -> results
	httpClient *http.Client
}

// ResultMetadata populated from environment variables
type ResultMetadata struct {
	JobName                  string `json:"jobName" env:"JOB_NAME"`
	UUID                     string `json:"uuid" env:"UUID"`
	AssignmentRepoBranchName string `json:"assignmentRepoBranchName" env:"ASSIGNMENT_REPO_BRANCH_NAME" envDefault:"main"`
	IsBuildSuccessful        bool   `json:"isBuildSuccessful" env:"IS_BUILD_SUCCESSFUL"`
	AssignmentRepoCommitHash string `json:"assignmentRepoCommitHash" env:"ASSIGNMENT_REPO_COMMIT_HASH"`
	TestsRepoCommitHash      string `json:"testsRepoCommitHash" env:"TESTS_REPO_COMMIT_HASH"`
	BuildCompletionTime      string `json:"buildCompletionTime" env:"BUILD_COMPLETION_TIME"`
}
type ResultDTO struct {
	ResultMetadata
	BuildJobs []junit.Suite   `json:"buildJobs"`
	BuildLogs []buildlogs.Log `json:"buildLogs"`
}

const artemisBaseURL = "http://localhost:8080" // Replace with actual Artemis URL
const newResultEndpoint = "api/programming/public/programming-exercises/new-result"
const requestTimeout = 10 * time.Second
const artemisAuthToken = "superduperlongtestingsecrettoken"

func NewAdapter(ctx context.Context) *ArtemisAdapter {
	aa := ArtemisAdapter{
		httpClient: &http.Client{Timeout: requestTimeout},
	}

	return &aa
}

// StoreLogs stores logs for a job ID and checks if results are ready
func (aa *ArtemisAdapter) StoreLogs(jobID string, logs []buildlogs.Log) error {
	aa.logs.Store(jobID, logs)
	return aa.checkAndSendIfReady(jobID)
}

// StoreResults stores results for a job ID and checks if logs are ready
func (aa *ArtemisAdapter) StoreResults(jobID string, results ResultDTO) error {
	aa.results.Store(jobID, results)
	return aa.checkAndSendIfReady(jobID)
}

// checkAndSendIfReady checks if both logs and results exist for a jobID
// If both are present, combines them and sends to Artemis
func (aa *ArtemisAdapter) checkAndSendIfReady(jobID string) error {
	logs, logsExist := aa.logs.Load(jobID)
	results, resultsExist := aa.results.Load(jobID)

	// If both logs and results exist, combine and send
	if logsExist && resultsExist {
		logs := logs.([]buildlogs.Log)
		results := results.(ResultDTO)

		results.BuildLogs = logs
		slog.Debug("Combined logs and results", "jobID", jobID)

		// Send to Artemis
		if err := aa.sendToArtemis(results); err != nil {
			slog.Error("Failed to send results with logs to Artemis", "jobID", jobID, "error", err)
			return err
		}

		// Clean up after successful send
		aa.logs.Delete(jobID)
		aa.results.Delete(jobID)

		slog.Info("Successfully sent results with logs to Artemis", "jobID", jobID)
	} else {
		slog.Debug("Waiting for complete data", "jobID", jobID, "hasLogs", logsExist, "hasResults", resultsExist)
	}

	return nil
}

// sendToArtemis sends the combined result DTO to the Artemis endpoint
func (aa *ArtemisAdapter) sendToArtemis(dto ResultDTO) error {
	endpoint := fmt.Sprintf("%s/%s/%s", artemisBaseURL, newResultEndpoint, dto.JobName)

	jsonData, err := json.Marshal(dto)
	if err != nil {
		slog.Error("Error parsing logs to JSON", "error", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		slog.Error("Error creating the request", "error", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", artemisAuthToken)

	resp, err := aa.httpClient.Do(req)
	if err != nil {
		slog.Error("Error sending the request", "error", err)
	}
	defer resp.Body.Close()

	slog.Info("Request sent", "status", resp.Status)

	return nil
}
