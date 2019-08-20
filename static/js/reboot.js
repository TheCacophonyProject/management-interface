function reboot() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open('POST', '/api/reboot', true);
  xmlHttp.setRequestHeader("Authorization", "Basic "+btoa("admin:feathers"))
  xmlHttp.onload = async function() {
    if (xmlHttp.status == 200) {
      alert("device will reboot shortly. Please go back to sidekick to look for the device again")
    } else {
      rebootError();
    }
  }
  xmlHttp.onerror = async function() {
    rebootError();
  }
  xmlHttp.send();
}

function rebootError() {
  alert("Error rebooting device. Please manually reboot the deice via the reset button on the device or unplugging and plugging in the power.")
}
