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

	"github.com/TheCacophonyProject/management-interface/api"
	goapi "github.com/TheCacophonyProject/go-api"


	"github.com/gobuffalo/packr"
	"github.com/gorilla/mux"
	yaml "gopkg.in/yaml.v2"
)

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

// Return the serial number for the Raspberr Pi in the device.
func getRaspberryPiSerialNumber() string {

	if runtime.GOOS == "windows" {
		return ""
	}

	// The /proc/cpuinfo file normally contains a serial number.
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer file.Close()
	out, err := ioutil.ReadAll(file)
	if err != nil {
		return ""
	}

	// Extract the serial number.
	serialNumber := ""
	rows := strings.Split(string(out), "\n")
	for _, row := range rows {
		parts := strings.Split(row, ":")
		if len(parts) == 2 {
			field := strings.ToUpper(strings.TrimSpace(parts[0]))
			if field == "SERIAL" {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return serialNumber
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
		if len(words) >= 6 {
			words[0], words[5] = words[5], words[0] // This swaps these 2 columns
			outputStrings = append(outputStrings, words)
		}
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
			words[0], words[1] = words[1], words[0] // This reverses the 2 columns
			words[0] = strings.Title(words[0])
		}
		outputStrings2 = append(outputStrings2, words)
		if words[0] == "Free Swap" {
			// Don't want any of the output after this line.
			break
		}
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

// AdvancedMenuHandler is a screen to more advanced settings.
func AdvancedMenuHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "advanced.html", nil)
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

// Return info on the packages that are currently installed on the device.
func getInstalledPackages() (string, error) {

	if runtime.GOOS == "windows" {
		return "", nil
	}

	out, err := exec.Command("/usr/bin/dpkg-query", "--show", "--showformat", "${Package}|${Version}|${Maintainer}\n").Output()

	if err != nil {
		return "", err
	}

	return string(out), nil

}

// AboutHandler shows the currently installed packages on the device.
func AboutHandler(w http.ResponseWriter, r *http.Request, apiObj *api.ManagementAPI) {

	type aboutResponse struct {
		RaspberryPiSerialNumber string
		Group string
		DeviceID int
		PackageDataRows         [][]string
		ErrorMessage            string
	}

	// Create response
	resp := aboutResponse{
		RaspberryPiSerialNumber: getRaspberryPiSerialNumber(),
	}


	// Get the device group from the API
	config, err := goapi.LoadConfig()
	if err != nil {
		log.Println("failed to read device config from API:", err)
	} else {
		resp.Group = config.Group
	}

	// Get the device ID from the device-priv.yaml file locally
	privConfig, err := goapi.LoadPrivateConfig()
	if err != nil {
		log.Println("error loading private config:", err)
	} else {
		if privConfig != nil {
			resp.DeviceID = privConfig.DeviceID
		}
	}

	// Get installed packages.
	packagesData, err := getInstalledPackages()
	if err != nil {
		resp.ErrorMessage = errorMessage(err)
		tmpl.ExecuteTemplate(w, "about.html", resp)
	}
	// Want to separate this into separate fields so that can display in a table in HTML
	dataRows := [][]string{}
	rows := strings.Split(packagesData, "\n")
	for _, row := range rows {
		// We only want packages related to cacophony.
		if !strings.Contains(strings.ToUpper(row), "CACOPHONY") {
			continue
		}
		words := strings.Split(strings.TrimSpace(row), "|")
		dataRows = append(dataRows, words[:2])
	}
	resp.PackageDataRows = dataRows

	tmpl.ExecuteTemplate(w, "about.html", resp)
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

// CameraHandler will show a frame from the camera to help with positioning
func CameraHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "camera.html", nil)
}

// CameraSnapshot - Still image from Lepton camera
func CameraSnapshot(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "/var/spool/cptv/still.png")
}

// Rename page to change device name and group
func Rename(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "rename.html", nil)
}
