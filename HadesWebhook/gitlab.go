package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GitLabAdapter handles GitLab webhook events.
// Register it at POST /webhook/gitlab.
//
// GitLab uses a static secret token sent in X-Gitlab-Token rather than HMAC.
// Configure GITLAB_WEBHOOK_SECRET to match the "Secret token" field in GitLab's
// webhook settings (Settings -> Webhooks).
type GitLabAdapter struct {
	secret string
}

func (a *GitLabAdapter) Validate(r *http.Request, body []byte) error {
	if a.secret == "" {
		return nil
	}
	if r.Header.Get("X-Gitlab-Token") != a.secret {
		return fmt.Errorf("invalid X-Gitlab-Token")
	}
	return nil
}

func (a *GitLabAdapter) Parse(r *http.Request, body []byte) (EventContext, error) {
	switch r.Header.Get("X-Gitlab-Event") {
	case "Push Hook", "Tag Push Hook":
		return parseGitLabPush(body)
	case "Merge Request Hook":
		return parseGitLabMR(body)
	default:
		return EventContext{}, ErrEventSkipped
	}
}

// --- GitLab-specific payload structs ---

type glProject struct {
	Name              string `json:"name"`
	PathWithNamespace string `json:"path_with_namespace"`
	HTTPURLToRepo     string `json:"http_url_to_repo"`
}

type glPushEvent struct {
	Ref          string    `json:"ref"`
	CheckoutSHA  string    `json:"checkout_sha"`
	UserUsername string    `json:"user_username"`
	Project      glProject `json:"project"`
	Commits      []struct {
		Message string `json:"message"`
	} `json:"commits"`
}

type glMREvent struct {
	User struct {
		Username string `json:"username"`
	} `json:"user"`
	Project          glProject `json:"project"`
	ObjectAttributes struct {
		IID          int    `json:"iid"`
		Title        string `json:"title"`
		Action       string `json:"action"`
		SourceBranch string `json:"source_branch"`
		LastCommit   struct {
			ID string `json:"id"`
		} `json:"last_commit"`
		Source glProject `json:"source"`
	} `json:"object_attributes"`
}

func parseGitLabPush(body []byte) (EventContext, error) {
	var ev glPushEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return EventContext{}, fmt.Errorf("unmarshal GitLab push event: %w", err)
	}
	branch := strings.TrimPrefix(ev.Ref, "refs/heads/")
	var commitMsg string
	if len(ev.Commits) > 0 {
		commitMsg = ev.Commits[0].Message
	}
	owner, name, _ := strings.Cut(ev.Project.PathWithNamespace, "/")
	return EventContext{
		Platform:          "gitlab",
		EventType:         "push",
		Action:            "push",
		RepoURL:           ev.Project.HTTPURLToRepo,
		RepoName:          name,
		RepoOwner:         owner,
		RepoFullName:      ev.Project.PathWithNamespace,
		Branch:            branch,
		SHA:               ev.CheckoutSHA,
		ShortSHA:          shortSHA(ev.CheckoutSHA),
		RefName:           ev.Ref,
		SenderLogin:       ev.UserUsername,
		HeadCommitMessage: commitMsg,
	}, nil
}

func parseGitLabMR(body []byte) (EventContext, error) {
	var ev glMREvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return EventContext{}, fmt.Errorf("unmarshal GitLab MR event: %w", err)
	}
	// Skip closed/merged MRs - no live code to run CI against.
	action := ev.ObjectAttributes.Action
	if action == "merge" || action == "close" {
		return EventContext{}, ErrEventSkipped
	}
	repoURL := cmp.Or(ev.ObjectAttributes.Source.HTTPURLToRepo, ev.Project.HTTPURLToRepo)
	sha := ev.ObjectAttributes.LastCommit.ID
	owner, name, _ := strings.Cut(ev.Project.PathWithNamespace, "/")
	return EventContext{
		Platform:     "gitlab",
		EventType:    "pull_request",
		Action:       normalizeGitLabMRAction(action),
		RepoURL:      repoURL,
		RepoName:     name,
		RepoOwner:    owner,
		RepoFullName: ev.Project.PathWithNamespace,
		Branch:       ev.ObjectAttributes.SourceBranch,
		SHA:          sha,
		ShortSHA:     shortSHA(sha),
		RefName:      fmt.Sprintf("refs/merge-requests/%d/head", ev.ObjectAttributes.IID),
		PRNumber:     ev.ObjectAttributes.IID,
		PRTitle:      ev.ObjectAttributes.Title,
		SenderLogin:  ev.User.Username,
	}, nil
}

// normalizeGitLabMRAction maps GitLab MR actions to GitHub-compatible PR actions
// so that templates can use the same ALLOWED_EVENTS values across platforms.
func normalizeGitLabMRAction(action string) string {
	switch action {
	case "open":
		return "opened"
	case "update":
		return "synchronize"
	case "reopen":
		return "reopened"
	default:
		return action
	}
}
