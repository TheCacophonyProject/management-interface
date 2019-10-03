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
	signalstrength "github.com/TheCacophonyProject/management-interface/signal-strength"
	"github.com/godbus/dbus"
	"github.com/gorilla/mux"
)

const (
	cptvGlob            = "*.cptv"
	failedUploadsFolder = "failed-uploads"
	rebootDelay         = time.Second * 5
)

type ManagementAPI struct {
	cptvDir string
	config  *goconfig.Config
}

func NewAPI(config *goconfig.Config) (*ManagementAPI, error) {
	thermalRecorder := goconfig.DefaultThermalRecorder()
	if err := config.Unmarshal(goconfig.ThermalRecorderKey, &thermalRecorder); err != nil {
		return nil, err
	}

	return &ManagementAPI{
		cptvDir: thermalRecorder.OutputDir,
		config:  config,
	}, nil
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
	log.Println(info)
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

// GetConfig will return the config settings and the defaults
func (api *ManagementAPI) GetConfig(w http.ResponseWriter, r *http.Request) {
	if err := api.config.Update(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}

	configSections := []string{
		goconfig.AudioKey,
		goconfig.BatteryKey,
		goconfig.DeviceKey,
		goconfig.GPIOKey,
		goconfig.LeptonKey,
		goconfig.LocationKey,
		goconfig.ModemdKey,
		goconfig.PortsKey,
		goconfig.TestHostsKey,
		goconfig.ThermalMotionKey,
		goconfig.ThermalRecorderKey,
		goconfig.ThermalThrottlerKey,
		goconfig.WindowsKey,
	}

	configDefaults := map[string]interface{}{
		goconfig.AudioKey:            goconfig.DefaultAudio(),
		goconfig.GPIOKey:             goconfig.DefaultGPIO(),
		goconfig.LeptonKey:           goconfig.DefaultLepton(),
		goconfig.ModemdKey:           goconfig.DefaultModemd(),
		goconfig.PortsKey:            goconfig.DefaultPorts(),
		goconfig.TestHostsKey:        goconfig.DefaultTestHosts(),
		goconfig.ThermalMotionKey:    goconfig.DefaultThermalMotion(),
		goconfig.ThermalRecorderKey:  goconfig.DefaultThermalRecorder(),
		goconfig.ThermalThrottlerKey: goconfig.DefaultThermalThrottler(),
		goconfig.WindowsKey:          goconfig.DefaultWindows(),
	}

	configMap := map[string]interface{}{}

	for _, section := range configSections {
		configMap[section] = api.config.Get(section)
	}

	valuesAndDefaults := map[string]interface{}{
		"values":   configMap,
		"defaults": configDefaults,
	}

	jsonString, err := json.Marshal(valuesAndDefaults)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	w.Write(jsonString)
}

// ClearConfigSection will delet the config from a section so the defautl values will be used.
func (api *ManagementAPI) ClearConfigSection(w http.ResponseWriter, r *http.Request) {
	section := r.FormValue("section")
	log.Printf("clearing config section %s", section)

	if err := api.config.Unset(section); err != nil {
		badRequest(&w, err)
	}
}

// SetLocation is a deprecated API endpoint specifically for writing to location settings. Use SetConfig instead.
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
		log.Println("bad timestamp")
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
		badRequest(&w, err)
		return
	}
}

func badRequest(w *http.ResponseWriter, err error) {
	(*w).WriteHeader(http.StatusBadRequest)
	io.WriteString(*w, err.Error())
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
