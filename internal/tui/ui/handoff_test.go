package ui

import (
	"errors"
	"testing"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
	"github.com/jorgerojas26/lazysql/models"
)

func TestHandoff_LazysqlMissingYieldsError(t *testing.T) {
	orig := lazysqlLookPath
	lazysqlLookPath = func() (string, error) { return "", errors.New("not found") }
	defer func() { lazysqlLookPath = orig }()

	cmd := handoffToLazysql(models.Connection{}, "postgres://x")
	if cmd == nil {
		t.Fatal("handoff returned nil cmd")
	}
	msg, ok := cmd().(types.LazysqlExitedMsg)
	if !ok || msg.Err == nil {
		t.Fatalf("want LazysqlExitedMsg with error, got %#v", cmd())
	}
}

func TestHandleLazysqlExited_ResetsToList(t *testing.T) {
	m := newTestModel()
	m.Screen = types.ScreenTestConnection
	conn := models.Connection{Name: "x"}
	m.CurrentConn = &conn

	res, _ := m.handleLazysqlExitedMsg(types.LazysqlExitedMsg{})
	got := res.(Model)
	if got.Screen != types.ScreenConnections {
		t.Errorf("Screen = %v, want Connections", got.Screen)
	}
	if got.CurrentConn != nil {
		t.Error("CurrentConn not cleared after handoff exit")
	}
}
