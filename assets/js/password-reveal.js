/**
 * Password show/hide: wire buttons with client-login-pw-toggle="<input id>".
 * Expects a sibling or referenced <input type="password"> by id.
 */
(function () {
  "use strict";

  function syncButton(btn, input) {
    var hidden = input.type === "password";
    var i = btn.querySelector("i");
    if (i) {
      i.className = hidden ? "bi bi-eye" : "bi bi-eye-slash";
    } else {
      btn.textContent = hidden ? "Show" : "Hide";
    }
    btn.setAttribute("aria-label", hidden ? "Show password" : "Hide password");
    btn.setAttribute("aria-pressed", hidden ? "false" : "true");
  }

  function wire(btn) {
    var tid = btn.getAttribute("client-login-pw-toggle");
    if (!tid) return;
    var input = document.getElementById(tid);
    if (!input || (input.type !== "password" && input.type !== "text")) return;
    syncButton(btn, input);
    btn.addEventListener("click", function (e) {
      e.preventDefault();
      input.type = input.type === "password" ? "text" : "password";
      syncButton(btn, input);
    });
  }

  function init() {
    document.querySelectorAll("[client-login-pw-toggle]").forEach(wire);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
