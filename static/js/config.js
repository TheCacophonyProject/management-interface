"use strict";

window.onload = async function () {
  await loadConfig();
};

async function loadConfig() {
  try {
    const response = await fetch("/api/config", {
      headers: {
        Authorization: "Basic " + btoa("admin:feathers"),
      },
    });

    if (!response.ok) {
      throw new Error("Failed to load config");
    }

    const data = await response.json();
    console.log(data);

    // Set placeholders and values for windows
    document.querySelector("#input-start-recording").placeholder = data.defaults.windows.StartRecording;
    document.querySelector("#input-stop-recording").placeholder = data.defaults.windows.StopRecording;

    document.querySelector("#input-start-recording").value = data.values.windows.StartRecording;
    document.querySelector("#input-stop-recording").value = data.values.windows.StopRecording;

    // Set placeholders and values for modem
    document.querySelector("#input-initial-on-duration").placeholder = formatDuration(data.defaults.modemd.InitialOnDuration);
    document.querySelector("#input-find-modem-timeout").placeholder = formatDuration(data.defaults.modemd.FindModemTimeout);
    document.querySelector("#input-connection-timeout").placeholder = formatDuration(data.defaults.modemd.ConnectionTimeout);

    document.querySelector("#input-initial-on-duration").value = formatDuration(data.values.modemd.InitialOnDuration);
    document.querySelector("#input-find-modem-timeout").value = formatDuration(data.values.modemd.FindModemTimeout);
    document.querySelector("#input-connection-timeout").value = formatDuration(data.values.modemd.ConnectionTimeout);

    // Set values for thermal motion
    document.querySelector("#input-do-tracking").checked = data.values.thermalMotion.DoTracking;
    document.querySelector("#input-run-classifier").checked = data.values.thermalMotion.RunClassifier;
    document.querySelector("#input-tracking-events").checked = data.values.thermalMotion.TrackingEvents;
    document.querySelector("#input-postprocess").checked = data.values.thermalMotion.PostProcess;
    document.querySelector("#input-postprocess-events").checked = data.values.thermalMotion.PostProcessEvents;

    // Set values for comms
    document.querySelector("#input-comms-enable").checked = data.values.comms.Enable;
    document.querySelector("#input-comms-trap-default").checked = data.values.comms.TrapEnabledByDefault;
    document.querySelector("#input-comms-bluetooth").checked = data.values.comms.Bluetooth;
    document.querySelector("#input-comms-power-output").value = data.values.comms.PowerOutput;
    document.querySelector("#input-comms-power-up-duration").value = formatDuration(data.values.comms.PowerUpDuration);
    document.querySelector("#input-comms-power-up-duration").placeholder = formatDuration(data.defaults.comms.PowerUpDuration);
    document.querySelector("#input-comms-type-select").value = data.values.comms.CommsOut;
    document.querySelector("#input-comms-trap-species").value = JSON.stringify(data.values.comms.TrapSpecies);
    document.querySelector("#input-comms-trap-duration").value = formatDuration(data.values.comms.TrapDuration);
    document.querySelector("#input-comms-trap-duration").placeholder = formatDuration(data.defaults.comms.TrapDuration);
    document.querySelector("#input-comms-protect-species").value = JSON.stringify(data.values.comms.ProtectSpecies);
    document.querySelector("#input-comms-protect-duration").value = formatDuration(data.values.comms.ProtectDuration);
    document.querySelector("#input-comms-protect-duration").placeholder = formatDuration(data.defaults.comms.ProtectDuration);

    // Set values for trap config
    const trapConfig = (data.values.trap && data.values.trap.Config) || {};
    setNumberInput("#input-trap-apir-d-threshold", trapConfig.apir_d_threshold);
    setNumberInput("#input-trap-apir-dt-threshold", trapConfig.apir_dt_threshold);
    setNumberInput("#input-trap-max-current", trapConfig.max_current);
    setNumberInput("#input-trap-spool-reset-delay", trapConfig.spool_reset_delay_minutes);
    setNumberInput("#input-trap-latitude", trapConfig.latitude);
    setNumberInput("#input-trap-longitude", trapConfig.longitude);
    setNumberInput("#input-trap-test-loop-interval", trapConfig.test_loop_interval);
    document.querySelector("#input-trap-switch-1-disable").value = trapConfig.switch_1_disable || "IGNORE";
    document.querySelector("#input-trap-switch-2-disable").value = trapConfig.switch_2_disable || "IGNORE";
    document.querySelector("#input-trap-switch-logic").value = trapConfig.switch_logic || "OR";
    document.querySelector("#input-trap-observation-mode").checked = trapConfig.observation_mode || false;
    setNumberInput("#input-trap-motion-message-gap", trapConfig.motion_message_gap);
    setNumberInput("#input-trap-post-reset-cooldown", trapConfig.post_reset_cooldown_seconds);
    document.querySelector("#input-trap-spool-reed-check").checked = trapConfig.spool_reed_check || false;
    setNumberInput("#input-trap-program", trapConfig.program);

  } catch (error) {
    console.error("Error loading config:", error);
    alert("Error loading config");
  }
}

function setNumberInput(selector, value) {
  if (value !== undefined && value !== null) {
    document.querySelector(selector).value = value;
  }
}

function formatDuration(nanoseconds) {
  if (!nanoseconds) {
    return "";
  }
  const seconds = Math.floor(nanoseconds / 1_000_000_000);
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const remainingSeconds = seconds % 60;

  return `${hours > 0 ? `${hours}h` : ""}${minutes > 0 ? `${minutes}m` : ""}${remainingSeconds}s`;
}

async function saveWindowsConfig() {
  const data = {
    "start-recording": document.querySelector("#input-start-recording").value || undefined,
    "stop-recording": document.querySelector("#input-stop-recording").value || undefined,
  };

  await saveConfig("windows", data);
}

async function saveModemConfig() {
  const data = {
    "initial-on-duration": document.querySelector("#input-initial-on-duration").value || undefined,
    "find-modem-timeout": document.querySelector("#input-find-modem-timeout").value || undefined,
    "connection-timeout": document.querySelector("#input-connection-timeout").value || undefined,
  };

  await saveConfig("modemd", data);
}

async function saveThermalMotionConfig() {
  const data = {
    "do-tracking": document.querySelector("#input-do-tracking").checked,
    "run-classifier": document.querySelector("#input-run-classifier").checked,
    "tracking-events": document.querySelector("#input-tracking-events").checked,
    "postprocess": document.querySelector("#input-postprocess").checked,
    "postprocess-events": document.querySelector("#input-postprocess-events").checked,
  };

  await saveConfig("thermal-motion", data);
}

async function saveCommsConfig() {

  try {
    var protectSpecies = JSON.parse(document.querySelector("#input-comms-protect-species").value) || undefined;
    console.log("protectSpecies", protectSpecies);
    var trapSpecies = JSON.parse(document.querySelector("#input-comms-trap-species").value) || undefined;
    console.log("TrapSpecies", trapSpecies);
  } catch (error) {
    console.error("Error parsing JSON:", error);
    alert("Error parsing JSON for trap or protect species");
    return;
  }

  const data = {
    "enable": document.querySelector("#input-comms-enable").checked,
    "trap-enabled-by-default": document.querySelector("#input-comms-trap-default").checked,
    "comms-out": document.querySelector("#input-comms-type-select").value || undefined,
    "bluetooth": document.querySelector("#input-comms-bluetooth").checked,
    "power-output": document.querySelector("#input-comms-power-output").value || undefined,
    "power-up-duration": document.querySelector("#input-comms-power-up-duration").value || undefined,
    "trap-species": trapSpecies,
    "trap-duration": document.querySelector("#input-comms-trap-duration").value || undefined,
    "protect-species": protectSpecies,
    "protect-duration": document.querySelector("#input-comms-protect-duration").value || undefined,
  };

  await saveConfig("comms", data);
}

async function saveTrapConfig() {
  const trapConfig = {};

  function addNumber(key, selector) {
    const val = document.querySelector(selector).value;
    if (val !== "") trapConfig[key] = parseFloat(val);
  }

  addNumber("apir_d_threshold", "#input-trap-apir-d-threshold");
  addNumber("apir_dt_threshold", "#input-trap-apir-dt-threshold");
  addNumber("max_current", "#input-trap-max-current");
  addNumber("spool_reset_delay_minutes", "#input-trap-spool-reset-delay");
  addNumber("latitude", "#input-trap-latitude");
  addNumber("longitude", "#input-trap-longitude");
  addNumber("test_loop_interval", "#input-trap-test-loop-interval");
  trapConfig["switch_1_disable"] = document.querySelector("#input-trap-switch-1-disable").value;
  trapConfig["switch_2_disable"] = document.querySelector("#input-trap-switch-2-disable").value;
  trapConfig["switch_logic"] = document.querySelector("#input-trap-switch-logic").value;
  trapConfig["observation_mode"] = document.querySelector("#input-trap-observation-mode").checked;
  addNumber("motion_message_gap", "#input-trap-motion-message-gap");
  addNumber("post_reset_cooldown_seconds", "#input-trap-post-reset-cooldown");
  trapConfig["spool_reed_check"] = document.querySelector("#input-trap-spool-reed-check").checked;
  addNumber("program", "#input-trap-program");

  await saveConfig("trap", { "config": trapConfig });
}

async function saveConfig(section, data) {
  const formData = new FormData();
  formData.append("section", section);
  formData.append("config", JSON.stringify(data));

  try {
    const response = await fetch("/api/config", {
      method: "POST",
      headers: {
        Authorization: "Basic " + btoa("admin:feathers"),
      },
      body: formData,
    });

    if (response.ok) {
      alert(`${section} config saved`);
      await loadConfig();
    } else {
      throw new Error("Failed to save config");
    }
  } catch (error) {
    console.error("Error saving config:", error);
    alert("Error saving config");
  }
}
