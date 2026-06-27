//go:build unix

package helpers

import (
	"os/exec"
	"syscall"
)

// setProxyProcessGroup puts the ProxyCommand in its own process group so the
// whole group can be killed together — aws ssm forks session-manager-plugin as
// a grandchild that a lone Process.Kill would orphan (leaking the SSM socket).
func setProxyProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProxyProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		return
	}
	_ = cmd.Process.Kill()
}
