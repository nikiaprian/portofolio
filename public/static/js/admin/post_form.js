(function () {
  var AC = window.AdminCrud;
  if (
    !AC ||
    typeof tinymce === "undefined" ||
    !window.TinyMCE ||
    typeof TomSelect === "undefined"
  ) {
    return;
  }

  var root = document.getElementById("admin-post-form-root");
  if (!root) return;

  var API_POSTS = "/api/v1/admin/posts";
  var API_UPLOAD_IMG = "/api/v1/admin/posts/upload-image";
  var API_GEN_DRAFT = "/api/v1/admin/posts/generate-draft";
  var API_GEN_IMAGE = "/api/v1/admin/posts/generate-image";
  var API_SUGGEST_TITLE = "/api/v1/admin/posts/bulk-suggest-titles";
  var API_AI_SETTING = "/api/v1/admin/ai-setting";
  var API_CAT = "/api/v1/admin/categories";
  var API_TAGS = "/api/v1/admin/tags";
  var TOKEN_KEY = "tirtayasa_token";
  var LOGIN_URL =
    (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) || "/login";
  var DRAFT_KEY_PREFIX = "tirtayasa_post_form_draft_v1";
  var IMAGE_MIN_EDGE = 700;
  var IMAGE_MAX_EDGE = 1024;

  var mode = (root.getAttribute("data-mode") || "create").toLowerCase();
  var postId = (root.getAttribute("data-post-id") || "").trim();

  var contentEditor = null;
  var tinyReady = null;
  var tsTags = null;
  var slugAutofill = { reset: function () {} };
  var tagModalSlugAutofill = { reset: function () {} };
  var draftSaveTimer = null;
  var webImageSearchState = { title: "", source: "auto", attempt: 0 };
  var coverImageOrigin = { origin: "", detail: "" };

  function setCoverImageOrigin(origin, detail) {
    coverImageOrigin.origin = origin ? String(origin).trim().toLowerCase() : "";
    coverImageOrigin.detail = detail ? String(detail).trim() : "";
  }

  function clearCoverImageOrigin() {
    coverImageOrigin.origin = "";
    coverImageOrigin.detail = "";
  }
  /** Local draft autosave/restore is create-only; edit always loads from server. */
  var allowDraftPersist = mode !== "edit";

  function el(id) {
    return document.getElementById(id);
  }

  /** Same rules as API / AdminNameSlugCrud.slugify; used so title→slug works without that export. */
  function postFormSlugify(str) {
    var s = String(str || "")
      .toLowerCase()
      .normalize("NFD")
      .replace(/[\u0300-\u036f]/g, "");
    s = s.replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
    return s;
  }

  /** Plain text from HTML (for client-side meta fallback when API omits fields or cache is stale). */
  function stripTagsToPlain(html) {
    try {
      var d = document.createElement("div");
      d.innerHTML = String(html || "");
      return String(d.textContent || d.innerText || "")
        .replace(/\s+/g, " ")
        .trim();
    } catch (e) {
      return "";
    }
  }

  function metaDescFallbackFromPlain(plain, title, maxLen) {
    maxLen = maxLen || 160;
    var base = String(plain || "").trim();
    if (!base) {
      base = String(title || "").trim();
    }
    if (!base) {
      return "";
    }
    if (base.length <= maxLen) {
      return base;
    }
    var cut = base.slice(0, maxLen);
    var sp = cut.lastIndexOf(" ");
    if (sp >= 60) {
      cut = cut.slice(0, sp);
    }
    cut = cut.trim();
    return cut.length < base.length ? cut + "…" : cut;
  }

  function updateCharacterCount(inputId, countId) {
    var input = el(inputId);
    var counter = el(countId);
    if (!input || !counter) return;
    var current = String(input.value || "").length;
    var max = parseInt(input.getAttribute("maxlength") || "0", 10);
    counter.textContent = max
      ? current + "/" + max + " karakter"
      : current + " karakter";
  }

  function updateTitleAndMetaCounts() {
    updateCharacterCount("pf-title", "pf-title-count");
    updateCharacterCount("pf-meta-desc", "pf-meta-desc-count");
  }

  /** Comma-separated tag labels in TomSelect order (matches server “keywords from tags”). */
  function tagKeywordsFallbackCSV() {
    if (!tsTags) {
      return "";
    }
    var ids = tsTags.getValue();
    var parts = [];
    for (var i = 0; i < ids.length; i++) {
      var id = ids[i];
      var opt = tsTags.options[id];
      var lab = opt && opt.text ? String(opt.text).trim() : "";
      if (lab) {
        parts.push(lab);
      }
    }
    return parts.join(", ");
  }

  function pickMetaFromGenerateResponse(data) {
    if (!data) {
      return { md: "", mkw: "" };
    }
    var md = "";
    if (data.meta_description != null) {
      md = String(data.meta_description).trim();
    }
    if (!md && data.metaDescription != null) {
      md = String(data.metaDescription).trim();
    }
    var mkw = "";
    if (data.meta_keywords != null) {
      mkw = String(data.meta_keywords).trim();
    }
    if (!mkw && data.metaKeywords != null) {
      mkw = String(data.metaKeywords).trim();
    }
    return { md: md, mkw: mkw };
  }

  function getRequesterRole() {
    return String(root.getAttribute("data-requester-role") || "")
      .trim()
      .toLowerCase();
  }

  function isAdminRole() {
    return getRequesterRole() === "admin";
  }

  function isWriterRole() {
    return getRequesterRole() === "writer";
  }

  function getRequesterIdFromPage() {
    var raw = root.getAttribute("data-requester-id");
    var n = raw ? parseInt(String(raw), 10) : NaN;
    return !isNaN(n) && n > 0 ? n : null;
  }

  /** Writer: always current account. Admin: value of #pf-author. */
  function resolveAuthorIdForPayload() {
    if (isWriterRole()) {
      return getRequesterIdFromPage() || getAuthorId();
    }
    var sel = el("pf-author");
    if (!sel) {
      return getAuthorId();
    }
    var v = parseInt(String(sel.value || "").trim(), 10);
    return !isNaN(v) && v > 0 ? v : null;
  }

  function syncPublishAtWrap() {
    var stEl = el("pf-status");
    var wrap = el("pf-publish-at-wrap");
    var inp = el("pf-publish-at");
    if (!wrap) return;
    var st = stEl ? String(stEl.value || "") : "";
    var show = st === "scheduled";
    wrap.classList.toggle("d-none", !show);
    if (inp) {
      if (show) {
        inp.setAttribute("required", "required");
      } else {
        inp.removeAttribute("required");
      }
    }
  }

  function isoToDatetimeLocal(iso) {
    if (!iso) return "";
    var d = new Date(String(iso));
    if (isNaN(d.getTime())) return "";
    var pad = function (n) {
      return n < 10 ? "0" + n : "" + n;
    };
    return (
      d.getFullYear() +
      "-" +
      pad(d.getMonth() + 1) +
      "-" +
      pad(d.getDate()) +
      "T" +
      pad(d.getHours()) +
      ":" +
      pad(d.getMinutes())
    );
  }

  function datetimeLocalToISO() {
    var inp = el("pf-publish-at");
    if (!inp || !String(inp.value || "").trim()) return null;
    var d = new Date(inp.value);
    if (isNaN(d.getTime())) return null;
    return d.toISOString();
  }

  function publishAtForPayload() {
    if (!el("pf-status") || el("pf-status").value !== "scheduled") return null;
    return datetimeLocalToISO();
  }

  function draftKey() {
    return (
      DRAFT_KEY_PREFIX + ":" + mode + ":" + (mode === "edit" ? postId : "new")
    );
  }

  function clearFormError() {
    AC.setInlineError(el("post-form-error"), null);
  }

  function setFormError(message) {
    AC.setInlineError(el("post-form-error"), message);
    if (message && typeof AC.showErrorToast === "function") {
      AC.showErrorToast(message);
    }
  }

  function postUrl(id) {
    return AC.itemUrl(API_POSTS, id);
  }

  function readDraft() {
    try {
      var raw = localStorage.getItem(draftKey());
      if (!raw) return null;
      var parsed = JSON.parse(raw);
      return parsed && typeof parsed === "object" ? parsed : null;
    } catch (e) {
      return null;
    }
  }

  function clearDraft() {
    try {
      localStorage.removeItem(draftKey());
    } catch (e) {}
  }

  function buildDraftPayload() {
    var catRaw =
      el("pf-category") && String(el("pf-category").value || "").trim();
    var catNum = catRaw ? parseInt(catRaw, 10) : NaN;
    return {
      title: el("pf-title") ? el("pf-title").value : "",
      slug: el("pf-slug") ? el("pf-slug").value : "",
      status: el("pf-status") ? el("pf-status").value : "draft",
      category_id:
        el("pf-category") && !isNaN(catNum) && catNum > 0 ? catNum : null,
      tags: getTagIDs(),
      source: el("pf-source") ? el("pf-source").value : "",
      include_source:
        el("pf-include-source") && el("pf-include-source").checked ? true : false,
      meta_description: el("pf-meta-desc") ? el("pf-meta-desc").value : "",
      meta_keywords: el("pf-meta-kw") ? el("pf-meta-kw").value : "",
      image: el("pf-image") ? el("pf-image").value : "",
      image_url: el("pf-image-url") ? el("pf-image-url").value : "",
      image_origin: coverImageOrigin.origin,
      image_origin_detail: coverImageOrigin.detail,
      image_mode: currentImageMode(),
      image_ai_prompt: el("pf-image-ai-prompt")
        ? el("pf-image-ai-prompt").value
        : "",
      image_ai_provider: el("pf-image-ai-provider")
        ? el("pf-image-ai-provider").value
        : "auto",
      ai_provider: el("pf-ai-provider") ? el("pf-ai-provider").value : "auto",
      ai_prompt_variant: el("pf-ai-prompt-variant")
        ? el("pf-ai-prompt-variant").value
        : "1",
      ai_enabled: el("pf-ai-enable") ? !!el("pf-ai-enable").checked : false,
      content_html: htmlContent(),
      publish_at_local:
        el("pf-status") &&
        el("pf-status").value === "scheduled" &&
        el("pf-publish-at")
          ? el("pf-publish-at").value
          : "",
      saved_at: Date.now(),
    };
  }

  function hasMeaningfulDraft(draft) {
    if (!draft) return false;
    return !!(
      String(draft.title || "").trim() ||
      String(draft.slug || "").trim() ||
      String(draft.source || "").trim() ||
      String(draft.meta_description || "").trim() ||
      String(draft.meta_keywords || "").trim() ||
      String(draft.content_html || "")
        .replace(/<[^>]+>/g, "")
        .trim()
    );
  }

  function saveDraftNow() {
    if (!allowDraftPersist) return;
    try {
      localStorage.setItem(draftKey(), JSON.stringify(buildDraftPayload()));
    } catch (e) {}
  }

  function saveDraftDebounced() {
    if (!allowDraftPersist) return;
    if (draftSaveTimer) clearTimeout(draftSaveTimer);
    draftSaveTimer = setTimeout(saveDraftNow, 180);
  }

  function applyDraft(draft) {
    if (!draft) return false;
    var restored = false;
    if (el("pf-title") && draft.title != null) {
      el("pf-title").value = String(draft.title);
      restored = true;
    }
    if (el("pf-slug") && draft.slug != null) {
      el("pf-slug").value = String(draft.slug);
      restored = true;
    }
    var pfTitle = el("pf-title");
    var pfSlug = el("pf-slug");
    if (
      pfTitle &&
      pfSlug &&
      String(pfTitle.value || "").trim() &&
      !String(pfSlug.value || "").trim()
    ) {
      pfSlug.value = postFormSlugify(pfTitle.value);
    }
    if (el("pf-status") && draft.status != null) {
      el("pf-status").value = String(draft.status);
      restored = true;
    }
    if (el("pf-publish-at") && draft.publish_at_local != null) {
      el("pf-publish-at").value = String(draft.publish_at_local || "");
      restored = true;
    }
    syncPublishAtWrap();
    if (el("pf-category") && draft.category_id != null) {
      var cid = parseInt(String(draft.category_id).trim(), 10);
      if (!isNaN(cid) && cid > 0) {
        el("pf-category").value = String(cid);
        restored = true;
      }
    }
    if (el("pf-source") && draft.source != null) {
      el("pf-source").value = String(draft.source);
      restored = true;
    }
    if (el("pf-include-source") && draft.include_source != null) {
      el("pf-include-source").checked = !!draft.include_source;
      restored = true;
    }
    if (el("pf-meta-desc") && draft.meta_description != null) {
      el("pf-meta-desc").value = String(draft.meta_description);
      restored = true;
    }
    updateTitleAndMetaCounts();
    if (el("pf-meta-kw") && draft.meta_keywords != null) {
      el("pf-meta-kw").value = String(draft.meta_keywords);
      restored = true;
    }
    if (el("pf-image") && draft.image != null) {
      el("pf-image").value = String(draft.image);
      restored = true;
    }
    if (el("pf-image-url") && draft.image_url != null) {
      el("pf-image-url").value = String(draft.image_url);
      restored = true;
    }
    if (draft.image_origin != null || draft.image_origin_detail != null) {
      setCoverImageOrigin(
        draft.image_origin != null ? draft.image_origin : "",
        draft.image_origin_detail != null ? draft.image_origin_detail : ""
      );
      restored = true;
    }
    if (draft.image_mode != null) {
      applyImageMode(String(draft.image_mode));
      restored = true;
    } else {
      refreshCoverImagePreview();
    }
    if (el("pf-image-ai-prompt") && draft.image_ai_prompt != null) {
      el("pf-image-ai-prompt").value = String(draft.image_ai_prompt);
      restored = true;
    }
    if (el("pf-image-ai-provider") && draft.image_ai_provider != null) {
      el("pf-image-ai-provider").value = String(draft.image_ai_provider);
      restored = true;
    }
    if (el("pf-ai-provider") && draft.ai_provider != null) {
      el("pf-ai-provider").value = String(draft.ai_provider);
      restored = true;
    }
    if (el("pf-ai-prompt-variant") && draft.ai_prompt_variant != null) {
      el("pf-ai-prompt-variant").value = String(draft.ai_prompt_variant);
      restored = true;
    }
    if (el("pf-ai-enable") && draft.ai_enabled != null && draft.ai_enabled) {
      el("pf-ai-enable").checked = !!draft.ai_enabled;
      setAIControlsEnabled(!!draft.ai_enabled);
      restored = true;
    }
    if (draft.content_html != null) {
      setEditorHTML(String(draft.content_html || "<p><br></p>"));
      restored = true;
    }
    if (
      tsTags &&
      Array.isArray(draft.tags) &&
      draft.tags.some(function (v) {
        var n = parseInt(String(v), 10);
        return !isNaN(n) && n > 0;
      })
    ) {
      tsTags.setValue(
        draft.tags
          .map(function (v) {
            return String(v);
          })
          .filter(Boolean)
      );
      restored = true;
    }
    return restored;
  }

  function parseJwtUserId(token) {
    try {
      var parts = String(token || "").split(".");
      if (parts.length < 2) return null;
      var b = parts[1].replace(/-/g, "+").replace(/_/g, "/");
      while (b.length % 4) b += "=";
      var payload = JSON.parse(atob(b));
      var uid = payload.user_id;
      return uid != null ? Number(uid) : null;
    } catch (e) {
      return null;
    }
  }

  function getAuthorId() {
    try {
      var raw = localStorage.getItem("tirtayasa_user_id");
      if (raw) {
        var n = parseInt(raw, 10);
        if (!isNaN(n) && n > 0) return n;
      }
    } catch (e) {}
    return parseJwtUserId(AC.getToken(TOKEN_KEY));
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

  async function uploadImageFile(file) {
    var token = AC.getToken(TOKEN_KEY);
    var fd = new FormData();
    fd.append("image", file);
    var slugEl = el("pf-slug");
    var slugVal = slugEl
      ? String(slugEl.value || "")
          .trim()
          .toLowerCase()
      : "";
    if (/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slugVal)) {
      fd.append("slug", slugVal);
    }
    var res = await fetch(API_UPLOAD_IMG, {
      method: "POST",
      headers: {
        Authorization: "Bearer " + token,
      },
      body: fd,
    });
    if (res.status === 401) {
      window.location.href = LOGIN_URL;
      throw new Error("unauthorized");
    }
    return res;
  }

  function imageDimensionRuleText() {
    return (
      "Sisi gambar terpanjang harus antara " +
      IMAGE_MIN_EDGE +
      " dan " +
      IMAGE_MAX_EDGE +
      " px."
    );
  }

  function loadImageSizeFromSrc(src) {
    return new Promise(function (resolve, reject) {
      var img = new Image();
      var done = false;
      var timer = setTimeout(function () {
        if (done) return;
        done = true;
        reject(new Error("timeout"));
      }, 12000);
      img.onload = function () {
        if (done) return;
        done = true;
        clearTimeout(timer);
        resolve({
          width: img.naturalWidth || 0,
          height: img.naturalHeight || 0,
        });
      };
      img.onerror = function () {
        if (done) return;
        done = true;
        clearTimeout(timer);
        reject(new Error("load failed"));
      };
      img.decoding = "async";
      img.src = src;
    });
  }

  function isWithinImageDimensionRules(width, height) {
    var longest = Math.max(Number(width) || 0, Number(height) || 0);
    return longest >= IMAGE_MIN_EDGE && longest <= IMAGE_MAX_EDGE;
  }

  async function validateImageDimensionsFromFile(file) {
    if (!file) return { ok: false, error: "File gambar wajib diunggah." };
    var objectUrl = URL.createObjectURL(file);
    try {
      var dim = await loadImageSizeFromSrc(objectUrl);
      if (!isWithinImageDimensionRules(dim.width, dim.height)) {
        return {
          ok: false,
          error:
            imageDimensionRuleText() +
            " Gambar terpilih: " +
            dim.width +
            "x" +
            dim.height +
            ".",
        };
      }
      return { ok: true };
    } catch (e) {
      return { ok: false, error: "Gagal membaca dimensi gambar yang dipilih." };
    } finally {
      try {
        URL.revokeObjectURL(objectUrl);
      } catch (e2) {}
    }
  }

  async function validateImageDimensionsFromURL(url) {
    try {
      var dim = await loadImageSizeFromSrc(url);
      if (!isWithinImageDimensionRules(dim.width, dim.height)) {
        return {
          ok: false,
          error:
            imageDimensionRuleText() +
            " Gambar URL: " +
            dim.width +
            "x" +
            dim.height +
            ".",
        };
      }
      return { ok: true };
    } catch (e) {
      return {
        ok: false,
        error:
          "Gagal memuat URL gambar. Pastikan URL publik dan bisa diakses, lalu coba lagi.",
      };
    }
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
    var btn = el("pf-submit");
    var spin = el("pf-submit-spinner");
    var icon = el("pf-submit-icon");
    var lab = el("pf-submit-label");
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

  function setTagModalBusy(busy) {
    var btn = el("pf-tag-modal-submit");
    var spin = el("pf-tag-modal-spinner");
    var icon = el("pf-tag-modal-check");
    if (!btn) return;
    btn.disabled = !!busy;
    if (spin) spin.classList.toggle("d-none", !busy);
    if (icon) icon.classList.toggle("d-none", !!busy);
  }

  /** Heuristic: user asked for ~800+ words (matches server-side continuation rules). */
  function promptSuggestsLongArticle(prompt) {
    var s = String(prompt || "");
    if (!/(?:kata|words)/i.test(s)) {
      return false;
    }
    // 4–5 digit targets (e.g. 2000 kata, 10000 words)
    if (/\b\d{4,5}\s*(?:kata|words)\b/i.test(s)) {
      return true;
    }
    if (/(?:1000|1100|1200|1300|1400|1500|seribu)\b/i.test(s)) {
      return true;
    }
    if (/\d{3,5}\s*[-–]\s*\d{3,5}\s*(?:kata|words)/i.test(s)) {
      return true;
    }
    if (
      /(?:minimal|min\.?|sekitar|±|kira-kira)\s*:?\s*\d{3,5}\s*(?:kata|words)/i.test(
        s
      )
    ) {
      return true;
    }
    if (/\b(?:8|9)\d{2}\s*(?:kata|words)\b/i.test(s)) {
      return true;
    }
    return false;
  }

  function setAIGenerateBusy(busy, longForm) {
    var btn = el("pf-ai-generate-btn");
    var spin = el("pf-ai-spinner");
    var icon = el("pf-ai-icon");
    var lab = el("pf-ai-label");
    if (!btn) return;
    btn.disabled = !!busy;
    if (busy) {
      if (spin) spin.classList.remove("d-none");
      if (icon) icon.classList.add("d-none");
      if (lab) {
        lab.textContent = longForm ? "Membuat artikel panjang..." : "Membuat...";
      }
    } else {
      if (spin) spin.classList.add("d-none");
      if (icon) icon.classList.remove("d-none");
      if (lab) lab.textContent = "Buat";
    }
  }

  function setTitleRegenerateBusy(busy) {
    var btn = el("pf-title-regenerate-btn");
    var spin = el("pf-title-regenerate-spinner");
    var icon = el("pf-title-regenerate-icon");
    var label = el("pf-title-regenerate-label");
    if (!btn) return;
    btn.disabled = !!busy;
    if (spin) spin.classList.toggle("d-none", !busy);
    if (icon) icon.classList.toggle("d-none", !!busy);
    if (label) label.textContent = busy ? "Membuat ulang..." : "Buat ulang";
  }

  function swalAvailable() {
    return typeof Swal !== "undefined";
  }

  function aiDraftGenLoadingHtml() {
    return (
      '<div class="text-start">' +
      '<p class="small text-secondary mb-2 mb-md-3" id="pf-swal-ai-msg">…</p>' +
      '<div class="progress" style="height:1.35rem" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="0" id="pf-swal-ai-progress-root">' +
      '<div id="pf-swal-ai-bar" class="progress-bar progress-bar-striped progress-bar-animated bg-primary" style="width:0%">0%</div>' +
      "</div>" +
      '<p class="small text-secondary mb-0 mt-2"><i class="bi bi-info-circle me-1" aria-hidden="true"></i>Dialog ini bisa diminimalkan saat proses masih berjalan.</p>' +
      '<div class="d-flex justify-content-center mt-3">' +
      '<button type="button" class="btn btn-sm btn-light border d-inline-flex align-items-center gap-1 px-2 py-1" id="pf-swal-minimize-btn" title="Sembunyikan dialog ini dan lanjutkan di background">' +
      '<i class="bi bi-dash-circle" aria-hidden="true"></i>' +
      "<span>Minimalkan</span>" +
      "</button>" +
      "</div>" +
      "</div>"
    );
  }

  function bindSwalMinimize(onMinimized) {
    if (!swalAvailable() || !Swal.isVisible()) return;
    var root = Swal.getHtmlContainer();
    if (!root) return;
    var btn = root.querySelector("#pf-swal-minimize-btn");
    if (!btn) return;
    btn.addEventListener("click", function () {
      if (typeof onMinimized === "function") onMinimized();
      Swal.close();
    });
  }

  function openAiDraftGenLoadingModal(longForm) {
    return new Promise(function (resolve) {
      if (!swalAvailable()) {
        resolve();
        return;
      }
      Swal.fire({
        title: longForm ? "Membuat artikel panjang" : "Membuat draf",
        html: aiDraftGenLoadingHtml(),
        allowOutsideClick: false,
        allowEscapeKey: false,
        showConfirmButton: false,
        width: "28rem",
        didOpen: function () {
          bindSwalMinimize(function () {
            if (typeof AC.showSuccessToast === "function") {
              AC.showSuccessToast("Pembuatan konten berjalan di background.");
            }
          });
          updateAiDraftGenLoadingModal(
            0,
            longForm
              ? "Memulai (akan ada proses tambahan jika target kata panjang)..."
              : "Memulai..."
          );
          resolve();
        },
      });
    });
  }

  function updateAiDraftGenLoadingModal(pct, msg) {
    if (!swalAvailable() || !Swal.isVisible()) {
      return;
    }
    var root = Swal.getHtmlContainer();
    if (!root) {
      return;
    }
    var p = Math.max(0, Math.min(100, Math.round(pct)));
    var bar = root.querySelector("#pf-swal-ai-bar");
    var label = root.querySelector("#pf-swal-ai-msg");
    var prog = root.querySelector("#pf-swal-ai-progress-root");
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

  function closeAiDraftGenSwal() {
    if (swalAvailable() && Swal.isVisible()) {
      Swal.close();
    }
  }

  function openAiImageGenLoadingModal() {
    if (!swalAvailable()) return;
    Swal.fire({
      title: "Membuat gambar",
      html: aiDraftGenLoadingHtml(),
      allowOutsideClick: false,
      allowEscapeKey: false,
      showConfirmButton: false,
      width: "28rem",
      didOpen: function () {
        bindSwalMinimize(function () {
          if (typeof AC.showSuccessToast === "function") {
            AC.showSuccessToast("Pembuatan gambar berjalan di background.");
          }
        });
        updateAiDraftGenLoadingModal(0, "Menyiapkan permintaan gambar...");
      },
    });
  }

  function openWebImageSearchLoadingModal() {
    if (!swalAvailable()) return;
    Swal.fire({
      title: "Mencari gambar web",
      html: aiDraftGenLoadingHtml(),
      allowOutsideClick: false,
      allowEscapeKey: false,
      showConfirmButton: false,
      width: "28rem",
      didOpen: function () {
        bindSwalMinimize(function () {
          if (typeof AC.showSuccessToast === "function") {
            AC.showSuccessToast("Pencarian gambar web berjalan di background.");
          }
        });
        updateAiDraftGenLoadingModal(0, "Menyiapkan pencarian web...");
      },
    });
  }

  function closeAiImageGenSwal() {
    if (swalAvailable() && Swal.isVisible()) {
      Swal.close();
    }
  }

  /** Smooth 0→~94% while image request in flight; finish at 100% when done. */
  function startAiImageFakeProgress() {
    var msgs = [
      "Mengirim prompt ke AI...",
      "Merender gambar sampul...",
      "Menyelesaikan data gambar...",
      "Hampir selesai...",
    ];
    var msgIdx = 0;
    var tick = 0;
    var pct = 0;
    var id = setInterval(function () {
      tick++;
      pct += (94 - pct) * (0.07 + Math.random() * 0.05);
      if (pct > 93) {
        pct = 93;
      }
      if (tick % 4 === 0 && msgIdx < msgs.length - 1) {
        msgIdx++;
      }
      updateAiDraftGenLoadingModal(pct, msgs[msgIdx]);
    }, 320);
    return function stopAiImageFakeProgress() {
      clearInterval(id);
    };
  }

  function startWebImageSearchFakeProgress() {
    var msgs = [
      "Mencari sumber gambar publik...",
      "Memeriksa kandidat...",
      "Mengunduh gambar...",
      "Hampir selesai...",
    ];
    var msgIdx = 0;
    var tick = 0;
    var pct = 0;
    var id = setInterval(function () {
      tick++;
      pct += (94 - pct) * (0.08 + Math.random() * 0.05);
      if (pct > 93) {
        pct = 93;
      }
      if (tick % 4 === 0 && msgIdx < msgs.length - 1) {
        msgIdx++;
      }
      updateAiDraftGenLoadingModal(pct, msgs[msgIdx]);
    }, 320);
    return function stopWebImageSearchFakeProgress() {
      clearInterval(id);
    };
  }

  /** After generate-draft: show how many AI round-trips the server ran. */
  function showAiGenerateStatsAlert(aiRequests, wordCount) {
    if (!swalAvailable()) {
      return;
    }
    var n = parseInt(String(aiRequests), 10);
    if (isNaN(n) || n < 1) {
      n = 1;
    }
    var wc = wordCount != null ? parseInt(String(wordCount), 10) : NaN;
    var lines =
      n === 1
        ? '<p class="mb-1">Model dipanggil <strong>1</strong> kali.</p>'
        : '<p class="mb-1">Model dipanggil <strong>' +
          n +
          "</strong> kali (draf awal ditambah lanjutan otomatis di server).</p>";
    if (!isNaN(wc) && wc > 0) {
      lines +=
        '<p class="small text-secondary mb-0">Perkiraan panjang isi: ~' +
        wc +
        " kata (teks biasa, tanpa tag HTML).</p>";
    }
    Swal.fire({
      icon: "success",
      title: "Selesai",
      html: lines,
      confirmButtonText: "OK",
      timer: n > 2 ? 6500 : 4200,
      timerProgressBar: true,
    });
  }

  /** Smooth 0→~92% while the single HTTP request is in flight; finish at 100% when done. */
  function startAiDraftFakeProgress(longForm) {
    var msgs = longForm
      ? [
          "Menulis draf pertama...",
          "Memeriksa panjang (server bisa menjalankan beberapa proses AI)...",
          "Memperluas dengan proses lanjutan jika perlu...",
          "Masih mengejar target jumlah kata...",
        ]
      : ["Menulis draf dengan AI...", "Hampir selesai..."];
    var msgIdx = 0;
    var tick = 0;
    var pct = 0;
    var id = setInterval(function () {
      tick++;
      pct += (92 - pct) * (0.055 + Math.random() * 0.045);
      if (pct > 91) {
        pct = 91;
      }
      if (longForm && tick % 5 === 0 && msgIdx < msgs.length - 1) {
        msgIdx++;
      }
      if (!longForm && tick === 4 && msgIdx < 1) {
        msgIdx = 1;
      }
      updateAiDraftGenLoadingModal(pct, msgs[msgIdx]);
    }, 420);
    return function stopAiDraftFakeProgress() {
      clearInterval(id);
    };
  }

  function destroyTagTomSelect() {
    if (tsTags) {
      tsTags.destroy();
      tsTags = null;
    }
  }

  function tomSelectTagOptions() {
    return {
      plugins: ["remove_button"],
      placeholder: "Cari atau pilih tag...",
      hideSelected: true,
      maxOptions: null,
      duplicates: false,
      // Show TomSelect "create" row when query is ≥3 chars and no tag name matches exactly
      // (substring matches like "saldo dana" for "dana" still appear above).
      create: function (input, callback) {
        openNewTagModal(String(input || "").trim());
        callback();
      },
      createFilter: function (input) {
        var q = String(input || "").trim();
        if (q.length < 3) return false;
        var low = q.toLowerCase();
        for (var k in this.options) {
          if (!Object.prototype.hasOwnProperty.call(this.options, k)) continue;
          var o = this.options[k];
          var text = String((o && o.text) || "").trim().toLowerCase();
          if (text === low) return false;
        }
        return true;
      },
      render: {
        option_create: function (data, escape) {
          var q = String((data && data.input) || "").trim();
          return (
            '<div class="create pf-ts-add-exact px-3 py-2 border-top">' +
            '<div class="d-inline-flex align-items-center gap-2 text-primary">' +
            '<i class="bi bi-plus-lg" aria-hidden="true"></i>' +
            '<span>Add <span class="fw-semibold">“' +
            escape(q) +
            '”</span> sebagai tag baru</span></div></div>'
          );
        },
        no_results: function (data, escape) {
          var q = String((data && data.input) || "").trim();
          if (q.length < 1) {
            return '<div class="pf-ts-hint px-3 py-2 small text-secondary">Ketik untuk mencari tag.</div>';
          }
          if (q.length < 3) {
            return '<div class="pf-ts-hint px-3 py-2 small text-secondary">Masukkan minimal 3 karakter untuk menambahkan tag baru dari pencarian.</div>';
          }
          return (
            '<div class="pf-ts-hint px-3 py-2 small text-secondary">Tidak ada nama tag dalam daftar yang cocok dengan pencarian ini.</div>'
          );
        },
      },
    };
  }

  function getTagIDs() {
    if (!tsTags) return [];
    return tsTags
      .getValue()
      .map(function (v) {
        return parseInt(v, 10);
      })
      .filter(function (n) {
        return !isNaN(n) && n > 0;
      });
  }

  function slugifyTagNameForModal(str) {
    return postFormSlugify(str);
  }

  /** Opens the new-tag modal; optional prefill from TomSelect search. */
  function openNewTagModal(prefillName) {
    var modalRoot = el("pf-tag-modal");
    if (!modalRoot) return;
    AC.setInlineError(el("pf-tag-modal-error"), null);
    var ne = el("pf-tag-modal-name");
    var se = el("pf-tag-modal-slug");
    var raw = String(prefillName || "").trim();
    tagModalSlugAutofill.reset();
    if (ne) ne.value = raw;
    if (se) {
      se.value = raw.length >= 3 ? slugifyTagNameForModal(raw) : "";
    }
    if (tsTags) {
      try {
        if (typeof tsTags.close === "function") {
          tsTags.close();
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

  function getTinyEditor() {
    if (contentEditor) return contentEditor;
    if (typeof tinymce !== "undefined") {
      var ed = tinymce.get("pf-content");
      if (ed) return ed;
    }
    return null;
  }

  function htmlContent() {
    var ed = getTinyEditor();
    if (!ed) return "";
    return String(ed.getContent() || "").trim();
  }

  function appendInlineStyle(el, decl) {
    if (!el || !decl) return;
    var prev = String(el.getAttribute("style") || "").trim();
    if (!prev) {
      el.setAttribute("style", decl);
      return;
    }
    var low = prev.toLowerCase();
    var need = decl
      .split(";")
      .map(function (s) {
        return String(s || "").trim();
      })
      .filter(Boolean);
    for (var i = 0; i < need.length; i++) {
      var k = need[i].split(":")[0];
      if (!k) continue;
      if (low.indexOf(String(k).toLowerCase() + ":") >= 0) continue;
      prev += (prev.slice(-1) === ";" ? " " : "; ") + need[i] + ";";
    }
    el.setAttribute("style", prev.trim());
  }

  // Keep image alignment consistent in saved article body.
  function normalizeContentImagesCentered(html) {
    var raw = String(html || "").trim();
    if (!raw) return raw;
    var wrap = document.createElement("div");
    wrap.innerHTML = raw;
    var imgs = wrap.querySelectorAll("img");
    if (!imgs || !imgs.length) return raw;
    for (var i = 0; i < imgs.length; i++) {
      var img = imgs[i];
      img.classList.add("ai-inline-image");
      img.setAttribute("align", "center");
      appendInlineStyle(img, "display:block; margin:0 auto; max-width:100%; height:auto");
      var p = img.parentElement;
      if (p && p.tagName && String(p.tagName).toLowerCase() === "p") {
        p.classList.add("ai-inline-image-center");
        p.setAttribute("align", "center");
        appendInlineStyle(p, "text-align:center");
      }
    }
    return String(wrap.innerHTML || "").trim();
  }

  function stripBrTagsToSpace(html) {
    var inHtml = String(html || "");
    if (!inHtml) return inHtml;
    var wrap = document.createElement("div");
    wrap.innerHTML = inHtml;
    var brs = Array.prototype.slice.call(wrap.querySelectorAll("p br"));
    if (!brs.length) return inHtml;
    for (var i = 0; i < brs.length; i++) {
      var br = brs[i];
      if (br && br.parentNode) {
        br.parentNode.replaceChild(document.createTextNode(" "), br);
      }
    }
    return String(wrap.innerHTML || "").trim();
  }

  /** Collapse whitespace/newlines between tags (use <p> for real breaks). */
  function stripWhitespaceBetweenTags(html) {
    return String(html || "").replace(/>[\t\r\n\f ]+</g, "><");
  }

  function inlinePhrasingTag(tag) {
    return /^(a|abbr|b|bdi|bdo|basefont|big|br|cite|code|data|del|dfn|em|font|i|ins|kbd|mark|nobr|q|ruby|rp|rt|s|samp|small|span|strike|strong|sub|sup|time|tt|u|var|wbr)$/i.test(
      String(tag || "")
    );
  }

  function isInlinePhrasingSubtreeOnly(p) {
    if (!p || !p.tagName || String(p.tagName).toLowerCase() !== "p") {
      return false;
    }
    function walk(x) {
      if (x.nodeType === 3) return true;
      if (x.nodeType === 8) return true;
      if (x.nodeType === 1) {
        if (!inlinePhrasingTag(x.tagName)) return false;
        var ch = x.childNodes;
        for (var i = 0; i < ch.length; i++) {
          if (!walk(ch[i])) return false;
        }
        return true;
      }
      return false;
    }
    var ch = p.childNodes;
    for (var i = 0; i < ch.length; i++) {
      if (!walk(ch[i])) return false;
    }
    return true;
  }

  /** <p> that only has text and optional <br> (no bold/link/etc.). */
  function isPlainPhrasingParagraph(p) {
    if (!p || !p.tagName || String(p.tagName).toLowerCase() !== "p") {
      return false;
    }
    var ch = p.childNodes;
    for (var i = 0; i < ch.length; i++) {
      var n = ch[i];
      if (n.nodeType === 3) continue;
      if (n.nodeType === 1 && String(n.tagName).toLowerCase() === "br") {
        continue;
      }
      return false;
    }
    return true;
  }

  function plainPHasLineBreaks(p) {
    var ch = p.childNodes;
    for (var i = 0; i < ch.length; i++) {
      var n = ch[i];
      if (n.nodeType === 1 && String(n.tagName).toLowerCase() === "br") {
        return true;
      }
      if (n.nodeType === 3 && /\r|\n/.test(String(n.nodeValue || ""))) {
        return true;
      }
    }
    return false;
  }

  function pContainsBr(p) {
    if (!p || !p.querySelector) return false;
    return !!p.querySelector("br");
  }

  function expandPlainParagraphBreaks(p) {
    var segs = splitPlainPOnBreaks(p);
    var had = plainPHasLineBreaks(p);
    if (!segs.length) {
      return [p];
    }
    if (segs.length === 1 && !had) {
      return [p];
    }
    var out = [];
    for (var s = 0; s < segs.length; s++) {
      var np = document.createElement("p");
      np.textContent = segs[s];
      out.push(np);
    }
    return out;
  }

  function subtreeHasNewlineText(root) {
    function walk(n) {
      if (n.nodeType === 3 && /\r|\n/.test(String(n.nodeValue || ""))) {
        return true;
      }
      var ch = n.childNodes;
      for (var i = 0; i < ch.length; i++) {
        if (walk(ch[i])) return true;
      }
      return false;
    }
    return walk(root);
  }

  /**
   * One <p> → one or more <p> (no br in output) / unchanged [p].
   */
  function expandParagraphBreaks(p) {
    if (!p || !p.tagName || String(p.tagName).toLowerCase() !== "p") {
      return [p];
    }
    if (pContainsBr(p)) {
      if (isPlainPhrasingParagraph(p)) {
        return expandPlainParagraphBreaks(p);
      }
      return splitMixedParagraphAtBRs(p);
    }
    if (plainPHasLineBreaks(p) || subtreeHasNewlineText(p)) {
      if (isPlainPhrasingParagraph(p)) {
        return expandPlainParagraphBreaks(p);
      }
      return splitMixedParagraphAtBRs(p);
    }
    return [p];
  }

  /** Split rich <p> on <br> / newlines anywhere (e.g. inside strong, a). */
  function splitMixedParagraphAtBRs(p) {
    var inner = String(p.innerHTML || "");
    if (!inner.trim()) return [p];
    var segs = inner.split(/(?:<br\s*\/?>\s*)+|(?:\r\n|\n|\r)/gi);
    if (segs.length <= 1) return [p];
    var out = [];
    for (var s = 0; s < segs.length; s++) {
      var seg = String(segs[s] || "").trim();
      if (!seg) continue;
      var np = document.createElement("p");
      np.innerHTML = seg;
      if (!np.firstChild) continue;
      out.push(np);
    }
    if (!out.length) return [p];
    if (out.length === 1) {
      p.innerHTML = out[0].innerHTML;
      return [p];
    }
    return out;
  }

  /** Split a plain <p> (text + br only) on <br> and newlines in text nodes. */
  function splitPlainPOnBreaks(p) {
    var segments = [];
    var cur = [];
    function flush() {
      var t = cur
        .join("")
        .replace(/\u00a0/g, " ")
        .replace(/\s+/g, " ")
        .trim();
      if (t) segments.push(t);
      cur = [];
    }
    var ch = p.childNodes;
    for (var i = 0; i < ch.length; i++) {
      var n = ch[i];
      if (n.nodeType === 3) {
        var txt = String(n.nodeValue || "");
        var parts = txt.split(/\r?\n+/);
        for (var j = 0; j < parts.length; j++) {
          if (j > 0) flush();
          cur.push(parts[j]);
        }
      } else if (n.nodeType === 1 && String(n.tagName).toLowerCase() === "br") {
        flush();
      }
    }
    flush();
    return segments;
  }

  /**
   * Replace <br> / raw newlines inside plain <p> with separate <p> blocks (no br).
   */
  function convertBrAndNewlinesToParagraphs(html) {
    var inHtml = String(html || "").trim();
    if (!inHtml) return { html: inHtml, changed: false };
    var wrap = document.createElement("div");
    wrap.innerHTML = inHtml;
    var ps = Array.prototype.slice.call(wrap.querySelectorAll("p")).reverse();
    var changed = false;
    for (var i = 0; i < ps.length; i++) {
      var p = ps[i];
      if (!p.parentNode) continue;
      var repl = expandParagraphBreaks(p);
      if (repl.length === 1 && repl[0] === p) continue;
      if (!repl.length) continue;
      var parent = p.parentNode;
      var mark = p.nextSibling;
      parent.removeChild(p);
      for (var j = 0; j < repl.length; j++) {
        parent.insertBefore(repl[j], mark);
      }
      changed = true;
    }
    if (!changed) {
      return { html: inHtml, changed: false };
    }
    var htmlOut = String(wrap.innerHTML || "").trim();
    if (!htmlOut) {
      return { html: inHtml, changed: false };
    }
    return { html: htmlOut, changed: true };
  }

  /**
   * Split running prose at ". " when the next non-space char is a capital letter
   * (Indonesian/Latin sentence starts). Skips digit.digit decimals.
   */
  function splitPlainTextIntoSentences(t) {
    t = String(t || "")
      .replace(/\u00a0/g, " ")
      .replace(/\s+/g, " ")
      .trim();
    if (!t) return [];
    var n = t.length;
    var sentences = [];
    var start = 0;
    var capRe = /^[A-ZÀ-ÖØ-ÞĀĆČĎĒĘĖĞĢĨĪĮİĶĻŁŃŅÓŌØŔŘŠŚŢȚŪÜŮŲÝŻŽŹ]/;
    for (var j = 0; j < n; j++) {
      if (t.charAt(j) !== ".") continue;
      if (
        j > 0 &&
        /\d/.test(t.charAt(j - 1)) &&
        j + 1 < n &&
        /\d/.test(t.charAt(j + 1))
      ) {
        continue;
      }
      var sp = j + 1;
      while (sp < n && t.charAt(sp) === " ") sp++;
      if (sp < n && capRe.test(t.charAt(sp))) {
        var piece = t.slice(start, j + 1).trim();
        if (piece) sentences.push(piece);
        start = sp;
      }
    }
    var last = t.slice(start).trim();
    if (last) sentences.push(last);
    if (!sentences.length) sentences.push(t);
    return sentences;
  }

  function buildNormalizedTextMap(node) {
    var chars = [];
    var before = [];
    var after = [];
    var rawOffset = 0;
    var prevSpace = true;
    function walk(n) {
      if (n.nodeType === 3) {
        var txt = String(n.nodeValue || "");
        for (var i = 0; i < txt.length; ) {
          var cp = txt.codePointAt(i);
          var ch = String.fromCodePoint(cp);
          var size = ch.length;
          var rawBefore = rawOffset + i;
          var rawAfter = rawBefore + size;
          if (/\s/.test(ch) || ch === "\u00a0") {
            if (!prevSpace) {
              chars.push(" ");
              before.push(rawBefore);
              after.push(rawAfter);
              prevSpace = true;
            }
            i += size;
            continue;
          }
          chars.push(ch);
          before.push(rawBefore);
          after.push(rawAfter);
          prevSpace = false;
          i += size;
        }
        rawOffset += txt.length;
        return;
      }
      var childNodes = n.childNodes || [];
      for (var j = 0; j < childNodes.length; j++) {
        walk(childNodes[j]);
      }
    }
    var children = node.childNodes || [];
    for (var k = 0; k < children.length; k++) {
      walk(children[k]);
    }
    if (chars.length && chars[chars.length - 1] === " ") {
      chars.pop();
      before.pop();
      after.pop();
    }
    return { text: chars.join(""), before: before, after: after };
  }

  function splitNormalizedTextIntoSentenceRanges(t) {
    t = String(t || "").trim();
    if (!t) return [];
    var chars = Array.from(t);
    var ranges = [];
    var start = 0;
    var capRe = /^[A-ZÀ-ÖØ-ÞĀĆČĎĒĘĖĞĢĨĪĮİĶĻŁŃŅÓŌØŔŘŠŚŢȚŪÜŮŲÝŻŽŹ]/;
    for (var i = 0; i < chars.length; i++) {
      if (chars[i] !== ".") continue;
      if (isDecimalPeriod(chars, i) || isDomainLikePeriod(chars, i)) {
        continue;
      }
      var sp = i + 1;
      while (sp < chars.length && chars[sp] === " ") sp++;
      if (sp < chars.length && capRe.test(chars[sp])) {
        ranges.push({ start: start, end: i + 1 });
        start = sp;
      }
    }
    if (start < chars.length) {
      ranges.push({ start: start, end: chars.length });
    }
    return ranges;
  }

  function isDecimalPeriod(chars, i) {
    return (
      i > 0 &&
      /\d/.test(chars[i - 1]) &&
      i + 1 < chars.length &&
      /\d/.test(chars[i + 1])
    );
  }

  function isDomainLikePeriod(chars, i) {
    if (i <= 0 || i + 1 >= chars.length) return false;
    if (!isDomainLabelChar(chars[i - 1]) || !isDomainLabelChar(chars[i + 1])) {
      return false;
    }

    var left = i - 1;
    while (left >= 0 && isDomainLabelChar(chars[left])) left--;
    var right = i + 1;
    while (right < chars.length && isDomainLabelChar(chars[right])) right++;
    return (
      hasLetter(chars.slice(left + 1, i)) &&
      hasLetter(chars.slice(i + 1, right))
    );
  }

  function isDomainLabelChar(ch) {
    return isLetterChar(ch) || /^[0-9-]$/.test(ch);
  }

  function hasLetter(chars) {
    for (var i = 0; i < chars.length; i++) {
      if (isLetterChar(chars[i])) return true;
    }
    return false;
  }

  function isLetterChar(ch) {
    return (
      typeof ch === "string" &&
      ch.length > 0 &&
      ch.toLowerCase() !== ch.toUpperCase()
    );
  }

  function cloneParagraphTextRange(p, start, end) {
    var np = document.createElement("p");
    var pos = { value: 0 };
    var children = p.childNodes || [];
    for (var i = 0; i < children.length; i++) {
      var cloned = cloneNodeTextRange(children[i], start, end, pos);
      if (cloned) np.appendChild(cloned);
    }
    return np;
  }

  function cloneNodeTextRange(node, start, end, pos) {
    if (node.nodeType === 3) {
      var txt = String(node.nodeValue || "");
      var nodeStart = pos.value;
      var nodeEnd = nodeStart + txt.length;
      pos.value = nodeEnd;
      if (end <= nodeStart || start >= nodeEnd) return null;
      var from = Math.max(start, nodeStart) - nodeStart;
      var to = Math.min(end, nodeEnd) - nodeStart;
      if (from >= to) return null;
      return document.createTextNode(txt.slice(from, to));
    }
    if (node.nodeType !== 1) return null;
    var clone = node.cloneNode(false);
    var children = node.childNodes || [];
    for (var i = 0; i < children.length; i++) {
      var child = cloneNodeTextRange(children[i], start, end, pos);
      if (child) clone.appendChild(child);
    }
    return clone.firstChild ? clone : null;
  }

  /**
   * Turn long inline-only <p> into several <p> (~2 sentences each).
   */
  function reflowLongPlainParagraphs(html) {
    var inHtml = String(html || "").trim();
    var wrap = document.createElement("div");
    wrap.innerHTML = inHtml;
    var MIN_CHARS = 150;
    var MIN_SENTENCES = 4;
    var GROUP = 2;
    var ps = Array.prototype.slice.call(wrap.querySelectorAll("p")).reverse();
    var changed = false;
    for (var i = 0; i < ps.length; i++) {
      var node = ps[i];
      if (!node.parentNode) continue;
      if (!isInlinePhrasingSubtreeOnly(node)) continue;
      var textMap = buildNormalizedTextMap(node);
      if (Array.from(textMap.text).length < MIN_CHARS) continue;
      var sents = splitNormalizedTextIntoSentenceRanges(textMap.text);
      if (sents.length < MIN_SENTENCES) continue;
      var repl = [];
      for (var g = 0; g < sents.length; g += GROUP) {
        var end = Math.min(g + GROUP, sents.length);
        var rawStart = textMap.before[sents[g].start];
        var rawEnd = textMap.after[sents[end - 1].end - 1];
        var np = cloneParagraphTextRange(node, rawStart, rawEnd);
        if (String(np.textContent || "").replace(/\u00a0/g, " ").trim()) {
          repl.push(np);
        }
      }
      if (!repl.length) continue;
      var parent = node.parentNode;
      var mark = node.nextSibling;
      parent.removeChild(node);
      for (var j = 0; j < repl.length; j++) {
        parent.insertBefore(repl[j], mark);
      }
      changed = true;
    }
    if (!changed) {
      return { html: inHtml, changed: false };
    }
    var htmlOut = String(wrap.innerHTML || "").trim();
    if (!htmlOut) {
      return { html: inHtml, changed: false };
    }
    return { html: htmlOut, changed: true };
  }

  function normalizeParagraphRows(html) {
    var orig = String(html || "").trim();
    var brp = convertBrAndNewlinesToParagraphs(orig);
    var step = String(brp.html || "").trim();
    var step2 = stripBrTagsToSpace(step);
    var step3 = stripWhitespaceBetweenTags(step2);
    var brStripped = step2 !== step;
    var flow = reflowLongPlainParagraphs(step3);
    return {
      html: String(flow.html || "").trim(),
      brToP: brp.changed || brStripped,
      reflowed: flow.changed,
    };
  }

  function setEditorHTML(html) {
    var ed = getTinyEditor();
    if (!ed) return;
    var body = html || "<p><br></p>";
    if (typeof ed.setContent === "function") {
      ed.setContent(body, { format: "html" });
    }
  }

  function ensureTiny() {
    if (contentEditor) return Promise.resolve(contentEditor);
    if (tinyReady) return tinyReady;
    tinyReady = window.TinyMCE.init({
      selector: "#pf-content",
      uploadUrl: API_UPLOAD_IMG,
      tokenKey: TOKEN_KEY,
      loginUrl: LOGIN_URL,
      onSetup: function (editor) {
        editor.on("input change undo redo SetContent", saveDraftDebounced);
      },
    }).then(function (ed) {
      contentEditor = ed;
      return ed;
    });
    return tinyReady;
  }

  function wireSlugAutofill(nameInput, slugInput) {
    var NS = window.AdminNameSlugCrud;
    if (NS && typeof NS.slugify === "function") {
      var slugTouched = false;
      function onNameInput() {
        if (slugTouched) return;
        slugInput.value = NS.slugify(nameInput.value);
      }
      function onSlugInput() {
        slugTouched = true;
      }
      nameInput.addEventListener("input", onNameInput);
      slugInput.addEventListener("input", onSlugInput);
      return {
        reset: function () {
          slugTouched = false;
        },
      };
    }
    return { reset: function () {} };
  }

  /**
   * Post title ↔ slug: editing the title always re-slugifies the slug (including when empty).
   * Clearing the slug clears the title; other slug edits never change the title.
   */
  function wirePostTitleSlugSync(titleInput, slugInput) {
    if (!titleInput || !slugInput) {
      return { reset: function () {} };
    }
    function syncSlugFromTitle() {
      slugInput.value = postFormSlugify(titleInput.value);
    }
    function onTitleInput() {
      syncSlugFromTitle();
    }
    function onSlugInput() {
      if (!String(slugInput.value || "").trim()) {
        titleInput.value = "";
      }
    }
    titleInput.addEventListener("input", onTitleInput);
    titleInput.addEventListener("change", onTitleInput);
    titleInput.addEventListener("compositionend", onTitleInput);
    slugInput.addEventListener("input", onSlugInput);
    if (!String(slugInput.value || "").trim()) {
      syncSlugFromTitle();
    }
    return { reset: function () {} };
  }

  function extractFirstH1Text(html) {
    try {
      var div = document.createElement("div");
      div.innerHTML = String(html || "");
      var h1 = div.querySelector("h1");
      return h1 ? String(h1.textContent || "").trim() : "";
    } catch (e) {
      return "";
    }
  }

  function setImagePreview(imageUrl) {
    var wrap = el("pf-image-preview-wrap");
    var img = el("pf-image-preview");
    if (!wrap || !img) return;
    wrap.classList.add("position-relative", "d-inline-block");
    if (imageUrl) {
      img.src = imageUrl;
      img.style.display = "block";
      wrap.classList.remove("d-none");
      return;
    }
    img.removeAttribute("src");
    wrap.classList.add("d-none");
  }

  function setImageSourceHint(sourceUrl) {
    var wrap = el("pf-image-source-wrap");
    var link = el("pf-image-source-link");
    if (!wrap || !link) return;
    var url = String(sourceUrl || "").trim();
    if (!url) {
      link.removeAttribute("href");
      link.textContent = "";
      wrap.classList.add("d-none");
      return;
    }
    link.href = url;
    link.textContent = url;
    wrap.classList.remove("d-none");
  }

  function clearSelectedImage() {
    var imageInput = el("pf-image-file");
    var imageUrlInput = el("pf-image-url");
    var imageHiddenInput = el("pf-image");
    revokeCoverPreviewBlobURL();
    if (imageInput) imageInput.value = "";
    if (imageUrlInput) imageUrlInput.value = "";
    if (imageHiddenInput) imageHiddenInput.value = "";
    setImageSourceHint("");
    setImagePreview("");
    clearCoverImageOrigin();
    saveDraftDebounced();
  }

  function ensureImageRemoveButton() {
    var wrap = el("pf-image-preview-wrap");
    if (!wrap || wrap.querySelector(".pf-image-remove-btn")) return;
    wrap.classList.add("position-relative", "d-inline-block");
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className =
      "btn btn-sm btn-secondary rounded-circle pf-image-remove-btn position-absolute top-0 end-0 m-1 d-inline-flex align-items-center justify-content-center";
    btn.style.width = "20px";
    btn.style.height = "20px";
    btn.style.padding = "0";
    btn.setAttribute("aria-label", "Remove cover image");
    btn.innerHTML = '<i class="bi bi-x-lg" aria-hidden="true"></i>';
    btn.addEventListener("click", function () {
      clearSelectedImage();
    });
    wrap.appendChild(btn);
  }

  function currentImageMode() {
    var urlRadio = el("pf-image-mode-url");
    var webRadio = el("pf-image-mode-web-search");
    var genRadio = el("pf-image-mode-generate");
    if (genRadio && genRadio.checked) return "generate";
    if (webRadio && webRadio.checked) return "web_search";
    return urlRadio && urlRadio.checked ? "url" : "upload";
  }

  function isHttpImageURL(v) {
    return /^https?:\/\//i.test(String(v || "").trim());
  }

  /** URL in the Image URL field (kept when switching mode so user can return without retyping). */
  function coverImageURLFieldValue() {
    var urlInput = el("pf-image-url");
    return urlInput ? String(urlInput.value || "").trim() : "";
  }

  var coverPreviewBlobURL = null;
  function revokeCoverPreviewBlobURL() {
    if (coverPreviewBlobURL) {
      try {
        URL.revokeObjectURL(coverPreviewBlobURL);
      } catch (e) {}
      coverPreviewBlobURL = null;
    }
  }

  /** Single place to update preview from hidden file, URL field, or upload selection. */
  function refreshCoverImagePreview() {
    var mode = currentImageMode();
    var hiddenEl = el("pf-image");
    var hidden = hiddenEl ? String(hiddenEl.value || "").trim() : "";
    var urlV = coverImageURLFieldValue();
    var fallbackHttp = isHttpImageURL(urlV) ? urlV : "";
    var fileInput = el("pf-image-file");
    var f =
      fileInput && fileInput.files && fileInput.files[0]
        ? fileInput.files[0]
        : null;

    if (mode === "url") {
      setImageSourceHint("");
      var u = fallbackHttp || (isHttpImageURL(hidden) ? hidden : "");
      setImagePreview(u);
      return;
    }

    if (mode === "generate") {
      setImageSourceHint("");
      if (hidden) {
        setImagePreview(hidden);
        return;
      }
      if (fallbackHttp) {
        setImagePreview(fallbackHttp);
        return;
      }
      setImagePreview("");
      return;
    }

    if (mode === "web_search") {
      if (hidden) {
        setImagePreview(hidden);
        return;
      }
      setImagePreview("");
      return;
    }

    // upload
    setImageSourceHint("");
    if (f) {
      revokeCoverPreviewBlobURL();
      try {
        coverPreviewBlobURL = URL.createObjectURL(f);
        setImagePreview(coverPreviewBlobURL);
      } catch (e) {
        setImagePreview(hidden || fallbackHttp || "");
      }
      return;
    }
    if (hidden) {
      setImagePreview(hidden);
      return;
    }
    if (fallbackHttp) {
      setImagePreview(fallbackHttp);
      return;
    }
    setImagePreview("");
  }

  function applyImageMode(mode) {
    var uploadWrap = el("pf-image-upload-wrap");
    var urlWrap = el("pf-image-url-wrap");
    var webWrap = el("pf-image-web-search-wrap");
    var genWrap = el("pf-image-generate-wrap");
    var fileInput = el("pf-image-file");
    var urlInput = el("pf-image-url");
    var webButton = el("pf-image-web-search-btn");
    var genProvider = el("pf-image-ai-provider");
    var genPrompt = el("pf-image-ai-prompt");
    var genButton = el("pf-image-ai-generate-btn");
    var modeUpload = el("pf-image-mode-upload");
    var modeURL = el("pf-image-mode-url");
    var modeWebSearch = el("pf-image-mode-web-search");
    var modeGenerate = el("pf-image-mode-generate");
    var m =
      mode === "url"
        ? "url"
        : mode === "generate"
        ? "generate"
        : mode === "web_search"
        ? "web_search"
        : "upload";

    if (modeUpload) modeUpload.checked = m === "upload";
    if (modeURL) modeURL.checked = m === "url";
    if (modeWebSearch) modeWebSearch.checked = m === "web_search";
    if (modeGenerate) modeGenerate.checked = m === "generate";
    if (uploadWrap) uploadWrap.classList.toggle("d-none", m !== "upload");
    if (urlWrap) urlWrap.classList.toggle("d-none", m !== "url");
    if (webWrap) webWrap.classList.toggle("d-none", m !== "web_search");
    if (genWrap) genWrap.classList.toggle("d-none", m !== "generate");
    if (fileInput) fileInput.disabled = m !== "upload";
    // readOnly (not disabled) so the URL is not dropped by the browser when switching modes.
    if (urlInput) {
      urlInput.readOnly = m !== "url";
      urlInput.disabled = false;
      urlInput.classList.toggle("bg-light", m !== "url");
      urlInput.classList.toggle("text-muted", m !== "url");
    }
    if (webButton) webButton.disabled = m !== "web_search";
    if (genProvider) genProvider.disabled = m !== "generate";
    if (genPrompt) genPrompt.disabled = m !== "generate";
    if (genButton) genButton.disabled = m !== "generate";
    refreshCoverImagePreview();
  }

  function setImageGenerateBusy(busy) {
    var btn = el("pf-image-ai-generate-btn");
    var spin = el("pf-image-ai-spinner");
    var icon = el("pf-image-ai-icon");
    var label = el("pf-image-ai-label");
    var webBtn = el("pf-image-web-search-btn");
    var webSpin = el("pf-image-web-search-spinner");
    var webIcon = el("pf-image-web-search-icon");
    var webLabel = el("pf-image-web-search-label");
    var contentBtn = el("pf-ai-generate-image-btn");
    var contentSpin = el("pf-ai-image-spinner");
    var contentIcon = el("pf-ai-image-icon");
    var contentLabel = el("pf-ai-generate-image-label");

    if (btn) {
      btn.disabled = !!busy;
      if (spin) spin.classList.toggle("d-none", !busy);
      if (icon) icon.classList.toggle("d-none", !!busy);
      if (label) label.textContent = busy ? "Membuat..." : "Buat";
    }
    if (webBtn) {
      webBtn.disabled = !!busy;
      if (webSpin) webSpin.classList.toggle("d-none", !busy);
      if (webIcon) webIcon.classList.toggle("d-none", !!busy);
      if (webLabel) webLabel.textContent = busy ? "Mencari..." : "Cari";
    }
    if (contentBtn) {
      contentBtn.disabled = !!busy;
      if (contentSpin) contentSpin.classList.toggle("d-none", !busy);
      if (contentIcon) contentIcon.classList.toggle("d-none", !!busy);
      if (contentLabel)
        contentLabel.textContent = busy ? "Membuat..." : "Buat gambar";
    }
  }

  function hasAnyAIKey(row) {
    if (!row) return false;
    var gemini = String(row.gemini || "").trim();
    var openai = String(row.openai || "").trim();
    var groq = String(row.groq || "").trim();
    var claude = String(row.claude || "").trim();
    return !!(gemini || openai || groq || claude);
  }

  async function syncAIKeyWarning() {
    var warning = el("pf-ai-key-warning");
    if (!warning) return;
    try {
      var res = await fetchAuth(API_AI_SETTING, { method: "GET" });
      if (!res.ok) return;
      var data = await res.json().catch(function () {
        return null;
      });
      if (hasAnyAIKey(data)) {
        warning.classList.add("d-none");
      } else {
        warning.classList.remove("d-none");
      }
    } catch (err) {
      if (err && err.message === "unauthorized") return;
    }
  }

  function setAIControlsEnabled(enabled) {
    var wrap = el("pf-ai-controls");
    var provider = el("pf-ai-provider");
    var promptVariant = el("pf-ai-prompt-variant");
    var button = el("pf-ai-generate-btn");
    var imageButton = el("pf-ai-generate-image-btn");
    var on = !!enabled;
    if (wrap) wrap.classList.toggle("d-none", !on);
    if (provider) provider.disabled = !on;
    if (promptVariant) promptVariant.disabled = !on;
    if (button) button.disabled = !on;
    if (imageButton) imageButton.disabled = !on;
  }

  function sortCategoriesForSelect(categories) {
    var list = (categories || []).slice();
    var childrenOf = Object.create(null);
    var roots = [];
    list.forEach(function (c) {
      if (!c || c.id == null) return;
      var pid = c.parent_id != null ? Number(c.parent_id) : 0;
      if (pid > 0) {
        var key = String(pid);
        if (!childrenOf[key]) childrenOf[key] = [];
        childrenOf[key].push(c);
      } else {
        roots.push(c);
      }
    });
    function byName(a, b) {
      return String(a.name || "").localeCompare(String(b.name || ""), undefined, {
        sensitivity: "base",
      });
    }
    roots.sort(byName);
    var out = [];
    roots.forEach(function (r) {
      out.push(r);
      var kids = childrenOf[String(r.id)] || [];
      kids.sort(byName);
      kids.forEach(function (ch) {
        out.push(ch);
      });
    });
    return out;
  }

  function categoryOptionLabel(c) {
    if (!c) return "";
    var pid = c.parent_id != null ? Number(c.parent_id) : 0;
    var name = c.name || "#" + c.id;
    return pid > 0 ? "— " + name : name;
  }

  function fillCategoryOptions(selectEl, categories, selectedId) {
    if (!selectEl) return;
    selectEl.innerHTML = "";
    var ph = document.createElement("option");
    ph.value = "";
    ph.textContent = "Pilih kategori...";
    ph.disabled = true;
    ph.selected = !selectedId;
    selectEl.appendChild(ph);
    sortCategoriesForSelect(categories).forEach(function (c) {
      if (!c || c.id == null) return;
      var o = document.createElement("option");
      o.value = String(c.id);
      o.textContent = categoryOptionLabel(c);
      if (selectedId && Number(c.id) === Number(selectedId)) {
        o.selected = true;
        ph.selected = false;
      }
      selectEl.appendChild(o);
    });
    var sid =
      selectedId != null && selectedId !== "" ? Number(selectedId) : NaN;
    if (!isNaN(sid) && sid > 0) {
      var found = Array.prototype.some.call(
        selectEl.options || [],
        function (opt) {
          return Number(opt.value) === sid;
        }
      );
      if (!found) {
        var o2 = document.createElement("option");
        o2.value = String(sid);
        o2.textContent = "Kategori #" + sid + " (tidak ada di daftar)";
        o2.selected = true;
        ph.selected = false;
        selectEl.appendChild(o2);
      }
    }
  }

  function fillTagOptions(selectEl, tags) {
    if (!selectEl) return;
    selectEl.innerHTML = "";
    (tags || []).forEach(function (t) {
      if (!t || t.id == null) return;
      var o = document.createElement("option");
      o.value = String(t.id);
      o.textContent = t.name || t.slug || "#" + t.id;
      selectEl.appendChild(o);
    });
  }

  async function loadCategoriesAndTags(selectedCatId, selectedTagIds) {
    destroyTagTomSelect();

    var catSel = el("pf-category");
    var tagSel = el("pf-tags");
    if (catSel) {
      catSel.innerHTML = '<option value="">Memuat kategori...</option>';
      catSel.disabled = true;
    }
    if (tagSel) {
      tagSel.innerHTML = '<option value="">Memuat tag...</option>';
    }

    var cats = [];
    var tags = [];
    try {
      var rc = await fetchAuth(API_CAT + "?page=1&size=500", {
        method: "GET",
      });
      if (rc.ok) {
        var jc = await rc.json();
        cats = jc.items || [];
      }
      var rt = await fetchAuth(API_TAGS + "?page=1&size=100", {
        method: "GET",
      });
      if (rt.ok) {
        var jt = await rt.json();
        tags = jt.items || [];
      }
    } catch (e) {}

    if (catSel) {
      catSel.disabled = false;
      fillCategoryOptions(catSel, cats, selectedCatId);
    }
    if (tagSel) {
      fillTagOptions(tagSel, tags);
    }

    if (tagSel) {
      tsTags = new TomSelect(tagSel, tomSelectTagOptions());
    }
    if (tsTags && selectedTagIds && selectedTagIds.length) {
      tsTags.setValue(selectedTagIds.map(String));
    }
  }

  document.addEventListener("DOMContentLoaded", function () {
    if (!AC.getToken(TOKEN_KEY)) {
      window.location.href = LOGIN_URL;
      return;
    }

    ensureImageRemoveButton();

    if (mode === "edit" && !postId) {
      setFormError("ID post tidak ditemukan. Kembali ke daftar post.");
      return;
    }

    slugAutofill = wirePostTitleSlugSync(el("pf-title"), el("pf-slug"));

    if (el("pf-tag-modal-name") && el("pf-tag-modal-slug")) {
      tagModalSlugAutofill = wireSlugAutofill(
        el("pf-tag-modal-name"),
        el("pf-tag-modal-slug")
      );
    }

    var tagModalEl = el("pf-tag-modal");

    document.addEventListener(
      "click",
      function (e) {
        var btn =
          e.target && e.target.closest
            ? e.target.closest(".pf-ts-no-results-add")
            : null;
        if (!btn) return;
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
        openNewTagModal(name);
      },
      true
    );

    var tagModalForm = el("pf-tag-modal-form");
    if (tagModalForm) {
      tagModalForm.addEventListener("submit", async function (e) {
        e.preventDefault();
        var errBox = el("pf-tag-modal-error");
        AC.setInlineError(errBox, null);
        var name = el("pf-tag-modal-name")
          ? String(el("pf-tag-modal-name").value || "").trim()
          : "";
        var slug = el("pf-tag-modal-slug")
          ? String(el("pf-tag-modal-slug").value || "").trim()
          : "";
        if (name.length < 3) {
          AC.setInlineError(errBox, "Nama harus minimal 3 karakter.");
          return;
        }
        if (slug.length < 3 || !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug)) {
          AC.setInlineError(
            errBox,
            "Slug: gunakan huruf kecil, angka, dan tanda hubung saja (minimal 3 karakter)."
          );
          return;
        }
        if (!tsTags) {
          AC.setInlineError(
            errBox,
            "Tag masih dimuat. Tunggu sebentar lalu coba lagi."
          );
          return;
        }
        setTagModalBusy(true);
        try {
          var res = await fetchJSON("POST", API_TAGS, {
            name: name,
            slug: slug,
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
            AC.setInlineError(errBox, "Respons server tidak valid.");
            return;
          }
          var sid = String(tid);
          tsTags.addOption({ value: sid, text: tname });
          tsTags.addItem(sid);
          saveDraftDebounced();
          if (tagModalEl) {
            bootstrap.Modal.getOrCreateInstance(tagModalEl).hide();
          }
          if (typeof AC.showSuccessToast === "function") {
            AC.showSuccessToast("Tag berhasil dibuat dan dipilih.");
          }
        } catch (err) {
          if (err && err.message === "unauthorized") return;
          AC.setInlineError(errBox, "Terjadi gangguan jaringan.");
        } finally {
          setTagModalBusy(false);
        }
      });
    }

    ensureTiny()
      .then(function () {
        syncAIKeyWarning();
        setAIControlsEnabled(true);
        return (async function init() {
          try {
            if (mode === "edit") {
              var res = await fetchAuth(postUrl(postId), { method: "GET" });
              if (!res.ok) {
                setFormError(
                  await AC.responseErrorMessage(res, "Gagal memuat post")
                );
                return;
              }
              var row = await res.json();
              el("pf-title").value = row && row.title ? row.title : "";
              el("pf-slug").value = row && row.slug ? row.slug : "";
              if (el("pf-slug") && el("pf-title")) {
                var sLoaded = String(el("pf-slug").value || "").trim();
                var tLoaded = String(el("pf-title").value || "").trim();
                if (!sLoaded && tLoaded) {
                  el("pf-slug").value = postFormSlugify(tLoaded);
                }
              }
              var rowSt = row && row.status ? String(row.status) : "";
              el("pf-status").value =
                rowSt === "published"
                  ? "published"
                  : rowSt === "scheduled"
                  ? "scheduled"
                  : "draft";
              if (el("pf-publish-at")) {
                el("pf-publish-at").value = isoToDatetimeLocal(
                  row && row.publish_at ? row.publish_at : ""
                );
              }
              syncPublishAtWrap();
              el("pf-source").value = row && row.source ? row.source : "";
              if (el("pf-include-source")) {
                el("pf-include-source").checked =
                  row && row.include_source !== false;
              }
              if (el("pf-meta-desc")) {
                el("pf-meta-desc").value =
                  row && row.meta_description != null
                    ? String(row.meta_description)
                    : "";
              }
              if (el("pf-meta-kw")) {
                el("pf-meta-kw").value =
                  row && row.meta_keywords != null
                    ? String(row.meta_keywords)
                    : "";
              }
              el("pf-image").value = row && row.image ? row.image : "";
              if (el("pf-image-url")) {
                el("pf-image-url").value = isHttpImageURL(el("pf-image").value)
                  ? el("pf-image").value
                  : "";
              }
              setCoverImageOrigin(
                row && row.image_origin ? row.image_origin : "",
                row && row.image_origin_detail ? row.image_origin_detail : ""
              );
              applyImageMode(
                isHttpImageURL(el("pf-image").value) ? "url" : "upload"
              );

              var catId =
                row && row.category_id != null ? row.category_id : null;
              var tagIds = (row && row.tags ? row.tags : [])
                .map(function (t) {
                  return t && t.id != null ? t.id : null;
                })
                .filter(function (x) {
                  return x != null;
                });

              await loadCategoriesAndTags(catId, tagIds);

              if (el("pf-author") && row && row.author_id != null) {
                el("pf-author").value = String(row.author_id);
              }

              setEditorHTML(
                row && row.content ? String(row.content) : "<p><br></p>"
              );
            } else {
              await loadCategoriesAndTags(null, []);
              el("pf-image").value = "";
              if (el("pf-image-url")) el("pf-image-url").value = "";
              applyImageMode("upload");
              setEditorHTML("<p><br></p>");
              syncPublishAtWrap();
            }
          } catch (e) {
            if (e && e.message === "unauthorized") return;
            setFormError("Terjadi gangguan jaringan.");
          } finally {
            if (mode === "edit") {
              try {
                clearDraft();
              } catch (e2) {}
            } else {
              var draftAfterLoad = readDraft();
              if (hasMeaningfulDraft(draftAfterLoad)) {
                var restoredAfterLoad = applyDraft(draftAfterLoad);
                if (
                  restoredAfterLoad &&
                  typeof AC.showSuccessToast === "function"
                ) {
                  AC.showSuccessToast("Draf yang belum disimpan berhasil dipulihkan.");
                }
              }
            }
            if (tsTags) {
              tsTags.on("change", saveDraftDebounced);
            }
            updateTitleAndMetaCounts();
          }
        })();
      })
      .catch(function (err) {
        console.error(err);
        setFormError(
          "Editor teks gagal dimuat. Periksa koneksi Anda lalu muat ulang."
        );
      });

    var form = el("form-post-admin");
    if (!form) return;

    updateTitleAndMetaCounts();

    var aiEnableInput = el("pf-ai-enable");
    if (aiEnableInput) {
      aiEnableInput.checked = true;
      setAIControlsEnabled(aiEnableInput.checked);
      aiEnableInput.addEventListener("change", function () {
        setAIControlsEnabled(aiEnableInput.checked);
        saveDraftDebounced();
      });
    }

    var titleInput = el("pf-title");
    if (titleInput) {
      titleInput.addEventListener("input", function () {
        updateTitleAndMetaCounts();
        saveDraftDebounced();
      });
    }
    var titleRegenerateBtn = el("pf-title-regenerate-btn");
    if (titleRegenerateBtn) {
      titleRegenerateBtn.addEventListener("click", async function () {
        clearFormError();
        var currentTitle = titleInput ? String(titleInput.value || "").trim() : "";
        if (currentTitle.length < 2) {
          setFormError("Masukkan judul minimal 2 karakter untuk dibuat ulang.");
          return;
        }
        var aiProviderEl = el("pf-ai-provider");
        var provider = aiProviderEl ? String(aiProviderEl.value || "auto") : "auto";
        setTitleRegenerateBusy(true);
        try {
          var res = await fetchJSON("POST", API_SUGGEST_TITLE, {
            keyword: currentTitle,
            count: 1,
            provider: provider,
          });
          var data = {};
          try {
            data = await res.json();
          } catch (e) {}
          if (!res.ok) {
            if (
              window.AdminAILimit &&
              typeof AdminAILimit.showFromResponse === "function" &&
              AdminAILimit.showFromResponse(data)
            ) {
              return;
            }
            setFormError((data && data.error) || "Gagal membuat ulang judul.");
            return;
          }
          var regeneratedTitle = "";
          if (Array.isArray(data && data.titles) && data.titles.length) {
            regeneratedTitle = String(data.titles[0] || "").trim();
          }
          if (!regeneratedTitle && data && data.titles_text != null) {
            regeneratedTitle = String(data.titles_text)
              .split(/\r?\n/)
              .map(function (line) {
                return String(line || "").trim();
              })
              .filter(Boolean)[0] || "";
          }
          if (!regeneratedTitle) {
            setFormError("AI mengembalikan judul kosong.");
            return;
          }
          if (titleInput) {
            titleInput.value = regeneratedTitle;
            titleInput.dispatchEvent(new Event("input", { bubbles: true }));
          }
          if (typeof AC.showSuccessToast === "function") {
            AC.showSuccessToast("Judul berhasil dibuat ulang.");
          }
        } catch (err) {
          if (err && err.message === "unauthorized") return;
          setFormError("Terjadi gangguan jaringan saat membuat ulang judul.");
        } finally {
          setTitleRegenerateBusy(false);
        }
      });
    }
    var slugInput = el("pf-slug");
    if (slugInput) slugInput.addEventListener("input", saveDraftDebounced);
    var statusInput = el("pf-status");
    if (statusInput) {
      statusInput.addEventListener("change", function () {
        syncPublishAtWrap();
        if (statusInput.value === "scheduled") {
          var pa = el("pf-publish-at");
          if (pa) {
            try {
              pa.focus();
            } catch (e) {}
          }
        }
        saveDraftDebounced();
      });
    }
    var publishAtInput = el("pf-publish-at");
    if (publishAtInput) {
      publishAtInput.addEventListener("change", saveDraftDebounced);
    }
    var catInput = el("pf-category");
    if (catInput) catInput.addEventListener("change", saveDraftDebounced);
    var authorSel = el("pf-author");
    if (authorSel) authorSel.addEventListener("change", saveDraftDebounced);
    var sourceInput = el("pf-source");
    if (sourceInput) sourceInput.addEventListener("input", saveDraftDebounced);
    var includeSourceInput = el("pf-include-source");
    if (includeSourceInput)
      includeSourceInput.addEventListener("change", saveDraftDebounced);
    var metaDescInput = el("pf-meta-desc");
    if (metaDescInput) {
      metaDescInput.addEventListener("input", function () {
        updateTitleAndMetaCounts();
        saveDraftDebounced();
      });
    }
    var metaKwInput = el("pf-meta-kw");
    if (metaKwInput) metaKwInput.addEventListener("input", saveDraftDebounced);
    var imageURLInput = el("pf-image-url");
    if (imageURLInput) {
      imageURLInput.addEventListener("input", function () {
        if (currentImageMode() !== "url") return;
        refreshCoverImagePreview();
        saveDraftDebounced();
      });
    }
    var aiProviderInput = el("pf-ai-provider");
    if (aiProviderInput)
      aiProviderInput.addEventListener("change", saveDraftDebounced);
    var aiPromptVariantInput = el("pf-ai-prompt-variant");
    if (aiPromptVariantInput)
      aiPromptVariantInput.addEventListener("change", saveDraftDebounced);
    var imageAIPromptInput = el("pf-image-ai-prompt");
    if (imageAIPromptInput)
      imageAIPromptInput.addEventListener("input", saveDraftDebounced);
    var imageAIProviderInput = el("pf-image-ai-provider");
    if (imageAIProviderInput)
      imageAIProviderInput.addEventListener("change", saveDraftDebounced);

    async function requestAIGeneratedImageURL(purpose) {
      purpose = purpose === "content" ? "content" : "cover";
      clearFormError();
      var title = el("pf-title") ? String(el("pf-title").value || "").trim() : "";
      if (title.length < 3) {
        setFormError(
          "Masukkan judul minimal 3 karakter sebelum membuat gambar."
        );
        return "";
      }
      var prompt = imageAIPromptInput
        ? String(imageAIPromptInput.value || "").trim()
        : "";
      var provider = imageAIProviderInput
        ? String(imageAIProviderInput.value || "auto")
        : "auto";
      setImageGenerateBusy(true);
      var stopImageFakeProgress = null;
      try {
        openAiImageGenLoadingModal();
        if (swalAvailable() && Swal.isVisible()) {
          stopImageFakeProgress = startAiImageFakeProgress();
        }
        var resImg = await fetchJSON("POST", API_GEN_IMAGE, {
          title: title,
          prompt: prompt,
          provider: provider,
          purpose: purpose,
        });
        var dataImg = {};
        try {
          dataImg = await resImg.json();
        } catch (e) {}
        if (stopImageFakeProgress) {
          stopImageFakeProgress();
          stopImageFakeProgress = null;
        }
        if (!resImg.ok) {
          closeAiImageGenSwal();
          setFormError((dataImg && dataImg.error) || "Gagal membuat gambar AI.");
          return "";
        }
        var image = dataImg && dataImg.image ? String(dataImg.image).trim() : "";
        if (!image) {
          closeAiImageGenSwal();
          setFormError("AI mengembalikan gambar kosong.");
          return "";
        }
        updateAiDraftGenLoadingModal(100, "Menerapkan pratinjau gambar...");
        await new Promise(function (r) {
          setTimeout(r, 200);
        });
        closeAiImageGenSwal();
        return image;
      } catch (err) {
        if (stopImageFakeProgress) {
          stopImageFakeProgress();
          stopImageFakeProgress = null;
        }
        closeAiImageGenSwal();
        if (err && err.message === "unauthorized") return;
        setFormError("Terjadi gangguan jaringan saat membuat gambar AI.");
        return "";
      } finally {
        setImageGenerateBusy(false);
      }
    }

    async function runAICoverImageGeneration() {
      var image = await requestAIGeneratedImageURL("cover");
      if (!image) return;
      if (el("pf-image")) el("pf-image").value = image;
      setCoverImageOrigin("ai", "");
      refreshCoverImagePreview();
      if (typeof AC.showSuccessToast === "function") {
        AC.showSuccessToast("Gambar AI berhasil dibuat untuk sampul.");
      }
      saveDraftDebounced();
    }

    async function runWebCoverImageSearch() {
      clearFormError();
      var title = el("pf-title") ? String(el("pf-title").value || "").trim() : "";
      if (title.length < 3) {
        setFormError(
          "Masukkan judul minimal 3 karakter sebelum mencari gambar."
        );
        return;
      }
      var webSourceEl = el("pf-image-web-search-source");
      var webSource = webSourceEl
        ? String(webSourceEl.value || "auto").trim().toLowerCase()
        : "auto";
      if (
        webSource !== "openverse" &&
        webSource !== "wikimedia" &&
        webSource !== "unsplash" &&
        webSource !== "pixabay" &&
        webSource !== "pexels" &&
        webSource !== "wordpress"
      ) {
        webSource = "auto";
      }
      if (
        webImageSearchState.title !== title ||
        webImageSearchState.source !== webSource
      ) {
        webImageSearchState.title = title;
        webImageSearchState.source = webSource;
        webImageSearchState.attempt = 0;
      }
      var attempt = webImageSearchState.attempt;
      var previousImage = el("pf-image")
        ? String(el("pf-image").value || "").trim()
        : "";
      setImageGenerateBusy(true);
      var stopWebFakeProgress = null;
      try {
        openWebImageSearchLoadingModal();
        if (swalAvailable() && Swal.isVisible()) {
          stopWebFakeProgress = startWebImageSearchFakeProgress();
        }
        var resImg = await fetchJSON("POST", API_GEN_IMAGE, {
          title: title,
          source: "web_search",
          purpose: "cover",
          search_attempt: attempt,
          web_search_source: webSource,
          previous_image: previousImage,
        });
        var dataImg = {};
        try {
          dataImg = await resImg.json();
        } catch (e) {}
        if (stopWebFakeProgress) {
          stopWebFakeProgress();
          stopWebFakeProgress = null;
        }
        if (!resImg.ok) {
          closeAiImageGenSwal();
          setFormError((dataImg && dataImg.error) || "Gagal mencari gambar web.");
          return;
        }
        var image = dataImg && dataImg.image ? String(dataImg.image).trim() : "";
        if (!image) {
          closeAiImageGenSwal();
          setFormError("Pencarian gambar web mengembalikan gambar kosong.");
          return;
        }
        if (el("pf-image")) el("pf-image").value = image;
        setImageSourceHint(
          dataImg && dataImg.image_source_url
            ? String(dataImg.image_source_url).trim()
            : ""
        );
        setCoverImageOrigin(
          (dataImg && dataImg.image_origin) || "web_search",
          dataImg && dataImg.image_origin_detail
            ? String(dataImg.image_origin_detail).trim()
            : ""
        );
        webImageSearchState.attempt = attempt + 1;
        updateAiDraftGenLoadingModal(100, "Menerapkan pratinjau gambar...");
        await new Promise(function (r) {
          setTimeout(r, 200);
        });
        closeAiImageGenSwal();
        refreshCoverImagePreview();
        if (typeof AC.showSuccessToast === "function") {
          AC.showSuccessToast("Gambar web berhasil dipilih untuk sampul.");
        }
        saveDraftDebounced();
      } catch (err) {
        if (stopWebFakeProgress) {
          stopWebFakeProgress();
          stopWebFakeProgress = null;
        }
        closeAiImageGenSwal();
        if (err && err.message === "unauthorized") return;
        setFormError("Terjadi gangguan jaringan saat mencari gambar web.");
      } finally {
        setImageGenerateBusy(false);
      }
    }

    async function runAIContentImageGeneration() {
      var image = await requestAIGeneratedImageURL("content");
      if (!image) return;
      try {
        await ensureTiny();
      } catch (e) {
        setFormError("Editor belum siap. Tunggu sebentar atau muat ulang.");
        return;
      }
      var ed = getTinyEditor();
      if (!ed || typeof ed.insertContent !== "function") {
        setFormError("Editor belum siap. Tunggu sebentar atau muat ulang.");
        return;
      }
      var rawHTML = String(ed.getContent() || "").trim();
      var wrap = document.createElement("div");
      wrap.innerHTML = rawHTML || "<p><br></p>";

      var imgWrap = document.createElement("p");
      imgWrap.className = "ai-inline-image-center";
      imgWrap.setAttribute("align", "center");
      var img = document.createElement("img");
      img.className = "ai-inline-image";
      img.setAttribute("align", "center");
      img.setAttribute("src", image);
      img.setAttribute("alt", "Gambar hasil AI");
      img.setAttribute("loading", "lazy");
      imgWrap.appendChild(img);

      var h2s = wrap.querySelectorAll("h2");
      if (h2s.length >= 2) {
        var secondH2 = h2s[1];
        if (secondH2.parentNode) {
          secondH2.parentNode.insertBefore(imgWrap, secondH2.nextSibling);
        } else {
          wrap.appendChild(imgWrap);
        }
      } else {
        // No heading structure detected: place image around the middle of content.
        var blocks = [];
        for (var i = 0; i < wrap.childNodes.length; i++) {
          var node = wrap.childNodes[i];
          if (node && node.nodeType === 1) blocks.push(node);
        }
        if (!blocks.length) {
          wrap.appendChild(imgWrap);
        } else {
          var mid = Math.floor(blocks.length / 2);
          var anchor = blocks[mid] || null;
          if (anchor && anchor.parentNode) {
            anchor.parentNode.insertBefore(imgWrap, anchor);
          } else {
            wrap.appendChild(imgWrap);
          }
        }
      }

      ed.focus();
      ed.setContent(wrap.innerHTML);
      if (typeof AC.showSuccessToast === "function") {
        AC.showSuccessToast("AI image inserted into content.");
      }
      saveDraftDebounced();
    }

    var aiBtn = el("pf-ai-generate-btn");
    if (aiBtn) {
      aiBtn.addEventListener("click", async function () {
        clearFormError();
        var titleEl = el("pf-title");
        var providerEl = el("pf-ai-provider");
        var title = titleEl ? String(titleEl.value || "").trim() : "";
        if (title.length < 3) {
          setFormError(
            "Masukkan judul minimal 3 karakter untuk membuat konten dari template pengaturan AI."
          );
          return;
        }
        var provider = providerEl ? String(providerEl.value || "auto") : "auto";
        var promptVariantEl = el("pf-ai-prompt-variant");
        var promptVariant = promptVariantEl
          ? parseInt(String(promptVariantEl.value || "1"), 10)
          : 1;
        if (isNaN(promptVariant) || promptVariant < 1 || promptVariant > 3) {
          promptVariant = 1;
        }
        var longArticle = promptSuggestsLongArticle(title);
        setAIGenerateBusy(true, longArticle);
        var stopFakeProgress = null;
        try {
          await openAiDraftGenLoadingModal(longArticle);
          if (swalAvailable() && Swal.isVisible()) {
            stopFakeProgress = startAiDraftFakeProgress(longArticle);
          }
          await ensureTiny();
          var res = await fetchJSON("POST", API_GEN_DRAFT, {
            prompt: "",
            title: title,
            provider: provider,
            prompt_content_variant: promptVariant,
            tags: getTagIDs(),
          });
          var data = {};
          try {
            data = await res.json();
          } catch (e) {}
          if (stopFakeProgress) {
            stopFakeProgress();
            stopFakeProgress = null;
          }
          if (!res.ok) {
            closeAiDraftGenSwal();
            if (
              window.AdminAILimit &&
              typeof AdminAILimit.showFromResponse === "function" &&
              AdminAILimit.showFromResponse(data)
            ) {
              return;
            }
            setFormError((data && data.error) || "Gagal membuat konten AI.");
            return;
          }
          var html = data && data.content != null ? String(data.content) : "";
          if (!html.trim()) {
            closeAiDraftGenSwal();
            setFormError("AI mengembalikan konten kosong.");
            return;
          }
          updateAiDraftGenLoadingModal(100, "Menerapkan ke editor...");
          await new Promise(function (r) {
            setTimeout(r, 280);
          });
          closeAiDraftGenSwal();
          setEditorHTML(html);
          if (contentEditor) {
            contentEditor.focus();
          }

          var picked = pickMetaFromGenerateResponse(data);
          var md = picked.md;
          var mkw = picked.mkw;
          var plainFromBody = stripTagsToPlain(html);
          if (!md) {
            md = metaDescFallbackFromPlain(plainFromBody, title, 160);
          }
          if (!mkw) {
            mkw = tagKeywordsFallbackCSV();
          }
          var metaDescEl = el("pf-meta-desc");
          var metaKwEl = el("pf-meta-kw");
          if (metaDescEl) {
            metaDescEl.value = md;
            updateTitleAndMetaCounts();
          }
          if (metaKwEl) {
            metaKwEl.value = mkw;
          }

          // If title is still empty, use first H1 from generated HTML (legacy; API normally omits H1).
          var titleInput = el("pf-title");
          if (titleInput && !String(titleInput.value || "").trim()) {
            var h1Text = extractFirstH1Text(html);
            if (h1Text) {
              titleInput.value = h1Text;
              if (mode === "create") {
                titleInput.dispatchEvent(new Event("input", { bubbles: true }));
              } else {
                updateTitleAndMetaCounts();
              }
            }
          }
          saveDraftDebounced();
          if (data.ai_requests != null) {
            showAiGenerateStatsAlert(data.ai_requests, data.word_count);
          }
        } catch (err) {
          if (stopFakeProgress) {
            stopFakeProgress();
            stopFakeProgress = null;
          }
          closeAiDraftGenSwal();
          if (err && err.message === "unauthorized") return;
          setFormError("Terjadi gangguan jaringan saat membuat konten AI.");
        } finally {
          setAIGenerateBusy(false, false);
        }
      });
    }

    var imageInput = el("pf-image-file");
    var imageModeUploadInput = el("pf-image-mode-upload");
    var imageModeURLInput = el("pf-image-mode-url");
    var imageModeWebSearchInput = el("pf-image-mode-web-search");
    if (imageModeUploadInput) {
      imageModeUploadInput.addEventListener("change", function () {
        if (imageModeUploadInput.checked) {
          applyImageMode("upload");
          saveDraftDebounced();
        }
      });
    }
    if (imageModeURLInput) {
      imageModeURLInput.addEventListener("change", function () {
        if (imageModeURLInput.checked) {
          applyImageMode("url");
          saveDraftDebounced();
        }
      });
    }
    if (imageModeWebSearchInput) {
      imageModeWebSearchInput.addEventListener("change", function () {
        if (imageModeWebSearchInput.checked) {
          applyImageMode("web_search");
          saveDraftDebounced();
        }
      });
    }
    var imageModeGenerateInput = el("pf-image-mode-generate");
    if (imageModeGenerateInput) {
      imageModeGenerateInput.addEventListener("change", function () {
        if (imageModeGenerateInput.checked) {
          applyImageMode("generate");
          saveDraftDebounced();
        }
      });
    }
    var imageWebSearchSource = el("pf-image-web-search-source");
    if (imageWebSearchSource) {
      imageWebSearchSource.addEventListener("change", function () {
        webImageSearchState.source = "";
        webImageSearchState.attempt = 0;
        saveDraftDebounced();
      });
    }
    var imageAIGenerateBtn = el("pf-image-ai-generate-btn");
    if (imageAIGenerateBtn) {
      imageAIGenerateBtn.addEventListener("click", runAICoverImageGeneration);
    }
    var imageWebSearchBtn = el("pf-image-web-search-btn");
    if (imageWebSearchBtn) {
      imageWebSearchBtn.addEventListener("click", runWebCoverImageSearch);
    }

    var aiGenerateImageBtn = el("pf-ai-generate-image-btn");
    if (aiGenerateImageBtn) {
      aiGenerateImageBtn.addEventListener("click", runAIContentImageGeneration);
    }

    var normalizeRowsBtn = el("pf-content-normalize-rows-btn");
    if (normalizeRowsBtn) {
      normalizeRowsBtn.addEventListener("click", async function () {
        clearFormError();
        try {
          await ensureTiny();
        } catch (e) {
          setFormError("Editor belum siap. Tunggu sebentar atau muat ulang.");
          return;
        }
        var ed = getTinyEditor();
        if (!ed) {
          setFormError("Editor belum siap. Tunggu sebentar atau muat ulang.");
          return;
        }
        var raw = String(ed.getContent() || "").trim();
        if (!raw) {
          if (typeof AC.showSuccessToast === "function") {
            AC.showSuccessToast("Tidak ada yang perlu dinormalisasi.");
          }
          return;
        }
        var result = normalizeParagraphRows(raw);
        if (!result.reflowed && !result.brToP) {
          if (typeof AC.showSuccessToast === "function") {
            AC.showSuccessToast(
              "Tidak ada perubahan: tidak ada baris lunak untuk diubah menjadi &lt;p&gt;, dan tidak ada paragraf yang memenuhi aturan pemecahan."
            );
          }
          return;
        }
        setEditorHTML(result.html);
        ed.focus();
        saveDraftDebounced();
        if (typeof AC.showSuccessToast === "function") {
          var parts = [];
          if (result.brToP) {
            parts.push(
              "mengubah baris lunak (&lt;br&gt;/baris baru) menjadi blok &lt;p&gt; terpisah"
            );
          }
          if (result.reflowed) {
            parts.push(
              "memecah paragraf panjang menjadi blok lebih pendek (~2 kalimat per blok)"
            );
          }
          AC.showSuccessToast(
            "Normalisasi baris: " + parts.join(" dan ") + "."
          );
        }
      });
    }
    if (imageInput) {
      imageInput.addEventListener("change", function () {
        var f =
          imageInput.files && imageInput.files[0] ? imageInput.files[0] : null;
        if (!f) return;
        applyImageMode("upload");
        refreshCoverImagePreview();
        saveDraftDebounced();
      });
    }

    window.addEventListener("beforeunload", saveDraftNow);

    form.addEventListener("submit", async function (e) {
      e.preventDefault();
      clearFormError();

      var sessionUid = getAuthorId();
      if (!sessionUid) {
        setFormError("Sesi tidak valid. Silakan masuk lagi.");
        return;
      }
      var authorId = resolveAuthorIdForPayload();
      if (!authorId) {
        setFormError(
          isAdminRole()
            ? "Pilih penulis terlebih dulu."
            : "Sesi tidak valid. Silakan masuk lagi."
        );
        return;
      }

      var catId = parseInt(el("pf-category").value, 10);
      if (isNaN(catId) || catId <= 0) {
        setFormError("Pilih kategori terlebih dulu.");
        return;
      }
      var tagIds = getTagIDs();

      var slugVal = el("pf-slug").value.trim();
      if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slugVal)) {
        setFormError("Slug: gunakan huruf kecil, angka, dan tanda hubung saja.");
        return;
      }

      try {
        await ensureTiny();
      } catch (e) {
        setFormError("Editor belum siap. Tunggu sebentar atau muat ulang.");
        return;
      }
      var contentHtml = htmlContent();
      contentHtml = normalizeContentImagesCentered(contentHtml);
      if (contentHtml.replace(/<[^>]+>/g, "").trim().length < 10) {
        setFormError("Isi harus berisi minimal 10 karakter teks.");
        return;
      }

      var src = el("pf-source").value.trim();
      var includeSource =
        el("pf-include-source") && el("pf-include-source").checked;
      var imagePath = el("pf-image").value.trim();
      var imageURL = el("pf-image-url") ? el("pf-image-url").value.trim() : "";
      var selectedImage =
        imageInput && imageInput.files && imageInput.files[0]
          ? imageInput.files[0]
          : null;
      var imageMode = currentImageMode();

      // Fallback: if URL is filled with valid http(s), prefer it even when radio state is stale.
      if (imageURL && isHttpImageURL(imageURL)) {
        imageMode = "url";
      }

      if (imageMode === "url") {
        if (!imageURL) {
          setFormError("Isi URL gambar atau ubah ke mode unggah file.");
          return;
        }
        if (!isHttpImageURL(imageURL)) {
          setFormError("URL gambar harus diawali http:// atau https://");
          return;
        }
        imagePath = imageURL;
        el("pf-image").value = imagePath;
      } else if (imageMode === "generate") {
        if (!imagePath) {
          setFormError(
            "Klik Buat pada mode Buat AI untuk membuat gambar sampul."
          );
          return;
        }
      } else if (imageMode === "web_search") {
        if (!imagePath) {
          setFormError(
            "Klik Cari pada mode Cari web untuk mengambil gambar sampul."
          );
          return;
        }
      } else if (selectedImage) {
        var upRes = await uploadImageFile(selectedImage);
        if (!upRes.ok) {
          setFormError(
            await AC.responseErrorMessage(upRes, "Gagal mengunggah gambar")
          );
          return;
        }
        var upJSON = await upRes.json();
        imagePath = upJSON && upJSON.image ? String(upJSON.image) : "";
        el("pf-image").value = imagePath;
      } else if (!imagePath) {
        setFormError(
          "Gambar sampul wajib diisi. Unggah file atau isi URL gambar sebelum menyimpan."
        );
        return;
      }

      var postStatus = el("pf-status").value;
      if (postStatus === "scheduled") {
        var paIso = datetimeLocalToISO();
        if (!paIso) {
          setFormError(
            "Post terjadwal memerlukan tanggal dan waktu terbit."
          );
          return;
        }
      }

      var payload = {
        title: el("pf-title").value.trim(),
        slug: el("pf-slug").value.trim(),
        content: contentHtml,
        status: postStatus,
        author_id: authorId,
        category_id: catId,
        tags: tagIds,
      };
      var paOut = publishAtForPayload();
      payload.publish_at = postStatus === "scheduled" && paOut ? paOut : null;
      if (imagePath) payload.image = imagePath;
      else if (mode === "edit") payload.image = null;
      if (coverImageOrigin.origin) {
        payload.image_origin = coverImageOrigin.origin;
        if (coverImageOrigin.detail) {
          payload.image_origin_detail = coverImageOrigin.detail;
        }
      }
      if (src) payload.source = src;
      else if (mode === "edit") payload.source = null;
      payload.include_source = !!includeSource;

      var metaDesc = el("pf-meta-desc")
        ? String(el("pf-meta-desc").value || "").trim()
        : "";
      var metaKw = el("pf-meta-kw")
        ? String(el("pf-meta-kw").value || "").trim()
        : "";
      payload.meta_description = metaDesc.length ? metaDesc : null;
      payload.meta_keywords = metaKw.length ? metaKw : null;

      setSubmitBusy(true);

      try {
        if (mode === "create") {
          var resC = await fetchJSON("POST", API_POSTS, payload);
          if (!resC.ok) {
            setFormError(
              await AC.responseErrorMessage(resC, "Gagal menyimpan post")
            );
            return;
          }
          try {
            allowDraftPersist = false;
            clearDraft();
            sessionStorage.setItem(
              "tirtayasa_post_saved_flash",
              JSON.stringify({ action: "created" })
            );
          } catch (x) {}
          window.location.href = "/admin/posts";
          return;
        }

        var title = el("pf-title").value.trim();
        var slug = el("pf-slug").value.trim();
        var status = el("pf-status").value;
        if (status === "scheduled") {
          var paIso2 = datetimeLocalToISO();
          if (!paIso2) {
            setFormError(
              "Post terjadwal memerlukan tanggal dan waktu terbit."
            );
            return;
          }
        }
        var putBody = {
          title: title,
          slug: slug,
          content: contentHtml,
          status: status,
          author_id: authorId,
          category_id: catId,
          tags: tagIds,
        };
        var paOut2 = publishAtForPayload();
        putBody.publish_at = status === "scheduled" && paOut2 ? paOut2 : null;
        if (imagePath) putBody.image = imagePath;
        else putBody.image = null;
        if (src) putBody.source = src;
        else putBody.source = null;
        putBody.include_source = !!includeSource;
        putBody.meta_description = metaDesc.length ? metaDesc : null;
        putBody.meta_keywords = metaKw.length ? metaKw : null;

        var resU = await fetchJSON("PUT", postUrl(postId), putBody);
        if (!resU.ok) {
          setFormError(
            await AC.responseErrorMessage(resU, "Gagal memperbarui post")
          );
          return;
        }
        try {
          allowDraftPersist = false;
          clearDraft();
          sessionStorage.setItem(
            "tirtayasa_post_saved_flash",
            JSON.stringify({ action: "updated" })
          );
        } catch (x) {}
        window.location.href = "/admin/posts";
      } catch (err) {
        if (err && err.message === "unauthorized") return;
        setFormError("Terjadi gangguan jaringan. Coba lagi.");
      } finally {
        setSubmitBusy(false);
      }
    });
  });
})();
