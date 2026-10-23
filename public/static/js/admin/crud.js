/**
 * Reusable admin CRUD helpers (list + pagination + authenticated fetch).
 *
 * HTML/CSS (shared):
 *   - Template: {{ template "admin-crud-list" . }} with .crud from the Go handler (see
 *     public/templates/partials/admin/crud_list.html).
 *   - Styles: /static/css/admin/crud.css (.admin-crud, .admin-crud-modal, …).
 *
 * JS (per resource, e.g. categories):
 *   AdminCrud.bindPage({
 *     selectors: { tbody: "#categories-tbody", error: "#categories-error",
 *       pageInfo: "#categories-page-info", pageSize: "#categories-page-size",
 *       prev: "#categories-prev-btn", next: "#categories-next-btn" },
 *     api: { list: (page, size) =>
 *       `/api/v1/admin/categories?page=${page}&size=${size}` },
 *     messages: { loadFailed: "Failed to load categories", noItems: "No categories", … },
 *     renderRow: (item) => { … return tr; },
 *     renderEmpty: (tbody) => { … },
 *     onEditClick: (id) => …,
 *     onDeleteClick: (id) => …,
 *     afterBind: (ctx) => { … forms; use ctx.fetchJSON, AdminCrud.itemUrl("/api/v1/admin/categories", id) },
 *   });
 *
 * Table rows: use .admin-crud-btn-edit / .admin-crud-btn-delete with data-item-id.
 * Modals: class "modal fade admin-crud-modal" (+ admin-crud-modal--danger for delete).
 */
(function (global) {
  "use strict";

  function qs(sel, root) {
    return (root || document).querySelector(sel);
  }

  function getToken(key) {
    key = key || "tirtayasa_token";
    try {
      return localStorage.getItem(key);
    } catch (e) {
      return null;
    }
  }

  function formatDate(raw) {
    if (!raw) return "";
    var d = new Date(raw);
    if (isNaN(d.getTime())) return String(raw);
    var formatted = d.toLocaleString("id-ID", {
      year: "numeric",
      month: "long",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    });
    return formatted.replace(/\s+pukul\s+/i, " ");
  }

  /** Avatar initials from a display name and optional fallback string (e.g. email). */
  function initials(name, fallback) {
    var n = (name || "").trim();
    if (n) {
      var p = n.split(/\s+/).filter(Boolean);
      if (p.length >= 2) {
        return (p[0][0] + p[p.length - 1][0]).toUpperCase();
      }
      return n.slice(0, 2).toUpperCase();
    }
    var f = (fallback || "").trim();
    if (f) return f.slice(0, 2).toUpperCase();
    return "?";
  }

  function showSuccessToast(message) {
    if (!message || typeof Swal === "undefined") return;
    Swal.mixin({
      toast: true,
      position: "top-end",
      showConfirmButton: false,
      timer: 3200,
      timerProgressBar: true,
    }).fire({
      icon: "success",
      title: message,
    });
  }

  function showErrorToast(message) {
    if (!message || typeof Swal === "undefined") return;
    Swal.mixin({
      toast: true,
      position: "top-end",
      showConfirmButton: false,
      timer: 3600,
      timerProgressBar: true,
    }).fire({
      icon: "error",
      title: message,
    });
  }

  async function confirmAction(options) {
    options = options || {};
    var title = options.title || "Are you sure?";
    var text = options.text || "";
    var icon = options.icon || "warning";
    var confirmButtonText = options.confirmButtonText || "Continue";
    var cancelButtonText = options.cancelButtonText || "Cancel";
    if (typeof Swal !== "undefined") {
      var result = await Swal.fire({
        title: title,
        text: text,
        icon: icon,
        showCancelButton: true,
        confirmButtonText: confirmButtonText,
        cancelButtonText: cancelButtonText,
        focusCancel: true,
      });
      return !!(result && result.isConfirmed);
    }
    var fallbackText = text ? title + "\n\n" + text : title;
    return window.confirm(fallbackText);
  }

  function setGlobalError(el, message) {
    if (!el) return;
    if (!message) {
      el.classList.add("d-none");
      el.textContent = "";
      return;
    }
    el.classList.remove("d-none");
    el.textContent = message;
  }

  function setInlineError(el, message) {
    if (!el) return;
    if (!message) {
      el.classList.add("d-none");
      el.textContent = "";
      return;
    }
    el.classList.remove("d-none");
    el.textContent = message;
  }

  function hideModal(el) {
    if (!el || typeof bootstrap === "undefined") return;
    var inst = bootstrap.Modal.getInstance(el);
    if (inst) inst.hide();
  }

  function defaultParseItems(payload) {
    if (Array.isArray(payload)) return payload;
    if (payload && Array.isArray(payload.items)) return payload.items;
    return null;
  }

  function defaultParseTotalPages(payload) {
    if (payload && typeof payload.total_pages === "number") {
      return payload.total_pages;
    }
    return 0;
  }

  /** Build GET/PUT/DELETE URL for one item: base + "/" + encoded id (no trailing slash on base). */
  function itemUrl(collectionBase, id) {
    var b = String(collectionBase || "").replace(/\/+$/, "");
    return b + "/" + encodeURIComponent(id);
  }

  /** Parse { error: "..." } from a failed JSON response; fallback to status text. */
  async function responseErrorMessage(res, fallbackPrefix) {
    var data = await res.json().catch(function () {
      return null;
    });
    if (data && data.error) return data.error;
    var p = fallbackPrefix || "Request failed";
    return p + " (" + res.status + ")";
  }

  /**
   * @param {object} cfg
   * @param {string} [cfg.tokenKey]
   * @param {string} [cfg.loginUrl]
   * @param {object} cfg.selectors - tbody, error, pageInfo, pageSize, prev, next
   * @param {object} cfg.api - list(page,size,q) => url string (when cfg.buildListURL is omitted)
   * @param {function} [cfg.buildListURL] - (page,size,q,ctx) => list URL; when set, replaces cfg.api.list
   * @param {function} cfg.renderRow(item) => HTMLTableRowElement
   * @param {function} cfg.renderEmpty(tbody, state)
   * @param {number} [cfg.initialPageSize]
   * @param {object} cfg.messages - loadFailed, unexpectedResponse, network, pageOf(page,total), noItems
   * @param {function} [cfg.onEditClick] (id)
   * @param {function} [cfg.onDeleteClick] (id)
   * @param {function} [cfg.afterBind] (ctx) - forms / modals; ctx has fetchJSON, loadList, state, …
   * @param {object} [cfg.bulkActions]
   * @param {function} [cfg.bulkActions.getItemId] (item) => id
   * @param {function} [cfg.bulkActions.deleteItem] (id, helpers) => Promise<void|string>
   * @param {function} [cfg.bulkActions.confirmText] (count) => string
   * @param {function} [cfg.bulkActions.successText] (deletedCount, failedCount) => string
   * @param {string} [cfg.bulkActions.confirmTitle]
   * @param {string} [cfg.bulkActions.confirmButtonText]
   * @param {string} [cfg.bulkActions.failedPrefix]
   */
  function bindPage(cfg) {
    var token = getToken(cfg.tokenKey);
    var loginUrl =
      cfg.loginUrl ||
      (typeof global !== "undefined" && global.TIRTAYASA_LOGIN_URL) ||
      "/login";
    if (!token) {
      window.location.href = loginUrl;
      return null;
    }

    var s = cfg.selectors || {};
    var tbody = qs(s.tbody);
    var errorEl = s.error ? qs(s.error) : null;
    var pageInfoEl = s.pageInfo ? qs(s.pageInfo) : null;
    var pageSizeSelect = s.pageSize ? qs(s.pageSize) : null;
    var prevBtn = s.prev ? qs(s.prev) : null;
    var nextBtn = s.next ? qs(s.next) : null;
    var searchInput = s.searchInput ? qs(s.searchInput) : null;
    var tbodyId = tbody ? tbody.id : "";
    var bulkCfg = cfg.bulkActions || null;
    var selectAllEl = tbodyId
      ? qs('[data-admin-crud-select-all="' + tbodyId + '"]')
      : null;
    var bulkDeleteBtn = tbodyId
      ? qs('[data-admin-crud-bulk-delete="' + tbodyId + '"]')
      : null;
    var selectedActionsToggle = tbodyId
      ? qs('[data-admin-crud-selected-actions-toggle="' + tbodyId + '"]')
      : null;
    var selectedCountEl = tbodyId
      ? qs('[data-admin-crud-selected-count="' + tbodyId + '"]')
      : null;
    var selectedIds = new Set();
    var bulkBusy = false;

    var allowedPageSizes = [10, 50, 100, 200, 500];
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

    var state = {
      page: 1,
      size: normalizePageSize(cfg.initialPageSize || 10),
      totalPages: 0,
      query: "",
    };

    /** Populated with methods below; passed to cfg.buildListURL and cfg.afterBind. */
    var ctx = { state: state };

    var parseItems = cfg.parseItems || defaultParseItems;
    var parseTotalPages = cfg.parseTotalPages || defaultParseTotalPages;
    var messages = cfg.messages || {};
    var getBulkItemId =
      bulkCfg && typeof bulkCfg.getItemId === "function"
        ? bulkCfg.getItemId
        : function (item) {
            return item && item.id != null ? item.id : null;
          };
    async function fetchAuth(url, options) {
      options = options || {};
      options.headers = Object.assign({}, options.headers || {});
      if (!options.headers.Authorization) {
        options.headers.Authorization = "Bearer " + token;
      }
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
        window.location.href = loginUrl;
        throw new Error("unauthorized");
      }
      return res;
    }

    async function fetchJSON(method, url, body) {
      var opt = { method: method || "GET" };
      var m = (method || "GET").toUpperCase();
      if (body !== undefined && body !== null && m !== "GET" && m !== "HEAD") {
        opt.body = typeof body === "string" ? body : JSON.stringify(body);
      }
      return fetchAuth(url, opt);
    }

    function setBtnDisabled(btn, disabled) {
      if (!btn) return;
      btn.disabled = !!disabled;
    }

    function syncBulkUi() {
      var enabled = !!bulkCfg && !!tbody;
      if (bulkDeleteBtn) {
        bulkDeleteBtn.classList.toggle("d-none", !enabled);
        bulkDeleteBtn.disabled = !enabled || bulkBusy || selectedIds.size === 0;
      }
      if (selectedActionsToggle) {
        selectedActionsToggle.classList.toggle(
          "d-none",
          !enabled || selectedIds.size === 0
        );
        selectedActionsToggle.disabled =
          !enabled || bulkBusy || selectedIds.size === 0;
      }
      if (selectedCountEl) {
        selectedCountEl.classList.toggle(
          "d-none",
          !enabled || selectedIds.size === 0
        );
        selectedCountEl.textContent = selectedIds.size + " selected";
      }
      if (selectAllEl) {
        if (!enabled) {
          selectAllEl.checked = false;
          selectAllEl.indeterminate = false;
          selectAllEl.disabled = true;
          return;
        }
        var rowChecks = tbody.querySelectorAll(".admin-crud-row-select");
        var total = rowChecks.length;
        var checked = 0;
        rowChecks.forEach(function (el) {
          if (el.checked) checked += 1;
        });
        selectAllEl.disabled = total === 0 || bulkBusy;
        selectAllEl.checked = total > 0 && checked === total;
        selectAllEl.indeterminate = checked > 0 && checked < total;
      }
    }

    function clearSelections() {
      selectedIds.clear();
      if (tbody) {
        tbody.querySelectorAll(".admin-crud-row-select").forEach(function (el) {
          el.checked = false;
        });
      }
      syncBulkUi();
    }

    function decorateSelectableRow(row, id) {
      if (!bulkCfg || !row || id == null || id === "") return;
      var firstCell = row.querySelector("td");
      if (!firstCell) return;
      var tdSelect = document.createElement("td");
      tdSelect.className = "text-center align-middle";
      var checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.className = "form-check-input admin-crud-row-select";
      checkbox.setAttribute("data-item-id", String(id));
      checkbox.setAttribute("aria-label", "Select row " + id);
      tdSelect.appendChild(checkbox);
      row.insertBefore(tdSelect, firstCell);
    }

    async function deleteSelectedRows() {
      if (
        !bulkCfg ||
        typeof bulkCfg.deleteItem !== "function" ||
        selectedIds.size === 0 ||
        bulkBusy
      ) {
        return;
      }
      var ids = Array.from(selectedIds);
      var confirmTitle = bulkCfg.confirmTitle || "Delete selected items?";
      var confirmText = bulkCfg.confirmText
        ? bulkCfg.confirmText(ids.length)
        : "This will permanently delete " + ids.length + " selected item(s).";
      var confirmed = false;
      if (typeof Swal !== "undefined") {
        var result = await Swal.fire({
          title: confirmTitle,
          text: confirmText,
          icon: "warning",
          showCancelButton: true,
          confirmButtonText: bulkCfg.confirmButtonText || "Delete",
          cancelButtonText: "Cancel",
          focusCancel: true,
        });
        confirmed = !!(result && result.isConfirmed);
      } else {
        confirmed = window.confirm(confirmText);
      }
      if (!confirmed) return;

      bulkBusy = true;
      syncBulkUi();

      var deleted = 0;
      var failed = 0;
      try {
        for (var i = 0; i < ids.length; i += 1) {
          var itemId = ids[i];
          try {
            var maybeErr = await bulkCfg.deleteItem(itemId, {
              fetchJSON: fetchJSON,
              fetchAuth: fetchAuth,
              responseErrorMessage: responseErrorMessage,
            });
            if (typeof maybeErr === "string" && maybeErr) {
              failed += 1;
              continue;
            }
            deleted += 1;
          } catch (err) {
            if (err && err.message === "unauthorized") return;
            failed += 1;
          }
        }
      } finally {
        bulkBusy = false;
      }

      clearSelections();
      await loadList();

      if (deleted > 0) {
        var okText = bulkCfg.successText
          ? bulkCfg.successText(deleted, failed)
          : deleted + " item(s) deleted.";
        showSuccessToast(okText);
      }
      if (failed > 0) {
        showErrorToast(
          (bulkCfg.failedPrefix || "Failed to delete") +
            " " +
            failed +
            " item(s)."
        );
      }
      syncBulkUi();
    }

    async function loadList() {
      try {
        setGlobalError(errorEl, null);
        var listUrl =
          typeof cfg.buildListURL === "function"
            ? cfg.buildListURL(
                state.page,
                state.size,
                state.query || "",
                ctx
              )
            : cfg.api && typeof cfg.api.list === "function"
              ? cfg.api.list(state.page, state.size, state.query || "")
              : "";
        var res = await fetchAuth(listUrl, { method: "GET" });

        if (!res.ok) {
          var data = await res.json().catch(function () {
            return null;
          });
          setGlobalError(
            errorEl,
            (data && data.error) ||
              (messages.loadFailed || "Failed to load") +
                " (" +
                res.status +
                ")"
          );
          return;
        }

        var payload = await res.json();
        var items = parseItems(payload);
        if (!Array.isArray(items)) {
          setGlobalError(
            errorEl,
            messages.unexpectedResponse || "Unexpected response from server."
          );
          return;
        }

        state.totalPages = parseTotalPages(payload);
        if (!state.totalPages && Array.isArray(payload)) {
          state.totalPages = 1;
        }

        if (tbody) tbody.innerHTML = "";
        clearSelections();
        if (items.length === 0) {
          cfg.renderEmpty(tbody, state);
        } else {
          items.forEach(function (item) {
            var row = cfg.renderRow(item);
            decorateSelectableRow(row, getBulkItemId(item));
            tbody.appendChild(row);
          });
        }

        if (pageInfoEl) {
          if (state.totalPages > 0) {
            pageInfoEl.textContent = messages.pageOf
              ? messages.pageOf(state.page, state.totalPages)
              : "Page " + state.page + " of " + state.totalPages;
          } else {
            pageInfoEl.textContent = messages.noItems || "No items found";
          }
        }

        if (state.totalPages > 0) {
          setBtnDisabled(prevBtn, state.page <= 1);
          setBtnDisabled(nextBtn, state.page >= state.totalPages);
        } else {
          setBtnDisabled(prevBtn, true);
          setBtnDisabled(nextBtn, true);
        }
      } catch (err) {
        if (err && err.message === "unauthorized") return;
        setGlobalError(
          errorEl,
          messages.network || "Network error. Please try again."
        );
      }
    }

    if (pageSizeSelect) {
      pageSizeSelect.addEventListener("change", function () {
        state.size = normalizePageSize(pageSizeSelect.value);
        state.page = 1;
        loadList();
      });
      pageSizeSelect.value = String(state.size);
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
      // Fires when user clears a <input type="search"> via the native "x" affordance.
      searchInput.addEventListener("search", triggerSearch);
      searchInput.addEventListener("keydown", function (e) {
        if (e.key === "Enter") {
          e.preventDefault();
          triggerSearch();
        }
      });

      var clearBtn = document.querySelector(
        '[data-admin-crud-search-clear="' + searchInput.id + '"]'
      );
      if (clearBtn) {
        clearBtn.addEventListener("click", function () {
          if (!searchInput.value) return;
          searchInput.value = "";
          triggerSearch();
          searchInput.focus();
        });
      }
    }

    if (tbody && (cfg.onEditClick || cfg.onDeleteClick)) {
      tbody.addEventListener("click", function (e) {
        if (cfg.onDeleteClick) {
          var del = e.target.closest(".admin-crud-btn-delete");
          if (del) {
            cfg.onDeleteClick(del.getAttribute("data-item-id"));
            return;
          }
        }
        if (cfg.onEditClick) {
          var ed = e.target.closest(".admin-crud-btn-edit");
          if (ed) cfg.onEditClick(ed.getAttribute("data-item-id"));
        }
      });
    }

    if (tbody && bulkCfg) {
      tbody.addEventListener("change", function (e) {
        var cb = e.target.closest(".admin-crud-row-select");
        if (!cb) return;
        var itemId = cb.getAttribute("data-item-id");
        if (!itemId) return;
        if (cb.checked) {
          selectedIds.add(itemId);
        } else {
          selectedIds.delete(itemId);
        }
        syncBulkUi();
      });
    }

    if (selectAllEl && tbody && bulkCfg) {
      selectAllEl.addEventListener("change", function () {
        var targetChecked = !!selectAllEl.checked;
        tbody.querySelectorAll(".admin-crud-row-select").forEach(function (cb) {
          cb.checked = targetChecked;
          var itemId = cb.getAttribute("data-item-id");
          if (!itemId) return;
          if (targetChecked) selectedIds.add(itemId);
          else selectedIds.delete(itemId);
        });
        syncBulkUi();
      });
    }

    if (bulkDeleteBtn && bulkCfg) {
      bulkDeleteBtn.addEventListener("click", function () {
        deleteSelectedRows();
      });
    }

    ctx.loadList = loadList;
    ctx.fetchAuth = fetchAuth;
    ctx.fetchJSON = fetchJSON;
    ctx.hideModal = hideModal;
    ctx.setInlineError = setInlineError;
    ctx.showSuccessToast = showSuccessToast;
    ctx.showErrorToast = showErrorToast;
    ctx.qs = qs;

    if (typeof cfg.afterBind === "function") {
      cfg.afterBind(ctx);
    }

    loadList();
    syncBulkUi();
    return ctx;
  }

  function escapeHtmlAILimit(s) {
    return String(s || "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  /** SweetAlert for OpenAI / Groq / Gemini quota & rate-limit responses from the API. */
  global.AdminAILimit = {
    showFromResponse: function (data) {
      if (!data || typeof data !== "object") {
        return false;
      }
      var code = data.code;
      if (
        code !== "openai_quota" &&
        code !== "groq_quota" &&
        code !== "gemini_quota"
      ) {
        return false;
      }
      var prov = String(data.provider || "").toLowerCase();
      var names = {
        openai: "OpenAI",
        groq: "Groq",
        gemini: "Google Gemini",
      };
      var provLabel = names[prov] || prov || "AI provider";
      var msg = data.error || "Quota or rate limit reached.";
      if (typeof Swal !== "undefined") {
        Swal.fire({
          icon: "warning",
          title: "Limit · " + provLabel,
          html: '<p class="text-start mb-0">' + escapeHtmlAILimit(msg) + "</p>",
          confirmButtonText: "OK",
        });
      } else {
        window.alert(provLabel + ": " + msg);
      }
      return true;
    },
  };

  /**
   * Root-relative path prefixed with PUBLIC_SITE_URL origin when the admin layout sets
   * body[data-public-site-origin] (canonical public hostname, e.g. apex without www).
   */
  function publicSitePath(path) {
    var p = path == null ? "" : String(path).trim();
    if (!p) return "";
    if (p.charAt(0) !== "/") p = "/" + p;
    var root = "";
    try {
      var b = document.body;
      if (b && b.getAttribute) {
        root = (b.getAttribute("data-public-site-origin") || "").trim();
      }
    } catch (e) {}
    if (root) return root.replace(/\/$/, "") + p;
    return p;
  }

  function publicArticleHref(slug, categorySlug) {
    var s = slug == null ? "" : String(slug).trim();
    if (!s) return "";
    var mode = "";
    try {
      var b = document.body;
      if (b && b.getAttribute) {
        mode = String(b.getAttribute("data-article-slug-mode") || "")
          .toLowerCase()
          .trim();
      }
    } catch (e) {}
    var path =
      mode === "category" &&
      categorySlug != null &&
      String(categorySlug).trim()
        ? "/" +
          encodeURIComponent(String(categorySlug).trim()) +
          "/" +
          encodeURIComponent(s)
        : "/" + encodeURIComponent(s);
    return publicSitePath(path);
  }

  global.AdminCrud = {
    qs: qs,
    getToken: getToken,
    formatDate: formatDate,
    initials: initials,
    showSuccessToast: showSuccessToast,
    showErrorToast: showErrorToast,
    confirmAction: confirmAction,
    setGlobalError: setGlobalError,
    setInlineError: setInlineError,
    hideModal: hideModal,
    bindPage: bindPage,
    defaultParseItems: defaultParseItems,
    defaultParseTotalPages: defaultParseTotalPages,
    itemUrl: itemUrl,
    responseErrorMessage: responseErrorMessage,
    publicSitePath: publicSitePath,
    publicArticleHref: publicArticleHref,
  };
})(window);
