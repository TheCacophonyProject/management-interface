updateSignal();

const refreshTime =2 * 1000
const clearSignalAttempts = 3 
const maxStrength = 100;
const minStrength = 0;
const bars = 5;
const signalRange = maxStrength - minStrength
const signalPerBar = signalRange / parseFloat(bars);

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
    rechecksignal(refreshTime); 
  }

  xmlHttp.onerror = function() {
    handleFailure("error occured accesing " + "/api/signal-strength")
    rechecksignal(refreshTime); 
  }

  xmlHttp.send(null);
}

function handleSignalSuccess(signalVal){
  signalFails = 0;
  var signalElement = document.getElementById("signal-strength");
  var strength =parseInt(signalVal);

  signalElement.innerText = strength;
  var signalBars = Math.ceil(strength / signalPerBar);
  for(var i = 1; i <= bars; i++){
    var bar = $(".signal-" + i);
    if(i <= signalBars){
      bar.addClass("signal")
      bar.removeClass("no-signal")
    }else{
      bar.addClass("no-signal")
      bar.removeClass("signal")
    }
  }
}

function handleSignalFailure(errorMessage){
  if(signalFails == 0){
    console.log(errorMessage);
  }
  signalFails++;
  if(signalFails >= clearSignalAttempts){
    var signalElement = document.getElementById("signal-strength");
    signalElement.innerText = " - "
  }
}

function reCheckSignal(ms) {
  setTimeout(updateSignal, ms);
}
