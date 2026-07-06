package provider

import (
	"errors"

	"github.com/itsLeonB/ungerr"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/yunobar/album/internal/core/config"
	"github.com/yunobar/album/internal/core/mail"
	"github.com/yunobar/album/internal/core/store"
)

type CoreServices struct {
	Mail  mail.MailService
	State store.StateStore

	NATSConn  *nats.Conn
	JetStream jetstream.JetStream
}

func (cs *CoreServices) Shutdown() error {
	var errs error
	if e := cs.State.Shutdown(); e != nil {
		errs = errors.Join(errs, e)
	}
	if e := cs.NATSConn.Drain(); e != nil {
		errs = errors.Join(errs, e)
	}
	return errs
}

func ProvideCoreServices() (*CoreServices, error) {
	nc, err := nats.Connect(config.Global.Url)
	if err != nil {
		return nil, ungerr.Wrap(err, "error connecting to NATS")
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, ungerr.Wrap(err, "error creating JetStream context")
	}

	stateStore, err := store.NewStateStore(js)
	if err != nil {
		nc.Close()
		return nil, err
	}

	return &CoreServices{
		Mail:  mail.NewMailService(config.Global.Mail),
		State: stateStore,

		NATSConn:  nc,
		JetStream: js,
	}, nil
}
