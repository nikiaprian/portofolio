(function () {
  "use strict";

  var API = "/api/v1/admin/scraper-setting";
  var API_USERS = "/api/v1/admin/users";
  var RUN_NOW_API = "/api/v1/admin/scraper-setting/run-now";
  var TOKEN_KEY = "bebii_token";
  var LOGIN_URL =
    (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login";

  var saveLock = false;
  var activeSavePromise = null;
  var pendingSaveOptions = null;

  var SCRAPER_WEB_SEARCH_SOURCES = [
    { value: "auto", label: "Auto (Pexels → Unsplash → Bing.net → Google → Bing)" },
    { value: "pexels", label: "Pexels" },
    { value: "unsplash", label: "Unsplash" },
    { value: "bing_net", label: "Bing.net" },
    { value: "google", label: "Google" },
    { value: "bing", label: "Bing" },
  ];

  function normalizeScraperWebSearchSource(raw) {
    var v = raw != null ? String(raw).toLowerCase().trim() : "";
    for (var i = 0; i < SCRAPER_WEB_SEARCH_SOURCES.length; i++) {
      if (SCRAPER_WEB_SEARCH_SOURCES[i].value === v) return v;
    }
    return "auto";
  }

  function syncScraperWebSearchSourceVisibility(imgModeSelect, webSourceWrap) {
    if (!imgModeSelect || !webSourceWrap) return;
    var show =
      String(imgModeSelect.value || "")
        .toLowerCase()
        .trim() === "web_search";
    webSourceWrap.classList.toggle("d-none", !show);
  }

  function qs(sel) {
    return document.querySelector(sel);
  }

  function getToken() {
    try {
      return localStorage.getItem(TOKEN_KEY);
    } catch (e) {
      return null;
    }
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

  function showSuccessToast(message) {
    if (!message || typeof Swal === "undefined") return;
    Swal.mixin({
      toast: true,
      position: "top-end",
      showConfirmButton: false,
      timer: 2400,
      timerProgressBar: true,
    }).fire({
      icon: "success",
      title: message,
    });
  }

  function queueSave(options) {
    options = options || {};
    if (!pendingSaveOptions) pendingSaveOptions = {};
    if (options.toastMsg) pendingSaveOptions.toastMsg = options.toastMsg;
    if (options.useSubmitBusy) pendingSaveOptions.useSubmitBusy = true;
  }

  function consumeQueuedSave() {
    var next = pendingSaveOptions;
    pendingSaveOptions = null;
    return next;
  }

  function populateAuthorSelectOptions(selectEl, users, selectedEmail) {
    if (!selectEl) return;
    var email = selectedEmail != null ? String(selectedEmail).trim() : "";
    selectEl.innerHTML = "";
    var emptyOpt = document.createElement("option");
    emptyOpt.value = "";
    emptyOpt.textContent = "Pilih penulis";
    selectEl.appendChild(emptyOpt);
    (users || []).forEach(function (u) {
      var em = u && u.email ? String(u.email).trim() : "";
      if (!em) return;
      var name = u && u.name ? String(u.name).trim() : "";
      var opt = document.createElement("option");
      opt.value = em;
      opt.textContent = name ? name + " (" + em + ")" : em;
      selectEl.appendChild(opt);
    });
    setAuthorSelectValue(selectEl, email);
  }

  async function loadAuthorUsersList() {
    var res = await fetchAuth(
      API_USERS +
        "?page=" +
        encodeURIComponent(1) +
        "&size=" +
        encodeURIComponent(500) +
        "&q=",
      { method: "GET" }
    );
    if (!res.ok) return [];
    var payload = await res.json().catch(function () {
      return null;
    });
    return payload && Array.isArray(payload.items) ? payload.items : [];
  }

  function renderCategoryFeeds(container, entries, users) {
    users = users || [];
    if (!container) return;
    container.innerHTML = "";
    if (!entries || !entries.length) {
      var empty = document.createElement("p");
      empty.className = "small text-secondary mb-0";
      empty.textContent =
        "Belum ada kategori. Tambahkan kategori terlebih dulu.";
      container.appendChild(empty);
      return;
    }
    entries.forEach(function (entry, idx) {
      var slug = entry.category_slug || "";
      var name = entry.category_name || slug;
      var urls = entry.feed_urls != null ? String(entry.feed_urls) : "";
      var catOn = entry.enabled !== false;
      var includeOn = entry.include_source !== false;
      var articleStatusRaw =
        entry.article_status != null ? String(entry.article_status) : "";
      var articleStatus =
        articleStatusRaw.toLowerCase().trim() === "draft"
          ? "draft"
          : "published";
      var imgModeRaw = entry.image_mode != null ? String(entry.image_mode) : "";
      var imgNorm = imgModeRaw.toLowerCase().trim();
      var imgMode =
        imgNorm === "generate"
          ? "generate"
          : imgNorm === "generate_image"
          ? "generate_image"
          : imgNorm === "web_search"
          ? "web_search"
          : "scrape";
      var imgToContent = !!entry.image_to_content;
      var webSearchSourceRaw =
        entry.web_search_source != null ? String(entry.web_search_source) : "";
      var webSearchSource = normalizeScraperWebSearchSource(webSearchSourceRaw);
      var idBase = "scr-feed-" + idx + "-" + slug.replace(/[^a-z0-9_-]/gi, "");
      var swId = idBase + "-en";
      var srcSwId = idBase + "-src";
      var articleStatusId = idBase + "-status";
      var imgModeId = idBase + "-imgmode";
      var webSourceId = idBase + "-websource";

      var wrap = document.createElement("div");
      wrap.className = "scr-cat-feed-card border rounded-2 p-3";
      wrap.style.background = "var(--admin-bg-secondary)";
      wrap.style.borderColor = "var(--admin-border)";

      var head = document.createElement("div");
      head.className =
        "d-flex flex-wrap align-items-center justify-content-between gap-2 mb-3 pb-2 border-bottom border-opacity-25";

      var lab = document.createElement("label");
      lab.className = "form-label fw-semibold mb-0";
      lab.setAttribute("for", idBase);
      lab.appendChild(document.createTextNode(name + " "));
      var sp = document.createElement("span");
      sp.className = "text-secondary fw-normal small";
      sp.textContent = "(" + slug + ")";
      lab.appendChild(sp);

      var swWrap = document.createElement("div");
      swWrap.className = "form-check form-switch m-0";
      var sw = document.createElement("input");
      sw.type = "checkbox";
      sw.className = "form-check-input scr-cat-enabled";
      sw.id = swId;
      sw.setAttribute("role", "switch");
      sw.setAttribute("data-category-slug", slug);
      sw.checked = catOn;
      var swLab = document.createElement("label");
      swLab.className = "form-check-label small ms-1";
      swLab.setAttribute("for", swId);
      swLab.textContent = "Aktif";
      swWrap.appendChild(sw);
      swWrap.appendChild(swLab);

      head.appendChild(lab);
      head.appendChild(swWrap);

      // Per-category options: include source, then author + image mode on one row.
      var opts = document.createElement("div");
      opts.className =
        "scr-cat-opt-row d-flex flex-wrap align-items-center column-gap-3 row-gap-2 mb-3";

      var srcWrap = document.createElement("div");
      srcWrap.className = "form-check form-switch m-0 me-sm-2";
      var srcSw = document.createElement("input");
      srcSw.type = "checkbox";
      srcSw.className = "form-check-input scr-cat-include-source";
      srcSw.id = srcSwId;
      srcSw.setAttribute("role", "switch");
      srcSw.setAttribute("data-category-slug", slug);
      srcSw.checked = includeOn;
      var srcLab = document.createElement("label");
      srcLab.className = "form-check-label small ms-1";
      srcLab.setAttribute("for", srcSwId);
      srcLab.textContent = "Sertakan sumber";
      srcWrap.appendChild(srcSw);
      srcWrap.appendChild(srcLab);

      var authorWrap = document.createElement("div");
      authorWrap.className =
        "scr-cat-field-inline d-flex align-items-center gap-2 flex-shrink-0";
      var authorLab = document.createElement("label");
      authorLab.className =
        "form-label small mb-0 text-secondary fw-semibold text-nowrap";
      authorLab.setAttribute("for", idBase + "-author");
      authorLab.textContent = "Penulis";
      var authorSel = document.createElement("select");
      authorSel.className = "form-select form-select-sm scr-cat-author";
      authorSel.id = idBase + "-author";
      authorSel.setAttribute("data-category-slug", slug);
      var entryAuthor =
        entry.author_email != null ? String(entry.author_email).trim() : "";
      populateAuthorSelectOptions(authorSel, users, entryAuthor);
      authorWrap.appendChild(authorLab);
      authorWrap.appendChild(authorSel);

      var imgWrap = document.createElement("div");
      imgWrap.className =
        "scr-cat-field-inline d-flex align-items-center gap-2 flex-shrink-0";

      var imgLab = document.createElement("label");
      imgLab.className =
        "form-label small mb-0 text-secondary fw-semibold text-nowrap";
      imgLab.setAttribute("for", imgModeId);
      imgLab.textContent = "Gambar:";

      var imgSel = document.createElement("select");
      imgSel.className = "form-select form-select-sm scr-cat-image-mode";
      imgSel.id = imgModeId;
      imgSel.setAttribute("data-category-slug", slug);
      var optScrape = document.createElement("option");
      optScrape.value = "scrape";
      optScrape.textContent = "Scrape gambar";
      var optGen = document.createElement("option");
      optGen.value = "generate";
      optGen.textContent = "Pakai template";
      var optGenImage = document.createElement("option");
      optGenImage.value = "generate_image";
      optGenImage.textContent = "Generate AI";
      var optWebSearch = document.createElement("option");
      optWebSearch.value = "web_search";
      optWebSearch.textContent = "Cari di web";
      imgSel.appendChild(optScrape);
      imgSel.appendChild(optWebSearch);
      imgSel.appendChild(optGen);
      imgSel.appendChild(optGenImage);
      imgSel.value = imgMode;

      imgWrap.appendChild(imgLab);
      imgWrap.appendChild(imgSel);

      var webSourceWrap = document.createElement("div");
      webSourceWrap.className =
        "scr-cat-field-inline d-flex align-items-center gap-2 flex-shrink-0" +
        (imgMode === "web_search" ? "" : " d-none");
      webSourceWrap.id = idBase + "-websource-wrap";
      var webSourceLab = document.createElement("label");
      webSourceLab.className =
        "form-label small mb-0 text-secondary fw-semibold text-nowrap";
      webSourceLab.setAttribute("for", webSourceId);
      webSourceLab.textContent = "Sumber web:";
      var webSourceSel = document.createElement("select");
      webSourceSel.className =
        "form-select form-select-sm scr-cat-web-search-source";
      webSourceSel.id = webSourceId;
      webSourceSel.setAttribute("data-category-slug", slug);
      SCRAPER_WEB_SEARCH_SOURCES.forEach(function (optDef) {
        var o = document.createElement("option");
        o.value = optDef.value;
        o.textContent = optDef.label;
        webSourceSel.appendChild(o);
      });
      webSourceSel.value = webSearchSource;
      imgSel.addEventListener("change", function () {
        syncScraperWebSearchSourceVisibility(imgSel, webSourceWrap);
      });
      webSourceWrap.appendChild(webSourceLab);
      webSourceWrap.appendChild(webSourceSel);

      var imgToContentWrap = document.createElement("div");
      var imgToContentWrapId = idBase + "-img2content-wrap";
      imgToContentWrap.className =
        "form-check form-switch m-0 ms-2" +
        (imgMode === "generate_image" ? "" : " d-none");
      imgToContentWrap.id = imgToContentWrapId;
      var imgToContentId = idBase + "-img2content";
      var imgToContentSw = document.createElement("input");
      imgToContentSw.type = "checkbox";
      imgToContentSw.className = "form-check-input scr-cat-image-to-content";
      imgToContentSw.id = imgToContentId;
      imgToContentSw.setAttribute("role", "switch");
      imgToContentSw.setAttribute("data-category-slug", slug);
      imgToContentSw.checked = imgToContent;
      var imgToContentLab = document.createElement("label");
      imgToContentLab.className = "form-check-label small ms-1";
      imgToContentLab.setAttribute("for", imgToContentId);
      imgToContentLab.textContent = "Gambar ke konten";
      imgToContentWrap.appendChild(imgToContentSw);
      imgToContentWrap.appendChild(imgToContentLab);

      var selectsGroup = document.createElement("div");
      selectsGroup.className =
        "d-flex flex-wrap align-items-center column-gap-3 row-gap-2 ms-md-auto";

      var statusWrap = document.createElement("div");
      statusWrap.className =
        "scr-cat-field-inline d-flex align-items-center gap-2 flex-shrink-0";
      var statusLab = document.createElement("label");
      statusLab.className =
        "form-label small mb-0 text-secondary fw-semibold text-nowrap";
      statusLab.setAttribute("for", articleStatusId);
      statusLab.textContent = "Status:";
      var statusSel = document.createElement("select");
      statusSel.className = "form-select form-select-sm scr-cat-article-status";
      statusSel.id = articleStatusId;
      statusSel.setAttribute("data-category-slug", slug);
      var stPublished = document.createElement("option");
      stPublished.value = "published";
      stPublished.textContent = "Terbit";
      var stDraft = document.createElement("option");
      stDraft.value = "draft";
      stDraft.textContent = "Draf";
      statusSel.appendChild(stPublished);
      statusSel.appendChild(stDraft);
      statusSel.value = articleStatus;
      statusWrap.appendChild(statusLab);
      statusWrap.appendChild(statusSel);

      selectsGroup.appendChild(statusWrap);
      selectsGroup.appendChild(authorWrap);
      selectsGroup.appendChild(imgWrap);
      selectsGroup.appendChild(webSourceWrap);
      selectsGroup.appendChild(imgToContentWrap);

      opts.appendChild(srcWrap);
      opts.appendChild(selectsGroup);

      var ta = document.createElement("textarea");
      ta.className = "form-control scr-cat-feed-input font-monospace small";
      ta.id = idBase;
      ta.rows = 1;
      ta.setAttribute("data-category-slug", slug);
      ta.placeholder = "https://example.com/rss, https://example.com/feed";
      ta.value = urls;
      var emptyHint = document.createElement("div");
      emptyHint.className = "small text-danger mt-2";
      emptyHint.textContent = "No URL RSS/Feed/XML";

      function syncEmptyHint() {
        emptyHint.classList.toggle(
          "d-none",
          String(ta.value || "").trim() !== ""
        );
      }
      ta.addEventListener("input", syncEmptyHint);
      syncEmptyHint();

      wrap.appendChild(head);
      wrap.appendChild(opts);
      wrap.appendChild(ta);
      wrap.appendChild(emptyHint);
      container.appendChild(wrap);
    });
  }

  async function fillForm(data) {
    if (!data) return;
    var enabled = qs("#scr-enabled");
    var interval = qs("#scr-interval-mins");
    var pauseBetween = qs("#scr-pause-between-categories-mins");
    var aiProvider = qs("#scr-ai-provider");
    var mount = qs("#scr-category-feeds");
    if (enabled) enabled.checked = !!data.enabled;
    if (interval && data.interval_mins != null)
      interval.value = data.interval_mins;
    if (pauseBetween && data.pause_between_categories_mins != null)
      pauseBetween.value = data.pause_between_categories_mins;
    if (aiProvider) {
      var p =
        data.scraper_ai_provider != null
          ? String(data.scraper_ai_provider).toLowerCase().trim()
          : "";
      if (
        p !== "off" &&
        p !== "auto" &&
        p !== "gemini" &&
        p !== "openai" &&
        p !== "groq" &&
        p !== "claude"
      ) {
        p = "auto";
      }
      aiProvider.value = p || "auto";
    }
    var users = await loadAuthorUsersList();
    renderCategoryFeeds(mount, data.category_feeds, users);
  }

  function setAuthorSelectValue(selectEl, emailValue) {
    if (!selectEl) return;
    var email = emailValue != null ? String(emailValue).trim() : "";
    selectEl.setAttribute("data-selected-email", email);
    if (!email) {
      selectEl.value = "";
      return;
    }
    var existing = Array.prototype.slice
      .call(selectEl.options || [])
      .find(function (opt) {
        return (
          String(opt.value || "")
            .trim()
            .toLowerCase() === email.toLowerCase()
        );
      });
    if (!existing) {
      var fallback = document.createElement("option");
      fallback.value = email;
      fallback.textContent = email + " (tidak ditemukan)";
      fallback.setAttribute("data-fallback", "1");
      selectEl.appendChild(fallback);
    }
    selectEl.value = email;
  }

  function collectCategoryFeeds() {
    var mount = qs("#scr-category-feeds");
    if (!mount) return [];
    var out = [];
    var wraps = mount.querySelectorAll(".scr-cat-feed-input");
    wraps.forEach(function (ta) {
      var slug = (ta.getAttribute("data-category-slug") || "").trim();
      if (!slug) return;
      var swId = ta.id ? ta.id + "-en" : "";
      var sw = swId ? document.getElementById(swId) : null;
      var on = sw ? !!sw.checked : true;

      var srcSwId = ta.id ? ta.id + "-src" : "";
      var srcSw = srcSwId ? document.getElementById(srcSwId) : null;
      var includeOn = srcSw ? !!srcSw.checked : true;

      var imgModeId = ta.id ? ta.id + "-imgmode" : "";
      var imgSel = imgModeId ? document.getElementById(imgModeId) : null;
      var imgMode =
        imgSel && imgSel.value
          ? String(imgSel.value).toLowerCase().trim()
          : "scrape";
      imgMode =
        imgMode === "generate"
          ? "generate"
          : imgMode === "generate_image"
          ? "generate_image"
          : imgMode === "web_search"
          ? "web_search"
          : "scrape";
      var imgToContentId = ta.id ? ta.id + "-img2content" : "";
      var imgToContentSw = imgToContentId
        ? document.getElementById(imgToContentId)
        : null;
      var imgToContent = !!(imgToContentSw && imgToContentSw.checked);
      if (imgMode !== "generate_image") {
        imgToContent = false;
      }

      var webSourceIdCollect = ta.id ? ta.id + "-websource" : "";
      var webSourceSelCollect = webSourceIdCollect
        ? document.getElementById(webSourceIdCollect)
        : null;
      var webSearchSourceOut = "auto";
      if (
        imgMode === "web_search" &&
        webSourceSelCollect &&
        webSourceSelCollect.value
      ) {
        webSearchSourceOut = normalizeScraperWebSearchSource(
          webSourceSelCollect.value
        );
      }

      var articleStatusId = ta.id ? ta.id + "-status" : "";
      var articleStatusSel = articleStatusId
        ? document.getElementById(articleStatusId)
        : null;
      var articleStatus =
        articleStatusSel && articleStatusSel.value
          ? String(articleStatusSel.value).toLowerCase().trim()
          : "published";
      articleStatus = articleStatus === "draft" ? "draft" : "published";

      var authId = ta.id ? ta.id + "-author" : "";
      var authSel = authId ? document.getElementById(authId) : null;
      var authorEmail =
        authSel && authSel.value ? String(authSel.value).trim() : "";

      out.push({
        category_slug: slug,
        feed_urls: (ta.value || "").trim(),
        enabled: on,
        include_source: includeOn,
        article_status: articleStatus,
        image_mode: imgMode,
        web_search_source: webSearchSourceOut,
        image_to_content: imgToContent,
        author_email: authorEmail,
      });
    });
    return out;
  }

  function buildPayload() {
    var intervalRaw =
      (qs("#scr-interval-mins") && qs("#scr-interval-mins").value) || "5";
    var intervalMins = parseInt(intervalRaw, 10);
    if (!intervalMins || intervalMins < 1) intervalMins = 5;
    var pauseRaw =
      (qs("#scr-pause-between-categories-mins") &&
        qs("#scr-pause-between-categories-mins").value) ||
      "0";
    var pauseBetween = parseInt(pauseRaw, 10);
    if (isNaN(pauseBetween) || pauseBetween < 0) pauseBetween = 0;
    if (pauseBetween > 1440) pauseBetween = 1440;
    return {
      enabled: !!(qs("#scr-enabled") && qs("#scr-enabled").checked),
      interval_mins: intervalMins,
      pause_between_categories_mins: pauseBetween,
      scraper_ai_provider:
        (qs("#scr-ai-provider") && qs("#scr-ai-provider").value) || "auto",
      category_feeds: collectCategoryFeeds(),
    };
  }

  async function fetchAuth(url, options) {
    var token = getToken();
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

  function saveScraper(errEl, options) {
    options = options || {};
    var toastMsg = options.toastMsg;
    var skipFill = !!options.skipFill;
    if (saveLock) {
      queueSave(options);
      return activeSavePromise || Promise.resolve(null);
    }
    saveLock = true;
    var submitBtn = qs("#scr-submit");
    if (submitBtn && options.useSubmitBusy) {
      submitBtn.disabled = true;
      submitBtn.setAttribute("aria-busy", "true");
    }
    var body = buildPayload();
    activeSavePromise = fetchAuth(API, {
      method: "PUT",
      body: JSON.stringify(body),
    })
      .then(async function (res) {
        if (res.ok) {
          var data = await res.json().catch(function () {
            return null;
          });
          if (data && !pendingSaveOptions && !skipFill) await fillForm(data);
          setGlobalError(errEl, null);
          if (toastMsg) showSuccessToast(toastMsg);
          return data;
        }
        var msg = "Gagal menyimpan (" + res.status + ")";
        try {
          var j = await res.json();
          if (j && j.error) msg = j.error;
        } catch (x) {}
        setGlobalError(errEl, msg);
        return null;
      })
      .catch(function (e) {
        if (e && e.message === "unauthorized") return null;
        setGlobalError(
          errEl,
          "Error jaringan. Periksa koneksi lalu coba lagi."
        );
        return null;
      })
      .finally(function () {
        saveLock = false;
        activeSavePromise = null;
        if (submitBtn && options.useSubmitBusy) {
          submitBtn.disabled = false;
          submitBtn.removeAttribute("aria-busy");
        }
        var nextOptions = consumeQueuedSave();
        if (nextOptions) saveScraper(errEl, nextOptions);
      });
    return activeSavePromise;
  }

  document.addEventListener("DOMContentLoaded", function () {
    if (!getToken()) {
      window.location.href = LOGIN_URL;
      return;
    }

    var errEl = qs("#scraper-settings-error");
    var form = qs("#form-scraper-settings");
    var runNowBtn = qs("#scr-run-now");

    fetchAuth(API, { method: "GET" })
      .then(async function (res) {
        if (res.ok) return res.json();
        var msg = "Gagal memuat pengaturan scraper (" + res.status + ")";
        try {
          var j = await res.json();
          if (j && j.error) msg = j.error;
        } catch (x) {}
        setGlobalError(errEl, msg);
        return null;
      })
      .then(async function (data) {
        if (data) {
          setGlobalError(errEl, null);
          await fillForm(data);
        }
      })
      .catch(function (e) {
        if (e && e.message === "unauthorized") return;
        setGlobalError(
          errEl,
          "Error jaringan. Periksa koneksi lalu coba lagi."
        );
      });

    if (!form) return;

    form.addEventListener("change", function (ev) {
      var t = ev.target;
      if (!t) return;
      if (t.id === "scr-enabled") {
        var on = !!t.checked;
        var mount = qs("#scr-category-feeds");
        if (mount) {
          mount.querySelectorAll(".scr-cat-enabled").forEach(function (el) {
            el.checked = on;
          });
        }
        saveScraper(errEl, { toastMsg: "Tersimpan.", skipFill: true });
        return;
      }
      if (
        t.id === "scr-pause-between-categories-mins" ||
        t.id === "scr-ai-provider" ||
        t.classList.contains("scr-cat-enabled") ||
        t.classList.contains("scr-cat-include-source") ||
        t.classList.contains("scr-cat-article-status") ||
        t.classList.contains("scr-cat-image-mode") ||
        t.classList.contains("scr-cat-web-search-source") ||
        t.classList.contains("scr-cat-image-to-content") ||
        t.classList.contains("scr-cat-author")
      ) {
        if (t.classList.contains("scr-cat-image-mode")) {
          var id = String(t.id || "").trim(); // e.g. scr-feed-0-tech-imgmode
          var base = id ? id.replace(/-imgmode$/, "") : "";
          var wrapId = base ? base + "-img2content-wrap" : "";
          var swId = base ? base + "-img2content" : "";
          var webWrapId = base ? base + "-websource-wrap" : "";
          var wrap = wrapId ? document.getElementById(wrapId) : null;
          var sw = swId ? document.getElementById(swId) : null;
          var webWrap = webWrapId ? document.getElementById(webWrapId) : null;
          var isGenAI =
            String(t.value || "")
              .toLowerCase()
              .trim() === "generate_image";
          if (wrap) wrap.classList.toggle("d-none", !isGenAI);
          if (sw && !isGenAI) sw.checked = false;
          syncScraperWebSearchSourceVisibility(t, webWrap);
        }
        saveScraper(errEl, { toastMsg: "Tersimpan.", skipFill: true });
      }
    });

    form.addEventListener("submit", function (e) {
      e.preventDefault();
      setGlobalError(errEl, null);
      saveScraper(errEl, {
        toastMsg: "Pengaturan scraper tersimpan.",
        useSubmitBusy: true,
      });
    });

    if (runNowBtn) {
      runNowBtn.addEventListener("click", function () {
        if (saveLock) return;
        setGlobalError(errEl, null);
        runNowBtn.disabled = true;
        runNowBtn.setAttribute("aria-busy", "true");
        fetchAuth(RUN_NOW_API, { method: "POST" })
          .then(async function (res) {
            if (res.status === 202 || res.ok) {
              showSuccessToast("Run scraper dimulai.");
              return;
            }
            var msg = "Gagal menjalankan scraper (" + res.status + ")";
            try {
              var j = await res.json();
              if (j && j.error) msg = j.error;
            } catch (x) {}
            setGlobalError(errEl, msg);
          })
          .catch(function (e) {
            if (e && e.message === "unauthorized") return;
            setGlobalError(
              errEl,
              "Error jaringan. Periksa koneksi lalu coba lagi."
            );
          })
          .finally(function () {
            runNowBtn.disabled = false;
            runNowBtn.removeAttribute("aria-busy");
          });
      });
    }
  });
})();
