const groupStorageKey = "cacophonyGroups";

function loadGroupFromURL() {
  var url = new URL(window.location.href);
  var groupsEncoded = url.searchParams.get("groups");
  if (groupsEncoded) {
    var groups = groupsEncoded.split("--");
    localStorage.setItem(groupStorageKey, JSON.stringify(groups));
  }
};

function hasGroupList() {
  return localStorage.getItem(groupStorageKey) && true;
}

function getGroups() {
  var g = localStorage.getItem(groupStorageKey);
  return JSON.parse(g);
}

loadGroupFromURL();
