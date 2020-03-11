"use strict";
window.onload = function() {
  const urlParams = new URLSearchParams(window.location.search);
  if(urlParams.get('timeout') == "off")  {
    snapshotLimit = Number.MAX_SAFE_INTEGER;
  }
  updateSnapshotLoop()
};

var snapshotCount = 0;
var snapshotLimit = 200;

function restartCameraViewing() {
  document.getElementById("snapshot-stopped").style.display = 'none';
  document.getElementById("snapshot-image").style.display = '';
  snapshotCount = 0;
  updateSnapshotLoop();
}

function updateSnapshotLoop() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("PUT", "/api/camera/snapshot", true);
  xmlHttp.setRequestHeader("Authorization", "Basic "+btoa("admin:feathers"))
  xmlHttp.onload = function() {
    if (xmlHttp.status == 200) {
      let snapshot = document.getElementById("snapshot-image")
      snapshot.src = "/camera/snapshot?"+ new Date().getTime();
      snapshotCount++;
      if (snapshotCount < snapshotLimit) {
        setTimeout(updateSnapshotLoop, 500);
      } else {
        stopSnapshots('Timeout for camera viewing.');
      }
    } else {
      console.log("status:", xmlHttp.status);
      console.log("response:", xmlHttp.response);
      stopSnapshots('Error getting new snapshot.')
    }
  }
  xmlHttp.onerror = function(err) {
    console.log('error:', err);
    stopSnapshots('Error getting new snapshot');
  }
  xmlHttp.send( null );
}

function stopSnapshots(message) {
  document.getElementById("snapshot-stopped-message").innerText = message;
  document.getElementById("snapshot-stopped").style.display = '';
  document.getElementById("snapshot-image").style.display = 'none';
}
