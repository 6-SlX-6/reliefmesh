# Backup and restore

## What to back up

| Item | Where | How |
| --- | --- | --- |
| Database | PostgreSQL volume `db_data` | `infrastructure/scripts/backup.sh` (pg_dump) |
| Data encryption keys | `.env` (`RELIEFMESH_DATA_ENCRYPTION_KEYS`) | **separately**, offline (password manager, sealed envelope, hardware token) |
| Configuration | `infrastructure/compose/.env` | separately and encrypted |

Without the data encryption keys, exact locations and contact details in a
backup cannot be decrypted. Without the database, the keys are useless. Keep
them apart so that one leaked item alone does not expose protected data.

## Creating backups

```bash
cd infrastructure/scripts
./backup.sh                       # writes ../../backups/reliefmesh-<timestamp>.sql.gz
RELIEFMESH_BACKUP_RECIPIENT=age1... ./backup.sh   # encrypts with age (or GPG key id)
RELIEFMESH_BACKUP_KEEP=30 ./backup.sh             # keep 30 backups (default 14)
```

Backups contain personal data (free text is not encrypted inside the
database). Always encrypt them before they leave the server.

Example cron entry (daily at 02:30):

```cron
30 2 * * * RELIEFMESH_BACKUP_RECIPIENT=age1yourkey /opt/reliefmesh/infrastructure/scripts/backup.sh >> /var/log/reliefmesh-backup.log 2>&1
```

## Restoring

```bash
cd infrastructure/scripts
./restore.sh ../../backups/reliefmesh-20261001T023000Z.sql.gz
```

The script asks you to type `RESTORE`, stops the API, recreates the database,
imports the dump and starts the API again. Make sure `.env` contains the data
encryption key(s) that were active when the backup was taken.

After restoring:

1. Sign in as administrator and run *Administration > Audit log > Verify
   integrity*.
2. Ask users to reload the app. Devices with queued offline changes will push
   them; changes to records that no longer exist are reported as conflicts or
   rejections in *Pending changes* (nothing is applied silently).

## Testing restores

Test a restore at least once per exercise cycle on a separate machine:

```bash
docker compose -f docker-compose.yml -p reliefmesh-restore-test up -d db
# import the dump into the test project and start the API with the same keys
```

## Retention and backups

Retention redacts data in the live database. Old backups still contain the
data until they are rotated out; choose `RELIEFMESH_BACKUP_KEEP` and your
off-site copies so that they match your retention promise.
