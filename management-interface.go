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
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
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

	"github.com/gobuffalo/packr"
	"github.com/gorilla/mux"
	yaml "gopkg.in/yaml.v2"
)

const fileName = "IEEE_float_mono_32kHz.wav"          // Default sound file name.
const secondaryPath = "/usr/lib/management-interface" // Check here if the file is not found in the executable directory.

const networkConfigFile = "/etc/cacophony/network.yaml"

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

// getNetworkSSID gets the ssid from the wpa_supplicant configuration with the specified id
func getNetworkSSID(networkID string) (string, error) {
	out, err := exec.Command("wpa_cli", "get_network", networkID, "ssid").Output()
	if err != nil {
		return "", fmt.Errorf("error executing wpa_cli get_network %s - error %s output %s", networkID, err, out)
	}

	stdOut := string(out)
	scanner := bufio.NewScanner(strings.NewReader(stdOut))
	scanner.Scan() //skip 1st line interface line
	scanner.Scan()
	ssid := scanner.Text()
	return ssid, err

}

// deleteNetwork removes the network from the wpa_supplicant configuration with specified id.
func deleteNetwork(id string) error {
	//check if is bushnet
	ssid, err := getNetworkSSID(id)
	if strings.ToLower(ssid) == "\"bushnet\"" {
		return errors.New("error bushnet cannot be deleted")
	}

	//remove network
	cmd := exec.Command("wpa_cli")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("error getting stdin pipe from cmd -error %s", err)
	}
	defer stdin.Close()
	io.WriteString(stdin, fmt.Sprintf("remove_network %s\n", id))
	io.WriteString(stdin, "quit\n")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error deleting wpa network -error %s", err)
	}
	errOccured := hasErrorOccured(string(out))
	if errOccured {
		reloadWPAConfig()
		err = errors.New("error deleting network")
		return err
	}

	//save and reload config
	err = saveWPAConfig()
	reloadErr := reloadWPAConfig()
	if err == nil { //probably wont happen
		err = reloadErr
	}
	return err
}

// doesWpaNetworkExist checks for a network with the specified ssid in the wpa_supplicant configuration.
func doesWPANetworkExist(ssid string) (bool, error) {
	networks, err := parseWPASupplicantConfig()
	if err != nil {
		return false, err
	}
	for _, v := range networks {
		if strings.ToLower(v.SSID) == strings.ToLower(ssid) {
			return true, nil
		}
	}
	return false, nil
}

// addWPANetwork adds a new wpa network in the wpa_supplication configuration
// with specified ssid and password (if it doesn't already exist)
func addWPANetwork(ssid string, password string) error {
	if ssid == "" {
		return errors.New("SSID must have a value")
	} else if strings.ToLower(ssid) == "bushnet" {
		return errors.New("SSID cannot be bushnet")
	}

	networkExists, err := doesWPANetworkExist(ssid)
	if err != nil {
		return err
	}
	if networkExists {
		return fmt.Errorf("SSID %s already exists", ssid)
	}

	networkId, err := addNewNetwork()
	if err != nil {
		return err
	}

	err = setWPANetworkDetails(ssid, password, networkId)
	if err != nil {
		return err
	}

	err = saveWPAConfig()
	reloadErr := reloadWPAConfig()
	if err == nil { //probably wont happen
		err = reloadErr
	}
	return err
}

// addNewNetwork adds a new network in the wpa_supplication configuration and returns the new network id
func addNewNetwork() (int, error) {
	out, err := exec.Command("wpa_cli", "add_network").Output()
	var networkId int = -1

	if err != nil {
		return networkId, fmt.Errorf("error executing wpa_cli add_network - error %s output %s", err, out)
	}
	stdOut := string(out)

	//get the networkid of the new networks from stdOut
	scanner := bufio.NewScanner(strings.NewReader(stdOut))
	scanner.Scan() //skip interface line
	if scanner.Scan() {
		line := scanner.Text()
		networkId, err = strconv.Atoi(line)
		if err != nil {
			return -1, fmt.Errorf("could not find network id - error %s from stdout %s", err, stdOut)
		}
	}
	return networkId, err
}

// setWPANetworkDetails sets the ssid and password of the specified networkID in the wpa_supplication configuration
func setWPANetworkDetails(ssid string, password string, networkId int) error {
	cmd := exec.Command("wpa_cli")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("error getting stdin pipe from cmd: %s", err)
	}

	defer stdin.Close()
	io.WriteString(stdin, fmt.Sprintf("set_network %d ssid \"%s\"\n", networkId, ssid))
	io.WriteString(stdin, fmt.Sprintf("set_network %d psk \"%s\"\n", networkId, password))
	io.WriteString(stdin, fmt.Sprintf("enable_network %d\n", networkId))
	io.WriteString(stdin, "quit\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error adding wpa network -error %s", err)
	}

	errOccured := hasErrorOccured(string(out))
	if errOccured {
		reloadWPAConfig()
		err = errors.New("error setting new network")
	}
	return err
}

// reloadWPAConfig executes wpa_cli reconfigure
func reloadWPAConfig() error {
	out, err := exec.Command("wpa_cli", "reconfigure").Output()
	if err != nil {
		return fmt.Errorf("error reloading config - error %s output %s", err, out)
	}

	errOccured := hasErrorOccured(string(out))
	if errOccured {
		err = errors.New("error reloading config")
	}
	return err
}

// hasErrorOccured checks string for FAIL text
func hasErrorOccured(stdOut string) bool {
	errorOccured := strings.Contains(stdOut, "\nFAIL\n")
	return errorOccured
}

// saveWPAConfig executes wpa_cli save config
func saveWPAConfig() error {
	out, err := exec.Command("wpa_cli", "save", "config").Output()
	if err != nil {
		return fmt.Errorf("error saving config - error %s output %s", err, out)
	}
	errOccured := hasErrorOccured(string(out))
	if errOccured {
		err = errors.New("error saving config")
	}
	return err
}

type wifiNetwork struct {
	SSID      string
	NetworkID int
}

// parseWPASupplicantConfig uses wpa_cli list_networks to get all networks in the wpa_supplicant configuration
func parseWPASupplicantConfig() ([]wifiNetwork, error) {
	out, err := exec.Command("wpa_cli", "list_networks").Output()
	networks := []wifiNetwork{}

	if err != nil {
		return networks, fmt.Errorf("error listing networks: %v", err)
	}
	networkList := string(out)
	scanner := bufio.NewScanner(strings.NewReader(networkList))
	scanner.Scan() //skip interface listing
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) > 2 {
			if id, err := strconv.Atoi(parts[0]); err == nil {
				if strings.ToLower(parts[1]) != "bushnet" {
					wNetwork := wifiNetwork{SSID: parts[1], NetworkID: id}
					networks = append(networks, wNetwork)
				}
			} else {
				err = fmt.Errorf("error parsing network_id %s for line %s", err, line)
			}
		}
	}

	sort.Slice(networks, func(i, j int) bool { return networks[i].SSID < networks[j].SSID })
	return networks, err
}

// WifiNetworkHandler show the wireless networks listed in the wpa_supplicant configuration
func WifiNetworkHandler(w http.ResponseWriter, r *http.Request) {

	type wifiProperties struct {
		Networks []wifiNetwork
		Error    string
	}
	var err error
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			log.Printf("WifiNetworkHandler error parsing form: %s", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		deleteID := r.FormValue("deleteID")
		if deleteID != "" {
			err = deleteNetwork(deleteID)
		} else {
			ssid := r.FormValue("ssid")
			password := r.FormValue("password")
			err = addWPANetwork(ssid, password)
		}
	}

	wifiProps := wifiProperties{}
	if err != nil {
		wifiProps.Error = err.Error()
	}
	wifiProps.Networks, err = parseWPASupplicantConfig()

	if wifiProps.Error == "" && err != nil {
		wifiProps.Error = err.Error()
	}

	tmpl.ExecuteTemplate(w, "wifi-networks.html", wifiProps)
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

func newClientError(msg string) *clientError {
	return &clientError{msg}
}

type clientError struct {
	msg string
}

func (e *clientError) Error() string {
	return e.msg
}

func isClientError(err error) bool {
	_, ok := err.(*clientError)
	return ok
}

func trimmedFormValue(r *http.Request, name string) string {
	return strings.TrimSpace(r.FormValue(name))
}

func parseFloat(val string) (float64, bool) {
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func floatToString(val float64) string {
	if val == 0 {
		return ""
	}
	return fmt.Sprint(val)
}

func successMessage(err error, msg string) string {
	if err == nil {
		return msg
	}
	return ""
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
