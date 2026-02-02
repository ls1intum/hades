package main

import (
	"context"
	"sync"

	"github.com/joshdk/go-junit"
	"github.com/ls1intum/hades/shared/buildlogs"
)

type ArtemisAdapter struct {
	logs    sync.Map // jobID (string) -> logsVersion
	results sync.Map // jobID (string) -> reults
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
	BuildJobs []junit.Suite `json:"buildJobs"`
	BuildLogs buildlogs.Log `json:"buildLogs"`
}

func NewAdapter(ctx context.Context) *ArtemisAdapter {
	aa := ArtemisAdapter{}

	// Start background cleanup goroutine
	//go aa.cleanupLoop(ctx)

	return &aa
}
