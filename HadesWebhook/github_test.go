package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testGHSecret = "test-github-secret"

// ghSig computes the X-Hub-Signature-256 value for body using the test secret.
func ghSig(body string) string {
	mac := hmac.New(sha256.New, []byte(testGHSecret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Realistic GitHub webhook payloads used across multiple test files.
const ghPushPayload = `{
  "ref": "refs/heads/main",
  "after": "abc123def456abcd",
  "repository": {
    "name": "myrepo",
    "full_name": "org/myrepo",
    "clone_url": "https://github.com/org/myrepo.git",
    "owner": {"login": "org"}
  },
  "sender": {"login": "alice"},
  "head_commit": {"message": "feat: add feature"}
}`

const ghPRPayload = `{
  "action": "opened",
  "number": 42,
  "repository": {
    "name": "myrepo",
    "full_name": "org/myrepo",
    "clone_url": "https://github.com/org/myrepo.git",
    "owner": {"login": "org"}
  },
  "sender": {"login": "bob"},
  "pull_request": {
    "title": "Fix login bug",
    "head": {
      "ref": "fix/login",
      "sha": "deadbeef12345678",
      "repo": {
        "name": "myrepo",
        "full_name": "org/myrepo",
        "clone_url": "https://github.com/org/myrepo.git",
        "owner": {"login": "org"}
      }
    }
  }
}`

const ghPRForkPayload = `{
  "action": "synchronize",
  "number": 7,
  "repository": {
    "name": "myrepo",
    "full_name": "org/myrepo",
    "clone_url": "https://github.com/org/myrepo.git",
    "owner": {"login": "org"}
  },
  "sender": {"login": "carol"},
  "pull_request": {
    "title": "Fork PR",
    "head": {
      "ref": "fork-branch",
      "sha": "1122334455667788",
      "repo": {
        "name": "myrepo",
        "full_name": "fork/myrepo",
        "clone_url": "https://github.com/fork/myrepo.git",
        "owner": {"login": "fork"}
      }
    }
  }
}`

func TestGitHubValidate(t *testing.T) {
	adapter := &GitHubAdapter{secret: testGHSecret}

	t.Run("valid signature", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("X-Hub-Signature-256", ghSig(ghPushPayload))
		if err := adapter.Validate(req, []byte(ghPushPayload)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("wrong signature", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("X-Hub-Signature-256", "sha256=badhash00000000000000000000000000000000000000000000000000000000")
		if err := adapter.Validate(req, []byte(ghPushPayload)); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("missing prefix", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("X-Hub-Signature-256", hex.EncodeToString([]byte("nosig")))
		if err := adapter.Validate(req, []byte(ghPushPayload)); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("empty secret skips validation", func(t *testing.T) {
		a := &GitHubAdapter{secret: ""}
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		// no signature header at all
		if err := a.Validate(req, []byte("anything")); err != nil {
			t.Errorf("unexpected error with empty secret: %v", err)
		}
	})
}

func TestGitHubParse_Push(t *testing.T) {
	adapter := &GitHubAdapter{}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-GitHub-Event", "push")

	ctx, err := adapter.Parse(req, []byte(ghPushPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.Platform != "github" {
		t.Errorf("Platform = %q, want github", ctx.Platform)
	}
	if ctx.EventType != "push" {
		t.Errorf("EventType = %q, want push", ctx.EventType)
	}
	if ctx.Action != "push" {
		t.Errorf("Action = %q, want push", ctx.Action)
	}
	if ctx.Branch != "main" {
		t.Errorf("Branch = %q, want main", ctx.Branch)
	}
	if ctx.SHA != "abc123def456abcd" {
		t.Errorf("SHA = %q, want abc123def456abcd", ctx.SHA)
	}
	if ctx.ShortSHA != "abc123de" {
		t.Errorf("ShortSHA = %q, want abc123de", ctx.ShortSHA)
	}
	if ctx.RepoOwner != "org" {
		t.Errorf("RepoOwner = %q, want org", ctx.RepoOwner)
	}
	if ctx.RepoName != "myrepo" {
		t.Errorf("RepoName = %q, want myrepo", ctx.RepoName)
	}
	if ctx.RepoFullName != "org/myrepo" {
		t.Errorf("RepoFullName = %q, want org/myrepo", ctx.RepoFullName)
	}
	if ctx.HeadCommitMessage != "feat: add feature" {
		t.Errorf("HeadCommitMessage = %q", ctx.HeadCommitMessage)
	}
	if ctx.SenderLogin != "alice" {
		t.Errorf("SenderLogin = %q, want alice", ctx.SenderLogin)
	}
}

func TestGitHubParse_PullRequest(t *testing.T) {
	adapter := &GitHubAdapter{}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-GitHub-Event", "pull_request")

	ctx, err := adapter.Parse(req, []byte(ghPRPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.Platform != "github" {
		t.Errorf("Platform = %q, want github", ctx.Platform)
	}
	if ctx.EventType != "pull_request" {
		t.Errorf("EventType = %q, want pull_request", ctx.EventType)
	}
	if ctx.Action != "opened" {
		t.Errorf("Action = %q, want opened", ctx.Action)
	}
	if ctx.PRNumber != 42 {
		t.Errorf("PRNumber = %d, want 42", ctx.PRNumber)
	}
	if ctx.PRTitle != "Fix login bug" {
		t.Errorf("PRTitle = %q, want Fix login bug", ctx.PRTitle)
	}
	if ctx.Branch != "fix/login" {
		t.Errorf("Branch = %q, want fix/login", ctx.Branch)
	}
	if ctx.SHA != "deadbeef12345678" {
		t.Errorf("SHA = %q", ctx.SHA)
	}
	if ctx.SenderLogin != "bob" {
		t.Errorf("SenderLogin = %q, want bob", ctx.SenderLogin)
	}
}

func TestGitHubParse_ForkPR(t *testing.T) {
	adapter := &GitHubAdapter{}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-GitHub-Event", "pull_request")

	ctx, err := adapter.Parse(req, []byte(ghPRForkPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// For a fork PR, the clone URL should be the fork's URL, not the base repo's.
	if ctx.RepoURL != "https://github.com/fork/myrepo.git" {
		t.Errorf("RepoURL = %q, want fork URL", ctx.RepoURL)
	}
	// Owner/name come from the base repository.
	if ctx.RepoOwner != "org" {
		t.Errorf("RepoOwner = %q, want org", ctx.RepoOwner)
	}
}

func TestGitHubParse_SkippedEvents(t *testing.T) {
	adapter := &GitHubAdapter{}

	for _, event := range []string{"ping", "deployment", "release", "star"} {
		t.Run(event, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set("X-GitHub-Event", event)
			_, err := adapter.Parse(req, []byte(`{}`))
			if !errors.Is(err, ErrEventSkipped) {
				t.Errorf("event %q: expected ErrEventSkipped, got %v", event, err)
			}
		})
	}
}
