(function () {
  "use strict";

  // crud.js loads after this file (end of body); read AdminCrud on DOMContentLoaded.
  var AC = null;

  var TOKEN_KEY = "bebii_token";
  var LOGIN =
    (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login";
  var API_SUGGEST = "/api/v1/admin/posts/bulk-suggest-titles";
  var API_SCRAPE_XML = "/api/v1/admin/posts/bulk-scrape-xml-titles";
  var API_FETCH_GOOGLE_TRENDS = "/api/v1/admin/posts/bulk-fetch-google-trends-titles";
  var API_CHECK_TITLES = "/api/v1/admin/posts/bulk-check-titles";
  var API_REGEN_TITLES = "/api/v1/admin/posts/bulk-regenerate-titles";
  var API_GEN = "/api/v1/admin/posts/bulk-generate";
  var API_GEN_JOB = "/api/v1/admin/posts/bulk-generate-jobs/";
  var API_ACTIVE_GEN_JOB = "/api/v1/admin/posts/bulk-generate-jobs-active";
  var API_TAGS = "/api/v1/admin/tags";

  var tsBulkTags = null;
  var bulkTagModalSlugAutofill = { reset: function () {} };
  var tsBulkDynamicLoadStarted = false;
  var activeBulkGenerateJobId = "";
  var activeBulkGenerateCanceled = false;
  var activeBulkGenerateStopping = false;
  /** Previous #bulk-titles lines (split) to detect auto-generated slugs while typing. */
  var bulkLastTitleLinesForSlugSync = null;

  function qs(sel) {
    return document.querySelector(sel);
  }

  function getToken() {
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

  function getAuthorId() {
    try {
      var raw = localStorage.getItem("bebii_user_id");
      if (raw) {
        var n = parseInt(raw, 10);
        if (!isNaN(n) && n > 0) {
          return n;
        }
      }
    } catch (e) {}
    return 0;
  }

  async function fetchAuth(url, opts) {
    var token = getToken();
    if (!token) {
      window.location.href = LOGIN;
      throw new Error("unauth");
    }
    opts = opts || {};
    opts.headers = Object.assign({}, opts.headers || {});
    opts.headers.Authorization = "Bearer " + token;
    var m = (opts.method || "GET").toUpperCase();
    if (
      opts.body &&
      m !== "GET" &&
      m !== "HEAD" &&
      !opts.headers["Content-Type"]
    ) {
      opts.headers["Content-Type"] = "application/json";
    }
    var res = await fetch(url, opts);
    if (res.status === 401) {
      window.location.href = LOGIN;
      throw new Error("unauth");
    }
    return res;
  }

  function setErr(el, msg) {
    if (!el) {
      return;
    }
    if (!msg) {
      el.classList.add("d-none");
      el.textContent = "";
      return;
    }
    el.classList.remove("d-none");
    el.textContent = msg;
  }

  function setOk(el, msg) {
    if (!el) {
      return;
    }
    if (!msg) {
      el.classList.add("d-none");
      el.textContent = "";
      return;
    }
    el.classList.remove("d-none");
    el.textContent = msg;
  }

  var BULK_SLUG_LINE_RE = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

  /** Same line count for titles & slugs; blank slug line = auto. Returns { err } or { titles, slugs, categoryRefs }. */
  function pairTitlesAndSlugsFromBulkUI() {
    var titleTa = qs("#bulk-titles");
    var slugTa = qs("#bulk-slugs");
    var catTa = qs("#bulk-category-refs");
    var titleText = titleTa ? titleTa.value : "";
    var slugText = slugTa ? slugTa.value : "";
    var catText = catTa ? catTa.value : "";
    var tLines = trimBulkTrailingBlankLines(String(titleText).split(/\r?\n/));
    var sLines = trimBulkTrailingBlankLines(String(slugText).split(/\r?\n/));
    var cLines = trimBulkTrailingBlankLines(String(catText).split(/\r?\n/));
    while (sLines.length < tLines.length) {
      sLines.push("");
    }
    if (sLines.length > tLines.length) {
      return {
        err:
          "Slug punya baris lebih banyak dari judul artikel (" +
          tLines.length +
          " vs " +
          sLines.length +
          ").",
      };
    }
    var hasCategoryRefs = countBulkNonEmptyLines(catText) > 0;
    if (hasCategoryRefs && cLines.length !== tLines.length) {
      return {
        err:
          "Kategori harus punya jumlah baris yang sama dengan judul artikel (" +
          tLines.length +
          " vs " +
          cLines.length +
          ").",
      };
    }
    var titles = [];
    var slugs = [];
    var categoryRefs = [];
    for (var i = 0; i < tLines.length; i++) {
      var ti = tLines[i].trim();
      var si = sLines[i].trim();
      var ci = hasCategoryRefs ? String(cLines[i] || "").trim() : "";
      if (!ti && !si && !ci) {
        continue;
      }
      if (!ti && si) {
        return { err: "Baris " + (i + 1) + ": slug tanpa judul." };
      }
      if (!ti && ci) {
        return { err: "Baris " + (i + 1) + ": kategori tanpa judul." };
      }
      if (ti) {
        if (hasCategoryRefs && !ci) {
          return { err: "Baris " + (i + 1) + ": kategori wajib diisi." };
        }
        if (si) {
          var slo = si.toLowerCase();
          if (slo.length < 3 || slo.length > 255) {
            return {
              err:
                "Baris " +
                (i + 1) +
                ": slug harus 3 sampai 255 karakter.",
            };
          }
          if (!BULK_SLUG_LINE_RE.test(slo)) {
            return {
              err:
                "Baris " +
                (i + 1) +
                ": slug hanya boleh memakai huruf kecil, angka, dan tanda hubung (contoh: slug-post-saya).",
            };
          }
        }
        titles.push(ti);
        slugs.push(si ? si.toLowerCase() : "");
        if (hasCategoryRefs) {
          categoryRefs.push(ci);
        }
      }
    }
    if (!titles.length) {
      return { err: "Isi minimal satu judul." };
    }
    if (titles.length > 1000) {
      return { err: "Maksimal 1000 judul per batch." };
    }
    return { titles: titles, slugs: slugs, categoryRefs: categoryRefs };
  }

  var bulkCheckTitlesLastMatches = [];

  function escapeBulkHtml(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function formatBulkSimilarityPct(sim) {
    var n = Number(sim);
    if (isNaN(n)) {
      return "—";
    }
    return Math.round(n * 100) + "%";
  }

  function bulkCheckSourceLabel(source) {
    if (source === "database") {
      return "Database";
    }
    if (source === "list") {
      return "List";
    }
    return source || "—";
  }

  function removeBulkTitlesByIndices(indices) {
    var pair = pairTitlesAndSlugsFromBulkUI();
    if (pair.err) {
      warnModal("Periksa input", pair.err);
      return false;
    }
    var removeSet = {};
    for (var r = 0; r < indices.length; r++) {
      removeSet[indices[r]] = true;
    }
    var titles = [];
    var slugs = [];
    var cats = [];
    var hadCats = pair.categoryRefs && pair.categoryRefs.length > 0;
    for (var i = 0; i < pair.titles.length; i++) {
      if (removeSet[i]) {
        continue;
      }
      titles.push(pair.titles[i]);
      slugs.push(pair.slugs[i] || "");
      if (hadCats) {
        cats.push(pair.categoryRefs[i] || "");
      }
    }
    var titleTa = qs("#bulk-titles");
    var slugTa = qs("#bulk-slugs");
    var catTa = qs("#bulk-category-refs");
    if (titleTa) {
      titleTa.value = titles.join("\n");
    }
    if (slugTa) {
      slugTa.value = slugs.join("\n");
    }
    if (catTa && hadCats) {
      catTa.value = cats.join("\n");
    }
    if (titleTa) {
      bulkLastTitleLinesForSlugSync = String(titleTa.value).split(/\r?\n/);
    }
    refreshBulkTitleSlugLineCounts();
    syncBulkCategoryInputUI();
    return true;
  }

  function renderBulkCheckTitlesModal(data) {
    var tbody = qs("#bulk-check-titles-tbody");
    var sub = qs("#bulk-check-titles-modal-sub");
    if (!tbody) {
      return;
    }
    var matches = (data && data.matches) || [];
    bulkCheckTitlesLastMatches = matches.slice();
    if (sub) {
      sub.textContent =
        matches.length +
        " judul mirip atau berpotensi duplikat dengan post di database atau baris sebelumnya dalam list.";
    }
    var html = "";
    for (var i = 0; i < matches.length; i++) {
      var m = matches[i];
      var rowNum = (m.index != null ? m.index : i) + 1;
      var oldT = escapeBulkHtml(m.old_title || "");
      var newT = escapeBulkHtml(m.new_title || "");
      var src = escapeBulkHtml(bulkCheckSourceLabel(m.source));
      var pct = formatBulkSimilarityPct(m.similarity);
      html +=
        "<tr><td class=\"text-nowrap\">" +
        rowNum +
        "</td><td>" +
        oldT +
        "</td><td>" +
        newT +
        "</td><td class=\"text-nowrap\">" +
        src +
        "</td><td class=\"text-nowrap\">" +
        pct +
        "</td></tr>";
    }
    tbody.innerHTML = html;
    var modalEl = qs("#bulk-check-titles-modal");
    if (modalEl && typeof bootstrap !== "undefined") {
      bootstrap.Modal.getOrCreateInstance(modalEl).show();
    }
  }

  function confirmRemoveBulkCheckTitleMatches(onDone) {
    var matches = bulkCheckTitlesLastMatches;
    if (!matches.length) {
      if (typeof onDone === "function") {
        onDone();
      }
      return;
    }
    var indices = [];
    for (var i = 0; i < matches.length; i++) {
      if (matches[i].index != null) {
        indices.push(matches[i].index);
      }
    }
    var n = indices.length;
    var msg =
      "Hapus " +
      n +
      " baris judul baru yang mirip dari daftar? Slug dan kategori pada baris yang sama ikut dihapus.";
    if (!swalAvailable()) {
      if (window.confirm(msg)) {
        removeBulkTitlesByIndices(indices);
        bulkCheckTitlesLastMatches = [];
      }
      if (typeof onDone === "function") {
        onDone();
      }
      return;
    }
    Swal.fire({
      icon: "warning",
      title: "Hapus judul baru?",
      text: msg,
      showCancelButton: true,
      confirmButtonText: "Ya, hapus",
      cancelButtonText: "Batal",
      confirmButtonColor: "#dc3545",
    }).then(function (result) {
      if (result && result.isConfirmed) {
        removeBulkTitlesByIndices(indices);
        bulkCheckTitlesLastMatches = [];
        Swal.fire({
          icon: "success",
          title: "Judul dihapus",
          text: n + " baris dihapus dari list.",
          timer: 2200,
          showConfirmButton: true,
        });
      }
      if (typeof onDone === "function") {
        onDone();
      }
    });
  }

  /** Menulis daftar bersama yang dipakai tombol “Buat artikel” (#bulk-titles — Judul artikel). */
  function fillBulkArticleTitles(text) {
    var ta = qs("#bulk-titles");
    if (!ta) {
      return;
    }
    ta.value = text == null ? "" : String(text);
    syncBulkSlugsAutofillFromTitles();
    try {
      ta.focus({ preventScroll: false });
      ta.scrollIntoView({ behavior: "smooth", block: "center" });
    } catch (e) {
      try {
        ta.focus();
        ta.scrollIntoView(true);
      } catch (e2) {}
    }
  }

  var BULK_WEB_SEARCH_SOURCES = [
    "auto",
    "pexels",
    "unsplash",
    "bing_net",
    "google",
    "bing",
  ];

  function normalizeBulkWebSearchSource(raw) {
    var v = raw != null ? String(raw).toLowerCase().trim() : "";
    for (var i = 0; i < BULK_WEB_SEARCH_SOURCES.length; i++) {
      if (BULK_WEB_SEARCH_SOURCES[i] === v) return v;
    }
    return "auto";
  }

  function currentBulkCoverWebSearchSource() {
    var sel = qs("#bulk-cover-web-search-source");
    return normalizeBulkWebSearchSource(sel && sel.value ? sel.value : "auto");
  }

  function currentBulkCoverMode() {
    var none = qs("#bulk-cover-mode-none");
    if (none && none.checked) return "none";
    var web = qs("#bulk-cover-mode-web-search");
    if (web && web.checked) return "web_search";
    var gen = qs("#bulk-cover-mode-generate");
    if (gen && gen.checked) return "generate_image";
    return "template";
  }

  function syncBulkCoverModeUI() {
    var mode = currentBulkCoverMode();
    var isGenerate = mode === "generate_image";
    var isWebSearch = mode === "web_search";
    var promptWrap = qs("#bulk-image-prompt-wrap");
    var contentWrap = qs("#bulk-image-to-content-wrap");
    var webSearchWrap = qs("#bulk-cover-web-search-wrap");
    var contentToggle = qs("#bulk-image-to-content-enabled");
    if (promptWrap) {
      promptWrap.classList.toggle("d-none", !isGenerate);
    }
    if (contentWrap) {
      contentWrap.classList.toggle("d-none", !isGenerate);
    }
    if (webSearchWrap) {
      webSearchWrap.classList.toggle("d-none", !isWebSearch);
    }
    if (!isGenerate && contentToggle) {
      contentToggle.checked = false;
    }
  }

  function bulkImageToContentEnabled() {
    var on = qs("#bulk-image-to-content-enabled");
    return !!(on && on.checked);
  }

  function getBulkSelectedTagIds() {
    if (tsBulkTags) {
      return tsBulkTags
        .getValue()
        .map(function (v) {
          return parseInt(v, 10);
        })
        .filter(function (n) {
          return !isNaN(n) && n > 0;
        });
    }
    var sel = qs("#bulk-tags");
    if (!sel || !sel.options) {
      return [];
    }
    var out = [];
    for (var i = 0; i < sel.options.length; i++) {
      if (sel.options[i].selected) {
        var id = parseInt(sel.options[i].value, 10);
        if (!isNaN(id) && id > 0) {
          out.push(id);
        }
      }
    }
    return out;
  }

  function tomSelectBulkTagOptions() {
    return {
      plugins: ["remove_button"],
      placeholder: "Cari atau pilih tag…",
      hideSelected: true,
      maxOptions: null,
      create: false,
      render: {
        no_results: function (data, escape) {
          var q = String((data && data.input) || "").trim();
          if (q.length < 1) {
            return '<div class="pf-ts-hint px-3 py-2 small text-secondary">Ketik untuk mencari tag.</div>';
          }
          if (q.length < 3) {
            return '<div class="pf-ts-hint px-3 py-2 small text-secondary">Masukkan minimal 3 karakter untuk menambahkan tag baru dari pencarian.</div>';
          }
          var enc = encodeURIComponent(q);
          return (
            '<div class="pf-ts-no-results p-2 border-top pf-ts-no-results__bar">' +
            '<button type="button" class="pf-ts-no-results-add btn btn-sm btn-outline-primary w-100 d-inline-flex align-items-center justify-content-center gap-2" data-prefill="' +
            enc +
            '">' +
            '<i class="bi bi-plus-lg" aria-hidden="true"></i>' +
            '<span>Tambah <span class="fw-semibold">“' +
            escape(q) +
            "”</span></span>" +
            "</button>" +
            "</div>"
          );
        },
      },
    };
  }

  function slugifyBulkTagName(str) {
    var NS = window.AdminNameSlugCrud;
    if (NS && typeof NS.slugify === "function") {
      return NS.slugify(str);
    }
    return String(str || "")
      .toLowerCase()
      .normalize("NFD")
      .replace(/[\u0300-\u036f]/g, "")
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "");
  }

  /** Lines split by newline; counts rows where trim is (ignores blank / whitespace-only lines). */
  function countBulkNonEmptyLines(text) {
    var s = text == null ? "" : String(text);
    var parts = s.split(/\r?\n/);
    var n = 0;
    for (var i = 0; i < parts.length; i++) {
      if (parts[i].trim()) {
        n++;
      }
    }
    return n;
  }

  function trimBulkTrailingBlankLines(lines) {
    lines = Array.isArray(lines) ? lines.slice() : [];
    while (lines.length > 0 && !String(lines[lines.length - 1] || "").trim()) {
      lines.pop();
    }
    return lines;
  }

  function normalizeBulkCategoryRef(raw) {
    return raw == null ? "" : String(raw).trim().toLowerCase();
  }

  function bulkCategoryLookupFromDropdown() {
    var sel = qs("#bulk-category");
    var lookup = {};
    if (!sel || !sel.options) {
      return lookup;
    }
    for (var i = 0; i < sel.options.length; i++) {
      var opt = sel.options[i];
      if (!opt || !opt.value) {
        continue;
      }
      var id = normalizeBulkCategoryRef(opt.value);
      var name = normalizeBulkCategoryRef(opt.textContent || opt.innerText || "");
      var slug = normalizeBulkCategoryRef(opt.getAttribute("data-slug") || "");
      if (id) lookup[id] = true;
      if (name) lookup[name] = true;
      if (slug) lookup[slug] = true;
    }
    return lookup;
  }

  function validateBulkCategoryRefs(categoryRefs) {
    if (!categoryRefs || !categoryRefs.length) {
      return null;
    }
    var lookup = bulkCategoryLookupFromDropdown();
    for (var i = 0; i < categoryRefs.length; i++) {
      var raw = String(categoryRefs[i] || "").trim();
      if (!lookup[normalizeBulkCategoryRef(raw)]) {
        return (
          "Baris " +
          (i + 1) +
          ': kategori "' +
          raw +
          '" tidak ditemukan. Gunakan ID, slug, atau nama kategori yang sudah ada.'
        );
      }
    }
    return null;
  }

  function refreshBulkTitleSlugLineCounts() {
    var tEl = qs("#bulk-titles-line-count");
    var sEl = qs("#bulk-slugs-line-count");
    var cEl = qs("#bulk-category-refs-line-count");
    var ta = qs("#bulk-titles");
    var sa = qs("#bulk-slugs");
    var ca = qs("#bulk-category-refs");
    var tc = ta ? countBulkNonEmptyLines(ta.value) : 0;
    var sc = sa ? countBulkNonEmptyLines(sa.value) : 0;
    var cc = ca ? countBulkNonEmptyLines(ca.value) : 0;
    if (tEl) {
      tEl.textContent = tc === 1 ? "1 baris" : tc + " baris";
    }
    if (sEl) {
      sEl.textContent = sc === 1 ? "1 baris" : sc + " baris";
    }
    if (cEl) {
      cEl.textContent = cc === 1 ? "1 baris" : cc + " baris";
    }
  }

  function syncBulkCategoryInputUI() {
    var catTa = qs("#bulk-category-refs");
    var wrap = qs("#bulk-category-wrap");
    var hasPerTitleCategories =
      catTa && countBulkNonEmptyLines(catTa.value) > 0;
    if (wrap) {
      wrap.classList.toggle("d-none", !!hasPerTitleCategories);
    }
  }

  /**
   * Keeps #bulk-slugs in sync while typing in #bulk-titles:
   * - empty slug line → slugify(title)
   * - slug equals slugify(previous title on that line) → update to slugify(new title)
   * - custom slug (anything else) → unchanged
   * - title line cleared → clear slug on that line if present (avoids slug-without-title)
   */
  function syncBulkSlugsAutofillFromTitles() {
    var titleTa = qs("#bulk-titles");
    var slugTa = qs("#bulk-slugs");
    if (!titleTa || !slugTa) {
      return;
    }
    var tLines = String(titleTa.value).split(/\r?\n/);
    var sLines = String(slugTa.value).split(/\r?\n/);
    while (sLines.length < tLines.length) {
      sLines.push("");
    }
    if (sLines.length > tLines.length) {
      sLines.length = tLines.length;
    }
    var prev = bulkLastTitleLinesForSlugSync;
    if (!prev) {
      prev = [];
    }
    var changed = false;
    for (var i = 0; i < tLines.length; i++) {
      var ti = tLines[i].trim();
      var raw = sLines[i] == null ? "" : String(sLines[i]);
      var si = raw.trim();
      var prevLine = i < prev.length ? String(prev[i] || "") : "";
      var prevT = prevLine.trim();
      var prevAuto = prevT ? slugifyBulkTagName(prevT) : "";
      if (!ti) {
        if (si) {
          sLines[i] = "";
          changed = true;
        }
        continue;
      }
      var gen = slugifyBulkTagName(ti);
      if (gen.length > 220) {
        gen = gen.slice(0, 220);
      }
      if (!si) {
        sLines[i] = gen;
        changed = true;
      } else if (prevAuto && si === prevAuto && si !== gen) {
        sLines[i] = gen;
        changed = true;
      }
    }
    if (changed) {
      slugTa.value = sLines.join("\n");
    }
    bulkLastTitleLinesForSlugSync = tLines.slice();
    refreshBulkTitleSlugLineCounts();
  }

  /** Replace titles and overwrite all slug lines from the new titles (regenerate flow). */
  function applyRegeneratedTitlesAndSlugs(titlesText) {
    var ta = qs("#bulk-titles");
    var slugTa = qs("#bulk-slugs");
    if (!ta) {
      return;
    }
    var raw = titlesText == null ? "" : String(titlesText);
    ta.value = raw;
    if (!slugTa) {
      refreshBulkTitleSlugLineCounts();
      try {
        ta.focus({ preventScroll: false });
        ta.scrollIntoView({ behavior: "smooth", block: "center" });
      } catch (fe) {
        try {
          ta.focus();
          ta.scrollIntoView(true);
        } catch (fe2) {}
      }
      return;
    }
    var tLines = raw.split(/\r?\n/);
    var out = [];
    for (var i = 0; i < tLines.length; i++) {
      var ti = tLines[i].trim();
      if (!ti) {
        out.push("");
        continue;
      }
      var g = slugifyBulkTagName(ti);
      if (g.length > 220) {
        g = g.slice(0, 220);
      }
      out.push(g);
    }
    slugTa.value = out.join("\n");
    bulkLastTitleLinesForSlugSync = tLines.slice();
    refreshBulkTitleSlugLineCounts();
    try {
      ta.focus({ preventScroll: false });
      ta.scrollIntoView({ behavior: "smooth", block: "center" });
    } catch (e) {
      try {
        ta.focus();
        ta.scrollIntoView(true);
      } catch (e2) {}
    }
  }

  function fillBulkArticleTitles(titlesText) {
    applyRegeneratedTitlesAndSlugs(titlesText);
  }

  function wireBulkTagModalSlug() {
    var ne = qs("#bulk-tag-modal-name");
    var se = qs("#bulk-tag-modal-slug");
    var NS = window.AdminNameSlugCrud;
    if (!ne || !se || !NS || typeof NS.slugify !== "function") {
      return { reset: function () {} };
    }
    var slugTouched = false;
    function onName() {
      if (slugTouched) return;
      se.value = NS.slugify(ne.value);
    }
    function onSlug() {
      slugTouched = true;
    }
    ne.addEventListener("input", onName);
    se.addEventListener("input", onSlug);
    return {
      reset: function () {
        slugTouched = false;
      },
    };
  }

  function setBulkTagModalBusy(busy) {
    var btn = qs("#bulk-tag-modal-submit");
    var spin = qs("#bulk-tag-modal-spinner");
    var icon = qs("#bulk-tag-modal-check");
    if (!btn) return;
    btn.disabled = !!busy;
    if (spin) spin.classList.toggle("d-none", !busy);
    if (icon) icon.classList.toggle("d-none", !!busy);
  }

  function openBulkNewTagModal(prefillName) {
    var modalRoot = qs("#bulk-tag-modal");
    if (!modalRoot) return;
    AC.setInlineError(qs("#bulk-tag-modal-error"), null);
    var ne = qs("#bulk-tag-modal-name");
    var se = qs("#bulk-tag-modal-slug");
    var raw = String(prefillName || "").trim();
    bulkTagModalSlugAutofill.reset();
    if (ne) ne.value = raw;
    if (se) {
      se.value = raw.length >= 3 ? slugifyBulkTagName(raw) : "";
    }
    if (tsBulkTags) {
      try {
        if (typeof tsBulkTags.close === "function") {
          tsBulkTags.close();
        }
      } catch (err) {}
    }
    bootstrap.Modal.getOrCreateInstance(modalRoot).show();
    window.setTimeout(function () {
      if (ne) {
        ne.focus();
        if (raw) ne.select();
      }
    }, 320);
  }

  function swalAvailable() {
    return typeof Swal !== "undefined";
  }

  function escapeHtml(s) {
    if (s == null) {
      return "";
    }
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function warnModal(title, text) {
    if (swalAvailable()) {
      return Swal.fire({
        icon: "warning",
        title: title,
        text: text,
        confirmButtonText: "OK",
      });
    }
    return Promise.resolve();
  }

  function errorModal(title, text) {
    if (swalAvailable()) {
      return Swal.fire({
        icon: "error",
        title: title,
        text: text,
        confirmButtonText: "OK",
      });
    }
    return Promise.resolve();
  }

  function confirmRegenerateTitles(totalLines, nonEmptyCount) {
    var msg =
      "AI akan menulis ulang judul artikel (tetap " +
      totalLines +
      " baris, termasuk baris kosong) dan mengganti semua slug dari judul baru. Saat ini ada " +
      nonEmptyCount +
      " baris judul yang terisi. Judul dan slug saat ini akan ditimpa.";
    if (swalAvailable()) {
      return Swal.fire({
        icon: "warning",
        title: "Buat ulang judul?",
        text: msg,
        showCancelButton: true,
        confirmButtonText: "Buat ulang",
        cancelButtonText: "Batal",
        reverseButtons: true,
        focusCancel: true,
      }).then(function (res) {
        return !!(res && res.isConfirmed);
      });
    }
    return Promise.resolve(
      window.confirm(
        "Buat ulang judul? Semua slug akan dibuat ulang dari judul baru. Lanjutkan?"
      )
    );
  }

  function bulkGenLoadingHtml() {
    return (
      '<div class="text-start">' +
      '<p class="small text-secondary mb-2 mb-md-3" id="bulk-swal-gen-msg">…</p>' +
      '<div class="progress" style="height:1.35rem" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="0" id="bulk-swal-gen-progress-root">' +
      '<div id="bulk-swal-gen-bar" class="progress-bar progress-bar-striped progress-bar-animated bg-primary" style="width:0%">0%</div>' +
      "</div></div>"
    );
  }

  function bulkGenFooterHtml() {
    return '<button type="button" class="swal2-styled swal2-deny" id="bulk-swal-run-bg-btn">Jalankan di latar belakang</button>';
  }

  function openBulkGenLoadingModal(n) {
    return new Promise(function (resolve) {
      if (!swalAvailable()) {
        resolve({ isBackground: false });
        return;
      }
      Swal.fire({
        title: "Buat artikel",
        html: bulkGenLoadingHtml(),
        footer: bulkGenFooterHtml(),
        allowOutsideClick: false,
        allowEscapeKey: false,
        showConfirmButton: false,
        width: "28rem",
        didOpen: function () {
          var state = { isBackground: false };
          var bgBtn = Swal.getFooter()
            ? Swal.getFooter().querySelector("#bulk-swal-run-bg-btn")
            : null;
          if (bgBtn) {
            bgBtn.addEventListener("click", function () {
              state.isBackground = true;
              closeSwal();
              if (typeof AC.showSuccessToast === "function") {
                AC.showSuccessToast(
                  "Pembuatan bulk berjalan di latar belakang. Progress tetap berjalan di tab ini."
                );
              }
            });
          }
          // Avoid a "stuck at 0%" look (especially when total=1 where it jumps to 100% at the end).
          updateBulkGenLoadingModal(1, "Memulai (0 / " + n + ")…");
          resolve(state);
        },
      });
    });
  }

  function updateBulkGenLoadingModal(pct, msg) {
    if (!swalAvailable() || !Swal.isVisible()) {
      return;
    }
    var root = Swal.getHtmlContainer();
    if (!root) {
      return;
    }
    var p = Math.max(0, Math.min(100, Math.round(pct)));
    var bar = root.querySelector("#bulk-swal-gen-bar");
    var label = root.querySelector("#bulk-swal-gen-msg");
    var prog = root.querySelector("#bulk-swal-gen-progress-root");
    if (bar) {
      bar.style.width = p + "%";
      bar.textContent = p + "%";
    }
    if (label) {
      label.textContent = msg || "";
    }
    if (prog) {
      prog.setAttribute("aria-valuenow", String(p));
    }
  }

  function closeSwal() {
    if (swalAvailable() && Swal.isVisible()) {
      Swal.close();
    }
  }

  function setBulkForceStopVisible(visible) {
    var btn = qs("#bulk-force-stop-btn");
    if (!btn) return;
    btn.disabled = !!activeBulkGenerateStopping;
  }

  function setBulkForceStopBusy(busy) {
    activeBulkGenerateStopping = !!busy;
    var btn = qs("#bulk-force-stop-btn");
    var sp = qs("#bulk-force-stop-spinner");
    var ic = qs("#bulk-force-stop-icon");
    if (btn) {
      btn.disabled = !!busy;
      btn.setAttribute("aria-busy", busy ? "true" : "false");
    }
    if (sp) sp.classList.toggle("d-none", !busy);
    if (ic) ic.classList.toggle("d-none", !!busy);
  }

  function attachActiveBulkGenerateJob(job) {
    if (!job || !job.job_id) {
      clearActiveBulkGenerateJob();
      return false;
    }
    var st = String(job.status || "").toLowerCase();
    if (st !== "queued" && st !== "running") {
      clearActiveBulkGenerateJob();
      return false;
    }
    activeBulkGenerateJobId = String(job.job_id);
    activeBulkGenerateCanceled = false;
    setBulkForceStopBusy(false);
    setBulkForceStopVisible(true);
    return true;
  }

  function clearActiveBulkGenerateJob() {
    activeBulkGenerateJobId = "";
    activeBulkGenerateCanceled = false;
    activeBulkGenerateStopping = false;
    setBulkForceStopBusy(false);
    setBulkForceStopVisible(false);
  }

  async function refreshActiveBulkGenerateJobStopButton() {
    try {
      var res = await fetchAuth(API_ACTIVE_GEN_JOB, { method: "GET" });
      var data = null;
      try {
        data = await res.json();
      } catch (e) {
        data = null;
      }
      if (!res.ok || !data || !data.active) {
        clearActiveBulkGenerateJob();
        return;
      }
      attachActiveBulkGenerateJob(data);
    } catch (e2) {
      if (e2 && e2.message === "unauth") return;
    }
  }

  async function requestCancelActiveBulkGenerateJob(errEl, okEl) {
    if (!activeBulkGenerateJobId) {
      warnModal(
        "Tidak ada bulk generation berjalan",
        "Tidak ada bulk generation aktif yang bisa dihentikan."
      );
      return;
    }
    if (activeBulkGenerateStopping) {
      return;
    }
    var jobId = activeBulkGenerateJobId;
    setBulkForceStopBusy(true);
    try {
      var res = await fetchAuth(
        API_GEN_JOB + encodeURIComponent(jobId) + "/cancel",
        { method: "POST" }
      );
      var data = null;
      try {
        data = await res.json();
      } catch (e) {
        data = null;
      }
      if (!res.ok) {
        var msg = (data && data.error) || "Gagal menghentikan bulk generation.";
        setErr(errEl, msg);
        errorModal("Gagal stop", msg);
        setBulkForceStopBusy(false);
        return;
      }
      activeBulkGenerateCanceled = true;
      activeBulkGenerateJobId = "";
      activeBulkGenerateStopping = false;
      setBulkForceStopVisible(true);
      setBulkForceStopBusy(false);
      setOk(okEl, "Permintaan stop bulk generation dikirim. Menunggu worker berhenti.");
      if (typeof AC.showSuccessToast === "function") {
        AC.showSuccessToast("Permintaan stop untuk bulk generation berjalan sudah dikirim.");
      }
      updateBulkGenLoadingModal(99, "Permintaan stop dikirim. Menunggu step saat ini selesai…");
    } catch (e2) {
      if (e2 && e2.message === "unauth") return;
      setErr(errEl, "Error jaringan saat menghentikan bulk generation.");
      errorModal("Error jaringan", "Tidak bisa menghentikan bulk generation yang sedang berjalan.");
      setBulkForceStopBusy(false);
    }
  }

  function showBulkGenResultModal(ok, bad, allResults, stopped) {
    if (!swalAvailable()) {
      return Promise.resolve();
    }
    var icon = stopped ? "warning" : bad === 0 ? "success" : ok === 0 ? "error" : "warning";
    var title = stopped
      ? "Pembuatan bulk dihentikan"
      : bad === 0
        ? "Semua artikel berhasil dibuat"
        : ok === 0
        ? "Generate gagal"
        : "Selesai dengan beberapa masalah";
    var parts = [];
    if (stopped) {
      parts.push(
        '<p class="mb-2">Pembuatan bulk yang berjalan sudah dihentikan. Request AI yang dibatalkan tidak dihitung sebagai artikel gagal.</p>'
      );
    }
    parts.push(
      '<p class="mb-2">' +
        "<strong>" +
        ok +
        "</strong> berhasil, <strong>" +
        bad +
        "</strong> gagal.</p>"
    );
    var fails = [];
    for (var i = 0; i < (allResults || []).length; i++) {
      var r = allResults[i];
      if (r && r.error) {
        fails.push({ t: r.title || "—", e: r.error });
      }
    }
    if (fails.length) {
      parts.push('<p class="small text-secondary mb-1">Item gagal:</p>');
      parts.push(
        '<ul class="list-unstyled small text-start mb-0 bulk-swal-fail-list">'
      );
      var max = 12;
      for (var j = 0; j < fails.length && j < max; j++) {
        parts.push(
          '<li class="mb-1"><span class="text-break">' +
            escapeHtml(fails[j].t) +
            '</span><br><span class="text-danger">' +
            escapeHtml(fails[j].e) +
            "</span></li>"
        );
      }
      if (fails.length > max) {
        parts.push(
          '<li class="text-secondary">… and ' +
            (fails.length - max) +
            " lainnya</li>"
        );
      }
      parts.push("</ul>");
    }
    return Swal.fire({
      icon: icon,
      title: title,
      html: parts.join(""),
      confirmButtonText: "OK",
      width: "34rem",
    });
  }

  function openSimpleLoadingModal(title, subtitle) {
    if (!swalAvailable()) {
      return;
    }
    Swal.fire({
      title: title,
      text: subtitle || "Mohon tunggu…",
      allowOutsideClick: false,
      allowEscapeKey: false,
      showConfirmButton: false,
      didOpen: function () {
        Swal.showLoading();
      },
    });
  }

  function setXmlScrapeBusy(busy) {
    var btn = qs("#bulk-xml-scrape-btn");
    var spin = qs("#bulk-xml-scrape-spinner");
    var icon = qs("#bulk-xml-scrape-icon");
    if (!btn) {
      return;
    }
    btn.disabled = !!busy;
    if (spin) {
      spin.classList.toggle("d-none", !busy);
    }
    if (icon) {
      icon.classList.toggle("d-none", !!busy);
    }
    var sg = qs("#bulk-suggest-btn");
    var rg = qs("#bulk-regen-titles-btn");
    var ck = qs("#bulk-check-titles-btn");
    if (sg) {
      sg.disabled = !!busy;
    }
    if (rg) {
      rg.disabled = !!busy;
    }
    if (ck) {
      ck.disabled = !!busy;
    }
  }

  function initBulkTagsTomSelect() {
    var tagSel = qs("#bulk-tags");
    if (!tagSel) {
      return false;
    }
    if (tsBulkTags) {
      return true;
    }
    if (typeof TomSelect === "undefined") {
      return false;
    }
    try {
      tsBulkTags = new TomSelect(tagSel, tomSelectBulkTagOptions());
      return true;
    } catch (err) {
      tsBulkTags = null;
      return false;
    }
  }

  function showBulkTagsTomSelectMissing() {
    var help = qs("#bulk-tags-help");
    if (!help || qs("#bulk-tags-ts-fallback-msg")) {
      return;
    }
    var d = document.createElement("div");
    d.id = "bulk-tags-ts-fallback-msg";
    d.className = "alert alert-warning py-2 small mb-2";
    d.setAttribute("role", "status");
    d.innerHTML =
      "UI pencarian tag gagal dimuat. Upload folder <code>public/static/vendor/tom-select/</code> saat deploy dan pastikan <code>/static/vendor/tom-select/js/tom-select.complete.min.js</code> mengembalikan 200 (bukan 404).";
    help.parentNode.insertBefore(d, help);
  }

  function loadTomSelectScriptThen(done) {
    done = typeof done === "function" ? done : function () {};
    if (typeof TomSelect !== "undefined") {
      done();
      return;
    }
    if (tsBulkDynamicLoadStarted) {
      done();
      return;
    }
    tsBulkDynamicLoadStarted = true;
    var root = qs("#admin-bulk-post-root");
    var src =
      (root && root.getAttribute("data-tom-select-js")) ||
      "/static/vendor/tom-select/js/tom-select.complete.min.js";
    var dup = document.querySelector("script[data-bebii-ts-injected='1']");
    if (dup) {
      dup.addEventListener("load", done);
      dup.addEventListener("error", function () {
        showBulkTagsTomSelectMissing();
        done();
      });
      return;
    }
    var s = document.createElement("script");
    s.src = src;
    s.async = false;
    s.setAttribute("data-bebii-ts-injected", "1");
    s.onload = function () {
      done();
    };
    s.onerror = function () {
      showBulkTagsTomSelectMissing();
      done();
    };
    (document.head || document.documentElement).appendChild(s);
  }

  function bootBulkTagsTomSelect() {
    if (initBulkTagsTomSelect()) {
      return;
    }
    if (typeof TomSelect !== "undefined") {
      showBulkTagsTomSelectMissing();
      return;
    }
    loadTomSelectScriptThen(function () {
      if (!initBulkTagsTomSelect()) {
        showBulkTagsTomSelectMissing();
      }
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    AC = window.AdminCrud;
    if (!AC) {
      return;
    }

    if (!getToken()) {
      window.location.href = LOGIN;
      return;
    }

    var bulkRoot = qs("#admin-bulk-post-root");
    bootBulkTagsTomSelect();
    window.requestAnimationFrame(function () {
      if (!tsBulkTags && qs("#bulk-tags")) {
        bootBulkTagsTomSelect();
      }
    });
    window.addEventListener("load", function bulkTsRetryOnLoad() {
      window.removeEventListener("load", bulkTsRetryOnLoad);
      if (!tsBulkTags && qs("#bulk-tags")) {
        bootBulkTagsTomSelect();
      }
    });

    if (qs("#bulk-tag-modal-name") && qs("#bulk-tag-modal-slug")) {
      bulkTagModalSlugAutofill = wireBulkTagModalSlug();
    }

    document.addEventListener(
      "click",
      function (e) {
        var btn =
          e.target && e.target.closest
            ? e.target.closest(".pf-ts-no-results-add")
            : null;
        if (!btn || !bulkRoot || !bulkRoot.contains(btn)) {
          return;
        }
        var enc = btn.getAttribute("data-prefill");
        if (!enc) return;
        var name = "";
        try {
          name = decodeURIComponent(enc);
        } catch (err) {
          name = "";
        }
        e.preventDefault();
        e.stopPropagation();
        openBulkNewTagModal(name);
      },
      true
    );

    var bulkTagModalForm = qs("#bulk-tag-modal-form");
    var bulkTagModalEl = qs("#bulk-tag-modal");
    if (bulkTagModalForm) {
      bulkTagModalForm.addEventListener("submit", async function (e) {
        e.preventDefault();
        var errBox = qs("#bulk-tag-modal-error");
        AC.setInlineError(errBox, null);
        var name = qs("#bulk-tag-modal-name")
          ? String(qs("#bulk-tag-modal-name").value || "").trim()
          : "";
        var slug = qs("#bulk-tag-modal-slug")
          ? String(qs("#bulk-tag-modal-slug").value || "").trim()
          : "";
        if (name.length < 3) {
          AC.setInlineError(errBox, "Nama minimal 3 karakter.");
          return;
        }
        if (slug.length < 3 || !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug)) {
          AC.setInlineError(
            errBox,
            "Slug: gunakan huruf kecil, angka, dan tanda hubung saja (minimal 3 karakter)."
          );
          return;
        }
        if (!tsBulkTags) {
          AC.setInlineError(
            errBox,
            "Tag masih dimuat. Tunggu sebentar lalu coba lagi."
          );
          return;
        }
        setBulkTagModalBusy(true);
        try {
          var res = await fetchAuth(API_TAGS, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ name: name, slug: slug }),
          });
          var data = {};
          try {
            data = await res.json();
          } catch (ex) {}
          if (!res.ok) {
            AC.setInlineError(
              errBox,
              (data && data.error) || "Gagal membuat tag."
            );
            return;
          }
          var tid = data && data.id != null ? data.id : null;
          var tname = data && data.name ? String(data.name) : name;
          if (tid == null) {
            AC.setInlineError(errBox, "Response server tidak valid.");
            return;
          }
          var sid = String(tid);
          tsBulkTags.addOption({ value: sid, text: tname });
          tsBulkTags.addItem(sid);
          if (bulkTagModalEl) {
            bootstrap.Modal.getOrCreateInstance(bulkTagModalEl).hide();
          }
          if (typeof AC.showSuccessToast === "function") {
            AC.showSuccessToast("Tag dibuat dan dipilih.");
          }
        } catch (err) {
          if (err && err.message === "unauth") return;
          AC.setInlineError(errBox, "Error jaringan.");
        } finally {
          setBulkTagModalBusy(false);
        }
      });
    }

    var errEl = qs("#bulk-post-error");
    var okEl = qs("#bulk-post-info");
    var titleSourceSel = qs("#bulk-title-source-mode");
    var suggestBtn = qs("#bulk-suggest-btn");
    var scrapeBtn = qs("#bulk-xml-scrape-btn");
    var regenBtn = qs("#bulk-regen-titles-btn");
    var checkTitlesBtn = qs("#bulk-check-titles-btn");
    var checkTitlesRemoveBtn = qs("#bulk-check-titles-remove-btn");
    var genBtn = qs("#bulk-generate-btn");
    var forceStopBtn = qs("#bulk-force-stop-btn");
    if (forceStopBtn) {
      forceStopBtn.addEventListener("click", function () {
        requestCancelActiveBulkGenerateJob(errEl, okEl);
      });
    }
    refreshActiveBulkGenerateJobStopButton();

    var bulkTitlesInput = qs("#bulk-titles");
    var bulkSlugsInput = qs("#bulk-slugs");
    var bulkCategoryRefsInput = qs("#bulk-category-refs");
    function onBulkTitlesSlugsInput() {
      refreshBulkTitleSlugLineCounts();
      syncBulkCategoryInputUI();
    }
    function syncBulkAdaptiveDelayNotice() {
      var noticeEl = qs("#bulk-adaptive-delay-notice");
      var textEl = qs("#bulk-adaptive-delay-text");
      if (!noticeEl || !textEl || !bulkTitlesInput) return;

      var raw = String(bulkTitlesInput.value || "");
      var lines = raw.split(/\r?\n/);
      var count = 0;
      for (var i = 0; i < lines.length; i++) {
        if (lines[i].trim()) count++;
      }

      if (count <= 20) {
        noticeEl.classList.add("d-none");
        return;
      }

      var msg = "";
      if (count > 80) {
        msg = count + " judul terdeteksi. Jeda bertahap diterapkan secara otomatis (s/d +4 menit pada artikel ke-80+) untuk menjaga kestabilan AI & mencegah rate limit.";
      } else if (count > 60) {
        msg = count + " judul terdeteksi. Jeda bertahap diterapkan secara otomatis (s/d +3 menit pada artikel ke-60+) untuk menjaga kestabilan AI & mencegah rate limit.";
      } else if (count > 40) {
        msg = count + " judul terdeteksi. Jeda bertahap diterapkan secara otomatis (+1m ke-20+, +2m ke-40+) agar pembuatan konten tetap stabil.";
      } else {
        msg = count + " judul terdeteksi. Jeda bertahap diterapkan secara otomatis (+1m mulai artikel ke-21) untuk menjaga kestabilan AI.";
      }

      textEl.textContent = msg;
      noticeEl.classList.remove("d-none");
    }

    function onBulkTitlesTyping() {
      syncBulkSlugsAutofillFromTitles();
      syncBulkAdaptiveDelayNotice();
    }
    if (bulkTitlesInput) {
      bulkTitlesInput.addEventListener("input", onBulkTitlesTyping);
      bulkTitlesInput.addEventListener("change", onBulkTitlesTyping);
      bulkTitlesInput.addEventListener("compositionend", onBulkTitlesTyping);
    }
    if (bulkSlugsInput) {
      bulkSlugsInput.addEventListener("input", onBulkTitlesSlugsInput);
      bulkSlugsInput.addEventListener("change", onBulkTitlesSlugsInput);
    }
    if (bulkCategoryRefsInput) {
      bulkCategoryRefsInput.addEventListener("input", onBulkTitlesSlugsInput);
      bulkCategoryRefsInput.addEventListener("change", onBulkTitlesSlugsInput);
    }
    syncBulkSlugsAutofillFromTitles();
    syncBulkAdaptiveDelayNotice();
    syncBulkCategoryInputUI();

    var googleTrendsBtn = qs("#bulk-google-trends-btn");

    function setGoogleTrendsBusy(busy) {
      var btn = qs("#bulk-google-trends-btn");
      var sp = qs("#bulk-google-trends-spinner");
      var ic = qs("#bulk-google-trends-icon");
      if (btn) {
        btn.disabled = !!busy;
        btn.setAttribute("aria-busy", busy ? "true" : "false");
      }
      if (sp) sp.classList.toggle("d-none", !busy);
      if (ic) ic.classList.toggle("d-none", !!busy);
    }

    function syncBulkTitleSourcePanels() {
      var panelAI = qs("#bulk-title-panel-ai");
      var panelScrape = qs("#bulk-title-panel-scrape");
      var panelTrends = qs("#bulk-title-panel-google-trends");
      if (!titleSourceSel) {
        return;
      }
      var v = String(titleSourceSel.value || "").trim();
      var showAI = v === "ai";
      var showScrape = v === "scrape";
      var showTrends = v === "google_trends";
      if (panelAI) panelAI.classList.toggle("d-none", !showAI);
      if (panelScrape) panelScrape.classList.toggle("d-none", !showScrape);
      if (panelTrends) panelTrends.classList.toggle("d-none", !showTrends);
    }
    if (titleSourceSel) {
      titleSourceSel.addEventListener("change", syncBulkTitleSourcePanels);
      syncBulkTitleSourcePanels();
    }

    function syncBulkScheduleWrap() {
      var sel = qs("#bulk-status");
      var wrap = qs("#bulk-schedule-wrap");
      if (!wrap) return;
      var st = sel ? String(sel.value || "") : "";
      wrap.classList.toggle("d-none", st !== "scheduled");
      syncBulkCoverModeNoneVisibility(st);
    }

    function syncBulkCoverModeNoneVisibility(status) {
      var st = status || ((qs("#bulk-status") && qs("#bulk-status").value) || "draft");
      var isDraft = st === "draft";
      var noneInput = qs("#bulk-cover-mode-none");
      var noneLabel = qs('label[for="bulk-cover-mode-none"]');
      if (noneInput) {
        noneInput.style.display = isDraft ? "" : "none";
        if (!isDraft && noneInput.checked) {
          noneInput.checked = false;
          var tpl = qs("#bulk-cover-mode-template");
          if (tpl) tpl.checked = true;
          syncBulkCoverModeUI();
        }
      }
      if (noneLabel) {
        noneLabel.style.display = isDraft ? "" : "none";
      }
    }
    var bulkStatusSel = qs("#bulk-status");
    if (bulkStatusSel) {
      bulkStatusSel.addEventListener("change", syncBulkScheduleWrap);
      syncBulkScheduleWrap();
    }
    var coverModeTemplate = qs("#bulk-cover-mode-template");
    var coverModeWebSearch = qs("#bulk-cover-mode-web-search");
    var coverModeGenerate = qs("#bulk-cover-mode-generate");
    var coverModeNone = qs("#bulk-cover-mode-none");
    if (coverModeTemplate) {
      coverModeTemplate.addEventListener("change", syncBulkCoverModeUI);
    }
    if (coverModeWebSearch) {
      coverModeWebSearch.addEventListener("change", syncBulkCoverModeUI);
    }
    if (coverModeGenerate) {
      coverModeGenerate.addEventListener("change", syncBulkCoverModeUI);
    }
    if (coverModeNone) {
      coverModeNone.addEventListener("change", syncBulkCoverModeUI);
    }
    syncBulkCoverModeUI();

    if (checkTitlesRemoveBtn) {
      checkTitlesRemoveBtn.addEventListener("click", function () {
        var modalEl = qs("#bulk-check-titles-modal");
        confirmRemoveBulkCheckTitleMatches(function () {
          if (modalEl && typeof bootstrap !== "undefined") {
            var inst = bootstrap.Modal.getInstance(modalEl);
            if (inst) {
              inst.hide();
            }
          }
        });
      });
    }

    if (checkTitlesBtn) {
      checkTitlesBtn.addEventListener("click", function () {
        setErr(errEl, null);
        var titleTa = qs("#bulk-titles");
        var titlesText = titleTa ? titleTa.value : "";
        var pair = pairTitlesAndSlugsFromBulkUI();
        if (pair.err) {
          setErr(errEl, pair.err);
          warnModal("Periksa input", pair.err);
          return;
        }
        checkTitlesBtn.disabled = true;
        if (regenBtn) {
          regenBtn.disabled = true;
        }
        if (suggestBtn) {
          suggestBtn.disabled = true;
        }
        if (scrapeBtn) {
          scrapeBtn.disabled = true;
        }
        openSimpleLoadingModal("Cek list judul", "Membandingkan dengan database…");
        fetchAuth(API_CHECK_TITLES, {
          method: "POST",
          body: JSON.stringify({ titles_text: titlesText }),
        })
          .then(function (res) {
            return res.json().then(function (data) {
              checkTitlesBtn.disabled = false;
              if (regenBtn) {
                regenBtn.disabled = false;
              }
              if (suggestBtn) {
                suggestBtn.disabled = false;
              }
              if (scrapeBtn) {
                scrapeBtn.disabled = false;
              }
              closeSwal();
              if (!res.ok) {
                var em = (data && data.error) || "Gagal mengecek judul.";
                setErr(errEl, em);
                errorModal("Gagal cek judul", em);
                return;
              }
              var count = data && data.count != null ? data.count : 0;
              if (!count) {
                bulkCheckTitlesLastMatches = [];
                if (swalAvailable()) {
                  Swal.fire({
                    icon: "success",
                    title: "Tidak ada judul mirip",
                    text:
                      "Semua baris di bawah ambang " +
                      Math.round((data.threshold || 0.9) * 100) +
                      "% kemiripan.",
                    timer: 2600,
                    showConfirmButton: true,
                  });
                } else {
                  setOk(
                    okEl,
                    "Tidak ada judul mirip (≥" +
                      Math.round((data.threshold || 0.9) * 100) +
                      "%)."
                  );
                }
                return;
              }
              renderBulkCheckTitlesModal(data);
            });
          })
          .catch(function (e) {
            checkTitlesBtn.disabled = false;
            if (regenBtn) {
              regenBtn.disabled = false;
            }
            if (suggestBtn) {
              suggestBtn.disabled = false;
            }
            if (scrapeBtn) {
              scrapeBtn.disabled = false;
            }
            closeSwal();
            if (e && e.message === "unauth") {
              return;
            }
            setErr(errEl, "Error jaringan.");
            errorModal("Error jaringan", "Tidak bisa terhubung ke server.");
          });
      });
    }

    if (suggestBtn) {
      suggestBtn.addEventListener("click", function () {
        setErr(errEl, null);
        var kwEl = qs("#bulk-keyword");
        var kw = kwEl ? kwEl.value.trim() : "";
        var count = parseInt(
          (qs("#bulk-count") && qs("#bulk-count").value) || "5",
          10
        );
        var prov =
          (qs("#bulk-provider") && qs("#bulk-provider").value) || "gemini";
        if (!kw || kw.length < 2) {
          var m1 = "Masukkan keyword (minimal 2 karakter).";
          setErr(errEl, m1);
          warnModal("Periksa input", m1);
          return;
        }
        if (isNaN(count) || count < 1 || count > 50) {
          var m2 = "Jumlah harus antara 1 sampai 50.";
          setErr(errEl, m2);
          warnModal("Periksa input", m2);
          return;
        }
        suggestBtn.disabled = true;
        if (regenBtn) {
          regenBtn.disabled = true;
        }
        if (scrapeBtn) {
          scrapeBtn.disabled = true;
        }
        openSimpleLoadingModal("Membuat judul", "Memanggil AI…");
        fetchAuth(API_SUGGEST, {
          method: "POST",
          body: JSON.stringify({
            keyword: kw,
            count: count,
            provider: prov,
          }),
        })
          .then(function (res) {
            return res.json().then(function (data) {
              suggestBtn.disabled = false;
              if (regenBtn) {
                regenBtn.disabled = false;
              }
              if (scrapeBtn) {
                scrapeBtn.disabled = false;
              }
              closeSwal();
              if (!res.ok) {
                var em = (data && data.error) || "Gagal membuat saran judul.";
                setErr(errEl, em);
                if (
                  window.AdminAILimit &&
                  typeof AdminAILimit.showFromResponse === "function" &&
                  AdminAILimit.showFromResponse(data)
                ) {
                  return;
                }
                errorModal("Gagal membuat judul", em);
                return;
              }
              if (data.titles_text != null) {
                fillBulkArticleTitles(data.titles_text);
              }
              if (swalAvailable()) {
                Swal.fire({
                  icon: "success",
                  title: "Judul siap",
                  text: "Disimpan ke Judul artikel — daftar yang sama dipakai untuk Buat artikel.",
                  timer: 2400,
                  showConfirmButton: true,
                });
              }
            });
          })
          .catch(function (e) {
            suggestBtn.disabled = false;
            if (regenBtn) {
              regenBtn.disabled = false;
            }
            if (scrapeBtn) {
              scrapeBtn.disabled = false;
            }
            closeSwal();
            if (e && e.message === "unauth") {
              return;
            }
            setErr(errEl, "Error jaringan.");
            errorModal("Error jaringan", "Tidak bisa terhubung ke server.");
          });
      });
    }

    if (scrapeBtn) {
      scrapeBtn.addEventListener("click", function () {
        setErr(errEl, null);
        var urlEl = qs("#bulk-xml-url");
        var urlVal = urlEl ? String(urlEl.value || "").trim() : "";
        if (!urlVal) {
          var ms = "Masukkan URL feed.";
          setErr(errEl, ms);
          warnModal("Periksa input", ms);
          return;
        }
        var body = { url: urlVal, xml: "" };
        setXmlScrapeBusy(true);
        openSimpleLoadingModal(
          "Mengambil judul",
          "Mengunduh dan membaca XML…"
        );
        fetchAuth(API_SCRAPE_XML, {
          method: "POST",
          body: JSON.stringify(body),
        })
          .then(function (res) {
            return res.json().then(function (data) {
              setXmlScrapeBusy(false);
              closeSwal();
              if (!res.ok) {
                var es = (data && data.error) || "Tidak bisa mengambil judul.";
                setErr(errEl, es);
                errorModal("Scrape gagal", es);
                return;
              }
              if (data.titles_text != null) {
                fillBulkArticleTitles(data.titles_text);
              }
              var n =
                data && data.count != null ? parseInt(data.count, 10) : null;
              var sub =
                n != null && !isNaN(n)
                  ? "Extracted " +
                    n +
                    " judul ke Judul artikel (box yang sama untuk Buat artikel)."
                  : "Judul sudah ditulis ke Judul artikel (box yang sama untuk Buat artikel).";
              if (swalAvailable()) {
                Swal.fire({
                  icon: "success",
                  title: "Judul dari XML",
                  text: sub,
                  timer: 2600,
                  showConfirmButton: true,
                });
              }
            });
          })
          .catch(function (e) {
            setXmlScrapeBusy(false);
            closeSwal();
            if (e && e.message === "unauth") {
              return;
            }
            setErr(errEl, "Error jaringan.");
            errorModal("Error jaringan", "Tidak bisa terhubung ke server.");
          });
      });
    }

    if (googleTrendsBtn) {
      googleTrendsBtn.addEventListener("click", function () {
        setErr(errEl, null);
        var urlEl = qs("#bulk-google-trends-url");
        var urlVal = urlEl ? String(urlEl.value || "").trim() : "";
        var body = { url: urlVal };
        setGoogleTrendsBusy(true);
        openSimpleLoadingModal(
          "Mengambil keyword Google Trends",
          "Mengunduh data Google Trends…"
        );
        fetchAuth(API_FETCH_GOOGLE_TRENDS, {
          method: "POST",
          body: JSON.stringify(body),
        })
          .then(function (res) {
            return res.json().then(function (data) {
              setGoogleTrendsBusy(false);
              closeSwal();
              if (!res.ok) {
                var es = (data && data.error) || "Tidak bisa mengambil keyword Google Trends.";
                setErr(errEl, es);
                errorModal("Google Trends gagal", es);
                return;
              }
              if (data.titles_text != null) {
                fillBulkArticleTitles(data.titles_text);
              }
              var n =
                data && data.count != null ? parseInt(data.count, 10) : null;
              var sub =
                n != null && !isNaN(n)
                  ? "Extracted " +
                    n +
                    " keyword ke Judul artikel (box yang sama untuk Buat artikel)."
                  : "Keyword sudah ditulis ke Judul artikel (box yang sama untuk Buat artikel).";
              if (swalAvailable()) {
                Swal.fire({
                  icon: "success",
                  title: "Keyword dari Google Trends",
                  text: sub,
                  timer: 2600,
                  showConfirmButton: true,
                });
              }
            });
          })
          .catch(function (e) {
            setGoogleTrendsBusy(false);
            closeSwal();
            if (e && e.message === "unauth") {
              return;
            }
            setErr(errEl, "Error jaringan.");
            errorModal("Error jaringan", "Tidak bisa terhubung ke server.");
          });
      });
    }

    if (regenBtn) {
      regenBtn.addEventListener("click", function () {
        setErr(errEl, null);
        var ta = qs("#bulk-titles");
        var titlesText = ta ? String(ta.value || "") : "";
        var lines = titlesText.split(/\r?\n/);
        var nonEmpty = 0;
        for (var ri = 0; ri < lines.length; ri++) {
          if (lines[ri].trim()) {
            nonEmpty++;
          }
        }
        if (!titlesText.trim() || nonEmpty === 0) {
          var mr =
            "Tambahkan minimal satu baris judul di Judul artikel sebelum regenerate.";
          setErr(errEl, mr);
          warnModal("Periksa input", mr);
          return;
        }
        if (lines.length > 1000) {
          var mMax = "Maksimal 1000 baris di Judul artikel.";
          setErr(errEl, mMax);
          warnModal("Periksa input", mMax);
          return;
        }
        var totalLines = lines.length;
        function runBulkRegenerateTitles() {
          regenBtn.disabled = true;
          if (suggestBtn) {
            suggestBtn.disabled = true;
          }
          if (scrapeBtn) {
            scrapeBtn.disabled = true;
          }
          if (genBtn) {
            genBtn.disabled = true;
          }
          openSimpleLoadingModal("Buat ulang judul", "Memanggil AI…");
          fetchAuth(API_REGEN_TITLES, {
            method: "POST",
            body: JSON.stringify({
              titles_text: titlesText,
              keyword: "",
              provider: "auto",
            }),
          })
            .then(function (res) {
              return res.json().then(function (data) {
                regenBtn.disabled = false;
                if (suggestBtn) {
                  suggestBtn.disabled = false;
                }
                if (scrapeBtn) {
                  scrapeBtn.disabled = false;
                }
                if (genBtn) {
                  genBtn.disabled = false;
                }
                closeSwal();
                if (!res.ok) {
                  var em = (data && data.error) || "Gagal membuat ulang judul.";
                  setErr(errEl, em);
                  if (
                    window.AdminAILimit &&
                    typeof AdminAILimit.showFromResponse === "function" &&
                    AdminAILimit.showFromResponse(data)
                  ) {
                    return;
                  }
                  errorModal("Buat ulang gagal", em);
                  return;
                }
                if (data.titles_text != null) {
                  applyRegeneratedTitlesAndSlugs(data.titles_text);
                }
                if (swalAvailable()) {
                  Swal.fire({
                    icon: "success",
                    title: "Judul berhasil dibuat ulang",
                    text:
                      "Judul artikel dan slug sudah diperbarui dari judul baru.",
                    timer: 2400,
                    showConfirmButton: true,
                  });
                }
              });
            })
            .catch(function (e) {
              regenBtn.disabled = false;
              if (suggestBtn) {
                suggestBtn.disabled = false;
              }
              if (scrapeBtn) {
                scrapeBtn.disabled = false;
              }
              if (genBtn) {
                genBtn.disabled = false;
              }
              closeSwal();
              if (e && e.message === "unauth") {
                return;
              }
              setErr(errEl, "Error jaringan.");
              errorModal("Error jaringan", "Tidak bisa terhubung ke server.");
            });
        }
        confirmRegenerateTitles(totalLines, nonEmpty).then(function (ok) {
          if (!ok) {
            return;
          }
          runBulkRegenerateTitles();
        });
      });
    }

    if (genBtn) {
      genBtn.addEventListener("click", function () {
        setErr(errEl, null);
        setOk(okEl, null);
        var pair = pairTitlesAndSlugsFromBulkUI();
        if (pair.err) {
          setErr(errEl, pair.err);
          warnModal("Periksa input", pair.err);
          return;
        }
        var titles = pair.titles;
        var slugs = pair.slugs;
        var categoryRefs = pair.categoryRefs || [];
        var categoryErr = validateBulkCategoryRefs(categoryRefs);
        if (categoryErr) {
          setErr(errEl, categoryErr);
          warnModal("Periksa input", categoryErr);
          return;
        }
        var cat = parseInt(
          (qs("#bulk-category") && qs("#bulk-category").value) || "0",
          10
        );
        if (!categoryRefs.length && !cat) {
          var v1 = "Pilih kategori.";
          setErr(errEl, v1);
          warnModal("Periksa input", v1);
          return;
        }
        var authorSel = qs("#bulk-author");
        var authorId = 0;
        if (authorSel && authorSel.options && authorSel.options.length) {
          try {
            var st = window.getComputedStyle(authorSel);
            if (st.display !== "none" && st.visibility !== "hidden") {
              authorId = parseInt(authorSel.value || "0", 10);
            }
          } catch (e) {
            authorId = parseInt(authorSel.value || "0", 10);
          }
        }
        if (!authorId) {
          authorId = getAuthorId();
        }
        if (!authorId) {
          var v2 = "Tidak bisa menentukan penulis.";
          setErr(errEl, v2);
          warnModal("Periksa input", v2);
          return;
        }
        var interval = parseInt(
          (qs("#bulk-interval") && qs("#bulk-interval").value) || "0",
          10
        );
        var status =
          (qs("#bulk-status") && qs("#bulk-status").value) || "draft";
        var promptVariant = parseInt(
          (qs("#bulk-prompt-variant") && qs("#bulk-prompt-variant").value) ||
            "1",
          10
        );
        if (isNaN(promptVariant) || promptVariant < 1 || promptVariant > 3) {
          promptVariant = 1;
        }
        var prov =
          (qs("#bulk-provider") && qs("#bulk-provider").value) || "gemini";
        var kwEl = qs("#bulk-keyword");
        var kw = kwEl ? kwEl.value.trim() : "";

        var batchBase = new Date().toISOString();
        var n = titles.length;
        var tagIds = getBulkSelectedTagIds();
        var coverFromFeedsEl = qs("#bulk-cover-from-feeds");
        var coverFromFeeds = !coverFromFeedsEl || !!coverFromFeedsEl.checked;
        var coverImageMode = currentBulkCoverMode();
        var coverWebSearchSource = currentBulkCoverWebSearchSource();
        var imagePrompt = (
          qs("#bulk-image-prompt") && qs("#bulk-image-prompt").value
            ? qs("#bulk-image-prompt").value
            : ""
        ).trim();
        var imageToContent = bulkImageToContentEnabled();

        var allSlugEmpty = slugs.every(function (s) {
          return !s;
        });
        var genPayload = {
          keyword: kw,
          titles: titles,
          category_id: cat,
          author_id: authorId,
          tags: tagIds,
          interval_minutes: isNaN(interval) ? 0 : interval,
          provider: prov,
          prompt_content_variant: promptVariant,
          status: status,
          batch_base_time: batchBase,
          title_index: 0,
          cover_image_from_feeds: coverFromFeeds,
          cover_image_mode: coverImageMode,
          cover_web_search_source: coverWebSearchSource,
          image_prompt: imagePrompt,
          image_to_content_enabled: imageToContent,
        };
        if (categoryRefs.length) {
          genPayload.category_refs = categoryRefs;
          delete genPayload.category_id;
        }
        if (!allSlugEmpty) {
          genPayload.slugs = slugs;
        }

        genBtn.disabled = true;
        if (suggestBtn) {
          suggestBtn.disabled = true;
        }
        if (scrapeBtn) {
          scrapeBtn.disabled = true;
        }
        if (regenBtn) {
          regenBtn.disabled = true;
        }

        (async function () {
          var bulkGenUiState = { isBackground: false };
          try {
            bulkGenUiState = await openBulkGenLoadingModal(n);
            var enqueueRes = await fetchAuth(API_GEN, {
              method: "POST",
              body: JSON.stringify(genPayload),
            });
            var enqueueData = null;
            try {
              enqueueData = await enqueueRes.json();
            } catch (je) {
              enqueueData = null;
            }
            if (!enqueueRes.ok) {
              var em =
                (enqueueData && enqueueData.error) || "Gagal memasukkan job ke antrean.";
              setErr(errEl, em);
              closeSwal();
              errorModal("Antrean gagal", em);
              return;
            }
            var jobId =
              enqueueData && enqueueData.job_id
                ? String(enqueueData.job_id)
                : "";
            if (!jobId) {
              setErr(errEl, "Response antrean tidak valid: job_id tidak ada.");
              closeSwal();
              errorModal("Antrean gagal", "Response antrean tidak valid.");
              return;
            }
            activeBulkGenerateJobId = jobId;
            activeBulkGenerateCanceled = false;
            setBulkForceStopBusy(false);
            setBulkForceStopVisible(true);

            var allResults = [];
            var ok = 0;
            var bad = 0;
            while (true) {
              var stRes = await fetchAuth(
                API_GEN_JOB + encodeURIComponent(jobId),
                {
                  method: "GET",
                }
              );
              var st = null;
              try {
                st = await stRes.json();
              } catch (jse) {
                st = null;
              }
              if (!stRes.ok) {
                var sem = (st && st.error) || "Gagal membaca status job.";
                setErr(errEl, sem);
                closeSwal();
                errorModal("Error job background", sem);
                return;
              }
              var done = parseInt((st && st.done) || 0, 10);
              var total = parseInt((st && st.total) || n, 10);
              ok = parseInt((st && st.succeeded) || 0, 10);
              bad = parseInt((st && st.failed) || 0, 10);
              allResults = (st && st.results) || [];
              if (!bulkGenUiState.isBackground) {
                var pct = total > 0 ? Math.round((done / total) * 100) : 0;
                var statusNow = String((st && st.status) || "").toLowerCase();
                // While running, keep progress in [1..99] so the user sees movement even when
                // the backend only reports done at the end (common for total=1).
                if (
                  statusNow !== "done" &&
                  statusNow !== "failed" &&
                  statusNow !== "canceled"
                ) {
                  if (pct < 1) pct = 1;
                  if (pct > 99) pct = 99;
                }
                var progressMsg =
                  "Selesai " +
                  done +
                  " dari " +
                  total +
                  " (ok: " +
                  ok +
                  ", gagal: " +
                  bad +
                  ")";
                if (statusNow === "canceled" || activeBulkGenerateCanceled) {
                  progressMsg =
                    statusNow === "canceled"
                      ? "Dihentikan. Selesai " + done + " dari " + total + "."
                      : "Permintaan stop dikirim. Menunggu step saat ini selesai…";
                }
                updateBulkGenLoadingModal(
                  pct,
                  progressMsg
                );
              }
              var statusNow = String((st && st.status) || "").toLowerCase();
              if (
                statusNow === "done" ||
                statusNow === "failed" ||
                statusNow === "canceled"
              ) {
                if (statusNow === "canceled") {
                  activeBulkGenerateCanceled = true;
                }
                break;
              }
              await new Promise(function (r) {
                setTimeout(r, 1500);
              });
            }
            var finishedText = activeBulkGenerateCanceled
              ? "Dihentikan: " + ok + " berhasil, " + bad + " gagal."
              : "Selesai: " + ok + " berhasil, " + bad + " gagal.";
            setOk(okEl, finishedText);
            closeSwal();
            if (swalAvailable()) {
              if (activeBulkGenerateCanceled) {
                Swal.fire({
                  icon: "warning",
                  title: "Pembuatan bulk dihentikan",
                  text: finishedText,
                  confirmButtonText: "Lihat detail",
                }).then(function () {
                  showBulkGenResultModal(ok, bad, allResults, true);
                });
              } else if (bulkGenUiState.isBackground) {
                Swal.fire({
                  icon: bad === 0 ? "success" : "warning",
                  title: "Generate background selesai",
                  text: "Selesai: " + ok + " berhasil, " + bad + " gagal.",
                  confirmButtonText: "Lihat detail",
                }).then(function () {
                  showBulkGenResultModal(ok, bad, allResults, false);
                });
              } else {
                await showBulkGenResultModal(ok, bad, allResults, false);
              }
            } else if (
              typeof window.AdminCrud !== "undefined" &&
              typeof window.AdminCrud.showSuccessToast === "function"
            ) {
              window.AdminCrud.showSuccessToast(
                activeBulkGenerateCanceled
                  ? "Pembuatan bulk dihentikan."
                  : "Bulk: " + ok + " artikel dibuat."
              );
            }
          } catch (e) {
            if (e && e.message === "unauth") {
              closeSwal();
              return;
            }
            setErr(errEl, "Error jaringan.");
            closeSwal();
            errorModal("Error jaringan", "Tidak bisa terhubung ke server.");
          } finally {
            clearActiveBulkGenerateJob();
            genBtn.disabled = false;
            if (suggestBtn) {
              suggestBtn.disabled = false;
            }
            if (scrapeBtn) {
              scrapeBtn.disabled = false;
            }
            if (regenBtn) {
              regenBtn.disabled = false;
            }
          }
        })();
      });
    }
  });
})();
