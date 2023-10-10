"use strict";
window.onload = function () {
  checkType();
};

async function checkType() {
  //TODO Check if device is TC2, just hard coded for now.
  document.getElementById("tc2-advanced").style.display = "block";
  return
  try {
    var deviceInfo = await apiGetJSON("/api/device-info");
    if (deviceInfo.type == "tc2") {
      
    }
  } catch (e) {
    console.log(e);
  }
}