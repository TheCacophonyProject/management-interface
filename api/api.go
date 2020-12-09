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
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	goapi "github.com/TheCacophonyProject/go-api"
	goconfig "github.com/TheCacophonyProject/go-config"
	"github.com/TheCacophonyProject/lepton3"
	signalstrength "github.com/TheCacophonyProject/management-interface/signal-strength"
	saltrequester "github.com/TheCacophonyProject/salt-updater"
	"github.com/godbus/dbus"
	"github.com/gorilla/mux"

	"github.com/TheCacophonyProject/event-reporter/eventclient"
)

const (
	cptvGlob            = "*.cptv"
	failedUploadsFolder = "failed-uploads"
	rebootDelay         = time.Second * 5
	apiVersion          = 5
)

type ManagementAPI struct {
	cptvDir    string
	config     *goconfig.Config
	appVersion string
}

func NewAPI(config *goconfig.Config, appVersion string) (*ManagementAPI, error) {
	thermalRecorder := goconfig.DefaultThermalRecorder()
	if err := config.Unmarshal(goconfig.ThermalRecorderKey, &thermalRecorder); err != nil {
		return nil, err
	}

	return &ManagementAPI{
		cptvDir:    thermalRecorder.OutputDir,
		config:     config,
		appVersion: appVersion,
	}, nil
}

func (api *ManagementAPI) GetVersion(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"apiVersion": apiVersion,
		"appVersion": api.appVersion,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
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
	}
	info := deviceInfo{
		ServerURL:  device.Server,
		Groupname:  device.Group,
		Devicename: device.Name,
		DeviceID:   device.ID,
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
		io.WriteString(w, "cptv file not found\n")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, cptvName))
	w.Header().Set("Content-Type", "application/x-cptv")
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
	log.Printf("delete cptv '%s'", cptvName)
	recPath := getRecordingPath(cptvName, api.cptvDir)
	if recPath == "" {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "cptv file not found\n")
		return
	}
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

// Reregister can change the devices name and gruop
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
	return
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

// SetConfig is a way of writing new config to the device. It can only update one seciton at a time
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
		goconfig.ThermalMotionKey:    goconfig.DefaultThermalMotion(lepton3.Model35), //TODO don't assume that model 3.5 is being used
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

func (api *ManagementAPI) writeConfig(newConfig map[string]interface{}) error {
	log.Printf("writing to config: %s", newConfig)
	for k, v := range newConfig {
		if err := api.config.Set(k, v); err != nil {
			return err
		}
	}
	return nil
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

func getRecordingPath(cptv, dir string) string {
	// Check that given file is a cptv file on the device.
	isCptvFile := false
	for _, name := range getCptvNames(dir) {
		if name == cptv {
			isCptvFile = true
			break
		}
	}
	if !isCptvFile {
		return ""
	}
	paths := []string{
		filepath.Join(dir, cptv),
		filepath.Join(dir, failedUploadsFolder, cptv),
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

//GetSaltUpdateState will get the salt update status
func (api *ManagementAPI) GetSaltUpdateState(w http.ResponseWriter, r *http.Request) {
	state, err := saltrequester.State()
	if err != nil {
		log.Printf("error getting salt state: %v", err)
		serverError(&w, errors.New("failed to get salt state"))
		return
	}
	json.NewEncoder(w).Encode(state)
}

func isEventKey(key uint64) (bool, error) {
	keys, err := eventclient.GetEventKeys()
	if err != nil {
		return false, err
	}
	for _, k := range keys {
		if k == key {
			return true, nil
		}
	}
	return false, nil
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
