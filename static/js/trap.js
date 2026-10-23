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
  await loadTrapPageConfig();
};

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
  ["program", "#input-trap-program"],
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

  if (await saveConfig("trap", { config: trapConfig })) {
    await loadTrapPageConfig();
  }
}
