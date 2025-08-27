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
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/godbus/dbus"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gobuffalo/packr"

	"github.com/gorilla/mux"
	"golang.org/x/net/websocket"

	goconfig "github.com/TheCacophonyProject/go-config"
	"github.com/TheCacophonyProject/go-cptv/cptvframe"
	"github.com/TheCacophonyProject/go-utils/logging"
	"github.com/TheCacophonyProject/lepton3"
	managementinterface "github.com/TheCacophonyProject/management-interface"
	"github.com/TheCacophonyProject/management-interface/api"
	netmanagerclient "github.com/TheCacophonyProject/rpi-net-manager/netmanagerclient"
	"github.com/TheCacophonyProject/thermal-recorder/headers"
	"github.com/alexflint/go-arg"
)

const (
	configDir     = goconfig.DefaultConfigDir
	socketTimeout = 7 * time.Second
	frameSocket   = "/var/spool/managementd"
)

var (
	haveClients = make(chan bool)
	version     = "<not set>"
	sockets     = make(map[int64]*WebsocketRegistration)
	socketsLock sync.RWMutex
	headerInfo  *headers.HeaderInfo
	frameCh     = make(chan *FrameData, 4)
	connected   atomic.Bool
	log         = logging.NewLogger("info")
)

func hasActiveClients() bool {
	socketsLock.RLock()
	defer socketsLock.RUnlock()
	return len(sockets) > 0
}

type Args struct {
	logging.LogArgs
}

func (Args) Version() string {
	return version
}

func procArgs() Args {
	args := Args{}
	arg.MustParse(&args)
	return args
}

func GetTC2AgentDbus() (dbus.BusObject, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	return conn.Object("org.cacophony.TC2Agent", "/org/cacophony/TC2Agent"), nil
}

// Set up and handle page requests.
func main() {
	args := procArgs()

	log = logging.NewLogger(args.LogLevel)

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
	router.HandleFunc("/audiorecording", managementinterface.GenAudioRecordingHandler(config.config)).Methods("GET")      // Form to view and/or set audio recording manually.
	router.HandleFunc("/low-power-thermal-recording", managementinterface.LowPowerThermalRecordingHandler).Methods("GET") // Form to view and/or set audio recording manually.
	router.HandleFunc("/location", managementinterface.GenLocationHandler(config.config)).Methods("GET")                  // Form to view and/or set location manually.
	router.HandleFunc("/clock", managementinterface.TimeHandler).Methods("GET")                                           // Form to view and/or adjust time settings.
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
	router.HandleFunc("/temperature-csv", managementinterface.DownloadTemperatureCSV).Methods("GET")

	// API
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiObj, err := api.NewAPI(apiRouter, config.config, version, log)
	if err != nil {
		log.Fatal(err)
		return
	}

	// Log all API requests (method, path, status, size, duration, client IP)
	apiRouter.Use(api.RequestLoggingMiddleware)
	apiRouter.HandleFunc("/device-info", apiObj.GetDeviceInfo).Methods("GET")
	apiRouter.HandleFunc("/recordings", apiObj.GetRecordings).Methods("GET")
	apiRouter.HandleFunc("/recording/{id}", apiObj.GetRecording).Methods("GET")
	apiRouter.HandleFunc("/recording/{id}", apiObj.DeleteRecording).Methods("DELETE")
	apiRouter.HandleFunc("/camera/snapshot", apiObj.TakeSnapshot).Methods("PUT")
	apiRouter.HandleFunc("/camera/snapshot-recording", apiObj.TakeSnapshotRecording).Methods("PUT")
	apiRouter.HandleFunc("/signal-strength", apiObj.GetSignalStrength).Methods("GET")
	apiRouter.HandleFunc("/reregister", apiObj.Reregister).Methods("POST")
	apiRouter.HandleFunc("/reregister-authorized", apiObj.ReregisterAuthorized).Methods("POST")
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
	apiRouter.HandleFunc("/salt-grains", apiObj.GetSaltGrains).Methods("GET")
	apiRouter.HandleFunc("/salt-grains", apiObj.SetSaltGrains).Methods("POST")
	apiRouter.HandleFunc("/modem/apn", apiObj.SetAPN).Methods("POST")
	apiRouter.HandleFunc("/modem-stay-on-for", apiObj.ModemStayOnFor).Methods("POST")
	apiRouter.HandleFunc("/battery", apiObj.GetBattery).Methods("GET")
	apiRouter.HandleFunc("/battery/config", apiObj.GetBatteryConfig).Methods("GET")
	apiRouter.HandleFunc("/battery/config", apiObj.SetBatteryConfig).Methods("POST")
	apiRouter.HandleFunc("/battery/config", apiObj.ClearBatteryConfig).Methods("DELETE")
	apiRouter.HandleFunc("/test-videos", apiObj.GetTestVideos).Methods("GET")
	apiRouter.HandleFunc("/play-test-video", apiObj.PlayTestVideo).Methods("POST")
	apiRouter.HandleFunc("/upload-test-recording", apiObj.UploadTestRecording).Methods("POST")
	apiRouter.HandleFunc("/network/interfaces", apiObj.GetNetworkInterfaces).Methods("GET")
	apiRouter.HandleFunc("/network/wifi", apiObj.ScanWifiNetwork).Methods("GET")
	apiRouter.HandleFunc("/network/wifi", apiObj.ConnectToWifi).Methods("POST")
	apiRouter.HandleFunc("/network/wifi/save", apiObj.SaveWifiNetwork).Methods("POST")
	apiRouter.HandleFunc("/network/wifi/saved", apiObj.GetSavedWifiNetworks).Methods("GET")
	apiRouter.HandleFunc("/network/wifi/saved", apiObj.GetSavedWifiNetworks).Methods("GET")
	apiRouter.HandleFunc("/network/wifi/forget", apiObj.ForgetWifiNetwork).Methods("DELETE")
	apiRouter.HandleFunc("/network/wifi/current", apiObj.GetCurrentWifiNetwork).Methods("GET")
	apiRouter.HandleFunc("/network/wifi/current", apiObj.DisconnectFromWifi).Methods("DELETE")
	apiRouter.HandleFunc("/wifi-check", apiObj.CheckWifiInternetConnection).Methods("GET")
	apiRouter.HandleFunc("/modem-check", apiObj.CheckModemInternetConnection).Methods("GET")
	apiRouter.HandleFunc("/wifi-networks", apiObj.GetWifiNetworks).Methods("GET")
	apiRouter.HandleFunc("/wifi-networks", apiObj.PostWifiNetwork).Methods("POST")
	apiRouter.HandleFunc("/wifi-networks", apiObj.DeleteWifiNetwork).Methods("Delete")
	apiRouter.HandleFunc("/wifi-network-scan", apiObj.ScanWifiNetwork).Methods("GET")
	apiRouter.HandleFunc("/enable-wifi", apiObj.EnableWifi).Methods("POST")
	apiRouter.HandleFunc("/enable-hotspot", apiObj.EnableHotspot).Methods("POST")
	apiRouter.HandleFunc("/wifi-status", apiObj.GetConnectionStatus).Methods("GET")
	apiRouter.HandleFunc("/upload-logs", apiObj.UploadLogs).Methods("PUT")

	apiRouter.HandleFunc("/audiorecording", apiObj.SetAudioRecording).Methods("POST")
	apiRouter.HandleFunc("/audiorecording", apiObj.GetAudioRecording).Methods("GET")
	apiRouter.HandleFunc("/audio/long-recording", apiObj.TakeLongAudioRecording).Methods("PUT")
	apiRouter.HandleFunc("/audio/test-recording", apiObj.TakeTestAudioRecording).Methods("PUT")
	apiRouter.HandleFunc("/audio/audio-status", apiObj.AudioRecordingStatus).Methods("GET")
	apiRouter.HandleFunc("/audio/recordings", apiObj.GetAudioRecordings).Methods("GET")

	apiRouter.HandleFunc("/offload-status", apiObj.RecordingOffloadStatus).Methods("GET")
	apiRouter.HandleFunc("/cancel-offload", apiObj.CancelOffload).Methods("PUT")
	apiRouter.HandleFunc("/thermal/long-test-recording", apiObj.TakeLongTestThermalRecording).Methods("PUT")
	apiRouter.HandleFunc("/thermal/short-test-recording", apiObj.TakeShortTestThermalRecording).Methods("PUT")
	apiRouter.HandleFunc("/thermal/thermal-status", apiObj.TestThermalRecordingStatus).Methods("GET")
	apiRouter.HandleFunc("/offload-now", apiObj.ForceRp2040Offload).Methods("PUT")
	apiRouter.HandleFunc("/serve-frames-now", apiObj.PrioritiseFrameServe).Methods("PUT")

	apiRouter.Use(basicAuth)

	apiRouter.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			netmanagerclient.KeepHotspotOnFor(60 * 5)

			out, err := exec.Command("stay-on-for", "5").CombinedOutput() // Stops camera from going to sleep for 5 minutes.
			if err != nil {
				log.Printf("error running stay-on-for: %s, error: %s", string(out), err)
			}
			next.ServeHTTP(w, r)
		})
	})

	go func() {
		for {
			err := os.Remove(frameSocket)
			if err != nil && !os.IsNotExist(err) {
				log.Printf("Couldn't remove  %v %v\n", frameSocket, err)
				time.Sleep(1000)
				continue
			}

			listener, err := net.Listen("unix", frameSocket)
			if err != nil {
				log.Println("Couldn't make socket", err)
				return
			}
			log.Print("waiting for frames from tc2-agent")

			listener.(*net.UnixListener).SetDeadline(time.Now().Add(5 * time.Second))
			conn, err := listener.Accept()
			if err != nil {
				if err.(net.Error).Timeout() {
					log.Printf("socket accept timed out, retrying...")

					if hasActiveClients() {
						listener.(*net.UnixListener).SetDeadline(time.Now().Add(30 * time.Second))
						// If there are users connected via web sockets, force the frames to get served.
						log.Println("Websocket has clients, forcing frame priority")
						tc2AgentDbus, err := GetTC2AgentDbus()
						if err != nil {
							log.Println(err)
							return
						}
						var result string
						err = tc2AgentDbus.Call("org.cacophony.TC2Agent.prioritiseframeserve", 0).Store(&result)
						if err != nil {
							log.Println(err)
							return
						}
					}

					continue
				}
				log.Printf("socket accept failed: %v", err)
				continue
			}

			// Prevent concurrent connections.
			listener.Close()

			log.Printf("accepted connection from client")
			err = handleConn(conn)
			frameCh <- &FrameData{Disconnected: true}
			log.Printf("camera connection ended with: %v", err)
			connected.Store(false)

		}
	}()

	listenAddr := fmt.Sprintf(":%d", config.Port)
	log.Printf("listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, router))
}

func handleConn(conn net.Conn) error {
	reader := bufio.NewReader(conn)
	var err error
	headerInfo, err = headers.ReadHeaderInfo(reader)
	if err != nil {
		return err
	}

	log.Printf("connection from %s %s (%dx%d@%dfps) frame size %d", headerInfo.Brand(), headerInfo.Model(), headerInfo.ResX(), headerInfo.ResY(), headerInfo.FPS(), headerInfo.FrameSize())

	clearB := make([]byte, 5)
	_, err = io.ReadFull(reader, clearB)
	if err != nil {
		return err
	}

	rawFrame := make([]byte, headerInfo.FrameSize())
	frame := cptvframe.NewFrame(headerInfo)
	frames := 0
	var lastFrame *FrameData
	connected.Store(true)
	for {
		_, err := io.ReadFull(reader, rawFrame)
		if err != nil {
			log.Println("Error reading frame ", err)
			return err
		}
		if len(sockets) == 0 {
			continue
		}
		if err := lepton3.ParseRawFrame(rawFrame, frame, 0); err != nil {
			log.Println("Could not parse lepton3 frame", err)
		} else {
			lastFrame = &FrameData{
				Frame: frame,
			}
			frameCh <- lastFrame
			frames += 1
			if frames == 1 || frames%100 == 0 {
				log.Printf("Got %v frames\n", frames)
			}
		}

	}
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
				if !connected.Load() {
					_ = websocket.Message.Send(ws, "disconnected")
				}
				socketsLock.Unlock()
				if firstSocket {
					log.Print("Get new client register")
					{
						tc2AgentDbus, err := GetTC2AgentDbus()
						if err != nil {
							log.Println(err)
							return
						}
						var isOffloading int
						var percentComplete int
						var secondsRemaining int
						var filesTotal int
						var filesRemaining int
						var eventsTotal int
						var eventsRemaining int
						err = tc2AgentDbus.Call("org.cacophony.TC2Agent.offloadstatus", 0).Store(&isOffloading, &percentComplete, &secondsRemaining, &filesTotal, &filesRemaining, &eventsTotal, &eventsRemaining)
						if err != nil {
							log.Println(err)
							return
						}
						if isOffloading == 1 {
							log.Printf("rp2040 is offloading files")
							var result string
							err = tc2AgentDbus.Call("org.cacophony.TC2Agent.canceloffload", 0).Store(&result)
							if err != nil {
								log.Println(err)
								return
							}
							log.Printf("requested offload cancellation")
						}
					}
					haveClients <- true
				}
			}
			if message.Type == "Heartbeat" {
				if socket, ok := sockets[message.Uuid]; ok {
					socket.LastHeartbeatAt = time.Now()
				}
			}
		}
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
	var lastFrame *FrameData

	for {
		// NOTE: Only bother with this work if we have clients connected.
		lastFrame = <-frameCh

		if len(sockets) != 0 {
			if lastFrame.Disconnected {
				socketsLock.RLock()
				for uuid, socket := range sockets {
					go func(socket *WebsocketRegistration, uuid int64, frameNum int) {
						// If the socket is busy sending the previous frame,
						// don't block, just move on to the next socket.
						if atomic.CompareAndSwapUint32(&socket.AtomicLock, 0, 1) {
							_ = websocket.Message.Send(socket.Socket, "disconnected")
							atomic.StoreUint32(&socket.AtomicLock, 0)
						} else {
							time.Sleep(100 * time.Millisecond)
						}
					}(socket, uuid, frameNum)
				}
				socketsLock.RUnlock()
			} else {
				// Make the frame info
				buffer := bytes.NewBuffer(make([]byte, 0))
				frameInfo := FrameInfo{
					Camera:    map[string]interface{}{"ResX": headerInfo.ResX(), "ResY": headerInfo.ResY()},
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
				frameNum = lastFrame.Frame.Status.FrameCount
			}
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

type FrameData struct {
	Disconnected bool
	Frame        *cptvframe.Frame
	Tracks       []map[string]interface{}
}
