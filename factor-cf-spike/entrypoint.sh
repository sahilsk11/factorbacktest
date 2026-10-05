#!/bin/sh
set -eu
REASON="${REASON:-http-health}"

case "$REASON" in
  cron:*)
    printf '{"ok":true,"mode":"cron","reason":"%s","boot_iso":"%s"}\n' "$REASON" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    exit 0
    ;;
esac

BOOT_ISO="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
mkdir -p /www
printf '{"ok":true,"boot_iso":"%s","service":"factor-cf-spike","mode":"http"}\n' "$BOOT_ISO" > /www/health
exec busybox httpd -f -p 8080 -h /www
