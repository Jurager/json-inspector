package secrets

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

var ErrUnavailable = errors.New("keychain unavailable")

const service = "json-inspector"

// What the `security` CLI exits with when the item is simply not there.
const notFoundExit = 44

func account(envID, name string) string {
	return "env:" + envID + ":" + name
}

func keychainAvailable() bool {
	return runtime.GOOS == "darwin"
}

func runSecurity(args ...string) ([]byte, error) {
	return exec.Command("/usr/bin/security", args...).Output()
}

func Set(envID, name, value string) error {
	if !keychainAvailable() {
		return ErrUnavailable
	}
	out, err := runSecurity("add-generic-password", "-U", "-s", service, "-a", account(envID, name), "-w", value)
	if err != nil {
		return fmt.Errorf("keychain write failed: %s", errorDetail(err, out))
	}
	return nil
}

func Get(envID, name string) (string, error) {
	if !keychainAvailable() {
		return "", ErrUnavailable
	}
	out, err := runSecurity("find-generic-password", "-s", service, "-a", account(envID, name), "-w")
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == notFoundExit {
			return "", nil
		}
		return "", fmt.Errorf("keychain read failed: %s", errorDetail(err, out))
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func Delete(envID, name string) error {
	if !keychainAvailable() {
		return ErrUnavailable
	}
	out, err := runSecurity("delete-generic-password", "-s", service, "-a", account(envID, name))
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == notFoundExit {
			return nil
		}
		return fmt.Errorf("keychain delete failed: %s", errorDetail(err, out))
	}
	return nil
}

func errorDetail(err error, out []byte) string {
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return err.Error()
	}
	return msg
}
