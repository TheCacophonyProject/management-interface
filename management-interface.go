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
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"

	"github.com/gobuffalo/packr"
)

var templateSrc = packr.NewBox("./html")

func parseTemplate(name string) *template.Template {
	t := template.New(name)
	return template.Must(t.Parse(templateSrc.String(name)))
}

// A struct used to wrap data being sent to the HTML templates.
type dataToBeDisplayed struct {
	Head  string
	Body  string
	Other string
}

func getDiskSpace() (string, error) {
	var out []byte
	err := error(nil)
	if runtime.GOOS == "windows" {
		// On Windows, commands need to be handled like this:
		out, err = exec.Command("cmd", "/C", "dir").Output()
	} else {
		// 'Nix.  Run df command to show disk space available on SD card.
		out, err = exec.Command("sh", "-c", "df -h").Output()
	}

	if err != nil {
		log.Printf(err.Error())
		return err.Error(), err
	}
	return string(out), nil

}

// Return info on memory e.g. memory used, memory available etc.
func getMemoryStats() (string, error) {

	return "Not implemented yet.", nil
}

// DiskMemoryHandler shows disk space usage and memory usage
func DiskMemoryHandler(w http.ResponseWriter, r *http.Request) {

	diskData, err := getDiskSpace()
	if err != nil {
		log.Fatal(err)
	}
	// Want to separate this into multiple lines so that can display each line on a separate line in HTML
	temp := strings.Split(diskData, "\n")
	var outputStrings []StringToBeDisplayed
	for _, str := range temp {
		outputStrings = append(outputStrings, StringToBeDisplayed{Text: str})
	}

	// memoryData, err := getMemoryStats()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// for _, str := range strings.Split(memoryData, "\n") {
	// 	outputStrings = append(outputStrings, StringToBeDisplayed{Text: str})
	// }

	// Need to put our output string in a struct so we can access it from html
	outputStruct := MultiLineStringToBeDisplayed{Strings: outputStrings}

	t := parseTemplate("disk-memory.html")
	t.Execute(w, outputStruct)

}

// IndexHandler is the root handler.
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	t := parseTemplate("index.html")
	t.Execute(w, "")

}

// NetworkInterfacesHandler - Show the status of each newtwork interface
func NetworkInterfacesHandler(w http.ResponseWriter, r *http.Request) {
	data, err := AvailableInterfaces()
	if err != nil {
		log.Fatal(err)
	}
	// Need to put our output string in a struct so we can access it from html
	outputStruct := MultiLineStringToBeDisplayed{Strings: data}

	t := parseTemplate("network-interfaces.html")
	t.Execute(w, outputStruct)
}

// CameraPositioningHandler will show a frame from the camera to help with positioning
func CameraPositioningHandler(w http.ResponseWriter, r *http.Request) {
	t := parseTemplate("camera-positioning.html")
	t.Execute(w, "Some data")

}

// ThreeGConnectivityHandler - Do we have 3G Connectivity?
func ThreeGConnectivityHandler(w http.ResponseWriter, r *http.Request) {
	t := parseTemplate("3G-connectivity.html")
	t.Execute(w, "Some data")

}

// APIServerHandler - API Server stuff
func APIServerHandler(w http.ResponseWriter, r *http.Request) {
	t := parseTemplate("API-server.html")
	t.Execute(w, "Some data")

}
