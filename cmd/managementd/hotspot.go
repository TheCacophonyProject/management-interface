package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
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
	return creatConfigFile(file_name, config_lines)
}

func startAccessPoint(name string) error {
	return exec.Command("systemctl", "restart", "hostapd").Start()
}

func stopAccessPoint() error {
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
	"interface usb0",
	"metric 300",
	"interface wlan0",
	"metric 200",
}

var dhcp_config_lines = []string{
	"interface wlan0",
	"static ip_address=" + router_ip + "/24",
	"nohook wpa_supplicant",
}

func createDHCPConfig() error {
	file_path := "/etc/dhcpcd.conf"

	// append to dhcpcd.conf if lines don't already exist
	config_lines := append(dhcp_config_default, dhcp_config_lines...)
	if err := writeLines(file_path, config_lines); err != nil {
		return err
	}
	return nil
}

func removeLines(file_path string, removed_lines []string) error {
	file, err := os.Open(file_path)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !contains(removed_lines, line) {
			lines = append(lines, line)
		}
	}

	return writeLines(file_path, lines)
}

func writeLines(file_path string, lines []string) error {
	file, err := os.Create(file_path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range lines {
		_, _ = fmt.Fprintln(w, line)
	}
	return w.Flush()
}

func contains(lines []string, line string) bool {
	for _, l := range lines {
		if strings.TrimSpace(l) == strings.TrimSpace(line) {
			return true
		}
	}
	return false
}

func startDHCP() error {
	// modify /etc/dhcpcd.conf
	if err := createDHCPConfig(); err != nil {
		return err
	}

	return exec.Command("systemctl", "restart", "dhcpcd").Run()
}

func restartDHCP() error {
	if err := writeLines("/etc/dhcpcd.conf", dhcp_config_default); err != nil {
		return err
	}

	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return err
	}
	return exec.Command("systemctl", "restart", "dhcpcd").Run()
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

func createDNSConfig(router_ip string, ip_range string) error {
	// DNSMASQ config
	file_name := "/etc/dnsmasq.conf"
	config_lines := []string{
		"interface=wlan0",
		"dhcp-range=" + ip_range + ",12h",
		"domain=wlan",
	}
	return creatConfigFile(file_name, config_lines)

}

func startDNS() error {
	return exec.Command("systemctl", "restart", "dnsmasq").Start()
}

func stopDNS() error {
	return exec.Command("systemctl", "stop", "dnsmasq").Start()
}

func creatConfigFile(name string, config []string) error {
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
func initilseHotspot() error {
	ssid := "bushnet"
	router_ip := "192.168.4.1"
	log.Printf("Setting DHCP to default...")
	if err := restartDHCP(); err != nil {
		log.Printf("Error restarting dhcpcd: %s", err)
	}
	// Check if already connected to a network
	// If not connected to a network, start hotspot
	log.Printf("Checking if connected to network...")
	if network, err := checkIsConnectedToNetwork(); err != nil {
		log.Printf("Starting Hotspot...")
		log.Printf("Creating Configs...")
		if err := createAPConfig(ssid); err != nil {
			return err
		}
		if err := createDNSConfig(router_ip, "192.168.4.2,192.168.4.20"); err != nil {
			return err
		}

		log.Printf("Starting DHCP...")
		if err := startDHCP(); err != nil {
			return err
		}
		log.Printf("Starting DNS...")
		if err := startDNS(); err != nil {
			return err
		}
		log.Printf("Starting Access Point...")
		if err := startAccessPoint(ssid); err != nil {
			return err
		}
		return nil
	} else {
		return fmt.Errorf("already connected to a network: %s", network)
	}
}

func stopHotspot() error {
	log.Printf("Stopping Hotspot")
	if err := stopAccessPoint(); err != nil {
		return err
	}
	if err := stopDNS(); err != nil {
		return err
	}
	if err := restartDHCP(); err != nil {
		return err
	}
	return nil
}
