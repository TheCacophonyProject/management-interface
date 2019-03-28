updateSignal();

const refreshSeconds = 2 * 1000
const maxRefreshDelay = 10
const clearSignalAttempts = 3 
const bars = 5;

var refreshTime = refreshSeconds
var signalFails = 0;

async function updateSignal() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("GET", "/api/signal-strength", true);
  xmlHttp.setRequestHeader("Authorization", "Basic "+btoa("admin:feathers"))
  
  xmlHttp.onload = async function() {
    if (xmlHttp.status == 200) {
      handleSignalSuccess(xmlHttp.response);
    } else {
      handleSignalFailure("status:" + xmlHttp.status + " response:" + xmlHttp.response);
    }
    reCheckSignal(refreshTime); 
  }

  xmlHttp.onerror = function() {
    handleSignalFailure("error occured accesing " + "/api/signal-strength")
    reCheckSignal(refreshTime); 
  }

  xmlHttp.send(null);
}

function handleSignalSuccess(signalVal){
  var strength =parseInt(signalVal);
  signalFails = 0;
  refreshTime = refreshSeconds;

  $(".signal-unavail").hide();
  $(".svg-signal").show();

  if(strength == 0 ){
    $(".signal-unavail").show().removeClass("no-modem");
  }

  for(var i = 1; i <= bars; i++){
    var bar = $(".signal-" + i);
    if(i <= strength){
      bar.addClass("signal")
      bar.removeClass("no-signal")
    }else{
      bar.addClass("no-signal")
      bar.removeClass("signal")
    }
  }
}

function handleSignalFailure(errorMessage){
  $(".signal-unavail").show().addClass("no-modem");
  $('*[class^="signal-"]').removeClass("signal").addClass("no-signal")

  if(signalFails == 0){
    console.log(errorMessage);
  }

  signalFails++;
  if(signalFails >= clearSignalAttempts){
    refreshTime += refreshSeconds;
    refreshTime = Math.max(refreshTime,maxRefreshDelay);
  }
}

function reCheckSignal(ms) {
  setTimeout(updateSignal, ms);
}
