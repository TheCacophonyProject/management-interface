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

func GenLocationHandler(config *goconfig.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		LocationHandler(config, w, r)
	}
}

// LocationHandler shows and updates the location of the device.
func LocationHandler(config *goconfig.Config, w http.ResponseWriter, r *http.Request) {
	type locationResponse struct {
		Location     *goconfig.Location
		Message      string
		ErrorMessage string
	}
	var location goconfig.Location
	err := config.Update()
	if err2 := config.Unmarshal(goconfig.LocationKey, &location); err2 != nil {
		err = err2
	}
	resp := &locationResponse{
		Location:     &location,
		ErrorMessage: errorMessage(err),
	}
	tmpl.ExecuteTemplate(w, "location.html", resp)
}
