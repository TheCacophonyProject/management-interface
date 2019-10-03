"use strict";

function clearLocation() {
  console.log("clear location");
  var data = [{"name":"section","value":"location"}]
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open('POST', '/api/clear-config-section', true);
  xmlHttp.setRequestHeader("Authorization", "Basic "+btoa("admin:feathers"))
  xmlHttp.setRequestHeader("Content-type", "application/x-www-form-urlencoded; charset=UTF-8");
  xmlHttp.onload = async function() {
    if (xmlHttp.status == 200) {
      $("#altitude").val(0);
      $("#accuracy").val(0);
      $("#longitude").val(0);
      $("#latitude").val(0);
      alert("updated location")

    } else {
      console.log(xmlHttp);
      updateLocationError(xmlHttp);
    }
  }

  xmlHttp.onerror = async function() {
    updateLocationError(xmlHttp);
  }

  xmlHttp.send($.param(data));
}

function updateLocation() {
  console.log("updateLocation");
  var data = $("#location-form").serializeArray()
  var utc = new Date().getTime()
  data.push({"name":"timestamp", "value":utc})

  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open('POST', '/api/location', true);
  xmlHttp.setRequestHeader("Authorization", "Basic "+btoa("admin:feathers"))
  xmlHttp.setRequestHeader("Content-type", "application/x-www-form-urlencoded; charset=UTF-8");
  xmlHttp.onload = async function() {
    if (xmlHttp.status == 200) {
      alert("updated location")
    } else {
      console.log(xmlHttp);
      updateLocationError(xmlHttp);
    }
  }

  xmlHttp.onerror = async function() {
    updateLocationError(xmlHttp);
  }

  xmlHttp.send($.param(data));
}

function updateLocationError(xmlHttp) {
  alert("error updating location: " + xmlHttp.responseText)
}
