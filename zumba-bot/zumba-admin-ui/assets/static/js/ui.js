// Kleine Helfer für die Glas-Oberfläche. Alles hängt per Delegation am
// document, damit es auch nach jedem HTMX-Tausch weiter greift.
(function () {
  "use strict";

  /* ── „Mehr"-Menü (Handy) ─────────────────────────────────────────── */
  function closeMore() {
    var m = document.querySelector(".more");
    if (m) m.open = false;
  }

  /* ── Bottom-Sheet ────────────────────────────────────────────────── */
  var opener = null;

  function closeSheet() {
    var s = document.getElementById("sheet");
    if (!s || !s.firstElementChild) return;
    s.innerHTML = "";
    document.body.classList.remove("has-sheet");
    if (opener && opener.isConnected) opener.focus();
    opener = null;
  }

  document.addEventListener("htmx:beforeRequest", function (e) {
    var t = e.detail.target;
    if (t && t.id === "sheet" && !document.body.classList.contains("has-sheet")) opener = e.detail.elt;
  });

  /* ── Matrix: Spalte auswählen ────────────────────────────────────── */
  var selKey = null;

  function applySel() {
    var m = document.querySelector(".matrix");
    if (!m) return;
    var key = selKey || m.dataset.sel;
    m.querySelectorAll(".is-sel").forEach(function (el) { el.classList.remove("is-sel"); });
    if (!key) return;
    m.querySelectorAll('[data-key="' + key + '"]').forEach(function (el) { el.classList.add("is-sel"); });
    var head = m.querySelector('.mx-col[data-key="' + key + '"]');
    var info = document.getElementById("mx-info");
    if (info && head) info.textContent = head.dataset.info || "";
  }

  /* ── Beim Laden: an das Ende scrollen (neueste Donnerstage rechts) ──
     Lädt sich #page nach einem Klick neu, bleibt die alte Position stehen –
     sonst spränge die Matrix nach jedem Umschalten zurück ans Ende. */
  var kept = {};

  document.addEventListener("htmx:beforeSwap", function () {
    document.querySelectorAll("[data-scroll-end][id]").forEach(function (el) {
      kept[el.id] = el.scrollLeft;
    });
  });

  function scrollEnds() {
    document.querySelectorAll("[data-scroll-end]").forEach(function (el) {
      if (el.dataset.scrolled) return;
      el.dataset.scrolled = "1";
      if (el.id && kept[el.id] != null) {
        el.scrollLeft = kept[el.id];
        delete kept[el.id];
      } else {
        el.scrollLeft = el.scrollWidth;
      }
    });
    var chip = document.querySelector(".sheet-strip .is-active:not([data-seen])");
    if (chip) {
      chip.dataset.seen = "1";
      chip.scrollIntoView({ block: "nearest", inline: "center" });
    }
  }

  /* ── Bot-Test: Sprechblase ↔ Webhook-JSON ────────────────────────── */
  function el(id) { return document.getElementById(id); }

  function readJSON() {
    var j = el("bot-json");
    if (!j) return null;
    try { return JSON.parse(j.value); } catch (e) { return null; }
  }

  // Das JSON hat Vorrang: es ist das Formularfeld, das abgeschickt wird. Die
  // Blase zeigt nur message.conversation daraus und schreibt dorthin zurück.
  function bubbleFromJSON() {
    var d = readJSON(), b = el("bot-msg");
    if (!d || !b) return;
    var data = d.data || {};
    var msg = data.message && data.message.conversation;
    if (typeof msg === "string") b.value = msg;
    var who = el("bot-sender");
    if (who && data.pushName) who.textContent = data.pushName;
  }

  function jsonFromBubble() {
    var d = readJSON(), b = el("bot-msg");
    if (!d || !b) return; // kaputtes JSON nicht überschreiben
    d.data = d.data || {};
    d.data.message = d.data.message || {};
    d.data.message.conversation = b.value;
    el("bot-json").value = JSON.stringify(d, null, 2);
  }

  function resetBot() {
    var r = el("bot-response"), p = el("bot-path-result");
    if (r) r.innerHTML = "";
    if (p) p.innerHTML = "";
  }

  /* ── Ereignisse ──────────────────────────────────────────────────── */
  document.addEventListener("click", function (e) {
    if (e.target.closest("[data-sheet-close]")) {
      e.preventDefault();
      closeSheet();
      return;
    }
    var more = document.querySelector(".more");
    if (more && more.open && !more.contains(e.target)) more.open = false;

    var c = e.target.closest(".matrix .mx-col, .matrix .mx-cell");
    if (c && c.dataset.key) {
      selKey = c.dataset.key;
      applySel();
    }
  });

  document.addEventListener("keydown", function (e) {
    if (e.key !== "Escape") return;
    closeSheet();
    closeMore();
  });

  document.addEventListener("input", function (e) {
    var id = e.target.id;
    if (id === "bot-msg") { jsonFromBubble(); resetBot(); }
    else if (id === "bot-json") { bubbleFromJSON(); resetBot(); }
    var msg = e.target.closest("form") && e.target.closest("form").querySelector(".form-msg");
    if (msg) msg.textContent = "";
  });

  document.addEventListener("change", function (e) {
    if (e.target.name === "szenario") resetBot();
  });

  // Sperrtag per Datum: nur Donnerstage. Der Server prüft es ohnehin, die
  // Meldung hier spart nur den Umweg über den Toast.
  document.addEventListener("submit", function (e) {
    var f = e.target.closest("[data-thursday-form]");
    if (!f) return;
    var v = (f.querySelector('input[type="date"]') || {}).value;
    var bad = "";
    if (!v) bad = "Bitte Datum wählen.";
    else if (new Date(v + "T00:00:00").getDay() !== 4) bad = "Kein Donnerstag – nur Donnerstage sind zulässig.";
    if (!bad) return;
    e.preventDefault();
    e.stopPropagation();
    var msg = f.querySelector(".form-msg");
    if (msg) msg.textContent = bad;
  }, true);

  document.addEventListener("htmx:afterSwap", function (e) {
    var t = e.detail.target;
    if (!t) return;
    if (t.id === "sheet") {
      document.body.classList.add("has-sheet");
      var panel = t.querySelector(".sheet");
      if (panel) panel.focus();
    }
    if (t.id === "bot-json") {
      // innerHTML ändert bei einer schon bearbeiteten Textarea nur den
      // Vorgabewert – den angezeigten Wert explizit nachziehen.
      t.value = t.textContent;
      bubbleFromJSON();
    }
  });

  document.addEventListener("htmx:load", function () {
    applySel();
    scrollEnds();
  });

  applySel();
  scrollEnds();
  bubbleFromJSON();
})();
