package testhelpers

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/nats-io/nats.go"
	"github.com/pressly/goose/v3"
	"github.com/yunobar/album/internal/adapters/db/postgres/migrations"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type testDBConfig struct {
	Host     string `required:"true"`
	Port     string `required:"true"`
	User     string `required:"true"`
	Password string `required:"true"`
	Name     string `required:"true" default:"album_test"`
}

// SetupTestDB loads .env.test, connects to the test database, runs migrations,
// and returns the *gorm.DB. Call the returned cleanup function in a defer.
func SetupTestDB(envPath string) (*gorm.DB, func(), error) {
	_ = godotenv.Load(envPath)

	var cfg testDBConfig
	if err := envconfig.Process("TEST_DB", &cfg); err != nil {
		return nil, nil, fmt.Errorf("testhelpers: failed to load TEST_DB config: %w", err)
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s",
		cfg.Host, cfg.User, cfg.Password, cfg.Name, cfg.Port,
	)

	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, nil, fmt.Errorf("testhelpers: failed to connect test DB: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("testhelpers: failed to get sql.DB: %w", err)
	}

	if err = refreshTestDB(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, nil, err
	}

	cleanup := func() { _ = sqlDB.Close() }
	return gormDB, cleanup, nil
}

func refreshTestDB(db *sql.DB) error {
	goose.SetBaseFS(migrations.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("testhelpers: failed to set goose dialect: %w", err)
	}
	// Reset may fail on a fresh DB with no goose table — that's fine, just run Up.
	_ = goose.Reset(db, ".")
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("testhelpers: failed to migrate test DB: %w", err)
	}
	return nil
}

// TruncateAll truncates all non-system tables in the test DB via TRUNCATE CASCADE.
// Call at the start of each test to ensure isolation.
func TruncateAll(t *testing.T, db *gorm.DB) {
	t.Helper()
	const q = `
		DO $$
		DECLARE r RECORD;
		BEGIN
			FOR r IN (
				SELECT tablename FROM pg_tables
				WHERE schemaname = 'public' AND tablename != 'goose_db_version'
			) LOOP
				EXECUTE 'TRUNCATE TABLE ' || quote_ident(r.tablename) || ' CASCADE';
			END LOOP;
		END $$;
	`
	if err := db.Exec(q).Error; err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}
}

// RequireTestDB skips the test if the test DB is not available.
func RequireTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	if db == nil {
		t.Skip("test DB not available")
	}
}

// SetupTestNATS connects to a local NATS server for pub/sub feature
// tests. Returns a nil conn (not an error) if unreachable — this project's
// established pattern (see SetupTestDB) is graceful skip over hard
// failure when optional local infra isn't running; CI provisions no
// services at all, matching how the DB-backed tests already skip there.
func SetupTestNATS() (*nats.Conn, func()) {
	url := os.Getenv("TEST_NATS_URL")
	if url == "" {
		url = nats.DefaultURL
	}
	nc, err := nats.Connect(url, nats.Timeout(2*time.Second))
	if err != nil {
		return nil, func() {}
	}
	return nc, func() { nc.Close() }
}

// RequireTestNATS skips the test if no local NATS server is reachable.
func RequireTestNATS(t *testing.T, nc *nats.Conn) {
	t.Helper()
	if nc == nil {
		t.Skip("test NATS server not available")
	}
}
