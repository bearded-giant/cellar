package commands

import (
	"context"
	"sync"
)

// the running query's cancel is held package-wide so the ui can cancel it
// without threading a handle through tea.Cmd signatures
type queryHandle struct {
	cancel context.CancelFunc
}

var (
	queryMu     sync.Mutex
	activeQuery *queryHandle
)

// StartQueryContext registers a cancellable context as the running query and
// returns it with a release func the command defers on completion. Releasing
// also cancels, freeing the context's resources.
func StartQueryContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	h := &queryHandle{cancel: cancel}
	queryMu.Lock()
	activeQuery = h
	queryMu.Unlock()

	release := func() {
		cancel()
		queryMu.Lock()
		if activeQuery == h {
			activeQuery = nil
		}
		queryMu.Unlock()
	}
	return ctx, release
}

// CancelRunningQuery cancels the in-flight query context, if any. Returns
// whether a query was running.
func CancelRunningQuery() bool {
	queryMu.Lock()
	h := activeQuery
	activeQuery = nil
	queryMu.Unlock()

	if h == nil {
		return false
	}
	h.cancel()
	return true
}
