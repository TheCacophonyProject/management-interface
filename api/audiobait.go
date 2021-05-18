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
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/TheCacophonyProject/audiobait/v3/audiobaitclient"
	"github.com/TheCacophonyProject/audiobait/v3/audiofilelibrary"
	"github.com/TheCacophonyProject/audiobait/v3/playlist"
	"github.com/TheCacophonyProject/go-config"
)

func (api *ManagementAPI) PlayAudiobaitSound(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		parseFormErrorResponse(&w, err)
		return
	}
	fileId, err := parseIntFromForm("fileId", r.Form)
	if err != nil {
		parseFormErrorResponse(&w, err)
		return
	}
	volume, err := parseIntFromForm("volume", r.Form)
	if err != nil {
		parseFormErrorResponse(&w, err)
		return
	}
	_, err = audiobaitclient.PlayFromId(fileId, volume, 99, nil)
	if err != nil {
		serverError(&w, err)
		return
	}
}

func (api *ManagementAPI) GetAudiobait(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	schedule, err := playlist.LoadScheduleFromDisk(config.DefaultAudio().Dir) //TODO properly get audiofile config
	if err != nil {
		serverError(&w, err)
		return
	}
	library, err := audiofilelibrary.OpenLibrary(config.DefaultAudio().Dir)
	if err != nil {
		serverError(&w, err)
		return
	}
	data := map[string]interface{}{
		"schedule": schedule,
		"library":  library,
	}
	json.NewEncoder(w).Encode(data)
}

func (api *ManagementAPI) PlayTestSound(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		parseFormErrorResponse(&w, err)
		return
	}
	volumeString := r.Form.Get("volume")
	volume, err := strconv.Atoi(volumeString)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, fmt.Sprintf("failed to parse '%s' to an int", volumeString))
		return
	}
	if err := audiobaitclient.PlayTestSound(volume); err != nil {
		serverError(&w, err)
	}
}
