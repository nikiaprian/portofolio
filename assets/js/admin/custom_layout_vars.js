(function () {
  var AC = window.AdminCrud;
  var root = document.getElementById("admin-custom-layout-form-root");
  if (!root) return;

  var API = "/api/v1/admin/site-layouts";
  var TOKEN_KEY = "bebii_token";
  var LOGIN_URL =
    (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login";

  var LAYOUT_VARS_CONFIG = {
    default_layout_name: "default",
    fragments: [
      { key: "header", field: "cl-header", label: "Header" },
      { key: "footer", field: "cl-footer", label: "Footer" },
      { key: "home_body", field: "cl-home-body", label: "Home body" },
      { key: "article_body", field: "cl-article-body", label: "Article body" },
      { key: "sidebar", field: "cl-sidebar", label: "Sidebar" },
      { key: "rss", field: "cl-rss", label: "RSS" },
      { key: "author_profile", field: "cl-author-profile", label: "Author profile" },
      { key: "tag", field: "cl-tag", label: "Tag" },
      { key: "kategori", field: "cl-kategori", label: "Kategori" },
      { key: "search_page", field: "cl-search-page", label: "Search page" },
    ],
  };

  var configCache = null;
  var defaultLayoutCache = null;
  var defaultLayoutPromise = null;

  var modalEl = document.getElementById("cl-fragment-vars-modal");
  var modalTitle = document.getElementById("cl-fragment-vars-modal-label");
  var modalSubtitle = document.getElementById("cl-fragment-vars-modal-subtitle");
  var modalBody = document.getElementById("cl-fragment-vars-modal-body");
  var copyBtn = document.getElementById("cl-fragment-vars-copy-btn");
  var copyBtnLabel = document.getElementById("cl-fragment-vars-copy-label");

  var modalContext = {
    fragmentKey: "",
    config: null,
    defaultLayout: null,
  };
  var copyBtnDefaultLabel = "Salin prompt & fragment";

  function escapeHtml(s) {
    return String(s || "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  async function fetchAuth(url, options) {
    if (!AC || typeof AC.getToken !== "function") {
      return fetch(url, options || {});
    }
    var token = AC.getToken(TOKEN_KEY);
    options = options || {};
    options.headers = Object.assign({}, options.headers || {});
    options.headers.Authorization = "Bearer " + token;
    var res = await fetch(url, options);
    if (res.status === 401) {
      window.location.href = LOGIN_URL;
      throw new Error("unauthorized");
    }
    return res;
  }

  function loadConfig() {
    if (configCache) return configCache;
    var defaultName =
      (root.getAttribute("data-default-layout-name") || "").trim() ||
      LAYOUT_VARS_CONFIG.default_layout_name ||
      "default";
    configCache = {
      default_layout_name: defaultName,
      fragments: LAYOUT_VARS_CONFIG.fragments,
    };
    return configCache;
  }

  function layoutFragmentValue(layout, key) {
    if (!layout) return "";
    if (layout[key] != null && String(layout[key]).trim()) {
      return String(layout[key]);
    }
    return "";
  }

  async function loadDefaultLayout(defaultName) {
    if (defaultLayoutCache) return defaultLayoutCache;
    if (defaultLayoutPromise) return defaultLayoutPromise;

    defaultLayoutPromise = (async function () {
      var name = String(defaultName || "default").trim() || "default";
      var url =
        API +
        "?q=" +
        encodeURIComponent(name) +
        "&page=1&size=50";
      var res = await fetchAuth(url, {
        headers: { Accept: "application/json" },
      });
      if (!res.ok) throw new Error("Gagal memuat layout default");
      var data = await res.json();
      var items = (data && data.items) || [];
      var match = items.find(function (row) {
        return (
          String(row && row.layout_name || "")
            .trim()
            .toLowerCase() === name.toLowerCase()
        );
      });
      defaultLayoutCache = match || null;
      return defaultLayoutCache;
    })();

    try {
      return await defaultLayoutPromise;
    } finally {
      defaultLayoutPromise = null;
    }
  }

  function resolvePrompt(defaultLayout) {
    var defVal = String(
      (defaultLayout && defaultLayout.current_prompt) || ""
    ).trim();
    if (defVal) {
      return { value: defVal, source: "default" };
    }
    return { value: "", source: "empty" };
  }

  function resolveFragmentContent(fragmentKey, defaultLayout) {
    var defVal = layoutFragmentValue(defaultLayout, fragmentKey).trim();
    if (defVal) {
      return { value: defVal, source: "default" };
    }
    return { value: "", source: "empty" };
  }

  function sourceBadge(source) {
    if (source === "default") {
      return '<span class="badge text-bg-secondary ms-1">layout default</span>';
    }
    return '<span class="badge text-bg-light border ms-1">kosong</span>';
  }

  function renderPreBlock(content, emptyLabel) {
    var text = String(content || "").trim();
    if (!text) {
      return (
        '<p class="text-secondary fst-italic mb-0">' +
        escapeHtml(emptyLabel || "Tidak ada konten.") +
        "</p>"
      );
    }
    return (
      '<pre class="bg-body-tertiary border rounded small p-2 mb-0 font-monospace" ' +
      'style="max-height:14rem;overflow:auto;white-space:pre-wrap;word-break:break-word;">' +
      escapeHtml(text) +
      "</pre>"
    );
  }

  function fragmentMeta(config, fragmentKey) {
    var fragments = (config && config.fragments) || [];
    return (
      fragments.find(function (f) {
        return f.key === fragmentKey;
      }) || null
    );
  }

  function buildCopyText(fragmentKey, config, defaultLayout) {
    var prompt = resolvePrompt(defaultLayout);
    var fragment = resolveFragmentContent(fragmentKey, defaultLayout);
    var parts = [];
    if (prompt.value) {
      parts.push(prompt.value);
    }
    if (fragment.value) {
      if (parts.length) {
        parts.push("\n\n");
      }
      parts.push(fragment.value);
    }
    return parts.join("");
  }

  function updateCopyButtonState(fragmentKey, config, defaultLayout) {
    if (!copyBtn) return;
    var text = buildCopyText(fragmentKey, config, defaultLayout);
    copyBtn.disabled = !String(text || "").trim();
  }

  async function copyModalContent() {
    var fragmentKey = modalContext.fragmentKey;
    var config = modalContext.config;
    if (!fragmentKey || !config) return;

    var text = buildCopyText(
      fragmentKey,
      config,
      modalContext.defaultLayout
    );
    if (!String(text || "").trim()) return;

    try {
      if (navigator.clipboard && typeof navigator.clipboard.writeText === "function") {
        await navigator.clipboard.writeText(text);
      } else {
        var ta = document.createElement("textarea");
        ta.value = text;
        ta.setAttribute("readonly", "");
        ta.style.position = "fixed";
        ta.style.left = "-9999px";
        document.body.appendChild(ta);
        ta.select();
        document.execCommand("copy");
        document.body.removeChild(ta);
      }

      if (copyBtnLabel) {
        copyBtnLabel.textContent = "Tersalin!";
        setTimeout(function () {
          if (copyBtnLabel) copyBtnLabel.textContent = copyBtnDefaultLabel;
        }, 1600);
      }
      if (AC && typeof AC.showSuccessToast === "function") {
        AC.showSuccessToast("Prompt dan fragment disalin ke clipboard.");
      }
    } catch (e) {
      if (AC && typeof AC.showErrorToast === "function") {
        AC.showErrorToast("Gagal menyalin ke clipboard.");
      }
    }
  }

  function renderContextDoc(config, defaultLayout, activeFragmentKey) {
    var prompt = resolvePrompt(defaultLayout);
    var defaultName =
      (config && config.default_layout_name) || "default";
    var html = "";

    html +=
      '<div class="mb-3">' +
      '<h6 class="fw-semibold mb-2">Current prompt' +
      sourceBadge(prompt.source) +
      "</h6>" +
      renderPreBlock(
        prompt.value,
        "Prompt kosong di layout \"" + defaultName + "\"."
      ) +
      "</div>";

    html +=
      '<h6 class="fw-semibold mb-2">Konten HTML fragment</h6>' +
      '<p class="text-secondary small mb-3">Referensi standar proyek dari layout <code>' +
      escapeHtml(defaultName) +
      "</code> (bukan input form yang sedang diedit).</p>";

    var fragments = (config && config.fragments) || [];
    fragments.forEach(function (frag) {
      var resolved = resolveFragmentContent(frag.key, defaultLayout);
      var isActive = frag.key === activeFragmentKey;
      html +=
        '<div class="border rounded-3 p-2 mb-2' +
        (isActive ? " border-primary bg-primary bg-opacity-10" : "") +
        '">' +
        '<div class="d-flex flex-wrap align-items-center gap-1 mb-2">' +
        '<span class="fw-semibold">' +
        escapeHtml(frag.label) +
        "</span>" +
        sourceBadge(resolved.source) +
        (isActive
          ? '<span class="badge text-bg-info ms-1">fragment ini</span>'
          : "") +
        (resolved.value
          ? '<span class="text-secondary small ms-auto">' +
            resolved.value.length +
            " karakter</span>"
          : "") +
        "</div>" +
        renderPreBlock(
          resolved.value,
          "Konten kosong di layout \"" + defaultName + "\"."
        ) +
        "</div>";
    });

    return html;
  }

  async function openFragmentModal(fragmentKey) {
    if (!modalEl || typeof bootstrap === "undefined") return;

    modalContext.fragmentKey = fragmentKey;
    modalContext.config = null;
    modalContext.defaultLayout = null;
    if (copyBtn) copyBtn.disabled = true;
    if (copyBtnLabel) copyBtnLabel.textContent = copyBtnDefaultLabel;

    if (modalBody) {
      modalBody.innerHTML =
        '<p class="text-secondary mb-0">Memuat prompt dan konten HTML…</p>';
    }
    if (modalTitle) modalTitle.textContent = "Prompt & Konten HTML";
    if (modalSubtitle) {
      modalSubtitle.textContent = "Memuat data…";
    }
    bootstrap.Modal.getOrCreateInstance(modalEl).show();

    try {
      var config = loadConfig();
      var defaultName = config.default_layout_name || "default";
      var defaultLayout = null;
      try {
        defaultLayout = await loadDefaultLayout(defaultName);
      } catch (e) {
        if (String(e && e.message) === "unauthorized") return;
      }

      var meta = fragmentMeta(config, fragmentKey);
      if (modalTitle) {
        modalTitle.textContent =
          (meta && meta.label ? meta.label : "Fragment") +
          " — Prompt & Konten HTML";
      }
      if (modalSubtitle) {
        modalSubtitle.textContent =
          "Current prompt dan konten HTML fragment dari layout standar \"" +
          defaultName +
          "\".";
      }
      modalContext.config = config;
      modalContext.defaultLayout = defaultLayout;
      if (modalBody) {
        modalBody.innerHTML = renderContextDoc(
          config,
          defaultLayout,
          fragmentKey
        );
      }
      updateCopyButtonState(fragmentKey, config, defaultLayout);
    } catch (err) {
      if (String(err && err.message) === "unauthorized") return;
      if (modalBody) {
        modalBody.innerHTML =
          '<p class="text-danger mb-0">Gagal memuat prompt dan konten HTML.</p>';
      }
    }
  }

  root.querySelectorAll(".cl-fragment-vars-info").forEach(function (btn) {
    btn.addEventListener("click", function (ev) {
      ev.preventDefault();
      var key = btn.getAttribute("data-fragment") || "";
      if (!key) return;
      openFragmentModal(key);
    });
  });

  if (copyBtn) {
    copyBtn.addEventListener("click", function () {
      copyModalContent();
    });
  }

  document.addEventListener("cl-layout-ai-reloaded", function () {
    defaultLayoutCache = null;
  });
})();
