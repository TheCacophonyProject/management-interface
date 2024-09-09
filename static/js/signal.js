updateSignal();
setInterval(updateSignal, 5000);

const refreshMillis = 2 * 1000;
const maxRefreshDelayMillis = 10 * 1000;
const clearSignalAttempts = 3;
const bars = 5;

var refreshTime = refreshMillis;
var signalFails = 0;

async function updateSignalTC2() {
  var response = await apiGetJSON("/api/modem");

  if (response.simCard) {
    if (response.simCard.simCardStatus == "READY") {
      $("#modem-status").html("");
    } else {
      $("#modem-status").html("No Sim Card");
    }
  } else {
    $("#modem-status").html("");
  }

  if (!response.signal) {
    handleSignalSuccess(0);
    return;
  }
  strength = response.signal.strength;

  // A signal strength of 99 means that the signal was not detectable.
  if (strength == 99) {
    handleSignalSuccess(0);
    return;
  }

  barsStrength = 1;
  if (strength < 9) {
    barsStrength = 2;
  } else if (strength < 14) {
    barsStrength = 3;
  } else if (strength < 19) {
    barsStrength = 4;
  } else if (strength < 30) {
    barsStrength = 5;
  } else {
    barsStrength = 6;
  }
  handleSignalSuccess(barsStrength);

  return;
}

async function updateSignal() {
  //TODO Check if device is TC2, just hard coded for now.
  updateSignalTC2();
  return;
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("GET", "/api/signal-strength", true);
  xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));

  xmlHttp.onload = async function () {
    if (xmlHttp.status == 200) {
      handleSignalSuccess(xmlHttp.response);
    } else {
      handleSignalFailure(
        "status:" + xmlHttp.status + " response:" + xmlHttp.response
      );
    }
    reCheckSignal(refreshTime);
  };

  xmlHttp.onerror = function () {
    handleSignalFailure("error occurred accessing " + "/api/signal-strength");
    reCheckSignal(refreshTime);
  };

  xmlHttp.send(null);
}

function handleSignalSuccess(signalVal) {
  var strength = parseInt(signalVal);
  signalFails = 0;
  refreshTime = refreshMillis;

  $(".signal-unavail").hide();
  $(".svg-signal").show();

  if (strength == 0) {
    $(".signal-unavail").show().removeClass("no-modem");
  }

  for (var i = 1; i <= bars; i++) {
    var bar = $(".signal-" + i);
    if (i <= strength) {
      bar.addClass("signal");
      bar.removeClass("no-signal");
    } else {
      bar.addClass("no-signal");
      bar.removeClass("signal");
    }
  }
}

function handleSignalFailure(errorMessage) {
  $(".signal-unavail").show().addClass("no-modem");
  $('*[class^="signal-"]').removeClass("signal").addClass("no-signal");

  if (signalFails == 0) {
    console.log(errorMessage);
  }

  signalFails++;
  if (signalFails >= clearSignalAttempts) {
    refreshTime += refreshMillis;
    refreshTime = Math.max(refreshTime, maxRefreshDelayMillis);
  }
}

function reCheckSignal(ms) {
  setTimeout(updateSignal, ms);
}
