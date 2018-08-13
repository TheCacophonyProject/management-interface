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

package main

import (
	"log"
	"net/http"

	"github.com/gobuffalo/packr"
	"github.com/gorilla/mux"

	managementinterface "github.com/TheCacophonyProject/management-interface"
	"github.com/TheCacophonyProject/management-interface/api"
)

const (
	cptvDir = "/var/spool/cptv/"
)

// Set up and handle page requests.
func main() {
	router := mux.NewRouter()
	router.HandleFunc("/3G-connectivity", managementinterface.ThreeGConnectivityHandler).Methods("GET")
	router.HandleFunc("/API-server", managementinterface.APIServerHandler).Methods("GET")
	router.HandleFunc("/camera-positioning", managementinterface.CameraPositioningHandler).Methods("GET")
	router.HandleFunc("/", managementinterface.IndexHandler).Methods("GET")
	router.HandleFunc("/network-interfaces", managementinterface.NetworkInterfacesHandler).Methods("GET")
	router.HandleFunc("/disk-memory", managementinterface.DiskMemoryHandler).Methods("GET")

	// Serve up static content.
	static := packr.NewBox("../../static")
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(static)))

	// API
	apiObj := api.NewAPI(cptvDir)
	router.HandleFunc("/api/recordings", apiObj.GetRecordings).Methods("GET")
	router.HandleFunc("/api/recording/{id}", apiObj.GetRecording).Methods("GET")
	router.HandleFunc("/api/recording/{id}", apiObj.DeleteRecording).Methods("DELETE")

	log.Fatal(http.ListenAndServe(":8080", router))
}
