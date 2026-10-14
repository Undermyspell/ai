// Tastatur-Labeln auf /ml-daten: Zeile anklicken (oder ↓), dann 1/2/3 für
// Zusage/Absage/invalid, 0 verwirft eine Synthetics-Nachricht. Nach dem Label
// springt der Fokus zur nächsten Zeile. Hängt per Delegation am document,
// greift also auch nach jedem HTMX-Tausch.
(function () {
  "use strict";

  var KEYS = { "1": "true", "2": "false", "3": "invalid", "0": "verworfen" };
  var advanceFrom = null; // id der Zeile, nach deren Tausch weitergesprungen wird

  function list() { return document.querySelector("[data-ml-keys]"); }

  function rows() {
    var l = list();
    return l ? Array.prototype.slice.call(l.querySelectorAll(".ml-row")) : [];
  }

  function current() {
    var a = document.activeElement;
    return a && a.closest ? a.closest("[data-ml-keys] .ml-row") : null;
  }

  function focusRow(r) {
    if (!r) return;
    rows().forEach(function (el) { el.classList.remove("is-focus"); });
    r.classList.add("is-focus");
    r.focus({ preventScroll: true });
    r.scrollIntoView({ block: "nearest" });
  }

  document.addEventListener("keydown", function (e) {
    if (!list() || e.ctrlKey || e.metaKey || e.altKey) return;
    var t = e.target;
    if (t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable)) return;

    var all = rows();
    var cur = current();
    var i = cur ? all.indexOf(cur) : -1;

    if (e.key === "ArrowDown" || e.key === "j") {
      if (!cur && e.key === "ArrowDown") return; // ohne Auswahl normal scrollen
      e.preventDefault();
      focusRow(all[Math.min(i + 1, all.length - 1)]);
      return;
    }
    if (e.key === "ArrowUp" || e.key === "k") {
      if (!cur) return;
      e.preventDefault();
      focusRow(all[Math.max(i - 1, 0)]);
      return;
    }

    var label = KEYS[e.key];
    if (!label || !cur) return;
    var btn = cur.querySelector('[data-ml-label="' + label + '"]');
    if (!btn) return;
    e.preventDefault();
    advanceFrom = cur.id;
    btn.click();
  });

  // Klick in eine Zeile wählt sie für die Tastatur aus.
  document.addEventListener("focusin", function (e) {
    var r = e.target && e.target.closest ? e.target.closest("[data-ml-keys] .ml-row") : null;
    if (r && !r.classList.contains("is-focus")) {
      rows().forEach(function (el) { el.classList.remove("is-focus"); });
      r.classList.add("is-focus");
    }
  });

  document.addEventListener("htmx:afterSettle", function () {
    if (!advanceFrom) return;
    var r = document.getElementById(advanceFrom);
    advanceFrom = null;
    if (!r) return;
    var next = r.nextElementSibling;
    focusRow(next && next.classList.contains("ml-row") ? next : r);
  });
})();
