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

package managementapi

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
)

const (
	cptvDir  = "/var/spool/cptv/"
	cptvGlob = "*.cptv"
)

// GetRecordings returns a list of cptv files in a array.
func GetRecordings(w http.ResponseWriter, r *http.Request) {
	log.Println("get recordings")
	names := getCptvNames()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(names)
}

// GetRecording downloads a cptv file
func GetRecording(w http.ResponseWriter, r *http.Request) {
	recordingName := mux.Vars(r)["id"]
	log.Printf("get recording '%s'", recordingName)
	if checkIfCptvFile(recordingName) == false {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "cptv file not found\n")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", recordingName))
	w.Header().Set("Content-Type", "application/x-cptv")
	f, err := os.Open(filepath.Join(cptvDir, recordingName))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	w.WriteHeader(http.StatusOK)
	reader := bufio.NewReader(f)
	io.Copy(w, reader)
}

// DeleteRecording deletes the given cptv file
func DeleteRecording(w http.ResponseWriter, r *http.Request) {
	// check that it is a cptv recording that is requested.
	cptvName := mux.Vars(r)["id"]
	log.Printf("delete cptv '%s'", cptvName)
	if checkIfCptvFile(cptvName) == false {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "cptv file not found\n")
		return
	}
	err := os.Remove(filepath.Join(cptvDir, cptvName))
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "failed to delete file")
		return
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "cptv file deleted")
}

func getCptvNames() []string {
	matches, _ := filepath.Glob(filepath.Join(cptvDir, cptvGlob))
	names := make([]string, len(matches))
	for i, filename := range matches {
		names[i] = filepath.Base(filename)
	}
	return names
}

func checkIfCptvFile(cptv string) bool {
	for _, n := range getCptvNames() {
		if n == cptv {
			return true
		}
	}
	return false
}
