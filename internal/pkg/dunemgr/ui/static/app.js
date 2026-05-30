// dunemgr SSE glue. One EventSource per channel; the topic disambiguates,
// so every stream uses the default "message" event (onmessage).
(function () {
  "use strict";
  var root = document.querySelector("[data-host]");
  if (!root || typeof EventSource === "undefined") return;
  var host = root.getAttribute("data-host");

  function open(channel, handler) {
    var src = new EventSource("/events/" + encodeURIComponent(host) + "/" + channel);
    src.onmessage = function (e) {
      try {
        handler(JSON.parse(e.data));
      } catch (_) {
        /* ignore malformed frame */
      }
    };
    src.onerror = function () {
      // EventSource auto-reconnects; surface the gap on the tunnel badge.
      var b = document.getElementById("tunnel-badge");
      if (b) b.textContent = "○ reconnecting…";
    };
    return src;
  }

  function setText(id, text) {
    var el = document.getElementById(id);
    if (el) el.textContent = text;
  }

  open("bg", function (d) {
    setText("bg-state", d.state || "UNKNOWN");
    setText("pod-count", (d.ready || 0) + "/" + (d.total || 0));
    // When the Lifecycle tab is open, refresh it from the (freshly
    // written) cache. The SSE frame is just the trigger; the server
    // re-renders the table.
    if (document.getElementById("lifecycle-status") && window.htmx) {
      window.htmx.ajax("GET", "/host/" + encodeURIComponent(host) + "/lifecycle/_partial", {
        target: "#tab-body",
        swap: "innerHTML",
      });
    }
  });

  open("tunnel", function (d) {
    setText("tunnel-badge", d.up ? "● tunnel up" : "○ tunnel down");
  });

  open("actions", function (d) {
    setText("last-action", d.action + " (" + d.result + ") by " + d.operator);
  });
})();

// Dark-mode toggle: flips data-theme on <html> and remembers the choice.
(function () {
  "use strict";
  var KEY = "dunemgr-theme";
  var saved = null;
  try { saved = localStorage.getItem(KEY); } catch (_) {}
  if (saved) document.documentElement.setAttribute("data-theme", saved);
  var btn = document.getElementById("theme-toggle");
  if (!btn) return;
  btn.addEventListener("click", function () {
    var cur = document.documentElement.getAttribute("data-theme");
    var next = cur === "dark" ? "light" : "dark";
    document.documentElement.setAttribute("data-theme", next);
    try { localStorage.setItem(KEY, next); } catch (_) {}
  });
})();
