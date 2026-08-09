// Package mail defines validated, provider-neutral outgoing mail messages.
package mail

import (
	"context"
	"errors"
	netmail "net/mail"
	"strings"
	"unicode/utf8"
)

const (
	// MaxAddressBytes limits the addr-spec portion of an address.
	MaxAddressBytes = 320
	// MaxNameBytes limits an address display name.
	MaxNameBytes = 256
	// MaxRecipients limits all To, CC, and BCC recipients on one message.
	MaxRecipients = 100
	// MaxSubjectBytes keeps the subject within one RFC 5322 header line.
	MaxSubjectBytes = 998
	// MaxBodyBytes limits the combined text and HTML body size.
	MaxBodyBytes = 1 << 20
)

var (
	// ErrInvalidAddress reports an address that cannot be sent safely.
	ErrInvalidAddress = errors.New("invalid mail address")
	// ErrInvalidMessage reports a message outside the portable sender contract.
	ErrInvalidMessage = errors.New("invalid mail message")
	// ErrSenderUnavailable reports an attempt to call a nil SenderFunc.
	ErrSenderUnavailable = errors.New("mail sender unavailable")
)

// Address is an RFC 5322 mailbox. Email must be a bare addr-spec; Name is an
// optional display name that a sender may encode for its transport.
type Address struct {
	Name  string
	Email string
}

// Validate reports whether a is a safe, portable mailbox.
func (a Address) Validate() error {
	if len(a.Email) == 0 || len(a.Email) > MaxAddressBytes || !validHeaderText(a.Email, MaxAddressBytes) {
		return ErrInvalidAddress
	}
	parsed, err := netmail.ParseAddress(a.Email)
	if err != nil || parsed.Address != a.Email {
		return ErrInvalidAddress
	}
	if !validHeaderText(a.Name, MaxNameBytes) {
		return ErrInvalidAddress
	}
	return nil
}

// Message is an outgoing email with optional plain text and HTML alternatives.
// A Sender must not retain or mutate the message after Send returns.
type Message struct {
	From    Address
	To      []Address
	CC      []Address
	BCC     []Address
	ReplyTo *Address
	Subject string
	Text    string
	HTML    string
}

// Validate reports whether m is bounded and safe to hand to a Sender.
func (m Message) Validate() error {
	if err := m.From.Validate(); err != nil {
		return ErrInvalidMessage
	}
	if total := len(m.To) + len(m.CC) + len(m.BCC); total == 0 || total > MaxRecipients {
		return ErrInvalidMessage
	}
	for _, recipients := range [][]Address{m.To, m.CC, m.BCC} {
		for _, recipient := range recipients {
			if err := recipient.Validate(); err != nil {
				return ErrInvalidMessage
			}
		}
	}
	if m.ReplyTo != nil {
		if err := m.ReplyTo.Validate(); err != nil {
			return ErrInvalidMessage
		}
	}
	if !validHeaderText(m.Subject, MaxSubjectBytes) || !validBody(m.Text) || !validBody(m.HTML) || len(m.Text)+len(m.HTML) > MaxBodyBytes {
		return ErrInvalidMessage
	}
	return nil
}

// Sender delivers one validated message. Implementations own transport policy,
// retries, and credentials; callers own any retry scheduling.
type Sender interface {
	Send(context.Context, Message) error
}

// SenderFunc adapts a function to Sender.
type SenderFunc func(context.Context, Message) error

// Send validates message and calls f.
func (f SenderFunc) Send(ctx context.Context, message Message) error {
	if f == nil {
		return ErrSenderUnavailable
	}
	if err := message.Validate(); err != nil {
		return err
	}
	return f(ctx, message)
}

func validHeaderText(value string, limit int) bool {
	if len(value) > limit || !utf8.ValidString(value) {
		return false
	}
	return !strings.ContainsAny(value, "\r\n\x00")
}

func validBody(value string) bool {
	return len(value) <= MaxBodyBytes && utf8.ValidString(value)
}
