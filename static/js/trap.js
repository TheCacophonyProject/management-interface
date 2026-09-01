"use strict";

const trapButtons = [
  "release-spool-button",
  "reset-spool-button",
  "open-door-1-button",
  "close-door-1-button",
  "open-door-2-button",
  "close-door-2-button",
  "restart-trap-button",
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

function restartTrap() {
  trapRequest("/api/trap/restart", "Restarting trap...", "Trap restarted.");
}
