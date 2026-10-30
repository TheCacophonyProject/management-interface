package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	goconfig "github.com/TheCacophonyProject/go-config"
	"github.com/godbus/dbus"
)

// The trap sends its status to tc2-hat-comms whenever it changes, which passes it on as a
// StatusChanged DBus signal. ListenForTrapStatus passes that on to any trap page that is
// open, through TrapStatusEvents, so the page shows a change straight away.

// trapStatusKeepAlive is how often an open status stream is written to when nothing has
// changed, so a connection that has gone is noticed, and it keeps the camera awake.
const trapStatusKeepAlive = 30 * time.Second

// trapStatusEvent is one server-sent event: "status" with the trap's status as JSON, or
// "unavailable" with why there isn't one, such as the trap service not running.
type trapStatusEvent struct {
	name string
	data string
}

// trapStatusHub hands each event to the open status streams.
type trapStatusHub struct {
	mu   sync.Mutex
	subs map[chan trapStatusEvent]struct{}
}

var trapStatuses = &trapStatusHub{subs: map[chan trapStatusEvent]struct{}{}}

func (h *trapStatusHub) subscribe() chan trapStatusEvent {
	ch := make(chan trapStatusEvent, 1)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subs[ch] = struct{}{}
	return ch
}

func (h *trapStatusHub) unsubscribe(ch chan trapStatusEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs, ch)
}

// publish hands the event to every open stream. Each event is the whole status, so a
// stream that hasn't written the last one yet only needs the newest.
func (h *trapStatusHub) publish(event trapStatusEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- event:
		default:
			// Only publish sends, and it holds the lock, so this makes room for the new one.
			select {
			case <-ch:
			default:
			}
			ch <- event
		}
	}
}

// trapCommsCheckInterval is how often it checks on tc2-hat-comms while the trap service
// isn't running, so the page hears if comms stops or gets started again.
const trapCommsCheckInterval = 5 * time.Second

// currentTrapStatus asks the trap for its status. tc2-hat-comms starts the trap service as
// it starts connecting to the trap, and gives the connection stage until it has connected.
func currentTrapStatus() trapStatusEvent {
	trapDbus, err := getTrapDbus()
	if err != nil {
		return trapStatusEvent{"unavailable", err.Error()}
	}
	var status string
	if err := trapDbus.Call(trapDbusName+".Status", 0).Store(&status); err != nil {
		if errors.Is(trapCallError(err), errTrapServiceNotRunning) {
			return trapCommsStatus()
		}
		return trapStatusEvent{"unavailable", err.Error()}
	}
	return trapStatusEvent{"status", status}
}

// trapCommsStatus says why the trap service isn't running. If comms is set up to talk to
// a trap and tc2-hat-comms is starting (or restarting, such as after a config change) it
// is the "STARTING" connection stage, otherwise why it isn't.
func trapCommsStatus() trapStatusEvent {
	config, err := goconfig.New(goconfig.DefaultConfigDir)
	if err != nil {
		return trapStatusEvent{"unavailable", fmt.Sprintf("failed to read the config: %v", err)}
	}
	comms := goconfig.DefaultComms()
	if err := config.Unmarshal(goconfig.CommsKey, &comms); err != nil {
		return trapStatusEvent{"unavailable", fmt.Sprintf("failed to read the comms config: %v", err)}
	}
	// tc2-hat-comms also runs trap control for "uart", for sidekick compatibility.
	if !comms.Enable || !slices.Contains([]string{"trap-control", "uart"}, comms.CommsOut) {
		return trapStatusEvent{"unavailable", "comms isn't set to trap-control, so the camera isn't talking to a trap"}
	}
	out, _ := exec.Command("systemctl", "is-active", "tc2-hat-comms").Output()
	switch state := strings.TrimSpace(string(out)); state {
	case "failed", "inactive":
		return trapStatusEvent{"unavailable", "trap comms (tc2-hat-comms) isn't running, it is " + state}
	default:
		return trapStatusEvent{"status", `{"Connection": "STARTING"}`}
	}
}

// ListenForTrapStatus passes the trap's StatusChanged signals on to the open status
// streams, and tells them when the trap service stops or starts. It doesn't return.
func ListenForTrapStatus() {
	for {
		err := listenForTrapStatus()
		log.Errorf("trap status: %v, trying again in 5 seconds", err)
		time.Sleep(5 * time.Second)
	}
}

func listenForTrapStatus() error {
	// A connection of its own, so it only gets the signals it asks for.
	conn, err := dbus.SystemBusPrivate()
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.Auth(nil); err != nil {
		return err
	}
	if err := conn.Hello(); err != nil {
		return err
	}
	for _, rule := range []string{
		"type='signal',interface='" + trapDbusName + "',member='StatusChanged'",
		"type='signal',sender='org.freedesktop.DBus',member='NameOwnerChanged',arg0='" + trapDbusName + "'",
	} {
		if call := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule); call.Err != nil {
			return call.Err
		}
	}
	signals := make(chan *dbus.Signal, 10)
	conn.Signal(signals)

	// While the trap service isn't running, comms is checked on every so often, and any
	// change in why is passed on.
	var serviceRunning bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, trapDbusName).Store(&serviceRunning); err != nil {
		return err
	}
	var lastCommsStatus trapStatusEvent
	checkComms := func() {
		if status := trapCommsStatus(); status != lastCommsStatus {
			lastCommsStatus = status
			trapStatuses.publish(status)
		}
	}
	commsCheck := time.NewTicker(trapCommsCheckInterval)
	defer commsCheck.Stop()

	for {
		select {
		case signal, ok := <-signals:
			if !ok {
				return errors.New("lost the connection to DBus")
			}
			switch signal.Name {
			case trapDbusName + ".StatusChanged":
				if status, ok := signalString(signal, 0); ok {
					trapStatuses.publish(trapStatusEvent{"status", status})
				}
			case "org.freedesktop.DBus.NameOwnerChanged":
				// The body is the name, its old owner and its new owner, "" when it has gone.
				newOwner, ok := signalString(signal, 2)
				if !ok {
					continue
				}
				serviceRunning = newOwner != ""
				if serviceRunning {
					trapStatuses.publish(currentTrapStatus())
				} else {
					lastCommsStatus = trapStatusEvent{}
					checkComms()
				}
			}
		case <-commsCheck.C:
			if !serviceRunning {
				checkComms()
			}
		}
	}
}

func signalString(signal *dbus.Signal, i int) (string, bool) {
	if len(signal.Body) <= i {
		return "", false
	}
	s, ok := signal.Body[i].(string)
	return s, ok
}

// TrapStatusEvents streams the trap's status as server-sent events. It starts with the
// current status, then sends it again each time it changes. Each is a "status" event with
// the JSON from GetTrapStatus, or an "unavailable" event with why there isn't one.
func (api *ManagementAPI) TrapStatusEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		serverError(&w, errors.New("streaming isn't supported"))
		return
	}
	// Subscribed before getting the current status, so a change in between isn't missed.
	events := trapStatuses.subscribe()
	defer trapStatuses.unsubscribe(events)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	write := func(text string) bool {
		if _, err := io.WriteString(w, text); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	writeEvent := func(event trapStatusEvent) bool {
		return write(fmt.Sprintf("event: %s\ndata: %s\n\n", event.name, event.data))
	}

	if !writeEvent(currentTrapStatus()) {
		return
	}
	keepAlive := time.NewTicker(trapStatusKeepAlive)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-events:
			if !writeEvent(event) {
				return
			}
		case <-keepAlive.C:
			// A comment line, which the page ignores.
			if !write(": keep-alive\n\n") {
				return
			}
			if api.KeepAwake != nil {
				api.KeepAwake()
			}
		}
	}
}
