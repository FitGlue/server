package email

import "context"

// Sender defines an interface for sending stylized emails.
type Sender interface {
	SendEmail(ctx context.Context, to string, subject string, htmlContent string) error
}
