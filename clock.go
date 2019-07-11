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

package managementinterface

import (
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
		out, err := exec.Command("/sbin/hwclock", "-r").Output()
		if err != nil {
			return ts, err
		}
		// Convert to time.Time
		ts.RTCTime, err = parseTimeString(strings.TrimSpace(string(out)))
		if err != nil {
			return ts, err
		}
		return ts, nil
	}
	return &timeSettings{}, nil

}

// Set both the hardware and system times to the date/time info passed in.
func setTimes(ISOdateTimeStr string, timeZone string) error {
	if runtime.GOOS != "windows" {

		// Convert ISOdateTimeStr to a time.Time struct.
		UTCTime, err := parseISOTimeString(ISOdateTimeStr)
		if err != nil {
			return err
		}

		// Convert UTC time to local time
		loc, err := time.LoadLocation(timeZone)
		if err != nil {
			return err
		}
		localTime := UTCTime.In(loc)

		// Now convert this back into a string suitable for the hardware call
		dateStr := timeToANSICString(localTime)

		//Run hwclock command to set the hardware clock to the given time.
		_, err = exec.Command("/sbin/hwclock", "--set", "--localtime", "--date", dateStr).Output()
		if err != nil {
			return err
		}

		// The camera needs a small delay before the next hwclock command is issued.
		time.Sleep(100 * time.Millisecond)

		// Now set the system time to that same time.
		_, err = exec.Command("/sbin/hwclock", "--hctosys").Output()
		if err != nil {
			return err
		}

		// And I put another delay here to make sure the above command has time to complete before
		// another command is issued.
		time.Sleep(100 * time.Millisecond)

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
		if err != nil {
			resp.ErrorMessage += " Could not retrieve times."
		}
		tmpl.ExecuteTemplate(w, "clock.html", resp)

	case "POST":
		var resp *timeSettingsResponse

		err := handleTimePostRequest(w, r)
		if err != nil {
			resp.ErrorMessage = errorMessage(err) + " Could not set times."
			tmpl.ExecuteTemplate(w, "clock.html", resp)
		}

		ts, err := getTimes()
		resp = newTimeSettingsResponse(ts, errorMessage(err))
		if err != nil {
			resp.ErrorMessage += " Could not retrieve times."
		} else {
			resp.Message = "Times successfully updated."
		}
		tmpl.ExecuteTemplate(w, "clock.html", resp)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}

// Set both the system time and the hardware clock time to the time passed in.
func handleTimePostRequest(w http.ResponseWriter, r *http.Request) error {

	return setTimes(trimmedFormValue(r, "currenttime"), trimmedFormValue(r, "timezone"))

}

// APITimeHandler sets the hardware clock and system time via a post request.
func APITimeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	err := handleTimePostRequest(w, r)
	if isClientError(err) {
		w.WriteHeader(http.StatusBadRequest)
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.WriteHeader(http.StatusOK)
}
