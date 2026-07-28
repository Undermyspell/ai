// Renders toasts triggered by the server via the HX-Trigger header:
//   HX-Trigger: {"showToast":{"level":"success|error","msg":"..."}}
document.body.addEventListener("showToast", function (e) {
  var d = e.detail || {};
  var stack = document.getElementById("toast-stack");
  if (!stack) return;
  var el = document.createElement("div");
  el.className = "toast " + (d.level || "info");
  el.textContent = d.msg || "";
  stack.appendChild(el);
  setTimeout(function () {
    el.classList.add("toast-out");
    setTimeout(function () { el.remove(); }, 300);
  }, 3200);
});
