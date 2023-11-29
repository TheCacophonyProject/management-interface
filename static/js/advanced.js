"use strict";
window.onload = function () {
  checkType();
};

async function checkType() {
  try {
    var deviceInfo = await apiGetJSON("/api/device-info");
    if (deviceInfo.type == "tc2") {
      document.getElementById("tc2-advanced").style.display = "block";
    }
  } catch (e) {
    console.log(e);
  }
}