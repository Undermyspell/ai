// Das Mehr-Menü ist ein <details>: auf- und zuklappen kann der Browser selbst.
// Auf dem Desktop ist es keine Klappe, sondern die zweite Gruppe der
// Seitenleiste – dort bleibt es dauerhaft offen. Dazu Tippen daneben und Escape.
(function () {
  var desktop = window.matchMedia("(min-width: 768px)");

  function menu() {
    return document.querySelector(".nav-more");
  }

  function sync() {
    var m = menu();
    if (m) m.open = desktop.matches;
  }

  document.addEventListener("DOMContentLoaded", function () {
    sync();
    desktop.addEventListener("change", sync);

    document.addEventListener("click", function (e) {
      var m = menu();
      if (desktop.matches || !m || !m.open || m.contains(e.target)) return;
      m.open = false;
    });

    document.addEventListener("keydown", function (e) {
      var m = menu();
      if (e.key === "Escape" && !desktop.matches && m) m.open = false;
    });
  });
})();
