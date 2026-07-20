package pubsub

import (
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// Publisher is a thin, technology-agnostic wrapper so domain services
// depend on an interface (mockable, matching every other core service in
// this codebase — cache.Cache, mail.MailService, store.StateStore) rather
// than a concrete NATS type.
type Publisher interface {
	Publish(subject string, data []byte) error
}

type natsPublisher struct {
	nc *nats.Conn
}

func NewPublisher(nc *nats.Conn) Publisher {
	return &natsPublisher{nc}
}

func (p *natsPublisher) Publish(subject string, data []byte) error {
	return p.nc.Publish(subject, data)
}

// LiveSubject is the one-channel-per-session subject every publisher and
// subscriber for a given session must agree on.
func LiveSubject(sessionID uuid.UUID) string {
	return "decision-sessions." + sessionID.String() + ".live"
}
