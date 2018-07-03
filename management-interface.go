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
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)

const NumPacketsToSend int = 3

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
	fmt.Fprintf(w, "Disk space usage is: \n\n%s\n", out)
	return nil

}

// Show the available network interfaces on this machine.
func availableInterfaces(w http.ResponseWriter) error {

	interfaces, err := net.Interfaces()

	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Available network interfaces on this machine:\n\n")
	for _, i := range interfaces {
		if i.Name != "lo" {

			fmt.Fprintf(w, "Testing %v...\n", i.Name)
			fmt.Fprintf(w, "  Sending %v packets\n", NumPacketsToSend)

			// Windows
			//commandStr := "ping -n " + strconv.Itoa(NumPacketsToSend) + " -w 15 1.1.1.1"
			//out, _ := exec.Command("cmd", "/C", commandStr).Output()
			//pos := strings.Index(outStr, "Received =")
			//if pos != -1 {
			//	numPacketsReceivedStr := string(outStr[pos+11])

			// Unix
			commandStr := "ping -I " + i.Name + " -c " + strconv.Itoa(NumPacketsToSend) + " -n -W 15 1.1.1.1"
			out, _ := exec.Command("sh", "-c", commandStr).Output()

			// Did we get the output we'd expect if the network interface is up?
			interfaceUp := false
			outStr := string(out)
			pos := strings.Index(outStr, "transmitted")
			if pos != -1 {
				numPacketsReceivedStr := string(outStr[pos+13])
				fmt.Fprintf(w, "  Received %v packets\n", numPacketsReceivedStr)
				numPacketsReceived, _ := strconv.Atoi(numPacketsReceivedStr)
				if numPacketsReceived > 0 {
					// I consider this interface to be "up".
					interfaceUp = true
				}
			}
			if interfaceUp {
				fmt.Fprintf(w, "\n  %v is UP.\n\n", i.Name)
			} else {
				fmt.Fprintf(w, "\n  %v is DOWN.\n\n", i.Name)
			}

		}
	}
	return nil
}

func indexHandler(w http.ResponseWriter, r *http.Request) {

	err := showDiskSpace(w)
	if err != nil {
		log.Fatal(err)
	}
}

// Show the status of each newtwork interface
func networkInterfacesHandler(w http.ResponseWriter, r *http.Request) {
	err := availableInterfaces(w)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/net/", networkInterfacesHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))

}
