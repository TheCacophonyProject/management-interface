window.onload = function() {
  updateSnapshotLoop()
};

var snapshotCount = 0;

function restartCameraViewing() {
  document.getElementById("timeout-message").style.display = 'none';
  document.getElementById("snapshot-image").style.display = '';
  snapshotCount = 0;
  updateSnapshotLoop();
}

async function updateSnapshotLoop() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("GET", "/api/take-snapshot", true);
  xmlHttp.onreadystatechange = async function() {
    if (xmlHttp.readyState == 4 && xmlHttp.status == 200) {
      let snapshot = document.getElementById("snapshot-image")
      snapshot.src = "/snapshot-image?"+ new Date().getTime();
      await sleep(500);
      snapshotCount++;
      if (snapshotCount < 200) {
        updateSnapshotLoop();
      } else {
        snapshot.style.display = 'none';
        document.getElementById("timeout-message").style.display = '';
      }
    } else if (xmlHttp.readyState == 4 && xmlHttp.status != 200) {
      console.log("failed to update image");
      await sleep(100);
      updateSnapshotLoop();
    }
  }
  xmlHttp.send( null );
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}
