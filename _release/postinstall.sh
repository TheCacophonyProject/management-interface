#!/bin/bash
set -e

# --- Configuration ---
# Define all services and what systemd action should be taken.
# Format is "service.name:<none|restart|start>"
SERVICES_TO_MANAGE=(
    "managementd.service:start"
)

# Extract just the service names for the check
service_files=()
for config in "${SERVICES_TO_MANAGE[@]}"; do
    service_files+=("${config%%:*}")
done

# Check all services at once to see if a reload is needed
if systemctl show "${service_files[@]}" --property=NeedDaemonReload | grep -q 'yes'; then
    echo "systemd reports unit files have changed. Running daemon-reload..."
    systemctl daemon-reload
else
    echo "No service file changes detected. Skipping daemon-reload."
fi

# Process each service based on the configuration array
for service_config in "${SERVICES_TO_MANAGE[@]}"; do
    SERVICE_NAME=${service_config%%:*}
    ACTION=${service_config##*:}

    echo "Processing service: $SERVICE_NAME"

    if ! systemctl is-enabled --quiet "$SERVICE_NAME"; then
        echo "Enabling '$SERVICE_NAME'..."
        systemctl enable "$SERVICE_NAME"
    else
        echo "Service '$SERVICE_NAME' is already enabled."
    fi

    if [ "$ACTION" = "start" ]; then
        echo "Starting '$SERVICE_NAME'..."
        systemctl start "$SERVICE_NAME"
    elif [ "$ACTION" = "restart" ]; then
        echo "Restarting '$SERVICE_NAME'..."
        systemctl restart "$SERVICE_NAME"
    fi
done

echo "Post-installation script finished."
