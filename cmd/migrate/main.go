// Command migrate creates service databases and applies ordered SQL migrations.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	service := flag.String("service", "all", "catalog, order, delivery, user, notification, or all")
	baseline := flag.Int("baseline", 0, "record already-applied versions in an existing database; requires a single service")
	flag.Parse()
	if *baseline < 0 || (*baseline > 0 && *service == "all") {
		log.Fatal("baseline requires one service and a positive version")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	raw := os.Getenv("DATABASE_URL")
	if raw == "" {
		raw = "postgres://app:catalog@localhost:5433/postgres?sslmode=disable"
	}
	admin, err := pgx.Connect(ctx, raw)
	if err != nil {
		log.Fatal(err)
	}
	defer admin.Close(context.Background())
	names := []string{"catalog", "order", "delivery", "user", "notification"}
	if *service != "all" {
		valid := false
		for _, name := range names {
			valid = valid || name == *service
		}
		if !valid {
			log.Fatal("unknown service")
		}
		names = []string{*service}
	}
	for _, name := range names {
		database := name + "_db"
		var exists bool
		if err = admin.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)", database).Scan(&exists); err != nil {
			log.Fatal(err)
		}
		if !exists {
			if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize()); err != nil {
				log.Fatal(err)
			}
		}
		parsed, err := url.Parse(raw)
		if err != nil {
			log.Fatal(err)
		}
		parsed.Path = "/" + database
		if err = apply(ctx, parsed.String(), name, *baseline); err != nil {
			log.Fatalf("%s: %v", name, err)
		}
	}
}

func apply(ctx context.Context, dsn, service string, baseline int) error {
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close(context.Background())
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(728106)"); err != nil {
		return err
	}
	var ledger, existing bool
	if err = tx.QueryRow(ctx, "SELECT to_regclass('public.app_schema_migrations') IS NOT NULL, EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE' AND table_name NOT IN ('schema_migrations','app_schema_migrations'))").Scan(&ledger, &existing); err != nil {
		return err
	}
	var legacyChecksums, legacyDirty bool
	if err = tx.QueryRow(ctx, `SELECT
 EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='schema_migrations' AND column_name='checksum'),
 EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='schema_migrations' AND column_name='dirty')`).Scan(&legacyChecksums, &legacyDirty); err != nil {
		return err
	}
	if !ledger && legacyDirty {
		var version int
		var dirty bool
		err = tx.QueryRow(ctx, "SELECT version,dirty FROM public.schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version, &dirty)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			if dirty {
				return errors.New("legacy migration history is dirty; repair the incomplete migration before upgrading")
			}
			if version < 0 {
				return errors.New("legacy migration version is invalid")
			}
			if baseline > 0 && baseline != version {
				return errors.New("explicit baseline differs from legacy migration version")
			}
			baseline = version
			log.Printf("import clean legacy history: service=%s version=%d", service, version)
		}
	}
	if !ledger && !legacyChecksums && existing && baseline == 0 {
		return errors.New("tables exist without migration history; inspect applied migrations and use -service NAME -baseline VERSION")
	}
	if (ledger || legacyChecksums) && baseline > 0 {
		return errors.New("baseline is allowed only before migration history exists")
	}
	if _, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS public.app_schema_migrations(version INT PRIMARY KEY,checksum TEXT NOT NULL,applied_at TIMESTAMPTZ NOT NULL DEFAULT now())"); err != nil {
		return err
	}
	if !ledger && legacyChecksums {
		if _, err = tx.Exec(ctx, "INSERT INTO public.app_schema_migrations(version,checksum) SELECT version,checksum FROM public.schema_migrations"); err != nil {
			return err
		}
	}
	paths, err := filepath.Glob(filepath.Join("services", service, "migrations", "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return errors.New("no migration files found; run from repository root")
	}
	baselineFound := baseline == 0
	for _, path := range paths {
		version, err := strconv.Atoi(strings.SplitN(filepath.Base(path), "_", 2)[0])
		if err != nil {
			return err
		}
		baselineFound = baselineFound || version == baseline
		sql, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256([]byte(strings.ReplaceAll(string(sql), "\r\n", "\n")))
		checksum := hex.EncodeToString(sum[:])
		var old string
		err = tx.QueryRow(ctx, "SELECT checksum FROM public.app_schema_migrations WHERE version=$1", version).Scan(&old)
		if err == nil {
			if old != checksum {
				return fmt.Errorf("migration %d changed after application", version)
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if version > baseline {
			if _, err = tx.Exec(ctx, string(sql)); err != nil {
				return fmt.Errorf("migration %s: %w", path, err)
			}
		}
		if _, err = tx.Exec(ctx, "INSERT INTO public.app_schema_migrations(version,checksum) VALUES ($1,$2)", version, checksum); err != nil {
			return err
		}
		log.Printf("migration recorded: service=%s version=%d baseline=%v", service, version, version <= baseline)
	}
	if !baselineFound {
		return errors.New("baseline version is absent from migration files")
	}
	return tx.Commit(ctx)
}
