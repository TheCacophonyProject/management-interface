"use strict";

class AudioState {
  constructor(
    public intervalId: number | null,
    public lastState: number | null
  ) {}
  clearInterval() {
    if (this.intervalId) {
      clearInterval(this.intervalId as number);
    }
    this.intervalId = null;
    this.lastState = null;
  }
  startPolling() {
    this.clearInterval();
    this.intervalId = setInterval(getAudioStatus, 1000);
  }
}
const audioState = new AudioState(null, null);
let countdown = 0;
async function getAudioStatus() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.responseType = "json";
  xmlHttp.open("GET", "/api/audio/audio-status", true);
  xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));
  var success = false;
  xmlHttp.onload = async function () {
    if (xmlHttp.status == 200) {
      const rp2040state = xmlHttp.response;
      const state = Number(rp2040state.status);
      const mode = Number(rp2040state.mode);

      let statusText = "";
      if (state == 1) {
        countdown = 2;
        if (audioState) {
          audioState.clearInterval();
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
        audioState.lastState = state;
        statusText = "Test Recording Pending";
        if (!audioState.intervalId) {
          audioState.startPolling();
          document
            .getElementById("audio-test-button")
            ?.setAttribute("disabled", "true");
        }
      } else if (state == 3) {
        audioState.lastState = state;
        if (countdown == 0) {
          statusText = "Taking Test Recording";
        } else {
          statusText = `Taking Test Recording in ${countdown}s`;
          countdown -= 1;
        }
        if (!audioState.intervalId) {
          audioState.startPolling();
          document
            .getElementById("audio-test-button")
            ?.setAttribute("disabled", "true");
        }
      } else if (state == 4) {
        countdown = 2;
        let recType = mode == 1 ? "an audio" : "a thermal";
        statusText = `Already Taking ${recType} Recording`;
        if (audioState.lastState != 4) {
          document
            .getElementById("audio-test-button")
            ?.setAttribute("disabled", "true");
          audioState.startPolling();
          //need to tell tc2 agent to poll state
          testAPICall(false);
          (
            document.getElementById("audio-test-button") as HTMLElement
          ).innerText = "Waiting for Recording to finish";
        }
        audioState.lastState = state;
      } else {
        countdown = 0;
        statusText = "unknow state";
        audioState.clearInterval();
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
  audioState.clearInterval();

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
          audioState.clearInterval();
          audioState.startPolling();
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
  data["audio-mode"] = (
    document.getElementById("audio-mode-select") as HTMLSelectElement
  ).value;
  data["audio-seed"] = (
    document.getElementById("audio-seed") as HTMLInputElement
  ).value;
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
        (document.getElementById("audio-mode-select") as HTMLSelectElement)
          .value != "Disabled"
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
  document
    .getElementById("updateBtn")
    ?.addEventListener("click", updateAudio, false);
  document
    .getElementById("audio-mode-select")
    ?.addEventListener("change", handleModeChange, false);

  const audioseed = (document.getElementById("audio-seed") as HTMLInputElement)
    .value;
  if (Number(audioseed) == 0) {
    // @ts-ignore
    (document.getElementById("audio-seed") as HTMLInputElement).value = null;
  }
};
