/*
management-interface - Web based management of Raspberry Pis over WiFi
Copyright (C) 2019, The Cacophony Project

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

package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	goconfig "github.com/TheCacophonyProject/go-config"
	"github.com/godbus/dbus"
)

func (api *ManagementAPI) GetAudioRecording(w http.ResponseWriter, r *http.Request) {
	audioRecording := goconfig.DefaultAudioRecording()
	if err := api.config.Unmarshal(goconfig.AudioRecordingKey, &audioRecording); err != nil {
		serverError(&w, err)
		return
	}
	type AudioRecording struct {
		AudioMode string `json:"audio-mode"`
		AudioSeed string `json:"audio-seed"`
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(AudioRecording{
		AudioMode: audioRecording.AudioMode,
	})
}

// SetAudioRecording is for specifically writing to audio recording setting.
func (api *ManagementAPI) SetAudioRecording(w http.ResponseWriter, r *http.Request) {
	log.Println("update audio recording")
	audioMode := r.FormValue("audio-mode")
	stringSeed := r.FormValue("audio-seed")
	var audioSeed uint32
	log.Printf("Audio seeds is %v", stringSeed)
	if stringSeed == "" {
		audioSeed = 0
		log.Printf("Set empty")

	} else {
		log.Printf("try parse")

		seed, err := strconv.ParseUint(r.FormValue("audio-seed"), 10, 32)
		if err != nil {
			badRequest(&w, err)
			return
		}
		audioSeed = uint32(seed)

	}

	audioRecording := goconfig.AudioRecording{
		AudioMode: audioMode,
		AudioSeed: audioSeed,
	}

	if err := api.config.Set(goconfig.AudioRecordingKey, &audioRecording); err != nil {
		serverError(&w, err)
	}
}

func (api *ManagementAPI) AudioRecordingStatus(w http.ResponseWriter, r *http.Request) {
	tc2AgentDbus, err := getTC2AgentDbus()
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to connect to DBus", http.StatusInternalServerError)
		return
	}

	var status int
	var mode int
	err = tc2AgentDbus.Call("org.cacophony.TC2Agent.audiostatus", 0).Store(&mode, &status)
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to request test audio recoding", http.StatusInternalServerError)
		return
	}
	rp2040status := map[string]int{"mode": mode, "status": status}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rp2040status)

}

func (api *ManagementAPI) TakeTestAudioRecording(w http.ResponseWriter, r *http.Request) {
	tc2AgentDbus, err := getTC2AgentDbus()
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to connect to DBus", http.StatusInternalServerError)
		return
	}

	var result string

	err = tc2AgentDbus.Call("org.cacophony.TC2Agent.testaudio", 0).Store(&result)
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to request test audio recoding", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(result)
}

func getTC2AgentDbus() (dbus.BusObject, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	return conn.Object("org.cacophony.TC2Agent", "/org/cacophony/TC2Agent"), nil
}
