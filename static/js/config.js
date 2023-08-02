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

      $("#input-initial-on-duration").attr(
        "placeholder",
        formatDuration(response.defaults.modemd.InitialOnDuration)
      );
      $("#input-find-modem-timeout").attr(
        "placeholder",
        formatDuration(response.defaults.modemd.FindModemTimeout)
      );
      $("#input-connection-timeout").attr(
        "placeholder",
        formatDuration(response.defaults.modemd.ConnectionTimeout)
      );

      console.log(response.values.windows.StartRecording);
      $("#input-start-recording").val(response.values.windows.StartRecording);
      $("#input-stop-recording").val(response.values.windows.StopRecording);
      $("#input-power-on").val(response.values.windows.PowerOn);
      $("#input-power-off").val(response.values.windows.PowerOff);
      if (response.values.modemd.InitialOnDuration != 0) {
        $("#input-initial-on-duration").val(formatDuration(response.values.modemd.InitialOnDuration));
      }
      if (response.values.modemd.FindModemTimeout != 0) {
        $("#input-find-modem-timeout").val(formatDuration(response.values.modemd.FindModemTimeout));
      }
      if (response.values.modemd.ConnectionTimeout != 0) {
        $("#input-connection-timeout").val(formatDuration(response.values.modemd.ConnectionTimeout));
      }
    } else {
      console.log("error with getting device details");
    }
  };

  xmlHttp.onerror = async function () {
    console.log("error with getting device config");
  };

  xmlHttp.send(null);
}

function formatDuration(nanoseconds) {
  var seconds = Math.floor(nanoseconds / 1000_000_000);
  var hours = Math.floor(seconds / 3600);
  var minutes = Math.floor((seconds % 3600) / 60);
  seconds = seconds % 60;

  var result = "";
  if (hours > 0) {
    result += hours + "h";
  }
  if (minutes > 0) {
    result += minutes + "m";
  }
  result += seconds + "s";

  return result;
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

function saveModemConfig() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("POST", "/api/config", true);
  xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));
  xmlHttp.onload = async function () {
    if (xmlHttp.status == 200) {
      alert("Modem config saved");
      loadConfig();
    } else {
      configError();
    }
  };
  xmlHttp.onerror = async function () {
    configError();
  };

  var data = {};
  if ($("#input-initial-on-duration").val() != "") {
    data["initial-on-duration"] = $("#input-initial-on-duration").val();
  }
  if ($("#input-find-modem-timeout").val() != "") {
    data["find-modem-timeout"] = $("#input-find-modem-timeout").val();
  }
  if ($("#input-connection-timeout").val() != "") {
    data["connection-timeout"] = $("#input-connection-timeout").val();
  }

  var formData = new FormData();
  formData.append("section", "modemd");
  formData.append("config", JSON.stringify(data));
  xmlHttp.send(formData);
}

function configError() {
  loadConfig();
  alert("Error saving config");
}
