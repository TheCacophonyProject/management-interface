"use strict";
window.onload = function () {
  getState();
  setInterval(getState, 5000); 
};

async function getState() {
  try {
    var response = await apiGetJSON("/api/modem");
    console.log(response);

    $("#timestamp").html(response.timestamp);
    $("#onOffReason").html(response.onOffReason);
    $("#powered").html(response.powered ? 'True' : 'False');
    if (typeof(response.GPS) == 'string') {
      $("#noGPS").show();
      $("#gpsData").hide();
      $("#noGPSReason").html(response.GPS);
    } else {
      $("#noGPS").hide();
      $("#gpsData").show();
      $("#gpsLatitude").html(response.GPS.latitude)
      $("#gpsLongitude").html(response.GPS.longitude)
      $("#gpsAltitude").html(response.GPS.altitude)
      $("#gpsUTCTime").html(response.GPS.utcDateTime)
      $("#gpsCourse").html(response.GPS.course)
      $("#gpsSpeed").html(response.GPS.speed)
    }

    $("#connectedTime").html(response.modem.connectedTime);
    $("#manufacturer").html(response.modem.manufacturer);
    $("#model").html(response.modem.model);
    $("#name").html(response.modem.name);
    $("#netdev").html(response.modem.netdev);
    $("#serial").html(response.modem.serial);
    $("#temp").html(response.modem.temp);
    $("#vendor").html(response.modem.vendor);
    $("#voltage").html(response.modem.voltage);

    $("#band").html(response.signal.band);
    $("#provider").html(response.signal.provider);
    $("#accessTechnology").html(response.signal.accessTechnology);
    $("#signalStrength").html(response.signal.strength);

    $("#ICCID").html(response.simCard.ICCID);
    $("#simProvider").html(response.simCard.provider);
    $("#simCardStatus").html(response.simCard.simCardStatus);

  } catch (e) {
    console.log(e);
  }
}
