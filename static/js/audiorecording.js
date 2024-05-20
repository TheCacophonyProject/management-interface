"use strict";

async function getAudioStatus() {
  
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("GET", "/api/audio/audio-status", true);
  xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));
  var success = false;
  xmlHttp.onload = async function () {
    if (xmlHttp.status == 200) {
      const state = Number(xmlHttp.response);
      let statusText ;
      if (state == 1 ){
        if (lastState){
          clearInterval(intervalId);
          lastState= null;
          intervalId = null;
          statusText = "";
          document.getElementById("audio-test-button").removeAttribute("disabled");
          document.getElementById("audio-test-button").innerText = 'Take Test Recording';
        }
      }else if(state ==2){
        lastState= state;
        statusText = "Test Recording Pending";
      }
      else if (state == 3){
        lastState= state;
        statusText = "Taking Test Recording";
      }else{
        statusText = "unknow state";
        clearInterval(intervalId);
        intervalId = null;
        document.getElementById("audio-test-button").removeAttribute("disabled");
        document.getElementById("audio-test-button").innerText = 'Take Test Recording';      
      } 
      document.getElementById("audio-test-status").innerText = statusText;    
    }

  }

  xmlHttp.onerror = async function () {
      updateAudioError(xmlHttp);
  };

  xmlHttp.send();
}

let intervalId = null;
let lastState= null;
async function takeTestRecording() {
    document.getElementById("audio-test-button").innerText = 'Making a test recording';
    document.getElementById("audio-test-button").setAttribute("disabled", "true");
 
    console.log("making a test recording");
    var xmlHttp = new XMLHttpRequest();
    xmlHttp.open("PUT", "/api/audio/test-recording", true);
    xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));
    var success = false;
    xmlHttp.onload = async function () {
      if (xmlHttp.status == 200) {
        success =         xmlHttp.responseText == "\"Asked for a test recording\"\n" ;
        if (!success){
          clearInterval(intervalId);

          document.getElementById("audio-test-button").removeAttribute("disabled");
          document.getElementById("audio-test-button").innerText = 'Take Test Recording';
      }else{
        clearInterval(intervalId);
        intervalId = setInterval(getAudioStatus, 1000); 

      }
        alert(xmlHttp.responseText);
      } else {
        console.log(xmlHttp);
        updateAudioError(xmlHttp);
      }
    };
  
    xmlHttp.onerror = async function () {
        updateAudioError(xmlHttp);
    };

    xmlHttp.send();

}

function handleEnableChange(event) {
    var checkBox = event.target;
    updateAudio();
}

function updateAudio() {
  console.log("update Audio Recording");
  var data ={}
  data["enabled"] = $("#enabledCheck").prop('checked');

//   var formData = new FormData();
//   formData.append("section", "audio-recording");
//   formData.append("config", JSON.stringify(data));

  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("POST", "/api/audiorecording", true);
  xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));
  xmlHttp.setRequestHeader(
    "Content-type",
    "application/x-www-form-urlencoded; charset=UTF-8"
  );
  xmlHttp.onload = async function () {
    if (xmlHttp.status == 200) {
      alert("updated audiorecording");
      if ($("#enabledCheck").prop('checked')){
        document.getElementById("audio-test-button").removeAttribute("disabled");
      }else{
        document.getElementById("audio-test-button").setAttribute("disabled", "true");
      }
    } else {
      console.log(xmlHttp);
      updateAudioError(xmlHttp);
    }
  };

  xmlHttp.onerror = async function () {
    updateAudioError(xmlHttp);
  };

  xmlHttp.send($.param(data));
}

function updateAudioError(xmlHttp) {
  alert("error updating audio recording: " + xmlHttp.responseText);
}
