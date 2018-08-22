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
)

// tmpl is our pointer to our parsed templates.
var tmpl *template.Template

// ParseTemplates parses our html templates upfront.
func ParseTemplates() {
	tmpl = template.Must(template.ParseGlob("../../html/*"))
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

	var out []byte
	err := error(nil)
	if runtime.GOOS == "windows" {
		// Will show more than just memory stuff.
		out, err = exec.Command("cmd", "/C", "systeminfo").Output()
	} else {
		// 'Nix.  Run vmstat command to show memory stats.
		out, err = exec.Command("sh", "-c", "vmstat -s").Output()
	}

	if err != nil {
		log.Printf(err.Error())
		return err.Error(), err
	}
	return string(out), nil
}

// DiskMemoryHandler shows disk space usage and memory usage
func DiskMemoryHandler(w http.ResponseWriter, r *http.Request) {

	diskData, err := getDiskSpace()
	if err != nil {
		log.Fatal(err)
	}
	// Want to separate this into multiple lines so that can display each line on a separate line in HTML
	var outputStrings []StringToBeDisplayed
	for _, str := range strings.Split(diskData, "\n") {
		outputStrings = append(outputStrings, StringToBeDisplayed{Text: str})
	}

	memoryData, err := getMemoryStats()
	if err != nil {
		log.Fatal(err)
	}
	for _, str := range strings.Split(memoryData, "\n") {
		outputStrings = append(outputStrings, StringToBeDisplayed{Text: str})
	}

	// Need to put our output string in a struct so we can access it from html
	outputStruct := MultiLineStringToBeDisplayed{Strings: outputStrings}

	tmpl.ExecuteTemplate(w, "disk-memory.html", outputStruct)
}

// IndexHandler is the root handler.
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "index.html", nil)
}

// NetworkInterfacesHandler - Show the status of each newtwork interface
func NetworkInterfacesHandler(w http.ResponseWriter, r *http.Request) {
	data, err := AvailableInterfaces()
	if err != nil {
		log.Fatal(err)
	}
	// Need to put our output string in a struct so we can access it from html
	outputStruct := MultiLineStringToBeDisplayed{Strings: data}

	tmpl.ExecuteTemplate(w, "network-interfaces.html", outputStruct)
}

// CameraPositioningHandler will show a frame from the camera to help with positioning
func CameraPositioningHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "camera-positioning.html", nil)
}

// ThreeGConnectivityHandler - Do we have 3G Connectivity?
func ThreeGConnectivityHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "3G-connectivity.html", nil)
}

// APIServerHandler - API Server stuff
func APIServerHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "API-server.html", nil)
}
