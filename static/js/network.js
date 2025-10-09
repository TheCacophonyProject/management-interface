(function () {
  const AUTH_HEADER = "Basic " + btoa("admin:feathers");
  const select = document.getElementById("hotspotInterfaceSelect");
  const applyBtn = document.getElementById("hotspotInterfaceApply");
  const feedback = document.getElementById("hotspotInterfaceFeedback");

  if (!select || !applyBtn) {
    return;
  }

  function setFeedback(message, variant) {
    if (!feedback) {
      return;
    }
    const baseClass = "ml-3";
    if (!message) {
      feedback.className = baseClass + " text-muted";
      feedback.textContent = "";
      return;
    }
    let variantClass = " text-muted";
    if (variant === "success") {
      variantClass = " text-success";
    } else if (variant === "error") {
      variantClass = " text-danger";
    }
    feedback.className = baseClass + variantClass;
    feedback.textContent = message;
  }

  async function fetchHotspotConfig() {
    let loaded = false;
    select.disabled = true;
    applyBtn.disabled = true;
    try {
      const response = await fetch("/api/network/hotspot", {
        cache: "no-store",
        headers: {
          Authorization: AUTH_HEADER,
        },
      });
      if (!response.ok) {
        if (response.status === 501) {
          const text = await response.text();
          setFeedback(
            text.trim() || "Hotspot interface selection not supported.",
            "info"
          );
          select.innerHTML = "";
          const opt = document.createElement("option");
          opt.value = "";
          opt.textContent = "Automatic selection";
          select.appendChild(opt);
          return false;
        }
        const text = await response.text();
        throw new Error(text || "Failed to load hotspot configuration");
      }
      const data = await response.json();
      const available = Array.isArray(data.available) ? data.available : [];
      const selected =
        typeof data.selected === "string" ? data.selected : "";

      select.innerHTML = "";
      const autoOption = document.createElement("option");
      autoOption.value = "";
      autoOption.textContent = "Automatic selection";
      select.appendChild(autoOption);

      available.forEach((iface) => {
        const option = document.createElement("option");
        option.value = iface;
        option.textContent = iface;
        select.appendChild(option);
      });

      select.value = selected || "";
      if (selected && select.value !== selected) {
        const missingOption = document.createElement("option");
        missingOption.value = selected;
        missingOption.textContent = `${selected} (unavailable)`;
        select.appendChild(missingOption);
        select.value = selected;
      }

      loaded = true;
      return true;
    } catch (error) {
      console.error(error);
      setFeedback(
        error.message || "Failed to load hotspot interfaces",
        "error"
      );
      return false;
    } finally {
      select.disabled = !loaded;
      applyBtn.disabled = !loaded;
      if (!loaded) {
        applyBtn.disabled = true;
      }
    }
  }

  async function applyHotspotInterface() {
    select.disabled = true;
    applyBtn.disabled = true;
    setFeedback("Applying selection...", "info");
    try {
      const payload = {
        interface: select.value || "",
      };
      const response = await fetch("/api/network/hotspot", {
        method: "POST",
        headers: {
          Authorization: AUTH_HEADER,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload),
      });
      if (!response.ok) {
        if (response.status === 501) {
          const text = await response.text();
          setFeedback(
            text.trim() || "Hotspot interface selection not supported.",
            "info"
          );
          select.disabled = true;
          applyBtn.disabled = true;
          return;
        }
        const text = await response.text();
        throw new Error(text || "Failed to update hotspot interface");
      }

      const loaded = await fetchHotspotConfig();
      if (loaded) {
        const applied = select.value;
        if (applied) {
          setFeedback(`Hotspot will use ${applied}`, "success");
        } else {
          setFeedback(
            "Hotspot interface set to automatic selection",
            "success"
          );
        }
      }
    } catch (error) {
      console.error(error);
      setFeedback(
        error.message || "Failed to update hotspot interface",
        "error"
      );
      select.disabled = false;
      applyBtn.disabled = false;
    }
  }

  document.addEventListener("DOMContentLoaded", function () {
    fetchHotspotConfig();
    applyBtn.addEventListener("click", function (event) {
      event.preventDefault();
      applyHotspotInterface();
    });
  });
})();
