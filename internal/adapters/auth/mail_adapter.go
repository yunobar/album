package authadapter

import (
	"context"
	"fmt"

	"github.com/itsLeonB/go-authkit"
	"github.com/yunobar/album/internal/core/mail"
)

type mailAdapter struct {
	inner mail.MailService
}

func NewMailAdapter(inner mail.MailService) authkit.MailService {
	return &mailAdapter{inner}
}

func (a *mailAdapter) SendVerification(ctx context.Context, email, name, url string) error {
	return a.inner.Send(ctx, mail.MailMessage{
		RecipientMail: email,
		RecipientName: name,
		Subject:       "Verify your email",
		TextContent:   fmt.Sprintf("Please verify your email by clicking the following link:\n\n%s", url),
	})
}

func (a *mailAdapter) SendPasswordReset(ctx context.Context, email, name, url string) error {
	return a.inner.Send(ctx, mail.MailMessage{
		RecipientMail: email,
		RecipientName: name,
		Subject:       "Reset your password",
		TextContent:   fmt.Sprintf("You have requested to reset your password.\nIf this is not you, ignore this mail.\nPlease reset your password by clicking the following link:\n\n%s", url),
	})
}
