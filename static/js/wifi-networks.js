function closeAlert() {
  $("#errorAlert").css("visibility", "hidden");
}

let passwordVisibility = false;
function showHidePassword(e) {
  e.preventDefault();
  if (passwordVisibility) {
    $("#show-password").show();
    $("#hide-password").hide();
    $("#text-password").attr("type", "password");
  } else {
    $("#show-password").hide();
    $("#hide-password").show();
    $("#text-password").attr("type", "text");
  }
  passwordVisibility = !passwordVisibility;
}

function addNetwork() {
  $("#add-network-button").prop("disabled", true);
  $("#add-network-button").css("opacity", "0.5");
  $("#add-network-button").text("Adding Network...");

  var ssid = document.getElementById("text-ssid").value;
  var ssid = document.getElementById("text-ssid").value;
  if (ssid == "") {
    ssid = document.getElementById("ssid-select").value;
  }
  var password = document.getElementById("text-password").value;

  // Prepare the request data
  var data = {
    ssid: ssid,
    psk: password,
  };

  // Send POST request to the server
  fetch(
    "/api/wifi-networks?ssid=" +
      encodeURIComponent(ssid) +
      "&psk=" +
      encodeURIComponent(password),
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Basic " + btoa("admin:feathers"),
      },
    }
  )
    .then((response) => {
      if (!response.ok) {
        throw new Error("Failed to add network");
      }
      return;
    })
    .catch((error) => {
      console.error("Error adding network:", error);
    })
    .finally(() => {
      setTimeout(function () {
        $("#add-network-button").prop("disabled", false);
        $("#add-network-button").css("opacity", "1");
        $("#add-network-button").text("Add Network");
        location.reload();
      }, 500);
    });
}

function removeNetwork(ssid) {
  // Send DELETE request to the server
  fetch("/api/wifi-networks?ssid=" + encodeURIComponent(ssid), {
    method: "DELETE",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Basic " + btoa("admin:feathers"),
    },
  })
    .then((response) => {
      if (!response.ok) {
        console.log(response);
        throw new Error("Network removal failed");
      }
      return;
    })
    .then((data) => {
      console.log("Network removed:", data);
      setTimeout(function () {
        location.reload();
      }, 500);
    })
    .catch((error) => {
      console.error("Error removing network:", error);
    });
}

function switchToWifi() {
  fetch("/api/enable-wifi", {
    method: "POST",
    headers: { Authorization: "Basic " + btoa("admin:feathers") },
  })
    .then((response) => {
      if (!response.ok) {
        console.log(response);
        throw new Error("Failed to enable wifi");
      }
      return;
    })
    .catch((error) => {
      console.error("Error removing network:", error);
    });
}

$("#toggle-password").click(showHidePassword);
