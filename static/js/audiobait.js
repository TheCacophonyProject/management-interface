authHeaders = new Headers();
authHeaders.append("Authorization", "Basic YWRtaW46ZmVhdGhlcnM=");

window.onload = async function () {
  // Load audio sounds
  document.getElementById("volume").oninput = function () {
    document.getElementById("volume-text").innerHTML = this.value;
  };
  res = await fetch("/api/audiobait", { method: "GET", headers: authHeaders });
  if (res.ok) {
    resJson = await res.json();
    filesById = resJson.library.FilesByID;
    audioSelect = document.getElementById("test-audio-select");
    for (var id in filesById) {
      option = document.createElement("option");
      option.text = filesById[id];
      option.value = id;
      audioSelect.add(option);
    }
  }

  res = await fetch("/api/service?service=audiobait", {
    method: "GET",
    headers: authHeaders,
  });
  resJson = await res.json();
  if (resJson.Active) {
    document.getElementById("running").style.display = "";
  } else {
    document.getElementById("not-running").style.display = "";
  }
};

async function playSound(element, id, volume) {
  console.log("play sound");
  startInner = element.innerHTML;
  element.innerHTML = "Playing";
  element.disabled = true;
  try {
    await apiFormURLEncodedPost("/api/play-audiobait-sound", {
      fileId: id,
      volume: volume,
    });
  } catch (e) {
    alert("failed to play audio");
  }
  element.innerHTML = startInner;
  element.disabled = false;
}

async function playTestSound(element) {
  startInner = element.innerHTML;
  element.innerHTML = "Playing";
  element.disabled = true;
  try {
    await apiFormURLEncodedPost("/api/play-test-sound", { volume: 10 });
  } catch (e) {
    alert("failed to play audio");
  }
  element.innerHTML = startInner;
  element.disabled = false;
}

async function resetAudiobait(element) {
  element.innerHTML = "Resetting";
  element.disabled = true;
  res = await fetch("/api/service-restart", {
    method: "POST",
    headers: authHeaders,
    body: new URLSearchParams({ service: "audiobait" }),
  });
  if (res.ok) {
    // Reload the page after a few seconds giving audiobait time to reset
    await delay(3000);
    window.location.reload();
  } else {
    log.Println(res);
  }
  element.innerHTML = "Reset";
  element.disabled = false;
}

function loadLogEntries(modal) {
  // Clear output text
  modal.find("p#outputText").prop("textContent", "");

  // Disable load button while waiting for response
  var button = document.getElementById("showLogEntries");
  button.disabled = true;
  // Also make it so the modal can't be closed while the test is running.
  modal.find("button#closeButton").prop("disabled", true);
  modal.find("button#crossCloseButton").prop("disabled", true);

  // Show spinner
  modal.find("div#loadingSpinner").prop("hidden", false);

  fetch("/api/logs?service=audiobait&lines=20", {
    method: "GET",
    headers: authHeaders,
  })
    .then(function (response) {
      console.log(response);
      return response.json();
    })
    .then(function (lines) {
      // Hide spinner
      modal.find("div#loadingSpinner").prop("hidden", true);

      // Show output text.
      modal.find("p#outputText").prop("textContent", lines.join("\n"));

      // Enable buttons again.
      button.disabled = false;
      modal.find("button#closeButton").prop("disabled", false);
      modal.find("button#crossCloseButton").prop("disabled", false);
    });
}

function testAudio(element) {
  volume = document.getElementById("volume").value;
  id = document.getElementById("test-audio-select").value;
  playSound(element, id, volume);
}

const delay = (ms) => new Promise((res) => setTimeout(res, ms));

$("#logEntriesModal").on("show.bs.modal", function (event) {
  loadLogEntries($(this));
});
