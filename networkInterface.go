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
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

const numPacketsToSend int = 3

// StringToBeDisplayed - Holds a string to send to the HTML templates.
type StringToBeDisplayed struct {
	Text string
}

// MultiLineStringToBeDisplayed - A wrapper struct used to send data to the HTML templates.
type MultiLineStringToBeDisplayed struct {
	Strings []StringToBeDisplayed
}

// AvailableInterfaces - Get the status of the available network interfaces on this machine.
// Return an array of strings with that output.
func AvailableInterfaces() ([]StringToBeDisplayed, error) {

	interfaces, err := net.Interfaces()

	if err != nil {
		return nil, err
	}
	outputStrings := make([]StringToBeDisplayed, 0)
	//outputStrings = append(outputStrings, StringToBeDisplayed{Text: "Available network interfaces on this machine:"})
	//fmt.Fprintf(w, "Available network interfaces on this machine:\n\n")
	for _, i := range interfaces {
		if i.Name != "lo" {

			outputStrings = append(outputStrings, StringToBeDisplayed{Text: fmt.Sprintf("Testing %v...", i.Name)})
			//fmt.Fprintf(w, "Testing %v...\n", i.Name)
			outputStrings = append(outputStrings, StringToBeDisplayed{Text: fmt.Sprintf("  Sending %v packets", numPacketsToSend)})
			// fmt.Fprintf(w, "  Sending %v packets\n", NumPacketsToSend)

			interfaceUp := false

			if runtime.GOOS == "windows" {
				// Windows
				commandStr := "ping -n " + strconv.Itoa(numPacketsToSend) + " -w 15 1.1.1.1"
				out, _ := exec.Command("cmd", "/C", commandStr).Output()
				// Did we get the output we'd expect if the network interface is up?
				outStr := string(out)
				pos := strings.Index(outStr, "Received =")
				if pos != -1 {
					numPacketsReceivedStr := string(outStr[pos+11])
					outputStrings = append(outputStrings, StringToBeDisplayed{Text: fmt.Sprintf("  Received %v packets", numPacketsReceivedStr)})
					// fmt.Fprintf(w, "  Received %v packets\n", numPacketsReceivedStr)
					numPacketsReceived, _ := strconv.Atoi(numPacketsReceivedStr)
					if numPacketsReceived > 0 {
						// I consider this interface to be "up".
						interfaceUp = true
					}
				}

			} else {
				// 'Nix
				commandStr := "ping -I  " + i.Name + " -c " + strconv.Itoa(numPacketsToSend) + " -n -W 15 1.1.1.1"
				out, _ := exec.Command("sh", "-c", commandStr).Output()
				// Did we get the output we'd expect if the network interface is up?
				outStr := string(out)
				pos := strings.Index(outStr, "transmitted")
				if pos != -1 {
					numPacketsReceivedStr := string(outStr[pos+13])
					outputStrings = append(outputStrings, StringToBeDisplayed{Text: fmt.Sprintf("  Received %v packets", numPacketsReceivedStr)})
					// fmt.Fprintf(w, "  Received %v packets\n", numPacketsReceivedStr)
					numPacketsReceived, _ := strconv.Atoi(numPacketsReceivedStr)
					if numPacketsReceived > 0 {
						// I consider this interface to be "up".
						interfaceUp = true
					}
				}
			}

			if interfaceUp {
				outputStrings = append(outputStrings, StringToBeDisplayed{Text: fmt.Sprintf("\n  %v is UP.", i.Name)})
				// fmt.Fprintf(w, "\n  %v is UP.\n\n", i.Name)
			} else {
				outputStrings = append(outputStrings, StringToBeDisplayed{Text: fmt.Sprintf("\n  %v is UP.", i.Name)})
				// fmt.Fprintf(w, "\n  %v is DOWN.\n\n", i.Name)
			}
			outputStrings = append(outputStrings, StringToBeDisplayed{Text: "------------------"})

		}
	}
	return outputStrings, nil
}
