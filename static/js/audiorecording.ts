"use strict";

let intervalId: number | null = null;
let lastState: number | null = null;
let countdown = 0;
async function getAudioStatus() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("GET", "/api/audio/audio-status", true);
  xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));
  var success = false;
  xmlHttp.onload = async function () {
    if (xmlHttp.status == 200) {
      const state = Number(xmlHttp.response);
      let statusText = "";
      if (state == 1) {
        countdown = 2;
        if (lastState) {
          clearInterval(intervalId as number);
          lastState = null;
          intervalId = null;
          statusText = "";
          document
            .getElementById("audio-test-button")
            ?.removeAttribute("disabled");
          (
            document.getElementById("audio-test-button") as HTMLElement
          ).innerText = "Take Test Recording";
        }
      } else if (state == 2) {
        countdown = 2;
        lastState = state;
        statusText = "Test Recording Pending";
        if (!intervalId) {
          intervalId = setInterval(getAudioStatus, 1000);
          document
            .getElementById("audio-test-button")
            ?.setAttribute("disabled", "true");
        }
      } else if (state == 3) {
        lastState = state;
        if (countdown == 0) {
          statusText = "Taking Test Recording";
        } else {
          statusText = `Taking Test Recording in ${countdown}s`;
          countdown -= 1;
        }
        if (!intervalId) {
          intervalId = setInterval(getAudioStatus, 1000);
          document
            .getElementById("audio-test-button")
            ?.setAttribute("disabled", "true");
        }
      } else if (state == 4) {
        countdown = 2;
        statusText = "Already Taking a Recording";
        if (lastState != 4) {
          clearInterval(intervalId as number);
          document
            .getElementById("audio-test-button")
            ?.setAttribute("disabled", "true");
          intervalId = setInterval(getAudioStatus, 10000);
          //need to tell tc2 agent to poll state
          testAPICall(false);
          (
            document.getElementById("audio-test-button") as HTMLElement
          ).innerText = "Waiting for Recording to finish";
        }
        lastState = state;
      } else {
        countdown = 0;
        statusText = "unknow state";
        clearInterval(intervalId as number);
        intervalId = null;
        document
          .getElementById("audio-test-button")
          ?.removeAttribute("disabled");
        (
          document.getElementById("audio-test-button") as HTMLElement
        ).innerText = "Take Test Recording";
      }
      (document.getElementById("audio-test-status") as HTMLElement).innerText =
        statusText;
    }
  };

  xmlHttp.onerror = async function () {
    updateAudioError(xmlHttp);
  };

  await xmlHttp.send();
}

function enableRecButton() {
  if (intervalId) {
    clearInterval(intervalId);
  }
  document.getElementById("audio-test-button")?.removeAttribute("disabled");
  (document.getElementById("audio-test-button") as HTMLElement).innerText =
    "Take Test Recording";
}

async function takeTestRecording() {
  (document.getElementById("audio-test-button") as HTMLElement).innerText =
    "Making a test recording";
  document
    .getElementById("audio-test-button")
    ?.setAttribute("disabled", "true");
  testAPICall(true);
}

async function testAPICall(checkResponse: boolean) {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("PUT", "/api/audio/test-recording", true);
  xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));
  var success = false;
  if (checkResponse) {
    xmlHttp.onload = async function () {
      if (xmlHttp.status == 200) {
        success = xmlHttp.responseText == '"Asked for a test recording"\n';
        if (!success) {
          enableRecButton();
          updateAudioError(xmlHttp);
        } else {
          clearInterval(intervalId as number);
          intervalId = setInterval(getAudioStatus, 1000);
        }
      } else {
        enableRecButton();
        updateAudioError(xmlHttp);
      }
    };
  }

  xmlHttp.onerror = async function () {
    enableRecButton();
    updateAudioError(xmlHttp);
  };

  xmlHttp.send();
}

function handleModeChange() {
  updateAudio();
}

function updateAudio() {
  var data: any = {};
  data["audio-mode"] = document.getElementById("audio-mode-select")?.nodeValue;

  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("POST", "/api/audiorecording", true);
  xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));
  xmlHttp.setRequestHeader(
    "Content-type",
    "application/x-www-form-urlencoded; charset=UTF-8"
  );
  xmlHttp.onload = async function () {
    if (xmlHttp.status == 200) {
      if (
        document.getElementById("audio-mode-select")?.nodeValue != "Disabled"
      ) {
        document
          .getElementById("audio-test-button")
          ?.removeAttribute("disabled");
      } else {
        document
          .getElementById("audio-test-button")
          ?.setAttribute("disabled", "true");
      }
    } else {
      updateAudioError(xmlHttp);
    }
  };

  xmlHttp.onerror = async function () {
    updateAudioError(xmlHttp);
  };

  xmlHttp.send($.param(data));
}

function updateAudioError(xmlHttp: XMLHttpRequest) {
  alert("error updating audio recording: " + xmlHttp.responseText);
}

window.onload = async function () {
  getAudioStatus();
};
