package notify

import (
	"context"
	"fmt"
	"time"

	"github.com/wneessen/go-mail"

	"github.com/rejisterjack/tock/internal/model"
)

// Email sends alerts over SMTP. Host may be empty; Send then returns an error
// the scheduler stores on the alert row.
type Email struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// Send implements Sender.
func (e Email) Send(ctx context.Context, ch model.Channel, msg Message) error {
	if e.Host == "" {
		return fmt.Errorf("SMTP is not configured")
	}
	if e.From == "" {
		return fmt.Errorf("TOCK_SMTP_FROM is not set")
	}
	cfg, err := ParseEmail(ch.Config)
	if err != nil {
		return err
	}
	if len(cfg.To) == 0 {
		return fmt.Errorf("email channel has no recipients")
	}
	m := mail.NewMsg()
	if err := m.From(e.From); err != nil {
		return err
	}
	if err := m.To(cfg.To...); err != nil {
		return err
	}
	m.Subject(Subject(msg))
	m.SetBodyString(mail.TypeTextPlain, Text(msg))

	opts := []mail.Option{
		mail.WithPort(e.Port),
		mail.WithTimeout(10 * time.Second),
		mail.WithTLSPolicy(mail.TLSOpportunistic),
	}
	if e.Port == 465 {
		opts = append(opts, mail.WithSSL(), mail.WithTLSPolicy(mail.TLSMandatory))
	}
	if e.Username != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(e.Username),
			mail.WithPassword(e.Password),
		)
	}
	client, err := mail.NewClient(e.Host, opts...)
	if err != nil {
		return err
	}
	return client.DialAndSendWithContext(ctx, m)
}
