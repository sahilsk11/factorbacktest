// Command migrate brings the production database forward to the migration
// version embedded in the running binary. It's intended to run as Fly's
// release_command: a temporary VM gets the same env-var secrets the API
// process gets, runs this binary, and a non-zero exit aborts the deploy
// before any new app VMs take traffic.
//
// The version-tracking scheme matches tools/migrations.py exactly so local
// dev (Python, against the dockerized test DB) and prod (Go, against Fly's
// Postgres) stay in lockstep:
//
//   - schema_version is a single-row table holding one int.
//   - *.up.sql files are numbered NNNNNN_*.up.sql and applied in numeric
//     order, but only those with a number strictly greater than the live
//     value of schema_version.version.
//   - The whole batch runs in one transaction; a failure rolls everything
//     back, so prod stays at its previous version and the deploy aborts.
package main

import (
	"database/sql"
	"factorbacktest/internal/util"
	"factorbacktest/migrations"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

func main() {
	secrets, err := util.LoadSecrets()
	if err != nil {
		log.Fatalf("migrate: load secrets: %v", err)
	}
	db, err := sql.Open("postgres", secrets.Db.ToConnectionStr())
	if err != nil {
		log.Fatalf("migrate: open db: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("migrate: ping db: %v", err)
	}

	if err := run(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}
}

func run(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	// Safe to call after a successful Commit (becomes a no-op).
	defer tx.Rollback()

	current, err := ensureSchemaVersion(tx)
	if err != nil {
		return err
	}
	pending, err := pendingUpMigrations(current)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		log.Printf("migrate: at version %d, nothing to apply", current)
		return tx.Commit()
	}

	for _, m := range pending {
		log.Printf("migrate: applying %s", m.filename)
		if _, err := tx.Exec(m.sql); err != nil {
			return fmt.Errorf("apply %s: %w", m.filename, err)
		}
		current = m.version
	}
	if _, err := tx.Exec(`UPDATE schema_version SET version = $1`, current); err != nil {
		return fmt.Errorf("update schema_version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	log.Printf("migrate: at version %d", current)
	return nil
}

// ensureSchemaVersion returns the live schema_version.version, bootstrapping
// the table from 000000_schema_version.sql on a fresh database. Mirrors the
// SAVEPOINT-based recovery in tools/migrations.py: if the SELECT errors
// because the table doesn't exist, we apply the bootstrap script in the same
// transaction and return version 0.
func ensureSchemaVersion(tx *sql.Tx) (int, error) {
	var v int
	err := tx.QueryRow(`SELECT version FROM schema_version`).Scan(&v)
	if err == nil {
		return v, nil
	}
	bootstrap, readErr := fs.ReadFile(migrations.FS, "000000_schema_version.sql")
	if readErr != nil {
		return 0, fmt.Errorf("read bootstrap: %w", readErr)
	}
	if _, execErr := tx.Exec(string(bootstrap)); execErr != nil {
		// Surface the original SELECT error too so we don't lose context if the
		// bootstrap fails for some reason other than "table missing".
		return 0, fmt.Errorf("bootstrap schema_version (after select error %v): %w", err, execErr)
	}
	return 0, nil
}

type migration struct {
	version  int
	filename string
	sql      string
}

func pendingUpMigrations(current int) ([]migration, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, err
	}

	// Collect every .up.sql file so we can validate version uniqueness across
	// the entire set before applying anything. Checking only pending migrations
	// would silently miss duplicates that are both already applied.
	var all []migration
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		prefix := strings.SplitN(name, "_", 2)[0]
		v, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("bad migration name %q: %w", name, err)
		}
		all = append(all, migration{version: v, filename: name})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].version < all[j].version })

	for i := 1; i < len(all); i++ {
		if all[i].version == all[i-1].version {
			return nil, fmt.Errorf(
				"duplicate migration version %d: %s and %s — rename one before deploying",
				all[i].version, all[i-1].filename, all[i].filename,
			)
		}
	}

	var out []migration
	for _, m := range all {
		if m.version <= current {
			continue
		}
		body, err := fs.ReadFile(migrations.FS, m.filename)
		if err != nil {
			return nil, err
		}
		m.sql = string(body)
		out = append(out, m)
	}
	return out, nil
}
