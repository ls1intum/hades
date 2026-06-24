package main

import (
	"errors"
	"net/http"
)

// ErrEventSkipped is returned by PlatformAdapter.Parse when the event should be
// acknowledged with 200 OK but not forwarded to Hades (e.g. a ping, a closed PR).
var ErrEventSkipped = errors.New("event skipped")

// PlatformAdapter translates a platform-specific webhook request into a
// normalized EventContext. Implement this interface to add a new platform.
type PlatformAdapter interface {
	// Validate authenticates the request (HMAC check, static token, etc.).
	// Returns nil when authentic or when no secret is configured.
	Validate(r *http.Request, body []byte) error

	// Parse extracts a normalized EventContext from the request.
	// Return ErrEventSkipped to acknowledge without submitting a job.
	Parse(r *http.Request, body []byte) (EventContext, error)
}

// EventContext holds the normalized fields extracted from any platform webhook.
// All fields are passed as template variables when rendering the job template.
type EventContext struct {
	Platform          string // "github", "gitlab", ...
	EventType         string // normalized: "push" or "pull_request"
	Action            string // e.g. "push", "opened", "synchronize", "reopened"
	RepoURL           string // HTTPS clone URL
	RepoName          string // e.g. "hades"
	RepoOwner         string // e.g. "ls1intum"
	RepoFullName      string // e.g. "ls1intum/hades"
	Branch            string // e.g. "main" or "feature/my-branch"
	SHA               string // full commit SHA
	ShortSHA          string // first 8 chars of SHA
	RefName           string // full Git ref, e.g. "refs/heads/main"
	PRNumber          int    // pull/merge request number (0 for push events)
	PRTitle           string // pull/merge request title
	SenderLogin       string // username of the user who triggered the event
	HeadCommitMessage string // head commit message (push events only)
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
