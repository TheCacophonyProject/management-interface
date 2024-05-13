"use strict";
window.onload = function () {
  getState();
  setInterval(getState, 5000); 
};

async function getState() {
  try {
    var response = await apiGetJSON("/api/modem");
    //console.log(response);

    $("#timestamp").html(response.timestamp);
    $("#onOffReason").html(response.onOffReason);
    $("#powered").html(response.powered ? 'True' : 'False');
    

    if (response.modem != null) {
      // No GPS data at the moment
      $("#gpsData").hide();
      $("#noGPS").hide();
      /*
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
      */
      $("#modemData").show();
      $("#connectedTime").html(response.modem.connectedTime);
      $("#manufacturer").html(response.modem.manufacturer);
      $("#model").html(response.modem.model);
      $("#name").html(response.modem.name);
      $("#netdev").html(response.modem.netdev);
      $("#serial").html(response.modem.serial);
      $("#temp").html(response.modem.temp);
      $("#vendor").html(response.modem.vendor);
      $("#voltage").html(response.modem.voltage);
      $("#apn").html(response.modem.apn);

      $("#band").html(response.signal.band);
      $("#provider").html(response.signal.provider);
      $("#accessTechnology").html(response.signal.accessTechnology);
      $("#signalStrength").html(response.signal.strength);

      $("#ICCID").html(response.simCard.ICCID);
      $("#simProvider").html(response.simCard.provider);
      $("#simCardStatus").html(response.simCard.simCardStatus);
    } else {
      $("#modemData").hide();
    }

  } catch (e) {
    console.log(e);
  }
}

async function turnModemOn() {
  var data = { minutes: 10 };
  try {
    await apiFormURLEncodedPost("/api/modem-stay-on-for", data);
  } catch (e) {
    console.log(e);
  }
}


let logData = [];

function startLogging() {
  $("#signal-log-button").prop('disabled', true);
  $("#signal-log-button").css('opacity', '0.5');
  $("#signal-log-button").text("Logging...");
  const intervalId = setInterval(() => {
    logSignalData();
  }, 2000);

  setTimeout(() => {
    clearInterval(intervalId);
    createCSVDownload();
  }, 60000);
}

async function updateAPN() {
  const apn = document.getElementById("set-apn-text").value;
  console.log("Updating APN to", apn);

  var authHeaders = new Headers();
  authHeaders.append("Authorization", "Basic YWRtaW46ZmVhdGhlcnM=");
  authHeaders.append("Content-Type", "application/json");

  try {
      const res = await fetch("/api/modem/apn", {
          method: "POST",
          headers: authHeaders,
          body: JSON.stringify({ "apn": apn }),
      });

      if (!res.ok) {
          throw new Error(`Failed to update APN: ${res.status}`);
      }
      console.log("APN updated successfully!");
  } catch (error) {
      console.error("Failed to update APN:", error);
      alert("Failed to update APN: " + error.message);
  }
}

async function logSignalData() {
  try {
    const response = await apiGetJSON("/api/modem");
    if (response && response.signal) {
      logData.push({
        timestamp: new Date().toISOString(),
        band: response.signal.band,
        strength: response.signal.strength
      });
    }
  } catch (e) {
    console.error(e);
  }
}

function createCSVDownload() {
  let csvContent = "data:text/csv;charset=utf-8,";
  csvContent += "Timestamp,Band,Strength\r\n";

  logData.forEach(row => {
    let rowString = `${row.timestamp},${row.band},${row.strength}\r\n`;
    csvContent += rowString;
  });

  var encodedUri = encodeURI(csvContent);
  var link = document.createElement("a");
  link.setAttribute("href", encodedUri);
  link.setAttribute("download", $("#signal-log-name").val());
  document.body.appendChild(link); // Required for FF

  link.click(); // This will download the file
  $("#signal-log-button").prop('disabled', false);
  $("#signal-log-button").css('opacity', '1');
  $("#signal-log-button").text("Start Logging");
}
