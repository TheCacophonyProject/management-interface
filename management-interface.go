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
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/mux"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const fileName = "IEEE_float_mono_32kHz.wav"          // Default sound file name.
const secondaryPath = "/usr/lib/management-interface" // Check here if the file is not found in the executable directory.

// The file system location of this execuable.
var executablePath = ""

// Using a packr box means the html files are bundled up in the binary application.
var templateBox = packr.NewBox("./html")

// tmpl is our pointer to our parsed templates.
var tmpl *template.Template

// This does some initialisation.  It parses our html templates up front and
// finds the location where this executable was started.
func init() {
	tmpl = template.New("")

	for _, name := range templateBox.List() {
		t := tmpl.New(name)
		template.Must(t.Parse(templateBox.String(name)))
	}

	executablePath = getExecutablePath()

}

// Get the directory of where this executable was started.
func getExecutablePath() string {
	ex, err := os.Executable()
	if err != nil {
		log.Printf(err.Error())
		return ""
	}
	return filepath.Dir(ex)
}

// Return info on the disk space available, disk space used etc.
func getDiskSpace() (string, error) {
	var out []byte
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
		return err.Error(), err
	}
	return string(out), nil

}

// Return info on memory e.g. memory used, memory available etc.
func getMemoryStats() (string, error) {

	var out []byte
	err := error(nil)
	if runtime.GOOS == "windows" {
		// Will show more than just memory stuff.
		out, err = exec.Command("cmd", "/C", "systeminfo").Output()
	} else {
		// 'Nix.  Run vmstat command to show memory stats.
		out, err = exec.Command("sh", "-c", "vmstat -s").Output()
	}

	if err != nil {
		log.Printf(err.Error())
		return err.Error(), err
	}
	return string(out), nil
}

// DiskMemoryHandler shows disk space usage and memory usage
func DiskMemoryHandler(w http.ResponseWriter, r *http.Request) {

	diskData, err := getDiskSpace()
	if err != nil {
		log.Fatal(err)
	}

	// Want to separate this into separate fields so that can display in a table in HTML
	outputStrings := [][]string{}
	rows := strings.Split(diskData, "\n")
	for _, row := range rows[1:] {
		words := strings.Fields(row)
		outputStrings = append(outputStrings, words)
	}

	memoryData, err := getMemoryStats()
	if err != nil {
		log.Fatal(err)
	}
	// Want to separate this into separate fields so that can display in a table in HTML
	outputStrings2 := [][]string{}
	rows = strings.Split(memoryData, "\n")
	for _, row := range rows[1:] {
		cleanRow := strings.Trim(row, " \t")
		words := strings.SplitN(cleanRow, " ", 2)
		if len(words) > 1 && strings.HasPrefix(words[1], "K ") {
			words[0] = words[0] + " K"
			words[1] = words[1][2:]
		}
		outputStrings2 = append(outputStrings2, words)
	}

	// Put it all in a struct so we can access it from HTML
	type table struct {
		NumDiskRows    int
		DiskDataRows   [][]string
		NumMemoryRows  int
		MemoryDataRows [][]string
	}
	outputStruct := table{NumDiskRows: len(outputStrings), DiskDataRows: outputStrings,
		NumMemoryRows: len(outputStrings2), MemoryDataRows: outputStrings2}

	// Execute the actual template.
	tmpl.ExecuteTemplate(w, "disk-memory.html", outputStruct)

}

// IndexHandler is the root handler.
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "index.html", nil)
}

// Get the IP address for a given interface.  There can be 0, 1 or 2 (e.g. IPv4 and IPv6)
func getIPAddresses(iface net.Interface) []string {

	var IPAddresses []string

	addrs, err := iface.Addrs()
	if err != nil {
		return IPAddresses // Blank entry.
	}

	for _, addr := range addrs {
		IPAddresses = append(IPAddresses, "  "+addr.String())
	}
	return IPAddresses
}

// NetworkInterfacesHandler - Show the status of each network interface
func NetworkInterfacesHandler(w http.ResponseWriter, r *http.Request) {

	// Type used in serving interface information.
	type interfaceProperties struct {
		Name        string
		IPAddresses []string
	}

	ifaces, err := net.Interfaces()
	interfaces := []interfaceProperties{}
	if err != nil {
		log.Print(err.Error())
	} else {
		// Filter out loopback interfaces
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback == 0 {
				// Not a loopback interface
				addresses := getIPAddresses(iface)
				ifaceProperties := interfaceProperties{Name: iface.Name, IPAddresses: addresses}
				interfaces = append(interfaces, ifaceProperties)
			}
		}
	}

	// Need to respond to individual requests to test if a network status is up or down.
	tmpl.ExecuteTemplate(w, "network-interfaces.html", interfaces)
}

// WifiNetworkHandler - Show the wireless netowrks the pi can see
func WifiNetworkHandler(w http.ResponseWriter, r *http.Request) {

	//wirelessNetworks :=
	//ifaces, err := net.Interfaces()
	configFile := "/home/zaza/go/src/github.com/TheCacophonyProject/management-interface/sup_test.conf"
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			log.Print(err.Error())
			return
		}

		ssid := r.FormValue("ssid")
		password := r.FormValue("password")
		addWpaNetwork(configFile, ssid, password)
		log.Print("Ssid: " + ssid + " password: " + password)
		//Add new network

	}
	networks := parseWpaSupplicantConfig(configFile)
	tmpl.ExecuteTemplate(w, "wifi-networks.html", networks)
}

// WifiNetworkHandler - Show the wireless netowrks the pi can see
func DeleteNetworkHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	log.Print("Delete network handler" + id)
	var out []byte
	err := error(nil)
	out, err = exec.Command("wpa_cli", "remove_network", id).Output()
	log.Print(string(out))
	if err != nil {
		log.Print("removing " + err.Error())
	}
	_, err = exec.Command("wpa_cli", "save", "config").Output()
	if err != nil {
		log.Print("saving " + err.Error())
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "network deleted")
}

func dequote(input string) string {
	if strings.HasPrefix(input, "\"") && strings.HasSuffix(input, "\"") {
		input = input[1 : len(input)-1]
	}
	return input
}

type wifiNetwork struct {
	Ssid      string
	NetworkID int
}

func addWpaNetwork(configFile string, ssid string, password string) {
	var out []byte
	err := error(nil)
	out, err = exec.Command("wpa_cli", "add_network").Output()
	if err != nil {
		log.Print(err)
	}
	stdOut := string(out)
	var networkId int = 0

	scanner := bufio.NewScanner(strings.NewReader(stdOut))
	log.Print("read output")
	for scanner.Scan() {
		line := scanner.Text()
		networkId, err = strconv.Atoi(line)

	}
	cmd := exec.Command("wpa_cli")
	stdin, err := cmd.StdinPipe()
	defer stdin.Close()

	inputS := fmt.Sprintf("set_network %d ssid \"%s\"\n", networkId, ssid)
	inputS = inputS + fmt.Sprintf("set_network %d psk \"%s\"\n", networkId, password)
	inputS = inputS + fmt.Sprintf("enable_network %d\n", networkId) + "save config\n" + "quit\n"
	log.Print(inputS)
	io.WriteString(stdin, inputS)
	/*log.Print()
	io.WriteString(stdin, fmt.Sprintf("set_network %d ssid \"%s\"\n", networkId, ssid))
	io.WriteString(stdin, fmt.Sprintf("set_network %d psk %s\n", networkId, password))
	io.WriteString(stdin, fmt.Sprintf("enable_network %d\n", networkId))
	io.WriteString(stdin, "save config\n")
	io.WriteString(stdin, "quit\n")*/
	err = cmd.Run()
	if err != nil {
		log.Print(err)
	}
}

func parseWpaSupplicantConfig(configFile string) []wifiNetwork {
	var out []byte
	err := error(nil)
	out, err = exec.Command("wpa_cli", "list_networks").Output()

	if err != nil {
		log.Printf("error parsing wpa config" + err.Error())
		//	return err.Error(), err
	}
	networkList := string(out)
	networks := []wifiNetwork{}
	var id int
	scanner := bufio.NewScanner(strings.NewReader(networkList))
	for scanner.Scan() {
		line := scanner.Text()
		log.Print(line)
		parts := strings.Split(line, "\t")
		log.Print(len(parts))
		if len(parts) > 2 {
			if id, err = strconv.Atoi(parts[0]); err == nil {
				if parts[1] != "bushnet" {
					wNetwork := wifiNetwork{Ssid: parts[1], NetworkID: id}
					networks = append(networks, wNetwork)
				}
			}
		}
	}

	sort.Slice(networks, func(i, j int) bool { return networks[i].Ssid < networks[j].Ssid })
	return networks

	/*

		, nil

		file, err := os.Open(configFile)
		if err != nil {
			log.Print(err.Error())
		}
		defer file.Close()

		networks := []wifiNetwork{}

		//networks := map[string]map[string]string{}
		var networkMap map[string]string

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimSpace(line)
			if line == "}" {
				ssid := networkMap["ssid"]
				if ssid == "" {
					log.Print("Empty SSID")
				} else if ssid != "bushnet"{
					wNetwork := wifiNetwork{Ssid: networkMap["ssid"], PassKey: networkMap["psk"]}
					networks = append(networks, wNetwork)
				}
				networkMap = nil
			} else if line != "" {
				parts := strings.Split(line, "=")
				if len(parts) != 2 {
					log.Print("Line incorrectly formated")
					//SOME kind of error
					break
				}
				key := parts[0]
				value := parts[1]
				if value == "{" {
					if key != "network" {
						log.Print("Line unsupported section")
						//  raise ParseError('unsupported section: "{}"'.format(left))
					} else if networkMap != nil {
						log.Print("Can't nest networks")
						//raise ParseError("can't nest networks")
					} else {
						networkMap = map[string]string{}
					}
				} else {
					networkMap[key] = dequote(value)
				}
			}
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}

		sort.Slice(networks, func(i, j int) bool { return networks[i].Ssid < networks[j].Ssid })
		return networks*/
}

// CheckInterfaceHandler checks an interface to see if it is up or down.
// To do this the ping command is used to send data to Cloudfare at 1.1.1.1
func CheckInterfaceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	response := make(map[string]string)
	// Extract interface name
	interfaceName := mux.Vars(r)["name"]
	// Lookup interface by name
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		response["status"] = "unknown"
		response["result"] = "Unable to find interface with name " + interfaceName
		json.NewEncoder(w).Encode(response)
		return
	}
	args := []string{"-I", iface.Name, "-c", "3", "-n", "-W", "15", "1.1.1.1"}
	output, err := exec.Command("ping", args...).Output()
	w.WriteHeader(http.StatusOK)
	response["result"] = string(output)
	if err != nil {
		// Ping was not successful
		response["status"] = "down"
	} else {
		response["status"] = "up"
	}
	json.NewEncoder(w).Encode(response)
}

// SpeakerTestHandler will show a frame from the camera to help with positioning
func SpeakerTestHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "speaker-test.html", nil)
}

// fileExists returns whether the given file or directory exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	} else {
		return false
	}
}

// findAudioFile locates our test audio file.  It returns true and the location of the file
// if the file is found. And false and empty string otherwise.
func findAudioFile() (bool, string) {

	// Check if the file is in the executable directory
	if fileExists(filepath.Join(executablePath, fileName)) {
		return true, filepath.Join(executablePath, fileName)
	}

	// In our default, second location?
	if fileExists(filepath.Join(secondaryPath, fileName)) {
		log.Printf("Secondary file path is: %s", filepath.Join(secondaryPath, fileName))
		return true, filepath.Join(secondaryPath, fileName)
	}

	// Test sound not available
	return false, ""

}

// SpeakerStatusHandler attempts to play a sound on connected speaker(s).
func SpeakerStatusHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	response := make(map[string]string)

	result, testAudioFileLocation := findAudioFile()
	if result {
		// Play the sound file
		args := []string{"-v10", "-q", testAudioFileLocation}
		output, err := exec.Command("play", args...).CombinedOutput()
		response["result"] = string(output)
		if err != nil {
			// Play command was not successful
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf(err.Error())
		} else {
			w.WriteHeader(http.StatusOK)
		}
	} else {
		// Report that the file was not found.
		w.WriteHeader(http.StatusInternalServerError)
		response["result"] = "File " + fileName + " not found."
		log.Printf("File " + fileName + " not found")
	}

	// Encode data to be sent back to html.
	json.NewEncoder(w).Encode(response)
}

// CameraHandler will show a frame from the camera to help with positioning
func CameraHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "camera.html", nil)
}

// CameraSnapshot - Still image from Lepton camera
func CameraSnapshot(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "/var/spool/cptv/still.png")
}
