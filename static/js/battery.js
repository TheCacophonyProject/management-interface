"use strict";

let batteryConfig = null;
let manualConfigActive = false;

window.onload = function () {
  getState();
  getBatteryConfig();
  setInterval(getState, 10000);
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

    // Update chemistry from API reading data if available (but preserve manual config)
    if (response.batteryChemistry && !manualConfigActive) {
      $("#batteryChemistry").html(response.batteryChemistry);
    }

    // Update cell count if available (but preserve manual config)
    if (response.batteryCellCount && !manualConfigActive) {
      $("#batteryCellCount").html(response.batteryCellCount + " cells");

      // Calculate and display voltage range
      if (response.batteryChemistry && batteryConfig) {
        updateVoltageRange(response.batteryChemistry, parseInt(response.batteryCellCount));
      }
    }

    // Check if backend has caught up with manual configuration
    if (manualConfigActive && batteryConfig && batteryConfig.manuallyConfigured) {
      const backendMatchesManual =
        (!batteryConfig.currentChemistry || response.batteryChemistry === batteryConfig.currentChemistry) &&
        (!batteryConfig.currentCellCount || response.batteryCellCount == batteryConfig.currentCellCount);

      if (backendMatchesManual) {
        console.log("Backend has caught up with manual configuration, allowing normal updates");
        manualConfigActive = false;
      }
    }

    // Update discharge rate if available
    if (response.dischargeRate && response.dischargeRate !== "0.00") {
      $("#batteryDischargeRate").html(response.dischargeRate + "%/hour");
    } else {
      $("#batteryDischargeRate").html("N/A");
    }

    // Update time remaining if available
    if (response.hoursRemaining && response.hoursRemaining !== "0.0") {
      const hours = parseFloat(response.hoursRemaining);
      if (hours > 24) {
        const days = Math.floor(hours / 24);
        const remainingHours = Math.round(hours % 24);
        $("#batteryTimeRemaining").html(`${days}d ${remainingHours}h`);
      } else {
        $("#batteryTimeRemaining").html(`${Math.round(hours)}h`);
      }
    } else {
      $("#batteryTimeRemaining").html("N/A");
    }

    // Update depletion confidence if available
    if (response.depletionConfidence && response.depletionConfidence !== "0.0") {
      $("#depletionConfidence").html(response.depletionConfidence + "%");
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
    updateAvailableChemistries();
  } catch (e) {
    console.log("Error loading battery config:", e);
    showBatteryConfigStatus("Error loading battery configuration", "danger");
  }
}

function updateBatteryConfigUI() {
  if (!batteryConfig) return;

  // Update configuration status
  const status = batteryConfig.manuallyConfigured ? "Manual Override" : "Auto-Detection";
  $("#configurationStatus").html(status);

  // Reset manual config flag if we're not manually configured (handles edge cases)
  if (!batteryConfig.manuallyConfigured) {
    manualConfigActive = false;
  }

  // Update button states
  if (batteryConfig.manuallyConfigured) {
    $("#saveBatteryConfigBtn").text("Update Configuration");
    $("#clearBatteryConfigBtn").removeClass("btn-secondary").addClass("btn-warning");
  } else {
    $("#saveBatteryConfigBtn").text("Save Configuration");
    $("#clearBatteryConfigBtn").removeClass("btn-warning").addClass("btn-secondary");
  }

  // Set current values in form
  if (batteryConfig.currentChemistry) {
    $("#chemistrySelect").val(batteryConfig.currentChemistry);
  }
  if (batteryConfig.currentCellCount && batteryConfig.currentCellCount > 0) {
    $("#cellCountInput").val(batteryConfig.currentCellCount);

    // Update voltage range display if we have both chemistry and cell count
    if (batteryConfig.currentChemistry) {
      updateVoltageRange(batteryConfig.currentChemistry, batteryConfig.currentCellCount);
    }
  }
}

function populateBatteryTypeSelect() {
  populateChemistrySelect();
  updateAvailableChemistries();
}

function populateChemistrySelect() {
  if (!batteryConfig || !batteryConfig.availableChemistries) return;

  const select = $("#chemistrySelect");
  select.empty();

  // Add default option
  select.append('<option value="">Auto-detect</option>');

  // Add available chemistries
  batteryConfig.availableChemistries.forEach(function (chem) {
    const selected = batteryConfig.currentChemistry === chem.chemistry ? "selected" : "";
    select.append(`<option value="${chem.chemistry}" ${selected}>${chem.chemistry} (${chem.minVoltage}V-${chem.maxVoltage}V)</option>`);
  });
}

function updateAvailableChemistries() {
  if (!batteryConfig || !batteryConfig.availableChemistries) return;

  const list = $("#availableChemistries");
  list.empty();

  batteryConfig.availableChemistries.forEach(function (chem) {
    const isCurrent = batteryConfig.currentChemistry === chem.chemistry;
    const className = isCurrent ? "font-weight-bold text-primary" : "";
    const marker = isCurrent ? " (current)" : "";

    list.append(`<li class="${className}">
      <small>${chem.chemistry}<br>
      ${chem.minVoltage}V - ${chem.maxVoltage}V per cell${marker}</small>
    </li>`);
  });
}

async function saveBatteryConfig() {
  const selectedChemistry = $("#chemistrySelect").val();
  const cellCountStr = $("#cellCountInput").val();
  const cellCount = cellCountStr ? parseInt(cellCountStr) : 0;

  if (!selectedChemistry && !cellCount) {
    showBatteryConfigStatus("Please select a chemistry or enter cell count", "warning");
    return;
  }

  if (cellCount && (cellCount < 1 || cellCount > 24)) {
    showBatteryConfigStatus("Cell count must be between 1 and 24", "warning");
    return;
  }

  try {
    showBatteryConfigStatus("Saving battery configuration...", "info");

    const body = {};
    if (selectedChemistry) body.chemistry = selectedChemistry;
    if (cellCount) body.cellCount = cellCount;

    const response = await fetch("/api/battery/config", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": "Basic " + btoa("admin:feathers")
      },
      body: JSON.stringify(body)
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Failed to save battery configuration: ${errorText}`);
    }

    const result = await response.json();
    console.log("Battery configuration saved:", result);

    let message = "Battery configuration saved: ";
    if (selectedChemistry) message += `Chemistry: ${selectedChemistry}`;
    if (selectedChemistry && cellCount) message += ", ";
    if (cellCount) message += `Cell count: ${cellCount}`;

    showBatteryConfigStatus(message, "success");

    updateDisplayWithManualConfig(selectedChemistry, cellCount);

    manualConfigActive = true;

    showBatteryConfigStatus(message + " - Updating battery reading...", "info");

    await getBatteryConfig();
    await new Promise(resolve => setTimeout(resolve, 1000));
    await getState();

    showBatteryConfigStatus(message + " - Battery reading updated!", "success");

  } catch (e) {
    console.log("Error saving battery configuration:", e);
    showBatteryConfigStatus(`Error: ${e.message}`, "danger");
  }
}

async function clearBatteryConfig() {
  try {
    showBatteryConfigStatus("Clearing manual configuration...", "info");

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

    showBatteryConfigStatus("Manual configuration cleared. Using auto-detection.", "success");

    clearManualConfigDisplay();

    manualConfigActive = false;

    showBatteryConfigStatus("Manual configuration cleared - Updating battery reading...", "info");

    $("#chemistrySelect").val("");
    $("#cellCountInput").val("");
    await getBatteryConfig();
    await new Promise(resolve => setTimeout(resolve, 1000));

    await getState();

    showBatteryConfigStatus("Manual configuration cleared - Battery reading updated!", "success");

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

function updateVoltageRange(chemistry, cellCount) {
  if (!batteryConfig || !batteryConfig.availableChemistries || !chemistry || !cellCount) {
    $("#batteryVoltageRange").html("N/A");
    return;
  }

  const chemProfile = batteryConfig.availableChemistries.find(chem => chem.chemistry === chemistry);
  if (!chemProfile) {
    $("#batteryVoltageRange").html("Unknown chemistry");
    return;
  }

  const minVoltage = (chemProfile.minVoltage * cellCount).toFixed(1);
  const maxVoltage = (chemProfile.maxVoltage * cellCount).toFixed(1);

  $("#batteryVoltageRange").html(`${minVoltage}V - ${maxVoltage}V`);
}

function updateDisplayWithManualConfig(chemistry, cellCount) {
  const currentChemistry = chemistry || (batteryConfig ? batteryConfig.currentChemistry : null);
  const currentCellCount = cellCount || (batteryConfig ? batteryConfig.currentCellCount : null);

  if (chemistry) {
    $("#batteryChemistry").html(chemistry);
  }

  if (cellCount && cellCount > 0) {
    $("#batteryCellCount").html(cellCount + " cells");
  }

  if (currentChemistry && currentCellCount && currentCellCount > 0) {
    updateVoltageRange(currentChemistry, currentCellCount);
  }
}

function clearManualConfigDisplay() {
  $("#batteryChemistry").html("Auto-detecting...");
  $("#batteryCellCount").html("Auto-detecting...");
  $("#batteryVoltageRange").html("Auto-detecting...");
}

function downloadBatteryCsv() {
  window.location.href = "/battery-csv";
}
