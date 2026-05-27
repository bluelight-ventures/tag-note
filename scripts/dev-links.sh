#!/bin/sh
set -eu

APP_URL="${TAGNOTE_DEV_BASE_URL:-http://localhost:3777}"
PROXY_URL="${TAGNOTE_DEV_PROXY_URL:-http://localhost:3778}"
GRAFANA_URL="${TAGNOTE_GRAFANA_URL:-http://localhost:3778/grafana/}"
OPERATIONAL_TOKEN="${OPERATIONAL_BEARER_TOKEN:-dev-operational-token}"

printf 'Waiting for TagNote at %s/healthz' "$APP_URL"
i=0
while [ "$i" -lt 60 ]; do
	if curl -fsS "$APP_URL/healthz" >/dev/null 2>&1; then
		printf '\n'
		break
	fi
	i=$((i + 1))
	printf '.'
	sleep 1
done

if [ "$i" -ge 60 ]; then
	printf '\nTagNote did not become ready at %s/healthz within 60 seconds.\n' "$APP_URL" >&2
	printf 'Check logs with: docker compose logs --tail=80 tagnote\n' >&2
	exit 1
fi

cat <<EOF

TagNote dev links
  Landing:    $APP_URL/
  App:        $APP_URL/app
  Admin:      $APP_URL/admin
  Health:     $APP_URL/healthz
  Status:     $APP_URL/status
  Metrics:    $APP_URL/metrics
  Proxy:      $PROXY_URL/
  Grafana:    $GRAFANA_URL

Operational endpoints require:
  Authorization: Bearer $OPERATIONAL_TOKEN
EOF
