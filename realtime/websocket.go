package realtime

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const (
	defaultMaxMessageBytes = 1 << 20
	defaultConnectionAge   = 15 * time.Minute
)

// WebSocketHandler receives a connected WebSocket and owns its read/write
// loop. The handler must return when ctx is canceled or the peer disconnects.
type WebSocketHandler func(context.Context, *websocket.Conn) error

// WebSocket upgrades a request and invokes handler. Origin validation should
// be configured explicitly in production.
func WebSocket(handler WebSocketHandler, options *websocket.AcceptOptions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, options)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		conn.SetReadLimit(defaultMaxMessageBytes)
		ctx, cancel := context.WithTimeout(r.Context(), defaultConnectionAge)
		defer cancel()
		if err := handler(ctx, conn); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			_ = conn.Close(websocket.StatusInternalError, "internal server error")
		}
	})
}

// Echo returns a small development handler that sends each message back to
// the browser. Replace it with an application-specific authenticated handler.
func Echo() WebSocketHandler {
	return func(ctx context.Context, conn *websocket.Conn) error {
		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				return err
			}
			if err := conn.Write(ctx, typ, data); err != nil {
				return err
			}
		}
	}
}
