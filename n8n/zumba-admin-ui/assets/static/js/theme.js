(function () {
  var KEY = "zumba-admin-theme";
  var root = document.documentElement;

  function effectiveTheme(stored) {
    if (stored === "light" || stored === "dark") return stored;
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }

  function apply(theme) {
    root.setAttribute("data-theme", theme);
    var btn = document.querySelector(".theme-toggle");
    if (btn) {
      btn.querySelector(".icon").textContent = theme === "dark" ? "☾" : "☀";
      btn.querySelector(".label").textContent = theme === "dark" ? "Nachtschicht" : "Tagesbar";
    }
  }

  // Run synchronously to avoid FOUC. The script is loaded in <head>.
  apply(effectiveTheme(localStorage.getItem(KEY)));

  document.addEventListener("DOMContentLoaded", function () {
    apply(effectiveTheme(localStorage.getItem(KEY)));

    document.addEventListener("click", function (e) {
      var btn = e.target.closest(".theme-toggle");
      if (!btn) return;
      e.preventDefault();
      var current = root.getAttribute("data-theme") || "light";
      var next = current === "dark" ? "light" : "dark";
      localStorage.setItem(KEY, next);
      apply(next);
    });

    // Follow system if user hasn't picked manually
    window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", function (e) {
      if (localStorage.getItem(KEY)) return;
      apply(e.matches ? "dark" : "light");
    });
  });
})();
