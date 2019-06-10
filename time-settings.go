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
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// timeSettings is a struct which hold the time settings.
type timeSettings struct {
	RTCTime    time.Time
	SystemTime time.Time
}

func getTimes() (*timeSettings, error) {
	if runtime.GOOS != "windows" {
		// 'Nix.  Run hwclock command to get the times we want.
		ts := &timeSettings{SystemTime: time.Now()}
		out, err := exec.Command("sh", "-c", "hwclock -r").Output()
		if err != nil {
			log.Printf(err.Error())
			return ts, err
		}
		// Convert to time.Time
		// log.Println(strings.Trim(string(out), " /t/n"))
		ts.RTCTime, err = parseTimeString(strings.TrimSpace(string(out)))
		if err != nil {
			log.Printf(err.Error())
			return ts, err
		}
		return ts, nil
	}
	return &timeSettings{}, nil

}

func setRTCTime(dateStr string, timeStr string) error {
	if runtime.GOOS != "windows" {
		// 'Nix.  Run hwclock command to set the RTC time
		// If dateStr is blank, todays date is used by hwclock.  And if timeStr is blank, then 00:00 is used.
		out, err := exec.Command("sh", "-c", "hwclock --set --date '"+dateStr+" "+timeStr+"'").Output()
		if err != nil {
			log.Printf(string(out) + err.Error())
			return err
		}
		return nil
	}
	return nil
}

func syncRTCTimeToSystemTime() error {
	if runtime.GOOS != "windows" {
		// 'Nix.  Run hwclock command to set the RTC to the system time..
		out, err := exec.Command("sh", "-c", "hwclock --systohc").Output()
		if err != nil {
			log.Printf(string(out) + err.Error())
			return err
		}
	}
	return nil
}

// This struct is used to send data to the time settings html form.
type timeSettingsResponse struct {
	SystemTime   string
	SystemDate   string
	RTCTime      string
	RTCDate      string
	Message      string
	ErrorMessage string
}

func newTimeSettingsResponse(ts *timeSettings, errStr string) *timeSettingsResponse {
	return &timeSettingsResponse{
		SystemTime:   extractTimeAsString(ts.SystemTime),
		SystemDate:   extractDateAsString(ts.SystemTime),
		RTCTime:      extractTimeAsString(ts.RTCTime),
		RTCDate:      extractDateAsString(ts.RTCTime),
		ErrorMessage: errStr,
	}
}

// TimeHandler shows and updates the time settings for the device
func TimeHandler(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET", "":
		var resp *timeSettingsResponse
		ts, err := getTimes()
		resp = newTimeSettingsResponse(ts, errorMessage(err))
		tmpl.ExecuteTemplate(w, "time-settings.html", resp)

	case "POST":
		var resp *timeSettingsResponse

		if r.FormValue("action") == "setrtctimetouservalue" {
			// The user wants to set the RTC time from the date/time they have set in the html form.
			err := setRTCTime(trimmedFormValue(r, "rtcdate"), trimmedFormValue(r, "rtctime"))
			if err != nil {
				resp.ErrorMessage = errorMessage(err)
			} else {
				ts, err := getTimes()
				if err != nil {
					resp.ErrorMessage = errorMessage(err)
				} else {
					resp = newTimeSettingsResponse(ts, "")
					resp.Message = "Run time clock successfully updated."
				}
			}
			tmpl.ExecuteTemplate(w, "time-settings.html", resp)

		} else {
			// Sync RTC time to system time.
			err := syncRTCTimeToSystemTime()
			if err != nil {
				resp.ErrorMessage = errorMessage(err)
			} else {
				ts, err := getTimes()
				if err != nil {
					resp.ErrorMessage = errorMessage(err)
				} else {
					resp = newTimeSettingsResponse(ts, "")
					resp.Message = "Run time clock successfully synced to system time."
				}
			}
			tmpl.ExecuteTemplate(w, "time-settings.html", resp)
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}
