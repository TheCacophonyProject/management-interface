"use strict";

const trapButtons = [
  "release-spool-button",
  "reset-spool-button",
  "open-door-1-button",
  "close-door-1-button",
  "open-door-2-button",
  "close-door-2-button",
  "restart-trap-button",
  "cage-hold-button",
  "cage-release-button",
  "cage-kill-button",
  "cage-wait-button",
  "cage-escaped-button",
  "cage-killed-button",
];

// Send a request to the trap. Only one request is sent at a time, as they all go to the
// trap over the same UART. The request returns once the trap has accepted it, not once it
// has finished moving the spool.
async function trapRequest(url, working, done) {
  const message = document.getElementById("trap-message");
  setTrapButtonsDisabled(true);
  message.className = "col-12 mt-3";
  message.innerText = working;
  try {
    await apiFormURLEncodedPost(url, {});
    message.className = "col-12 mt-3 text-success";
    message.innerText = done;
  } catch (e) {
    console.log(e);
    message.className = "col-12 mt-3 text-danger";
    message.innerText = "Request failed: " + e;
  } finally {
    setTrapButtonsDisabled(false);
  }
}

function setTrapButtonsDisabled(disabled) {
  for (const id of trapButtons) {
    document.getElementById(id).disabled = disabled;
  }
}

function releaseSpool() {
  trapRequest(
    "/api/trap/release-spool",
    "Releasing spool...",
    "Spool released. The trap is now in manual mode, restart it to go back to running its sequence."
  );
}

function resetSpool() {
  trapRequest(
    "/api/trap/reset-spool",
    "Resetting spool...",
    "Resetting the spool, this takes a few seconds. The trap is now in manual mode, restart it to go back to running its sequence."
  );
}

// The trap only reports that a door has finished moving with a DOOR_OPENED/DOOR_CLOSED
// message, which doesn't come back over this request, so the door is still moving when
// the button re-enables.
function openDoor(door) {
  trapRequest(
    "/api/trap/door/" + door + "/open",
    "Opening door " + door + "...",
    "Opening door " + door +
      ", this takes about 20 seconds. The trap is now in manual mode, restart it to go back to running its sequence."
  );
}

function closeDoor(door) {
  trapRequest(
    "/api/trap/door/" + door + "/close",
    "Closing door " + door + "...",
    "Door " + door +
      " closed. The trap is now in manual mode, restart it to go back to running its sequence."
  );
}

// Stop is deliberately left out of trapButtons so it stays pressable while another
// request is in flight, as stopping a move part way through is the point of it.
function stopTrap() {
  trapRequest(
    "/api/trap/stop",
    "Stopping...",
    "Stopped. The trap is now in manual mode, restart it to go back to running its sequence."
  );
}

function restartTrap() {
  trapRequest("/api/trap/restart", "Restarting trap...", "Trap restarted.");
}

// Messages for each cage command: [while sending, once the trap has accepted it].
const cageCommandMessages = {
  hold: ["Holding animal...", "Holding the animal for another cage_hold_hours."],
  release: ["Releasing animal...", "Opening door 1 to release the animal, this takes about 20 seconds."],
  kill: ["Opening door to kill mechanism...", "Opening door 2 to the kill mechanism, this takes about 20 seconds."],
  wait: ["Waiting longer...", "Waiting another cage_exit_wait_hours for the animal to leave the cage."],
  escaped: ["Marking as escaped...", "Marked as escaped. The trap will re-arm the cage and reset the spool."],
  killed: ["Marking as killed...", "Marked as killed. The trap will re-arm the cage and reset the spool."],
};

// Commands for an animal in the cage. The trap refuses one that doesn't apply at the
// time, such as Wait Longer while it is still holding the animal, and says why.
function cageCommand(command) {
  if (command === "kill" && !confirm("Open the door to the kill mechanism?")) {
    return;
  }
  const [working, done] = cageCommandMessages[command];
  trapRequest("/api/trap/cage/" + command, working, done);
}

window.onload = async function () {
  await Promise.all([loadTrapPageConfig(), loadTrapHistory()]);
};

// What the messages mean, for the history. Messages not listed show their type.
const trapMessageNames = {
  // Sent by the camera.
  PING: "Ping",
  ENABLE: "Enable",
  DISABLE: "Disable",
  RESTART: "Restart",
  STOP: "Stop",
  RELEASE_SPOOL: "Release spool",
  RESET_SPOOL: "Reset spool",
  OPEN_DOOR_1: "Open door 1",
  OPEN_DOOR_2: "Open door 2",
  CLOSE_DOOR_1: "Close door 1",
  CLOSE_DOOR_2: "Close door 2",
  CAGE_HOLD: "Cage: hold",
  CAGE_RELEASE: "Cage: release",
  CAGE_KILL: "Cage: kill",
  CAGE_WAIT: "Cage: wait longer",
  CAGE_ESCAPED: "Cage: animal escaped",
  CAGE_KILLED: "Cage: animal killed",
  READ_TIME: "Read time",
  WRITE_TIME: "Set time",
  LS: "Check files",
  WRITE: "Upload file chunk",
  DECOMPRESS: "Unpack file",
  DELETE: "Delete file",
  DELETE_TMP: "Delete temporary files",
  COMMIT: "Save uploaded files",
  // Responses from the trap.
  ACK: "Accepted",
  NACK: "Refused",
  BAD_KEY: "Not supported",
  READ_TIME_R: "Time",
  // Sent by the trap on its own.
  RUNNING: "Started program",
  ENABLED: "Enabled",
  DISABLED: "Disabled",
  MOTION: "Motion",
  MAX_MOTION: "Too much motion",
  TRIGGERED: "Triggered",
  SPOOL_RESET: "Spool reset",
  ERROR_CODE: "Error",
  EXCEPTION: "Software error",
  DOOR_OPENED: "Door opened",
  DOOR_CLOSED: "Door closed",
  MANUAL_MODE: "Manual mode",
  STOPPED: "Stopped",
  MANUAL_MODE_TIMEOUT: "Manual mode timed out",
  MANUAL_ERROR: "Manual mode error",
  CAGE_NO_ENTRY: "Cage: nothing entered",
  CAGE_CAPTURED: "Cage: animal captured",
  CAGE_HOLDING: "Cage: holding longer",
  CAGE_RELEASING: "Cage: releasing",
  CAGE_KILLING: "Cage: opening to kill mechanism",
  CAGE_WAITING: "Cage: waiting longer",
  CAGE_FINISHED: "Cage: finished",
};

const trapHistoryDirs = {
  sent: "Sent",
  received: "Received",
  failed: "No response",
  lost: "Lost",
};

async function loadTrapHistory() {
  const message = document.querySelector("#trap-history-message");
  const tbody = document.querySelector("#trap-history");
  try {
    const response = await fetch("/api/trap/history", {
      headers: {
        Authorization: "Basic " + btoa("admin:feathers"),
      },
    });
    if (!response.ok) {
      throw new Error(await response.text());
    }
    const entries = await response.json();
    tbody.innerHTML = "";
    for (const entry of entries) {
      const row = document.createElement("tr");
      const name = trapMessageNames[entry.type];
      const time = new Date(entry.time);
      for (const text of [
        // A lost entry from a line in the history file that couldn't be read has no time.
        time.getFullYear() > 1 ? time.toLocaleString() : "Unknown",
        trapHistoryDirs[entry.dir] || "Received",
        entry.id || "",
        entry.dir === "lost" ? "Line lost" : name ? `${name} (${entry.type})` : entry.type,
        entry.payload || "",
      ]) {
        const cell = document.createElement("td");
        cell.textContent = text;
        row.appendChild(cell);
      }
      tbody.appendChild(row);
    }
    message.innerText = entries.length === 0 ? "No messages from the trap yet." : "";
  } catch (e) {
    message.innerText = "Failed to load history: " + e;
  }
}

// Number fields in the trap config: [config key, input id].
const trapConfigNumbers = [
  ["apir_d_threshold", "#input-trap-apir-d-threshold"],
  ["apir_dt_threshold", "#input-trap-apir-dt-threshold"],
  ["max_current", "#input-trap-max-current"],
  ["spool_reset_delay_minutes", "#input-trap-spool-reset-delay"],
  ["latitude", "#input-trap-latitude"],
  ["longitude", "#input-trap-longitude"],
  ["test_loop_interval", "#input-trap-test-loop-interval"],
  ["motion_message_gap", "#input-trap-motion-message-gap"],
  ["post_reset_cooldown_seconds", "#input-trap-post-reset-cooldown"],
  ["manual_mode_timeout_minutes", "#input-trap-manual-mode-timeout"],
  ["cage_entry_timeout_minutes", "#input-trap-cage-entry-timeout"],
  ["cage_hold_hours", "#input-trap-cage-hold-hours"],
  ["cage_exit_wait_hours", "#input-trap-cage-exit-wait-hours"],
];

// Checkbox fields in the trap config: [config key, input id].
const trapConfigChecks = [
  ["observation_mode", "#input-trap-observation-mode"],
  ["spool_reed_check", "#input-trap-spool-reed-check"],
  ["cage_enabled", "#input-trap-cage-enabled"],
];

// Select fields in the trap config: [config key, input id, default].
const trapConfigSelects = [
  ["switch_1_disable", "#input-trap-switch-1-disable", "IGNORE"],
  ["switch_2_disable", "#input-trap-switch-2-disable", "IGNORE"],
  ["switch_logic", "#input-trap-switch-logic", "OR"],
];

// The program where the camera enables the trap when it sees a target species. The camera
// can only do that in high power mode, where the Raspberry Pi is on and doing the recording.
const aiModeProgram = 3;

// The spool-py programs (src/codeNN.py) users can pick, by number. The trap runs program 1
// if none is set. The other programs are for development.
const trapPrograms = [
  [1, "Non-AI Mode"],
  [2, "Digital Trigger Mode"],
  [3, "AI Mode"],
  [8, "Cycle Testing Mode"],
];

// setProgramOptions fills the program select and selects the saved program. A saved number
// that isn't in trapPrograms (e.g. a development program) still gets an option, so saving
// doesn't change it. In low power mode AI Mode can't be picked.
function setProgramOptions(program, lowPowerMode) {
  const select = document.querySelector("#input-trap-program");
  const options = [["", "Default (Non-AI Mode)"]];
  for (const [n, name] of trapPrograms) {
    options.push([String(n), name]);
  }
  if (program !== undefined && program !== null && !trapPrograms.some(([n]) => n === program)) {
    options.push([String(program), `Program ${program} (development)`]);
  }
  select.innerHTML = "";
  for (const [value, text] of options) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = text;
    if (lowPowerMode && value === String(aiModeProgram)) {
      option.disabled = true;
      option.textContent += " (needs high power mode)";
    }
    select.appendChild(option);
  }
  select.value = program === undefined || program === null ? "" : String(program);

  const warning = document.querySelector("#program-warning");
  warning.innerText = lowPowerMode && program === aiModeProgram
    ? "The camera is in low power mode, so it can't enable the trap in AI Mode. Pick another program, or put the camera in high power mode."
    : "";
}

async function loadTrapPageConfig() {
  try {
    const data = await fetchConfig();
    const comms = data.values.comms;

    // The trap is only talked to when comms is enabled and set to trap-control. tc2-hat-comms
    // also runs trap control for "uart", for sidekick compatibility.
    const warning = document.querySelector("#comms-warning");
    if (!comms.Enable || !["trap-control", "uart"].includes(comms.CommsOut)) {
      warning.innerHTML =
        'Comms needs to be enabled and set to trap-control for the camera to talk to the trap. Change this on the <a href="/config">config</a> page.';
    } else {
      warning.innerText = "";
    }

    document.querySelector("#input-comms-trap-default").checked = comms.TrapEnabledByDefault;
    document.querySelector("#input-comms-trap-species").value = JSON.stringify(comms.TrapSpecies);
    document.querySelector("#input-comms-trap-duration").value = formatDuration(comms.TrapDuration);
    document.querySelector("#input-comms-trap-duration").placeholder = formatDuration(data.defaults.comms.TrapDuration);
    document.querySelector("#input-comms-protect-species").value = JSON.stringify(comms.ProtectSpecies);
    document.querySelector("#input-comms-protect-duration").value = formatDuration(comms.ProtectDuration);
    document.querySelector("#input-comms-protect-duration").placeholder = formatDuration(data.defaults.comms.ProtectDuration);

    const trapConfig = (data.values.trap && data.values.trap.Config) || {};
    for (const [key, selector] of trapConfigNumbers) {
      document.querySelector(selector).value = "";
      setNumberInput(selector, trapConfig[key]);
    }
    for (const [key, selector] of trapConfigChecks) {
      document.querySelector(selector).checked = trapConfig[key] || false;
    }
    for (const [key, selector, fallback] of trapConfigSelects) {
      document.querySelector(selector).value = trapConfig[key] || fallback;
    }
    setProgramOptions(trapConfig.program, data.values.thermalRecorder?.UseLowPowerMode || false);
  } catch (error) {
    console.error("Error loading config:", error);
    alert("Error loading config");
  }
}

// saveTrapSettings saves only the trap's part of the comms config, the rest of the comms
// config is left as it is.
async function saveTrapSettings() {
  let trapSpecies, protectSpecies;
  try {
    trapSpecies = JSON.parse(document.querySelector("#input-comms-trap-species").value) || undefined;
    protectSpecies = JSON.parse(document.querySelector("#input-comms-protect-species").value) || undefined;
  } catch (error) {
    console.error("Error parsing JSON:", error);
    alert("Error parsing JSON for trap or protect species");
    return;
  }

  const data = {
    "trap-enabled-by-default": document.querySelector("#input-comms-trap-default").checked,
    "trap-species": trapSpecies,
    "trap-duration": document.querySelector("#input-comms-trap-duration").value || undefined,
    "protect-species": protectSpecies,
    "protect-duration": document.querySelector("#input-comms-protect-duration").value || undefined,
  };

  if (await saveConfig("comms", data)) {
    await loadTrapPageConfig();
  }
}

// saveTrapConfig saves the whole trap config. Empty number fields are left out so the trap
// uses its default for them.
async function saveTrapConfig() {
  const trapConfig = {};
  for (const [key, selector] of trapConfigNumbers) {
    const val = document.querySelector(selector).value;
    if (val !== "") trapConfig[key] = parseFloat(val);
  }
  for (const [key, selector] of trapConfigChecks) {
    trapConfig[key] = document.querySelector(selector).checked;
  }
  for (const [key, selector] of trapConfigSelects) {
    trapConfig[key] = document.querySelector(selector).value;
  }
  const program = document.querySelector("#input-trap-program").value;
  if (program !== "") trapConfig.program = parseInt(program);

  if (await saveConfig("trap", { config: trapConfig })) {
    await loadTrapPageConfig();
  }
}
