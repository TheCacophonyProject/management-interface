package api

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	routerIP        = "192.168.4.1"
	hotspotSSID     = "bushnet"
	hotspotPassword = "feathers"
)

var dhcpConfigDefault = []string{
	"hostname",
	"clientid",
	"persistent",
	"option rapid_commit",
	"option domain_name_servers, domain_name, domain_search, host_name",
	"option classless_static_routes",
	"option interface_mtu",
	"require dhcp_server_identifier",
	"slaac private",
}

func createAPConfig() error {
	fileName := "/etc/hostapd/hostapd.conf"
	configLines := []string{
		"country_code=NZ",
		"interface=wlan0",
		fmt.Sprintf("ssid=%s", hotspotSSID),
		"hw_mode=g",
		"channel=7",
		"macaddr_acl=0",
		"ignore_broadcast_ssid=0",
		"wpa=2",
		fmt.Sprintf("wpa_passphrase=%s", hotspotPassword),
		"wpa_key_mgmt=WPA-PSK",
		"wpa_pairwise=TKIP",
		"rsn_pairwise=CCMP",
	}
	return createConfigFile(fileName, configLines)
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
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isInterfaceUp(interfaceName) {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("interface %s did not come up within the specified timeout", interfaceName)
}

func runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s failed: %w", command, err)
	}
	return nil
}

func startAccessPoint() error {
	if err := runCommand("systemctl", "restart", "hostapd"); err != nil {
		return err
	}
	return waitForInterface("wlan0", 30*time.Second)
}

func stopAccessPoint() error {
	return runCommand("systemctl", "stop", "hostapd")
}

func createDHCPConfig(isHotspot bool) error {
	filePath := "/etc/dhcpcd.conf"
	configLines := dhcpConfigDefault

	if isHotspot {
		configLines = append(configLines, "interface wlan0", fmt.Sprintf("static ip_address=%s/24", routerIP))
	}

	return writeLines(filePath, configLines)
}

func writeLines(filePath string, lines []string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("could not create file %s: %w", filePath, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("could not write to file %s: %w", filePath, err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("could not flush writer for file %s: %w", filePath, err)
	}
	return nil
}

func restartDHCP() error {
	if err := runCommand("systemctl", "daemon-reload"); err != nil {
		return err
	}
	return runCommand("systemctl", "restart", "dhcpcd")
}

func checkIsConnectedToNetwork() (string, error) {
	output, err := exec.Command("iwgetid", "wlan0", "-r").Output()
	if err != nil {
		return "", fmt.Errorf("could not get network status: %w", err)
	}
	network := strings.TrimSpace(string(output))
	if network == "" {
		return "", fmt.Errorf("not connected to a network")
	}
	return network, nil
}

func checkIsConnectedToNetworkWithRetries() (string, error) {
	var network string
	var err error
	for i := 0; i < 3; i++ {
		network, err = checkIsConnectedToNetwork()
		if err == nil {
			return network, nil
		}
		time.Sleep(2 * time.Second)
	}
	return "", err
}

func createDNSConfig(ipRange string) error {
	fileName := "/etc/dnsmasq.conf"
	configLines := []string{
		"interface=wlan0",
		fmt.Sprintf("dhcp-range=%s,12h", ipRange),
		"domain=wlan",
	}
	return createConfigFile(fileName, configLines)
}

func startDNS() error {
	return runCommand("systemctl", "restart", "dnsmasq")
}

func stopDNS() error {
	return runCommand("systemctl", "stop", "dnsmasq")
}

func createConfigFile(name string, config []string) error {
	return writeLines(name, config)
}

func enableNetwork(ssid string) error {
	output, err := exec.Command("wpa_cli", "list_networks").Output()
	if err != nil {
		return fmt.Errorf("could not list networks: %w", err)
	}

	networks := parseNetworks(string(output))
	for id, network := range networks {
		if network == ssid {
			return runCommand("wpa_cli", "enable_network", id)
		}
	}

	return fmt.Errorf("network %s not found", ssid)
}

func parseNetworks(output string) map[string]string {
	networks := make(map[string]string)
	lines := strings.Split(output, "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) > 1 {
			networks[fields[0]] = fields[1]
		}
	}
	return networks
}

func initializeHotspot() error {
	log.Printf("Initializing hotspot...")

	// Ensure that bushnet and Bushnet networks are enabled
	if err := enableNetwork("bushnet"); err != nil {
		log.Printf("Failed to enable bushnet network: %v", err)
	}
	if err := enableNetwork("Bushnet"); err != nil {
		log.Printf("Failed to enable Bushnet network: %v", err)
	}

	network, err := checkIsConnectedToNetworkWithRetries()
	if err == nil {
		return fmt.Errorf("already connected to network: %s", network)
	}

	if err := createDHCPConfig(true); err != nil {
		return fmt.Errorf("failed to create DHCP config: %v", err)
	}

	if err := restartDHCP(); err != nil {
		return fmt.Errorf("failed to restart DHCP: %v", err)
	}

	if err := createAPConfig(); err != nil {
		return fmt.Errorf("failed to create AP config: %v", err)
	}

	if err := createDNSConfig("192.168.4.2,192.168.4.20"); err != nil {
		return fmt.Errorf("failed to create DNS config: %v", err)
	}

	if err := startAccessPoint(); err != nil {
		return fmt.Errorf("failed to start access point: %v", err)
	}

	if err := startDNS(); err != nil {
		return fmt.Errorf("failed to start DNS: %v", err)
	}

	log.Printf("Hotspot initialized successfully")
	return nil
}

func stopHotspot() error {
	log.Printf("Stopping hotspot...")

	if err := stopAccessPoint(); err != nil {
		return fmt.Errorf("failed to stop access point: %v", err)
	}

	if err := stopDNS(); err != nil {
		return fmt.Errorf("failed to stop DNS: %v", err)
	}

	if err := createDHCPConfig(false); err != nil {
		return fmt.Errorf("failed to create DHCP config: %v", err)
	}

	if err := restartDHCP(); err != nil {
		return fmt.Errorf("failed to restart DHCP: %v", err)
	}

	log.Printf("Hotspot stopped successfully")
	return nil
}
