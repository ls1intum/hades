# HadesWebhook

Receives Git platform webhook events and forwards them to Hades as jobs. Each incoming event is rendered through a user-supplied Go template to produce a fully configured Hades job payload, then posted to HadesAPI.

Supported platforms:

| Platform | Endpoint | Auth |
|---|---|---|
| GitHub | `POST /webhook/github` | HMAC-SHA256 (`X-Hub-Signature-256`) |
| GitLab | `POST /webhook/gitlab` | Static token (`X-Gitlab-Token`) |

---

## How it works

1. GitHub/GitLab sends a webhook `POST` to `/webhook/{platform}`.
2. HadesWebhook validates the signature and parses the event into a normalized `EventContext` (repo URL, branch, SHA, PR number, etc.).
3. The `EventContext` is rendered into a Hades job payload using your `.json.tmpl` template file.
4. The rendered job is posted to `HadesAPI /build`.

Events that do not match `ALLOWED_EVENTS` are acknowledged with `200 OK` and dropped.

---

## Configuration

All configuration is via environment variables.

| Variable | Default | Description |
|---|---|---|
| `WEBHOOK_PORT` | `8083` | Listen port |
| `HADES_API_URL` | `http://localhost:8080` | Base URL of HadesAPI |
| `HADES_AUTH_KEY` | - | Basic Auth password for HadesAPI (`AUTH_KEY`) |
| `GITHUB_WEBHOOK_SECRET` | - | HMAC secret configured in GitHub. Leave empty to skip validation (dev only). |
| `GITLAB_WEBHOOK_SECRET` | - | Static token configured in GitLab. Leave empty to skip validation (dev only). |
| `JOB_TEMPLATE_PATH` | - | Path to a `.json.tmpl` file. Falls back to a built-in echo job if unset. |
| `ALLOWED_EVENTS` | `push,pull_request` | Comma-separated list of normalized event types to forward. |

---

## Setting up the webhook in GitHub

### 1. Deploy HadesWebhook

HadesWebhook must be reachable from GitHub's servers over HTTPS. For local development you can use a tunneling tool such as [ngrok](https://ngrok.com):

```bash
ngrok http 8083
# Forwarding: https://<id>.ngrok.io -> localhost:8083
```

For production, run it behind a TLS-terminating reverse proxy (Traefik, nginx, etc.) and expose it on a stable public hostname.

### 2. Generate a webhook secret

```bash
openssl rand -hex 32
```

Copy the output. You will use it in both GitHub and your deployment config.

### 3. Add the webhook in GitHub

1. Open your repository on GitHub.
2. Go to **Settings** -> **Webhooks** -> **Add webhook**.
3. Fill in the form:
   - **Payload URL**: `https://<your-host>/webhook/github`
   - **Content type**: `application/json`
   - **Secret**: paste the value from step 2
   - **Which events**: choose *Let me select individual events*, then tick **Pushes** and **Pull requests** (uncheck everything else).
4. Click **Add webhook**.

GitHub will immediately send a `ping` event. HadesWebhook responds with `200 OK` and logs `event skipped`.

### 4. Set the environment variable

Pass the same secret to HadesWebhook:

```bash
GITHUB_WEBHOOK_SECRET=<value from step 2>
```

In the Docker Compose setup add it to the `hadesWebhook` service environment:

```yaml
environment:
  - HADES_API_URL=http://hadesAPI:8080
  - GITHUB_WEBHOOK_SECRET=<your-secret>
  - JOB_TEMPLATE_PATH=/templates/claude-code-push.json.tmpl
```

### 5. Verify delivery

Push a commit to the repository. In GitHub go to **Settings** -> **Webhooks** -> your webhook -> **Recent Deliveries**. A `200` response confirms HadesWebhook received and accepted the event. Check HadesLogManager (`GET /jobs`) to see the submitted job.

---

## Job templates

HadesWebhook uses Go [`text/template`](https://pkg.go.dev/text/template) to render the Hades job JSON. Point `JOB_TEMPLATE_PATH` at your template file.

### Template variables

All fields come from the normalized `EventContext` and are available directly by name:

| Variable | Example value | Description |
|---|---|---|
| `.Platform` | `github` | Platform that sent the event |
| `.EventType` | `push` or `pull_request` | Normalized event type |
| `.Action` | `push`, `opened`, `synchronize` | Event action |
| `.RepoURL` | `https://github.com/org/repo.git` | HTTPS clone URL |
| `.RepoFullName` | `org/repo` | `owner/name` |
| `.RepoOwner` | `org` | Repository owner login |
| `.RepoName` | `repo` | Repository name |
| `.Branch` | `main` | Branch name |
| `.SHA` | `a1b2c3d4...` | Full commit SHA |
| `.ShortSHA` | `a1b2c3d4` | First 8 characters of SHA |
| `.RefName` | `refs/heads/main` | Full Git ref |
| `.PRNumber` | `42` | Pull/merge request number (0 for push) |
| `.PRTitle` | `Fix login bug` | Pull/merge request title |
| `.SenderLogin` | `alice` | Username who triggered the event |
| `.HeadCommitMessage` | `feat: add feature` | Head commit message (push only) |

### Template functions

| Function | Usage | Output |
|---|---|---|
| `json` | `{{ .RepoURL \| json }}` | JSON-encoded string including surrounding quotes and proper escaping. Use for every string field. |
| `env` | `{{ env "MY_VAR" \| json }}` | Value of an environment variable, JSON-encoded. Use for secrets and configuration. |

### Writing a template

The template must render a valid JSON object matching the Hades `RESTPayload` schema. Use `{{ ... | json }}` for all string fields to guarantee correct JSON escaping:

```json
{
  "name": {{ printf "%s: %s@%s" .EventType .RepoFullName .ShortSHA | json }},
  "priority": 3,
  "steps": [
    {
      "id": 1,
      "name": "my-step",
      "image": "alpine:latest",
      "metadata": {
        "TOKEN": {{ env "MY_TOKEN" | json }}
      },
      "script": {{ printf "echo 'Running on %s at %s'" .RepoFullName .Branch | json }}
    }
  ]
}
```

HadesWebhook validates that the rendered output is valid JSON before posting it to HadesAPI. If the template produces invalid JSON you will see an error in the logs.

### Feedback via a GitHub comment

Add a final step to post a comment back to GitHub after your job steps complete. The step only runs when all preceding steps succeed.

**Commit comment** (works for push events - comments on the triggering commit):

```json
{
  "id": 2,
  "name": "github-comment",
  "image": "alpine:latest",
  "metadata": {
    "GITHUB_TOKEN": {{ env "GITHUB_TOKEN" | json }}
  },
  "script": {{ printf "set -e; apk add -q --no-cache curl; curl -s -X POST -H \"Authorization: Bearer $GITHUB_TOKEN\" -H 'Accept: application/vnd.github+json' -H 'Content-Type: application/json' -d '{\"body\":\"Job completed for %s@%s.\"}' 'https://api.github.com/repos/%s/%s/commits/%s/comments'" .RepoFullName .ShortSHA .RepoOwner .RepoName .SHA | json }}
}
```

**PR comment** (works for pull_request events):

```json
{
  "id": 2,
  "name": "github-comment",
  "image": "alpine:latest",
  "metadata": {
    "GITHUB_TOKEN": {{ env "GITHUB_TOKEN" | json }}
  },
  "script": {{ printf "set -e; apk add -q --no-cache curl; curl -s -X POST -H \"Authorization: Bearer $GITHUB_TOKEN\" -H 'Accept: application/vnd.github+json' -H 'Content-Type: application/json' -d '{\"body\":\"Job completed for PR #%d.\"}' 'https://api.github.com/repos/%s/%s/issues/%d/comments'" .PRNumber .RepoOwner .RepoName .PRNumber | json }}
}
```

The `GITHUB_TOKEN` is passed as a container environment variable; the shell expands `$GITHUB_TOKEN` at runtime so the token never appears in the rendered script text.

### Included example: Claude Code + GitHub comment

`templates/claude-code-push.json.tmpl` is a ready-to-use three-step template:

1. **Clone** - clones the repository into `/shared/repo` using `hades-clone-container`.
2. **Implement** - runs Claude Code with `--allowedTools Edit,Write,Read,Bash`, commits the result, and pushes a new `hades/ai-<sha>` branch.
3. **GitHub comment** - posts a commit comment linking to the new branch.

Required environment variables for this template:

```bash
GIT_TOKEN=<GitHub PAT with repo scope>
CLAUDE_CODE_IMAGE=ghcr.io/anthropics/claude-code:latest
ANTHROPIC_API_KEY=sk-ant-...
CLAUDE_PROMPT=<the task description>
GITHUB_TOKEN=<GitHub PAT with repo scope - can be the same as GIT_TOKEN>
```

---

## Adding a new platform

1. Create a new file (e.g. `bitbucket.go`) implementing the `PlatformAdapter` interface:

```go
type PlatformAdapter interface {
    Validate(r *http.Request, body []byte) error
    Parse(r *http.Request, body []byte) (EventContext, error)
}
```

- `Validate` authenticates the request. Return `nil` to accept, an error to reject.
- `Parse` extracts a normalized `EventContext`. Return `ErrEventSkipped` to acknowledge without submitting a job.

2. Register it in `main.go`:

```go
adapters := map[string]PlatformAdapter{
    "github":    &GitHubAdapter{secret: cfg.GitHubSecret},
    "gitlab":    &GitLabAdapter{secret: cfg.GitLabSecret},
    "bitbucket": &BitbucketAdapter{secret: cfg.BitbucketSecret},
}
```

The platform is then reachable at `POST /webhook/bitbucket`.

Bitbucket uses the same HMAC-SHA256 scheme as GitHub. The `validateHMACSignature` helper in `github.go` is available to all adapters in the package.
