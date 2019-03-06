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
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/gobuffalo/packr"
	"github.com/gorilla/mux"
	yaml "gopkg.in/yaml.v2"
)

const fileName = "IEEE_float_mono_32kHz.wav"          // Default sound file name.
const secondaryPath = "/usr/lib/management-interface" // Check here if the file is not found in the executable directory.

const networkConfigFile = "/etc/cacophony/network.yaml"
const deviceLocationFile = "/etc/cacophony/location.yaml"

// The file system location of this execuable.
var executablePath = ""

// Using a packr box means the html files are bundled up in the binary application.
var templateBox = packr.NewBox("./html")

// tmpl is our pointer to our parsed templates.
var tmpl *template.Template

// This does some initialisation.  It parses our html templates up front and
// finds the location where this executable was started.
func init() {

	// The name of the device we are running this executable on.
	deviceName := getDeviceName()
	tmpl = template.New("")
	tmpl.Funcs(template.FuncMap{"DeviceName": func() string { return deviceName }})

	for _, name := range templateBox.List() {
		t := tmpl.New(name)
		template.Must(t.Parse(templateBox.String(name)))
	}

	executablePath = getExecutablePath()

}

// NetworkConfig is a struct to store our network configuration values in.
type NetworkConfig struct {
	Online bool `yaml:"online"`
}

// LocationData is a struct to store our location values in.
type LocationData struct {
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`
}

// WriteNetworkConfig writes the config value(s) to the network config file.
// If it doesn't exist, it is created.
func WriteNetworkConfig(filepath string, config *NetworkConfig) error {
	outBuf, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath, outBuf, 0640)
}

// ParseNetworkConfig retrieves a value(s) from the network config file.
func ParseNetworkConfig(filepath string) (*NetworkConfig, error) {

	// Create a default config
	config := &NetworkConfig{Online: true}

	inBuf, err := ioutil.ReadFile(filepath)
	if os.IsNotExist(err) {
		return config, nil
	} else if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(inBuf, config); err != nil {
		return nil, err
	}
	return config, nil
}

// WriteLocationData writes the location values to the location data file.
// If it doesn't exist, it is created.
func WriteLocationData(filepath string, location LocationData) error {
	outBuf, err := yaml.Marshal(location)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath, outBuf, 0640)
}

// ParseLocationFile retrieves values from the location data file.
func ParseLocationFile(filepath string) (*LocationData, error) {

	// Create a default config
	location := &LocationData{}

	inBuf, err := ioutil.ReadFile(filepath)
	if os.IsNotExist(err) {
		return location, nil
	} else if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(inBuf, location); err != nil {
		return nil, err
	}
	return location, nil
}

// Get the host name (device name) this executable was started on.
// Store it in a module level variable. It is inserted into the html templates at run time.
func getDeviceName() string {
	name, err := os.Hostname()
	if err != nil {
		log.Printf(err.Error())
		return "Unknown"
	}
	// Make sure we handle the case when name could be something like: 'host.corp.com'
	// If it is, just use the part before the first dot.
	return strings.SplitN(name, ".", 2)[0]
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

// NetworkHandler - Show the status of each network interface
func NetworkHandler(w http.ResponseWriter, r *http.Request) {

	// Type used in serving interface information.
	type interfaceProperties struct {
		Name        string
		IPAddresses []string
	}

	type networkState struct {
		Interfaces       []interfaceProperties
		Config           NetworkConfig
		ErrorEncountered bool
		ErrorMessage     string
	}

	errorMessage := ""
	ifaces, err := net.Interfaces()
	interfaces := []interfaceProperties{}
	if err != nil {
		errorMessage = err.Error()
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

	// Read online/offline status from 'network.yaml'
	config, err := ParseNetworkConfig(networkConfigFile)
	if err != nil {
		errorMessage += "Failed to read network config file. " + err.Error()
		// Create a default config so that the page will still load.
		config = &NetworkConfig{
			Online: true,
		}
	}

	state := networkState{
		Interfaces:       interfaces,
		Config:           *config,
		ErrorEncountered: err != nil,
		ErrorMessage:     errorMessage}

	// Need to respond to individual requests to test if a network status is up or down.
	tmpl.ExecuteTemplate(w, "network.html", state)
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

// ToggleOnlineState attempts to toggle the 'online' value in the network config file.
func ToggleOnlineState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	type OnlineState struct {
		Online bool
	}

	type Resp struct {
		Result string `json:"result"`
		State  bool   `json:"state"`
	}
	resp := Resp{Result: "", State: true} // Default response.

	// Get any value(s) from the config file.
	config, err := ParseNetworkConfig(networkConfigFile)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		resp.Result = "Failed to read network config file. " + err.Error()
		json.NewEncoder(w).Encode(resp)
		return
	}
	// Set our response to contain our config 'online' value.  If we encounter an error below, we will return this value.
	resp.State = config.Online

	// Get the desired value of 'online' from the request body.
	stateMap := OnlineState{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&stateMap)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		resp.Result = "Failed to understand request. " + err.Error()
		json.NewEncoder(w).Encode(resp)
		return
	}

	// We got the desired 'online' value from the request body, so now write this to our config file.
	config.Online = stateMap.Online
	err = WriteNetworkConfig(networkConfigFile, config)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		resp.Result = "Failed to update network config. " + err.Error()
		json.NewEncoder(w).Encode(resp)
		return
	}

	// We were able to retrieve the 'online' value from the request body and write it to the config file, so all good.
	w.WriteHeader(http.StatusOK)
	resp.Result = "Successfully set online state"
	resp.State = stateMap.Online
	json.NewEncoder(w).Encode(resp)

}

// SpeakerTestHandler will show a frame from the camera to help with positioning
func SpeakerTestHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "speaker-test.html", nil)
}

// fileExists returns whether the given file or directory exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

// handleLocationPostRequest handles a POST request to set the location coordinates of a device.
func handleLocationPostRequest(w http.ResponseWriter, r *http.Request) {

	// Get the latitude and longitude values from the request.
	location := LocationData{}
	fLatitude, err := strconv.ParseFloat(r.FormValue("latitude"), 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	location.Latitude = fLatitude

	fLongitude, err := strconv.ParseFloat(r.FormValue("longitude"), 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	location.Longitude = fLongitude

	// Now write them to the location file.
	err = WriteLocationData(deviceLocationFile, location)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Failed to update location. Could not write to location file. " + err.Error())
		return
	}

	// Everything is fine in the world :)
	w.WriteHeader(http.StatusOK)
}

// LocationHandler shows the location of the device.  The location can be viewed and/or set manually.
func LocationHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" || r.Method == "" {

		// Handle GET request

		type locationResponse struct {
			Location         LocationData
			ErrorEncountered bool
			ErrorMessage     string
		}

		errorMessage := ""
		// Read the location of this device from 'location.yaml'
		location, err := ParseLocationFile(deviceLocationFile)
		if err != nil {
			errorMessage += "Failed to read location data file. " + err.Error()
			// Create a default location struct so that the page will still load.
			location = &LocationData{}
		}

		resp := locationResponse{
			Location:         *location,
			ErrorEncountered: err != nil,
			ErrorMessage:     errorMessage}

		tmpl.ExecuteTemplate(w, "location.html", resp)

	} else {

		// Handle POST request
		handleLocationPostRequest(w, r)

	}

}

// APILocationHandler writes the location of the device to the deviceLocationFile
func APILocationHandler(w http.ResponseWriter, r *http.Request) {

	// This should be a POST request
	if r.Method != "POST" {
		w.WriteHeader(http.StatusBadRequest)
	} else {
		handleLocationPostRequest(w, r)
	}

}
