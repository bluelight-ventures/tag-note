#!/bin/bash
set -euo pipefail

# TagNote backup script.
# Creates a WAL-safe SQLite snapshot + uploads tarball.
# Install: crontab -e → 0 3 * * * /opt/tagnote/scripts/backup.sh >> /var/log/tagnote-backup.log 2>&1

BACKUP_DIR="${TAGNOTE_BACKUP_DIR:-/opt/tagnote/backups}"
DB_PATH="${TAGNOTE_DB_PATH:-/opt/tagnote/data/tagnote.db}"
DATA_DIR="${TAGNOTE_DATA_DIR:-/opt/tagnote/data}"
RETAIN_DAYS="${TAGNOTE_RETAIN_DAYS:-7}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

mkdir -p "$BACKUP_DIR"

# sqlite3 .backup produces a consistent snapshot even while the server is running
# with WAL mode. Never copy tagnote.db directly — the WAL may be dirty.
sqlite3 "$DB_PATH" ".backup $BACKUP_DIR/tagnote-$TIMESTAMP.db"

tar -czf "$BACKUP_DIR/tagnote-$TIMESTAMP.tar.gz" \
    -C "$BACKUP_DIR" "tagnote-$TIMESTAMP.db" \
    -C "$DATA_DIR" uploads

rm "$BACKUP_DIR/tagnote-$TIMESTAMP.db"

# Uncomment to upload to Backblaze B2 (requires `b2` CLI configured):
# b2 upload-file tagnote-backups "$BACKUP_DIR/tagnote-$TIMESTAMP.tar.gz" \
#     "backups/tagnote-$TIMESTAMP.tar.gz"

find "$BACKUP_DIR" -name "tagnote-*.tar.gz" -mtime +"$RETAIN_DAYS" -delete

echo "[$(date)] Backup completed: tagnote-$TIMESTAMP.tar.gz"
