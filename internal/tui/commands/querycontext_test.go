package commands

import (
	"testing"

	"github.com/bearded-giant/cellar/internal/tui/config"
	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/models"
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

func TestQueryRowLimit_Resolution(t *testing.T) {
	cases := []struct {
		name string
		c    *Commands
		want int
	}{
		{"nil cfg", &Commands{}, 5000},
		{"zero means default", New(&config.Config{AppConfig: &models.AppConfig{}}), 5000},
		{"explicit", New(&config.Config{AppConfig: &models.AppConfig{QueryRowLimit: 100}}), 100},
		{"negative means unlimited", New(&config.Config{AppConfig: &models.AppConfig{QueryRowLimit: -1}}), 0},
	}
	for _, tc := range cases {
		if got := tc.c.queryRowLimit(); got != tc.want {
			t.Errorf("%s: queryRowLimit() = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestCapQueryRows(t *testing.T) {
	rows := [][]string{{"id"}, {"1"}, {"2"}, {"3"}}
	got, total, truncated := capQueryRows(rows, 3, 2)
	if !truncated || total != 2 || len(got) != 3 {
		t.Errorf("cap: rows=%d total=%d truncated=%v", len(got), total, truncated)
	}
	got, total, truncated = capQueryRows(rows, 3, 0)
	if truncated || total != 3 || len(got) != 4 {
		t.Error("limit 0 must not cap")
	}
	got, total, truncated = capQueryRows(rows, 3, 3)
	if truncated || total != 3 {
		t.Error("at-limit result must not flag truncation")
	}
}
