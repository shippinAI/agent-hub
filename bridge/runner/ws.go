package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"devagotchi/bridge/wire"
)

// wsRun keeps a WebSocket to the hub for inbound frames, reconnecting with backoff. Results still go back
// over the HTTP /bridge/v1/result endpoint (hubclient.Send), which the hub routes the same way.
func (r *Runner) WSRun(ctx context.Context) error {
	backoff := time.Second
	for ctx.Err() == nil {
		err := r.wsOnce(ctx)
		if ctx.Err() != nil {
			return nil
		}
		r.Log("ws disconnected: " + errStr(err) + "; retrying")
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
	return nil
}

func (r *Runner) wsOnce(ctx context.Context) error {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+r.Client.Token)
	ws, _, err := websocket.Dial(ctx, r.Client.URL+"/bridge/v1", &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		return err
	}
	defer ws.Close(websocket.StatusNormalClosure, "bye")
	ws.SetReadLimit(32 << 20)
	for {
		_, b, err := ws.Read(ctx)
		if err != nil {
			return err
		}
		var f wire.Frame
		if json.Unmarshal(b, &f) == nil {
			r.Handle(ctx, f)
		}
	}
}

func errStr(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
}
