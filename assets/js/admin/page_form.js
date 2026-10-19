(function () {
  var AC = window.AdminCrud;
  if (!AC || typeof tinymce === "undefined" || !window.TinyMCE) {
    return;
  }

  var root = document.getElementById("admin-page-form-root");
  if (!root) return;

  var API_PAGES = "/api/v1/admin/pages";
  var API_UPLOAD_IMG = "/api/v1/admin/pages/upload-image";
  var TOKEN_KEY = "bebii_token";
  var LOGIN_URL =
    (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login";

  var mode = (root.getAttribute("data-mode") || "create").toLowerCase();
  var pageIdRaw = (root.getAttribute("data-page-id") || "").trim();

  var contentEditor = null;
  var tinyReady = null;
  var slugAutofill = { reset: function () {} };

  function el(id) {
    return document.getElementById(id);
  }

  function clearFormError() {
    AC.setInlineError(el("page-form-error"), null);
  }

  function setFormError(message) {
    AC.setInlineError(el("page-form-error"), message);
    if (message && typeof AC.showErrorToast === "function") {
      AC.showErrorToast(message);
    }
  }

  function pageUrl(id) {
    return AC.itemUrl(API_PAGES, id);
  }

  async function fetchAuth(url, options) {
    var token = AC.getToken(TOKEN_KEY);
    options = options || {};
    options.headers = Object.assign({}, options.headers || {});
    options.headers.Authorization = "Bearer " + token;
    var m = (options.method || "GET").toUpperCase();
    if (
      options.body &&
      m !== "GET" &&
      m !== "HEAD" &&
      !options.headers["Content-Type"]
    ) {
      options.headers["Content-Type"] = "application/json";
    }
    var res = await fetch(url, options);
    if (res.status === 401) {
      window.location.href = LOGIN_URL;
      throw new Error("unauthorized");
    }
    return res;
  }

  async function fetchJSON(method, url, body) {
    var opt = { method: method || "GET" };
    opt.headers = Object.assign({}, opt.headers || {}, {
      Accept: "application/json",
    });
    var m = (method || "GET").toUpperCase();
    if (body !== undefined && body !== null && m !== "GET" && m !== "HEAD") {
      opt.body = typeof body === "string" ? body : JSON.stringify(body);
    }
    return fetchAuth(url, opt);
  }

  function setSubmitBusy(busy) {
    var btn = el("pg-submit");
    var spin = el("pg-submit-spinner");
    var icon = el("pg-submit-icon");
    var lab = el("pg-submit-label");
    if (!btn) return;
    if (busy) {
      btn.disabled = true;
      btn.setAttribute("aria-busy", "true");
      if (spin) spin.classList.remove("d-none");
      if (icon) icon.classList.add("d-none");
      if (lab && btn.dataset.labelBusy) lab.textContent = btn.dataset.labelBusy;
    } else {
      btn.disabled = false;
      btn.removeAttribute("aria-busy");
      if (spin) spin.classList.add("d-none");
      if (icon) icon.classList.remove("d-none");
      if (lab && btn.dataset.labelDefault) {
        lab.textContent = btn.dataset.labelDefault;
      }
    }
  }

  function getTinyEditor() {
    if (contentEditor) return contentEditor;
    if (typeof tinymce !== "undefined") {
      var ed = tinymce.get("pg-content");
      if (ed) return ed;
    }
    return null;
  }

  function htmlContent() {
    var ed = getTinyEditor();
    if (!ed) return "";
    return String(ed.getContent() || "").trim();
  }

  function setEditorHTML(html) {
    var ed = getTinyEditor();
    if (!ed) return;
    ed.setContent(html || "<p><br></p>");
  }

  function ensureTiny() {
    if (contentEditor) return Promise.resolve(contentEditor);
    if (tinyReady) return tinyReady;
    tinyReady = window.TinyMCE.init({
      selector: "#pg-content",
      uploadUrl: API_UPLOAD_IMG,
      tokenKey: TOKEN_KEY,
      loginUrl: LOGIN_URL,
    }).then(function (ed) {
      contentEditor = ed;
      return ed;
    });
    return tinyReady;
  }

  function wireSlugAutofill(titleInput, slugInput) {
    var NS = window.AdminNameSlugCrud;
    if (NS && typeof NS.slugify === "function") {
      var slugTouched = false;
      function onTitleInput() {
        if (slugTouched) return;
        slugInput.value = NS.slugify(titleInput.value);
      }
      function onSlugInput() {
        slugTouched = true;
      }
      titleInput.addEventListener("input", onTitleInput);
      slugInput.addEventListener("input", onSlugInput);
      return {
        reset: function () {
          slugTouched = false;
        },
      };
    }
    return { reset: function () {} };
  }

  document.addEventListener("DOMContentLoaded", function () {
    if (!AC.getToken(TOKEN_KEY)) {
      window.location.href = LOGIN_URL;
      return;
    }

    if (mode === "edit" && !pageIdRaw) {
      setFormError("ID halaman tidak ditemukan. Kembali ke daftar halaman.");
      return;
    }

    if (mode === "create") {
      slugAutofill = wireSlugAutofill(el("pg-title"), el("pg-slug"));
    }

    ensureTiny()
      .then(function () {
        return (async function init() {
          try {
            if (mode === "edit") {
              var res = await fetchAuth(pageUrl(pageIdRaw), { method: "GET" });
              if (!res.ok) {
                setFormError(
                  await AC.responseErrorMessage(res, "Gagal memuat halaman")
                );
                return;
              }
              var row = await res.json();
              el("pg-title").value = row && row.title ? row.title : "";
              el("pg-slug").value = row && row.slug ? row.slug : "";
              el("pg-status").value =
                row && row.status === "published" ? "published" : "draft";
              setEditorHTML(
                row && row.content ? String(row.content) : "<p><br></p>"
              );
            } else {
              setEditorHTML("<p><br></p>");
            }
          } catch (e) {
            if (e && e.message === "unauthorized") return;
            setFormError("Terjadi gangguan jaringan.");
          }
        })();
      })
      .catch(function (err) {
        console.error(err);
        setFormError(
          "Editor teks gagal dimuat. Periksa koneksi Anda lalu muat ulang."
        );
      });

    var form = el("form-page-admin");
    if (!form) return;

    form.addEventListener("submit", async function (e) {
      e.preventDefault();
      clearFormError();

      var slugVal = el("pg-slug").value.trim();
      if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slugVal)) {
        setFormError("Slug: gunakan huruf kecil, angka, dan tanda hubung saja.");
        return;
      }

      try {
        await ensureTiny();
      } catch (err) {
        setFormError("Editor belum siap. Tunggu sebentar atau muat ulang.");
        return;
      }

      var contentHtml = htmlContent();
      if (contentHtml.replace(/<[^>]+>/g, "").trim().length < 10) {
        setFormError("Konten harus berisi minimal 10 karakter teks.");
        return;
      }
      if (contentHtml.length > 65535) {
        setFormError("Konten terlalu panjang (maksimal 65535 karakter).");
        return;
      }

      var payload = {
        title: el("pg-title").value.trim(),
        slug: slugVal,
        content: contentHtml,
        status: el("pg-status").value,
      };

      setSubmitBusy(true);

      try {
        if (mode === "create") {
          var resC = await fetchJSON("POST", API_PAGES, payload);
          if (!resC.ok) {
            setFormError(
              await AC.responseErrorMessage(resC, "Gagal menyimpan halaman")
            );
            return;
          }
          try {
            sessionStorage.setItem(
              "bebii_page_saved_flash",
              JSON.stringify({ action: "created" })
            );
          } catch (x) {}
          window.location.href = "/admin/pages";
          return;
        }

        var resU = await fetchJSON("PUT", pageUrl(pageIdRaw), payload);
        if (!resU.ok) {
          setFormError(
            await AC.responseErrorMessage(resU, "Gagal memperbarui halaman")
          );
          return;
        }
        try {
          sessionStorage.setItem(
            "bebii_page_saved_flash",
            JSON.stringify({ action: "updated" })
          );
        } catch (x) {}
        window.location.href = "/admin/pages";
      } catch (err) {
        if (err && err.message === "unauthorized") return;
        setFormError("Terjadi gangguan jaringan. Coba lagi.");
      } finally {
        setSubmitBusy(false);
      }
    });
  });
})();
