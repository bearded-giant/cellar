//go:build !unix

package helpers

import "os/exec"

func setProxyProcessGroup(*exec.Cmd) {}

func killProxyProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
