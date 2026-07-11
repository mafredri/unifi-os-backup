# UniFi OS backup archiver design

Classification: cross-cutting feature, spanning HTTP integration, scheduling, filesystem retention, health reporting, container packaging, and CI.

## Requirement and unknowns

The service must periodically authenticate to one or more UniFi OS Consoles, download full or targeted backups, retain a configurable daily window followed by a configurable weekly window, expose health, and ship as a Go container published to GHCR.

Before researching the protocol, the unknowns were the login and download endpoints, filename and response behavior, whether a target is encoded in the path or query, how multiple consoles should be configured, how retention should count overlapping daily and weekly copies, and what constitutes prolonged failure.

The supplied PowerShell reference answers the protocol questions: login is `POST /api/auth/login` with JSON `username` and `password`; download is `GET /api/backup/download`; the response is an octet-stream and supplies a filename header beginning with `unifi_os_backup_`. The requested target values are query parameters. The second supplied repository contains general backup models but no more authoritative implementation of this endpoint. A live console is not available in this workspace, so protocol integration is tested with an HTTP fixture rather than claimed end to end.

## Acceptance criteria

- A single process schedules an immediate backup and subsequent backups at a configurable interval per console.
- Configuration is supplied by environment variables and supports multiple named consoles without embedding credentials in a JSON blob.
- Each console and target has its own directory and retention lifecycle.
- The full backup is the default; `users`, `uos`, `network`, `protect`, and `innerspace` are accepted targets. Every configured target is an independent download request; `network,protect` means two requests.
- A successful response is written atomically, without exposing a partial archive.
- The newest `daily` successful archives, ordered by archive write time, are retained. Older archives are grouped into fixed weekly-age buckets and up to `weekly` representatives are retained. The daily window therefore includes any weekly-cadence archive that falls inside it, avoiding double counting. Retention does not parse filename timestamps because target filename formats differ.
- Invalid credentials, HTTP errors, malformed filenames, empty bodies, and filesystem failures count as failed attempts.
- `/healthz` is unhealthy after the configured per-console failure age has elapsed since the last successful backup, without exposing credentials.
- Docker Compose, a Dockerfile, an example environment file, tests, and a GHCR GitHub Action are included.

Out of scope: creating a backup on the console, cloud storage adapters, encryption at rest, and browser automation for consoles requiring an interactive MFA flow.

## Alternatives

| Dimension | A: one process, explicit env config | B: one process per console | C: external scheduler plus one-shot CLI |
| --- | --- | --- | --- |
| Description | A Go daemon owns scheduling, download, retention, and health. Console names select explicit per-console variables and secret files. | Compose runs a separate configured container for each console. | A CLI downloads once; cron or a platform scheduler invokes it. |
| Correctness risk | Shared scheduler and per-console state must be isolated. | Configuration drift and duplicated images complicate global health. | Retention and overlapping runs depend on the external scheduler. |
| Complexity | Moderate, standard library only. | Low code, higher deployment configuration. | Low daemon code, higher operational requirements. |
| Reversibility | Console config can later move to files or a database. | Easy to split, harder to aggregate status. | Easy to reuse, but no built-in health endpoint. |
| Operational impact | One health endpoint and one mounted volume. | Many containers and health checks. | Requires scheduler and lock management. |
| Makes easy | Multiple consoles, one deployment, testable health. | Per-console resource isolation. | Existing cron environments. |
| Makes hard | Per-console process limits. | Cross-console observability. | Reliable prolonged-failure health. |

Approach A is selected because it directly supports the requested multi-console feature, centralizes the health contract, and keeps deployment to one service. The cost is a larger but inspectable environment surface instead of one compact JSON value; credentials can remain in Docker secret files.

## Boundary and data flow

`Config` parses `CONSOLES` and matching per-console environment variables into `ConsoleConfig` values. Credentials may be read from `*_FILE` paths, which matches Docker secrets. `Service` checks the newest existing archive for each console/target on startup and skips the download when it is newer than that console's interval. It then runs each console on its own interval and issues one job per configured target. `Downloader.Download` creates a per-console HTTP client with that console's TLS policy and timeout, posts credentials, gets the selected backup, validates status, content type, filename, and non-empty body, and streams to a temporary file. `Store.Save` renames the temporary file into the console/target directory and `Store.Prune` applies that console's retention policy by archive timestamp. `Health` records the last success and last error per job; the HTTP server returns 200 only while every job has succeeded within its console's health age threshold.

The filename is treated as untrusted input. Only its base name is used, path separators and control characters are rejected, and target-specific known filename families determine which files are eligible for pruning. Temporary files are created in the destination directory so rename is atomic on the same filesystem. A repeated filename is given a download timestamp suffix before rename so it cannot overwrite an earlier archive.

## Failure behavior and concurrency

Each job is independent. A failed login or download records an error and does not alter existing archives. A failed rename or prune also fails the job because the requested archive was not safely archived. Each console has one scheduler loop, so its targets run serially and its own interval cannot overlap; other consoles continue independently. HTTP requests have a per-console timeout. Health has a startup grace period until the first scheduled attempt completes, then evaluates each console's last success age.

## Testing and delivery

Unit tests cover config validation, filename extraction, retention with overlapping daily and weekly windows, HTTP login/download including target query, atomic save, and health transitions. `go test ./...`, `go vet ./...`, and `go build ./...` are the local gates. The container uses the non-root distroless runtime `gcr.io/distroless/static-debian13:nonroot` and Compose mounts `/backups`. GitHub Actions builds and pushes `ghcr.io/mafredri/unifi-os-backup` on the default branch and tags. The image healthcheck calls the binary's `healthcheck` mode because distroless images do not include a shell or `wget`.

## Implementation steps

1. Add config, protocol client, and filesystem store. Verify with unit tests and an HTTP fixture.
2. Add scheduler, health state, and HTTP server. Verify health transitions and graceful shutdown.
3. Add Docker, Compose, documentation, and GHCR workflow. Verify formatting, tests, vet, build, and container build where Docker is available.

## Self-review

The design solves the stated archiving problem, preserves the overlap rule by pruning only after the daily window, handles network, malformed input, empty input, downstream failure, concurrent ticks, and rollback-safe atomic writes. It introduces no third-party runtime dependency. The remaining protocol limitation is the unavailable live console; the fixture verifies the documented request and response contract but cannot prove compatibility with every UniFi OS version.
