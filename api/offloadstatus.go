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
)

func (api *ManagementAPI) RecordingOffloadStatus(w http.ResponseWriter, r *http.Request) {
	tc2AgentDbus, err := getTC2AgentDbus()
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to connect to DBus", http.StatusInternalServerError)
		return
	}
	type OffloadNotInProgress struct {
		InProgress bool `json:"offload-in-progress"`
	}
	type OffloadInProgress struct {
		InProgress       bool `json:"offload-in-progress"`
		SecondsRemaining int  `json:"seconds-remaining"`
		PercentComplete  int  `json:"percent-complete"`
	}

	var isOffloading int
	var percentComplete int
	var secondsRemaining int
	err = tc2AgentDbus.Call("org.cacophony.TC2Agent.offloadstatus", 0).Store(&isOffloading, &percentComplete, &secondsRemaining)
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to request recording offload status", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	if isOffloading == 1 {
		json.NewEncoder(w).Encode(OffloadInProgress{
			InProgress:       true,
			PercentComplete:  percentComplete,
			SecondsRemaining: secondsRemaining,
		})
	} else {
		json.NewEncoder(w).Encode(OffloadNotInProgress{
			InProgress: false,
		})
	}

}
