function closeAlert(){
  $("#errorAlert").css("visibility","hidden");
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
  // Get the SSID and password from the form
  var ssid = document.getElementById('text-ssid').value;
  var ssid = document.getElementById('text-ssid').value;
  if (ssid == "") {
    ssid = document.getElementById('ssid-select').value;
  }
  var password = document.getElementById('text-password').value;

  // Prepare the request data
  var data = {
    ssid: ssid,
    psk: password
  };

  // Send POST request to the server
  fetch('/api/wifi-networks?ssid=' + encodeURIComponent(ssid) + '&psk=' + encodeURIComponent(password), { // Replace with your actual API endpoint
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Basic ' + btoa('admin:feathers') // Update with actual credentials
    },
    //body: JSON.stringify(data)
  })
  .then(response => {
    if (!response.ok) {
      throw new Error('Failed to add network');
    }
    return;
  })
  .then(data => {
    console.log('Network added:', data);
    location.reload();
  })
  .catch(error => {
    console.error('Error adding network:', error);
  });
}


function removeNetwork(ssid) {
  console.log("remove network " + ssid);

  // Prepare the request options
  const requestOptions = {
    method: 'DELETE',
     headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Basic ' + btoa('admin:feathers')
    }
  };

  // Send DELETE request to the server
  fetch('/api/wifi-networks?ssid=' + encodeURIComponent(ssid), requestOptions)
    .then(response => {
      if (!response.ok) {
        console.log(response)
        throw new Error('Network removal failed');
      }
      return;
    })
    .then(data => {
      console.log('Network removed:', data);
      // Refresh the page
      location.reload();
    })
    .catch(error => {
      console.error('Error removing network:', error);
    });
}

$('#toggle-password').click(showHidePassword);
