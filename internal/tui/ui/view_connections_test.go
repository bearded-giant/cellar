package ui

import (
	"strings"
	"testing"

	"github.com/jorgerojas26/lazysql/models"
)

func TestRenderConnCard_NeverLeaksURLOrSecret(t *testing.T) {
	m := newTestModel()
	m.Width = 120
	conn := models.Connection{
		Name:     "frost-dev",
		URL:      "postgres://postgres:SUPERSECRETPW@db.internal:5432/postgres",
		Provider: "postgres",
		UseSSH:   true,
	}
	out := m.renderConnCard(conn, true)

	if strings.Contains(out, "SUPERSECRETPW") {
		t.Errorf("card leaks the password:\n%s", out)
	}
	if strings.Contains(out, "db.internal") || strings.Contains(out, "postgres://") {
		t.Errorf("card leaks the connection string:\n%s", out)
	}
	if !strings.Contains(out, "frost-dev") {
		t.Errorf("card missing name:\n%s", out)
	}
	if !strings.Contains(out, "postgres") {
		t.Errorf("card missing provider badge:\n%s", out)
	}
	if !strings.Contains(out, "SSH") {
		t.Errorf("card missing SSH badge:\n%s", out)
	}
}
