// Package secrets keeps environment secrets in the OS keychain instead of the
// app's localStorage, so a leaked storage file can't hand over credentials.
//
// macOS is served by the `security` tool, which is the same generic-password
// service the Keychain Access UI writes to. Other platforms return
// ErrUnavailable: the caller keeps the secrets in memory for the session and
// says so in the UI rather than silently pretending they were saved.
package secrets

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// ErrUnavailable reports that this build has no keychain backend to talk to.
// It is not a failure of the user's system — the app is expected to carry on.
var ErrUnavailable = errors.New("keychain unavailable")

// service groups every item this app owns, so a user can find (and revoke) them
// in Keychain Access under one name.
const service = "json-inspector"

// notFoundExit is what `security` returns when the item simply isn't there.
const notFoundExit = 44

// account is the per-variable key. The environment id is part of it because
// keychain items are addressed by (service, account) — adding the id to the
// account string is what lets two environments hold a `token` each.
func account(envID, name string) string {
	return "env:" + envID + ":" + name
}

func available() bool {
	return runtime.GOOS == "darwin"
}

func run(args ...string) ([]byte, error) {
	return exec.Command("/usr/bin/security", args...).Output()
}

// Set writes (or rewrites) one secret.
func Set(envID, name, value string) error {
	if !available() {
		return ErrUnavailable
	}
	// -U updates in place, so saving the same variable twice behaves the way
	// the editor implies instead of erroring on a duplicate item.
	out, err := run("add-generic-password", "-U", "-s", service, "-a", account(envID, name), "-w", value)
	if err != nil {
		return fmt.Errorf("keychain write failed: %s", detail(err, out))
	}
	return nil
}

// Get reads one secret. A secret that was never saved yields an empty string
// and no error — "not there yet" is a normal state for a new variable, and
// reporting it as a failure would tell the user their keychain is broken.
func Get(envID, name string) (string, error) {
	if !available() {
		return "", ErrUnavailable
	}
	out, err := run("find-generic-password", "-s", service, "-a", account(envID, name), "-w")
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == notFoundExit {
			return "", nil
		}
		return "", fmt.Errorf("keychain read failed: %s", detail(err, out))
	}
	// `security` terminates the value with a newline that isn't part of it.
	return strings.TrimRight(string(out), "\n"), nil
}

// Delete removes one secret. Removing something that isn't stored is a no-op,
// so callers don't have to check first.
func Delete(envID, name string) error {
	if !available() {
		return ErrUnavailable
	}
	out, err := run("delete-generic-password", "-s", service, "-a", account(envID, name))
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == notFoundExit {
			return nil
		}
		return fmt.Errorf("keychain delete failed: %s", detail(err, out))
	}
	return nil
}

// There is deliberately no List: enumerating items would mean parsing
// `security dump-keychain`, which can raise a keychain access prompt merely for
// opening the app. The frontend already knows every variable name from its own
// state, so it reads them one by one instead.

func detail(err error, out []byte) string {
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return err.Error()
	}
	return msg
}
