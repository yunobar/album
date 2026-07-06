package config

import (
	"errors"

	"github.com/itsLeonB/ungerr"
	"github.com/kelseyhightower/envconfig"
)

// type configurable interface {
// 	Prefix() string
// }

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

	var app App
	if err := envconfig.Process("APP", &app); err != nil {
		errs = errors.Join(errs, err)
	}

	var nats NATS
	if err := envconfig.Process("NATS", &nats); err != nil {
		errs = errors.Join(errs, err)
	}

	var mail Mail
	if err := envconfig.Process("MAIL", &mail); err != nil {
		errs = errors.Join(errs, err)
	}

	oAuthProviders, err := loadOAuthProviderConfig()
	if err != nil {
		errs = errors.Join(errs, err)
	}

	var auth Auth
	if err = envconfig.Process("AUTH", &auth); err != nil {
		errs = errors.Join(errs, err)
	}

	var db DB
	if err = envconfig.Process("DB", &db); err != nil {
		errs = errors.Join(errs, err)
	}

	var otel OTel
	if err = envconfig.Process(otel.Prefix(), &otel); err != nil {
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
