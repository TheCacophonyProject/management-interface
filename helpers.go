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

func floatToString(val float64) string {
	if val == 0 {
		return ""
	}
	return fmt.Sprint(val)
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
