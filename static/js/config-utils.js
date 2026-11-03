"use strict";

// Helpers shared by the pages that show and save device config (config.js, trap.js).

// fetchConfig returns the device config as {values, defaults}, keyed by camelCase section.
async function fetchConfig() {
  const response = await fetch("/api/config", {
    headers: {
      Authorization: "Basic " + btoa("admin:feathers"),
    },
  });
  if (!response.ok) {
    throw new Error("Failed to load config");
  }
  return response.json();
}

function setNumberInput(selector, value) {
  if (value !== undefined && value !== null) {
    document.querySelector(selector).value = value;
  }
}

function formatDuration(nanoseconds) {
  if (!nanoseconds) {
    return "";
  }
  const seconds = Math.floor(nanoseconds / 1_000_000_000);
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const remainingSeconds = seconds % 60;

  return `${hours > 0 ? `${hours}h` : ""}${minutes > 0 ? `${minutes}m` : ""}${remainingSeconds}s`;
}

// saveConfig saves the given keys of a config section. Keys that are left out keep their
// current values. Returns whether it was saved, so the page can reload what it shows.
async function saveConfig(section, data) {
  const formData = new FormData();
  formData.append("section", section);
  formData.append("config", JSON.stringify(data));

  try {
    const response = await fetch("/api/config", {
      method: "POST",
      headers: {
        Authorization: "Basic " + btoa("admin:feathers"),
      },
      body: formData,
    });

    if (!response.ok) {
      throw new Error("Failed to save config");
    }
    alert(`${section} config saved`);
    return true;
  } catch (error) {
    console.error("Error saving config:", error);
    alert("Error saving config");
    return false;
  }
}
