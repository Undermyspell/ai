// Kopierknopf der Öffentlich-Seite. Delegiert am document, damit er auch nach
// jedem HTMX-Tausch der Statuskarten noch funktioniert.
document.addEventListener('click', function (e) {
  var btn = e.target.closest('[data-copy]');
  if (!btn || !navigator.clipboard) return;
  navigator.clipboard.writeText(btn.dataset.copy).then(function () {
    var before = btn.textContent;
    btn.textContent = 'Kopiert';
    btn.classList.add('is-copied');
    setTimeout(function () {
      btn.textContent = before;
      btn.classList.remove('is-copied');
    }, 1200);
  });
});
