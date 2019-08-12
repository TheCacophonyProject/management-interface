"use strict";
window.onload = function() {
  loadDeviceDetails();
};

function loadDeviceDetails() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("GET", "/api/device-info", true);
  xmlHttp.setRequestHeader("Authorization", "Basic "+btoa("admin:feathers"))

  xmlHttp.onload = async function() {
    if (xmlHttp.status == 200) {
      var response = JSON.parse(xmlHttp.response);
      document.getElementById("new-name").value=response.devicename;
      document.getElementById("new-group").value=response.groupname;
    } else {
      console.log("error with getting device details");
      console.log(xmlHttp);
    }
  }

  xmlHttp.onerror = async function() {
    console.log("error with getting device details");
    console.log(xmlHttp);
  }

  xmlHttp.send(null)
}

function rename() {
  var updateButton = document.getElementById("rename-button");
  updateButton.disabled = true;
  updateButton.innerHTML = "Renaming";

  var data = new FormData();
  data.append('name', document.getElementById("new-name").value);
  data.append('group', document.getElementById("new-group").value);

  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open('POST', '/api/rename', true);
  xmlHttp.setRequestHeader("Authorization", "Basic "+btoa("admin:feathers"))
  xmlHttp.onload = async function() {
    if (xmlHttp.status == 200) {
      alert("updated device name and gruop")
      resetRenameButton();
    } else {
      renameError(xmlHttp);
    }
  }

  xmlHttp.onerror = async function() {
    renameError(xmlHttp);
  }

  xmlHttp.send(data);
}

function renameError(xmlHttp) {
  resetRenameButton();
  alert("error with renaming device");
  console.log(xmlHttp);
}

function resetRenameButton() {
  var updateButton = document.getElementById("rename-button");
  updateButton.disabled = false;
  updateButton.innerHTML = "Rename";
}
