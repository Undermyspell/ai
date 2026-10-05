// Hell/Dunkel. Läuft synchron im <head>, damit die Seite nicht erst im
// falschen Theme aufblitzt. Ohne gespeicherte Wahl gilt die Systemeinstellung.
(function () {
  var KEY = "zumba-admin-theme";
  var root = document.documentElement;

  function stored() {
    try { return localStorage.getItem(KEY); } catch (e) { return null; }
  }

  function effectiveTheme(s) {
    if (s === "light" || s === "dark") return s;
    return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
  }

  function apply(theme) {
    root.setAttribute("data-theme", theme);
    document.querySelectorAll(".theme-toggle .icon").forEach(function (el) {
      el.textContent = theme === "dark" ? "☾" : "☀";
    });
  }

  apply(effectiveTheme(stored()));

  document.addEventListener("DOMContentLoaded", function () {
    apply(effectiveTheme(stored()));

    document.addEventListener("click", function (e) {
      if (!e.target.closest(".theme-toggle")) return;
      e.preventDefault();
      var next = root.getAttribute("data-theme") === "dark" ? "light" : "dark";
      try { localStorage.setItem(KEY, next); } catch (err) { /* privates Fenster */ }
      apply(next);
    });

    window.matchMedia("(prefers-color-scheme: light)").addEventListener("change", function (e) {
      if (stored()) return;
      apply(e.matches ? "light" : "dark");
    });
  });
})();
