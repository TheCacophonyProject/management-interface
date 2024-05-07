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

package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	goapi "github.com/TheCacophonyProject/go-api"
	goconfig "github.com/TheCacophonyProject/go-config"
	"github.com/TheCacophonyProject/lepton3"
	signalstrength "github.com/TheCacophonyProject/management-interface/signal-strength"
	saltrequester "github.com/TheCacophonyProject/salt-updater"
	"github.com/godbus/dbus"
	"github.com/gorilla/mux"

	"github.com/TheCacophonyProject/event-reporter/eventclient"
	"github.com/TheCacophonyProject/trap-controller/trapdbusclient"
)

const (
	cptvGlob            = "*.cptv"
	failedUploadsFolder = "failed-uploads"
	rebootDelay         = time.Second * 5
	apiVersion          = 9
)

type ManagementAPI struct {
	config       *goconfig.Config
	router       *mux.Router
	hotspotTimer *time.Ticker
	cptvDir      string
	appVersion   string
}

func NewAPI(router *mux.Router, config *goconfig.Config, appVersion string) (*ManagementAPI, error) {
	thermalRecorder := goconfig.DefaultThermalRecorder()
	if err := config.Unmarshal(goconfig.ThermalRecorderKey, &thermalRecorder); err != nil {
		return nil, err
	}

	return &ManagementAPI{
		config:     config,
		router:     router,
		cptvDir:    thermalRecorder.OutputDir,
		appVersion: appVersion,
	}, nil
}

func (s *ManagementAPI) StartHotspotTimer() {
	s.hotspotTimer = time.NewTicker(5 * time.Minute)
	go func() {
		s.router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				s.hotspotTimer.Reset(5 * time.Minute)
				next.ServeHTTP(w, r)
			})
		})
		<-s.hotspotTimer.C
		if err := stopHotspot(); err != nil {
			log.Println("Failed to stop hotspot:", err)
		}
	}()
}

func (s *ManagementAPI) StopHotspotTimer() {
	if s.hotspotTimer != nil {
		s.hotspotTimer.Stop()
	}
}

func (server *ManagementAPI) ManageHotspot() {
	if err := initializeHotspot(); err != nil {
		log.Println("Failed to initialise hotspot:", err)
		if err := stopHotspot(); err != nil {
			log.Println("Failed to stop hotspot:", err)
		}
	} else {
		server.StartHotspotTimer()
	}
}

func (api *ManagementAPI) GetVersion(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"apiVersion": apiVersion,
		"appVersion": api.appVersion,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

func readFile(file string) string {
	// The /etc/salt/minion_id file contains the ID.
	out, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return string(out)
}

// GetDeviceInfo returns information about this device
func (api *ManagementAPI) GetDeviceInfo(w http.ResponseWriter, r *http.Request) {
	var device goconfig.Device
	if err := api.config.Unmarshal(goconfig.DeviceKey, &device); err != nil {
		log.Printf("/device-info failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "failed to read device config\n")
		return
	}

	type deviceInfo struct {
		ServerURL  string `json:"serverURL"`
		Groupname  string `json:"groupname"`
		Devicename string `json:"devicename"`
		DeviceID   int    `json:"deviceID"`
		SaltID     string `json:"saltID"`
		Type       string `json:"type"`
	}
	info := deviceInfo{
		ServerURL:  device.Server,
		Groupname:  device.Group,
		Devicename: device.Name,
		DeviceID:   device.ID,
		SaltID:     strings.TrimSpace(readFile("/etc/salt/minion_id")),
		Type:       getDeviceType(),
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}

// GetRecordings returns a list of cptv files in a array.
func (api *ManagementAPI) GetRecordings(w http.ResponseWriter, r *http.Request) {
	log.Println("get recordings")
	names := getCptvNames(api.cptvDir)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(names)
}

func (api *ManagementAPI) GetSignalStrength(w http.ResponseWriter, r *http.Request) {
	sig, err := signalstrength.Run()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "failed to connect to modem\n")
		return
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, strconv.Itoa(sig))
}

// GetRecording downloads a cptv file
func (api *ManagementAPI) GetRecording(w http.ResponseWriter, r *http.Request) {
	cptvName := mux.Vars(r)["id"]
	log.Printf("get recording '%s'", cptvName)
	cptvPath := getRecordingPath(cptvName, api.cptvDir)
	if cptvPath == "" {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "file not found\n")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, cptvName))

	ext := filepath.Ext(cptvName)
	if ext == ".cptv" {
		w.Header().Set("Content-Type", "application/x-cptv")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	f, err := os.Open(cptvPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer f.Close()
	w.WriteHeader(http.StatusOK)
	io.Copy(w, bufio.NewReader(f))
}

// DeleteRecording deletes the given cptv file
func (api *ManagementAPI) DeleteRecording(w http.ResponseWriter, r *http.Request) {
	cptvName := mux.Vars(r)["id"]
	recPath := getRecordingPath(cptvName, api.cptvDir)
	if recPath == "" {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "cptv file not found\n")
		return
	}

	metaFile := strings.TrimSuffix(recPath, filepath.Ext(recPath)) + ".txt"
	if _, err := os.Stat(metaFile); !os.IsNotExist(err) {
		log.Printf("deleting meta '%s'", metaFile)
		os.Remove(metaFile)
	}
	log.Printf("delete cptv '%s'", recPath)
	err := os.Remove(recPath)
	if os.IsNotExist(err) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "cptv file not found\n")
		return
	} else if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "failed to delete file")
		return
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "cptv file deleted")
}

// TakeSnapshot will request a new snapshot to be taken by thermal-recorder
func (api *ManagementAPI) TakeSnapshot(w http.ResponseWriter, r *http.Request) {
	conn, err := dbus.SystemBus()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	recorder := conn.Object("org.cacophony.thermalrecorder", "/org/cacophony/thermalrecorder")
	err = recorder.Call("org.cacophony.thermalrecorder.TakeSnapshot", 0).Err
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// TakeSnapshotRecording will request a new snapshot recording to be taken by thermal-recorder
func (api *ManagementAPI) TakeSnapshotRecording(w http.ResponseWriter, r *http.Request) {
	conn, err := dbus.SystemBus()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	recorder := conn.Object("org.cacophony.thermalrecorder", "/org/cacophony/thermalrecorder")
	err = recorder.Call("org.cacophony.thermalrecorder.TakeTestRecording", 0).Err
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// Reregister can change the devices name and group
func (api *ManagementAPI) Reregister(w http.ResponseWriter, r *http.Request) {
	group := r.FormValue("group")
	name := r.FormValue("name")
	if group == "" && name == "" {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "must set name or group\n")
		return
	}
	apiClient, err := goapi.New()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, fmt.Sprintf("failed to get api client for device: %s", err.Error()))
		return
	}
	if group == "" {
		group = apiClient.GroupName()
	}
	if name == "" {
		name = apiClient.DeviceName()
	}

	log.Printf("renaming with name: '%s' group: '%s'", name, group)
	if err := apiClient.Reregister(name, group, randString(20)); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (api *ManagementAPI) ReregisterAuthorized(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Group          string `json:"newGroup"`
		Name           string `json:"newName"`
		AuthorizedUser string `json:"authorizedUser"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(&w, err)
		return
	}

	if req.Group == "" && req.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "must set name or group\n")
		return
	}
	apiClient, err := goapi.New()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, fmt.Sprintf("failed to get api client for device: %s", err.Error()))
		return
	}
	if req.Group == "" {
		req.Group = apiClient.GroupName()
	}
	if req.Name == "" {
		req.Name = apiClient.DeviceName()
	}

	if err := apiClient.ReRegisterByAuthorized(req.Name, req.Group, randString(20), req.AuthorizedUser); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, err.Error())
		return
	}

	if err := api.config.Reload(); err != nil {
		serverError(&w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Reboot will reboot the device after a delay so a response can be sent back
func (api *ManagementAPI) Reboot(w http.ResponseWriter, r *http.Request) {
	go func() {
		log.Printf("device rebooting in %s seconds", rebootDelay)
		time.Sleep(rebootDelay)
		log.Println("rebooting")
		log.Println(exec.Command("/sbin/reboot").Run())
	}()
	w.WriteHeader(http.StatusOK)
}

// SetConfig is a way of writing new config to the device. It can only update one section at a time
func (api *ManagementAPI) SetConfig(w http.ResponseWriter, r *http.Request) {
	section := r.FormValue("section")
	newConfigRaw := r.FormValue("config")
	newConfig := map[string]interface{}{}
	if err := json.Unmarshal([]byte(newConfigRaw), &newConfig); err != nil {
		badRequest(&w, err)
		return
	}
	if err := api.config.SetFromMap(section, newConfig, false); err != nil {
		badRequest(&w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// GetConfig will return the config settings and the defaults
func (api *ManagementAPI) GetConfig(w http.ResponseWriter, r *http.Request) {
	if err := api.config.Reload(); err != nil {
		serverError(&w, err)
		return
	}

	configDefaults := map[string]interface{}{
		goconfig.AudioKey:            goconfig.DefaultAudio(),
		goconfig.GPIOKey:             goconfig.DefaultGPIO(),
		goconfig.LeptonKey:           goconfig.DefaultLepton(),
		goconfig.ModemdKey:           goconfig.DefaultModemd(),
		goconfig.PortsKey:            goconfig.DefaultPorts(),
		goconfig.TestHostsKey:        goconfig.DefaultTestHosts(),
		goconfig.ThermalMotionKey:    goconfig.DefaultThermalMotion(lepton3.Model35), // TODO don't assume that model 3.5 is being used
		goconfig.ThermalRecorderKey:  goconfig.DefaultThermalRecorder(),
		goconfig.ThermalThrottlerKey: goconfig.DefaultThermalThrottler(),
		goconfig.WindowsKey:          goconfig.DefaultWindows(),
	}

	configValues := map[string]interface{}{
		goconfig.AudioKey:            &goconfig.Audio{},
		goconfig.GPIOKey:             &goconfig.GPIO{},
		goconfig.LeptonKey:           &goconfig.Lepton{},
		goconfig.ModemdKey:           &goconfig.Modemd{},
		goconfig.PortsKey:            &goconfig.Ports{},
		goconfig.TestHostsKey:        &goconfig.TestHosts{},
		goconfig.ThermalMotionKey:    &goconfig.ThermalMotion{},
		goconfig.ThermalRecorderKey:  &goconfig.ThermalRecorder{},
		goconfig.ThermalThrottlerKey: &goconfig.ThermalThrottler{},
		goconfig.WindowsKey:          &goconfig.Windows{},
	}

	for section, sectionStruct := range configValues {
		if err := api.config.Unmarshal(section, sectionStruct); err != nil {
			serverError(&w, err)
			return
		}
	}

	valuesAndDefaults := map[string]interface{}{
		"values":   configValues,
		"defaults": configDefaults,
	}

	jsonString, err := json.Marshal(valuesAndDefaults)
	if err != nil {
		serverError(&w, err)
		return
	}
	w.Write(jsonString)
}

// ClearConfigSection will delete the config from a section so the default values will be used.
func (api *ManagementAPI) ClearConfigSection(w http.ResponseWriter, r *http.Request) {
	section := r.FormValue("section")
	log.Printf("clearing config section %s", section)

	if err := api.config.Unset(section); err != nil {
		serverError(&w, err)
	}
}

func (api *ManagementAPI) GetLocation(w http.ResponseWriter, r *http.Request) {
	var location goconfig.Location
	if err := api.config.Unmarshal(goconfig.LocationKey, &location); err != nil {
		serverError(&w, err)
		return
	}
	type Location struct {
		Latitude  float32 `json:"latitude"`
		Longitude float32 `json:"longitude"`
		Altitude  float32 `json:"altitude"`
		Accuracy  float32 `json:"accuracy"`
		Timestamp string  `json:"timestamp"`
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Location{
		Latitude:  location.Latitude,
		Longitude: location.Longitude,
		Altitude:  location.Altitude,
		Accuracy:  location.Accuracy,
		Timestamp: location.Timestamp.UTC().Format(time.RFC3339),
	})
}

// SetLocation is for specifically writing to location setting.
func (api *ManagementAPI) SetLocation(w http.ResponseWriter, r *http.Request) {
	log.Println("update location")
	latitude, err := strconv.ParseFloat(r.FormValue("latitude"), 32)
	if err != nil {
		badRequest(&w, err)
		return
	}
	longitude, err := strconv.ParseFloat(r.FormValue("longitude"), 32)
	if err != nil {
		badRequest(&w, err)
		return
	}
	altitude, err := strconv.ParseFloat(r.FormValue("altitude"), 32)
	if err != nil {
		badRequest(&w, err)
		return
	}
	accuracy, err := strconv.ParseFloat(r.FormValue("accuracy"), 32)
	if err != nil {
		badRequest(&w, err)
		return
	}

	timeMillis, err := strconv.ParseInt(r.FormValue("timestamp"), 10, 64)
	if err != nil {
		badRequest(&w, err)
		return
	}

	location := goconfig.Location{
		Latitude:  float32(latitude),
		Longitude: float32(longitude),
		Accuracy:  float32(accuracy),
		Altitude:  float32(altitude),
		Timestamp: time.Unix(timeMillis/1000, 0),
	}

	if err := api.config.Set(goconfig.LocationKey, &location); err != nil {
		serverError(&w, err)
	}
}

func badRequest(w *http.ResponseWriter, err error) {
	(*w).WriteHeader(http.StatusBadRequest)
	io.WriteString(*w, err.Error())
}

func serverError(w *http.ResponseWriter, err error) {
	log.Printf("server error: %v", err)
	(*w).WriteHeader(http.StatusInternalServerError)
}

func getCptvNames(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, cptvGlob))
	failedUploadMatches, _ := filepath.Glob(filepath.Join(dir, failedUploadsFolder, cptvGlob))
	matches = append(matches, failedUploadMatches...)
	names := make([]string, len(matches))
	for i, filename := range matches {
		names[i] = filepath.Base(filename)
	}
	return names
}

func getRecordingPath(file, dir string) string {
	// Check that given file is a cptv file on the device.
	paths := []string{
		filepath.Join(dir, file),
		filepath.Join(dir, failedUploadsFolder, file),
	}
	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			return path
		}
	}
	return ""
}

// GetEventKeys will return an array of the event keys on the device
func (api *ManagementAPI) GetEventKeys(w http.ResponseWriter, r *http.Request) {
	log.Println("getting event keys")
	keys, err := eventclient.GetEventKeys()
	if err != nil {
		serverError(&w, err)
	}
	json.NewEncoder(w).Encode(keys)
}

// GetEvents takes an array of keys ([]uint64) and will return a JSON of the results.
func (api *ManagementAPI) GetEvents(w http.ResponseWriter, r *http.Request) {
	log.Println("getting events")
	keys, err := getListOfEvents(r)
	if err != nil {
		badRequest(&w, err)
		return
	}
	log.Printf("getting %d events", len(keys))
	events := map[uint64]interface{}{}
	for _, key := range keys {
		event, err := eventclient.GetEvent(key)
		if err != nil {
			events[key] = map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("error getting event '%d': %v", key, err),
			}
		} else {
			events[key] = map[string]interface{}{
				"success": true,
				"event":   event,
			}
		}
	}

	json.NewEncoder(w).Encode(events)
}

// DeleteEvent takes an array of event keys ([]uint64) and will delete all given events.
func (api *ManagementAPI) DeleteEvents(w http.ResponseWriter, r *http.Request) {
	log.Println("deleting events")
	keys, err := getListOfEvents(r)
	if err != nil {
		badRequest(&w, err)
		return
	}
	log.Printf("deleting %d events", len(keys))
	for _, key := range keys {
		if err := eventclient.DeleteEvent(key); err != nil {
			serverError(&w, err)
			return
		}
	}
}

// Trigger trap
func (api *ManagementAPI) TriggerTrap(w http.ResponseWriter, r *http.Request) {
	log.Println("triggering trap")

	if err := trapdbusclient.TriggerTrap(map[string]interface{}{"test": true}); err != nil {
		badRequest(&w, err)
		return
	}
}

// CheckSaltConnection will try to ping the salt server and return the response
func (api *ManagementAPI) CheckSaltConnection(w http.ResponseWriter, r *http.Request) {
	log.Println("pinging salt server")
	state, err := saltrequester.RunPingSync()
	if err != nil {
		log.Printf("error running salt sync ping: %v", err)
		serverError(&w, errors.New("failed to make ping call to salt server"))
		return
	}
	json.NewEncoder(w).Encode(state)
}

// StartSaltUpdate will start a salt update process if not already running
func (api *ManagementAPI) StartSaltUpdate(w http.ResponseWriter, r *http.Request) {
	state, err := saltrequester.State()
	if err != nil {
		serverError(&w, errors.New("failed to check salt state"))
	}
	if state.RunningUpdate {
		w.Write([]byte("already runing salt update"))
		return
	}
	if err := saltrequester.RunUpdate(); err != nil {
		log.Printf("error calling a salt update: %v", err)
		serverError(&w, errors.New("failed to call a salt update"))
	}
}

// GetSaltUpdateState will get the salt update status
func (api *ManagementAPI) GetSaltUpdateState(w http.ResponseWriter, r *http.Request) {
	state, err := saltrequester.State()
	if err != nil {
		log.Printf("error getting salt state: %v", err)
		serverError(&w, errors.New("failed to get salt state"))
		return
	}
	json.NewEncoder(w).Encode(state)
}

// GetSaltAutoUpdate will check if salt auto update is enabled
func (api *ManagementAPI) GetSaltAutoUpdate(w http.ResponseWriter, r *http.Request) {
	autoUpdate, err := saltrequester.IsAutoUpdateOn()
	if err != nil {
		log.Printf("error getting salt auto update state: %v", err)
		serverError(&w, errors.New("failed to get salt auto update state"))
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"autoUpdate": autoUpdate})
}

// PostSaltAutoUpdate will set if auto update is enabled or not
func (api *ManagementAPI) PostSaltAutoUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		parseFormErrorResponse(&w, err)
		return
	}
	autoUpdateStr := strings.ToLower(r.Form.Get("autoUpdate"))
	if autoUpdateStr != "true" && autoUpdateStr != "false" {
		parseFormErrorResponse(&w, errors.New("invalid value for autoUpdate"))
		return
	}
	autoUpdate := strings.ToLower(autoUpdateStr) == "true"
	if err := saltrequester.SetAutoUpdate(autoUpdate); err != nil {
		log.Printf("error setting auto update: %v", err)
		serverError(&w, errors.New("failed to set auto update"))
	}
}

func (api *ManagementAPI) GetServiceLogs(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		parseFormErrorResponse(&w, err)
		return
	}
	service := r.Form.Get("service")
	if service == "" {
		parseFormErrorResponse(&w, errors.New("service field was empty"))
		return
	}
	lines, err := parseIntFromForm("lines", r.Form)
	if err != nil {
		parseFormErrorResponse(&w, err)
		return
	}
	logs, err := getServiceLogs(service, lines)
	if err != nil {
		serverError(&w, err)
		return
	}
	json.NewEncoder(w).Encode(logs)
}

func (api *ManagementAPI) GetServiceStatus(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		parseFormErrorResponse(&w, err)
		return
	}
	service := r.Form.Get("service")
	if service == "" {
		parseFormErrorResponse(&w, errors.New("service field was empty"))
		return
	}
	serviceStatus, err := getServiceStatus(service)
	if err != nil {
		serverError(&w, err)
		return
	}
	json.NewEncoder(w).Encode(serviceStatus)
}

// Network API
func (api *ManagementAPI) GetNetworkInterfaces(w http.ResponseWriter, r *http.Request) {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Error getting network interfaces: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "failed to get network interfaces\n")
		return
	}

	var result []map[string]interface{}
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			log.Printf("Error getting addresses for interface %s: %v", iface.Name, err)
			continue // Skip this interface
		}

		var addrStrings []string
		for _, addr := range addrs {
			addrStrings = append(addrStrings, addr.String())
		}

		result = append(result, map[string]interface{}{
			"name":       iface.Name,
			"addresses":  addrStrings,
			"mtu":        iface.MTU,
			"macAddress": iface.HardwareAddr.String(),
			"flags":      iface.Flags.String(),
		})
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func getCurrentWifiNetwork() (string, error) {
	cmd := exec.Command("iwgetid", "wlan0", "-r")
	output, err := cmd.Output()
	if err != nil {
		if len(output) == 0 {
			return "", nil
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// CheckInternetConnection checks if a specified network interface has internet access
func CheckInternetConnection(interfaceName string) bool {
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		fmt.Println("Error getting interface details:", err)
		return false
	}
	args := []string{"-I", iface.Name, "-c", "3", "-n", "-W", "15", "1.1.1.1"}

	if err := exec.Command("ping", args...).Run(); err != nil {
		fmt.Println("Error pinging:", err)
		return false
	}
	return true
}

func (api *ManagementAPI) CheckModemInternetConnection(w http.ResponseWriter, r *http.Request) {
	// Check if connected to modem
	log.Println("Checking modem connection")
	connected := CheckInternetConnection("eth1")
	log.Printf("Modem connection: %v", connected)

	// Send the current network as a JSON response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"connected": connected})
}

// Check Wifi Connection
func (api *ManagementAPI) CheckWifiInternetConnection(w http.ResponseWriter, r *http.Request) {
	// Check if connected to Wi-Fi
	log.Println("Checking Wi-Fi connection")
	connected := CheckInternetConnection("wlan0")
	log.Printf("Wi-Fi connection: %v", connected)

	// Send the current network as a JSON response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"connected": connected})
}

func (api *ManagementAPI) GetCurrentWifiNetwork(w http.ResponseWriter, r *http.Request) {
	// Get the current Wi-Fi network
	log.Println("Getting current Wi-Fi network")
	currentNetwork, err := getCurrentWifiNetwork()
	if err != nil {
		log.Printf("Error getting current Wi-Fi network: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "failed to get current Wi-Fi network\n")
		return
	}
	log.Printf("Current Wi-Fi network: %s", currentNetwork)

	// Send the current network as a JSON response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"SSID": currentNetwork})
}

func (api *ManagementAPI) ConnectToWifi(w http.ResponseWriter, r *http.Request) {
	var wifiDetails struct {
		SSID     string `json:"ssid"`
		Password string `json:"password"`
	}

	// Decode the JSON body
	if err := json.NewDecoder(r.Body).Decode(&wifiDetails); err != nil {
		log.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Attempting to connect to Wi-Fi SSID: %s", wifiDetails.SSID)

	if savedId, err := checkIsSavedNetwork(wifiDetails.SSID); err != nil {
		log.Printf("Error checking if Wi-Fi network is saved: %v", err)
		if err := connectToWifi(wifiDetails.SSID, wifiDetails.Password); err != nil {
			log.Printf("Error connecting to Wi-Fi: %v", err)
			http.Error(w, "Failed to connect to Wi-Fi: "+err.Error(), http.StatusInternalServerError)
			go api.ManageHotspot()
			return
		}
	} else {
		log.Printf("Wi-Fi network is saved: %s", wifiDetails.SSID)
		if err := exec.Command("wpa_cli", "-i", "wlan0", "select_network", savedId).Run(); err != nil {
			log.Printf("Error enabling Wi-Fi network: %v", err)
			http.Error(w, "Failed to connect to Wi-Fi: "+err.Error(), http.StatusInternalServerError)
			go api.ManageHotspot()
			return
		}
		if err := exec.Command("wpa_cli", "-i", "wlan0", "reassociate").Run(); err != nil {
			log.Printf("Error reassociating Wi-Fi network: %v", err)
			http.Error(w, "Failed to connect to Wi-Fi: "+err.Error(), http.StatusInternalServerError)
			go api.ManageHotspot()
			return
		}
		if err := EnableAllSavedNetworks(); err != nil {
			log.Printf("Error enabling all saved networks: %v", err)
		}
	}
	if err := stopHotspot(); err != nil {
		log.Println("Failed to stop hotspot:", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if connected := streamWifiState(ctx, wifiDetails.SSID); !connected {
		if err := removeNetworkFromWPA(wifiDetails.SSID); err != nil {
			log.Printf("Error removing Wi-Fi network: %v", err)
		}
		if err := EnableAllSavedNetworks(); err != nil {
			log.Printf("Error enabling all saved networks: %v", err)
		}
		// Reconfigure the Wi-Fi
		if err := exec.Command("wpa_cli", "-i", "wlan0", "reconfigure").Run(); err != nil {
			log.Printf("Error reconfiguring Wi-Fi: %v", err)
		}

		log.Printf("Failed to connect to Wi-Fi SSID: %s", wifiDetails.SSID)
		http.Error(w, "Failed to connect to Wi-Fi: timed out", http.StatusInternalServerError)
		go api.ManageHotspot()
		return
	} else {
		// restart dhcp
		log.Printf("Successfully connected to Wi-Fi SSID: %s", wifiDetails.SSID)
		log.Println("Connected to Wi-Fi successfully")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Connected to Wi-Fi successfully"))
	}
}

func EnableAllSavedNetworks() error {
	ssidIds, err := getSSIDIds()
	if err != nil {
		return err
	}
	for _, id := range ssidIds {
		if err := exec.Command("wpa_cli", "-i", "wlan0", "enable_network", id).Run(); err != nil {
			return err
		}
	}
	return nil
}

func streamWifiState(ctx context.Context, ssid string) bool {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check the Wi-Fi connection status
			statusCmd := exec.Command("wpa_cli", "-i", "wlan0", "status")
			statusOut, err := statusCmd.Output()
			if err != nil {
				log.Printf("Error checking Wi-Fi status: %v", err)
				continue
			}

			// Parse the status output
			status := strings.TrimSpace(string(statusOut))
			if strings.Contains(status, "ssid="+ssid) && strings.Contains(status, "wpa_state=COMPLETED") {
				// Connected to the specified SSID
				return true
			}
		case <-ctx.Done():
			// Timeout
			return false
		}
	}
}

func saveConfig() error {
	// Enable all saved networks
	ssidIds, err := getSSIDIds()
	if err != nil {
		return err
	}

	for _, id := range ssidIds {
		if err := exec.Command("wpa_cli", "-i", "wlan0", "enable_network", id).Run(); err != nil {
			return err
		}
	}

	// Save the config
	if err := exec.Command("wpa_cli", "-i", "wlan0", "save_config").Run(); err != nil {
		return err
	}

	return nil
}

func checkIsSavedNetwork(ssid string) (string, error) {
	ssidIds, err := getSSIDIds()
	if err != nil {
		return "", err
	}
	for ssidId, ssidName := range ssidIds {
		if ssidName == ssid {
			return ssidId, nil
		}
	}
	return "", errors.New("ssid not found")
}

// connectToWifi connects to a Wi-Fi network using wpa_cli
func connectToWifi(ssid string, passkey string) error {
	output, err := exec.Command("wpa_cli", "-i", "wlan0", "add_network").Output()
	if err != nil {
		log.Printf("Error adding Wi-Fi network to config: %v", err)
		return err
	}

	id := strings.TrimSpace(string(output))

	if err := exec.Command("wpa_cli", "-i", "wlan0", "set_network", id, "ssid", "\""+ssid+"\"").Run(); err != nil {
		log.Printf("Error adding Wi-Fi network to config: %v", err)
		return err
	}

	if err := exec.Command("wpa_cli", "-i", "wlan0", "set_network", id, "psk", "\""+passkey+"\"").Run(); err != nil {
		log.Printf("Error adding Wi-Fi network to config: %v", err)
		return err
	}

	// Reload the wpa_supplicant config
	if err := exec.Command("wpa_cli", "-i", "wlan0", "save_config").Run(); err != nil {
		log.Printf("Error reconfiguring Wi-Fi: %v", err)
		// Remove the network from the config
		if err := removeNetworkFromWPA(ssid); err != nil {
			log.Printf("Error removing Wi-Fi network: %v", err)
			return err
		}
		return err
	}

	if err := exec.Command("wpa_cli", "-i", "wlan0", "select_network", id).Run(); err != nil {
		log.Printf("Error enabling Wi-Fi network: %v", err)
		return err
	}

	// Reassociate the Wi-Fi
	if err := exec.Command("wpa_cli", "-i", "wlan0", "reassociate").Run(); err != nil {
		log.Printf("Error reassociating Wi-Fi: %v", err)
		return err
	}

	if err := EnableAllSavedNetworks(); err != nil {
		log.Printf("Error enabling all saved networks: %v", err)
	}

	return nil
}

func getSSIDIds() (map[string]string, error) {
	// Get the list of Wi-Fi networks
	cmd := exec.Command("wpa_cli", "-i", "wlan0", "list_networks")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse the output
	ssidIds := make(map[string]string)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		ssidIds[fields[1]] = fields[0]
	}

	return ssidIds, nil
}

func removeNetworkFromWPA(ssid string) error {
	if ssid == "" || ssid == "bushnet" || ssid == "Bushnet" {
		return nil
	}
	ssidIds, err := getSSIDIds()
	if err != nil {
		return err
	}

	id, ok := ssidIds[ssid]
	if !ok {
		return nil // Network not found
	}

	// Remove the network from the config
	if err := exec.Command("wpa_cli", "-i", "wlan0", "remove_network", id).Run(); err != nil {
		return err
	}

	// Save the config
	if err := saveConfig(); err != nil {
		return err
	}

	return nil
}

func (api *ManagementAPI) GetWifiNetworks(w http.ResponseWriter, r *http.Request) {
	// Execute the command to scan for Wi-Fi networks
	log.Println("Scanning for Wi-Fi networks")
	var output []byte
	var err error
	maxRetries := 3
	retryInterval := time.Second * 2

	for i := 0; i < maxRetries; i++ {
		cmd := exec.Command("iwlist", "wlan0", "scan")
		output, err = cmd.Output()
		if err == nil {
			break
		}
		log.Printf("Error scanning for Wi-Fi networks: %v", err)
		time.Sleep(retryInterval)
	}

	if err != nil {
		log.Printf("Failed to scan for Wi-Fi networks after %d retries", maxRetries)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "failed to scan for Wi-Fi networks\n")
		return
	}

	// Parse the output to extract network information
	networks := parseWiFiScanOutput(string(output))
	log.Printf("Found %d Wi-Fi networks", len(networks))
	// Send the list of networks as a JSON response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(networks)
}

func parseWiFiScanOutput(output string) []map[string]string {
	var networks []map[string]string
	var currentNetwork map[string]string

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Cell") {
			if currentNetwork != nil {
				networks = append(networks, currentNetwork)
			}
			currentNetwork = make(map[string]string)
		} else if strings.Contains(line, "ESSID:") {
			ssid := strings.Split(line, "\"")[1]
			currentNetwork["SSID"] = ssid
		} else if strings.Contains(line, "Quality=") {
			quality := regexp.MustCompile("Quality=([0-9]+/[0-9]+)").FindStringSubmatch(line)[1]
			signalLevel := regexp.MustCompile("Signal level=(-?[0-9]+ dBm)").FindStringSubmatch(line)[1]
			currentNetwork["Quality"] = quality
			currentNetwork["Signal Level"] = signalLevel
		} else if strings.Contains(line, "Encryption key:on") {
			currentNetwork["Security"] = "On"
		} else if strings.Contains(line, "IE: IEEE 802.11i/WPA2") {
			currentNetwork["Security"] = "WPA2"
		} else if strings.Contains(line, "IE: WPA Version 1") {
			currentNetwork["Security"] = "WPA"
		} else if strings.Contains(line, "IE: Unknown") && currentNetwork["Security"] == "" {
			// Default to 'Unknown' if no other security information is found
			currentNetwork["Security"] = "Unknown"
		}
	}

	// Append the last network if not empty
	if currentNetwork != nil {
		networks = append(networks, currentNetwork)
	}

	return networks
}

func (api *ManagementAPI) DisconnectFromWifi(w http.ResponseWriter, r *http.Request) {
	// Respond to client before actually disconnecting
	log.Printf("Will disconnect from Wi-Fi network")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "will disconnect from Wi-Fi network shortly\n")

	go func() {
		if err := exec.Command("wpa_cli", "-i", "wlan0", "disconnect").Run(); err != nil {
			log.Printf("Error disconnecting from Wi-Fi network: %v", err)
			// Handle the error
		}

		if err := restartDHCP(); err != nil {
			log.Printf("Error restarting DHCP client: %v", err)
		}
		// if no current network, then restart hotspot
		currentSSID, err := checkIsConnectedToNetwork()
		if err != nil {
			log.Printf("Error getting current Wi-Fi network: %v", err)
		}
		if currentSSID == "" {
			log.Printf("No current Wi-Fi network, restarting hotspot")
			go api.ManageHotspot()
		}
	}()
}

func (api *ManagementAPI) ForgetWifiNetwork(w http.ResponseWriter, r *http.Request) {
	// Parse request for SSID
	var wifiDetails struct {
		SSID string `json:"ssid"`
	}

	currentSSID, err := checkIsConnectedToNetwork()
	if err != nil {
		log.Printf("Error getting current Wi-Fi network: %v", err)
	}

	// Decode the JSON body
	if err := json.NewDecoder(r.Body).Decode(&wifiDetails); err != nil {
		log.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if currentSSID == wifiDetails.SSID {
		log.Printf("Will forget Wi-Fi network: %s", wifiDetails.SSID)
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "will forget Wi-Fi network shortly\n")
		// Forget the network
		go func() {
			if err := removeNetworkFromWPA(wifiDetails.SSID); err != nil {
				log.Printf("Error removing Wi-Fi network: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, "failed to remove Wi-Fi network\n")
			}
			if err := restartDHCP(); err != nil {
				log.Printf("Error restarting DHCP client: %v", err)
			}
			// if no current network, then restart hotspot
			currentSSID, err := checkIsConnectedToNetwork()
			if err != nil {
				log.Printf("Error getting current Wi-Fi network: %v", err)
			}
			if currentSSID == "" {
				log.Printf("No current Wi-Fi network, restarting hotspot")
				go api.ManageHotspot()
			}
		}()
	} else {
		ssids, err := getSSIDIds()
		if err != nil {
			log.Printf("Error getting Wi-Fi networks: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "failed to get Wi-Fi networks\n")
			return
		}

		// Find the network ID
		id, ok := ssids[wifiDetails.SSID]
		if !ok {
			log.Printf("Wi-Fi network not found: %s", wifiDetails.SSID)
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, "Wi-Fi network not found\n")
			return
		}

		// Remove the network using wpa_cli
		cmd := exec.Command("wpa_cli", "-i", "wlan0", "remove_network", id)
		if err := cmd.Run(); err != nil {
			log.Printf("Error removing Wi-Fi network: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "failed to remove Wi-Fi network\n")
			return
		}

		// Save the changes
		if err := saveConfig(); err != nil {
			log.Printf("Error reloading Wi-Fi config: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "failed to reload Wi-Fi config\n")
			return
		}

		log.Printf("Successfully removed Wi-Fi network: %s", wifiDetails.SSID)
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "successfully removed Wi-Fi network\n")
	}
}

func (api *ManagementAPI) GetModem(w http.ResponseWriter, r *http.Request) {
	// Send dbus call to modem service to get all modem statuses
	conn, err := dbus.SystemBus()
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to connect to DBus", http.StatusInternalServerError)
		return
	}

	modemDbus := conn.Object("org.cacophony.modemd", "/org/cacophony/modemd")

	var status map[string]interface{}
	err = modemDbus.Call("org.cacophony.modemd.GetStatus", 0).Store(&status)
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to get modem status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(status); err != nil {
		http.Error(w, "Failed to encode status to JSON", http.StatusInternalServerError)
	}
}

func getDeviceType() string {
	data, err := os.ReadFile("/etc/salt/minion_id")
	if err != nil {
		log.Println(err)
		return ""
	}

	parts := strings.Split(string(data), "-")
	if len(parts) < 2 {
		log.Printf("Failed to parse '%s' device type from /etc/salt/minion_id", string(data))
		return ""
	}

	return strings.Join(parts[:len(parts)-1], "-")
}

func (api *ManagementAPI) RestartService(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		parseFormErrorResponse(&w, err)
		return
	}
	service := r.Form.Get("service")
	if service == "" {
		parseFormErrorResponse(&w, errors.New("service field was empty"))
		return
	}
	_, err := exec.Command("systemctl", "restart", service).Output()
	if err != nil {
		serverError(&w, err)
		return
	}
}

type serviceStatus struct {
	Enabled  bool
	Active   bool
	Duration int
}

func getServiceStatus(service string) (*serviceStatus, error) {
	status := &serviceStatus{}
	enabledOut, _ := exec.Command("systemctl", "is-enabled", service).Output()
	status.Enabled = strings.TrimSpace(string(enabledOut)) == "enabled"
	activeOut, _ := exec.Command("systemctl", "is-active", service).Output()
	status.Active = strings.TrimSpace(string(activeOut)) == "active"
	if !status.Active {
		return status, nil
	}

	pidofOut, err := exec.Command("pidof", service).Output()
	if err != nil {
		return nil, err
	}
	pidOfStr := strings.TrimSpace(string(pidofOut))
	_, err = strconv.Atoi(pidOfStr)
	if err != nil {
		return nil, err
	}
	eTimeOut, err := exec.Command("ps", "-p", pidOfStr, "-o", "etimes").Output()
	if err != nil {
		return nil, err
	}
	eTimeStr := strings.TrimSpace(string(eTimeOut))
	eTimeStr = strings.TrimPrefix(eTimeStr, "ELAPSED")
	eTimeStr = strings.TrimSpace(eTimeStr)
	eTime, err := strconv.Atoi(eTimeStr)
	if err != nil {
		return nil, err
	}
	status.Duration = eTime
	return status, nil
}

func getListOfEvents(r *http.Request) ([]uint64, error) {
	r.ParseForm()
	keysStr := r.Form.Get("keys")
	var keys []uint64
	if err := json.Unmarshal([]byte(keysStr), &keys); err != nil {
		return nil, fmt.Errorf("failed to parse keys '%s' as a list of uint64: %v", keysStr, err)
	}
	return keys, nil
}

func parseIntFromForm(field string, form url.Values) (int, error) {
	intStr := form.Get(field)
	if intStr == "" {
		return 0, fmt.Errorf("didn't find a value for '%s'", field)
	}
	i, err := strconv.Atoi(intStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse '%s' from field '%s' to an int: %v", intStr, field, err)
	}
	return i, nil
}

func parseFormErrorResponse(w *http.ResponseWriter, err error) {
	(*w).WriteHeader(http.StatusBadRequest)
	io.WriteString((*w), fmt.Sprintf("failed to parse form: %v", err))
}

func getServiceLogs(service string, lines int) ([]string, error) {
	out, err := exec.Command(
		"/bin/journalctl",
		"-u", service,
		"--no-pager",
		"-n", fmt.Sprint(lines)).Output()
	if err != nil {
		return nil, err
	}
	logLines := strings.Split(string(out), "\n")
	if logLines[len(logLines)-1] == "" { // sometimes the last line is an empty string
		return logLines[:len(logLines)-1], nil
	}
	return logLines, nil
}

func (api *ManagementAPI) SaveWifiNetwork(w http.ResponseWriter, r *http.Request) {
	var wifiDetails struct {
		SSID     string `json:"ssid"`
		Password string `json:"password"`
	}

	// Decode the JSON body
	if err := json.NewDecoder(r.Body).Decode(&wifiDetails); err != nil {
		log.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "will save Wi-Fi network shortly\n")

	go func() {
		// Save the Wi-Fi network using wpa_cli
		if err := saveWifiNetwork(wifiDetails.SSID, wifiDetails.Password); err != nil {
			log.Printf("Error saving Wi-Fi network: %v", err)
			http.Error(w, "Failed to save Wi-Fi network: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}()
}

func saveWifiNetwork(ssid string, passkey string) error {
	// add using wpa_cli
	if output, err := exec.Command("wpa_cli", "-i", "wlan0", "add_network").Output(); err != nil {
		log.Printf("Error adding Wi-Fi network: %v, %s", err, output)
		return err
	}
	if output, err := exec.Command("wpa_cli", "-i", "wlan0", "set_network", "0", "ssid", "\""+ssid+"\"").Output(); err != nil {
		log.Printf("Error setting Wi-Fi network SSID: %v, %s", err, output)
		return err
	}
	if output, err := exec.Command("wpa_cli", "-i", "wlan0", "set_network", "0", "psk", "\""+passkey+"\"").Output(); err != nil {
		log.Printf("Error setting Wi-Fi network PSK: %v, %s", err, output)
		return err
	}

	// Save the config
	if err := saveConfig(); err != nil {
		log.Printf("Error saving Wi-Fi network: %v", err)
		return err
	}

	return nil
}

func parseSavedWiFiNetworks(output string) []string {
	var networks []string
	lines := strings.Split(output, "\n")
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		networks = append(networks, fields[1])
	}
	return networks
}

// Return array of saved wifi networks
func getSavedWifiNetworks() ([]string, error) {
	// Get the list of Wi-Fi networks
	cmd := exec.Command("wpa_cli", "-i", "wlan0", "list_networks")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse the output to extract network information
	networks := parseSavedWiFiNetworks(string(output))
	return networks, nil
}

func (api *ManagementAPI) GetSavedWifiNetworks(w http.ResponseWriter, r *http.Request) {
	networks, err := getSavedWifiNetworks()
	if err != nil {
		serverError(&w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(networks)
}

func (api *ManagementAPI) UploadLogs(w http.ResponseWriter, r *http.Request) {
	if err := exec.Command("cp", "/var/log/syslog", "/tmp/syslog").Run(); err != nil {
		log.Printf("Error copying syslog: %v", err)
		serverError(&w, err)
		return
	}

	if err := exec.Command("gzip", "/tmp/syslog", "-f").Run(); err != nil {
		log.Printf("Error compressing syslog: %v", err)
		serverError(&w, err)
		return
	}

	if err := exec.Command("salt-call", "cp.push", "/tmp/syslog.gz").Run(); err != nil {
		log.Printf("Error pushing syslog with salt: %v", err)
		serverError(&w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
