 function checkSaltConnection() {
    $("#check-salt-button").attr('disabled', true);
    var xmlHttp = new XMLHttpRequest();
    xmlHttp.open("GET", "/api/check-salt-connection", true);
    xmlHttp.setRequestHeader("Authorization", "Basic "+btoa("admin:feathers"))
  
    xmlHttp.timeout = 20000; // Set timeout for 20 seconds
    xmlHttp.onload = async function() {
        if (xmlHttp.status == 200) {
            var response = JSON.parse(xmlHttp.response);
            console.log(response);
            if (response.LastCallSuccess) {
                alert("Salt connected and accepted")
            } else {
                alert("Salt ping failed:" + response.LastCallOut)
            }
        } else {
            console.log("error with checking salt connection");
        }
        $("#check-salt-button").attr('disabled', false);
    }
    xmlHttp.onerror = async function() {
      console.log("error with checking salt connection");
      $("#check-salt-button").attr('disabled', false);
    }
    xmlHttp.ontimeout = async function() {
        alert("connection timeout")
        $("#check-salt-button").attr('disabled', false);
    }
    xmlHttp.send(null)
 }

function runSaltUpdate() {
    var xmlHttp = new XMLHttpRequest();
    xmlHttp.open("POST", "/api/salt-update", true);
    xmlHttp.setRequestHeader("Authorization", "Basic "+btoa("admin:feathers"))
    xmlHttp.onload = async function() {
        if (xmlHttp.status == 200) {
            $("#salt-update-button").attr('disabled', true);
            $("#salt-update-button").html("Running Salt Update..."); 
            pollSaltUpdateState()       
        } else {
            console.log(response)
        }
    }
    xmlHttp.onerror = async function() {
        console.log("error with running salt update");
    }

    xmlHttp.send(null)
}


var runningSaltUpdate = true;
// Check salt update state. Returns true if it is no longer running
function checkSaltUpdateState() {
    console.log("checking salt update state");
    var xmlHttp = new XMLHttpRequest();
    xmlHttp.open("GET", "/api/salt-update", true);
    xmlHttp.setRequestHeader("Authorization", "Basic "+btoa("admin:feathers"))
    xmlHttp.onload = async function() {
        if (xmlHttp.status == 200) {
            var response = JSON.parse(xmlHttp.response);
            runningSaltUpdate = response.RunningUpdate
            if (!runningSaltUpdate) {
                $("#salt-update-button").attr('disabled', false);
                $("#salt-update-button").html("Run Salt Update..."); 
                if (response.LastCallSuccess) {
                    alert("Salt update finished")
                    // Reload page to update values
                    location.reload();
                } else {
                    alert("Salt update failed")
                }
            }
        }
        console.log(response)
    }
    xmlHttp.onerror = async function() {
        $("#salt-update-button").attr('disabled', false);
        $("#salt-update-button").html("Run Salt Update...");
        console.log("error with running salt update");
    }
    xmlHttp.send(null)
} 

// Will check the salt update state until it has finished
function pollSaltUpdateState() {
    if (runningSaltUpdate == false) {
        return;
    }
    checkSaltUpdateState()
    setTimeout(pollSaltUpdateState, 3000)
}