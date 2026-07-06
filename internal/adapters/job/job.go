package job

import (
	"github.com/yunobar/album/internal/adapters/job/migrate"
	"github.com/yunobar/album/internal/core/config"
	"github.com/yunobar/album/internal/core/logger"
	"github.com/yunobar/album/internal/provider"
)

type Job struct {
	providers *provider.Providers
	migrate   *migrate.Migrate
}

func Setup(cfg *config.Config) (*Job, error) {
	providers, err := provider.All()
	if err != nil {
		return nil, err
	}

	migrator, err := migrate.Setup(providers)
	if err != nil {
		if e := providers.Shutdown(); e != nil {
			logger.Error(e)
		}
		return nil, err
	}

	return &Job{providers, migrator}, nil
}

func (j *Job) Run() {
	logger.Info("running all jobs...")

	defer func() {
		if err := j.providers.Shutdown(); err != nil {
			logger.Error(err)
		}
	}()

	logger.Info("running migrations...")
	if err := j.migrate.Run(); err != nil {
		logger.Error(err)
		return
	}
	logger.Info("success running migrations")

	logger.Info("success running all jobs")
}
