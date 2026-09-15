package updater

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"
)

// When the last check ran. The file is what keeps a launch from asking GitHub again a minute
// after the last one, and losing it costs a check, not the app.

type checkState struct {
	LastCheck time.Time `json:"last_check"`
	Latest    string    `json:"latest"`
}

func checkStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".json-inspector.update.json"), nil
}

func readCheckState() (checkState, error) {
	var s checkState
	path, err := checkStatePath()
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return checkState{}, err
	}
	return s, nil
}

// writeCheckState remembers when the last check ran, so the app does not ask GitHub again on the
// next launch. Every way it can fail leaves a check that will simply run again sooner: none of them
// is worth stopping a launch over, and a silent one would be a state file nobody can explain.
func writeCheckState(s checkState) {
	path, err := checkStatePath()
	if err != nil {
		log.Printf("[update] remembering the check: %v", err)
		return
	}
	data, err := json.Marshal(s)
	if err != nil {
		log.Printf("[update] remembering the check: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		log.Printf("[update] remembering the check: %v", err)
	}
}
