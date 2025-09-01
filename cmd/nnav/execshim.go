package main

import (
	"os"
	"os/exec"
)

// execCommand wraps exec.Command to automatically connect the child process
// to the current process’s standard input/output/error streams.
//
// Purpose:
//   - Ensures the spawned process (e.g., an editor) behaves interactively,
//     as if launched directly from the user’s shell.
//   - Avoids the need for explicit I/O wiring at every call site.
//
// Returns: a ready-to-run *exec.Cmd that can be started with Run/Start.
func execCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)

	// Inherit terminal input and output so the launched command
	// feels native to the user’s environment.
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	return cmd
}

