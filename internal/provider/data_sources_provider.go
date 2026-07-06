package provider

import (
	"database/sql"
	"fmt"

	ezgorm "github.com/itsLeonB/ezutil/v2/gorm"
	"github.com/itsLeonB/ungerr"
	"github.com/yunobar/album/internal/core/config"
	"github.com/yunobar/album/internal/core/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DataSources struct {
	Gorm *gorm.DB
	SQL  *sql.DB
}

func (ds *DataSources) Shutdown() error {
	if err := ds.SQL.Close(); err != nil {
		return ungerr.Wrap(err, "error closing SQL db")
	}
	return nil
}

func ProvideDataSource(cfg config.DB) (*DataSources, error) {
	gormDB, err := gorm.Open(postgres.Open(dsn(cfg)), &gorm.Config{
		Logger: ezgorm.NewGormLogger(logger.Global),
	})
	if err != nil {
		return nil, ungerr.Wrap(err, "error opening gorm connection")
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, ungerr.Wrap(err, "error obtaining sql.DB instance")
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err = sqlDB.Ping(); err != nil {
		return nil, ungerr.Wrap(err, "error pinging SQL DB")
	}

	return &DataSources{
		Gorm: gormDB,
		SQL:  sqlDB,
	}, nil
}

func dsn(cfg config.DB) string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s",
		cfg.Host,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.Port,
	)
}
