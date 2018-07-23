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

// A struct used to wrap data being sent to the HTML templates.
type dataToBeDisplayed struct {
	Head  string
	Body  string
	Other string
}

func getDiskSpace() (string, error) {

	out := make([]byte, 0)
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
		//fmt.Fprintf(w, err.Error()+"\n")
		//fmt.Fprintf(w, "Cannot show disk space at this time.\n")
		return err.Error(), err
	}

	//fmt.Fprintf(w, "Disk space usage is: \n\n%s\n", out)
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
	outputStrings := make([]StringToBeDisplayed, 0)
	for _, str := range temp {
		outputStrings = append(outputStrings, StringToBeDisplayed{Text: str})
	}
	// Need to put our output string in a struct so we can access it from html
	outputStruct := MultiLineStringToBeDisplayed{Strings: outputStrings}

	// memoryData, err := getMemoryStats()
	// if err != nil {
	// 	log.Fatal(err)
	// }

	t, _ := template.ParseFiles("../html/disk-memory.html")
	t.Execute(w, outputStruct)

}

// IndexHandler is the root handler.
func IndexHandler(w http.ResponseWriter, r *http.Request) {

	t, _ := template.ParseFiles("../html/index.html")
	t.Execute(w, "")

}

// NetworkInterfacesHandler - Show the status of each newtwork interface
func NetworkInterfacesHandler(w http.ResponseWriter, r *http.Request) {
	data, err := AvailableInterfaces()
	//fmt.Println(data) // remove later
	if err != nil {
		log.Fatal(err)
	}
	// Need to put our output string in a struct so we can access it from html
	outputStruct := MultiLineStringToBeDisplayed{Strings: data}

	t, _ := template.ParseFiles("../html/network-interfaces.html")
	t.Execute(w, outputStruct)
}

// CameraPositioningHandler will show a frame from the camera to help with positioning
func CameraPositioningHandler(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("../html/camera-positioning.html")
	t.Execute(w, "Some data")

}

// ThreeGConnectivityHandler - Do we have 3G Connectivity?
func ThreeGConnectivityHandler(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("../html/3G-connectivity.html")
	t.Execute(w, "Some data")

}

// APIServerHandler - API Server stuff
func APIServerHandler(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("../html/API-server.html")
	t.Execute(w, "Some data")

}
