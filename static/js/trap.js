"use strict";

// Ask the trap to restart. This can take a few seconds as the request is sent
// to the trap over UART, which could be busy sending other messages.
async function restartTrap() {
  const button = document.getElementById("restart-trap-button");
  const message = document.getElementById("restart-trap-message");
  button.disabled = true;
  message.className = "col-12 mt-3";
  message.innerText = "Restarting trap...";
  try {
    await apiFormURLEncodedPost("/api/trap/restart", {});
    message.className = "col-12 mt-3 text-success";
    message.innerText = "Trap restarted.";
  } catch (e) {
    console.log(e);
    message.className = "col-12 mt-3 text-danger";
    message.innerText = "Failed to restart trap: " + e;
  } finally {
    button.disabled = false;
  }
}
