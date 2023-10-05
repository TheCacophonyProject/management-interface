"use strict";
window.onload = function () {
  getState();
  setInterval(getState, 5000); 
};

async function getState() {
  try {
    var response = await apiGetJSON("/api/modem");
    console.log(response);

    $("#GPS").html(response.GPS);
    $("#band").html(response.band);
    $("#connectedTime").html(response.connectedTime);
    $("#name").html(response.name);
    $("#netdev").html(response.netdev);
    $("#onOffReason").html(response.onOffReason);
    $("#powered").html(response.powered ? 'True' : 'False');
    $("#signalStrength").html(response.signalStrength);
    $("#simCardStatus").html(response.simCardStatus);
    $("#time").html(response.time);
    $("#vendor").html(response.vendor);

  } catch (e) {
    console.log(e);
  }
}
