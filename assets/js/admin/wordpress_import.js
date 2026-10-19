(function () {
  "use strict";

  var API_PREVIEW = "/api/v1/admin/database/wordpress/preview";
  var API_IMPORT = "/api/v1/admin/database/wordpress/import";
  var TOKEN_KEY = "bebii_token";
  var LOGIN =
    (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login";

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

  function setMsg(el, text, isError) {
    if (!el) return;
    if (!text) {
      el.classList.add("d-none");
      el.textContent = "";
      return;
    }
    el.classList.remove("d-none");
    el.textContent = text;
    el.className = "alert " + (isError ? "alert-danger" : "alert-success");
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

  function validateFile() {
    var fileEl = qs("#wp-import-file");
    if (!fileEl || !fileEl.files || !fileEl.files[0]) {
      return null;
    }
    var f = fileEl.files[0];
    var name = String(f.name || "").toLowerCase();
    var isSQL = name.endsWith(".sql");
    var isGzip = name.endsWith(".gz") || name.endsWith(".sql.gz");
    if (!isSQL && !isGzip) {
      return null;
    }
    return f;
  }

  function getDownloadImages() {
    var cb = qs("#wp-download-images");
    return cb && cb.checked ? "true" : "false";
  }

  function showLogs(logs) {
    var logContainer = qs("#wp-logs");
    var logContent = qs("#wp-logs-content");
    if (!logs || !logs.length || !logContainer || !logContent) return;
    logContainer.classList.remove("d-none");
    logContent.textContent = logs.join("\n");
    logContent.scrollTop = logContent.scrollHeight;
  }

  function hideLogs() {
    var logContainer = qs("#wp-logs");
    if (logContainer) logContainer.classList.add("d-none");
  }

  function showPreview(data, logs) {
    var preview = qs("#wp-preview");
    if (!preview) return;
    preview.classList.remove("d-none");
    var p = data || {};
    setText("#wp-prev-authors", p.authors || 0);
    setText("#wp-prev-categories", p.categories || 0);
    setText("#wp-prev-tags", p.tags || 0);
    setText("#wp-prev-posts", p.posts || 0);
    setText("#wp-prev-pages", p.pages || 0);
    var importBtn = qs("#wp-import-btn");
    if (importBtn) importBtn.disabled = false;
    showLogs(logs);
  }

  function setText(sel, val) {
    var el = qs(sel);
    if (el) el.textContent = val;
  }

  function formatResult(r) {
    if (!r) return "Selesai.";
    var lines = [];
    lines.push("Author: " + (r.authors_created || 0) + " dibuat, " + (r.authors_skipped || 0) + " dilewati");
    lines.push("Category: " + (r.categories_created || 0) + " dibuat, " + (r.categories_skipped || 0) + " dilewati");
    lines.push("Tag: " + (r.tags_created || 0) + " dibuat, " + (r.tags_skipped || 0) + " dilewati");
    lines.push("Post: " + (r.posts_created || 0) + " dibuat, " + (r.posts_skipped || 0) + " dilewati");
    lines.push("Page: " + (r.pages_created || 0) + " dibuat, " + (r.pages_skipped || 0) + " dilewati");
    if (r.post_tag_links_created) {
      lines.push("Post-Tag links: " + r.post_tag_links_created);
    }
    if (r.images_downloaded) {
      lines.push("Gambar diunduh: " + r.images_downloaded + " berhasil" + (r.images_failed ? ", " + r.images_failed + " gagal" : ""));
    }
    return lines.join("\n");
  }

  // Smooth fake progress for Preview (single request, no streaming)
  function startPreviewProgress(modal) {
    var stages = [
      { max: 40, msg: "Membaca & memparsing SQL…" },
      { max: 75, msg: "Menghitung data…" },
      { max: 90, msg: "Menyusun hasil…" },
    ];
    var pct = 5;
    var si = 0;
    var timer = setInterval(function () {
      pct += 0.8 + Math.random() * 1.2;
      if (si < stages.length - 1 && pct >= stages[si].max) {
        si++;
      }
      if (pct > 90) pct = 90;
      if (modal && AC && AC.updateBulkLoadingModal) {
        AC.updateBulkLoadingModal(Math.round(pct), stages[si].msg);
      }
    }, 350);
    return timer;
  }

  document.addEventListener("DOMContentLoaded", function () {
    var errEl = qs("#wp-error");
    var okEl = qs("#wp-info");
    var previewBtn = qs("#wp-preview-btn");
    var importForm = qs("#wp-import-form");
    var importBtn = qs("#wp-import-btn");

    // Preview button - uses smooth fake progress
    if (previewBtn) {
      previewBtn.addEventListener("click", async function () {
        setMsg(errEl, null);
        setMsg(okEl, null);
        hideLogs();
        var f = validateFile();
        if (!f) {
          setMsg(errEl, "Pilih file .sql atau .sql.gz dari WordPress terlebih dahulu.", true);
          return;
        }
        previewBtn.disabled = true;
        previewBtn.innerHTML = '<span class="spinner-border spinner-border-sm me-1" role="status" aria-hidden="true"></span> Memproses...';

        var modal = null;
        if (typeof AC !== "undefined" && AC.openBulkLoadingModal) {
          modal = AC.openBulkLoadingModal("Preview WordPress", 100);
        }
        var ptimer = startPreviewProgress(modal);

        try {
          var fd = new FormData();
          fd.append("file", f);
          var res = await fetchAuth(API_PREVIEW, { method: "POST", body: fd });
          clearInterval(ptimer);
          if (modal && AC && AC.closeBulkLoadingModal) {
            if (res.ok) {
              if (AC.updateBulkLoadingModal) AC.updateBulkLoadingModal(100, "Selesai!");
              setTimeout(function () { AC.closeBulkLoadingModal(); }, 400);
            } else {
              AC.closeBulkLoadingModal();
            }
          }
          var data = {};
          try {
            data = await res.json();
          } catch (e) {}
          if (!res.ok) {
            var em = data.error || "Preview gagal.";
            if (data.detail) em += "\n" + data.detail;
            setMsg(errEl, em, true);
            showLogs(data.logs);
            return;
          }
          if (data.preview) {
            showPreview(data.preview, data.logs);
          }
        } catch (e) {
          clearInterval(ptimer);
          if (modal && AC && AC.closeBulkLoadingModal) AC.closeBulkLoadingModal();
          if (e && e.message === "unauth") return;
          setMsg(errEl, "Terjadi kesalahan saat preview.", true);
        } finally {
          previewBtn.disabled = false;
          previewBtn.innerHTML = '<i class="bi bi-eye me-1"></i> Preview';
        }
      });
    }

    // Import - uses smooth pseudo-progress (single JSON response)
    if (importForm) {
      importForm.addEventListener("submit", async function (e) {
        e.preventDefault();
        setMsg(errEl, null);
        setMsg(okEl, null);
        hideLogs();
        var f = validateFile();
        if (!f) {
          setMsg(errEl, "Pilih file .sql atau .sql.gz dari WordPress terlebih dahulu.", true);
          return;
        }
        var dlImages = getDownloadImages() === "true";

        var confirmText = "Import akan menambahkan data WordPress ke database. Lanjutkan?";
        if (dlImages) {
          confirmText += "\n\nGambar akan diunduh dari URL asli dan disimpan ke server.";
        }
        if (typeof Swal !== "undefined") {
          var confirmRes = await Swal.fire({
            icon: "question",
            title: "Konfirmasi Import",
            text: confirmText,
            showCancelButton: true,
            confirmButtonColor: "#dc3545",
            confirmButtonText: "Ya, Import!",
            cancelButtonText: "Batal",
          });
          if (!confirmRes.isConfirmed) return;
        } else if (!window.confirm(confirmText)) {
          return;
        }

        if (importBtn) importBtn.disabled = true;
        if (previewBtn) previewBtn.disabled = true;

        var modal = null;
        if (typeof AC !== "undefined" && AC.openBulkLoadingModal) {
          modal = AC.openBulkLoadingModal("Import WordPress", 100);
        }
        var ptimer = startPreviewProgress(modal);

        try {
          var fd = new FormData();
          fd.append("file", f);
          fd.append("download_images", getDownloadImages());

          var res = await fetchAuth(API_IMPORT, { method: "POST", body: fd });
          clearInterval(ptimer);
          if (modal && AC && AC.closeBulkLoadingModal) {
            if (res.ok) {
              if (AC.updateBulkLoadingModal) AC.updateBulkLoadingModal(100, "Selesai!");
              setTimeout(function () { AC.closeBulkLoadingModal(); }, 400);
            } else {
              AC.closeBulkLoadingModal();
            }
          }
          var data = {};
          try {
            data = await res.json();
          } catch (e) {}
          if (!res.ok) {
            var em = data.error || "Import gagal.";
            if (data.detail) em += "\n" + data.detail;
            setMsg(errEl, em, true);
            showLogs(data.logs);
            return;
          }
          var resultText = data.message || "Import selesai.";
          if (data.result) {
            resultText += "\n\n" + formatResult(data.result);
          }
          if (typeof Swal !== "undefined") {
            Swal.fire({ icon: "success", title: "Import Berhasil", text: resultText });
          } else {
            setMsg(okEl, data.message || "Import selesai.", false);
          }
          showLogs(data.logs || []);
          importForm.reset();
          var preview = qs("#wp-preview");
          if (preview) preview.classList.add("d-none");
          if (typeof AC !== "undefined" && AC.showSuccessToast) {
            AC.showSuccessToast("WordPress import completed.");
          }
        } catch (err) {
          clearInterval(ptimer);
          if (modal && AC && AC.closeBulkLoadingModal) AC.closeBulkLoadingModal();
          if (err && err.message === "unauth") return;
          setMsg(errEl, "Terjadi kesalahan saat import.", true);
        } finally {
          if (importBtn) importBtn.disabled = false;
          if (previewBtn) previewBtn.disabled = false;
        }
      });
    }
  });
})();