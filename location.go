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
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	yaml "gopkg.in/yaml.v2"
)

const deviceLocationFile = "/etc/cacophony/location.yaml"

// LocationHandler shows and updates the location of the device.
func LocationHandler(w http.ResponseWriter, r *http.Request) {
	type locationResponse struct {
		Location     *rawLocationData
		Message      string
		ErrorMessage string
	}

	switch r.Method {
	case "GET", "":
		location, err := readLocationFile(deviceLocationFile)
		resp := &locationResponse{
			Location:     location.rawLocationData(),
			ErrorMessage: errorMessage(err),
		}
		tmpl.ExecuteTemplate(w, "location.html", resp)

	case "POST":
		rawLocation, err := handleLocationPostRequest(w, r)
		resp := &locationResponse{
			Location:     rawLocation,
			Message:      successMessage(err, "Location updated"),
			ErrorMessage: errorMessage(err),
		}
		tmpl.ExecuteTemplate(w, "location.html", resp)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// APILocationHandler writes the location of the device to the deviceLocationFile
func APILocationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	_, err := handleLocationPostRequest(w, r)
	if isClientError(err) {
		w.WriteHeader(http.StatusBadRequest)
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.WriteHeader(http.StatusOK)
}

func handleLocationPostRequest(w http.ResponseWriter, r *http.Request) (*rawLocationData, error) {
	var rawLocation *rawLocationData
	var location *locationData

	if r.FormValue("action") == "clear" {
		location = new(locationData)
		rawLocation = new(rawLocationData)
	} else {
		var err error
		rawLocation = newRawLocationData(r)
		location, err = rawLocation.locationData()
		if err != nil {
			return rawLocation, err
		}
	}
	if err := writeLocationFile(deviceLocationFile, location); err != nil {
		log.Printf("Could not write to location file: %v", err)
		return nil, errors.New("could not write location file")
	}
	return rawLocation, nil
}

// locationData holds a (latitude, longitude) pair
type locationData struct {
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`
}

func (l *locationData) rawLocationData() *rawLocationData {
	return &rawLocationData{
		Latitude:  floatToString(l.Latitude),
		Longitude: floatToString(l.Longitude),
	}
}

// rawLocationData holds unconverted location form values
type rawLocationData struct {
	Latitude  string
	Longitude string
}

func newRawLocationData(r *http.Request) *rawLocationData {
	return &rawLocationData{
		Latitude:  trimmedFormValue(r, "latitude"),
		Longitude: trimmedFormValue(r, "longitude"),
	}
}

func (fl *rawLocationData) locationData() (*locationData, error) {
	lat, ok := parseFloat(fl.Latitude)
	if !ok {
		return nil, newClientError("Invalid latitude")
	}
	lon, ok := parseFloat(fl.Longitude)
	if !ok {
		return nil, newClientError("Invalid longitude")
	}
	return &locationData{
		Latitude:  lat,
		Longitude: lon,
	}, nil
}

// writeLocationFile writes the location values to the location data file.
// If it doesn't exist, it is created.
func writeLocationFile(filepath string, location *locationData) error {
	outBuf, err := yaml.Marshal(location)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath, outBuf, 0644)
}

// readLocationFile retrieves values from the location data file.
func readLocationFile(filepath string) (*locationData, error) {
	location := new(locationData)

	inBuf, err := ioutil.ReadFile(filepath)
	if os.IsNotExist(err) {
		return location, nil
	} else if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(inBuf, location); err != nil {
		return nil, err
	}
	return location, nil
}
