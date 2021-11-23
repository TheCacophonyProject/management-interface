"use strict";
window.onload = function () {
  loadConfig();
};

function loadConfig() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("GET", "/api/config", true);
  xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));

  xmlHttp.onload = async function () {
    if (xmlHttp.status == 200) {
      var response = JSON.parse(xmlHttp.response);
      console.log(response);
      $("#input-start-recording").attr(
        "placeholder",
        response.defaults.windows.StartRecording
      );
      $("#input-stop-recording").attr(
        "placeholder",
        response.defaults.windows.StopRecording
      );
      $("#input-power-on").attr(
        "placeholder",
        response.defaults.windows.PowerOn
      );
      $("#input-power-off").attr(
        "placeholder",
        response.defaults.windows.PowerOff
      );
      console.log(response.values.windows.StartRecording);
      $("#input-start-recording").val(response.values.windows.StartRecording);
      $("#input-stop-recording").val(response.values.windows.StopRecording);
      $("#input-power-on").val(response.values.windows.PowerOn);
      $("#input-power-off").val(response.values.windows.PowerOff);
    } else {
      console.log("error with getting device details");
    }
  };

  xmlHttp.onerror = async function () {
    console.log("error with getting device config");
  };

  xmlHttp.send(null);
}

function saveWindowsConfig() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("POST", "/api/config", true);
  xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));
  xmlHttp.onload = async function () {
    if (xmlHttp.status == 200) {
      alert("Config saved");
      loadConfig();
    } else {
      configError();
    }
  };
  xmlHttp.onerror = async function () {
    configError();
  };

  var data = {};
  if ($("#input-start-recording").val() != "") {
    data["start-recording"] = $("#input-start-recording").val();
  }
  if ($("#input-stop-recording").val() != "") {
    data["stop-recording"] = $("#input-stop-recording").val();
  }
  if ($("#input-power-on").val() != "") {
    data["power-on"] = $("#input-power-on").val();
  }
  if ($("#input-power-off").val() != "") {
    data["power-off"] = $("#input-power-off").val();
  }

  var formData = new FormData();
  formData.append("section", "windows");
  formData.append("config", JSON.stringify(data));
  xmlHttp.send(formData);
}

function configError() {
  loadConfig();
  alert("Error saving config");
}
