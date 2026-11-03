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
  trapRequestRunning = true;
  updateTrapButtons();
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
    trapRequestRunning = false;
    updateTrapButtons();
    loadTrapHistory();
  }
}

let trapRequestRunning = false;

// The cage phase each cage button can be used in. The trap refuses them at other times.
const cageButtonPhases = {
  "cage-hold-button": "HOLDING",
  "cage-release-button": "HOLDING",
  "cage-kill-button": "HOLDING",
  "cage-wait-button": "EXITING",
  "cage-escaped-button": "EXITING",
  "cage-killed-button": "EXITING",
};

// The cage buttons that need the kill door, hidden for a cage with only the spool door.
const cageKillButtons = ["cage-kill-button", "cage-killed-button"];

// How many doors the trap's cage has (cage_doors in the trap config), 0 without a cage.
let cageDoors = 0;

// updateTrapButtons disables the buttons while a request is being sent or the camera is
// still connecting to the trap, and the cage buttons that don't apply to what the cage is
// doing. Without a status from the trap (such as older trap software) the cage buttons are
// left for the trap to refuse.
function updateTrapButtons() {
  const status = trapStatus && trapStatus.status;
  const connecting = Boolean(status && status.Connection);
  for (const id of trapButtons) {
    let disabled = trapRequestRunning || connecting;
    if (id in cageButtonPhases && status) {
      disabled = disabled || status.CagePhase !== cageButtonPhases[id];
    }
    document.getElementById(id).disabled = disabled;
  }
  // The cage buttons are only shown for a trap with a cage, and the kill ones only for a
  // cage with the kill door.
  document.getElementById("cage-section").style.display = cageDoors === 0 ? "none" : "";
  for (const id of cageKillButtons) {
    document.getElementById(id).style.display = cageDoors < 2 ? "none" : "";
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
  streamTrapStatus();
  await Promise.all([loadTrapPageConfig(), loadTrapHistory()]);
};

const trapStatusRetry = 2000;

// The trap's states (State in its status).
const trapStates = {
  RESETTING: "Resetting the spool",
  RESET_FAILED: "Spool reset failed, waiting to be reset or restarted",
  ARMED: "Armed",
  TRIGGERED: "Triggered",
  DRY_TRIGGERED: "Would have triggered (Observation Mode)",
  CAGE: "Running the cage sequence",
  MANUAL: "Manual mode",
};

// The cage phases (CagePhase in the status), for a trap with a cage.
const cagePhases = {
  IDLE: "Empty",
  WAITING_FOR_ENTRY: "Waiting for the animal to go in",
  MOVING: "Moving a door",
  HOLDING: "Holding an animal",
  EXITING: "Door open, waiting for the animal to leave",
  MANUAL: "Stopped, the trap is in manual mode",
};

// What the camera is doing to connect to the trap (Connection in the status), before the
// trap's own status can be shown. STARTING is while tc2-hat-comms is starting up, such as
// after a config change.
const trapConnectionStages = {
  STARTING: "Starting trap comms",
  INITIALIZING: "Starting trap comms",
  PINGING: "Connecting to the trap (waiting for it to answer)",
  CHECKING_SOFTWARE: "Checking the trap's software is up to date",
  UPDATING_SOFTWARE: "Updating the trap's software",
  CHECKING_CONFIG: "Checking the trap's config is up to date",
  UPDATING_CONFIG: "Updating the trap's config",
  RESTARTING: "Restarting the trap",
};

// trapStatusRows turns the trap's status into [label, value] rows.
function trapStatusRows(status) {
  if (status.Connection) {
    return [["State", trapConnectionStages[status.Connection] || status.Connection]];
  }
  const rows = [];
  rows.push(["State", trapStates[status.State] || status.State || "Unknown (this program doesn't run the trap sequence)"]);
  if (status.ObservationMode) {
    rows.push(["Observation Mode", "On, motion won't trigger the trap"]);
  }
  if (status.Enabled !== undefined) {
    rows.push(["Enabled", status.Enabled ? "Yes" : "No: " + (status.DisabledReason || "")]);
  }
  if (status.Request) {
    rows.push(["Running", trapMessageNames[status.Request] || status.Request]);
  }
  if (status.CagePhase) {
    rows.push(["Cage", cagePhases[status.CagePhase] || status.CagePhase]);
  }
  if (status.CameraEnabled !== undefined) {
    rows.push(["Camera", status.CameraEnabled ? "Enabling the trap" : "Not enabling the trap"]);
  }
  return rows;
}

// The last status from the trap, and when it came, or null if there isn't one.
let trapStatus = null;
// Why there isn't a status, shown instead of one.
let trapStatusProblem = "Connecting...";

// streamTrapStatus reads the trap's status from the camera as server-sent events: the
// current status, then each change as the trap reports it. EventSource can't send the
// Authorization header, so the stream is read with fetch. It reconnects if it drops.
async function streamTrapStatus() {
  for (;;) {
    try {
      const response = await fetch("/api/trap/status/events", {
        headers: {
          Authorization: "Basic " + btoa("admin:feathers"),
        },
      });
      if (!response.ok) {
        throw new Error(await response.text());
      }
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      for (;;) {
        const { value, done } = await reader.read();
        if (done) {
          break;
        }
        buffer += decoder.decode(value, { stream: true });
        let end;
        while ((end = buffer.indexOf("\n\n")) >= 0) {
          handleTrapStatusEvent(buffer.slice(0, end));
          buffer = buffer.slice(end + 2);
        }
      }
      throw new Error("the connection closed");
    } catch (e) {
      console.log(e);
      trapStatus = null;
      trapStatusProblem = "Lost the connection to the camera, reconnecting...";
      renderTrapStatus();
    }
    await new Promise((resolve) => setTimeout(resolve, trapStatusRetry));
  }
}

// handleTrapStatusEvent handles one event from the status stream. Comment lines, which
// keep the connection alive, are ignored.
function handleTrapStatusEvent(text) {
  let name = "message";
  const data = [];
  for (const line of text.split("\n")) {
    if (line.startsWith("event: ")) {
      name = line.slice("event: ".length);
    } else if (line.startsWith("data: ")) {
      data.push(line.slice("data: ".length));
    }
  }
  if (name === "status") {
    trapStatus = { status: JSON.parse(data.join("\n")), receivedAt: Date.now() };
    // Whatever changed the status is in the history too.
    loadTrapHistory();
  } else if (name === "unavailable") {
    trapStatus = null;
    trapStatusProblem = "Couldn't get the trap's status: " + data.join("\n");
  } else {
    return;
  }
  renderTrapStatus();
}

// renderTrapStatus shows the last status.
function renderTrapStatus() {
  const message = document.querySelector("#trap-status-message");
  const tbody = document.querySelector("#trap-status");
  tbody.innerHTML = "";
  updateTrapButtons();
  if (!trapStatus) {
    message.className = "text-danger";
    message.innerText = trapStatusProblem;
    return;
  }
  for (const [label, value] of trapStatusRows(trapStatus.status)) {
    const row = document.createElement("tr");
    for (const text of [label, value]) {
      const cell = document.createElement("td");
      cell.textContent = text;
      row.appendChild(cell);
    }
    tbody.appendChild(row);
  }
  message.className = "text-success";
  message.innerText = "Live, last changed at " + new Date(trapStatus.receivedAt).toLocaleTimeString();
}

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
  STATUS: "Get status",
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
  STATUS_R: "Status",
  STATUS_CHANGED: "Status changed",
  // Sent by the trap on its own.
  RUNNING: "Started program",
  ENABLED: "Enabled",
  DISABLED: "Disabled",
  MOTION: "Motion",
  MAX_MOTION: "Too much motion",
  TRIGGERED: "Triggered",
  DRY_TRIGGERED: "Dry trigger",
  SPOOL_RESET: "Spool reset",
  SPOOL_RESET_FAILED: "Spool reset failed",
  RTC_BATTERY_FLAT: "RTC battery flat",
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

let lastTrapHistory = null;

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
    const text = await response.text();
    if (text === lastTrapHistory) {
      return;
    }
    lastTrapHistory = text;
    const entries = JSON.parse(text);
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
const defaultTrapProgram = 1;

// The programs in trapPrograms that run the trap sequence, triggering from the PIRs, and of
// those the ones that talk to the camera over UART (program 2 is only enabled by a pin).
const pirPrograms = [1, 2, 3];
const uartPrograms = [1, 3];

// When each settings row is shown, given the program and cage picked on the page. For a
// program that isn't in trapPrograms (a development one) every row is shown.
const trapSettingRows = {
  "row-trap-cage-doors": (t) => t.pir,
  "row-comms-trap-default": (t) => t.ai,
  "row-comms-trap-species": (t) => t.ai,
  "row-comms-trap-duration": (t) => t.ai,
  "row-comms-protect-species": (t) => t.ai,
  "row-comms-protect-duration": (t) => t.ai,
  // With a cage the cage sequence decides when the spool is reset, not the delay.
  "row-trap-spool-reset-delay": (t) => t.pir && t.cageDoors === 0,
  "row-trap-cage-entry-timeout": (t) => t.pir && t.cageDoors > 0,
  "row-trap-cage-hold-hours": (t) => t.pir && t.cageDoors > 0,
  "row-trap-cage-exit-wait-hours": (t) => t.pir && t.cageDoors > 0,
  // Only the programs that check the night only switch use the location.
  "row-trap-latitude": (t) => t.uart,
  "row-trap-longitude": (t) => t.uart,
  "row-trap-observation-mode": (t) => t.pir,
  "row-trap-apir-d-threshold": (t) => t.pir,
  "row-trap-apir-dt-threshold": (t) => t.pir,
  // The cage's reed switches are SW1 for door 1 and SW2 for door 2, so a switch the cage
  // uses can't also disable the trap.
  "row-trap-switch-1-disable": (t) => t.pir && t.cageDoors < 1,
  "row-trap-switch-2-disable": (t) => t.pir && t.cageDoors < 2,
  "row-trap-switch-logic": (t) => t.pir && t.cageDoors === 0,
  "row-trap-motion-message-gap": (t) => t.uart,
  "row-trap-post-reset-cooldown": (t) => t.pir,
  "row-trap-manual-mode-timeout": (t) => t.uart,
  // Only program 9, a development program, loops on an interval.
  "row-trap-test-loop-interval": () => false,
};

// Species labels from the classifier, for the species lists: [label, name shown].
const knownSpecies = [
  ["possum", "Possum"],
  ["rodent", "Rodent (rat, mouse)"],
  ["mustelid", "Mustelid (stoat, ferret, weasel)"],
  ["cat", "Cat"],
  ["hedgehog", "Hedgehog"],
  ["leporidae", "Rabbit or hare"],
  ["wallaby", "Wallaby"],
  ["deer", "Deer"],
  ["dog", "Dog"],
  ["sheep", "Sheep"],
  ["bird", "Bird"],
  ["kiwi", "Kiwi"],
  ["penguin", "Penguin"],
  ["human", "Human"],
];
const otherSpecies = "other";

// Whether the camera is in low power mode, where AI Mode can't be used.
let lowPowerMode = false;

// The comms and trap config as loaded, as JSON, so saving only writes the parts that changed.
let loadedTrapSettings = { comms: "", trap: "" };

// pickedTrapProgram returns the program picked on the page, or the default if none is.
function pickedTrapProgram() {
  const value = document.querySelector("#input-trap-program").value;
  return value === "" ? defaultTrapProgram : parseInt(value);
}

// updateTrapSettingRows shows only the settings rows that apply to the program and cage
// picked on the page, and warns about AI Mode in low power mode.
function updateTrapSettingRows() {
  const program = pickedTrapProgram();
  const known = trapPrograms.some(([n]) => n === program);
  const trap = {
    pir: pirPrograms.includes(program),
    uart: uartPrograms.includes(program),
    ai: program === aiModeProgram,
    cageDoors: parseInt(document.querySelector("#input-trap-cage-doors").value),
  };
  for (const [id, shown] of Object.entries(trapSettingRows)) {
    document.getElementById(id).style.display = !known || shown(trap) ? "" : "none";
  }

  document.querySelector("#program-warning").innerText = lowPowerMode && program === aiModeProgram
    ? "The camera is in low power mode, so it can't enable the trap in AI Mode. Pick another program, or put the camera in high power mode."
    : "";
}

// setProgramOptions fills the program select and selects the saved program. A saved number
// that isn't in trapPrograms (e.g. a development program) still gets an option, so saving
// doesn't change it. In low power mode AI Mode can't be picked.
function setProgramOptions(program) {
  const select = document.querySelector("#input-trap-program");
  const options = [["", `Default (${trapPrograms.find(([n]) => n === defaultTrapProgram)[1]})`]];
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
}

// setSpeciesList fills a species list ("trap" or "protect") from the comms config.
function setSpeciesList(kind, species) {
  document.getElementById(`${kind}-species-list`).innerHTML = "";
  for (const [label, confidence] of Object.entries(species || {})) {
    addSpeciesRow(kind, label, confidence);
  }
}

// addSpeciesRow adds a species and its confidence to a species list. Without a label it
// picks the first known species that isn't in the list yet. A label that isn't in
// knownSpecies gets an option of its own, and "Other" lets one be typed in.
function addSpeciesRow(kind, label, confidence) {
  const list = document.getElementById(`${kind}-species-list`);
  if (label === undefined) {
    const used = readSpeciesRows(kind).map((row) => row.label);
    label = (knownSpecies.find(([l]) => !used.includes(l)) || [otherSpecies])[0];
  }

  const row = document.createElement("div");
  // On a narrow screen the species gets a line of its own, above the confidence.
  row.className = "d-flex flex-wrap align-items-center mb-1";
  const select = document.createElement("select");
  select.className = "form-control form-control-sm mr-2 mb-1";
  select.style.flex = "1 1 9em";
  const options = [...knownSpecies];
  if (label !== otherSpecies && !knownSpecies.some(([l]) => l === label)) {
    options.push([label, label]);
  }
  options.push([otherSpecies, "Other..."]);
  for (const [value, text] of options) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = text;
    select.appendChild(option);
  }
  select.value = label;

  const other = document.createElement("input");
  other.type = "text";
  other.className = "form-control form-control-sm mr-2 mb-1";
  other.placeholder = "Classifier label";
  other.style.flex = "1 1 9em";
  other.style.display = label === otherSpecies ? "" : "none";
  select.onchange = () => {
    other.style.display = select.value === otherSpecies ? "" : "none";
  };

  // The confidence, with a % after it.
  const confGroup = document.createElement("div");
  confGroup.className = "input-group input-group-sm mr-2 mb-1";
  confGroup.style.width = "6em";
  confGroup.style.flex = "none";
  const conf = document.createElement("input");
  conf.type = "number";
  conf.min = "0";
  conf.max = "100";
  conf.step = "1";
  conf.className = "form-control";
  conf.title = "Confidence";
  conf.value = confidence === undefined ? "" : String(confidence);
  const percent = document.createElement("div");
  percent.className = "input-group-append";
  percent.innerHTML = '<span class="input-group-text">%</span>';
  confGroup.append(conf, percent);

  const remove = document.createElement("button");
  remove.type = "button";
  remove.className = "btn btn-sm btn-outline-danger mb-1";
  remove.title = "Remove";
  remove.textContent = "\u2715";
  remove.onclick = () => row.remove();

  row.speciesInputs = { select, other, conf };
  row.append(select, other, confGroup, remove);
  list.appendChild(row);
}

// readSpeciesRows returns what is in a species list, as [{label, confidence}], with the
// confidence as typed in.
function readSpeciesRows(kind) {
  return [...document.getElementById(`${kind}-species-list`).children].map((row) => {
    const { select, other, conf } = row.speciesInputs;
    const label = select.value === otherSpecies ? other.value.trim().toLowerCase() : select.value;
    return { label, confidence: conf.value };
  });
}

// readSpecies returns a species list as the comms config has it, {label: confidence}, or
// throws an error saying what is wrong with it.
function readSpecies(kind) {
  const species = {};
  for (const { label, confidence } of readSpeciesRows(kind)) {
    if (label === "") {
      throw new Error(`Type in the classifier label for the "Other" species to ${kind}.`);
    }
    if (label in species) {
      throw new Error(`${label} is in the species to ${kind} more than once.`);
    }
    const n = Number(confidence);
    if (confidence === "" || !Number.isInteger(n) || n < 0 || n > 100) {
      throw new Error(`Enter a confidence from 0 to 100 for ${label} in the species to ${kind}.`);
    }
    species[label] = n;
  }
  return species;
}

// The durations are shown in minutes and saved as a Go duration string.
function durationMinutes(nanoseconds) {
  return nanoseconds ? String(nanoseconds / 60e9) : "";
}

function minutesDuration(selector) {
  const minutes = document.querySelector(selector).value;
  return minutes === "" ? undefined : `${minutes}m`;
}

// commsSettings returns the trap's part of the comms config as set on the page. Empty
// durations are left out, so they keep their saved value.
function commsSettings() {
  return {
    "trap-enabled-by-default": document.querySelector("#input-comms-trap-default").checked,
    "trap-species": readSpecies("trap"),
    "trap-duration": minutesDuration("#input-comms-trap-duration"),
    "protect-species": readSpecies("protect"),
    "protect-duration": minutesDuration("#input-comms-protect-duration"),
  };
}

// trapConfigSettings returns the whole trap config as set on the page. Empty number fields
// are left out so the trap uses its default for them. Hidden fields are kept as they are.
function trapConfigSettings() {
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
  trapConfig.cage_doors = parseInt(document.querySelector("#input-trap-cage-doors").value);
  return trapConfig;
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
    setSpeciesList("trap", comms.TrapSpecies);
    document.querySelector("#input-comms-trap-duration").value = durationMinutes(comms.TrapDuration);
    document.querySelector("#input-comms-trap-duration").placeholder = durationMinutes(data.defaults.comms.TrapDuration);
    setSpeciesList("protect", comms.ProtectSpecies);
    document.querySelector("#input-comms-protect-duration").value = durationMinutes(comms.ProtectDuration);
    document.querySelector("#input-comms-protect-duration").placeholder = durationMinutes(data.defaults.comms.ProtectDuration);

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
    lowPowerMode = data.values.thermalRecorder?.UseLowPowerMode || false;
    setProgramOptions(trapConfig.program);
    cageDoors = trapConfig.cage_doors || 0;
    document.querySelector("#input-trap-cage-doors").value = String(cageDoors);
    updateTrapSettingRows();
    updateTrapButtons();

    loadedTrapSettings = { comms: "", trap: JSON.stringify(trapConfigSettings()) };
    try {
      loadedTrapSettings.comms = JSON.stringify(commsSettings());
    } catch {
      // A saved species list the page can't read back counts as changed.
    }
  } catch (error) {
    console.error("Error loading config:", error);
    alert("Error loading config");
  }
}

// saveTrapSettings saves the trap's part of the comms config and the trap config, each only
// if it has changed. The rest of the comms config is left as it is.
async function saveTrapSettings() {
  let comms;
  try {
    comms = commsSettings();
  } catch (error) {
    alert(error.message);
    return;
  }
  const trap = trapConfigSettings();
  const commsChanged = JSON.stringify(comms) !== loadedTrapSettings.comms;
  const trapChanged = JSON.stringify(trap) !== loadedTrapSettings.trap;
  if (!commsChanged && !trapChanged) {
    alert("Nothing to save, the trap settings haven't changed.");
    return;
  }

  let saved = true;
  if (trapChanged) {
    saved = (await saveConfig("trap", { config: trap })) && saved;
  }
  if (commsChanged) {
    saved = (await saveConfig("comms", comms)) && saved;
  }
  if (saved) {
    await loadTrapPageConfig();
  }
}
