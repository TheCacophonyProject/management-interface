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
)

// Set up and handle page requests.
func main() {
	router := mux.NewRouter()
	router.HandleFunc("/3G-connectivity.html", managementinterface.ThreeGConnectivityHandler).Methods("GET")
	router.HandleFunc("/API-server.html", managementinterface.APIServerHandler).Methods("GET")
	router.HandleFunc("/camera-positioning.html", managementinterface.CameraPositioningHandler).Methods("GET")
	router.HandleFunc("/", managementinterface.IndexHandler).Methods("GET")
	router.HandleFunc("/network-interfaces.html", managementinterface.NetworkInterfacesHandler).Methods("GET")
	router.HandleFunc("/disk-memory.html", managementinterface.DiskMemoryHandler).Methods("GET")

	// Serve up static content.
	static := packr.NewBox("../../static")
	router.Handle("/static/", http.StripPrefix("/static/", http.FileServer(static))).Methods("GET")

	log.Fatal(http.ListenAndServe(":8080", router))
}
