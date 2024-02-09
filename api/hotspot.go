package api

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
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

func isInterfaceUp(interfaceName string) bool {
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		log.Printf("Error getting interface %s: %v", interfaceName, err)
		return false
	}
	return iface.Flags&net.FlagUp == net.FlagUp
}

func waitForInterface(interfaceName string, timeout time.Duration) error {
	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		if isInterfaceUp(interfaceName) {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("interface %s did not come up within the specified timeout", interfaceName)
}

func startAccessPoint(_ string) error {
	err := exec.Command("systemctl", "restart", "hostapd").Run()
	if err != nil {
		return err
	}
	// Wait for wlan0 to be up before proceeding
	return waitForInterface("wlan0", 30*time.Second)
}

func stopAccessPoint() error {
	println("Stopping Access Point")
	return exec.Command("systemctl", "stop", "hostapd").Start()
}

const router_ip = "192.168.4.1"

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
	"interface wlan0",
	"metric 200",
}

var dhcp_config_lines = []string{
	"static ip_address=" + router_ip + "/24",
}

func createDHCPConfig() (bool, error) {
	file_path := "/etc/dhcpcd.conf"

	// append to dhcpcd.conf if lines don't already exist
	config_lines := append(dhcp_config_default, dhcp_config_lines...)
	return writeLines(file_path, config_lines)
}

func writeLines(file_path string, lines []string) (bool, error) {
	// Check if file already exists with the same config.
	if _, err := os.Stat(file_path); err == nil {
		currentContent, err := os.ReadFile(file_path)
		if err != nil {
			return false, err
		}
		newContent := strings.Join(lines, "\n") + "\n"
		if string(currentContent) == newContent {
			return false, nil
		}
	}

	file, err := os.Create(file_path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range lines {
		_, _ = fmt.Fprintln(w, line)
	}
	if err := w.Flush(); err != nil {
		return false, err
	}

	return true, nil
}

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

func RestartDHCP() error {
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

func checkIsConnectedToNetwork() (string, error) {
	if val, err := exec.Command("iwgetid", "wlan0", "-r").Output(); err != nil {
		return "", err
	} else {
		network := strings.TrimSpace(string(val))
		if network == "" {
			return "", fmt.Errorf("not connected to a network")
		} else {
			return string(network), nil
		}
	}
}

func checkIsConnectedToNetworkWithRetries() (string, error) {
	var err error
	var ssid string
	for i := 0; i < 5; i++ {
		ssid, err = checkIsConnectedToNetwork()
		if ssid != "" {
			return ssid, nil
		}
		time.Sleep(time.Second * 5)
	}
	return ssid, err
}

func createDNSConfig(ip_range string) error {
	// DNSMASQ config
	file_name := "/etc/dnsmasq.conf"
	config_lines := []string{
		"interface=wlan0",
		"dhcp-range=" + ip_range + ",12h",
		"domain=wlan",
	}
	return createConfigFile(file_name, config_lines)
}

func startDNS() error {
	return exec.Command("systemctl", "restart", "dnsmasq").Start()
}

func stopDNS() error {
	println("Stopping DNS")
	return exec.Command("systemctl", "stop", "dnsmasq").Start()
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

func isHotspotRunning() bool {
	// Check the status of the hostapd service
	cmd := exec.Command("systemctl", "is-active", "hostapd")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error checking hostapd status: %v", err)
		return false
	}

	// The output is "active" if the service is running
	return strings.TrimSpace(string(output)) == "active"
}

// Setup Hotspot and stop after 5 minutes using a new goroutine
func initilseHotspot() error {
	ssid := "bushnet"
	log.Printf("Setting DHCP to default...")
	if err := RestartDHCP(); err != nil {
		log.Printf("Error restarting dhcpcd: %s", err)
	}
	// Check if already connected to a network
	if val, err := exec.Command("iwgetid", "wlan0", "-r").Output(); err != nil {
		log.Printf("Error checking if connected to network: %s", err)
	} else {
		log.Printf("Wlan0 is connected to: %s", val)
	}
	// If not connected to a network, start hotspot
	log.Printf("Checking if connected to network...")
	if network, err := checkIsConnectedToNetworkWithRetries(); err == nil {
		// Check if the hotspot is already running
		return fmt.Errorf("already connected to a network: %s", network)
	}
	log.Printf("Starting Hotspot...")
	log.Printf("Creating Configs...")
	if err := createAPConfig(ssid); err != nil {
		return err
	}
	if err := createDNSConfig("192.168.4.2,192.168.4.20"); err != nil {
		return err
	}

	log.Printf("Starting Access Point...")
	if err := startAccessPoint(ssid); err != nil {
		return err
	}
	log.Printf("Starting DNS...")
	if err := startDNS(); err != nil {
		return err
	}
	log.Printf("Starting DHCP...")
	if err := startDHCP(); err != nil {
		return err
	}
	return nil
}

func stopHotspot() error {
	log.Printf("Stopping Hotspot")
	if err := stopAccessPoint(); err != nil {
		return err
	}
	if err := stopDNS(); err != nil {
		return err
	}
	if err := RestartDHCP(); err != nil {
		return err
	}
	return nil
}

// listConnectedDevices returns a slice of MAC addresses of devices connected to the hotspot
func listConnectedDevices() ([]string, error) {
	cmd := exec.Command("iw", "dev", "wlan0", "station", "dump")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	return parseMACAddresses(out.String()), nil
}

// parseMACAddresses parses the output of 'iw dev wlan0 station dump' to extract MAC addresses
func parseMACAddresses(output string) []string {
	var macAddresses []string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Station") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				macAddresses = append(macAddresses, fields[1])
			}
		}
	}
	return macAddresses
}
