package config

import "time"

type OTel struct {
	Enabled              bool          `required:"true" default:"false"`
	ExporterOtlpEndpoint string        `split_words:"true" required:"true"`
	ExporterOtlpInsecure bool          `split_words:"true" required:"true" default:"false"`
	ExporterOtlpHeaders  string        `split_words:"true" required:"true"`
	ServiceName          string        `split_words:"true" required:"true" default:"album"`
	ServiceInstanceId    string        `split_words:"true" required:"true"`
	MaxQueueSize         int           `split_words:"true" required:"true" default:"256"`
	MaxExportBatchSize   int           `split_words:"true" required:"true" default:"64"`
	BatchTimeout         time.Duration `split_words:"true" required:"true" default:"3s"`
	ExportTimeout        time.Duration `split_words:"true" required:"true" default:"3s"`
	MetricsEnabled       bool          `split_words:"true" required:"true" default:"false"`
	LogsEnabled          bool          `split_words:"true" required:"true" default:"false"`
	TracesEnabled        bool          `split_words:"true" required:"true" default:"false"`
}

func (o OTel) Prefix() string {
	return "OTEL"
}
