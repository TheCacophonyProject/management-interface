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
	"os/exec"
)

func showDiskSpace(w http.ResponseWriter) error {

	// Run df command to show disk space available on SD card.
	out, err := exec.Command("sh", "-c", "df -h").Output()
	// On Windows, commands need to be handled like this:
	//out, err := exec.Command("cmd", "/C", "dir").Output()
	if err != nil {
		log.Printf(err.Error())
		fmt.Fprintf(w, err.Error()+"\n")
		fmt.Fprintf(w, "Cannot show disk space at this time.\n")
		return err
	}
	fmt.Fprintf(w, "%s\n", out)
	return nil

}

func indexHandler(w http.ResponseWriter, r *http.Request) {

	fmt.Fprintf(w, "Disk space usage is: \n\n")
	showDiskSpace(w)

}

func main() {

	http.HandleFunc("/", indexHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))

}
