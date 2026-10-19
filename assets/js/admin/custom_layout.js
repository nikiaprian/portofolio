(function () {
  var tbody = document.getElementById("custom-layout-tbody");
  if (!tbody) return;

  var AC = window.AdminCrud;
  if (!AC) return;

  var API = "/api/v1/admin/site-layouts";
  var TOKEN_KEY = "bebii_token";
  var LOGIN_URL =
    (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login";

  var tplRow = document.getElementById("tpl-custom-layout-row");
  var tplEmpty = document.getElementById("tpl-custom-layout-empty");
  var errorEl = document.getElementById("custom-layout-error");
  var pageInfoEl = document.getElementById("custom-layout-page-info");
  var pageSizeSelect = document.getElementById("custom-layout-page-size");
  var prevBtn = document.getElementById("custom-layout-prev-btn");
  var nextBtn = document.getElementById("custom-layout-next-btn");
  var searchInput = document.getElementById("custom-layout-search");
  var modalDeleteEl = document.getElementById("modalCustomLayoutDelete");
  var deleteConfirmBtn = document.getElementById("cld-confirm");
  var deleteErrEl = document.getElementById("custom-layout-delete-error");
  var deleteNameEl = document.getElementById("cld-name");
  var deleteIdEl = document.getElementById("cld-id");
  var modalDuplicateEl = document.getElementById("modalCustomLayoutDuplicate");
  var duplicateConfirmBtn = document.getElementById("cldup-confirm");
  var duplicateErrEl = document.getElementById("custom-layout-duplicate-error");
  var duplicateNameInput = document.getElementById("cldup-layout-name");
  var duplicateSourceNameEl = document.getElementById("cldup-source-name");
  var duplicateSourceIdEl = document.getElementById("cldup-source-id");

  var pendingDeleteId = null;
  var deleteModal = null;
  var pendingDuplicateId = null;
  var duplicateModal = null;

  var msg = {
    loadFailed: "Gagal memuat layout",
    unexpectedResponse: "Respons server tidak sesuai.",
    network: "Terjadi gangguan jaringan. Coba lagi.",
    noItems: "Layout tidak ditemukan",
    deleteSuccess: "Layout berhasil dihapus.",
    deleteFailed: "Gagal menghapus layout",
    deleteInUse: "Layout sedang digunakan dan tidak dapat dihapus.",
    duplicateSuccess: "Layout berhasil diduplikasi.",
    duplicateFailed: "Gagal menduplikasi layout",
    duplicateNameRequired: "Nama layout baru wajib diisi.",
    applySuccess: "Layout berhasil diterapkan dan ditandai sedang dipakai.",
    applyFailed: "Gagal menerapkan layout",
  };

  var allowedPageSizes = [10, 50, 100, 200, 500];
  var state = { page: 1, size: 10, totalPages: 0, query: "" };

  function normalizePageSize(n) {
    var v = parseInt(n, 10);
    if (isNaN(v) || v <= 0) return 10;
    if (allowedPageSizes.indexOf(v) !== -1) return v;
    if (v > 500) return 500;
    var best = 10;
    for (var i = 0; i < allowedPageSizes.length; i++) {
      if (allowedPageSizes[i] <= v) best = allowedPageSizes[i];
    }
    return best;
  }

  function itemUrl(id) {
    return AC.itemUrl(API, id);
  }

  function setListError(text) {
    AC.setGlobalError(errorEl, text);
  }

  function clearListError() {
    AC.setGlobalError(errorEl, null);
  }

  function setModalError(el, text) {
    if (!el) return;
    if (!text) {
      el.classList.add("d-none");
      el.textContent = "";
      return;
    }
    el.textContent = text;
    el.classList.remove("d-none");
  }

  function setDeleteError(text) {
    setModalError(deleteErrEl, text);
  }

  function setDuplicateError(text) {
    setModalError(duplicateErrEl, text);
  }

  function setBtnDisabled(btn, disabled) {
    if (btn) btn.disabled = !!disabled;
  }

  function statusBadgeClasses(used) {
    return used
      ? "badge rounded-pill bg-warning-subtle text-warning-emphasis border border-warning-subtle"
      : "badge rounded-pill bg-secondary-subtle text-secondary-emphasis border border-secondary-subtle";
  }

  function cloneTemplate(tpl) {
    if (!tpl || !tpl.content || !tpl.content.firstElementChild) return null;
    return tpl.content.firstElementChild.cloneNode(true);
  }

  function setField(root, name, value) {
    if (!root) return;
    var node = root.querySelector('[data-field="' + name + '"]');
    if (node) node.textContent = value == null ? "" : String(value);
  }

  function fillRow(item) {
    var row = cloneTemplate(tplRow);
    if (!row || !item) return null;

    var id = item.id != null ? String(item.id) : "";
    var name = item.layout_name ? String(item.layout_name) : "—";
    var used = !!item.is_used;

    setField(row, "avatar", AC.initials(name, name));
    setField(row, "layout_name", name);
    setField(row, "id_badge", "#" + id);
    setField(row, "updated_at", AC.formatDate(item.updated_at) || "—");

    var badge = row.querySelector('[data-field="status_badge"]');
    if (badge) {
      badge.className = statusBadgeClasses(used);
      badge.textContent = used ? "Sedang dipakai" : "Tidak dipakai";
    }

    var editLink = row.querySelector('[data-field="edit_href"]');
    if (editLink && id) {
      editLink.href = "/admin/custom-layout/" + encodeURIComponent(id);
    }

    var dupBtn = row.querySelector('[data-action="duplicate-layout"]');
    if (dupBtn) {
      dupBtn.setAttribute("data-item-id", id);
      dupBtn.setAttribute("data-layout-name", name === "—" ? "" : name);
    }

    var applyBtn = row.querySelector('[data-action="apply-layout"]');
    if (applyBtn) {
      applyBtn.setAttribute("data-item-id", id);
      applyBtn.setAttribute("data-layout-name", name === "—" ? "" : name);
      applyBtn.disabled = false;
      applyBtn.setAttribute("title", used ? "Terapkan ulang layout ini" : "Apply layout");
    }

    var delBtn = row.querySelector('[data-action="delete-layout"]');
    if (delBtn) {
      delBtn.setAttribute("data-item-id", id);
      delBtn.setAttribute("data-layout-name", name === "—" ? "" : name);
      delBtn.disabled = used;
      if (used) {
        delBtn.setAttribute("title", "Tidak dapat dihapus (sedang dipakai)");
      }
    }

    return row;
  }

  function suggestDuplicateName(sourceName) {
    var base = String(sourceName || "").trim();
    if (!base || base === "—") return "";
    var suffix = "-copy";
    var max = 255;
    if (base.length+suffix.length <= max) return base + suffix;
    return base.slice(0, max - suffix.length) + suffix;
  }

  function duplicateUrl(id) {
    return itemUrl(id) + "/duplicate";
  }

  function applyUrl(id) {
    return itemUrl(id) + "/apply";
  }

  var applyBusy = false;

  async function runApplyLayout(id, name, btn) {
    if (applyBusy || !id) return;

    var confirmed = true;
    if (typeof AC.confirmAction === "function") {
      confirmed = await AC.confirmAction({
        title: "Terapkan layout?",
        text:
          'Layout "' +
          (name || id) +
          '" akan menimpa file template klien di disk dan ditandai sedang dipakai. Lanjutkan?',
        icon: "warning",
        confirmButtonText: "Apply layout",
        cancelButtonText: "Batal",
      });
    } else {
      confirmed = window.confirm(
        'Terapkan layout "' + (name || id) + '" ke file template dan aktifkan?'
      );
    }
    if (!confirmed) return;

    applyBusy = true;
    var prevHtml = btn ? btn.innerHTML : "";
    if (btn) {
      btn.disabled = true;
      btn.innerHTML = "…";
    }
    clearListError();

    try {
      var res = await fetchJSON("POST", applyUrl(id), {});
      var data = {};
      try {
        data = await res.json();
      } catch (e) {}
      if (!res.ok) {
        throw new Error((data && data.error) || msg.applyFailed);
      }
      if (typeof AC.showSuccessToast === "function") {
        AC.showSuccessToast(msg.applySuccess);
      }
      await loadList();
    } catch (err) {
      if (err && err.message === "unauthorized") return;
      setListError((err && err.message) || msg.applyFailed);
      if (typeof AC.showErrorToast === "function") {
        AC.showErrorToast((err && err.message) || msg.applyFailed);
      }
    } finally {
      applyBusy = false;
      if (btn) {
        btn.disabled = false;
        btn.innerHTML = prevHtml;
      }
    }
  }

  function renderEmpty() {
    var row = cloneTemplate(tplEmpty);
    if (row) tbody.appendChild(row);
  }

  function renderItems(items) {
    tbody.textContent = "";
    if (!items.length) {
      renderEmpty();
      return;
    }
    for (var i = 0; i < items.length; i++) {
      var row = fillRow(items[i]);
      if (row) tbody.appendChild(row);
    }
  }

  function getToken() {
    return AC.getToken(TOKEN_KEY);
  }

  async function fetchAuth(url, options) {
    var token = getToken();
    if (!token) {
      window.location.href = LOGIN_URL;
      throw new Error("unauthorized");
    }
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
    var opt = { method: method || "GET", headers: { Accept: "application/json" } };
    var m = (method || "GET").toUpperCase();
    if (body !== undefined && body !== null && m !== "GET" && m !== "HEAD") {
      opt.body = JSON.stringify(body);
    }
    return fetchAuth(url, opt);
  }

  function listUrl() {
    return (
      API +
      "?page=" +
      encodeURIComponent(state.page) +
      "&size=" +
      encodeURIComponent(state.size) +
      "&q=" +
      encodeURIComponent(state.query || "")
    );
  }

  async function loadList() {
    clearListError();
    try {
      var res = await fetchJSON("GET", listUrl());
      if (!res.ok) {
        setListError(msg.loadFailed + " (" + res.status + ")");
        return;
      }
      var payload = await res.json();
      var items = AC.defaultParseItems(payload);
      if (!Array.isArray(items)) {
        setListError(msg.unexpectedResponse);
        return;
      }

      state.totalPages = AC.defaultParseTotalPages(payload);
      renderItems(items);

      if (pageInfoEl) {
        if (state.totalPages > 0) {
          pageInfoEl.textContent =
            "Halaman " + state.page + " dari " + state.totalPages;
        } else {
          pageInfoEl.textContent = msg.noItems;
        }
      }

      if (state.totalPages > 0) {
        setBtnDisabled(prevBtn, state.page <= 1);
        setBtnDisabled(nextBtn, state.page >= state.totalPages);
      } else {
        setBtnDisabled(prevBtn, true);
        setBtnDisabled(nextBtn, true);
      }
    } catch (e) {
      if (e && e.message === "unauthorized") return;
      setListError(msg.network);
    }
  }

  function openDeleteModal(id, name) {
    pendingDeleteId = id;
    setDeleteError(null);
    if (deleteIdEl) deleteIdEl.textContent = "#" + id;
    if (deleteNameEl) deleteNameEl.textContent = name || "—";
    if (!modalDeleteEl || !window.bootstrap || !window.bootstrap.Modal) return;
    if (!deleteModal) {
      deleteModal = window.bootstrap.Modal.getOrCreateInstance(modalDeleteEl);
    }
    deleteModal.show();
  }

  function closeDeleteModal() {
    pendingDeleteId = null;
    if (deleteModal) deleteModal.hide();
  }

  function openDuplicateModal(id, name) {
    pendingDuplicateId = id;
    setDuplicateError(null);
    if (duplicateSourceIdEl) duplicateSourceIdEl.textContent = "#" + id;
    if (duplicateSourceNameEl) {
      duplicateSourceNameEl.textContent = name || "—";
    }
    if (duplicateNameInput) {
      duplicateNameInput.value = suggestDuplicateName(name);
    }
    if (!modalDuplicateEl || !window.bootstrap || !window.bootstrap.Modal) return;
    if (!duplicateModal) {
      duplicateModal = window.bootstrap.Modal.getOrCreateInstance(modalDuplicateEl);
    }
    duplicateModal.show();
    if (duplicateNameInput) {
      duplicateNameInput.focus();
      duplicateNameInput.select();
    }
  }

  function closeDuplicateModal() {
    pendingDuplicateId = null;
    if (duplicateModal) duplicateModal.hide();
  }

  async function confirmDuplicate() {
    if (!pendingDuplicateId) return;
    var newName = duplicateNameInput
      ? String(duplicateNameInput.value || "").trim()
      : "";
    if (!newName) {
      setDuplicateError(msg.duplicateNameRequired);
      return;
    }
    setDuplicateError(null);
    setBtnDisabled(duplicateConfirmBtn, true);
    try {
      var res = await fetchJSON("POST", duplicateUrl(pendingDuplicateId), {
        layout_name: newName,
      });
      if (res.status === 409) {
        var conflict = await res.json().catch(function () {
          return null;
        });
        setDuplicateError((conflict && conflict.error) || msg.duplicateFailed);
        return;
      }
      if (!res.ok) {
        var data = await res.json().catch(function () {
          return null;
        });
        setDuplicateError((data && data.error) || msg.duplicateFailed);
        return;
      }
      closeDuplicateModal();
      AC.showSuccessToast(msg.duplicateSuccess);
      await loadList();
    } catch (e) {
      if (e && e.message === "unauthorized") return;
      setDuplicateError(msg.network);
    } finally {
      setBtnDisabled(duplicateConfirmBtn, false);
    }
  }

  async function confirmDelete() {
    if (!pendingDeleteId) return;
    setDeleteError(null);
    setBtnDisabled(deleteConfirmBtn, true);
    try {
      var res = await fetchJSON("DELETE", itemUrl(pendingDeleteId));
      if (res.status === 409) {
        setDeleteError(msg.deleteInUse);
        return;
      }
      if (!res.ok) {
        var data = await res.json().catch(function () {
          return null;
        });
        setDeleteError((data && data.error) || msg.deleteFailed);
        return;
      }
      closeDeleteModal();
      AC.showSuccessToast(msg.deleteSuccess);
      await loadList();
    } catch (e) {
      if (e && e.message === "unauthorized") return;
      setDeleteError(msg.network);
    } finally {
      setBtnDisabled(deleteConfirmBtn, false);
    }
  }

  function consumeLayoutSavedFlash() {
    try {
      var raw = sessionStorage.getItem("bebii_custom_layout_saved_flash");
      if (!raw) return;
      sessionStorage.removeItem("bebii_custom_layout_saved_flash");
      var info = JSON.parse(raw);
      var text =
        info && info.action === "created"
          ? "Layout berhasil dibuat."
          : "Layout berhasil diperbarui.";
      AC.showSuccessToast(text);
    } catch (e) {
      /* ignore */
    }
  }

  tbody.addEventListener("click", function (ev) {
    var applyBtn = ev.target.closest('[data-action="apply-layout"]');
    if (applyBtn && !applyBtn.disabled) {
      var applyId = applyBtn.getAttribute("data-item-id");
      if (!applyId) return;
      runApplyLayout(applyId, applyBtn.getAttribute("data-layout-name") || "", applyBtn);
      return;
    }
    var dupBtn = ev.target.closest('[data-action="duplicate-layout"]');
    if (dupBtn) {
      var dupId = dupBtn.getAttribute("data-item-id");
      if (!dupId) return;
      openDuplicateModal(dupId, dupBtn.getAttribute("data-layout-name") || "");
      return;
    }
    var delBtn = ev.target.closest('[data-action="delete-layout"]');
    if (!delBtn || delBtn.disabled) return;
    var id = delBtn.getAttribute("data-item-id");
    if (!id) return;
    openDeleteModal(id, delBtn.getAttribute("data-layout-name") || "");
  });

  if (deleteConfirmBtn) {
    deleteConfirmBtn.addEventListener("click", confirmDelete);
  }

  if (duplicateConfirmBtn) {
    duplicateConfirmBtn.addEventListener("click", confirmDuplicate);
  }

  if (duplicateNameInput) {
    duplicateNameInput.addEventListener("keydown", function (ev) {
      if (ev.key === "Enter") {
        ev.preventDefault();
        confirmDuplicate();
      }
    });
  }

  if (pageSizeSelect) {
    state.size = normalizePageSize(pageSizeSelect.value || 10);
    pageSizeSelect.addEventListener("change", function () {
      state.size = normalizePageSize(pageSizeSelect.value);
      state.page = 1;
      loadList();
    });
  }

  if (prevBtn) {
    prevBtn.addEventListener("click", function () {
      if (state.page <= 1) return;
      state.page -= 1;
      loadList();
    });
  }

  if (nextBtn) {
    nextBtn.addEventListener("click", function () {
      if (state.totalPages > 0 && state.page >= state.totalPages) return;
      state.page += 1;
      loadList();
    });
  }

  if (searchInput) {
    var searchTimeout = null;
    function triggerSearch() {
      var value = searchInput.value || "";
      if (searchTimeout) clearTimeout(searchTimeout);
      searchTimeout = setTimeout(function () {
        state.query = value.trim();
        state.page = 1;
        loadList();
      }, 200);
    }
    searchInput.addEventListener("input", triggerSearch);
    searchInput.addEventListener("change", triggerSearch);
    searchInput.addEventListener("search", triggerSearch);
  }

  consumeLayoutSavedFlash();
  loadList();
})();
