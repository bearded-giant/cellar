//go:build unix

package helpers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A backgrounded grandchild (mimicking aws ssm -> session-manager-plugin) must
// die when the ProxyCommand is torn down, not orphan.
func TestKillProxyProcess_KillsGrandchild(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "gc.pid")

	cmd := exec.Command("sh", "-c", "sleep 30 & echo $! > "+pidFile+"; wait")
	setProxyProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	gcPid := 0
	for i := 0; i < 100; i++ {
		if b, err := os.ReadFile(pidFile); err == nil {
			if p, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && p > 0 {
				gcPid = p
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if gcPid == 0 {
		t.Fatal("did not capture grandchild pid")
	}

	killProxyProcess(cmd)
	_ = cmd.Wait()

	dead := false
	for i := 0; i < 100; i++ {
		if err := syscall.Kill(gcPid, 0); err != nil {
			dead = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !dead {
		_ = syscall.Kill(gcPid, syscall.SIGKILL)
		t.Errorf("grandchild %d survived ProxyCommand teardown (orphaned)", gcPid)
	}
}
