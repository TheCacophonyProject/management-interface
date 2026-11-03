package api

import (
	"os"
	"path/filepath"
	"testing"

	goconfig "github.com/TheCacophonyProject/go-config"
)

func TestCheckTrapProgram(t *testing.T) {
	for _, lowPower := range []bool{false, true} {
		dir := t.TempDir()
		toml := "[thermal-recorder]\nuse-low-power-mode = false\n"
		if lowPower {
			toml = "[thermal-recorder]\nuse-low-power-mode = true\n"
		}
		if err := os.WriteFile(filepath.Join(dir, goconfig.ConfigFileName), []byte(toml), 0644); err != nil {
			t.Fatal(err)
		}
		config, err := goconfig.New(dir)
		if err != nil {
			t.Fatal(err)
		}
		api := &ManagementAPI{config: config}

		trapProgram := func(program any) map[string]any {
			return map[string]any{"trap": map[string]any{"config": map[string]any{"program": program}}}
		}
		// Programs arrive as float64 from the JSON request.
		if err := api.checkTrapProgram(trapProgram(float64(1))); err != nil {
			t.Errorf("low power %v: program 1 refused: %v", lowPower, err)
		}
		if err := api.checkTrapProgram(map[string]any{"comms": map[string]any{"enable": true}}); err != nil {
			t.Errorf("low power %v: other section refused: %v", lowPower, err)
		}
		if err := api.checkTrapProgram(map[string]any{"trap": map[string]any{"config": map[string]any{}}}); err != nil {
			t.Errorf("low power %v: trap config without a program refused: %v", lowPower, err)
		}
		err = api.checkTrapProgram(trapProgram(float64(3)))
		if lowPower && err == nil {
			t.Error("AI Mode allowed in low power mode")
		}
		if !lowPower && err != nil {
			t.Errorf("AI Mode refused in high power mode: %v", err)
		}
	}
}
