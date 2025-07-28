/*
management-interface - Web based management of Raspberry Pis over WiFi
Copyright (C) 2019, The Cacophony Project

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
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/TheCacophonyProject/audiobait/v3/audiofilelibrary"
	"github.com/TheCacophonyProject/audiobait/v3/playlist"
	goconfig "github.com/TheCacophonyProject/go-config"
	"github.com/TheCacophonyProject/rpi-net-manager/netmanagerclient"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/TheCacophonyProject/go-utils/logging"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/mux"
)

var log = logging.NewLogger("info")

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
		s, err := templateBox.FindString(name)
		if err != nil {
			log.Fatal(err)
		}
		template.Must(t.Parse(s))
	}
}

// NetworkConfig is a struct to store our network configuration values in.
type NetworkConfig struct {
	Online bool `yaml:"online"`
}

// Get the host name (device name) this executable was started on.
// Store it in a module level variable. It is inserted into the html templates at run time.
func getDeviceName() string {
	name, err := os.Hostname()
	if err != nil {
		log.Println(err.Error())
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
	out, err := io.ReadAll(file)
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

// Return the salt minion ID for the device.
func getSaltMinionID() string {
	return strings.TrimSpace(readFile("/etc/salt/minion_id"))
}

// Return the time of the last salt update.
func getLastSaltUpdate() string {
	timeStr := strings.TrimSpace(readFile("/etc/cacophony/last-salt-update"))
	if timeStr == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// Return context from file returning an empty string if on windows or if read fails
func readFile(file string) string {
	if runtime.GOOS == "windows" {
		return ""
	}

	// The /etc/salt/minion_id file contains the ID.
	out, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return string(out)
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
		log.Println(err.Error())
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
		log.Println(err.Error())
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
			titleCaser := cases.Title(language.Und)
			words[0] = titleCaser.String(words[0])
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
	outputStruct := table{
		NumDiskRows: len(outputStrings), DiskDataRows: outputStrings,
		NumMemoryRows: len(outputStrings2), MemoryDataRows: outputStrings2,
	}

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

	config := &NetworkConfig{
		Online: true,
	}

	state := networkState{
		Interfaces:       interfaces,
		Config:           *config,
		ErrorEncountered: err != nil,
		ErrorMessage:     errorMessage,
	}

	// Need to respond to individual requests to test if a network status is up or down.
	tmpl.ExecuteTemplate(w, "network.html", state)
}

type wifiNetwork struct {
	ID        string
	NetworkID int
}

// WifiNetworkHandler show the wireless networks listed in the wpa_supplicant configuration
func WifiNetworkHandler(w http.ResponseWriter, r *http.Request) {
	type wifiProperties struct {
		AvailableNetworks []string
		Networks          []wifiNetwork
		Error             string
	}
	wifiProps := wifiProperties{}

	wifiNetworks, err := netmanagerclient.ListUserSavedWifiNetworks()
	if err != nil {
		log.Println(err)
		wifiProps.Error = "Error while getting saved networks: " + err.Error()
	}
	wifiProps.Networks = []wifiNetwork{}
	for _, network := range wifiNetworks {
		if network.ID == "" || network.ID == "bushnet" || network.ID == "Bushnet" || network.ID == "BushnetHotspot" {
			continue
		}
		wifiProps.Networks = append(wifiProps.Networks, wifiNetwork{ID: network.ID})
	}

	availableWifiNetworks, err := netmanagerclient.ScanWiFiNetworks()
	wifiProps.AvailableNetworks = []string{}
	if err != nil {
		log.Println(err)
		wifiProps.Error = "Error while getting available networks: " + err.Error()
	}
	for _, network := range availableWifiNetworks {
		wifiProps.AvailableNetworks = append(wifiProps.AvailableNetworks, network.SSID)
	}

	if wifiProps.Error == "" && err != nil {
		wifiProps.Error = "Wifi Error: " + err.Error()
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

// AboutHandlerGen is a wrapper for the AboutHandler function.
func AboutHandlerGen(conf *goconfig.Config) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		AboutHandler(w, r, conf)
	}
}

// AboutHandler shows the currently installed packages on the device.
func AboutHandler(w http.ResponseWriter, r *http.Request, conf *goconfig.Config) {

	type aboutResponse struct {
		RaspberryPiSerialNumber string
		SaltMinionID            string
		Group                   string
		DeviceID                int
		LastSaltUpdate          string
		SaltNodegroup           string
		PackageDataRows         [][]string
		ErrorMessage            string
	}

	// Get the device group from the API
	var device goconfig.Device
	if err := conf.Unmarshal(goconfig.DeviceKey, &device); err != nil {
		log.Printf("/device-info failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "failed to read device config\n")
		return
	}

	// Create response
	resp := aboutResponse{
		RaspberryPiSerialNumber: getRaspberryPiSerialNumber(),
		SaltMinionID:            getSaltMinionID(),
		Group:                   device.Group,
		DeviceID:                device.ID,
		LastSaltUpdate:          getLastSaltUpdate(),
		SaltNodegroup:           readFile("/etc/cacophony/salt-nodegroup"),
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

// CameraHandler will show a frame from the camera to help with positioning
func CameraHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "camera.html", nil)
}

func LowPowerThermalRecordingHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "low-power-thermal-recording.html", nil)
}

// CameraSnapshot - Still image from Lepton camera
func CameraSnapshot(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "/var/spool/cptv/still.png")
}

func TimeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "clock.html", nil)
}

// Rename page to change device name and group
func Rename(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "rename.html", nil)
}

// Config page to change devices config
func Config(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "config.html", nil)
}

func Modem(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "modem.html", nil)
}

func Battery(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "battery.html", nil)
}

func DownloadTemperatureCSV(w http.ResponseWriter, r *http.Request) {
	filePath := "/var/log/temperature.csv"

	file, err := os.Open(filePath)
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=temperature.csv")

	_, err = io.Copy(w, file)
	if err != nil {
		log.Println(err)
		return
	}
}

func DownloadBatteryCSV(w http.ResponseWriter, r *http.Request) {
	filePath := "/var/log/battery-readings.csv"

	file, err := os.Open(filePath)
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=battery-readings.csv")

	_, err = io.Copy(w, file)
	if err != nil {
		log.Println(err)
		return
	}
}

type audiobaitResponse struct {
	Running      bool
	Schedule     schedule
	Message      string
	ErrorMessage string
}

type schedule struct {
	Combos        []combo
	PlayNights    int
	ControlNights int
	StartDay      int
	Description   string
}

type combo struct {
	Every     int // Minutes
	From      string
	Until     string
	SoundInfo []soundInfo
}

type soundInfo struct {
	SoundFileName        string
	Volume               int
	SoundFileDisplayText string
	Waits                int
	ID                   int
}

func Audiobait(w http.ResponseWriter, r *http.Request) {
	// TODO Rather than generating the HTML like this it should probably use the
	// audiobait APIs that are now available...
	playSchedule, err := playlist.LoadScheduleFromDisk(goconfig.DefaultAudioBait().Dir)
	if err != nil {
		log.Println(err)
		tmpl.ExecuteTemplate(w, "audiobait.html", audiobaitResponse{
			ErrorMessage: err.Error(),
		})
		return
	}
	library, err := audiofilelibrary.OpenLibrary(goconfig.DefaultAudioBait().Dir)
	if err != nil {
		log.Println(err)
		tmpl.ExecuteTemplate(w, "audiobait.html", audiobaitResponse{
			ErrorMessage: err.Error(),
		})
		return
	}
	combos := make([]combo, len(playSchedule.Combos))
	for i, c := range playSchedule.Combos {

		soundInfos := make([]soundInfo, len(c.Sounds))
		for i, sound := range c.Sounds {
			si := soundInfo{
				Volume: c.Volumes[i],
				Waits:  c.Waits[i] / 60,
			}

			if sound == "random" || sound == "same" {
				si.SoundFileDisplayText = sound
			} else {
				soundId, err := strconv.Atoi(sound)
				if err != nil {
					log.Println(err)
					return
				}
				si.ID = soundId
				si.SoundFileDisplayText = library.FilesByID[soundId]
				si.SoundFileName = library.FilesByID[soundId]
			}

			soundInfos[i] = si
		}
		combos[i] = combo{
			Every:     c.Every / 60,
			From:      c.From.Format("15:04"),
			Until:     c.Until.Format("15:04"),
			SoundInfo: soundInfos,
		}
	}
	ar := &audiobaitResponse{
		Schedule: schedule{
			ControlNights: playSchedule.ControlNights,
			Combos:        combos,
			PlayNights:    playSchedule.PlayNights,
			StartDay:      playSchedule.StartDay,
			Description:   playSchedule.Description,
		},
		Running: true,
	}
	tmpl.ExecuteTemplate(w, "audiobait.html", ar)
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
