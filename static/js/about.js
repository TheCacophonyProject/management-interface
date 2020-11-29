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
            if (response.success) {
                alert("Salt connected and accepted")
            } else {
                alert(response.output)
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