#!/usr/bin/env bash
# Usage: image-test.sh IMAGE
#
# Checks that the binary rejects a missing configuration and that the server
# starts and answers /healthz. The console URL is unreachable, so /healthz
# returns 503; the Go tests cover backup downloads.
set -euo pipefail

image=$1
name=unifi-os-backup-test-$$
trap 'docker rm -f "$name" >/dev/null 2>&1 || true' EXIT

status=0
docker run --rm "$image" >/dev/null 2>&1 || status=$?
if [ "$status" -ne 2 ]; then
	echo "FAIL: running without CONSOLES exited $status, want 2" >&2
	exit 1
fi

docker run -d --name "$name" -p 127.0.0.1::8080 \
	-e CONSOLES=test \
	-e CONSOLE_TEST_URL=https://127.0.0.1:1 \
	-e CONSOLE_TEST_USERNAME=test \
	-e CONSOLE_TEST_PASSWORD=test \
	"$image" >/dev/null
port=$(docker port "$name" 8080/tcp | head -n1 | cut -d: -f2)

tries=30
until code=$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:$port/healthz") && [ "$code" = 503 ]; do
	tries=$((tries - 1))
	if [ "$tries" -eq 0 ]; then
		docker logs "$name" >&2 || true
		echo "FAIL: /healthz returned ${code:-no response}, want 503" >&2
		exit 1
	fi
	sleep 1
done

echo "unifi-os-backup: OK"
