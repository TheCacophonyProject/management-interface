/*
management-interface - Web based management of Raspberry Pis over WiFi
Copyright (C) 2018, The Cacophony Project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package managementinterface

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"

	"github.com/gobuffalo/packr"
)

// SpeakerTestHandler will show a frame from the camera to help with positioning
func SpeakerTestHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "speaker-test.html", nil)
}

var audioBox = packr.NewBox("./audio")

// SpeakerStatusHandler attempts to play a sound on connected speaker(s).
func SpeakerStatusHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	response := make(map[string]string)

	if output, err := playTestAudio(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("audio output failed: %v", err)
		response["result"] = fmt.Sprintf("Error: %v. Output:\n%s", err.Error(), string(output))
	} else {
		w.WriteHeader(http.StatusOK)
		response["result"] = string(output)
	}

	// Encode data to be sent back to html.
	json.NewEncoder(w).Encode(response)
}

func playTestAudio() ([]byte, error) {
	wav := audioBox.Bytes("test.wav")
	if wav == nil {
		return nil, errors.New("unable to load test audio")
	}
	cmd := exec.Command("play", "-t", "wav", "--norm=-3", "-q", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("unable to play audio: %v", err)
	}

	go func() {
		defer stdin.Close()
		w := bufio.NewWriter(stdin)
		if _, err := w.Write(wav); err != nil {
			log.Printf("unable to pass audio: %v", err)
		}
	}()

	return cmd.CombinedOutput()
}
