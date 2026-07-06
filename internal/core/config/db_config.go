package config

import "time"

type DB struct {
	Host            string        `required:"true"`
	Port            string        `required:"true"`
	User            string        `required:"true"`
	Password        string        `required:"true"`
	Name            string        `required:"true" default:"album"`
	MaxOpenConns    int           `split_words:"true" default:"25"`
	MaxIdleConns    int           `split_words:"true" default:"5"`
	ConnMaxLifetime time.Duration `split_words:"true" default:"5m"`
}

func (DB) Prefix() string {
	return "DB"
}
