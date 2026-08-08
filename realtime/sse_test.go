package realtime

import (
	"bytes"
	"testing"
)

func TestWriteEventEscapesLineBreaks(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	writeEvent(&out, Event{
		ID:   "1\nevil",
		Name: "update\r\nevil",
		Data: []byte("first\n\nevent: injected"),
	})
	const want = "id: 1evil\nevent: updateevil\ndata: first\ndata: \ndata: event: injected\n\n"
	if got := out.String(); got != want {
		t.Fatalf("writeEvent() = %q, want %q", got, want)
	}
}
