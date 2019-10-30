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
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func trimmedFormValue(r *http.Request, name string) string {
	return strings.TrimSpace(r.FormValue(name))
}

func parseFloat(val string) (float64, bool) {
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func parseOptionalFloat(val string) (float64, bool) {
	if val == "" {
		return 0, true
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func floatToString(val float64) string {
	if val == 0 {
		return ""
	}
	return fmt.Sprint(val)
}

// parseTimestamp returns the field at 'val' as a time value
func parseTimestamp(val string) (time.Time, bool) {
	if val == "" {
		return time.Now(), true
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// parseTimeString parses a string containing a time in the format returned by hwclock
// and returns a time.Time value.
func parseTimeString(timeStr string) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, newClientError("Could not parse time string.")
	}
	t, err := time.Parse("2006-01-02 15:04:05.999999-0700", timeStr)
	if err != nil {
		return time.Time{}, newClientError("Could not parse time string." + err.Error())
	}
	return t, nil
}

// parseISOTimeString parses a string containing a time in ISO format and returns a time.Time value.
func parseISOTimeString(timeStr string) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, newClientError("Could not parse time string.")
	}
	t, err := time.Parse("2006-01-02T15:04:05.999Z", timeStr)
	if err != nil {
		return time.Time{}, newClientError("Could not parse ISO time string." + err.Error())
	}
	return t, nil
}

// Return a time string in RFC3339 format
func timestampToString(t time.Time) string {
	return t.Format(time.RFC3339)
}

// Return a time string in ANSIC format
func timeToANSICString(t time.Time) string {
	// return t.Format("Mon Jan _2 15:04:05 2006")
	return t.Format(time.ANSIC)
}

// Return the time part of the time.Time struct as a string.
func extractTimeAsString(t time.Time) string {
	return t.Format("15:04:05")
}

// Return the date part fo the time.Time struct as a string.
func extractDateAsString(t time.Time) string {
	return t.Format("2006-01-02")
}

func successMessage(err error, msg string) string {
	if err == nil {
		return msg
	}
	return ""
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Reverse a slice of strings in place.
func reverse(ss []string) {
	last := len(ss) - 1
	for i := 0; i < len(ss)/2; i++ {
		ss[i], ss[last-i] = ss[last-i], ss[i]
	}
}
