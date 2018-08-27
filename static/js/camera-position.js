window.onload = function() {
  updateSnapshotLoop()
};

async function updateSnapshotLoop() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("GET", "/api/take-snapshot", true);
  xmlHttp.onreadystatechange = async function() {
    if (xmlHttp.readyState == 4 && xmlHttp.status == 200) {
      let snapshot = document.getElementById("snapshot-image")
      snapshot.src = "/snapshot-image?"+ new Date().getTime();
      await sleep(100);
      updateSnapshotLoop();
    }
  }
  xmlHttp.send( null );
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}
