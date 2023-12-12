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

// refactor createAPConfig to remove duplication
func createAPConfig(name string) error {
	file_name := "/etc/hostapd/hostapd.conf"
	config_lines := []string{
		"country_code=NZ",
		"interface=wlan0",
		"ssid=" + name,
		"hw_mode=g",
		"channel=7",
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

/*
func createDHCPConfig() (bool, error) {
	file_path := "/etc/dhcpcd.conf"

	// append to dhcpcd.conf if lines don't already exist
	config_lines := append(dhcp_config_default, dhcp_config_hotspot_extra_lines...)
	return writeLines(file_path, config_lines)
}

func writeLines(file_path string, lines []string) (bool, error) {

}
*/

/*
func startDHCP() error {
	// modify /etc/dhcpcd.conf
	configModified, err := createDHCPConfig()
	if err != nil {
		return err
	}
	if configModified {
		return exec.Command("systemctl", "restart", "dhcpcd").Run()
	}
	return exec.Command("systemctl", "start", "dhcpcd").Run()

}
*/

/*
func setDHCPToDefault() error {
	// Only restart if config has changed
	configModified, err := writeLines("/etc/dhcpcd.conf", dhcp_config_default)
	if err != nil {
		return err
	}

	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return err
	}
	if configModified {
		return exec.Command("systemctl", "restart", "dhcpcd").Run()
	}
	return exec.Command("systemctl", "start", "dhcpcd").Run()
}
*/

/*
func checkIsConnectedToNetworkOld() (string, bool, error) {
	if val, err := exec.Command("iwgetid", "wlan0", "-r").Output(); err != nil {
		return "", false, err
	} else {
		network := strings.TrimSpace(string(val))
		if network == "" {
			return "", false, fmt.Errorf("not connected to a network")
		} else {
			return string(network), true, nil
		}
	}
}
*/

func checkIsConnectedToNetwork() (bool, error) {
	cmd := exec.Command("wpa_cli", "-i", "wlan0", "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("error executing wpa_cli: %w", err)
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
		return true, nil
	} else {
		return false, nil
	}
}

// To check if connected to a network run `wpa_cli -i wlan0 status and check that wpa_state=COMPLETED and that there is a ip_address`
/*
var connected = `
$ wpa_cli -i wlan0 status
bssid=be:dc:b0:3e:21:f5
freq=2462
ssid=bushnet
id=0
mode=station
pairwise_cipher=CCMP
group_cipher=CCMP
key_mgmt=WPA2-PSK
wpa_state=COMPLETED
ip_address=192.168.50.41
p2p_device_address=ba:27:eb:c5:25:75
address=b8:27:eb:c5:25:75
`

var notConnected = `
$ wpa_cli -i wlan0 status
wpa_state=SCANNING
p2p_device_address=ba:27:eb:c5:25:75
address=b8:27:eb:c5:25:75
uuid=50431fd7-93e8-5d27-a41d-0ab834fb4511
`

var tryingToConnectWithWrongPassword = `
$ wpa_cli -i wlan0 status
wpa_state=ASSOCIATING
p2p_device_address=ba:27:eb:c5:25:75
address=b8:27:eb:c5:25:75
uuid=50431fd7-93e8-5d27-a41d-0ab834fb4511

and

$ wpa_cli -i wlan0 status
bssid=7c:ff:4d:1b:58:3e
freq=2417
ssid=bushnet2
id=2
mode=station
pairwise_cipher=CCMP
group_cipher=CCMP
key_mgmt=WPA2-PSK
wpa_state=COMPLETED
p2p_device_address=ba:27:eb:c5:25:75
address=b8:27:eb:c5:25:75
uuid=50431fd7-93e8-5d27-a41d-0ab834fb4511
`
*/
// waitAndCheckIfConnectedToNetwork will return true if a network is connected to within 10 seconds
func waitAndCheckIfConnectedToNetwork() (bool, error) {
	for i := 0; i < 30; i++ {
		connected, err := checkIsConnectedToNetwork()
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

func setupHotspot() error {
	log.Println("Setting up network for hosting a hotspot.")

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
	log.Printf("Starting DNS...")
	if err := run("systemctl", "restart", "dnsmasq"); err != nil {
		return err
	}
	log.Printf("Starting Access Point...")
	if err := run("systemctl", "restart", "hostapd"); err != nil {
		return err
	}
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
func setupWifi() error {
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
	// TODO run `rm /var/run/wpa_supplicant/wlan0` if needing to restart network.
	/*
		if err := run("systemctl", "restart", "networking"); err != nil {
			_ = run("rm", "/var/run/wpa_supplicant/wlan0")
			if err := run("systemctl", "restart", "networking"); err != nil {
				return err
			}
		}
	*/
	log.Println("Restart WPA Supplicant")
	if err := run("systemctl", "restart", "wpa_supplicant"); err != nil {
		return err
	}
	log.Println("Re-enable wlan0")
	if err := run("ip", "link", "set", "wlan0", "up"); err != nil {
		return err
	}
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
	// ip link set wlan0 down
	// ip link set eth0 down
	// ip link set wlan0 up
	// ip link set eth0 up
	return run("systemctl", "restart", "dhcpcd")
}
