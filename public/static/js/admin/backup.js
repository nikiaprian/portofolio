(function () {
  "use strict";

  var API_EXPORT = "/api/v1/admin/database/export";
  var API_IMPORT = "/api/v1/admin/database/import";
  var TOKEN_KEY = "tirtayasa_token";
  var LOGIN =
    (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) || "/login";

  function qs(sel) {
    return document.querySelector(sel);
  }

  function token() {
    try {
      return (
        localStorage.getItem(TOKEN_KEY) ||
        sessionStorage.getItem(TOKEN_KEY) ||
        ""
      );
    } catch (e) {
      return "";
    }
  }

  function setMsg(el, text) {
    if (!el) return;
    if (!text) {
      el.classList.add("d-none");
      el.textContent = "";
      return;
    }
    el.classList.remove("d-none");
    el.textContent = text;
  }

  async function fetchAuth(url, opts) {
    var t = token();
    if (!t) {
      window.location.href = LOGIN;
      throw new Error("unauth");
    }
    opts = opts || {};
    opts.headers = Object.assign({}, opts.headers || {});
    opts.headers.Authorization = "Bearer " + t;
    var res = await fetch(url, opts);
    if (res.status === 401) {
      window.location.href = LOGIN;
      throw new Error("unauth");
    }
    return res;
  }

  function triggerDownloadBlob(blob, filename) {
    var u = URL.createObjectURL(blob);
    var a = document.createElement("a");
    a.href = u;
    a.download = filename || "database.sql";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(u);
  }

  function filenameFromDisposition(disposition) {
    var s = String(disposition || "");
    var m = s.match(/filename="?([^"]+)"?/i);
    if (!m || !m[1]) return "";
    return m[1];
  }

  document.addEventListener("DOMContentLoaded", function () {
    var AC = window.AdminCrud || null;
    var errEl = qs("#db-backup-error");
    var okEl = qs("#db-backup-info");
    var exportBtn = qs("#db-export-btn");
    var importForm = qs("#db-import-form");
    var importBtn = qs("#db-import-btn");
    var fileEl = qs("#db-import-file");

    if (exportBtn) {
      exportBtn.addEventListener("click", async function () {
        setMsg(errEl, null);
        setMsg(okEl, null);
        exportBtn.disabled = true;
        try {
          var res = await fetchAuth(API_EXPORT, { method: "GET" });
          if (!res.ok) {
            var data = {};
            try {
              data = await res.json();
            } catch (e) {}
            var em =
              (data && data.error ? String(data.error) : "Export failed.") +
              (data && data.detail ? "\nDetail: " + String(data.detail) : "");
            setMsg(errEl, em);
            return;
          }
          var cd = res.headers.get("content-disposition");
          var filename = filenameFromDisposition(cd) || "database.sql";
          var blob = await res.blob();
          triggerDownloadBlob(blob, filename);
          setMsg(okEl, "Export succeeded. Your SQL file is downloading.");
          if (AC && typeof AC.showSuccessToast === "function") {
            AC.showSuccessToast("SQL export completed.");
          }
        } catch (e) {
          if (e && e.message === "unauth") return;
          setMsg(errEl, "Network error during export.");
        } finally {
          exportBtn.disabled = false;
        }
      });
    }

    if (importForm) {
      importForm.addEventListener("submit", async function (e) {
        e.preventDefault();
        setMsg(errEl, null);
        setMsg(okEl, null);
        if (!fileEl || !fileEl.files || !fileEl.files[0]) {
          setMsg(errEl, "Please choose a .sql file first.");
          return;
        }
        var f = fileEl.files[0];
        var name = String(f.name || "").toLowerCase();
        var isSQL = name.endsWith(".sql");
        var isGzip = name.endsWith(".gz") || name.endsWith(".sql.gz");
        if (!isSQL && !isGzip) {
          setMsg(errEl, "File must be .sql or .sql.gz format.");
          return;
        }

        if (
          !window.confirm(
            "Import may overwrite existing database data. Continue?"
          )
        ) {
          return;
        }

        var fd = new FormData();
        fd.append("file", f);
        if (importBtn) importBtn.disabled = true;
        try {
          var res = await fetchAuth(API_IMPORT, { method: "POST", body: fd });
          var data = {};
          try {
            data = await res.json();
          } catch (jerr) {}
          if (!res.ok) {
            if (res.status === 413) {
              setMsg(
                errEl,
                "Upload was rejected because it is too large for the gateway/proxy (HTTP 413). Try a smaller .sql.gz file, or increase upload limits on the server (Nginx client_max_body_size / Cloudflare upload limit / ingress limit)."
              );
              return;
            }
            var im =
              (data && data.error ? String(data.error) : "Import failed.") +
              (data && data.detail ? "\nDetail: " + String(data.detail) : "");
            setMsg(errEl, im);
            return;
          }
          setMsg(okEl, (data && data.message) || "Import succeeded.");
          importForm.reset();
          if (AC && typeof AC.showSuccessToast === "function") {
            AC.showSuccessToast("SQL import completed.");
          }
        } catch (err) {
          if (err && err.message === "unauth") return;
          setMsg(errEl, "Network error during import.");
        } finally {
          if (importBtn) importBtn.disabled = false;
        }
      });
    }
  });
})();
