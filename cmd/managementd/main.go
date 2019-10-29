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
	"fmt"
	"log"
	"net/http"

	"github.com/gobuffalo/packr"
	"github.com/gorilla/mux"

	goconfig "github.com/TheCacophonyProject/go-config"
	managementinterface "github.com/TheCacophonyProject/management-interface"
	"github.com/TheCacophonyProject/management-interface/api"
)

const (
	configDir = goconfig.DefaultConfigDir
)

var version = "<not set>"

// Set up and handle page requests.
func main() {
	log.SetFlags(0) // Removes timestamp output
	log.Printf("running version: %s", version)

	config, err := ParseConfig(configDir)
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Printf("config: %v", config)
	if config.Port != 80 {
		log.Printf("warning: avahi service is advertised on port 80 but port %v is being used", config.Port)
	}

	router := mux.NewRouter()

	// Serve up static content.
	static := packr.NewBox("../../static")
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(static)))

	// UI handlers.
	router.HandleFunc("/", managementinterface.IndexHandler).Methods("GET")
	router.HandleFunc("/wifi-networks", managementinterface.WifiNetworkHandler).Methods("GET", "POST")
	router.HandleFunc("/network", managementinterface.NetworkHandler).Methods("GET")
	router.HandleFunc("/interface-status/{name:[a-zA-Z0-9-* ]+}", managementinterface.CheckInterfaceHandler).Methods("GET")
	router.HandleFunc("/speaker", managementinterface.SpeakerTestHandler).Methods("GET")
	router.HandleFunc("/speaker/status", managementinterface.SpeakerStatusHandler).Methods("GET")
	router.HandleFunc("/disk-memory", managementinterface.DiskMemoryHandler).Methods("GET")
	router.HandleFunc("/location", managementinterface.GenLocationHandler(config.config)).Methods("GET") // Form to view and/or set location manually.
	router.HandleFunc("/clock", managementinterface.TimeHandler).Methods("GET", "POST")                  // Form to view and/or adjust time settings.
	router.HandleFunc("/about", managementinterface.AboutHandlerGen(config.config)).Methods("GET")
	router.HandleFunc("/audiobait", managementinterface.AudiobaitHandlerGen(config.config)).Methods("GET")

	router.HandleFunc("/advanced", managementinterface.AdvancedMenuHandler).Methods("GET")
	router.HandleFunc("/camera", managementinterface.CameraHandler).Methods("GET")
	router.HandleFunc("/camera/snapshot", managementinterface.CameraSnapshot).Methods("GET")
	router.HandleFunc("/rename", managementinterface.Rename).Methods("GET")

	// API
	apiObj, err := api.NewAPI(config.config)
	if err != nil {
		log.Fatal(err)
		return
	}
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/device-info", apiObj.GetDeviceInfo).Methods("GET")
	apiRouter.HandleFunc("/recordings", apiObj.GetRecordings).Methods("GET")
	apiRouter.HandleFunc("/recording/{id}", apiObj.GetRecording).Methods("GET")
	apiRouter.HandleFunc("/recording/{id}", apiObj.DeleteRecording).Methods("DELETE")
	apiRouter.HandleFunc("/camera/snapshot", apiObj.TakeSnapshot).Methods("PUT")
	apiRouter.HandleFunc("/signal-strength", apiObj.GetSignalStrength).Methods("GET")
	apiRouter.HandleFunc("/reregister", apiObj.Reregister).Methods("POST")
	apiRouter.HandleFunc("/reboot", apiObj.Reboot).Methods("POST")
	apiRouter.HandleFunc("/config", apiObj.GetConfig).Methods("GET")
	apiRouter.HandleFunc("/clear-config-section", apiObj.ClearConfigSection).Methods("POST")
	apiRouter.HandleFunc("/location", apiObj.SetLocation).Methods("POST")              // Set location via a POST request.
	apiRouter.HandleFunc("/clock", managementinterface.APITimeHandler).Methods("POST") // Set times via a POST request.
	apiRouter.Use(basicAuth)

	listenAddr := fmt.Sprintf(":%d", config.Port)
	log.Printf("listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, router))
}

func basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userPassEncoded := "YWRtaW46ZmVhdGhlcnM=" // admin:feathers base64 encoded.
		if r.Header.Get("Authorization") == "Basic "+userPassEncoded {
			next.ServeHTTP(w, r)
		} else {
			http.Error(w, "Forbidden", http.StatusForbidden)
		}
	})
}
