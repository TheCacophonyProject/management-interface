"use strict";
window.onload = function() {
  getState()
}

async function getState() {
  try {
    var response = await apiGetJSON('/api/clock');
    console.log(response)
    $("#rtc-date-utc").html(response.RTCTimeUTC)
    $("#rtc-date-local").html(response.RTCTimeLocal)
    $("#system-date").html(response.SystemTime)
    if (response.LowRTCBattery) {
      $("#rtc-battery").html("Low/Empty. Replace soon.")
    } else {
      $("#rtc-battery").html("OK.")
    }
    if (response.RTCIntegrity) {
      $("#rtc-integrity").html("True.")
    } else {
      $("#rtc-integrity").html("False. Don't trust time from RTC")
    }
    if (response.NTPSynced) {
      $("#ntp-synced").html("True.")
    } else {
      $("#ntp-synced").html("False.")
    }
  } catch(e) {
    console.log(e)
  }
}

async function setTime() {
  var now = new Date()
  var data = {"date": now.toISOString()}
  try {
    await apiFormURLEncodedPost('/api/clock', data)
    alert("udpated time")
    getState()
  } catch(e) {
    console.log(e);
    alert("failed to update time")
  }
}
