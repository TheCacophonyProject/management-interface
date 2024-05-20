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
	"net/http"

	goconfig "github.com/TheCacophonyProject/go-config"
)

func GenAudioRecordingHandler(config *goconfig.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		AudioRecordingHandler(config, w, r)
	}
}

// LocationHandler shows and updates the location of the device.
func AudioRecordingHandler(config *goconfig.Config, w http.ResponseWriter, r *http.Request) {
	type audioRecordingResponse struct {
		AudioRecording *goconfig.AudioRecording
		Message        string
		ErrorMessage   string
	}
	var audioRecording goconfig.AudioRecording
	err := config.Update()
	if err2 := config.Unmarshal(goconfig.AudioRecordingKey, &audioRecording); err2 != nil {
		err = err2
	}
	resp := &audioRecordingResponse{
		AudioRecording: &audioRecording,
		ErrorMessage:   errorMessage(err),
	}
	tmpl.ExecuteTemplate(w, "audiorecording.html", resp)
}
