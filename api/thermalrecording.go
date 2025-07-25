/*
management-interface - Web based management of Raspberry Pis over WiFi
Copyright (C) 2025, The Cacophony Project

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
)

func (api *ManagementAPI) TestThermalRecordingStatus(w http.ResponseWriter, r *http.Request) {
	tc2AgentDbus, err := getTC2AgentDbus()
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to connect to DBus", http.StatusInternalServerError)
		return
	}

	var status int
	var mode int
	err = tc2AgentDbus.Call("org.cacophony.TC2Agent.testthermalstatus", 0).Store(&mode, &status)
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to get test thermal recording status", http.StatusInternalServerError)
		return
	}
	rp2040status := map[string]int{"mode": mode, "status": status}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rp2040status)

}

func (api *ManagementAPI) TakeLongTestThermalRecording(w http.ResponseWriter, r *http.Request) {
	tc2AgentDbus, err := getTC2AgentDbus()
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to connect to DBus", http.StatusInternalServerError)
		return
	}
	seconds, err := strconv.ParseUint(r.URL.Query().Get("seconds"), 10, 32)
	if err != nil {
		badRequest(&w, err)
		return
	}
	var result string

	err = tc2AgentDbus.Call("org.cacophony.TC2Agent.longtestthermalrecording", 0, seconds).Store(&result)
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to request 5 minute test thermal recording", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(result)
}

func (api *ManagementAPI) TakeShortTestThermalRecording(w http.ResponseWriter, r *http.Request) {
	tc2AgentDbus, err := getTC2AgentDbus()
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to connect to DBus", http.StatusInternalServerError)
		return
	}

	var result string

	err = tc2AgentDbus.Call("org.cacophony.TC2Agent.shorttestthermalrecording", 0).Store(&result)
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to request short test thermal recording", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(result)
}
