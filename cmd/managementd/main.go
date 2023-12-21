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

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gobuffalo/packr"
	"github.com/godbus/dbus"
	"github.com/gorilla/mux"
	"golang.org/x/net/websocket"

	goconfig "github.com/TheCacophonyProject/go-config"
	"github.com/TheCacophonyProject/go-cptv/cptvframe"
	managementinterface "github.com/TheCacophonyProject/management-interface"
	"github.com/TheCacophonyProject/management-interface/api"
	"github.com/TheCacophonyProject/rpi-net-manager/netmanagerclient"
)

const (
	configDir     = goconfig.DefaultConfigDir
	socketTimeout = 7 * time.Second
)

var haveClients = make(chan bool)
var version = "<not set>"
var sockets = make(map[int64]*WebsocketRegistration)
var socketsLock sync.RWMutex
var cameraInfo map[string]interface{}
var lastFrame *FrameData
var currentFrame = -1

// Set up and handle page requests.
func main() {
	log.SetFlags(0) // Removes timestamp output
	log.Printf("running version: %s", version)

	config, err := ParseConfig(configDir)
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Printf("config: %v", config)
	if config.Port != 80 {
		log.Printf("warning: avahi service is advertised on port 80 but port %v is being used", config.Port)
	}

	router := mux.NewRouter()

	// Serve up static content.
	static := packr.NewBox("../../static")
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(static)))
	router.Handle("/ws", websocket.Handler(WebsocketServer))
	router.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		favicon, err := static.Find("favicon.ico")
		if err != nil {
			http.Error(w, "Favicon not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "image/x-icon")
		w.WriteHeader(http.StatusOK)
		w.Write(favicon)
	})

	go sendFrameToSockets()
	// UI handlers.
	router.HandleFunc("/", managementinterface.IndexHandler).Methods("GET")
	router.HandleFunc("/wifi-networks", managementinterface.WifiNetworkHandler).Methods("GET")
	router.HandleFunc("/network", managementinterface.NetworkHandler).Methods("GET")
	router.HandleFunc("/interface-status/{name:[a-zA-Z0-9-* ]+}", managementinterface.CheckInterfaceHandler).Methods("GET")
	router.HandleFunc("/disk-memory", managementinterface.DiskMemoryHandler).Methods("GET")
	router.HandleFunc("/location", managementinterface.GenLocationHandler(config.config)).Methods("GET") // Form to view and/or set location manually.
	router.HandleFunc("/clock", managementinterface.TimeHandler).Methods("GET")                          // Form to view and/or adjust time settings.
	router.HandleFunc("/about", managementinterface.AboutHandlerGen(config.config)).Methods("GET")
	router.HandleFunc("/advanced", managementinterface.AdvancedMenuHandler).Methods("GET")
	router.HandleFunc("/camera", managementinterface.CameraHandler).Methods("GET")
	router.HandleFunc("/camera/snapshot", managementinterface.CameraSnapshot).Methods("GET")
	router.HandleFunc("/rename", managementinterface.Rename).Methods("GET")
	router.HandleFunc("/config", managementinterface.Config).Methods("GET")
	router.HandleFunc("/audiobait", managementinterface.Audiobait).Methods("GET")
	router.HandleFunc("/modem", managementinterface.Modem).Methods("GET")
	router.HandleFunc("/battery", managementinterface.Battery).Methods("GET")
	router.HandleFunc("/battery-csv", managementinterface.DownloadBatteryCSV).Methods("GET")

	// API
	apiObj, err := api.NewAPI(config.config, version)

	if err != nil {
		log.Fatal(err)
		return
	}
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/device-info", apiObj.GetDeviceInfo).Methods("GET")
	apiRouter.HandleFunc("/recordings", apiObj.GetRecordings).Methods("GET")
	apiRouter.HandleFunc("/recording/{id}", apiObj.GetRecording).Methods("GET")
	apiRouter.HandleFunc("/recording/{id}", apiObj.DeleteRecording).Methods("DELETE")
	apiRouter.HandleFunc("/camera/snapshot", apiObj.TakeSnapshot).Methods("PUT")
	apiRouter.HandleFunc("/camera/snapshot-recording", apiObj.TakeSnapshotRecording).Methods("PUT")
	apiRouter.HandleFunc("/signal-strength", apiObj.GetSignalStrength).Methods("GET")
	apiRouter.HandleFunc("/reregister", apiObj.Reregister).Methods("POST")
	apiRouter.HandleFunc("/reboot", apiObj.Reboot).Methods("POST")
	apiRouter.HandleFunc("/config", apiObj.GetConfig).Methods("GET")
	apiRouter.HandleFunc("/config", apiObj.SetConfig).Methods("POST")
	apiRouter.HandleFunc("/clear-config-section", apiObj.ClearConfigSection).Methods("POST")
	apiRouter.HandleFunc("/location", apiObj.SetLocation).Methods("POST") // Set location via a POST request.
	apiRouter.HandleFunc("/location", apiObj.GetLocation).Methods("GET")  // Get location via a POST request.
	apiRouter.HandleFunc("/clock", apiObj.GetClock).Methods("GET")
	apiRouter.HandleFunc("/clock", apiObj.PostClock).Methods("POST")
	apiRouter.HandleFunc("/version", apiObj.GetVersion).Methods("GET")
	apiRouter.HandleFunc("/event-keys", apiObj.GetEventKeys).Methods("GET")
	apiRouter.HandleFunc("/events", apiObj.GetEvents).Methods("GET")
	apiRouter.HandleFunc("/events", apiObj.DeleteEvents).Methods("DELETE")
	apiRouter.HandleFunc("/trigger-trap", apiObj.TriggerTrap).Methods("PUT")
	apiRouter.HandleFunc("/check-salt-connection", apiObj.CheckSaltConnection).Methods("GET")
	apiRouter.HandleFunc("/salt-update", apiObj.StartSaltUpdate).Methods("POST")
	apiRouter.HandleFunc("/salt-update", apiObj.GetSaltUpdateState).Methods("GET")
	apiRouter.HandleFunc("/auto-update", apiObj.GetSaltAutoUpdate).Methods("GET")
	apiRouter.HandleFunc("/auto-update", apiObj.PostSaltAutoUpdate).Methods("POST")
	apiRouter.HandleFunc("/audiobait", apiObj.GetAudiobait).Methods("GET")
	apiRouter.HandleFunc("/play-test-sound", apiObj.PlayTestSound).Methods("POST")
	apiRouter.HandleFunc("/play-audiobait-sound", apiObj.PlayAudiobaitSound).Methods("POST")
	apiRouter.HandleFunc("/logs", apiObj.GetServiceLogs).Methods("GET")
	apiRouter.HandleFunc("/service", apiObj.GetServiceStatus).Methods("GET")
	apiRouter.HandleFunc("/service-restart", apiObj.RestartService).Methods("POST")
	apiRouter.HandleFunc("/modem", apiObj.GetModem).Methods("GET")
	apiRouter.HandleFunc("/modem-stay-on-for", apiObj.ModemStayOnFor).Methods("POST")
	apiRouter.HandleFunc("/battery", apiObj.GetBattery).Methods("GET")
	apiRouter.HandleFunc("/test-videos", apiObj.GetTestVideos).Methods("GET")
	apiRouter.HandleFunc("/play-test-video", apiObj.PlayTestVideo).Methods("POST")
	apiRouter.HandleFunc("/wifi-networks", apiObj.GetWifiNetworks).Methods("GET")
	apiRouter.HandleFunc("/wifi-networks", apiObj.PostWifiNetwork).Methods("POST")
	apiRouter.HandleFunc("/wifi-networks", apiObj.DeleteWifiNetwork).Methods("Delete")
	apiRouter.HandleFunc("/wifi-network-scan", apiObj.ScanWifiNetwork).Methods("GET")
	apiRouter.HandleFunc("/enable-wifi", apiObj.EnableWifi).Methods("POST")
	apiRouter.HandleFunc("/enable-hotspot", apiObj.EnableHotspot).Methods("POST")
	apiRouter.HandleFunc("/wifi-status", apiObj.GetConnectionStatus).Methods("GET")

	apiRouter.Use(basicAuth)

	apiRouter.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			netmanagerclient.KeepHotspotOnFor(60 * 5)
			next.ServeHTTP(w, r)
		})
	})

	listenAddr := fmt.Sprintf(":%d", config.Port)
	log.Printf("listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, router))

}

func basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userPassEncoded := "YWRtaW46ZmVhdGhlcnM=" // admin:feathers base64 encoded.
		if r.Header.Get("Authorization") == "Basic "+userPassEncoded {
			next.ServeHTTP(w, r)
		} else {
			http.Error(w, "Forbidden", http.StatusForbidden)
		}
	})
}

type WebsocketRegistration struct {
	AtomicLock      uint32
	Socket          *websocket.Conn
	LastHeartbeatAt time.Time
}

func (socket *WebsocketRegistration) Inactive() bool {
	return time.Since(socket.LastHeartbeatAt) >= socketTimeout
}

type message struct {
	// the json tag means this will serialize as a lowercased field
	Type string `json:"type"`
	Data string `json:"data"`
	Uuid int64  `json:"uuid"`
}

func WebsocketServer(ws *websocket.Conn) {
	for {
		// Receive any messages from the client
		message := message{}
		if err := websocket.JSON.Receive(ws, &message); err != nil {
			// Probably EOF error, when there's no message.  Maybe could sleep, so we're not thrashing this?
		} else {
			// When we first get a connection, register the websocket and push it onto an array of websockets.
			// Occasionally go through the list and cull any that are no-longer sending heart-beats.
			if message.Type == "Register" {
				socketsLock.Lock()
				firstSocket := len(sockets) == 0
				sockets[message.Uuid] = &WebsocketRegistration{
					Socket:          ws,
					LastHeartbeatAt: time.Now(),
					AtomicLock:      0,
				}
				socketsLock.Unlock()
				if firstSocket {
					log.Print("Get new client register")
					haveClients <- true
				}
			}
			if message.Type == "Heartbeat" {
				if socket, ok := sockets[message.Uuid]; ok {
					socket.LastHeartbeatAt = time.Now()
				}
			}
		}
		// TODO(jon): This blocks, so lets avoid busy-waiting
		time.Sleep(1 * time.Millisecond)
	}
}

type FrameInfo struct {
	Camera        map[string]interface{}
	Telemetry     cptvframe.Telemetry
	Calibration   map[string]interface{}
	BinaryVersion string
	AppVersion    string
	Mode          string
	Tracks        []map[string]interface{}
}

func sendFrameToSockets() {
	frameNum := 0
	var fps int32 = 9
	sleepDuration := time.Duration(1000/fps) * time.Millisecond
	for {
		// NOTE: Only bother with this work if we have clients connected.
		if len(sockets) != 0 {
			if cameraInfo == nil {
				cameraInfo = Headers()
				// waiting for camera to connect
				if cameraInfo == nil {
					time.Sleep(time.Second)
					continue
				}
				fps = cameraInfo["FPS"].(int32)
				sleepDuration = time.Duration(1000/fps) * time.Millisecond
			}
			time.Sleep(sleepDuration)
			lastFrame = GetFrame()
			if lastFrame == nil {
				continue
			}
			// Make the frame info
			buffer := bytes.NewBuffer(make([]byte, 0))
			// lastFrameLock.RLock()
			frameInfo := FrameInfo{
				Camera:    cameraInfo,
				Telemetry: lastFrame.Frame.Status,
				Tracks:    lastFrame.Tracks,
			}
			frameInfoJson, _ := json.Marshal(frameInfo)
			frameInfoLen := len(frameInfoJson)
			// Write out the length of the frameInfo json as a u16
			_ = binary.Write(buffer, binary.LittleEndian, uint16(frameInfoLen))
			_ = binary.Write(buffer, binary.LittleEndian, frameInfoJson)
			for _, row := range lastFrame.Frame.Pix {
				_ = binary.Write(buffer, binary.LittleEndian, row)
			}
			// Send the buffer back to the client
			frameBytes := buffer.Bytes()
			socketsLock.RLock()
			for uuid, socket := range sockets {
				go func(socket *WebsocketRegistration, uuid int64, frameNum int) {
					// If the socket is busy sending the previous frame,
					// don't block, just move on to the next socket.
					if atomic.CompareAndSwapUint32(&socket.AtomicLock, 0, 1) {
						_ = websocket.Message.Send(socket.Socket, frameBytes)
						atomic.StoreUint32(&socket.AtomicLock, 0)
					} else {
						// Locked, skip this frame to let client catch up.
						log.Println("Skipping frame for", uuid, frameNum)
					}
				}(socket, uuid, frameNum)
			}
			socketsLock.RUnlock()
			frameNum++

			var socketsToRemove []int64
			socketsLock.RLock()
			for uuid, socket := range sockets {
				if socket.Inactive() {
					socketsToRemove = append(socketsToRemove, uuid)
				}
			}
			socketsLock.RUnlock()
			if len(socketsToRemove) != 0 {
				socketsLock.Lock()
				for _, socketUuid := range socketsToRemove {
					socket := sockets[socketUuid]
					delete(sockets, socketUuid)
					go func(socket *WebsocketRegistration, uuid int64) {
						log.Println("Dropping old socket", uuid)
						_ = socket.Socket.Close()
						log.Println("Dropped old socket", uuid)
					}(socket, socketUuid)
				}
				socketsLock.Unlock()
			}
		} else {
			log.Print("Wait for new client camera register")
			<-haveClients
		}
	}
}

func Headers() map[string]interface{} {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil
	}
	recorder := conn.Object("org.cacophony.thermalrecorder", "/org/cacophony/thermalrecorder")
	specs := map[string]interface{}{}
	err = recorder.Call("org.cacophony.thermalrecorder.CameraInfo", 0).Store(&specs)
	if err != nil {
		log.Printf("Error getting camera headers %v", err)
		return nil
	}
	return specs
}

type FrameData struct {
	Frame  *cptvframe.Frame
	Tracks []map[string]interface{}
}

func GetFrame() *FrameData {

	conn, err := dbus.SystemBus()
	if err != nil {
		return nil
	}

	recorder := conn.Object("org.cacophony.thermalrecorder", "/org/cacophony/thermalrecorder")
	f := &FrameData{&cptvframe.Frame{}, nil}
	start := time.Now()

	c := recorder.Call("org.cacophony.thermalrecorder.TakeSnapshot", 0, currentFrame)
	if c.Err != nil {
		log.Printf("Err taking snapshot %v", err)
		return nil
	}
	val := c.Body[0].([]interface{})
	elapsed := time.Since(start)
	log.Printf("Snapshot took %s", elapsed)

	tel := val[1].([]interface{})
	f.Frame.Pix = val[0].([][]uint16)
	f.Frame.Status.TimeOn = time.Duration(tel[0].(int64))
	f.Frame.Status.FFCState = tel[1].(string)
	f.Frame.Status.FrameCount = int(tel[2].(int32))
	f.Frame.Status.FrameMean = tel[3].(uint16)
	f.Frame.Status.TempC = tel[4].(float64)
	f.Frame.Status.LastFFCTempC = tel[5].(float64)
	f.Frame.Status.LastFFCTime = time.Duration(tel[6].(int64))
	f.Frame.Status.BackgroundFrame = tel[7].(bool)
	if len(val) == 3 {
		jsonS := val[2].(string)
		if jsonS != "" {
			json.Unmarshal([]byte(jsonS), &f.Tracks)
		}
	}

	currentFrame = f.Frame.Status.FrameCount

	return f
}
