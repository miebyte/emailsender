package email

import (
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"
)

func (m Message) Validate() error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"from", m.From},
		{"to", m.To},
	} {
		if !utf8.ValidString(field.value) || utf8.RuneCountInString(field.value) > 320 || strings.ContainsAny(field.value, "\r\n") {
			return fmt.Errorf("%w: %s must be an address of at most 320 characters without line breaks", ErrInvalidMessage, field.name)
		}
		if _, err := mail.ParseAddress(field.value); err != nil {
			return fmt.Errorf("%w: %s must contain one valid email address", ErrInvalidMessage, field.name)
		}
	}
	if !utf8.ValidString(m.Subject) || utf8.RuneCountInString(m.Subject) > 255 || strings.ContainsAny(m.Subject, "\r\n") {
		return fmt.Errorf("%w: subject must be at most 255 characters without line breaks", ErrInvalidMessage)
	}
	if !utf8.ValidString(m.Body) || len(m.Body) > 65535 {
		return fmt.Errorf("%w: body must be valid UTF-8 of at most 65535 bytes", ErrInvalidMessage)
	}
	return nil
}
