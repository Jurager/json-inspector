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

const notFoundExit = 44

func account(envID, name string) string {
	return "env:" + envID + ":" + name
}

func available() bool {
	return runtime.GOOS == "darwin"
}

func run(args ...string) ([]byte, error) {
	return exec.Command("/usr/bin/security", args...).Output()
}

func Set(envID, name, value string) error {
	if !available() {
		return ErrUnavailable
	}
	out, err := run("add-generic-password", "-U", "-s", service, "-a", account(envID, name), "-w", value)
	if err != nil {
		return fmt.Errorf("keychain write failed: %s", detail(err, out))
	}
	return nil
}

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
	return strings.TrimRight(string(out), "\n"), nil
}

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

func detail(err error, out []byte) string {
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return err.Error()
	}
	return msg
}
