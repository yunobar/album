package config

import (
	"net/http"
	"time"
)

type Auth struct {
	SecretKey            string        `split_words:"true" default:"thisissecret"`
	TokenDuration        time.Duration `split_words:"true" default:"24h"`
	RefreshTokenDuration time.Duration `split_words:"true" default:"720h"`
	Issuer               string        `default:"album"`
	HashCost             int           `split_words:"true" default:"10"`
	StateStore           string        `split_words:"true" default:"inmemory"`
	CookieDomain         string        `split_words:"true" default:"localhost"`
	CookieSecure         bool          `split_words:"true" default:"false"`
	CookieSameSite       string        `split_words:"true" default:"strict"`
	TurnstileSecretKey   string        `split_words:"true"`
}

func (a Auth) ParsedSameSite() http.SameSite {
	switch a.CookieSameSite {
	case "none":
		return http.SameSiteNoneMode
	case "lax":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteStrictMode
	}
}

func (Auth) Prefix() string {
	return "AUTH"
}
