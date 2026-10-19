(function () {
  var AC = window.AdminCrud;
  if (!AC) return;

  var root = document.getElementById("admin-custom-layout-form-root");
  if (!root) return;

  var API = "/api/v1/admin/site-layouts";
  var API_UPLOAD_IMAGE_REF = "/api/v1/admin/site-layouts/upload-image-ref";
  var TOKEN_KEY = "bebii_token";
  var LOGIN_URL =
    (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login";

  var mode = (root.getAttribute("data-mode") || "create").toLowerCase();
  var layoutIdRaw = (root.getAttribute("data-layout-id") || "").trim();

  var imageRefUrls = [];
  var imageUploadBusy = false;
  var applyLayoutBusy = false;

  function el(id) {
    return document.getElementById(id);
  }

  function clearFormError() {
    AC.setInlineError(el("custom-layout-form-error"), null);
  }

  function setFormError(message) {
    AC.setInlineError(el("custom-layout-form-error"), message);
    if (message && typeof AC.showErrorToast === "function") {
      AC.showErrorToast(message);
    }
  }

  function setUploadError(message) {
    AC.setInlineError(el("cl-image-ref-upload-error"), message);
  }

  function itemUrl(id) {
    return AC.itemUrl(API, id);
  }

  async function fetchAuth(url, options) {
    var token = AC.getToken(TOKEN_KEY);
    options = options || {};
    options.headers = Object.assign({}, options.headers || {});
    options.headers.Authorization = "Bearer " + token;
    var m = (options.method || "GET").toUpperCase();
    var isFormData =
      typeof FormData !== "undefined" && options.body instanceof FormData;
    if (
      options.body &&
      m !== "GET" &&
      m !== "HEAD" &&
      !isFormData &&
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
    var btn = el("cl-submit");
    var spin = el("cl-submit-spinner");
    var icon = el("cl-submit-icon");
    var lab = el("cl-submit-label");
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

  function parseImageRef(raw) {
    if (!raw) return [];
    if (Array.isArray(raw)) {
      return raw.map(function (u) {
        return String(u || "").trim();
      }).filter(Boolean);
    }
    var s = String(raw).trim();
    if (!s) return [];
    try {
      var parsed = JSON.parse(s);
      if (Array.isArray(parsed)) {
        return parsed.map(function (u) {
          return String(u || "").trim();
        }).filter(Boolean);
      }
      if (typeof parsed === "string" && parsed.trim()) {
        return [parsed.trim()];
      }
    } catch (e) {
      if (s.indexOf(",") >= 0) {
        return s.split(",").map(function (part) {
          return part.trim();
        }).filter(Boolean);
      }
      return [s];
    }
    return [];
  }

  function syncImageRefHidden() {
    var hidden = el("cl-image-ref");
    if (!hidden) return;
    hidden.value = JSON.stringify(imageRefUrls);
  }

  function cloneThumbTemplate() {
    var tpl = el("tpl-layoutref-thumb");
    if (!tpl || !tpl.content || !tpl.content.firstElementChild) return null;
    return tpl.content.firstElementChild.cloneNode(true);
  }

  function renderImageRefGrid() {
    var grid = el("cl-image-ref-grid");
    if (!grid) return;
    grid.textContent = "";
    for (var i = 0; i < imageRefUrls.length; i++) {
      var url = imageRefUrls[i];
      var node = cloneThumbTemplate();
      if (!node) continue;
      node.setAttribute("data-url", url);
      var img = node.querySelector('[data-field="img"]');
      if (img) {
        img.src = url;
        img.alt = "Referensi " + (i + 1);
      }
      grid.appendChild(node);
    }
    syncImageRefHidden();
  }

  function addImageRefUrl(url) {
    var u = String(url || "").trim();
    if (!u) return;
    if (imageRefUrls.indexOf(u) >= 0) return;
    imageRefUrls.push(u);
    renderImageRefGrid();
  }

  function removeImageRefUrl(url) {
    var u = String(url || "").trim();
    imageRefUrls = imageRefUrls.filter(function (item) {
      return item !== u;
    });
    renderImageRefGrid();
  }

  function isImageFile(file) {
    if (!file) return false;
    var t = String(file.type || "").toLowerCase();
    if (t.indexOf("image/") === 0) return true;
    var n = String(file.name || "").toLowerCase();
    return /\.(jpe?g|png|gif|webp)$/i.test(n);
  }

  async function uploadImageFile(file) {
    if (!isImageFile(file)) {
      throw new Error("File bukan gambar.");
    }
    var fd = new FormData();
    fd.append("image", file, file.name || "paste.png");
    var res = await fetchAuth(API_UPLOAD_IMAGE_REF, {
      method: "POST",
      body: fd,
    });
    var data = await res.json().catch(function () {
      return null;
    });
    if (!res.ok) {
      throw new Error((data && data.error) || "Gagal mengunggah gambar.");
    }
    var url = data && data.image ? String(data.image).trim() : "";
    if (!url) throw new Error("Respons unggahan tidak valid.");
    return url;
  }

  async function handleImageFiles(fileList) {
    if (!fileList || !fileList.length || imageUploadBusy) return;
    imageUploadBusy = true;
    setUploadError(null);
    try {
      for (var i = 0; i < fileList.length; i++) {
        var url = await uploadImageFile(fileList[i]);
        addImageRefUrl(url);
      }
    } catch (e) {
      if (String(e && e.message) !== "unauthorized") {
        setUploadError((e && e.message) || "Gagal mengunggah gambar.");
      }
    } finally {
      imageUploadBusy = false;
      var input = el("cl-image-ref-input");
      if (input) input.value = "";
    }
  }

  function filesFromPasteEvent(ev) {
    var out = [];
    var cd = ev && ev.clipboardData;
    if (!cd) return out;
    if (cd.files && cd.files.length) {
      for (var i = 0; i < cd.files.length; i++) {
        if (isImageFile(cd.files[i])) out.push(cd.files[i]);
      }
    }
    if (out.length) return out;
    if (!cd.items) return out;
    for (var j = 0; j < cd.items.length; j++) {
      var item = cd.items[j];
      if (item && item.kind === "file" && item.type && item.type.indexOf("image/") === 0) {
        var f = item.getAsFile();
        if (f) out.push(f);
      }
    }
    return out;
  }

  function isImageRefPasteContext() {
    var section = el("cl-image-ref-section");
    if (!section) return false;
    var active = document.activeElement;
    if (active && section.contains(active)) return true;
    try {
      return section.matches(":hover");
    } catch (e) {
      return false;
    }
  }

  function handleImagePaste(ev) {
    var files = filesFromPasteEvent(ev);
    if (!files.length) return;
    ev.preventDefault();
    handleImageFiles(files);
  }

  var imageRefContextMenu = null;
  var imageRefDropClickTimer = null;

  function pasteShortcutLabel() {
    var ua = navigator.userAgent || "";
    var platform = navigator.platform || "";
    if (/Mac|iPhone|iPad|iPod/i.test(platform) || /Mac OS X/i.test(ua)) {
      return "⌘V";
    }
    return "Ctrl+V";
  }

  function isImageRefMenuTarget(target) {
    if (!target) return false;
    var section = el("cl-image-ref-section");
    if (!section || !section.contains(target)) return false;
    if (target.closest('[data-action="remove-layoutref"]')) return false;
    return !!(
      target.closest("#cl-image-ref-drop") || target.closest("#cl-image-ref-grid")
    );
  }

  function ensureImageRefContextMenu() {
    if (imageRefContextMenu) return imageRefContextMenu;
    var menu = document.createElement("div");
    menu.id = "cl-image-ref-context-menu";
    menu.className = "dropdown-menu shadow show d-none position-fixed py-1";
    menu.style.zIndex = "1080";
    menu.setAttribute("role", "menu");
    menu.innerHTML =
      '<button type="button" class="dropdown-item d-flex align-items-center gap-2" data-action="paste-layoutref" role="menuitem">' +
      '<i class="bi bi-clipboard" aria-hidden="true"></i>' +
      '<span>Tempel gambar</span>' +
      '<small class="text-secondary ms-auto paste-shortcut-label"></small>' +
      "</button>";
    document.body.appendChild(menu);
    var shortcut = menu.querySelector(".paste-shortcut-label");
    if (shortcut) shortcut.textContent = pasteShortcutLabel();
    menu.addEventListener("click", function (ev) {
      var btn = ev.target.closest('[data-action="paste-layoutref"]');
      if (!btn) return;
      ev.preventDefault();
      pasteImageFromClipboard();
    });
    imageRefContextMenu = menu;
    return menu;
  }

  function hideImageRefContextMenu() {
    if (!imageRefContextMenu) return;
    imageRefContextMenu.classList.add("d-none");
  }

  function positionImageRefContextMenu(clientX, clientY) {
    var menu = ensureImageRefContextMenu();
    menu.classList.remove("d-none");
    var pad = 8;
    var rect = menu.getBoundingClientRect();
    var maxX = window.innerWidth - rect.width - pad;
    var maxY = window.innerHeight - rect.height - pad;
    var x = Math.max(pad, Math.min(clientX, maxX));
    var y = Math.max(pad, Math.min(clientY, maxY));
    menu.style.left = x + "px";
    menu.style.top = y + "px";
  }

  function showImageRefContextMenu(ev) {
    if (!isImageRefMenuTarget(ev.target)) return;
    ev.preventDefault();
    ev.stopPropagation();
    var drop = el("cl-image-ref-drop");
    if (drop) drop.focus();
    positionImageRefContextMenu(ev.clientX, ev.clientY);
  }

  async function readClipboardImageFiles() {
    if (!navigator.clipboard || typeof navigator.clipboard.read !== "function") {
      throw new Error(
        "Tempel dari menu tidak didukung browser ini. Gunakan " +
          pasteShortcutLabel() +
          "."
      );
    }
    var items = await navigator.clipboard.read();
    var files = [];
    for (var i = 0; i < items.length; i++) {
      var item = items[i];
      var types = item.types || [];
      for (var j = 0; j < types.length; j++) {
        var type = types[j];
        if (type.indexOf("image/") !== 0) continue;
        var blob = await item.getType(type);
        var ext = (type.split("/")[1] || "png").replace("jpeg", "jpg");
        files.push(new File([blob], "paste." + ext, { type: type }));
      }
    }
    if (!files.length) {
      throw new Error(
        "Clipboard tidak berisi gambar. Salin screenshot atau gambar terlebih dahulu."
      );
    }
    return files;
  }

  async function pasteImageFromClipboard() {
    hideImageRefContextMenu();
    if (imageUploadBusy) return;
    setUploadError(null);
    try {
      var files = await readClipboardImageFiles();
      await handleImageFiles(files);
    } catch (e) {
      if (String(e && e.message) !== "unauthorized") {
        setUploadError((e && e.message) || "Gagal menempel gambar.");
      }
    }
  }

  function bindImageRefPasteMenu(section, drop, grid) {
    ensureImageRefContextMenu();

    document.addEventListener("click", function (ev) {
      if (!imageRefContextMenu || imageRefContextMenu.classList.contains("d-none")) {
        return;
      }
      if (ev.target.closest("#cl-image-ref-context-menu")) return;
      hideImageRefContextMenu();
    });
    document.addEventListener("keydown", function (ev) {
      if (ev.key === "Escape") hideImageRefContextMenu();
    });
    window.addEventListener("resize", hideImageRefContextMenu);
    window.addEventListener(
      "scroll",
      function () {
        hideImageRefContextMenu();
      },
      true
    );

    var openMenu = function (ev) {
      showImageRefContextMenu(ev);
    };

    if (section) {
      section.addEventListener("contextmenu", openMenu);
    }

    function bindDoubleClickMenu(node) {
      if (!node) return;
      node.addEventListener("dblclick", function (ev) {
        if (ev.target.closest("#cl-image-ref-pick")) return;
        if (ev.target.closest('[data-action="remove-layoutref"]')) return;
        ev.preventDefault();
        ev.stopPropagation();
        if (imageRefDropClickTimer) {
          clearTimeout(imageRefDropClickTimer);
          imageRefDropClickTimer = null;
        }
        openMenu(ev);
      });
    }

    bindDoubleClickMenu(drop);
    bindDoubleClickMenu(grid);
  }

  function bindImageRefUpload() {
    var drop = el("cl-image-ref-drop");
    var pick = el("cl-image-ref-pick");
    var input = el("cl-image-ref-input");
    var grid = el("cl-image-ref-grid");
    var section = el("cl-image-ref-section");

    if (pick && input) {
      pick.addEventListener("click", function (ev) {
        ev.preventDefault();
        ev.stopPropagation();
        input.click();
      });
    }

    if (drop && input) {
      drop.addEventListener("click", function (ev) {
        if (ev.target.closest("#cl-image-ref-pick")) return;
        if (imageRefDropClickTimer) clearTimeout(imageRefDropClickTimer);
        imageRefDropClickTimer = setTimeout(function () {
          imageRefDropClickTimer = null;
          input.click();
        }, 220);
      });
      drop.addEventListener("keydown", function (ev) {
        if (ev.key === "Enter" || ev.key === " ") {
          ev.preventDefault();
          input.click();
        }
      });
      drop.addEventListener("dragover", function (ev) {
        ev.preventDefault();
        drop.classList.add("border-primary");
      });
      drop.addEventListener("dragleave", function () {
        drop.classList.remove("border-primary");
      });
      drop.addEventListener("drop", function (ev) {
        ev.preventDefault();
        drop.classList.remove("border-primary");
        if (ev.dataTransfer && ev.dataTransfer.files) {
          handleImageFiles(ev.dataTransfer.files);
        }
      });
    }

    if (input) {
      input.addEventListener("change", function () {
        if (input.files) handleImageFiles(input.files);
      });
    }

    if (grid) {
      grid.addEventListener("click", function (ev) {
        var btn = ev.target.closest('[data-action="remove-layoutref"]');
        if (!btn) return;
        var wrap = btn.closest(".layoutref-thumb");
        if (!wrap) return;
        removeImageRefUrl(wrap.getAttribute("data-url") || "");
      });
    }

    document.addEventListener("paste", function (ev) {
      if (!isImageRefPasteContext()) return;
      handleImagePaste(ev);
    });

    bindImageRefPasteMenu(section, drop, grid);
  }

  function collectPayload() {
    syncImageRefHidden();
    return {
      layout_name: (el("cl-layout-name") && el("cl-layout-name").value) || "",
      header: (el("cl-header") && el("cl-header").value) || "",
      footer: (el("cl-footer") && el("cl-footer").value) || "",
      home_body: (el("cl-home-body") && el("cl-home-body").value) || "",
      article_body: (el("cl-article-body") && el("cl-article-body").value) || "",
      sidebar: (el("cl-sidebar") && el("cl-sidebar").value) || "",
      rss: (el("cl-rss") && el("cl-rss").value) || "",
      author_profile:
        (el("cl-author-profile") && el("cl-author-profile").value) || "",
      tag: (el("cl-tag") && el("cl-tag").value) || "",
      kategori: (el("cl-kategori") && el("cl-kategori").value) || "",
      search_page: (el("cl-search-page") && el("cl-search-page").value) || "",
      misc: (el("cl-misc") && el("cl-misc").value) || "",
      latest_prompt:
        (el("cl-latest-prompt") && el("cl-latest-prompt").value) || "",
      current_prompt:
        (el("cl-current-prompt") && el("cl-current-prompt").value) || "",
      image_ref: (el("cl-image-ref") && el("cl-image-ref").value) || "[]",
      css_data: (el("cl-css-data") && el("cl-css-data").value) || "",
      js_data: (el("cl-js-data") && el("cl-js-data").value) || "",
    };
  }

  function fillForm(row) {
    if (!row) return;
    if (el("cl-layout-name")) el("cl-layout-name").value = row.layout_name || "";
    if (el("cl-header")) el("cl-header").value = row.header || "";
    if (el("cl-footer")) el("cl-footer").value = row.footer || "";
    if (el("cl-home-body")) el("cl-home-body").value = row.home_body || "";
    if (el("cl-article-body")) el("cl-article-body").value = row.article_body || "";
    if (el("cl-sidebar")) el("cl-sidebar").value = row.sidebar || "";
    if (el("cl-rss")) el("cl-rss").value = row.rss || "";
    if (el("cl-author-profile")) el("cl-author-profile").value = row.author_profile || "";
    if (el("cl-tag")) el("cl-tag").value = row.tag || "";
    if (el("cl-kategori")) el("cl-kategori").value = row.kategori || "";
    if (el("cl-search-page")) el("cl-search-page").value = row.search_page || "";
    if (el("cl-misc")) el("cl-misc").value = row.misc || "";
    if (el("cl-latest-prompt")) {
      el("cl-latest-prompt").value = row.latest_prompt || "";
    }
    if (el("cl-current-prompt")) {
      el("cl-current-prompt").value = row.current_prompt || "";
    }
    imageRefUrls = parseImageRef(row.image_ref);
    renderImageRefGrid();
    if (el("cl-css-data")) el("cl-css-data").value = row.css_data || "";
    if (el("cl-js-data")) el("cl-js-data").value = row.js_data || "";
  }

  async function loadExisting() {
    if (mode !== "edit" || !layoutIdRaw) return;
    var res = await fetchJSON("GET", itemUrl(layoutIdRaw));
    if (!res.ok) {
      setFormError("Gagal memuat layout (" + res.status + ").");
      return;
    }
    var row = await res.json();
    fillForm(row);
  }

  async function onSubmit(ev) {
    ev.preventDefault();
    clearFormError();
    var payload = collectPayload();
    if (!String(payload.layout_name || "").trim()) {
      setFormError("Nama layout wajib diisi.");
      return;
    }
    setSubmitBusy(true);
    try {
      var res;
      if (mode === "create") {
        res = await fetchJSON("POST", API, payload);
      } else {
        res = await fetchJSON("PUT", itemUrl(layoutIdRaw), payload);
      }
      if (res.status === 409) {
        var conflict = await res.json().catch(function () {
          return null;
        });
        setFormError((conflict && conflict.error) || "Nama layout sudah digunakan.");
        return;
      }
      if (!res.ok) {
        var data = await res.json().catch(function () {
          return null;
        });
        setFormError(
          (data && data.error) ||
            "Gagal menyimpan layout (" + res.status + ")."
        );
        return;
      }
      try {
        sessionStorage.setItem(
          "bebii_custom_layout_saved_flash",
          JSON.stringify({ action: mode === "create" ? "created" : "updated" })
        );
      } catch (e) {
        /* ignore */
      }
      window.location.href = "/admin/custom-layout";
    } catch (e) {
      if (String(e && e.message) !== "unauthorized") {
        setFormError("Terjadi gangguan jaringan. Coba lagi.");
      }
    } finally {
      setSubmitBusy(false);
    }
  }

  bindImageRefUpload();
  renderImageRefGrid();

  async function runApplyLayout(btn) {
    if (applyLayoutBusy) return;
    clearFormError();

    if (mode !== "edit" || !layoutIdRaw) {
      setFormError("Simpan layout terlebih dahulu sebelum Apply layout.");
      return;
    }

    var confirmed = true;
    if (typeof AC.confirmAction === "function") {
      confirmed = await AC.confirmAction({
        title: "Terapkan layout ke file template?",
        text:
          "Semua fragment tersimpan di database akan menimpa file template klien di disk, dan layout ini akan ditandai sedang dipakai. Simpan perubahan form dulu jika ada edit yang belum disimpan. Lanjutkan?",
        icon: "warning",
        confirmButtonText: "Apply layout",
        cancelButtonText: "Batal",
      });
    } else {
      confirmed = window.confirm(
        "Semua fragment tersimpan akan menimpa file template klien dan mengaktifkan layout ini. Lanjutkan?"
      );
    }
    if (!confirmed) return;

    applyLayoutBusy = true;
    var prevHtml = btn ? btn.innerHTML : "";
    if (btn) {
      btn.disabled = true;
      btn.innerHTML = "Applying…";
    }

    try {
      var res = await fetchJSON("POST", API + "/" + encodeURIComponent(layoutIdRaw) + "/apply", {});
      var data = {};
      try {
        data = await res.json();
      } catch (e) {}

      if (!res.ok) {
        throw new Error((data && data.error) || "Gagal menerapkan layout.");
      }

      var count = (data && data.fragments_count) || (data.applied && data.applied.length) || 0;
      if (typeof AC.showSuccessToast === "function") {
        AC.showSuccessToast(
          count
            ? "Layout diterapkan (" + count + " fragment) dan ditandai sedang dipakai."
            : "Layout berhasil diterapkan dan ditandai sedang dipakai."
        );
      }
    } catch (err) {
      if (err && err.message === "unauthorized") return;
      setFormError((err && err.message) || "Gagal menerapkan layout.");
    } finally {
      applyLayoutBusy = false;
      if (btn) {
        btn.disabled = false;
        btn.innerHTML = prevHtml || "Apply layout";
      }
    }
  }

  var applyLayoutBtn = el("cl-apply-layout");
  if (applyLayoutBtn) {
    applyLayoutBtn.addEventListener("click", function () {
      runApplyLayout(applyLayoutBtn);
    });
  }

  var form = el("form-custom-layout-admin");
  if (form) {
    form.addEventListener("submit", onSubmit);
  }
  loadExisting();

  document.addEventListener("cl-layout-ai-reloaded", function (ev) {
    if (ev && ev.detail && ev.detail.layout) {
      fillForm(ev.detail.layout);
    }
  });
})();
