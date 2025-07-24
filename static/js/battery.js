"use strict";

let batteryConfig = null;

window.onload = function () {
  getState();
  getBatteryConfig();
  setInterval(getState, 5000);
};

async function getState() {
  try {
    var response = await apiGetJSON("/api/battery");
    console.log(response);

    $("#time").html(response.time);
    $("#mainBattery").html(response.mainBattery);
    $("#rtcBattery").html(response.rtcBattery);
    
    // Update battery percentage if available
    if (response.batteryPercentage) {
      $("#batteryPercentage").html(response.batteryPercentage + "%");
    }
    
    // Update battery type if available from reading data
    if (response.batteryType) {
      $("#batteryType").html(response.batteryType);
    }
    
    // Update chemistry from API reading data if available
    if (response.batteryChemistry) {
      $("#batteryChemistry").html(response.batteryChemistry);
    }
  } catch (e) {
    console.log(e);
  }
}

async function getBatteryConfig() {
  try {
    batteryConfig = await apiGetJSON("/api/battery/config");
    console.log("Battery config:", batteryConfig);
    
    updateBatteryConfigUI();
    populateBatteryTypeSelect();
    updateAvailableBatteryTypes();
  } catch (e) {
    console.log("Error loading battery config:", e);
    showBatteryConfigStatus("Error loading battery configuration", "danger");
  }
}

function updateBatteryConfigUI() {
  if (!batteryConfig) return;
  
  // Update current battery type display
  $("#batteryType").html(batteryConfig.currentType || "Unknown");
  $("#batteryChemistry").html(batteryConfig.currentChemistry || "Unknown");
  
  // Update configuration status
  const status = batteryConfig.manuallyConfigured ? "Manual Override" : "Auto-Detection";
  $("#configurationStatus").html(status);
  
  // Update button states
  if (batteryConfig.manuallyConfigured) {
    $("#saveBatteryTypeBtn").text("Update Manual Override");
    $("#clearBatteryTypeBtn").removeClass("btn-secondary").addClass("btn-warning");
  } else {
    $("#saveBatteryTypeBtn").text("Save Manual Override");
    $("#clearBatteryTypeBtn").removeClass("btn-warning").addClass("btn-secondary");
  }
}

function populateBatteryTypeSelect() {
  if (!batteryConfig || !batteryConfig.availableTypes) return;
  
  const select = $("#batteryTypeSelect");
  select.empty();
  
  // Add default option
  if (!batteryConfig.manuallyConfigured) {
    select.append('<option value="">Auto-Detection (Current)</option>');
  } else {
    select.append('<option value="">Select Battery Type</option>');
  }
  
  // Add available types
  batteryConfig.availableTypes.forEach(function(type) {
    const selected = batteryConfig.manuallyConfigured && 
                    batteryConfig.currentType === type.name ? "selected" : "";
    select.append(`<option value="${type.name}" ${selected}>${type.description}</option>`);
  });
}

function updateAvailableBatteryTypes() {
  if (!batteryConfig || !batteryConfig.availableTypes) return;
  
  const list = $("#availableBatteryTypes");
  list.empty();
  
  batteryConfig.availableTypes.forEach(function(type) {
    const isCurrent = batteryConfig.currentType === type.name;
    const className = isCurrent ? "font-weight-bold text-primary" : "";
    const marker = isCurrent ? " (current)" : "";
    
    list.append(`<li class="${className}">
      <small>${type.name} (${type.chemistry})<br>
      ${type.minVoltage}V - ${type.maxVoltage}V${marker}</small>
    </li>`);
  });
}

async function saveBatteryType() {
  const selectedType = $("#batteryTypeSelect").val();
  
  if (!selectedType) {
    showBatteryConfigStatus("Please select a battery type", "warning");
    return;
  }
  
  try {
    showBatteryConfigStatus("Saving battery type...", "info");
    
    const response = await fetch("/api/battery/config", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": "Basic " + btoa("admin:feathers")
      },
      body: JSON.stringify({ batteryType: selectedType })
    });
    
    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Failed to save battery type: ${errorText}`);
    }
    
    const result = await response.json();
    console.log("Battery type saved:", result);
    
    showBatteryConfigStatus(`Battery type manually set to: ${selectedType}`, "success");
    
    // Refresh config
    await getBatteryConfig();
    
  } catch (e) {
    console.log("Error saving battery type:", e);
    showBatteryConfigStatus(`Error: ${e.message}`, "danger");
  }
}

async function clearBatteryType() {
  try {
    showBatteryConfigStatus("Clearing manual override...", "info");
    
    const response = await fetch("/api/battery/config", {
      method: "DELETE",
      headers: {
        "Authorization": "Basic " + btoa("admin:feathers")
      }
    });
    
    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Failed to clear battery configuration: ${errorText}`);
    }
    
    const result = await response.json();
    console.log("Battery config cleared:", result);
    
    showBatteryConfigStatus("Manual override cleared. Using auto-detection.", "success");
    
    // Refresh config
    await getBatteryConfig();
    
  } catch (e) {
    console.log("Error clearing battery config:", e);
    showBatteryConfigStatus(`Error: ${e.message}`, "danger");
  }
}

function showBatteryConfigStatus(message, type) {
  const statusDiv = $("#batteryConfigStatus");
  const alertClass = `alert alert-${type}`;
  
  statusDiv.removeClass().addClass(alertClass).html(message).show();
  
  // Auto-hide success and info messages after 5 seconds
  if (type === "success" || type === "info") {
    setTimeout(() => {
      statusDiv.fadeOut();
    }, 5000);
  }
}

function downloadBatteryCsv() {
  window.location.href = "/battery-csv";
}
