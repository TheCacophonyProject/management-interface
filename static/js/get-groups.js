const groupStorageKey = "cacophonyGroups";

function loadGroupFromURL() {
  var url = new URL(window.location.href);
  var groupsEncoded = url.searchParams.get("groups");
  if (groupsEncoded) {
    var groups = groupsEncoded.split("--");
    console.log(groups);
    localStorage.setItem(groupStorageKey, JSON.stringify(groups));
  }
};

function hasGroupList() {
  return localStorage.getItem(groupStorageKey) && true;
}

function getGroups() {
  var g = localStorage.getItem(groupStorageKey);
  console.log(g);
  return JSON.parse(g);
}

loadGroupFromURL();
