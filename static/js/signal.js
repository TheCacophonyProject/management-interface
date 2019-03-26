window.onload = function() {
  updateSignalLoop()
};

async function updateSignalLoop() {
  const refreshTime =2 * 1000 

  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("GET", "/api/signal-strength", true);
  xmlHttp.setRequestHeader("Authorization", "Basic "+btoa("admin:feathers"))
  xmlHttp.onload = async function() {
    if (xmlHttp.status == 200) {
      var signalElement = document.getElementById("signal-strength");
      signalElement.innerText = xmlHttp.response;
      await sleep(refreshTime);
    } else {
      console.log("status:", xmlHttp.status);
      console.log("response:", xmlHttp.response);
      await sleep(refreshTime);
    }
    updateSignalLoop(); 
  }
  xmlHttp.onerror = function(err) {
    console.log('error:', err);
  }

  xmlHttp.send( null );
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}
