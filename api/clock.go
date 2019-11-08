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
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/TheCacophonyProject/rtc-utils/rtc"
)

const (
	timeFormat    = "2006-01-02T15:04:05Z07:00"
	dateCmdFormat = "+%Y-%m-%dT%H:%M:%S%:z"
)

type clockInfo struct {
	RTCTimeUTC    string
	RTCTimeLocal  string
	SystemTime    string
	LowRTCBattery bool
	RTCIntegrity  bool
	NTPSynced     bool
}

func (api *ManagementAPI) GetClock(w http.ResponseWriter, r *http.Request) {
	out, err := exec.Command("date", dateCmdFormat).CombinedOutput()
	if err != nil {
		serverError(&w, err)
		return
	}
	systemTime, err := time.Parse(timeFormat, strings.TrimSpace(string(out)))
	if err != nil {
		serverError(&w, err)
		return
	}
	ntpSynced, err := rtc.IsNTPSynced()
	if err != nil {
		serverError(&w, err)
		return
	}
	rtcState, err := rtc.State(1)
	if err != nil {
		serverError(&w, err)
		return
	}

	b, err := json.Marshal(&clockInfo{
		RTCTimeUTC:    rtcState.Time.UTC().Format(timeFormat),
		RTCTimeLocal:  rtcState.Time.Local().Format(timeFormat),
		SystemTime:    systemTime.Format(timeFormat),
		LowRTCBattery: rtcState.LowBattery,
		RTCIntegrity:  rtcState.ClockIntegrity,
		NTPSynced:     ntpSynced,
	})
	if err != nil {
		serverError(&w, err)
		return
	}
	w.Write(b)
}

func (api *ManagementAPI) PostClock(w http.ResponseWriter, r *http.Request) {
	date, err := time.Parse(timeFormat, r.FormValue("date"))
	if err != nil {
		badRequest(&w, err)
		return
	}
	cmd := exec.Command("date", dateCmdFormat, "--utc", fmt.Sprintf("--set=%s", date.Format(timeFormat)))
	_, err = cmd.CombinedOutput()
	if err != nil {
		serverError(&w, err)
		return
	}
	if err := rtc.Write(1); err != nil {
		serverError(&w, err)
	}
}
