#!/bin/bash

# Variables
INTERFACE="wlan0"
HOSTAPD_SERVICE="hostapd"
SSID="$1"
PASSWORD="$2"
TEMP_WPA_SUPPLICANT_CONF="/tmp/wpa_temp.conf"
PROCESS_KILLED=0

# Create a temporary wpa_supplicant config
echo "ctrl_interface=DIR=/var/run/wpa_supplicant GROUP=netdev
update_config=1
country=NZ

network={
    ssid="bushnet"
    psk="feathers"
}
" > $TEMP_WPA_SUPPLICANT_CONF

cat $TEMP_WPA_SUPPLICANT_CONF
CONNECTION_STATUS=2

# Stop the hotspot
echo "stopping hotspot"
sudo systemctl stop $HOSTAPD_SERVICE

# Start wpa_supplicant in the background and pipe its output to grep
sudo wpa_supplicant -Dwext -c$TEMP_WPA_SUPPLICANT_CONF -i$INTERFACE | while read -r line && [[ $PROCESS_KILLED -eq 0 ]]; do
    echo "$line"  # Print the output to the console
    if echo "$line" | grep -q "CTRL-EVENT-CONNECTED"; then
        CONNECTION_STATUS=0
        # Connection established, kill wpa_supplicant
        sudo pkill -f "wpa_supplicant -Dwext -c$TEMP_WPA_SUPPLICANT_CONF -i$INTERFACE"
        PROCESS_KILLED=1
    fi
    if echo "$line" | grep -q "reason=WRONG_KEY" && [[ $PROCESS_KILLED -eq 0 ]]; then
        CONNECTION_STATUS=1
        # Connection failed, kill wpa_supplicant
        sudo pkill -f "wpa_supplicant -Dwext -c$TEMP_WPA_SUPPLICANT_CONF -i$INTERFACE"
        PROCESS_KILLED=1
    fi
done

# Start the hotspot again
sudo systemctl start $HOSTAPD_SERVICE

# Remove the temporary config
sudo rm $TEMP_WPA_SUPPLICANT_CONF

# Exit with connection status
exit $CONNECTION_STATUS
