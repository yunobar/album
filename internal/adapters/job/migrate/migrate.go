package migrate

import (
	"database/sql"

	"github.com/itsLeonB/ungerr"
	"github.com/pressly/goose/v3"
	"github.com/yunobar/album/internal/adapters/db/postgres/migrations"
	"github.com/yunobar/album/internal/core/logger"
	"github.com/yunobar/album/internal/provider"
)

type Migrate struct {
	db *sql.DB
}

func Setup(providers *provider.Providers) (*Migrate, error) {
	goose.SetBaseFS(migrations.Migrations)
	goose.SetLogger(logger.Global)

	if err := goose.SetDialect("postgres"); err != nil {
		return nil, ungerr.Wrap(err, "error setting migrator dialect to postgres")
	}

	return &Migrate{providers.SQL}, nil
}

func (m *Migrate) Run() error {
	if err := goose.Up(m.db, "."); err != nil {
		return ungerr.Wrap(err, "error running migrations")
	}
	return nil
}
