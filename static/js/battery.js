"use strict";
window.onload = function () {
  getState();
  setInterval(getState, 5000); 
};

async function getState() {
  try {
    var response = await apiGetJSON("/api/battery");
    console.log(response);

    $("#time").html(response.time);
    $("#mainBattery").html(response.mainBattery);
    $("#rtcBattery").html(response.rtcBattery);

  } catch (e) {
    console.log(e);
  }
}

function downloadBatteryCsv() {
  window.location.href = "/battery-csv";
}
