package config

type NATS struct {
	Url              string `required:"true"`
	StateStoreBucket string `split_words:"true" default:"state-store"`
}

func (NATS) Prefix() string {
	return "NATS"
}
