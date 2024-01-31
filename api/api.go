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
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/TheCacophonyProject/event-reporter/eventclient"
	"github.com/TheCacophonyProject/trap-controller/trapdbusclient"

	netmanagerclient "github.com/TheCacophonyProject/rpi-net-manager/netmanagerclient"
)

const (
	cptvGlob            = "*.cptv"
	failedUploadsFolder = "failed-uploads"
	rebootDelay         = time.Second * 5
	apiVersion          = 8
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
	if err := initilseHotspot(); err != nil {
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
		GroupName  string `json:"groupname"`
		Devicename string `json:"devicename"`
		DeviceID   int    `json:"deviceID"`
		SaltID     string `json:"saltID"`
		Type       string `json:"type"`
	}
	info := deviceInfo{
		ServerURL:  device.Server,
		GroupName:  device.Group,
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
		toCamelCase(goconfig.AudioKey):            &goconfig.Audio{},
		toCamelCase(goconfig.GPIOKey):             &goconfig.GPIO{},
		toCamelCase(goconfig.LeptonKey):           &goconfig.Lepton{},
		toCamelCase(goconfig.ModemdKey):           &goconfig.Modemd{},
		toCamelCase(goconfig.PortsKey):            &goconfig.Ports{},
		toCamelCase(goconfig.TestHostsKey):        &goconfig.TestHosts{},
		toCamelCase(goconfig.ThermalMotionKey):    &goconfig.ThermalMotion{},
		toCamelCase(goconfig.ThermalRecorderKey):  &goconfig.ThermalRecorder{},
		toCamelCase(goconfig.ThermalThrottlerKey): &goconfig.ThermalThrottler{},
		toCamelCase(goconfig.WindowsKey):          &goconfig.Windows{},
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

type BatteryReading struct {
	Time        string `json:"time"`
	MainBattery string `json:"mainBattery"`
	RTCBattery  string `json:"rtcBattery"`
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
	if len(parts) != 3 {
		return BatteryReading{}, errors.New("unexpected format in battery-readings.csv")
	}

	return BatteryReading{
		Time:        parts[0],
		MainBattery: parts[1],
		RTCBattery:  parts[2],
	}, nil
}

func (api *ManagementAPI) GetTestVideos(w http.ResponseWriter, r *http.Request) {
	recordings, err := os.ReadDir("/var/spool/cptv/test-recordings")
	if err != nil {
		serverError(&w, err)
		return
	}
	recordingNames := []string{}
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

	out, err := exec.Command("systemctl", "stop", "thermal-recorder").CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to run command: %s, %s", err, out)
	}
	out, err = exec.Command("systemctl", "stop", "tc2-agent").CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to run command: %s, %s", err, out)
	}

	cmd := exec.Command("/home/pi/classifier/bin/python3", "/home/pi/classifier-pipeline/piclassify.py", "-c", "/home/pi/classifier-pipeline/pi-classifier.yaml", "--file", videoName)
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

	out, err = exec.Command("systemctl", "start", "thermal-recorder").CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to run command: %s, %s", err, out)
	}
	out, err = exec.Command("systemctl", "start", "tc2-agent").CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to run command: %s, %s", err, out)
	}
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
func CheckInternetConnection(interfaceName string) (bool, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return false, err
	}
	var foundInterface net.Interface
	for _, iface := range interfaces {
		if iface.Name == interfaceName {
			foundInterface = iface
			break
		}
	}
	if foundInterface.Flags&net.FlagUp == 0 {
		return false, fmt.Errorf("interface %s is down", interfaceName)
	}
	_, err = net.LookupHost("www.google.com")
	if err != nil {
		return false, nil // Internet is not reachable
	}
	return true, nil
}

func (api *ManagementAPI) CheckModemInternetConnection(w http.ResponseWriter, r *http.Request) {
	// Check if connected to modem
	log.Println("Checking modem connection")
	connected, err := CheckInternetConnection("usb0")
	if err != nil {
		log.Printf("Error checking modem connection: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "failed to check modem connection\n")
		return
	}
	log.Printf("Modem connection: %v", connected)

	// Send the current network as a JSON response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"connected": connected})
}

// Check Wifi Connection
func (api *ManagementAPI) CheckWifiInternetConnection(w http.ResponseWriter, r *http.Request) {
	// Check if connected to Wi-Fi
	log.Println("Checking Wi-Fi connection")
	connected, err := CheckInternetConnection("wlan0")
	if err != nil {
		log.Printf("Error checking Wi-Fi connection: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "failed to check Wi-Fi connection\n")
		return
	}
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

const (
	connectionCheckTimeout  = 30 * time.Second
	connectionCheckInterval = 5 * time.Second
)

// waitForWifiConnection checks if the device has connected to the specified SSID
func (api *ManagementAPI) waitForWifiConnection(ssid string) bool {
	timeout := time.After(connectionCheckTimeout)
	tick := time.NewTicker(connectionCheckInterval)
	defer tick.Stop()

	for {
		select {
		case <-timeout:
			return false // Timed out
		case <-tick.C:
			if currentSSID, _ := getCurrentWifiNetwork(); currentSSID == ssid {
				return true // Successfully connected
			}
		}
	}
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
	// Connect to the specified Wi-Fi network using wpa_cli
	if err := connectToWifi(wifiDetails.SSID, wifiDetails.Password); err != nil {
		log.Printf("Error connecting to Wi-Fi: %v", err)
		http.Error(w, "Failed to connect to Wi-Fi: "+err.Error(), http.StatusInternalServerError)
		go api.ManageHotspot()
		return
	}
	// Disconnect from the wifi to send the response
	//	log.Println("Disconnecting from Wi-Fi")
	//	cmd := exec.Command("wpa_cli", "-i", "wlan0", "disconnect")
	//	if err := cmd.Run(); err != nil {
	//		log.Printf("Error disconnecting from Wi-Fi: %v", err)
	//	}
	log.Println("Connected to Wi-Fi successfully")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Connected to Wi-Fi successfully"))
	//if err := connectToWifi(wifiDetails.SSID, wifiDetails.Password); err != nil {
	//	log.Printf("Error connecting to Wi-Fi: %v", err)
	//	http.Error(w, "Failed to connect to Wi-Fi: "+err.Error(), http.StatusInternalServerError)
	//}
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

// connectToWifi connects to a Wi-Fi network using wpa_cli
func connectToWifi(ssid string, passkey string) error {
	if err := addNetworkToWPAConfig(ssid, passkey); err != nil {
		log.Printf("Error adding Wi-Fi network to config: %v", err)
		return err
	}

	// Reload the wpa_supplicant config
	if err := exec.Command("wpa_cli", "-i", "wlan0", "reconfigure").Run(); err != nil {
		log.Printf("Error reconfiguring Wi-Fi: %v", err)
		// Remove the network from the config
		if err := removeNetworkFromWPAConfig(ssid); err != nil {
			log.Printf("Error removing Wi-Fi network: %v", err)
			return err
		}
		return err
	}

	if ssids, err := getSSIDIds(); err == nil {
		if id, ok := ssids[ssid]; ok {
			// Connect to the network
			if err := exec.Command("wpa_cli", "-i", "wlan0", "select_network", id).Run(); err != nil {
				log.Printf("Error selecting Wi-Fi network: %v", err)
				// Remove the network from the config
				if err := removeNetworkFromWPAConfig(ssid); err != nil {
					log.Printf("Error removing Wi-Fi network: %v", err)
					return err
				}
				return err
			} else {
				// reassociate
				if err := exec.Command("wpa_cli", "-i", "wlan0", "reassociate").Run(); err != nil {
					log.Printf("Error reassociating Wi-Fi network: %v", err)
					// Remove the network from the config
					if err := removeNetworkFromWPAConfig(ssid); err != nil {
						log.Printf("Error removing Wi-Fi network: %v", err)
						return err
					}
					return err
				}
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if connected := streamWifiState(ctx, ssid); !connected {
		// Remove the network from the config
		if err := netmanagerclient.RemoveWifiNetwork(ssid); err != nil {
			log.Printf("Error removing Wi-Fi network: %v", err)
			return err
		}
		return errors.New("failed to connect to Wi-Fi within the timeout")
	}

	log.Printf("Successfully connected to Wi-Fi SSID: %s", ssid)
	// restart dhcp client
	// remove static ip from dhcpcd.conf
	if err := exec.Command("sed", "-i", "/static ip_address=/d", "/etc/dhcpcd.conf").Run(); err != nil {
		log.Printf("Error removing static ip from dhcpcd.conf: %v", err)
	}
	if err := exec.Command("systemctl", "restart", "dhcpcd").Run(); err != nil {
		log.Printf("Error restarting DHCP client: %v", err)
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

func addNetworkToWPAConfig(ssid string, passkey string) error {
	configFile := "/etc/wpa_supplicant/wpa_supplicant.conf"

	// Read the current config file
	content, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}

	// Check if the network is already in the config
	if strings.Contains(string(content), fmt.Sprintf("ssid=\"%s\"", ssid)) {
		return nil
	}

	// Append the network to the config
	newContent := fmt.Sprintf(`%s
network={
    ssid="%s"
    psk="%s"
}`, string(content), ssid, passkey)

	return os.WriteFile(configFile, []byte(newContent), 0644)
}

func removeNetworkFromWPAConfig(ssid string) error {
	configFile := "/etc/wpa_supplicant/wpa_supplicant.conf"

	// Read the current config file
	content, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	var blockLines []string
	inBlock := false

	for _, line := range lines {
		if strings.Contains(line, "network={") {
			inBlock = true
			blockLines = []string{line}
		} else if inBlock {
			blockLines = append(blockLines, line)
			if strings.Contains(line, "}") {
				// Check the whole block for the SSID
				if !strings.Contains(strings.Join(blockLines, "\n"), fmt.Sprintf("ssid=\"%s\"", ssid)) {
					newLines = append(newLines, blockLines...)
				}
				inBlock = false
				blockLines = nil
			}
		} else {
			newLines = append(newLines, line)
		}
	}

	return os.WriteFile(configFile, []byte(strings.Join(newLines, "\n")), 0644)
}

func (api *ManagementAPI) GetWifiNetworks(w http.ResponseWriter, r *http.Request) {
	// Execute the command to scan for Wi-Fi networks
	log.Println("Scanning for Wi-Fi networks")
	cmd := exec.Command("iwlist", "wlan0", "scan")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error scanning for Wi-Fi networks: %v", err)
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

func reloadWPAConfig() error {
	cmd := exec.Command("wpa_cli", "reconfigure")
	return cmd.Run()
}

func (api *ManagementAPI) DisconnectFromWifi(w http.ResponseWriter, r *http.Request) {
	// Parse request for SSID
	currentSSID, err := checkIsConnectedToNetwork()
	if err != nil {
		log.Printf("Error getting current Wi-Fi network: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "failed to get current Wi-Fi network\n")
		return
	}
	// Respond to client before actually disconnecting
	log.Printf("Will disconnect from Wi-Fi network")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "will disconnect from Wi-Fi network shortly\n")

	go func() {
		// if ssid is bushnet/Bushnet don't remove from wpa_supplicant.conf
		if currentSSID != "bushnet" && currentSSID != "Bushnet" {
			if err := removeNetworkFromWPAConfig(currentSSID); err != nil {
				log.Printf("Error removing network from wpa_supplicant.conf: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, "failed to remove network from configuration\n")
				return
			}
		} else {
			if err := RestartDHCP(); err != nil {
				log.Printf("Error restarting DHCP client: %v", err)
			}
		}

		if err := exec.Command("wpa_cli", "-i", "wlan0", "disconnect").Run(); err != nil {
			log.Printf("Error disconnecting from Wi-Fi network: %v", err)
			// Handle the error
		}
		// Instruct wpa_supplicant to reconfigure
		cmd := exec.Command("wpa_cli", "-i", "wlan0", "reconfigure")
		if _, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Error reconfiguring Wi-Fi network: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "failed to reconfigure Wi-Fi network\n")
			return
		}
		// Reconnect to a network
		if err := exec.Command("wpa_cli", "-i", "wlan0", "reconnect").Run(); err != nil {
			log.Printf("Error reconnecting to Wi-Fi network: %v", err)
			// Handle the error
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

		log.Printf("Successfully deleted Wi-Fi network: %s", currentSSID)
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "deleted Wi-Fi network\n")
	}()
}

func getLastKey(m map[string]string) string {
	var lastKey string
	for k := range m {
		lastKey = k
	}
	return lastKey
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
	if err := netmanagerclient.RemoveWifiNetwork(ssid); err != nil {
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

// RecordAudio creates a mock audio file
func (api *ManagementAPI) RecordAudio(w http.ResponseWriter, r *http.Request) {
	// Create the audio folder if it doesn't exist
	audioFolderPath := goconfig.ThermalRecorder{}.OutputDir + "/audio"
	if _, err := os.Stat(audioFolderPath); os.IsNotExist(err) {
		err := os.Mkdir(audioFolderPath, 0755)
		if err != nil {
			log.Printf("Error creating audio folder: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "Failed to create audio folder\n")
			return
		}
	}

	// Create a new file within the audio folder
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	audioFilePrefix := "audio" + strconv.Itoa(random.Intn(1000000))
	audioFileSuffix := ".wav"
	filePath := audioFolderPath + "/" + audioFilePrefix + audioFileSuffix
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("Error creating audio file: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "Failed to create audio file\n")
		return
	}
	defer file.Close()

	// Write some mock data to the file
	_, err = file.WriteString("This is a mock audio file.")
	if err != nil {
		log.Printf("Error writing to audio file: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "Failed to write to audio file\n")
		return
	}

	// Send a success response
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "Audio file created successfully\n")
}

// GetAudioFiles returns a list of audio files
func (api *ManagementAPI) GetAudioFiles(w http.ResponseWriter, r *http.Request) {
	// Get the list of audio files
	audioFolderPath := goconfig.ThermalRecorder{}.OutputDir + "/audio"
	files, err := os.ReadDir(audioFolderPath)
	if err != nil {
		log.Printf("Error reading audio folder: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "Failed to read audio folder\n")
		return
	}

	// Send the list of audio files as a JSON response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(files)
}

// DownloadAudioFile downloads an audio file
func (api *ManagementAPI) DownloadAudioFile(w http.ResponseWriter, r *http.Request) {
	// Get the file name from the request
	fileName := mux.Vars(r)["fileName"]
	if fileName == "" {
		log.Printf("Error getting file name from request: %v", fileName)
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "Failed to get file name from request\n")
		return
	}

	// Open the file
	audioFolderPath := goconfig.ThermalRecorder{}.OutputDir + "/audio"
	filePath := audioFolderPath + "/" + fileName
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening audio file: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "Failed to open audio file\n")
		return
	}
	defer file.Close()

	// Set the response headers
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	w.Header().Set("Content-Type", r.Header.Get("Content-Type"))

	// Send the file as a response
	w.WriteHeader(http.StatusOK)
	io.Copy(w, file)
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
