(function () {
  "use strict";

  function pad(n) {
    return String(n).padStart(2, "0");
  }

  // Render <time datetime> elements in the viewer's local timezone, keeping the
  // same "YYYY-MM-DD HH:MM" shape.
  document.querySelectorAll("time[datetime]").forEach(function (el) {
    var d = new Date(el.getAttribute("datetime"));
    if (isNaN(d.getTime())) return;
    el.textContent =
      d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate()) +
      " " + pad(d.getHours()) + ":" + pad(d.getMinutes());
  });

  // Open/close <dialog> modals via data attributes.
  document.querySelectorAll("[data-open-dialog]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var d = document.querySelector(btn.getAttribute("data-open-dialog"));
      if (d && !d.open) d.showModal();
    });
  });
  document.querySelectorAll("[data-close-dialog]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var d = btn.closest("dialog");
      if (d) d.close();
    });
  });
})();
