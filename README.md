# CFX Escrow Service

CFX Escrow Service is a durable HTTP job service for running CFX escrow uploads outside GitHub Actions.

GitHub Actions discovers resource paths, submits a signed job, and polls its status. The Ubuntu service clones the requested commit, runs the configured escrow uploader, mirrors FXAP output when requested, commits updated `.escrow` markers, and pushes them back to the source branch.

## Features

- HMAC-SHA256 request authentication with replay protection
- Idempotent job submission
- Persistent job state and logs
- Automatic recovery of interrupted queued and running jobs
- Serial execution to prevent overlapping CFX uploads
- Isolated Git clone per job
- Repository, branch, commit, and resource-path validation
- Upload, mirror, and upload-plus-mirror operations
- Automatic `.escrow` marker commit and push-back
- Hardened systemd service
- GitHub Actions submission and polling example
- Standard-library-only Go runtime

## Requirements

- Ubuntu server
- Go 1.24 or newer for building
- Git
- Node.js and the existing `cfx-escrow-bot` checkout
- Chromium dependencies required by Puppeteer
- SSH access from the service account to the source repository
- A CFX forum `_t` cookie
- A GitHub token with write access to the mirror repository
- HTTPS, a private VPN, or a protected reverse proxy in front of the service

## Build

```bash
go test ./...
go vet ./...
go build -trimpath -ldflags="-s -w" -o cfx-escrow-service ./cmd/escrowd
```

## Ubuntu installation

Create the service account and directories:

```bash
sudo useradd --system --home /var/lib/cfx-escrow-service --create-home --shell /usr/sbin/nologin cfx-escrow
sudo install -d -o cfx-escrow -g cfx-escrow -m 0750 /var/lib/cfx-escrow-service
sudo install -d -o root -g cfx-escrow -m 0750 /etc/cfx-escrow-service
```

Install the binary and systemd unit:

```bash
sudo install -o root -g root -m 0755 cfx-escrow-service /usr/local/bin/cfx-escrow-service
sudo install -o root -g root -m 0644 deploy/cfx-escrow-service.service /etc/systemd/system/cfx-escrow-service.service
sudo install -o root -g cfx-escrow -m 0640 deploy/cfx-escrow-service.env.example /etc/cfx-escrow-service.env
```

Edit `/etc/cfx-escrow-service.env`, install an SSH deploy key for the `cfx-escrow` account, and verify that the configured uploader runs as that account.

Enable the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now cfx-escrow-service
sudo systemctl status cfx-escrow-service
```

View logs:

```bash
journalctl -u cfx-escrow-service -f
```

## Configuration

| Variable | Required | Default | Purpose |
|---|---:|---|---|
| `LISTEN_ADDRESS` | No | `127.0.0.1:8080` | HTTP listen address |
| `API_SECRET` | Yes | | HMAC signing secret |
| `DATA_DIRECTORY` | No | `/var/lib/cfx-escrow-service` | Persistent state and temporary clones |
| `SOURCE_REPOSITORY` | Yes | | Allowed `owner/repository` identifier |
| `SOURCE_REPOSITORY_URL` | Yes | | Git clone URL |
| `SOURCE_BRANCH` | No | `main` | Allowed source branch |
| `RESOURCE_ROOT` | No | `server-files/resources` | Allowed resource path root |
| `UPLOADER_BINARY` | No | `node` | Uploader executable |
| `UPLOADER_ARGS_JSON` | No | `["/opt/cfx-escrow-bot/src/cli-escrow.js"]` | Fixed arguments placed before generated uploader arguments |
| `CFX_FORUM_COOKIE` | Yes | | CFX forum `_t` cookie |
| `MIRROR_REPOSITORY` | For mirror operations | | Mirror `owner/repository` identifier |
| `MIRROR_BRANCH` | No | `main` | Mirror branch |
| `MIRROR_TOKEN` | For mirror operations | | Mirror write token |
| `GIT_AUTHOR_NAME` | No | `cfx-escrow-service` | Marker commit author |
| `GIT_AUTHOR_EMAIL` | No | `cfx-escrow-service@users.noreply.github.com` | Marker commit email |
| `JOB_TIMEOUT` | No | `3h` | Per-job timeout |
| `MAX_BODY_BYTES` | No | `1048576` | Maximum signed request size |

## API

### Health

```http
GET /healthz
```

### Submit a job

```http
POST /v1/jobs
X-Escrow-Timestamp: 1785000000
X-Escrow-Signature: hmac-sha256-hex
Idempotency-Key: example-org/fivem-server:commit:upload_and_mirror
Content-Type: application/json
```

```json
{
  "repository": "example-org/fivem-server",
  "branch": "main",
  "commit": "0123456789abcdef0123456789abcdef01234567",
  "operation": "upload_and_mirror",
  "resources": [
    "server-files/resources/[qbx]/qbx_houserobbery",
    "server-files/resources/[standalone]/safecracker"
  ]
}
```

The response uses status `202 Accepted` and contains the durable job record.

### Get a job

```http
GET /v1/jobs/{id}
X-Escrow-Timestamp: 1785000000
X-Escrow-Signature: hmac-sha256-hex
```

### List jobs

```http
GET /v1/jobs
X-Escrow-Timestamp: 1785000000
X-Escrow-Signature: hmac-sha256-hex
```

The signature input is:

```text
timestamp + "\n" + exact request body bytes
```

GET requests use an empty body.

## GitHub Actions

Copy [examples/github-actions/escrow.yml](examples/github-actions/escrow.yml) into the source repository as `.github/workflows/escrow.yml`.

Add these repository secrets:

- `ESCROW_SERVICE_URL`
- `ESCROW_SERVICE_SECRET`

The service URL must be reachable from GitHub-hosted runners. Put it behind HTTPS or expose it through a private authenticated network path.

Push runs submit resources changed by the current commit. Manual runs can accept explicit resource paths or submit every resource containing a `.escrow` marker.

## Job lifecycle

Jobs move through:

```text
queued -> running -> succeeded
                  -> failed
```

Submitted jobs are persisted in `jobs.json`. Jobs found in `queued` or `running` state after a service restart are queued again.

The service executes one job at a time. This prevents multiple jobs from creating conflicting CFX versions or racing to push marker updates.

## Security

- Use a random API secret of at least 32 bytes.
- Keep the service bound to loopback unless a firewall, VPN, or reverse proxy protects it.
- Use an SSH deploy key for the source repository.
- Use a fine-grained GitHub token limited to the mirror repository.
- Run the service as the dedicated `cfx-escrow` account.
- Rotate the CFX forum cookie and API secret when access changes.
- Do not place credentials in repository URLs, job requests, logs, or workflow files.

## License

Licensed under the Apache License, Version 2.0.
