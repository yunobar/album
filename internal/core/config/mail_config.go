package config

type Mail struct {
	SenderMail string `split_words:"true" required:"true"`
	SenderName string `split_words:"true" required:"true"`
	ApiKey     string `split_words:"true" required:"true"`
}

func (Mail) Prefix() string {
	return "MAIL"
}
