package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GitHubAdapter handles GitHub webhook events.
// Register it at POST /webhook/github.
type GitHubAdapter struct {
	secret string
}

func (a *GitHubAdapter) Validate(r *http.Request, body []byte) error {
	if a.secret == "" {
		return nil
	}
	return validateHMACSignature(r.Header.Get("X-Hub-Signature-256"), body, a.secret, "sha256=")
}

func (a *GitHubAdapter) Parse(r *http.Request, body []byte) (EventContext, error) {
	switch r.Header.Get("X-GitHub-Event") {
	case "push":
		return parseGitHubPush(body)
	case "pull_request":
		return parseGitHubPR(body)
	default:
		return EventContext{}, ErrEventSkipped
	}
}

// validateHMACSignature verifies a "<prefix><hex_digest>" signature header using
// HMAC-SHA256. Used by GitHub; also usable by Bitbucket which uses the same scheme.
func validateHMACSignature(header string, body []byte, secret, prefix string) error {
	if !strings.HasPrefix(header, prefix) {
		return fmt.Errorf("missing %s prefix in signature header", prefix)
	}
	expected, err := hex.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	if !hmac.Equal(mac.Sum(nil), expected) {
		return fmt.Errorf("HMAC mismatch")
	}
	return nil
}

// --- GitHub-specific payload structs ---

type ghRepository struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	CloneURL string `json:"clone_url"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type ghSender struct {
	Login string `json:"login"`
}

type ghPushEvent struct {
	Ref        string       `json:"ref"`
	After      string       `json:"after"`
	Repository ghRepository `json:"repository"`
	Sender     ghSender     `json:"sender"`
	HeadCommit *struct {
		Message string `json:"message"`
	} `json:"head_commit"`
}

type ghPullRequestEvent struct {
	Action      string       `json:"action"`
	Number      int          `json:"number"`
	Repository  ghRepository `json:"repository"`
	Sender      ghSender     `json:"sender"`
	PullRequest struct {
		Title string `json:"title"`
		Head  struct {
			Ref  string       `json:"ref"`
			SHA  string       `json:"sha"`
			Repo ghRepository `json:"repo"`
		} `json:"head"`
	} `json:"pull_request"`
}

func parseGitHubPush(body []byte) (EventContext, error) {
	var ev ghPushEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return EventContext{}, fmt.Errorf("unmarshal GitHub push event: %w", err)
	}
	branch := strings.TrimPrefix(ev.Ref, "refs/heads/")
	var commitMsg string
	if ev.HeadCommit != nil {
		commitMsg = ev.HeadCommit.Message
	}
	return EventContext{
		Platform:          "github",
		EventType:         "push",
		Action:            "push",
		RepoURL:           ev.Repository.CloneURL,
		RepoName:          ev.Repository.Name,
		RepoOwner:         ev.Repository.Owner.Login,
		RepoFullName:      ev.Repository.FullName,
		Branch:            branch,
		SHA:               ev.After,
		ShortSHA:          shortSHA(ev.After),
		RefName:           ev.Ref,
		SenderLogin:       ev.Sender.Login,
		HeadCommitMessage: commitMsg,
	}, nil
}

func parseGitHubPR(body []byte) (EventContext, error) {
	var ev ghPullRequestEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return EventContext{}, fmt.Errorf("unmarshal GitHub pull_request event: %w", err)
	}
	repoURL := ev.PullRequest.Head.Repo.CloneURL
	if repoURL == "" {
		repoURL = ev.Repository.CloneURL
	}
	sha := ev.PullRequest.Head.SHA
	return EventContext{
		Platform:     "github",
		EventType:    "pull_request",
		Action:       ev.Action,
		RepoURL:      repoURL,
		RepoName:     ev.Repository.Name,
		RepoOwner:    ev.Repository.Owner.Login,
		RepoFullName: ev.Repository.FullName,
		Branch:       ev.PullRequest.Head.Ref,
		SHA:          sha,
		ShortSHA:     shortSHA(sha),
		RefName:      fmt.Sprintf("refs/pull/%d/head", ev.Number),
		PRNumber:     ev.Number,
		PRTitle:      ev.PullRequest.Title,
		SenderLogin:  ev.Sender.Login,
	}, nil
}
