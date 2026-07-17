package commands

import (
	"testing"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func TestCancelRunningQuery_NoActiveQuery(t *testing.T) {
	if CancelRunningQuery() {
		t.Error("CancelRunningQuery with nothing running must return false")
	}
}

func TestStartQueryContext_CancelRunningQueryCancelsContext(t *testing.T) {
	ctx, done := StartQueryContext()
	defer done()

	if !CancelRunningQuery() {
		t.Fatal("CancelRunningQuery must report a running query")
	}

	select {
	case <-ctx.Done():
	default:
		t.Error("context must be cancelled after CancelRunningQuery")
	}
}

func TestStartQueryContext_ReleaseClearsRegistry(t *testing.T) {
	ctx, done := StartQueryContext()
	done()

	select {
	case <-ctx.Done():
	default:
		t.Error("release must cancel the context to free resources")
	}

	if CancelRunningQuery() {
		t.Error("registry must be cleared once the query completes")
	}
}

func TestStartQueryContext_NewQueryReplacesOld(t *testing.T) {
	_, done1 := StartQueryContext()
	ctx2, done2 := StartQueryContext()
	defer done2()

	// releasing the stale first query must not clear the second's registration
	done1()

	if !CancelRunningQuery() {
		t.Fatal("second query must still be cancellable after first released")
	}
	select {
	case <-ctx2.Done():
	default:
		t.Error("second query's context must be cancelled")
	}
}

func TestRunQuery_ReleasesQueryContext(t *testing.T) {
	stub := &stubDriver{queryRows: [][]string{{"id"}}}
	c := &Commands{}
	_ = c.RunQuery(stub, "SELECT 1", false, "")().(types.QueryExecutedMsg)

	if CancelRunningQuery() {
		t.Error("query context must be released after RunQuery completes")
	}
}
