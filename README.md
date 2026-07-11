# UniFi OS backup archiver

This Go service logs in to UniFi OS Consoles, downloads their full or targeted backup archives, and stores them on a mounted filesystem. Each console has its own schedule, TLS policy, timeout, and retention policy.

## Run with Compose

```sh
cp .env.example .env
# Create these two files with your console credentials, then edit .env.
mkdir -p secrets
printf '%s' 'admin' > secrets/home_username
printf '%s' 'change-me' > secrets/home_password
chmod 600 secrets/home_username secrets/home_password
docker compose up -d
curl -f http://localhost:8080/healthz
```

`CONSOLES` is a comma-separated list of console names. Each name gets a group of variables such as `CONSOLE_HOME_URL`, `CONSOLE_HOME_USERNAME_FILE`, `CONSOLE_HOME_PASSWORD_FILE`, and `CONSOLE_HOME_TARGETS`. `full` or an omitted target downloads the complete archive. Each subset target such as `network` or `protect` is a separate download request, so `CONSOLE_HOME_TARGETS=network,protect` produces two requests and two archive streams. Each target is stored under `backups/<console>/<target>/`. Secret files are preferred over direct `*_USERNAME` and `*_PASSWORD` variables.

For each console and target, retention sorts successfully archived files by their filesystem write time. It keeps the newest `CONSOLE_<NAME>_RETENTION_DAILY` archives, then keeps at most `CONSOLE_<NAME>_RETENTION_WEEKLY` older archives, selecting one representative from each `CONSOLE_<NAME>_WEEKLY_INTERVAL` age bucket. Weekly representatives inside the daily window are already counted as daily archives, so they are not duplicated. It does not parse the timestamp text in filenames because UniFi uses different formats for full, Network, Protect, and application backups.

The service recognizes full files beginning with `unifi_os_backup_`, Network files beginning with `network_backup_`, Protect files beginning with `unifi_protect_backup.`, and targeted OS files beginning with `unifi_os_backup_for_<target>_`. If a console returns the same filename again, the second archive receives a download timestamp suffix instead of overwriting the first.

The default health endpoint is `/healthz`. It returns 503 until every configured console/target has completed one successful archive, and after a success becomes 503 when any job has been unsuccessful for longer than that console's `CONSOLE_<NAME>_HEALTH_MAX_AGE`. `/readyz` has the same contract. Errors are logged without credentials. A Docker or Compose healthcheck receiving 503 marks the container unhealthy; it does not restart the container by itself. The service must exit for a restart policy to restart it.

## Configuration

See [.env.example](.env.example). Durations use Go syntax such as `24h`, and `168h` represents one week. TLS verification is enabled by default. Set `CONSOLE_<NAME>_SKIP_TLS_VERIFY=true` only for that console when it intentionally uses a self-signed certificate.

For another console, add its name to `CONSOLES`, then define the matching `CONSOLE_<NAME>_URL`, credential or credential-file variables, target list, and per-console scheduling/retention variables. Names use lowercase letters, numbers, and underscores; the environment-variable prefix is uppercase. Per-console defaults are 24 hours, 48 hours, 30 minutes, 14 daily, 12 weekly, and 168 hours respectively.

The service uses the UniFi OS API endpoints `POST /api/auth/login` and `GET /api/backup/download`. It does not create backups on the console. The controller account must have permission to download backups.

## Development

```sh
go test ./...
go vet ./...
go build ./...
```

The Compose file pulls `ghcr.io/mafredri/unifi-os-backup:latest`. The GitHub Action runs these checks and publishes that image on pushes to `main` and version tags. To build locally, use `docker build` directly; Compose intentionally does not build the image.

## Dependency updates

Renovate is configured in [`renovate.json`](renovate.json) for Docker images, GitHub Actions, and Go modules. It groups container and workflow updates and runs `go mod tidy` after Go dependency updates.
