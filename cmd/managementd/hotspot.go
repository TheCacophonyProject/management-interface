package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

type networkState string

const (
	NS_WIFI          networkState = "WIFI"
	NS_WIFI_SETUP    networkState = "WIFI_SETUP"
	NS_HOTSPOT       networkState = "HOTSPOT"
	NS_HOTSPOT_SETUP networkState = "HOTSPOT_SETUP"
)

type networkHandler struct {
	state networkState
}

func (nh *networkHandler) setState(ns networkState) {
	if nh.state != ns {
		log.Printf("State changed from %s to %s", nh.state, ns)
		nh.state = ns
	}
}

func (nh *networkHandler) reconfigureWifi() error {

	if nh.busy() {
		return fmt.Errorf("busy")
	}
	log.Println("Reconfigure wifi network.")
	if nh.state != NS_WIFI {
		log.Println("Setting up wifi before reconfiguring.")
		err := nh.setupWifi()
		if err != nil {
			return err
		}
	}
	nh.setState(NS_WIFI_SETUP)
	if err := run("wpa_cli", "-i", "wlan0", "reconfigure"); err != nil {
		return err
	}
	connected, err := waitAndCheckIfConnectedToNetwork()
	if err != nil {
		return err
	}
	if !connected {
		log.Println("Failed to connect to network, starting up hotspot.")
		return nh.setupHotspot()
	}
	log.Println("WIFI connected after reconfigure.")
	nh.setState(NS_WIFI)
	return nil
}

// refactor createAPConfig to remove duplication
func createAPConfig(name string) error {
	file_name := "/etc/hostapd/hostapd.conf"
	config_lines := []string{
		"country_code=NZ",
		"interface=wlan0",
		"ssid=" + name,
		"hw_mode=g",
		"channel=6",
		"macaddr_acl=0",
		"ignore_broadcast_ssid=0",
		"wpa=2",
		"wpa_passphrase=feathers",
		"wpa_key_mgmt=WPA-PSK",
		"wpa_pairwise=TKIP",
		"rsn_pairwise=CCMP",
	}
	return createConfigFile(file_name, config_lines)
}

const router_ip = "192.168.4.1"

func checkIsConnectedToNetwork() (bool, string, string, error) {
	cmd := exec.Command("wpa_cli", "-i", "wlan0", "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// TODO this stop the hotspot from running
		//log.Println("Failed to read wpa_cli status. Restarting networking.")
		//_ = run("systemctl", "restart", "networking")
		//cmd := exec.Command("wpa_cli", "-i", "wlan0", "status")
		//output, err = cmd.CombinedOutput()
		if err != nil {
			return false, "", "", fmt.Errorf("error executing wpa_cli: %w", err)
		}
	}

	ssid := ""
	ipAddress := ""
	stateCompleted := false
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ssid=") {
			ssid = strings.TrimPrefix(line, "ssid=")
		}
		if strings.HasPrefix(line, "ip_address=") {
			ipAddress = strings.TrimPrefix(line, "ip_address=")
		}
		if strings.Contains(line, "wpa_state=COMPLETED") {
			stateCompleted = true
		}
	}
	// When connecting to a network with the wrong password and wpa_state can be 'COMPLETED',
	// so to check that it has the correct password we also check for an ip address.
	if stateCompleted && ssid != "" && ipAddress != "" {
		log.Printf("Connected to '%s' with address '%s'", ssid, ipAddress)
		return true, ssid, ipAddress, nil
	} else {
		return false, ssid, ipAddress, nil
	}
}

// waitAndCheckIfConnectedToNetwork will return true if a network is connected to within 10 seconds
func waitAndCheckIfConnectedToNetwork() (bool, error) {
	for i := 0; i < 30; i++ {
		connected, _, _, err := checkIsConnectedToNetwork()
		if err != nil {
			return false, err
		}
		if connected {
			return true, nil
		}
		time.Sleep(time.Second)
	}
	return false, nil
}

func createDNSConfig(router_ip string, ip_range string) error {
	// DNSMASQ config
	file_name := "/etc/dnsmasq.conf"
	config_lines := []string{
		"interface=wlan0",
		"dhcp-range=" + ip_range + ",12h",
		"domain=wlan",
	}
	return createConfigFile(file_name, config_lines)
}

func createConfigFile(name string, config []string) error {
	file, err := os.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range config {
		_, err = fmt.Fprintln(w, line)
		if err != nil {
			return err
		}
	}
	err = w.Flush()
	if err != nil {
		return err
	}
	return nil
}

/*
// Setup Hotspot and stop after 5 minutes using a new goroutine
func initialiseHotspot() error {
	log.Println("Initialising Hotspot, first checking if device is connected to a wifi network.")
	log.Printf("Setting up DHCP config for connecting to wifi networks.")
	if err := setupWifi(); err != nil {
		return err
	}

	log.Printf("Checking if connected to network in next 10 seconds...")
	connected, err := waitAndCheckIfConnectedToNetwork()
	if err != nil {
		return err
	}
	if connected {
		return fmt.Errorf("already connected to a network")
	}
	log.Println("Not connected to a network, starting up hotspot.")
	return setupHotspot()
}
*/

func (nh *networkHandler) busy() bool {
	return nh.state == NS_WIFI_SETUP || nh.state == NS_HOTSPOT_SETUP
}

func (nh *networkHandler) setupWifiWithRollback() error {
	if nh.busy() {
		return fmt.Errorf("busy")
	}
	if nh.state != NS_WIFI {
		err := nh.setupWifi()
		if err != nil {
			return err
		}
	}

	log.Println("WiFi network is up, checking that device can connect to a network.")
	connected, err := waitAndCheckIfConnectedToNetwork()
	if err != nil {
		return err
	}
	if !connected {
		log.Println("Failed to connect to wifi. Starting up hotspot.")
		return nh.setupHotspot()
	}
	return nil
}

func (nh *networkHandler) setupHotspot() error {
	if nh.busy() {
		return fmt.Errorf("busy")
	}
	nh.setState(NS_HOTSPOT_SETUP)
	log.Println("Setting up network for hosting a hotspot.")

	if err := run("wpa_cli", "-i", "wlan0", "disconnect"); err != nil {
		return err
	}

	if err := run("systemctl", "stop", "wpa_supplicant"); err != nil {
		return err
	}

	if err := run("ip", "addr", "flush", "dev", "wlan0"); err != nil {
		return err
	}

	if err := run("ip", "link", "set", "wlan0", "down"); err != nil {
		return err
	}

	hotspotSSID := "bushnet"
	log.Printf("Creating AP config...")
	if err := createAPConfig(hotspotSSID); err != nil {
		return err
	}
	log.Printf("Creating DNS config...")
	if err := createDNSConfig(router_ip, "192.168.4.2,192.168.4.20"); err != nil {
		return err
	}

	log.Printf("Setting up DHCP config for hosting a hotspot.")
	if err := setDHCPMode(dhcpModeHotspot); err != nil {
		return err
	}

	if err := run("ip", "link", "set", "wlan0", "up"); err != nil {
		return err
	}

	log.Printf("Starting DNS...")
	if err := run("systemctl", "restart", "dnsmasq"); err != nil {
		return err
	}
	log.Printf("Starting Access Point...")
	if err := run("systemctl", "restart", "hostapd"); err != nil {
		return err
	}
	nh.setState(NS_HOTSPOT)
	return nil
}

func run(args ...string) error {
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		argsStr := strings.TrimSpace(strings.Join(args, " "))
		outStr := strings.TrimSpace(string(out))
		return fmt.Errorf("error running '%s', output: '%s', error: '%s'", argsStr, outStr, err)
	}
	return nil
}

// setupWifi will set up the wifi network settings for connecting to wifi networks.
// This includes stopping the hotspot.
func (nh *networkHandler) setupWifi() error {
	nh.setState(NS_WIFI_SETUP)
	log.Println("Setting up network for connecting to Wifi networks.")
	log.Println("Setting up DHCP config for connecting to wifi networks.")
	if err := setDHCPMode(dhcpModeWifi); err != nil {
		return err
	}
	log.Println("Stopping Hotspot")
	if err := run("systemctl", "stop", "hostapd"); err != nil {
		return err
	}
	log.Println("Stopping dnsmasq")
	if err := run("systemctl", "stop", "dnsmasq"); err != nil {
		return err
	}

	// Check if network needs restarting.

	log.Println("Restart networking") // This slows down the process, //TODO Find a safe way to not do this.

	// Only needs to be run when the hotspot restarts
	_, _, ipAddress, err := checkIsConnectedToNetwork()
	if err != nil || ipAddress == "192.168.4.1" {
		// Networking needs restarting
		log.Println("Networking needs restarting.")
		if err := run("systemctl", "restart", "networking"); err != nil {
			_ = run("rm", "/var/run/wpa_supplicant/wlan0")
			if err := run("systemctl", "restart", "networking"); err != nil {
				return err
			}
		}
	}

	log.Println("Restart WPA Supplicant")
	if err := run("systemctl", "restart", "wpa_supplicant"); err != nil {
		return err
	}
	log.Println("Re-enable wlan0")
	if err := run("ip", "link", "set", "wlan0", "up"); err != nil {
		return err
	}

	nh.setState(NS_WIFI)
	return nil
}

type dhcpMode string

const (
	dhcpModeWifi    dhcpMode = "WIFI"
	dhcpModeHotspot dhcpMode = "HOTSPOT"
)

var dhcp_config_default = []string{
	"hostname",
	"clientid",
	"persistent",
	"option rapid_commit",
	"option domain_name_servers, domain_name, domain_search, host_name",
	"option classless_static_routes",
	"option interface_mtu",
	"require dhcp_server_identifier",
	"slaac private",
	"denyinterfaces eth0",
	// TODO Add these lines when in wifi mode only, or maybe with the dhcpcd fix this won't be an issue anymore.
	// "interface usb0",
	// "metric 300",
	// "interface wlan0",
	// "metric 200",
}

var dhcp_config_hotspot_extra_lines = []string{
	"interface wlan0",
	"static ip_address=" + router_ip + "/24",
	"nohook wpa_supplicant",
	"nohook lookup-hostname, waitip, waitipv6 wlan0",
	"nohook lookup-hostname, waitip, waitipv6 eth0",
}

func setDHCPMode(mode dhcpMode) error {
	// TODO Have this done instead by having the config file be a symbolic link to either the wifi
	// or hotspot configuration. Can then check where the symbolic link is pointed to to see if it needs
	// changed and restarted.

	// Get config from what mode selected.
	config := []string{}
	switch mode {
	case dhcpModeWifi:
		config = dhcp_config_default
	case dhcpModeHotspot:
		config = append(dhcp_config_default, dhcp_config_hotspot_extra_lines...)
	}

	// Check if file already exists with the same config.
	filePath := "/etc/dhcpcd.conf"
	if _, err := os.Stat(filePath); err == nil {
		currentContent, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		newContent := strings.Join(config, "\n") + "\n"
		if string(currentContent) == newContent {
			// Config has not changed, ensure DHCP is running.
			return run("systemctl", "start", "dhcpcd")
		}
	}

	// Writing new config
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	w := bufio.NewWriter(file)
	for _, line := range config {
		_, _ = fmt.Fprintln(w, line)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	// Restart DHCP
	log.Println("Restarting dhcpcd")

	// TODO Sometimes dhcpcd takes a wile to restart, running these can help.
	//_ = run("ip", "link", "set", "wlan0", "down")
	//_ = run("ip", "link", "set", "eth0", "down")
	//_ = run("ip", "link", "set", "wlan0", "up")
	//_ = run("ip", "link", "set", "eth0", "up")
	log.Println("Rebooting dhcpcd")
	if err := run("systemctl", "restart", "dhcpcd"); err != nil {
		return err
	}
	log.Println("Done rebooting dhcpcd")
	return nil
}
