window.onload = function() {
  updateSnapshotLoop()
};

var snapshotCount = 0;

function restartCameraViewing() {
  document.getElementById("snapshot-stopped").style.display = 'none';
  document.getElementById("snapshot-image").style.display = '';
  snapshotCount = 0;
  updateSnapshotLoop();
}

async function updateSnapshotLoop() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("PUT", "/api/camera/snapshot", true);
  xmlHttp.onload = async function() {
    if (xmlHttp.status == 200) {
      let snapshot = document.getElementById("snapshot-image")
      snapshot.src = "/camera/snapshot?"+ new Date().getTime();
      await sleep(500);
      snapshotCount++;
      if (snapshotCount < 200) {
        updateSnapshotLoop();
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

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}
