"use strict";
enum AudioMode {
  Audio = 0,
  Thermal = 1,
}

enum AudioStatus {
  Ready = 1,
  WaitingToTakeTestRecording = 2,
  TakingTestRecording = 3,
  Recording = 4,
}
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
      if (state == AudioStatus.Ready) {
        countdown = 2;
        if (audioState) {
          audioState.clearInterval();
          statusText = "";
          enableRecButtons();
        }
      } else if (state == AudioStatus.WaitingToTakeTestRecording) {
        countdown = 2;
        audioState.lastState = state;
        statusText = "Test Recording Pending";
        if (!audioState.intervalId) {
          audioState.startPolling();
          disableRecButtons();
        }
      } else if (state == AudioStatus.TakingTestRecording) {
        audioState.lastState = state;
        if (countdown == 0) {
          statusText = "Taking Test Recording";
        } else {
          statusText = `Taking Test Recording in ${countdown}s`;
          countdown -= 1;
        }
        if (!audioState.intervalId) {
          audioState.startPolling();
          disableRecButtons();
        }
      } else if (state == AudioStatus.Recording) {
        countdown = 2;
        let recType = mode == AudioMode.Audio ? "an audio" : "a thermal";
        statusText = `Already Taking ${recType} Recording`;
        if (audioState.lastState != AudioStatus.TakingTestRecording) {
          disableRecButtons();
          audioState.startPolling();
          //need to tell tc2 agent to poll state
          testAPICall(false);
        }
        audioState.lastState = state;
      } else {
        countdown = 0;
        statusText = "unknow state";
        audioState.clearInterval();
        enableRecButtons();
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

function enableRecButtons() {
  audioState.clearInterval();
  document
    .getElementById("audio-recording-button")
    ?.removeAttribute("disabled");

  document.getElementById("audio-test-button")?.removeAttribute("disabled");
}

function disableRecButtons() {
  document
    .getElementById("audio-test-button")
    ?.setAttribute("disabled", "true");
  document
    .getElementById("audio-recording-button")
    ?.setAttribute("disabled", "true");
}

async function takeLongRecording() {
  disableRecButtons();
  recordingAPICall(true);
}

async function takeTestRecording() {
  disableRecButtons();
  testAPICall(true);
}

async function recordingAPICall(checkResponse: boolean) {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("PUT", "/api/audio/long-recording?seconds=300", true);
  xmlHttp.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));

  var success = false;
  if (checkResponse) {
    xmlHttp.onload = async function () {
      if (xmlHttp.status == 200) {
        success = xmlHttp.responseText == '"Asked for a test recording"\n';
        if (!success) {
          enableRecButtons();
          updateAudioError(xmlHttp);
        } else {
          audioState.clearInterval();
          audioState.startPolling();
        }
      } else {
        enableRecButtons();
        updateAudioError(xmlHttp);
      }
    };
  }

  xmlHttp.onerror = async function () {
    enableRecButtons();
    updateAudioError(xmlHttp);
  };

  xmlHttp.send();
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
          enableRecButtons();
          updateAudioError(xmlHttp);
        } else {
          audioState.clearInterval();
          audioState.startPolling();
        }
      } else {
        enableRecButtons();
        updateAudioError(xmlHttp);
      }
    };
  }

  xmlHttp.onerror = async function () {
    enableRecButtons();
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
    .getElementById("audio-test-button")
    ?.addEventListener("click", takeTestRecording, false);
  document
    .getElementById("audio-recording-button")
    ?.addEventListener("click", takeLongRecording, false);
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
