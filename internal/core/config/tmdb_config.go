package config

type TMDB struct {
	BaseUrl string `split_words:"true" required:"true"`
	ApiKey  string `split_words:"true" required:"true"`
}

func (TMDB) Prefix() string {
	return "TMDB"
}
