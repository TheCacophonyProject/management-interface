function apiGetJSON(url) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("GET", url, true);
    xhr.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));
    xhr.onload = () => {
      if (200 <= xhr.status && xhr.status < 300) {
        resolve(JSON.parse(xhr.responseText));
      } else {
        reject(xhr);
      }
    };
    xhr.onerror = () => reject(xhr);
    xhr.send();
  });
}

function apiFormURLEncodedPost(url, data) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", url, true);
    xhr.setRequestHeader("Authorization", "Basic " + btoa("admin:feathers"));
    xhr.setRequestHeader(
      "Content-type",
      "application/x-www-form-urlencoded; charset=UTF-8"
    );
    xhr.onload = () => {
      if (200 <= xhr.status && xhr.status < 300) {
        resolve(xhr.responseText);
      } else {
        // The API handlers write the reason for a failure to the body, so use
        // that when there is one.
        reject(xhr.responseText || xhr.statusText);
      }
    };
    xhr.onerror = () => reject(xhr);
    xhr.send($.param(data));
  });
}
