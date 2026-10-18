package bridgehub

import "agenthub/internal/store"

// ErrBusy means the bridge's queue is full.
var ErrBusy = store.Fail(503, "machine_busy", "the bridge has too many queued calls; try again shortly")

// ErrTimeout means the bridge did not answer before the call deadline.
var ErrTimeout = store.Fail(504, "timeout", "the bridge did not finish the call before its deadline")
