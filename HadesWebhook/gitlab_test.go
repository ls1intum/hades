package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testGLSecret = "gitlab-webhook-secret"

const glPushPayload = `{
  "ref": "refs/heads/main",
  "checkout_sha": "deadbeef12345678abcd",
  "user_username": "carol",
  "project": {
    "name": "myrepo",
    "path_with_namespace": "org/myrepo",
    "http_url_to_repo": "https://gitlab.com/org/myrepo.git"
  },
  "commits": [{"message": "fix: something"}]
}`

const glMROpenPayload = `{
  "user": {"username": "dave"},
  "project": {
    "name": "myrepo",
    "path_with_namespace": "org/myrepo",
    "http_url_to_repo": "https://gitlab.com/org/myrepo.git"
  },
  "object_attributes": {
    "iid": 7,
    "title": "Add feature",
    "action": "open",
    "source_branch": "feature/x",
    "last_commit": {"id": "aabbcc112233aabb"},
    "source": {
      "name": "myrepo",
      "path_with_namespace": "fork/myrepo",
      "http_url_to_repo": "https://gitlab.com/fork/myrepo.git"
    }
  }
}`

const glMRUpdatePayload = `{
  "user": {"username": "dave"},
  "project": {
    "name": "myrepo",
    "path_with_namespace": "org/myrepo",
    "http_url_to_repo": "https://gitlab.com/org/myrepo.git"
  },
  "object_attributes": {
    "iid": 7,
    "title": "Add feature",
    "action": "update",
    "source_branch": "feature/x",
    "last_commit": {"id": "aabbcc112233aabb"},
    "source": {}
  }
}`

const glMRMergePayload = `{
  "user": {"username": "eve"},
  "project": {
    "name": "myrepo",
    "path_with_namespace": "org/myrepo",
    "http_url_to_repo": "https://gitlab.com/org/myrepo.git"
  },
  "object_attributes": {
    "iid": 7,
    "title": "Add feature",
    "action": "merge",
    "source_branch": "feature/x",
    "last_commit": {"id": "aabbcc112233aabb"},
    "source": {}
  }
}`

func TestGitLabValidate(t *testing.T) {
	adapter := &GitLabAdapter{secret: testGLSecret}

	t.Run("valid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("X-Gitlab-Token", testGLSecret)
		if err := adapter.Validate(req, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("wrong token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("X-Gitlab-Token", "wrong-token")
		if err := adapter.Validate(req, nil); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("missing token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		if err := adapter.Validate(req, nil); err == nil {
			t.Error("expected error for missing header, got nil")
		}
	})

	t.Run("empty secret skips validation", func(t *testing.T) {
		a := &GitLabAdapter{secret: ""}
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		if err := a.Validate(req, nil); err != nil {
			t.Errorf("unexpected error with empty secret: %v", err)
		}
	})
}

func TestGitLabParse_Push(t *testing.T) {
	adapter := &GitLabAdapter{}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Gitlab-Event", "Push Hook")

	ctx, err := adapter.Parse(req, []byte(glPushPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.Platform != "gitlab" {
		t.Errorf("Platform = %q, want gitlab", ctx.Platform)
	}
	if ctx.EventType != "push" {
		t.Errorf("EventType = %q, want push", ctx.EventType)
	}
	if ctx.Branch != "main" {
		t.Errorf("Branch = %q, want main", ctx.Branch)
	}
	if ctx.SHA != "deadbeef12345678abcd" {
		t.Errorf("SHA = %q, want deadbeef12345678abcd", ctx.SHA)
	}
	if ctx.ShortSHA != "deadbeef" {
		t.Errorf("ShortSHA = %q, want deadbeef", ctx.ShortSHA)
	}
	if ctx.RepoOwner != "org" {
		t.Errorf("RepoOwner = %q, want org", ctx.RepoOwner)
	}
	if ctx.RepoName != "myrepo" {
		t.Errorf("RepoName = %q, want myrepo", ctx.RepoName)
	}
	if ctx.HeadCommitMessage != "fix: something" {
		t.Errorf("HeadCommitMessage = %q", ctx.HeadCommitMessage)
	}
	if ctx.SenderLogin != "carol" {
		t.Errorf("SenderLogin = %q, want carol", ctx.SenderLogin)
	}
}

func TestGitLabParse_TagPush(t *testing.T) {
	adapter := &GitLabAdapter{}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Gitlab-Event", "Tag Push Hook")

	ctx, err := adapter.Parse(req, []byte(glPushPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.EventType != "push" {
		t.Errorf("EventType = %q, want push", ctx.EventType)
	}
}

func TestGitLabParse_MROpen(t *testing.T) {
	adapter := &GitLabAdapter{}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")

	ctx, err := adapter.Parse(req, []byte(glMROpenPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.Platform != "gitlab" {
		t.Errorf("Platform = %q, want gitlab", ctx.Platform)
	}
	if ctx.EventType != "pull_request" {
		t.Errorf("EventType = %q, want pull_request", ctx.EventType)
	}
	if ctx.Action != "opened" {
		t.Errorf("Action = %q, want opened (normalized from 'open')", ctx.Action)
	}
	if ctx.PRNumber != 7 {
		t.Errorf("PRNumber = %d, want 7", ctx.PRNumber)
	}
	if ctx.Branch != "feature/x" {
		t.Errorf("Branch = %q, want feature/x", ctx.Branch)
	}
	// Fork source URL should be preferred over project URL.
	if ctx.RepoURL != "https://gitlab.com/fork/myrepo.git" {
		t.Errorf("RepoURL = %q, want fork URL", ctx.RepoURL)
	}
}

func TestGitLabParse_MRUpdate(t *testing.T) {
	adapter := &GitLabAdapter{}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")

	ctx, err := adapter.Parse(req, []byte(glMRUpdatePayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Action != "synchronize" {
		t.Errorf("Action = %q, want synchronize (normalized from 'update')", ctx.Action)
	}
	// Source URL is empty, should fall back to project URL.
	if ctx.RepoURL != "https://gitlab.com/org/myrepo.git" {
		t.Errorf("RepoURL = %q, want project URL fallback", ctx.RepoURL)
	}
}

func TestGitLabParse_SkippedEvents(t *testing.T) {
	adapter := &GitLabAdapter{}

	t.Run("merge action skipped", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
		_, err := adapter.Parse(req, []byte(glMRMergePayload))
		if !errors.Is(err, ErrEventSkipped) {
			t.Errorf("expected ErrEventSkipped for merge action, got %v", err)
		}
	})

	t.Run("close action skipped", func(t *testing.T) {
		closePayload := `{"user":{"username":"x"},"project":{"name":"r","path_with_namespace":"o/r","http_url_to_repo":"u"},"object_attributes":{"iid":1,"title":"t","action":"close","source_branch":"b","last_commit":{"id":"s"},"source":{}}}`
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
		_, err := adapter.Parse(req, []byte(closePayload))
		if !errors.Is(err, ErrEventSkipped) {
			t.Errorf("expected ErrEventSkipped for close action, got %v", err)
		}
	})

	t.Run("Note Hook skipped", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("X-Gitlab-Event", "Note Hook")
		_, err := adapter.Parse(req, []byte(`{}`))
		if !errors.Is(err, ErrEventSkipped) {
			t.Errorf("expected ErrEventSkipped for unknown event, got %v", err)
		}
	})
}
