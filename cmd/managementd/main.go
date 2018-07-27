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

	managementinterface "github.com/TheCacophonyProject/management-interface"
)

// Set up and handle page requests.
func main() {
	http.HandleFunc("/3G-connectivity.html/", managementinterface.ThreeGConnectivityHandler)
	http.HandleFunc("/API-server.html/", managementinterface.APIServerHandler)
	http.HandleFunc("/camera-positioning.html/", managementinterface.CameraPositioningHandler)
	http.HandleFunc("/", managementinterface.IndexHandler)
	http.HandleFunc("/network-interfaces.html/", managementinterface.NetworkInterfacesHandler)
	http.HandleFunc("/disk-memory.html/", managementinterface.DiskMemoryHandler)

	// Serve up static content.
	static := packr.NewBox("../../static")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(static)))

	log.Fatal(http.ListenAndServe(":8080", nil))

}
