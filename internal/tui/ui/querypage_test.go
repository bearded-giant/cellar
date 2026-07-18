package ui

import (
	"fmt"
	"github.com/bearded-giant/cellar/drivers"
	"testing"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func TestQueryResult_ClientPaging(t *testing.T) {
	m := browseModel()
	m.Browse.Limit = 100
	m.ActiveDriver = &drivers.SQLite{}
	m.QueryRunning = true
	m.Screen = types.ScreenEditor

	// build a 250-row result (+ header)
	rows := [][]string{{"id"}}
	for i := 0; i < 250; i++ {
		rows = append(rows, []string{fmt.Sprintf("%d", i)})
	}
	res, _ := m.handleQueryExecutedMsg(types.QueryExecutedMsg{IsSelect: true, Rows: rows})
	m = res.(Model)

	if m.Browse.Total != 250 {
		t.Fatalf("Total = %d, want 250", m.Browse.Total)
	}
	if len(m.Browse.Rows) != 100 {
		t.Fatalf("first page = %d rows, want 100", len(m.Browse.Rows))
	}
	if m.Browse.Rows[0][0] != "0" {
		t.Errorf("page0 first row = %q, want 0", m.Browse.Rows[0][0])
	}

	// next page (client-side, no driver)
	m = m.pageQueryResult(+1).(Model)
	if m.Browse.Offset != 100 || m.Browse.Rows[0][0] != "100" {
		t.Errorf("page1 offset=%d first=%q, want 100/100", m.Browse.Offset, m.Browse.Rows[0][0])
	}

	// last page is partial (250-200 = 50)
	m = m.pageQueryResult(+1).(Model)
	if m.Browse.Offset != 200 || len(m.Browse.Rows) != 50 {
		t.Errorf("page2 offset=%d len=%d, want 200/50", m.Browse.Offset, len(m.Browse.Rows))
	}

	// can't page past the end
	before := m.Browse.Offset
	m = m.pageQueryResult(+1).(Model)
	if m.Browse.Offset != before {
		t.Error("should not page past the last page")
	}
}
