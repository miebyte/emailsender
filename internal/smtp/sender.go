package smtp

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	netsmtp "net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/miebyte/emailsender/internal/email"
)

var _ email.Sender = (*Sender)(nil)

// Sender keeps SMTP details at the infrastructure boundary. goutils/emailutils
// currently fixes the body to HTML and does not support cancellation or timeouts.
type Sender struct {
	config    Config
	account   string
	tlsConfig *tls.Config
}

func NewSender(config Config) (*Sender, error) {
	config.SetDefault()
	if err := config.Validate(); err != nil {
		return nil, err
	}
	account, _ := mail.ParseAddress(config.Username)
	return &Sender{
		config:  config,
		account: account.Address,
		tlsConfig: &tls.Config{
			ServerName: config.Host,
			MinVersion: tls.VersionTLS12,
		},
	}, nil
}

func (s *Sender) Send(ctx context.Context, message email.Message) error {
	data, recipient, err := buildMessage(message)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.config.TimeoutSeconds)*time.Second)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port)))
	if err != nil {
		return sendError(ctx, "connect", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return sendError(ctx, "set deadline", err)
	}
	tlsConn := tls.Client(conn, s.tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return sendError(ctx, "tls handshake", err)
	}
	client, err := netsmtp.NewClient(tlsConn, s.config.Host)
	if err != nil {
		return sendError(ctx, "greeting", err)
	}
	defer client.Close()
	if err := client.Auth(netsmtp.PlainAuth("", s.account, s.config.Password, s.config.Host)); err != nil {
		return sendError(ctx, "authenticate", err)
	}
	if err := client.Mail(s.account); err != nil {
		return sendError(ctx, "mail from", err)
	}
	if err := client.Rcpt(recipient); err != nil {
		return sendError(ctx, "recipient", err)
	}
	writer, err := client.Data()
	if err != nil {
		return sendError(ctx, "data", err)
	}
	if _, err := writer.Write(data); err != nil {
		return sendError(ctx, "write message", err)
	}
	if err := writer.Close(); err != nil {
		return sendError(ctx, "submit message", err)
	}
	// DATA's final successful response confirms acceptance. A failed QUIT cannot
	// undo that acceptance and must not encourage a duplicate delivery.
	_ = client.Quit()
	return nil
}

func sendError(ctx context.Context, operation string, err error) error {
	if ctx.Err() != nil {
		err = ctx.Err()
	} else if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		err = context.DeadlineExceeded
	}
	return fmt.Errorf("smtp %s: %w", operation, err)
}

func buildMessage(message email.Message) ([]byte, string, error) {
	if strings.ContainsAny(message.From+message.To+message.Subject, "\r\n") {
		return nil, "", fmt.Errorf("%w: invalid email header", email.ErrInvalidMessage)
	}
	from, err := mail.ParseAddress(message.From)
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid sender", email.ErrInvalidMessage)
	}
	to, err := mail.ParseAddress(message.To)
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid recipient", email.ErrInvalidMessage)
	}
	var data bytes.Buffer
	fmt.Fprintf(&data, "From: %s\r\nTo: %s\r\nSubject: %s\r\n", from.String(), to.String(), mime.QEncoding.Encode("UTF-8", message.Subject))
	fmt.Fprintf(&data, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	data.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n")
	writer := quotedprintable.NewWriter(&data)
	if _, err := writer.Write([]byte(message.Body)); err != nil {
		return nil, "", fmt.Errorf("encode email body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("encode email body: %w", err)
	}
	return data.Bytes(), to.Address, nil
}
