package config

import "github.com/kelseyhightower/envconfig"

type OAuthProviders struct {
	Google OAuthProvider
}

type OAuthProvider struct {
	ClientID     string `split_words:"true" required:"true"`
	ClientSecret string `split_words:"true" required:"true"`
	RedirectUrl  string `split_words:"true" required:"true"`
}

func loadOAuthProviderConfig() (OAuthProviders, error) {
	var oAuthGoogle OAuthProvider
	if err := envconfig.Process("OAUTH_GOOGLE", &oAuthGoogle); err != nil {
		return OAuthProviders{}, err
	}

	return OAuthProviders{
		Google: oAuthGoogle,
	}, nil
}
