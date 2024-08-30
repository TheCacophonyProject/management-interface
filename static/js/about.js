authHeaders = new Headers();
authHeaders.append("Authorization", "Basic YWRtaW46ZmVhdGhlcnM=");

window.onload = async function () {
  readAutoUpdate();
  getEnvironmentState();
  updateSaltState();
};

async function setAutoUpdate(autoUpdate) {

  var headers = new Headers(authHeaders);
  headers.append("Content-Type", "application/x-www-form-urlencoded");
  console.log("set auto update", autoUpdate);
  var res = await fetch("/api/auto-update", {
    method: "POST",
    headers: headers,
    body: new URLSearchParams({ autoUpdate: autoUpdate }),
  });
  if (!res.ok) {
    alert("failed to update auto update state");
  }
  await readAutoUpdate();
}

async function readAutoUpdate() {
  var res = await fetch("/api/auto-update", { headers: authHeaders });
  if (res.ok) {
    resJson = await res.json();
    document.getElementById("auto-update-checkbox").checked =
      resJson.autoUpdate;
  }
}

function checkSaltConnection() {
  $("#check-salt-button").attr("disabled", true);
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("GET", "/api/check-salt-connection", true);
  xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));

  xmlHttp.timeout = 20000; // Set timeout for 20 seconds
  xmlHttp.onload = async function () {
    if (xmlHttp.status == 200) {
      var response = JSON.parse(xmlHttp.response);
      console.log(response);
      if (response.LastCallSuccess) {
        alert("Salt connected and accepted");
      } else {
        alert("Salt ping failed:" + response.LastCallOut);
      }
    } else {
      console.log("error with checking salt connection");
    }
    $("#check-salt-button").attr("disabled", false);
  };
  xmlHttp.onerror = async function () {
    console.log("error with checking salt connection");
    $("#check-salt-button").attr("disabled", false);
  };
  xmlHttp.ontimeout = async function () {
    alert("connection timeout");
    $("#check-salt-button").attr("disabled", false);
  };
  xmlHttp.send(null);
}

function runSaltUpdate() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("POST", "/api/salt-update", true);
  xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));
  xmlHttp.setRequestHeader("Content-Type", "application/json");
  xmlHttp.onload = async function () {
    if (xmlHttp.status == 200) {
      $("#salt-update-button").attr("disabled", true);
      $("#salt-update-button").html("Running Salt Update...");
      setTimeout(updateSaltState, 2000);
    } else {
      console.log(xmlHttp.responseText);
    }
  };
  xmlHttp.onerror = async function () {
    console.log("error with running salt update");
  };

  var jsonPayload = JSON.stringify({ force: true });
  xmlHttp.send(jsonPayload);
}

async function uploadLogs() {
  $("#upload-logs-button").attr("disabled", true);
  $("#upload-logs-button").html("Uploading logs...");
  try {
    const response = await fetch("/api/upload-logs", {
      method: "PUT",
      headers: {
        Authorization: "Basic " + btoa("admin:feathers"),
        "Content-Type": "application/json",
      },
    });

    if (response.ok) {
      alert("Logs uploaded");
    } else {
      alert("Error uploading logs");
      console.error("Error with response:", await response.text());
    }
  } catch (error) {
    alert("Error uploading logs");
    console.error("Error with uploading logs:", error);
  }
  $("#upload-logs-button").attr("disabled", false);
  $("#upload-logs-button").html("Upload logs");
}

async function updateSaltState() {
  try {
    const response = await fetch("/api/salt-update", {
      method: "GET",
      headers: {
        Authorization: "Basic " + btoa("admin:feathers"),
        "Content-Type": "application/json",
      },
    });

    if (response.ok) {
      var data = JSON.parse(await response.text());

      if (data.RunningUpdate) {
        document
          .getElementById("salt-update-button")
          .setAttribute("disabled", true);
        document.getElementById("salt-update-button").textContent =
          "Running Salt Update...";
        setTimeout(updateSaltState, 2000);
      } else {
        enableSaltButton();
      }

      document.getElementById("salt-update-progress").textContent =
        data.UpdateProgressPercentage;
      document.getElementById("salt-update-progress-text").textContent =
        data.UpdateProgressStr;
      document.getElementById("running-salt-command").textContent =
        data.RunningUpdate ? "Yes" : "No";
      document.getElementById("running-salt-arguements").textContent =
        data.RunningArgs ? data.RunningArgs.join(", ") : "None";
      document.getElementById("previous-run-arguments").textContent =
        data.LastCallArgs ? data.LastCallArgs.join(", ") : "None";
      document.getElementById("previous-output").textContent = data.LastCallOut;
      document.getElementById("previous-success").textContent =
        data.LastCallSuccess ? "Yes" : "No";
      document.getElementById("previous-nodegroup").textContent =
        data.LastCallNodegroup;
    } else {
      alert("Error updating salt");
      console.error("Error with response:", await response.text());
      enableSaltButton();
    }
  } catch (error) {
    alert("Error updating salt");
    console.error("Error with fetching salt update:", error);
    enableSaltButton();
  }
}

function enableSaltButton() {
  document.getElementById("salt-update-button").removeAttribute("disabled");
  document.getElementById("salt-update-button").textContent =
    "Run Salt Update...";
}

var runningSaltUpdate = true;
// Check salt update state. Returns true if it is no longer running
function checkSaltUpdateState() {
  console.log("checking salt update state");
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("GET", "/api/salt-update", true);
  xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));
  xmlHttp.onload = async function () {
    if (xmlHttp.status == 200) {
      var response = JSON.parse(xmlHttp.response);
      runningSaltUpdate = response.RunningUpdate;
      if (!runningSaltUpdate) {
        $("#salt-update-button").attr("disabled", false);
        $("#salt-update-button").html("Run Salt Update...");
        if (response.LastCallSuccess) {
          alert("Salt update finished");
          // Reload page to update values
          location.reload();
        } else {
          alert("Salt update failed");
        }
      }
    }
    console.log(response);
  };
  xmlHttp.onerror = async function () {
    $("#salt-update-button").attr("disabled", false);
    $("#salt-update-button").html("Run Salt Update...");
    console.log("error with running salt update");
  };
  xmlHttp.send(null);
}

// Will check the salt update state until it has finished
function pollSaltUpdateState() {
  if (runningSaltUpdate == false) {
    return;
  }
  checkSaltUpdateState();
  setTimeout(pollSaltUpdateState, 3000);
}

function getEnvironmentState() {
  fetch("/api/salt-grains", {
    headers: authHeaders,
  })
    .then((response) => response.json())
    .then((data) => {
      if (data.environment) {
        document.getElementById("environment-select").value = data.environment;
      }
    })
    .catch((error) =>
      console.error("Error fetching environment state:", error)
    );
}

async function setEnvironment() {
  const selectedEnvironment =
    document.getElementById("environment-select").value;
  if (selectedEnvironment == "") {
    alert('Please select an environment');
    return;
  }
  $("#set-environment-button").attr("disabled", true);
  $("#set-environment-button").html("Setting Environment");

  headers = authHeaders;
  headers.append("Content-Type", "application/json");
  try {
    var response = await fetch("/api/salt-grains", {
      method: "POST",
      headers: headers,
      body: JSON.stringify({ environment: selectedEnvironment }),
    });
    if (response.ok) {
      alert("Environment set successfully");
    } else {
      alert("Failed to set environment");
    }
  } catch (error) {
    console.error("Error setting environment:", error);
  }
  $("#set-environment-button").attr("disabled", false);
  $("#set-environment-button").html("Set Environment");
}

function downloadTemperatureCsv() {
  window.location.href = "/temperature-csv";
}
