package config

import (
	"errors"

	"github.com/itsLeonB/ungerr"
	"github.com/kelseyhightower/envconfig"
)

type configurable interface {
	Prefix() string
}

func load[T configurable]() (T, error) {
	var t T
	err := envconfig.Process(t.Prefix(), &t)
	return t, err
}

type Config struct {
	App
	Auth
	DB
	Mail
	OAuthProviders
	NATS
	OTel
}

var Global *Config

func Load() error {
	var errs error

	app, err := load[App]()
	if err != nil {
		errs = errors.Join(errs, err)
	}

	nats, err := load[NATS]()
	if err != nil {
		errs = errors.Join(errs, err)
	}

	mail, err := load[Mail]()
	if err != nil {
		errs = errors.Join(errs, err)
	}

	oAuthProviders, err := loadOAuthProviderConfig()
	if err != nil {
		errs = errors.Join(errs, err)
	}

	auth, err := load[Auth]()
	if err != nil {
		errs = errors.Join(errs, err)
	}

	db, err := load[DB]()
	if err != nil {
		errs = errors.Join(errs, err)
	}

	otel, err := load[OTel]()
	if err != nil {
		errs = errors.Join(errs, err)
	}

	if errs != nil {
		return ungerr.Wrap(errs, "error loading config")
	}

	Global = &Config{
		app,
		auth,
		db,
		mail,
		oAuthProviders,
		nats,
		otel,
	}

	return nil
}
