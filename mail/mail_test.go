package mail

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAddressValidate(t *testing.T) {
	t.Parallel()
	if err := (Address{Name: "Ada Lovelace", Email: "ada@example.com"}).Validate(); err != nil {
		t.Fatalf("valid address error = %v", err)
	}
	for _, address := range []Address{
		{Name: "Ada\r\nBcc: attacker@example.com", Email: "ada@example.com"},
		{Email: "Ada <ada@example.com>"},
		{Email: "not an address"},
		{Email: strings.Repeat("a", MaxAddressBytes+1)},
	} {
		if err := address.Validate(); !errors.Is(err, ErrInvalidAddress) {
			t.Errorf("Address(%+v).Validate() error = %v, want %v", address, err, ErrInvalidAddress)
		}
	}
}

func TestMessageValidate(t *testing.T) {
	t.Parallel()
	from := Address{Email: "sender@example.com"}
	to := Address{Email: "recipient@example.com"}
	valid := Message{
		From:    from,
		To:      []Address{to},
		Subject: "Monthly report",
		Text:    "The report is ready.",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid message error = %v", err)
	}

	overstatedRecipients := make([]Address, MaxRecipients+1)
	for i := range overstatedRecipients {
		overstatedRecipients[i] = to
	}
	for _, message := range []Message{
		{From: from},
		{From: from, To: []Address{to}, Subject: "safe\r\nBcc: attacker@example.com"},
		{From: from, To: overstatedRecipients},
		{From: from, To: []Address{to}, Text: strings.Repeat("a", MaxBodyBytes), HTML: "b"},
	} {
		if err := message.Validate(); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("Message.Validate() error = %v, want %v", err, ErrInvalidMessage)
		}
	}
}

func TestSenderFuncValidatesBeforeSending(t *testing.T) {
	t.Parallel()
	called := false
	sender := SenderFunc(func(context.Context, Message) error {
		called = true
		return nil
	})
	message := Message{
		From: Address{Email: "sender@example.com"},
		To:   []Address{{Email: "recipient@example.com"}},
	}
	if err := sender.Send(t.Context(), message); err != nil || !called {
		t.Fatalf("Send() = %v, called = %t", err, called)
	}
	called = false
	if err := sender.Send(t.Context(), Message{}); !errors.Is(err, ErrInvalidMessage) || called {
		t.Fatalf("invalid Send() = %v, called = %t", err, called)
	}
	var unavailable SenderFunc
	if err := unavailable.Send(t.Context(), message); !errors.Is(err, ErrSenderUnavailable) {
		t.Fatalf("nil SenderFunc error = %v, want %v", err, ErrSenderUnavailable)
	}
}
