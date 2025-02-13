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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	goapi "github.com/TheCacophonyProject/go-api"
	goconfig "github.com/TheCacophonyProject/go-config"
	"github.com/TheCacophonyProject/go-utils/logging"
	"github.com/TheCacophonyProject/go-utils/saltutil"
	signalstrength "github.com/TheCacophonyProject/management-interface/signal-strength"
	saltrequester "github.com/TheCacophonyProject/salt-updater"
	"github.com/godbus/dbus"
	"github.com/gorilla/mux"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/TheCacophonyProject/event-reporter/eventclient"
	"github.com/TheCacophonyProject/trap-controller/trapdbusclient"

	netmanagerclient "github.com/TheCacophonyProject/rpi-net-manager/netmanagerclient"
)

var log *logging.Logger

const (
	cptvGlob            = "*.cptv"
	aacGlob             = "*.aac"
	failedUploadsFolder = "failed-uploads"
	rebootDelay         = time.Second * 5
	apiVersion          = 8
)

type ManagementAPI struct {
	config       *goconfig.Config
	router       *mux.Router
	hotspotTimer *time.Ticker
	recordingDir string
	appVersion   string
}

func NewAPI(router *mux.Router, config *goconfig.Config, appVersion string, l *logging.Logger) (*ManagementAPI, error) {
	log = l
	thermalRecorder := goconfig.DefaultThermalRecorder()
	if err := config.Unmarshal(goconfig.ThermalRecorderKey, &thermalRecorder); err != nil {
		return nil, err
	}

	return &ManagementAPI{
		config:       config,
		router:       router,
		recordingDir: thermalRecorder.OutputDir,
		appVersion:   appVersion,
	}, nil
}

func (api *ManagementAPI) StopHotspotTimer() {
	if api.hotspotTimer != nil {
		api.hotspotTimer.Stop()
	}
}

func checkIsConnectedToNetworkWithRetries() (string, error) {
	// Try to get the current network name
	var ssid string
	var err error
	for i := 0; i < 5; i++ {
		ssid, err = getCurrentWifiNetwork()
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	return ssid, err
}

func (api *ManagementAPI) ManageHotspot() {
	// Check if we are connected to a network
	ssid, err := checkIsConnectedToNetworkWithRetries()
	if err != nil {
		log.Printf("Error checking if connected to network: %v", err)
	}
	if ssid != "" {
		log.Printf("Connected to network: %s", ssid)
		return
	}

	if err := netmanagerclient.EnableHotspot(true); err != nil {
		log.Println("Failed to initialise hotspot:", err)
		if err := netmanagerclient.EnableWifi(true); err != nil {
			log.Println("Failed to stop hotspot:", err)
		}
		return
	}
	netmanagerclient.KeepHotspotOnFor(60 * 5)
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
		ServerURL   string `json:"serverURL"`
		GroupName   string `json:"groupname"`
		Devicename  string `json:"devicename"`
		DeviceID    int    `json:"deviceID"`
		SaltID      string `json:"saltID"`
		LastUpdated string `json:"lastUpdated"`
		Type        string `json:"type"`
	}
	info := deviceInfo{
		ServerURL:   device.Server,
		GroupName:   device.Group,
		Devicename:  device.Name,
		DeviceID:    device.ID,
		SaltID:      strings.TrimSpace(readFile("/etc/salt/minion_id")),
		LastUpdated: getLastSaltUpdate(),
		Type:        getDeviceType(),
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}

// GetRecordings returns a list of recordings in a array.
func (api *ManagementAPI) GetRecordings(w http.ResponseWriter, r *http.Request) {
	log.Println("get recordings")
	names := getCptvNames(api.recordingDir)
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

// GetRecording downloads a recordings
func (api *ManagementAPI) GetRecording(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["id"]
	log.Printf("get recording '%s'", name)
	recordingPath := getRecordingPath(name, api.recordingDir)
	if recordingPath == "" {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "file not found\n")
		return
	}

	ext := filepath.Ext(name)
	switch ext {
	case ".cptv":
		sendFile(w, recordingPath, name, "application/x-cptv")
	case ".mp3":
		sendFile(w, recordingPath, name, "audio/mp4")
	default:
		sendFile(w, recordingPath, name, "application/json")
	}
}

func sendFile(w http.ResponseWriter, path, name, contentType string) {
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	w.Header().Set("Content-Type", contentType)

	f, err := os.Open(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer f.Close()

	w.WriteHeader(http.StatusOK)
	io.Copy(w, bufio.NewReader(f))
}

// DeleteRecording deletes the given recording file id
func (api *ManagementAPI) DeleteRecording(w http.ResponseWriter, r *http.Request) {
	recordingName := mux.Vars(r)["id"]
	recPath := getRecordingPath(recordingName, api.recordingDir)
	if recPath == "" {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "recording file not found\n")
		return
	}

	metaFile := strings.TrimSuffix(recPath, filepath.Ext(recPath)) + ".txt"
	if _, err := os.Stat(metaFile); !os.IsNotExist(err) {
		log.Printf("deleting meta '%s'", metaFile)
		os.Remove(metaFile)
	}
	log.Printf("delete recording '%s'", recPath)
	err := os.Remove(recPath)
	if os.IsNotExist(err) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "recording file not found\n")
		return
	} else if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "failed to delete file")
		return
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "recording file deleted")
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

// Reregister can change the devices name and group
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
		log.Printf("Error with unmarshal: %s", err)
		badRequest(&w, err)
		return
	}
	if err := api.config.SetFromMap(section, newConfig, false); err != nil {
		log.Printf("Error with SetFromMap: %s", err)
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

	defaultValues := map[string]interface{}{}
	for k, v := range goconfig.GetDefaults() {
		defaultValues[toCamelCase(k)] = v
	}

	values, err := api.config.GetAllValues()
	if err != nil {
		serverError(&w, err)
		return
	}

	configValuesCC := map[string]interface{}{}
	for k, v := range values {
		configValuesCC[toCamelCase(k)] = v
	}
	values = configValuesCC

	valuesAndDefaults := map[string]interface{}{
		"values":   values,
		"defaults": defaultValues,
	}

	jsonString, err := json.Marshal(valuesAndDefaults)
	if err != nil {
		serverError(&w, err)
		return
	}
	w.Write(jsonString)
}

func toCamelCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_'
	})
	c := cases.Title(language.English)
	for i := 1; i < len(words); i++ {
		words[i] = c.String(words[i])
	}
	return strings.Join(words, "")
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

func getAacNames(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, aacGlob))
	failedUploadMatches, _ := filepath.Glob(filepath.Join(dir, failedUploadsFolder, aacGlob))
	matches = append(matches, failedUploadMatches...)
	names := make([]string, len(matches))
	for i, filename := range matches {
		names[i] = filepath.Base(filename)
	}
	return names
}

func getRecordingPath(file, dir string) string {
	// Check that given file is a recording file on the device.
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
	var requestBody struct {
		Force bool `json:"force"`
	}

	// Decode the JSON request body if there is one.
	if r.ContentLength >= 0 {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			serverError(&w, errors.New("failed to parse request body"))
			return
		}
	}

	state, err := saltrequester.State()
	if err != nil {
		serverError(&w, errors.New("failed to check salt state"))
		return
	}

	// Check if the update is already running
	if state.RunningUpdate {
		w.Write([]byte("already running salt update"))
		return
	}

	// Check if we should force the update
	if requestBody.Force {
		err := saltrequester.ForceUpdate()
		if err != nil {
			log.Printf("error forcing salt update: %v", err)
			serverError(&w, errors.New("failed to force salt update"))
			return
		}
		w.Write([]byte("force salt update started"))
		return
	}

	// Run the update, this will only run an update if one is required.
	err = saltrequester.RunUpdate()
	if err != nil {
		log.Printf("error calling a salt update: %v", err)
		serverError(&w, errors.New("failed to call a salt update"))
		return
	}
	w.Write([]byte("salt update started"))
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

type BatteryReading struct {
	Time           string `json:"time"`
	MainBattery    string `json:"mainBattery"`
	MainBatteryLow string `json:"mainBatteryLow"`
	RTCBattery     string `json:"rtcBattery"`
}

func getLastBatteryReading() (BatteryReading, error) {
	file, err := os.Open("/var/log/battery-readings.csv")
	if err != nil {
		return BatteryReading{}, err
	}
	defer file.Close()

	var lastLine string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lastLine = scanner.Text()
	}

	if scanner.Err() != nil {
		return BatteryReading{}, scanner.Err()
	}

	parts := strings.Split(lastLine, ",")
	if len(parts) != 4 {
		return BatteryReading{}, errors.New("unexpected format in battery-readings.csv")
	}

	return BatteryReading{
		Time:           parts[0],
		MainBattery:    parts[1],
		MainBatteryLow: parts[2],
		RTCBattery:     parts[3],
	}, nil
}

func (api *ManagementAPI) GetTestVideos(w http.ResponseWriter, r *http.Request) {
	recordingNames := []string{}
	testRecordingsPath := "/var/spool/cptv/test-recordings"
	_, err := os.Stat(testRecordingsPath)
	if os.IsNotExist(err) {
		http.Error(w, "Directory does not exist", http.StatusNotFound)
		json.NewEncoder(w).Encode(recordingNames)
		return
	}
	recordings, err := os.ReadDir(testRecordingsPath)
	if err != nil {
		serverError(&w, err)
		return
	}

	for _, recording := range recordings {
		if strings.HasSuffix(recording.Name(), ".cptv") {
			recordingNames = append(recordingNames, recording.Name())
		}
	}
	json.NewEncoder(w).Encode(recordingNames)
}

type VideoRequest struct {
	Video string `json:"video"`
}

func (api *ManagementAPI) UploadTestRecording(w http.ResponseWriter, r *http.Request) {
	log.Info("Uploading test recording")
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		parseFormErrorResponse(&w, err)
		return
	}
	file, handler, err := r.FormFile("recording")
	if err != nil {
		serverError(&w, err)
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		serverError(&w, err)
		return
	}

	err = os.MkdirAll("/var/spool/cptv/test-recordings", 0755)
	if err != nil {
		serverError(&w, err)
		return
	}

	err = os.WriteFile("/var/spool/cptv/test-recordings/"+handler.Filename, fileBytes, 0644)
	if err != nil {
		serverError(&w, err)
		return
	}
}

func (api *ManagementAPI) PlayTestVideo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		parseFormErrorResponse(&w, err)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	var req VideoRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		log.Printf("Failed to parse request body: %s, error: %s", bodyBytes, err)
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	videoName := "/var/spool/cptv/test-recordings/" + req.Video
	log.Printf("Playing %s", videoName)

	recorderService := "thermal-recorder-py"
	tc2AgentService := "tc2-agent"
	if err := manageService("stop", recorderService); err != nil {
		serverError(&w, err)
		return
	}
	if err := manageService("stop", tc2AgentService); err != nil {
		serverError(&w, err)
		return
	}

	cmd := exec.Command("/home/pi/.venv/classifier/bin/pi_classify", "--fps", "9", "--file", videoName)
	log.Println(strings.Join(cmd.Args, " "))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to create stdout pipe: %s", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalf("Failed to create stderr pipe: %s", err)
	}

	go streamOutput(stdout)
	go streamOutput(stderr)

	err = cmd.Start()
	if err != nil {
		log.Fatalf("Failed to start command: %s", err)
	}

	err = cmd.Wait()
	if err != nil {
		log.Fatalf("Command finished with error: %s", err)
	}

	if err := manageService("start", recorderService); err != nil {
		serverError(&w, err)
		return
	}
	if err := manageService("start", tc2AgentService); err != nil {
		serverError(&w, err)
		return
	}
}

// Start or stop a service
func manageService(action, serviceName string) error {
	cmd := exec.Command("systemctl", action, serviceName)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to %s service %s: %v", action, serviceName, err)
	}
	log.Printf("Service %s %sd successfully.\n", serviceName, action)
	return nil
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
	connected := CheckInternetConnection("usb0")
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
	// Get currently saved Wi-Fi networks
	_, saved := netmanagerclient.FindNetworkBySSID(wifiDetails.SSID)

	// Find network in saved networks array
	if !saved {
		log.Printf("Attempting to connect to Wi-Fi SSID: %s", wifiDetails.SSID)
		if err := netmanagerclient.AddWifiNetwork(wifiDetails.SSID, wifiDetails.Password); err != nil {
			log.Printf("Error connecting to Wi-Fi: %v", err)
			http.Error(w, "Failed to connect to Wi-Fi: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := netmanagerclient.EnableWifi(true); err != nil {
			log.Printf("Error enabling Wi-Fi: %v", err)
			http.Error(w, "Failed to enable Wi-Fi: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Println("Connected to Wi-Fi successfully")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Connected to Wi-Fi successfully"))
	} else {
		if err := netmanagerclient.ConnectWifiNetwork(wifiDetails.SSID); err != nil {
			log.Printf("Error connecting to Wi-Fi: %v", err)
			http.Error(w, "Failed to connect to Wi-Fi: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("Wi-Fi network already saved: %s", wifiDetails.SSID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Wi-Fi network already saved"))
	}
}

func (api *ManagementAPI) DisconnectFromWifi(w http.ResponseWriter, r *http.Request) {
	currentSSID, err := getCurrentWifiNetwork()
	if err != nil {
		log.Printf("Error getting current Wi-Fi network: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "failed to get current Wi-Fi network\n")
		return
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "disconnecting from Wi-Fi network\n")
	if err != nil {
		log.Fatalf("Error getting state changes: %v", err)
	}
	go func() {
		if err := netmanagerclient.DisconnectWifiNetwork(currentSSID, true); err != nil {
			log.Printf("Error removing Wi-Fi network: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "failed to remove Wi-Fi network\n")
			return
		}
	}()
}

func (api *ManagementAPI) ForgetWifiNetwork(w http.ResponseWriter, r *http.Request) {
	// Parse request for SSID
	var wifiDetails struct {
		SSID string `json:"ssid"`
	}

	// Decode the JSON body
	if err := json.NewDecoder(r.Body).Decode(&wifiDetails); err != nil {
		log.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Will forget Wi-Fi network: %s", wifiDetails.SSID)
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "will forget Wi-Fi network shortly\n")
	go func() {
		currentSSID, _ := getCurrentWifiNetwork()
		// Forget the network
		currentlyConnetedTo := currentSSID == wifiDetails.SSID
		if err := netmanagerclient.RemoveWifiNetwork(wifiDetails.SSID, currentlyConnetedTo, currentlyConnetedTo); err != nil {
			log.Printf("Error removing Wi-Fi network: %v", err)
		}
	}()
}

func streamOutput(pipe io.ReadCloser) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		log.Println(scanner.Text())
	}
}

func (api *ManagementAPI) GetBattery(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		parseFormErrorResponse(&w, err)
		return
	}
	battery, err := getLastBatteryReading()
	if err != nil {
		serverError(&w, err)
		return
	}
	json.NewEncoder(w).Encode(battery)
}

func getModemDbus() (dbus.BusObject, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	return conn.Object("org.cacophony.modemd", "/org/cacophony/modemd"), nil
}

func (api *ManagementAPI) ModemStayOnFor(w http.ResponseWriter, r *http.Request) {
	modemDbus, err := getModemDbus()
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to connect to DBus", http.StatusInternalServerError)
		return
	}

	minutes, err := strconv.Atoi(r.FormValue("minutes"))
	if err != nil {
		badRequest(&w, err)
		return
	}
	err = modemDbus.Call("org.cacophony.modemd.StayOnFor", 0, minutes).Store()
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to request modem to stay on", http.StatusInternalServerError)
		return
	}
}

func (api *ManagementAPI) GetModem(w http.ResponseWriter, r *http.Request) {
	// Send dbus call to modem service to get all modem statuses
	modemDbus, err := getModemDbus()
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to connect to DBus", http.StatusInternalServerError)
		return
	}

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

func (api *ManagementAPI) GetSaltGrains(w http.ResponseWriter, r *http.Request) {
	file, err := os.Open("/etc/salt/grains")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open grains file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	grains := make(map[string]interface{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			grains[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse grains file: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(grains); err != nil {
		http.Error(w, "Failed to encode grains to JSON", http.StatusInternalServerError)
	}
}

// SetSaltGrains handles the HTTP request to set the grains from a JSON payload
func (api *ManagementAPI) SetSaltGrains(w http.ResponseWriter, r *http.Request) {
	// Read body as a JSON
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse JSON body
	var grains map[string]string
	if err := json.Unmarshal(body, &grains); err != nil {
		http.Error(w, "Failed to parse JSON body", http.StatusBadRequest)
		return
	}

	// Validate and set grains
	approvedKeyAndValues := map[string][]string{
		"environment": {"tc2-dev", "tc2-test", "tc2-prod"},
	}
	for key, value := range grains {
		if approvedValues, ok := approvedKeyAndValues[key]; !ok {
			http.Error(w, fmt.Sprintf("Key %s is not approved for setting grains", key), http.StatusBadRequest)
			return
		} else {
			approved := false
			for _, approvedValue := range approvedValues {
				if value == approvedValue {
					approved = true
					break
				}
			}
			if !approved {
				http.Error(w, fmt.Sprintf("Value %s is not approved for key %s", value, key), http.StatusBadRequest)
				return
			}
		}

		if !saltutil.IsSaltIdSet() {
			http.Error(w, "Salt is not yet ready to set grains", http.StatusInternalServerError)
			return
		}
		cmd := exec.Command("salt-call", "grains.setval", key, value)
		if output, err := cmd.CombinedOutput(); err != nil {
			http.Error(w, fmt.Sprintf("failed to set grain: %s, output: %s", err, output), http.StatusInternalServerError)
			return
		}
	}
}

func (api *ManagementAPI) SetAPN(w http.ResponseWriter, r *http.Request) {
	var apnDetails struct {
		APN string `json:"apn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&apnDetails); err != nil {
		log.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("Received APN to set: %s", apnDetails.APN)

	// Set APN using modemd dbus service
	modemDbus, err := getModemDbus()
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to connect to DBus", http.StatusInternalServerError)
		return
	}
	err = modemDbus.Call("org.cacophony.modemd.SetAPN", 0, apnDetails.APN).Store()
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to set APN", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
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

func (api *ManagementAPI) GetWifiNetworks(w http.ResponseWriter, r *http.Request) {
	networks, err := netmanagerclient.ListUserSavedWifiNetworks()
	if err != nil {
		serverError(&w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(networks)
}

func (api *ManagementAPI) PostWifiNetwork(w http.ResponseWriter, r *http.Request) {
	ssid := r.FormValue("ssid")
	if ssid == "" {
		badRequest(&w, errors.New("ssid field was empty"))
		return
	}
	psk := r.FormValue("psk")
	if psk == "" {
		badRequest(&w, errors.New("psk field was empty"))
		return
	}
	if err := netmanagerclient.AddWifiNetwork(ssid, psk); err != nil {
		if _, ok := err.(netmanagerclient.InputError); ok {
			badRequest(&w, err)
			return
		}
		serverError(&w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (api *ManagementAPI) DeleteWifiNetwork(w http.ResponseWriter, r *http.Request) {
	ssid := r.FormValue("ssid")
	if ssid == "" {
		badRequest(&w, errors.New("ssid field was empty"))
		return
	}
	if err := netmanagerclient.RemoveWifiNetwork(ssid, false, false); err != nil {
		if _, ok := err.(netmanagerclient.InputError); ok {
			badRequest(&w, err)
			return
		}
		serverError(&w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (api *ManagementAPI) ScanWifiNetwork(w http.ResponseWriter, r *http.Request) {
	networks, err := netmanagerclient.ScanWiFiNetworks()
	if err != nil {
		serverError(&w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(networks)
}

func (api *ManagementAPI) EnableWifi(w http.ResponseWriter, r *http.Request) {
	if err := netmanagerclient.EnableWifi(false); err != nil {
		serverError(&w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (api *ManagementAPI) EnableHotspot(w http.ResponseWriter, r *http.Request) {
	go func() {
		// TODO Wait before enabling hotspot to give time for response
		time.Sleep(time.Second)
		if err := netmanagerclient.EnableHotspot(true); err != nil {
			log.Println(err)
		}
	}()
	w.WriteHeader(http.StatusOK)
}

func (api *ManagementAPI) GetConnectionStatus(w http.ResponseWriter, r *http.Request) {
	state, err := netmanagerclient.ReadState()
	if err != nil {
		serverError(&w, err)
		return
	}

	data := map[string]interface{}{
		"state": state,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
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

	// Get currently saved Wi-Fi networks
	_, saved := netmanagerclient.FindNetworkBySSID(wifiDetails.SSID)

	if !saved {
		log.Printf("Attempting to connect to Wi-Fi SSID: %s", wifiDetails.SSID)
		if err := netmanagerclient.AddWifiNetwork(wifiDetails.SSID, wifiDetails.Password); err != nil {
			log.Printf("Error connecting to Wi-Fi: %v", err)
			http.Error(w, "Failed to connect to Wi-Fi: "+err.Error(), http.StatusInternalServerError)
			return
		}

		scanWifis, err := netmanagerclient.ScanWiFiNetworks()
		if err != nil {
			log.Printf("Error scanning Wi-Fi networks: %v", err)
			http.Error(w, "Failed to scan Wi-Fi networks: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var found bool
		for _, wifi := range scanWifis {
			if wifi.SSID == wifiDetails.SSID {
				found = true
				break
			}
		}

		if found {
			if err := netmanagerclient.ConnectWifiNetwork(wifiDetails.SSID); err != nil {
				log.Printf("Error connecting to Wi-Fi: %v", err)
				http.Error(w, "Failed to connect to Wi-Fi: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		log.Println("Added Wi-Fi successfully")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Connected to Wi-Fi successfully"))
	} else {
		if err := netmanagerclient.ConnectWifiNetwork(wifiDetails.SSID); err != nil {
			log.Printf("Error connecting to Wi-Fi: %v", err)
			http.Error(w, "Failed to connect to Wi-Fi: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Wi-Fi network already saved: %s", wifiDetails.SSID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Wi-Fi network already saved"))
	}
}

func (api *ManagementAPI) GetSavedWifiNetworks(w http.ResponseWriter, r *http.Request) {
	networks, err := netmanagerclient.ListUserSavedWifiNetworks()
	if err != nil {
		serverError(&w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(networks)
}

// DownloadAudioFile downloads an audio file
func (api *ManagementAPI) DownloadAudioFile(w http.ResponseWriter, r *http.Request) {
	// Get the file name from the request
	fileName := mux.Vars(r)["fileName"]
	if fileName == "" {
		http.Error(w, "Failed to get file name from request", http.StatusBadRequest)
		return
	}

	// Construct the full path to the aac file
	audioFolderPath := api.recordingDir
	aacFilePath := filepath.Join(audioFolderPath, fileName)
	defer os.Remove(aacFilePath) // Clean up the temporary file when done

	// Open the converted M4A file
	aacFile, err := os.Open(aacFilePath)
	if err != nil {
		log.Printf("Error opening converted M4A file: %v", err)
		http.Error(w, "Failed to open converted audio file", http.StatusInternalServerError)
		return
	}
	defer aacFile.Close()

	// Set the response headers
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	w.Header().Set("Content-Type", "audio/mp4")

	// Send the file as a response
	if _, err := io.Copy(w, aacFile); err != nil {
		log.Printf("Error sending M4A file: %v", err)
		http.Error(w, "Failed to send audio file", http.StatusInternalServerError)
		return
	}
}

func (api *ManagementAPI) GetAudioRecordings(w http.ResponseWriter, r *http.Request) {
	log.Println("get audio recordings")
	names := getAacNames(api.recordingDir)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(names)
}

func (api *ManagementAPI) UploadLogs(w http.ResponseWriter, r *http.Request) {
	twoWeeksAgo := time.Now().AddDate(0, 0, -14).Format("2006-01-02")
	journalctlCmd := exec.Command("journalctl", "--since", twoWeeksAgo)

	logFileName := "/tmp/journalctl-logs-last-2-weeks.log"
	logFile, err := os.Create(logFileName)
	if err != nil {
		log.Printf("Failed to create log file: %v", err)
		serverError(&w, err)
		return
	}
	defer logFile.Close()

	journalctlCmd.Stdout = logFile

	if err := journalctlCmd.Run(); err != nil {
		log.Printf("Failed to run journalctl command: %v", err)
		serverError(&w, err)
		return
	}

	if err := exec.Command("gzip", "-f", logFileName).Run(); err != nil {
		log.Printf("Failed to compress log file: %v", err)
		serverError(&w, err)
		return
	}

	if !saltutil.IsSaltIdSet() {
		http.Error(w, "Salt is not yet ready to upload logs", http.StatusInternalServerError)
		return
	}
	if err := exec.Command("salt-call", "cp.push", logFileName+".gz").Run(); err != nil {
		log.Printf("Error pushing log file with salt: %v", err)
		serverError(&w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
