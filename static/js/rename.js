"use strict";
window.onload = function() {
  loadDeviceDetails();

  if (hasGroupList()) {
    var groups = getGroups();
    for (var i in groups) {
      var group = groups[i];
      $('<option/>', {
        value: group,
        innerHTML: group,
      }).appendTo('#group-list');
    }
  }
};

function loadDeviceDetails() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("GET", "/api/device-info", true);
  xmlHttp.setRequestHeader("Authorization", "Basic "+btoa("admin:feathers"))

  xmlHttp.onload = async function() {
    if (xmlHttp.status == 200) {
      var response = JSON.parse(xmlHttp.response);
      $("#new-name").attr('placeholder', response.devicename)
      $("#new-group").attr('placeholder', response.groupname)
    } else {
      console.log("error with getting device details");
    }
  }

  xmlHttp.onerror = async function() {
    console.log("error with getting device details");
  }

  xmlHttp.send(null)
}

function rename() {
  var updateButton = document.getElementById("rename-button");
  updateButton.disabled = true;
  updateButton.innerHTML = "Renaming";

  var data = $("#rename-form").serialize()

  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open('POST', '/api/rename', true);
  xmlHttp.setRequestHeader("Authorization", "Basic "+btoa("admin:feathers"))
  xmlHttp.setRequestHeader("Content-type", "application/x-www-form-urlencoded; charset=UTF-8");
  xmlHttp.onload = async function() {
    if (xmlHttp.status == 200) {
      alert("updated device name and group. Reboot device for changes to take place")
      resetRenameButton();
      $("#reboot-div").show()
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
  alert("error renaming device: " + getResponseMessage(xmlHttp.responseText))
}

function resetRenameButton() {
  var updateButton = document.getElementById("rename-button");
  updateButton.disabled = false;
  updateButton.innerHTML = "Rename";
}


function getResponseMessage(bodyString) {
  // The body string needs to be sliced as the go-api will add a prefix onto the response from the server
  var jsonString = bodyString.slice(bodyString.indexOf("{\""))
  try {
    return JSON.parse(jsonString).message;
  } catch(e) {
    return bodyString
  }
}
