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

	// Get a list of network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	outputStrings := make([]StringToBeDisplayed, 0)

	if runtime.GOOS == "linux" {

		// Go through each network interface and test if it is up or down.
		for _, i := range interfaces {

			outputStrings = append(outputStrings, StringToBeDisplayed{Text: fmt.Sprintf("Testing %v...", i.Name)})
			outputStrings = append(outputStrings, StringToBeDisplayed{Text: fmt.Sprintf("  Sending %v packets", numPacketsToSend)})

			interfaceUp := false
			commandStr := "ping -I  " + i.Name + " -c " + strconv.Itoa(numPacketsToSend) + " -n -W 15 1.1.1.1"
			out, _ := exec.Command("sh", "-c", commandStr).Output()
			// Did we get the output we'd expect if the network interface is up?
			outStr := string(out)
			fmt.Println(outStr) // Remove later
			pos := strings.Index(outStr, "transmitted")
			if pos != -1 {
				numPacketsReceivedStr := string(outStr[pos+13])
				outputStrings = append(outputStrings, StringToBeDisplayed{Text: fmt.Sprintf("  Received %v packets", numPacketsReceivedStr)})
				numPacketsReceived, _ := strconv.Atoi(numPacketsReceivedStr)
				if numPacketsReceived > 0 {
					// I consider this interface to be "up".
					interfaceUp = true
				}
			}

			if interfaceUp {
				outputStrings = append(outputStrings, StringToBeDisplayed{Text: fmt.Sprintf("\n  %v is UP.", i.Name)})
			} else {
				outputStrings = append(outputStrings, StringToBeDisplayed{Text: fmt.Sprintf("\n  %v is DOWN.", i.Name)})
			}
			outputStrings = append(outputStrings, StringToBeDisplayed{Text: "------------------"})

		}
	} else {
		// Windows
		// It's difficult to ping from a particular interface from Windows so just show the output of ipconfig
		out, _ := exec.Command("cmd", "/C", "ipconfig").Output()
		// Want to separate this into multiple lines so that can display each line on a separate line in HTML
		for _, str := range strings.Split(string(out), "\n") {
			outputStrings = append(outputStrings, StringToBeDisplayed{Text: str})
		}

	}
	return outputStrings, nil
}
