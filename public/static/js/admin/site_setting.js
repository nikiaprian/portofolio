(function () {
  "use strict";

  var API = "/api/v1/admin/site-setting";
  var URL_RECYCLE_RUN_API =
    "/api/v1/admin/site-setting/post-date-url-recycle/run";
  var UPLOAD_API = "/api/v1/admin/site-setting/upload-image";
  var SCRIPTS_API = "/api/v1/admin/site-setting/scripts";
  var CLOUD_API = "/api/v1/admin/site-cloud-storage";
  var WEB_IMAGE_API = "/api/v1/admin/site-web-image-providers";
  var TOKEN_KEY = "tirtayasa_token";
  var ADMIN_THEME_CACHE_KEY = "tirtayasa_admin_theme";
  function loginRedirectUrl() {
    try {
      if (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL)
        return window.TIRTAYASA_LOGIN_URL;
    } catch (e) {}
    return "/login";
  }

  /** Aligned with entity.SiteColorPreset (non-custom presets). */
  var COLOR_PRESETS = {
    default: {
      text_primary: "#1f2937",
      text_secondary: "#64748b",
      background_primary: "#ffffff",
      background_secondary: "#ffffff",
      primary_button: "#0d6efd",
      secondary_button: "#6c757d",
      accent_color: "#c62828",
      border_color: "#e5e7eb",
    },
    ocean: {
      text_primary: "#0d2137",
      text_secondary: "#4a6572",
      background_primary: "#f5fbff",
      background_secondary: "#e3f2fd",
      primary_button: "#0277bd",
      secondary_button: "#546e7a",
      accent_color: "#006064",
      border_color: "#b3e5fc",
    },
    forest: {
      text_primary: "#1b2e1b",
      text_secondary: "#4a6352",
      background_primary: "#f7fcf8",
      background_secondary: "#e8f5e9",
      primary_button: "#2e7d32",
      secondary_button: "#5d4037",
      accent_color: "#1b5e20",
      border_color: "#c8e6c9",
    },
    sunset: {
      text_primary: "#3e2723",
      text_secondary: "#6d4c41",
      background_primary: "#fffaf5",
      background_secondary: "#fff3e0",
      primary_button: "#ef6c00",
      secondary_button: "#8d6e63",
      accent_color: "#d84315",
      border_color: "#ffccbc",
    },
    slate: {
      text_primary: "#1e293b",
      text_secondary: "#64748b",
      background_primary: "#f8fafc",
      background_secondary: "#f1f5f9",
      primary_button: "#334155",
      secondary_button: "#64748b",
      accent_color: "#0f172a",
      border_color: "#cbd5e1",
    },
    violet: {
      text_primary: "#1a1a2e",
      text_secondary: "#5c5470",
      background_primary: "#faf8ff",
      background_secondary: "#f3e8ff",
      primary_button: "#6d28d9",
      secondary_button: "#7c3aed",
      accent_color: "#5b21b6",
      border_color: "#ddd6fe",
    },
    amber: {
      text_primary: "#292524",
      text_secondary: "#78716c",
      background_primary: "#fffbeb",
      background_secondary: "#fef3c7",
      primary_button: "#d97706",
      secondary_button: "#a8a29e",
      accent_color: "#b45309",
      border_color: "#fde68a",
    },
    dark: {
      text_primary: "#e2e8f0",
      text_secondary: "#94a3b8",
      background_primary: "#0f172a",
      background_secondary: "#1e293b",
      primary_button: "#38bdf8",
      secondary_button: "#64748b",
      accent_color: "#0026bd",
      border_color: "#334155",
    },
  };
  var FONT_FAMILY_PRESETS = {
    system:
      "system-ui, -apple-system, 'Segoe UI', Roboto, Ubuntu, 'Helvetica Neue', sans-serif",
    inter:
      "Inter, 'Segoe UI', 'Helvetica Neue', Arial, sans-serif",
    opensans:
      "'Open Sans', 'Segoe UI', Tahoma, Arial, sans-serif",
    roboto:
      "Roboto, 'Segoe UI', 'Helvetica Neue', Arial, sans-serif",
    nunitosans:
      "'Nunito Sans', 'Segoe UI', 'Helvetica Neue', Arial, sans-serif",
    montserrat:
      "Montserrat, 'Segoe UI', 'Helvetica Neue', Arial, sans-serif",
    poppins:
      "Poppins, 'Segoe UI', 'Helvetica Neue', Arial, sans-serif",
    worksans:
      "'Work Sans', 'Segoe UI', 'Helvetica Neue', Arial, sans-serif",
    oswald:
      "Oswald, 'Arial Narrow', 'Segoe UI', sans-serif",
    comic:
      "'Comic Sans MS', 'Comic Neue', cursive",
    georgia: "Georgia, 'Times New Roman', serif",
    merriweather:
      "Merriweather, Georgia, 'Times New Roman', serif",
    lora:
      "Lora, Georgia, 'Times New Roman', serif",
    ptserif:
      "'PT Serif', Georgia, 'Times New Roman', serif",
    sourceserif:
      "'Source Serif 4', Georgia, 'Times New Roman', serif",
    playfair:
      "'Playfair Display', Georgia, 'Times New Roman', serif",
    literata:
      "Literata, Georgia, 'Times New Roman', serif",
  };

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

  function updateCharacterCount(inputSelector, countSelector) {
    var input = qs(inputSelector);
    var counter = qs(countSelector);
    if (!input || !counter) return;
    var current = String(input.value || "").length;
    var max = parseInt(input.getAttribute("maxlength") || "0", 10);
    counter.textContent = max
      ? current + "/" + max + " karakter"
      : current + " karakter";
  }

  function updateMetaCharacterCounts() {
    updateCharacterCount("#sm-default-title", "#sm-default-title-count");
    updateCharacterCount(
      "#sm-default-description",
      "#sm-default-description-count"
    );
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
      window.location.href = loginRedirectUrl();
      throw new Error("unauthorized");
    }
    return res;
  }

  function cacheAdminTheme(data) {
    if (!data) return;
    try {
      localStorage.setItem(
        ADMIN_THEME_CACHE_KEY,
        JSON.stringify({
          text_primary: data.text_primary || "",
          text_secondary: data.text_secondary || "",
          background_primary: data.background_primary || "",
          background_secondary: data.background_secondary || "",
          primary_button: data.primary_button || "",
          secondary_button: data.secondary_button || "",
          accent_color: data.accent_color || "",
          border_color: data.border_color || "",
        })
      );
    } catch (e) {}
  }

  function normalizeHexForPicker(raw) {
    var s = String(raw || "")
      .trim()
      .toLowerCase();
    var m3 = /^#([0-9a-f]{3})$/.exec(s);
    if (m3) {
      var c = m3[1];
      return "#" + c[0] + c[0] + c[1] + c[1] + c[2] + c[2];
    }
    var m6 = /^#([0-9a-f]{6})$/.exec(s);
    if (m6) return "#" + m6[1];
    return "";
  }

  function wireColorPickerPair(textSelector, pickerSelector) {
    var textEl = qs(textSelector);
    var pickEl = qs(pickerSelector);
    if (!textEl || !pickEl) return;

    function syncPickerFromText() {
      var hex = normalizeHexForPicker(textEl.value);
      if (hex) pickEl.value = hex;
    }
    function syncTextFromPicker() {
      textEl.value = pickEl.value;
    }

    textEl.addEventListener("input", syncPickerFromText);
    pickEl.addEventListener("input", syncTextFromPicker);
    syncPickerFromText();
  }

  function wireThemeColorPickers() {
    wireColorPickerPair("#ss-text-primary", "#ss-text-primary-picker");
    wireColorPickerPair("#ss-text-secondary", "#ss-text-secondary-picker");
    wireColorPickerPair(
      "#ss-background-primary",
      "#ss-background-primary-picker"
    );
    wireColorPickerPair(
      "#ss-background-secondary",
      "#ss-background-secondary-picker"
    );
    wireColorPickerPair("#ss-primary-button", "#ss-primary-button-picker");
    wireColorPickerPair("#ss-secondary-button", "#ss-secondary-button-picker");
    wireColorPickerPair("#ss-accent-color", "#ss-accent-color-picker");
    wireColorPickerPair("#ss-border-color", "#ss-border-color-picker");
  }

  function syncPickerFromText(textSelector, pickerSelector) {
    var textEl = qs(textSelector);
    var pickEl = qs(pickerSelector);
    if (!textEl || !pickEl) return;
    var hex = normalizeHexForPicker(textEl.value);
    if (hex) pickEl.value = hex;
  }

  function normalizeFontFamilyValue(raw) {
    return String(raw || "")
      .trim()
      .replace(/\s+/g, " ")
      .toLowerCase();
  }

  function resolveFontPresetKey(rawFamily) {
    var normalized = normalizeFontFamilyValue(rawFamily);
    if (!normalized) return "system";
    var keys = Object.keys(FONT_FAMILY_PRESETS);
    for (var i = 0; i < keys.length; i++) {
      var key = keys[i];
      if (normalizeFontFamilyValue(FONT_FAMILY_PRESETS[key]) === normalized) {
        return key;
      }
    }
    return "custom";
  }

  function syncFontPresetFromInput() {
    var familyInput = qs("#ss-font-family");
    var presetSel = qs("#ss-font-family-preset");
    if (!familyInput || !presetSel) return;
    presetSel.value = resolveFontPresetKey(familyInput.value);
  }

  function wireFontFamilyControls() {
    var familyInput = qs("#ss-font-family");
    var presetSel = qs("#ss-font-family-preset");
    if (!familyInput || !presetSel) return;
    presetSel.addEventListener("change", function () {
      if (presetSel.value === "custom") return;
      familyInput.value =
        FONT_FAMILY_PRESETS[presetSel.value] || FONT_FAMILY_PRESETS.system;
    });
    familyInput.addEventListener("input", syncFontPresetFromInput);
    syncFontPresetFromInput();
  }

  function applyAssetPanels(prefix, urlSelected) {
    var uw = qs("#" + prefix + "-upload-wrap");
    var ww = qs("#" + prefix + "-url-wrap");
    if (uw) uw.classList.toggle("d-none", !!urlSelected);
    if (ww) ww.classList.toggle("d-none", !urlSelected);
  }

  function wireAssetSourceRadios(prefix) {
    var u = qs("#" + prefix + "-mode-upload");
    var r = qs("#" + prefix + "-mode-url");
    function sync() {
      applyAssetPanels(prefix, !!(r && r.checked));
    }
    if (u) u.addEventListener("change", sync);
    if (r) r.addEventListener("change", sync);
    sync();
  }

  function normalizeAssetPreviewURL(raw) {
    var v = String(raw || "").trim();
    if (!v) return "";
    return v;
  }

  function ensureAssetPreviewUI(prefix, imageShapeClass) {
    var urlWrap = qs("#" + prefix + "-url-wrap");
    if (!urlWrap || qs("#" + prefix + "-preview-wrap")) return;
    var previewWrap = document.createElement("div");
    previewWrap.id = prefix + "-preview-wrap";
    previewWrap.className = "mt-2 d-none position-relative d-inline-block";

    var img = document.createElement("img");
    img.id = prefix + "-preview-img";
    img.alt = prefix + " preview";
    img.className =
      "img-fluid border bg-body " + (imageShapeClass || "rounded");
    img.style.maxHeight = "160px";
    img.style.objectFit = "contain";
    img.style.display = "block";

    var removeBtn = document.createElement("button");
    removeBtn.type = "button";
    removeBtn.className =
      "btn btn-sm btn-secondary rounded-circle position-absolute top-0 end-0 m-1 d-inline-flex align-items-center justify-content-center";
    removeBtn.style.width = "20px";
    removeBtn.style.height = "20px";
    removeBtn.style.padding = "0";
    removeBtn.setAttribute("aria-label", "Hapus gambar");
    removeBtn.innerHTML = '<i class="bi bi-x-lg" aria-hidden="true"></i>';
    removeBtn.addEventListener("click", function () {
      var urlInput = qs("#" + prefix + "-url");
      var fileInput = qs("#" + prefix + "-file");
      if (urlInput) {
        urlInput.value = "";
        urlInput.dispatchEvent(new Event("input", { bubbles: true }));
      }
      if (fileInput) fileInput.value = "";
      renderAssetPreview(prefix);
    });

    previewWrap.appendChild(img);
    previewWrap.appendChild(removeBtn);
    urlWrap.appendChild(previewWrap);
  }

  function renderAssetPreview(prefix) {
    var urlInput = qs("#" + prefix + "-url");
    var wrap = qs("#" + prefix + "-preview-wrap");
    var img = qs("#" + prefix + "-preview-img");
    if (!urlInput || !wrap || !img) return;
    var src = normalizeAssetPreviewURL(urlInput.value);
    if (!src) {
      img.removeAttribute("src");
      wrap.classList.add("d-none");
      return;
    }
    img.src = src;
    wrap.classList.remove("d-none");
  }

  function maybeSyncAssetModes(data) {
    if (!data) return;
    function nonempty(k) {
      return data[k] != null && String(data[k]).trim() !== "";
    }
    function setMode(prefix, useUrl) {
      var up = qs("#" + prefix + "-mode-upload");
      var ur = qs("#" + prefix + "-mode-url");
      if (up) up.checked = !useUrl;
      if (ur) ur.checked = !!useUrl;
      applyAssetPanels(prefix, !!useUrl);
    }
    setMode("ss-logo", nonempty("logo_url"));
    setMode("ss-favicon", nonempty("favicon_url"));
    setMode("sm-default-image", nonempty("default_image"));
    renderAssetPreview("ss-logo");
    renderAssetPreview("ss-favicon");
    renderAssetPreview("sm-default-image");
  }

  async function uploadSiteAsset(file, kind) {
    var fd = new FormData();
    fd.append("image", file);
    fd.append("kind", kind);
    var token = getToken();
    var res = await fetch(UPLOAD_API, {
      method: "POST",
      headers: { Authorization: "Bearer " + token },
      body: fd,
    });
    if (res.status === 401) {
      window.location.href = loginRedirectUrl();
      throw new Error("unauthorized");
    }
    if (!res.ok) {
      var msg = "Upload gagal (" + res.status + ")";
      try {
        var j = await res.json();
        if (j && j.error) msg = j.error;
      } catch (x) {}
      throw new Error(msg);
    }
    var body = await res.json();
    if (!body || body.image == null || String(body.image).trim() === "") {
      throw new Error("Respons upload tidak valid");
    }
    return String(body.image).trim();
  }

  function wireSiteAssetFile(fileId, urlFieldId, kind) {
    var fileInp = qs(fileId);
    if (!fileInp) return;
    fileInp.addEventListener("change", function () {
      var f = fileInp.files && fileInp.files[0] ? fileInp.files[0] : null;
      if (!f) return;
      uploadSiteAsset(f, kind)
        .then(function (url) {
          var inp = qs(urlFieldId);
          if (inp) {
            inp.value = url;
            inp.dispatchEvent(new Event("input", { bubbles: true }));
          }
          showSuccessToast("File berhasil diunggah. Simpan agar tersimpan.");
        })
        .catch(function (e) {
          if (e && e.message === "unauthorized") return;
          var m = (e && e.message) || "Upload gagal";
          if (typeof Swal !== "undefined") {
            Swal.mixin({
              toast: true,
              position: "top-end",
              showConfirmButton: false,
              timer: 4500,
              timerProgressBar: true,
            }).fire({ icon: "error", title: m });
          }
          fileInp.value = "";
        });
    });
  }
  function cacheAdminThemeFromForm() {
    cacheAdminTheme({
      text_primary:
        (qs("#ss-text-primary") && qs("#ss-text-primary").value) || "",
      text_secondary:
        (qs("#ss-text-secondary") && qs("#ss-text-secondary").value) || "",
      background_primary:
        (qs("#ss-background-primary") && qs("#ss-background-primary").value) ||
        "",
      background_secondary:
        (qs("#ss-background-secondary") &&
          qs("#ss-background-secondary").value) ||
        "",
      primary_button:
        (qs("#ss-primary-button") && qs("#ss-primary-button").value) || "",
      secondary_button:
        (qs("#ss-secondary-button") && qs("#ss-secondary-button").value) || "",
      accent_color:
        (qs("#ss-accent-color") && qs("#ss-accent-color").value) || "",
      border_color:
        (qs("#ss-border-color") && qs("#ss-border-color").value) || "",
    });
  }

  function applyColorPresetToForm(presetKey) {
    var m = COLOR_PRESETS[presetKey];
    if (!m) return;
    var map = [
      ["#ss-text-primary", m.text_primary],
      ["#ss-text-secondary", m.text_secondary],
      ["#ss-background-primary", m.background_primary],
      ["#ss-background-secondary", m.background_secondary],
      ["#ss-primary-button", m.primary_button],
      ["#ss-secondary-button", m.secondary_button],
      ["#ss-accent-color", m.accent_color],
      ["#ss-border-color", m.border_color],
    ];
    for (var i = 0; i < map.length; i++) {
      var el = qs(map[i][0]);
      if (el) el.value = map[i][1];
    }
    syncAllThemePickersFromText();
    cacheAdminThemeFromForm();
  }

  function toggleCustomColorsVisibility() {
    var sel = qs("#ss-color-theme");
    var wrap = qs("#ss-custom-colors-wrapper");
    if (!sel || !wrap) return;
    if (sel.value === "custom") {
      wrap.classList.remove("d-none");
    } else {
      wrap.classList.add("d-none");
    }
  }
  function syncAllThemePickersFromText() {
    syncPickerFromText("#ss-text-primary", "#ss-text-primary-picker");
    syncPickerFromText("#ss-text-secondary", "#ss-text-secondary-picker");
    syncPickerFromText(
      "#ss-background-primary",
      "#ss-background-primary-picker"
    );
    syncPickerFromText(
      "#ss-background-secondary",
      "#ss-background-secondary-picker"
    );
    syncPickerFromText("#ss-primary-button", "#ss-primary-button-picker");
    syncPickerFromText("#ss-secondary-button", "#ss-secondary-button-picker");
    syncPickerFromText("#ss-accent-color", "#ss-accent-color-picker");
    syncPickerFromText("#ss-border-color", "#ss-border-color-picker");
  }

  function fillForm(data) {
    if (!data) return;
    cacheAdminTheme(data);
    var siteName = qs("#ss-site-name");
    var footerCopy = qs("#ss-footer-copy");
    var tagline = qs("#ss-tagline");
    var logo = qs("#ss-logo-url");
    var fav = qs("#ss-favicon-url");
    var textPrimary = qs("#ss-text-primary");
    var textSecondary = qs("#ss-text-secondary");
    var backgroundPrimary = qs("#ss-background-primary");
    var backgroundSecondary = qs("#ss-background-secondary");
    var primaryButton = qs("#ss-primary-button");
    var secondaryButton = qs("#ss-secondary-button");
    var accentColor = qs("#ss-accent-color");
    var borderColor = qs("#ss-border-color");
    var fontFamily = qs("#ss-font-family");
    var contentFontSize = qs("#ss-content-font-size");
    var sidebarFontSize = qs("#ss-sidebar-font-size");
    var sidebarListLimitPopular = qs("#ss-sidebar-list-limit-popular");
    var sidebarListLimitLatest = qs("#ss-sidebar-list-limit-latest");
    var imageCaptionEnabled = qs("#ss-image-caption-enabled");
    var imageCaptionMode = qs("#ss-image-caption-mode");
    var imageCaptionTemplate = qs("#ss-image-caption-template");
    var imageResizeEnabled = qs("#ss-image-resize-enabled");
    var imgResizeEnabled = qs("#ss-img-resize-enabled");
    var imgResizeWidth = qs("#ss-img-resize-width");
    var imgResizeHeight = qs("#ss-img-resize-height");
    var imgResizeQuality = qs("#ss-img-resize-quality");
    var imageResizeWidth = qs("#ss-image-resize-width");
    var imageResizeQuality = qs("#ss-image-resize-quality");
    var storedImageMaxWidth = qs("#ss-stored-image-max-width");
    var storedImageQuality = qs("#ss-stored-image-quality");
    var avatarResizeWidth = qs("#ss-avatar-resize-width");
    var avatarResizeQuality = qs("#ss-avatar-resize-quality");
    var homepageLayout = qs("#ss-homepage-layout");
    var articleSlugMode = qs("#ss-article-slug-mode");
    var scraperAutoTag = qs("#ss-scraper-auto-tag");
    var autoHrefEnabled = qs("#ss-auto-href-enabled");
    var alsoReadEnabled = qs("#ss-also-read-enabled");
    var alsoReadMaxInserts = qs("#ss-also-read-max-inserts");
    var postDateRefreshEnabled = qs("#ss-post-date-refresh-enabled");
    var postDateRefreshDays = qs("#ss-post-date-refresh-days");
    var postDateIntervalMins = qs("#ss-post-date-interval-mins");
    var postDateUrlRecycleEnabled = qs("#ss-post-date-url-recycle-enabled");
    var postDateUrlRecycleIntervalMins = qs(
      "#ss-post-date-url-recycle-interval-mins"
    );
    var sourceDisclaimerMode = qs("#ss-source-disclaimer-mode");
    var sourceDisclaimerTemplate = qs("#ss-source-disclaimer-template");
    var colorTheme = qs("#ss-color-theme");
    var notFoundRedirectMode = qs("#ss-not-found-redirect-mode");
    var defaultTitle = qs("#sm-default-title");
    var defaultDescription = qs("#sm-default-description");
    var defaultImage = qs("#sm-default-image");
    var defaultKeywords = qs("#sm-default-keywords");
    var headerScript = qs("#ss-header-script");
    var adsTop = qs("#ss-ads-top");
    var adsCenter = qs("#ss-ads-center");
    var adsBottom = qs("#ss-ads-bottom");
    var adsTxt = qs("#ss-ads-txt");
    var robotsTxt = qs("#ss-robots-txt");
    var socialFacebookURL = qs("#ss-social-facebook-url");
    var socialXURL = qs("#ss-social-x-url");
    var socialInstagramURL = qs("#ss-social-instagram-url");
    var socialYouTubeURL = qs("#ss-social-youtube-url");
    var socialTikTokURL = qs("#ss-social-tiktok-url");
    if (siteName && data.site_name != null) siteName.value = data.site_name;
    if (footerCopy && data.footer_copy != null) footerCopy.value = data.footer_copy;
    if (tagline && data.tagline != null) tagline.value = data.tagline;
    if (logo && data.logo_url != null) logo.value = data.logo_url;
    if (fav && data.favicon_url != null) fav.value = data.favicon_url;
    if (textPrimary && data.text_primary != null)
      textPrimary.value = data.text_primary;
    if (textSecondary && data.text_secondary != null)
      textSecondary.value = data.text_secondary;
    if (backgroundPrimary && data.background_primary != null)
      backgroundPrimary.value = data.background_primary;
    if (backgroundSecondary && data.background_secondary != null)
      backgroundSecondary.value = data.background_secondary;
    if (primaryButton && data.primary_button != null)
      primaryButton.value = data.primary_button;
    if (secondaryButton && data.secondary_button != null)
      secondaryButton.value = data.secondary_button;
    if (accentColor && data.accent_color != null)
      accentColor.value = data.accent_color;
    if (borderColor && data.border_color != null)
      borderColor.value = data.border_color;
    if (fontFamily && data.font_family != null)
      fontFamily.value = data.font_family;
    if (contentFontSize && data.content_font_size != null)
      contentFontSize.value = data.content_font_size;
    if (sidebarFontSize && data.sidebar_font_size != null)
      sidebarFontSize.value = data.sidebar_font_size;
    function clampSidebarLimit(n) {
      if (isNaN(n) || n < 1) return 5;
      if (n > 20) return 20;
      return n;
    }
    if (sidebarListLimitPopular) {
      var limPop =
        data.sidebar_list_limit_popular != null &&
        data.sidebar_list_limit_popular !== ""
          ? parseInt(data.sidebar_list_limit_popular, 10)
          : data.sidebar_list_limit != null && data.sidebar_list_limit !== ""
          ? parseInt(data.sidebar_list_limit, 10)
          : 5;
      sidebarListLimitPopular.value = String(clampSidebarLimit(limPop));
    }
    if (sidebarListLimitLatest) {
      var limLat =
        data.sidebar_list_limit_latest != null &&
        data.sidebar_list_limit_latest !== ""
          ? parseInt(data.sidebar_list_limit_latest, 10)
          : data.sidebar_list_limit != null && data.sidebar_list_limit !== ""
          ? parseInt(data.sidebar_list_limit, 10)
          : 5;
      sidebarListLimitLatest.value = String(clampSidebarLimit(limLat));
    }
    if (imageCaptionEnabled)
      imageCaptionEnabled.checked = !!data.image_caption_enabled;
    if (imageCaptionMode && data.image_caption_mode != null) {
      imageCaptionMode.value =
        String(data.image_caption_mode).toLowerCase().trim() === "custom"
          ? "custom"
          : "default";
    }
    if (imageCaptionTemplate && data.image_caption_template != null) {
      imageCaptionTemplate.value = data.image_caption_template;
    }
    if (imageResizeEnabled)
      imageResizeEnabled.checked = data.image_resize_enabled !== false;
    if (imgResizeEnabled)
      imgResizeEnabled.checked = !!data.img_resize_enabled;
    if (imgResizeWidth && data.img_resize_width != null)
      imgResizeWidth.value = data.img_resize_width;
    if (imgResizeHeight && data.img_resize_height != null)
      imgResizeHeight.value = data.img_resize_height;
    if (imgResizeQuality && data.img_resize_quality != null)
      imgResizeQuality.value = data.img_resize_quality;
    if (imageResizeWidth && data.image_resize_width != null)
      imageResizeWidth.value = data.image_resize_width;
    if (imageResizeQuality && data.image_resize_quality != null)
      imageResizeQuality.value = data.image_resize_quality;
    if (storedImageMaxWidth && data.stored_image_max_width != null)
      storedImageMaxWidth.value = data.stored_image_max_width;
    if (storedImageQuality && data.stored_image_quality != null)
      storedImageQuality.value = data.stored_image_quality;
    if (avatarResizeWidth && data.avatar_resize_width != null)
      avatarResizeWidth.value = data.avatar_resize_width;
    if (avatarResizeQuality && data.avatar_resize_quality != null)
      avatarResizeQuality.value = data.avatar_resize_quality;
    if (homepageLayout && data.homepage_layout != null)
      homepageLayout.value = data.homepage_layout || "default";
    if (articleSlugMode && data.article_slug_mode != null)
      articleSlugMode.value = data.article_slug_mode || "direct";
    var articleSchemaType = qs("#ss-article-schema-type");
    if (articleSchemaType && data.article_schema_type != null) {
      articleSchemaType.value =
        data.article_schema_type === "NewsArticle" ? "NewsArticle" : "Article";
    }
    if (scraperAutoTag)
      scraperAutoTag.checked = data.scraper_auto_tag !== false;
    if (autoHrefEnabled)
      autoHrefEnabled.checked = !!data.auto_href_enabled;
    if (alsoReadEnabled) {
      var ar = data.also_read_enabled;
      alsoReadEnabled.checked =
        ar === true || ar === 1 || String(ar).toLowerCase() === "true";
    }
    if (alsoReadMaxInserts) {
      var maxVal =
        data.also_read_max_inserts != null && data.also_read_max_inserts !== ""
          ? data.also_read_max_inserts
          : 6;
      alsoReadMaxInserts.value = String(clampAlsoReadMaxInserts(maxVal));
    }
    if (postDateRefreshEnabled) {
      var v = data.post_date_refresh_enabled;
      postDateRefreshEnabled.checked =
        v === true || v === 1 || String(v).toLowerCase() === "true";
    }
    if (postDateRefreshDays && data.post_date_refresh_days != null)
      postDateRefreshDays.value = data.post_date_refresh_days;
    if (postDateIntervalMins && data.post_date_interval_mins != null)
      postDateIntervalMins.value = data.post_date_interval_mins;
    if (postDateUrlRecycleEnabled) {
      var urEn = data.post_date_url_recycle_enabled;
      postDateUrlRecycleEnabled.checked =
        urEn === true ||
        urEn === 1 ||
        String(urEn).toLowerCase() === "true";
    }
    if (
      postDateUrlRecycleIntervalMins &&
      data.post_date_url_recycle_interval_mins != null
    )
      postDateUrlRecycleIntervalMins.value =
        data.post_date_url_recycle_interval_mins;
    if (sourceDisclaimerMode && data.source_disclaimer_mode != null) {
      sourceDisclaimerMode.value =
        String(data.source_disclaimer_mode).toLowerCase().trim() === "custom"
          ? "custom"
          : "default";
    }
    if (sourceDisclaimerTemplate && data.source_disclaimer_template != null) {
      sourceDisclaimerTemplate.value = data.source_disclaimer_template;
    }
    if (colorTheme && data.color_theme != null)
      colorTheme.value = data.color_theme || "default";
    if (notFoundRedirectMode && data.not_found_redirect_mode != null) {
      notFoundRedirectMode.value = data.not_found_redirect_mode || "home";
    }
    if (defaultTitle && data.default_title != null)
      defaultTitle.value = data.default_title;
    if (defaultDescription && data.default_description != null)
      defaultDescription.value = data.default_description;
    updateMetaCharacterCounts();
    if (defaultImage && data.default_image != null)
      defaultImage.value = data.default_image;
    if (defaultKeywords && data.default_keywords != null)
      defaultKeywords.value = data.default_keywords;
    if (headerScript && data.header_script != null)
      headerScript.value = data.header_script;
    if (adsTop && data.ads_top != null) adsTop.value = data.ads_top;
    if (adsCenter && data.ads_center != null) adsCenter.value = data.ads_center;
    if (adsBottom && data.ads_bottom != null) adsBottom.value = data.ads_bottom;
    var adsLazyLoad = qs("#ss-ads-lazy-load-enabled");
    if (adsLazyLoad && data.ads_lazy_load_enabled != null) {
      var lz = data.ads_lazy_load_enabled;
      adsLazyLoad.checked =
        lz === true || lz === 1 || String(lz).toLowerCase() === "true";
    }
    if (adsTxt && data.ads_txt != null) adsTxt.value = data.ads_txt;
    if (robotsTxt && data.robots_txt != null) robotsTxt.value = data.robots_txt;
    if (data.article_ads_json != null) {
      var articleAdsHidden = qs("#ss-article-ads-data");
      if (articleAdsHidden) articleAdsHidden.value = String(data.article_ads_json || "");
      var freshAds = readArticleAdsFromHidden();
      var articleAdsCountEl = qs("#ss-article-ads-count");
      if (articleAdsCountEl) articleAdsCountEl.value = String(freshAds.length || 0);
      renderArticleAdCards(freshAds.length, freshAds, false);
    }
    if (socialFacebookURL && data.social_facebook_url != null)
      socialFacebookURL.value = data.social_facebook_url;
    if (socialXURL && data.social_x_url != null)
      socialXURL.value = data.social_x_url;
    if (socialInstagramURL && data.social_instagram_url != null)
      socialInstagramURL.value = data.social_instagram_url;
    if (socialYouTubeURL && data.social_youtube_url != null)
      socialYouTubeURL.value = data.social_youtube_url;
    if (socialTikTokURL && data.social_tiktok_url != null)
      socialTikTokURL.value = data.social_tiktok_url;
    syncAllThemePickersFromText();
    syncFontPresetFromInput();
    maybeSyncAssetModes(data);
    toggleCustomColorsVisibility();
    syncCdnPostImageFieldsVisibility();
    syncImgProxyFieldsVisibility();
    syncPostDateRefreshFieldsVisibility();
    syncPostDateUrlRecycleFieldsVisibility();
    syncAlsoReadMaxFieldsVisibility();
    syncSourceDisclaimerTemplateVisibility();
    syncImageCaptionFieldsVisibility();
  }

  function syncCdnPostImageFieldsVisibility() {
    var en = qs("#ss-image-resize-enabled");
    var wrap = qs("#ss-cdn-resize-fields");
    if (!wrap) return;
    var on = !!(en && en.checked);
    wrap.classList.toggle("d-none", !on);
  }

  function syncImgProxyFieldsVisibility() {
    var en = qs("#ss-img-resize-enabled");
    var wrap = qs("#ss-img-proxy-fields");
    if (!wrap) return;
    var on = !!(en && en.checked);
    wrap.classList.toggle("d-none", !on);
  }

  function clampAlsoReadMaxInserts(n) {
    var v = parseInt(n, 10);
    if (isNaN(v) || v < 1) return 6;
    if (v > 20) return 20;
    return v;
  }

  function syncAlsoReadMaxFieldsVisibility() {
    var en = qs("#ss-also-read-enabled");
    var wrap = qs("#ss-also-read-max-fields");
    var maxEl = qs("#ss-also-read-max-inserts");
    var on = !!(en && en.checked);
    if (wrap) wrap.classList.toggle("d-none", !on);
    if (maxEl) maxEl.disabled = !on;
  }

  function syncPostDateRefreshFieldsVisibility() {
    var en = qs("#ss-post-date-refresh-enabled");
    var wrap = qs("#ss-post-date-refresh-fields");
    var days = qs("#ss-post-date-refresh-days");
    var mins = qs("#ss-post-date-interval-mins");
    var on = !!(en && en.checked);
    if (wrap) wrap.classList.toggle("d-none", !on);
    if (days) days.disabled = !on;
    if (mins) mins.disabled = !on;
  }

  function syncPostDateUrlRecycleFieldsVisibility() {
    var en = qs("#ss-post-date-url-recycle-enabled");
    var wrap = qs("#ss-post-date-url-recycle-fields");
    var ta = qs("#ss-post-date-url-recycle-urls");
    var im = qs("#ss-post-date-url-recycle-interval-mins");
    var runBtn = qs("#ss-post-date-url-recycle-run");
    var on = !!(en && en.checked);
    if (wrap) {
      if (on) {
        wrap.classList.remove("d-none");
      } else {
        wrap.classList.add("d-none");
      }
    }
    if (ta) ta.disabled = !on;
    if (im) im.disabled = !on;
    if (runBtn) runBtn.disabled = !on;
  }

  function syncSourceDisclaimerTemplateVisibility() {
    var mode = qs("#ss-source-disclaimer-mode");
    var wrap = qs("#ss-source-disclaimer-template-wrap");
    if (!wrap) return;
    var isCustom =
      mode && String(mode.value || "").toLowerCase().trim() === "custom";
    wrap.classList.toggle("d-none", !isCustom);
  }

  function syncImageCaptionFieldsVisibility() {
    var en = qs("#ss-image-caption-enabled");
    var modeWrap = qs("#ss-image-caption-mode-wrap");
    var tplWrap = qs("#ss-image-caption-template-wrap");
    var mode = qs("#ss-image-caption-mode");
    var on = !!(en && en.checked);
    if (modeWrap) modeWrap.classList.toggle("d-none", !on);
    if (tplWrap) {
      var isCustom =
        on &&
        mode &&
        String(mode.value || "").toLowerCase().trim() === "custom";
      tplWrap.classList.toggle("d-none", !isCustom);
    }
  }

  function fillCloudStorageForm(data) {
    if (!data) return;
    var r2AccountID = qs("#ss-r2-account-id");
    var r2AccessKeyID = qs("#ss-r2-access-key-id");
    var r2SecretAccessKey = qs("#ss-r2-secret-access-key");
    var r2Bucket = qs("#ss-r2-bucket");
    var r2PublicBaseURL = qs("#ss-r2-public-base-url");
    var r2Prefix = qs("#ss-r2-prefix");
    if (r2AccountID && data.r2_account_id != null)
      r2AccountID.value = data.r2_account_id;
    if (r2AccessKeyID && data.r2_access_key_id != null)
      r2AccessKeyID.value = data.r2_access_key_id;
    if (r2SecretAccessKey && data.r2_secret_access_key != null)
      r2SecretAccessKey.value = data.r2_secret_access_key;
    if (r2Bucket)
      r2Bucket.value =
        data.r2_bucket != null && String(data.r2_bucket).trim() !== ""
          ? data.r2_bucket
          : "media";
    if (r2PublicBaseURL && data.r2_public_base_url != null)
      r2PublicBaseURL.value = data.r2_public_base_url;
    if (r2Prefix)
      r2Prefix.value =
        data.r2_prefix != null && String(data.r2_prefix).trim() !== ""
          ? data.r2_prefix
          : "uploads";
  }

  function normalizeLoginURLInput(raw) {
    var s = String(raw || "").trim();
    if (!s) return "/login";
    var lower = s.toLowerCase();
    if (lower.indexOf("http://") === 0 || lower.indexOf("https://") === 0) {
      return s;
    }
    var pathOnly = s.replace(/^\/+/, "");
    if (!pathOnly) return "/login";
    return "/" + pathOnly;
  }

  function clampInt(n, min, max) {
    var v = parseInt(n, 10);
    if (isNaN(v)) v = min;
    if (v < min) v = min;
    if (v > max) v = max;
    return v;
  }

  function readArticleAdsFromHidden() {
    var el = qs("#ss-article-ads-data");
    var raw = (el && el.value) || "";
    if (!String(raw).trim()) return [];
    try {
      var parsed = JSON.parse(raw);
      if (!Array.isArray(parsed)) return [];
      var out = [];
      parsed.forEach(function (p) {
        if (!p || typeof p !== "object") return;
        var pos = clampInt(p.position, 1, 99);
        var content = String(p.content == null ? "" : p.content);
        out.push({ position: pos, content: content });
      });
      out.sort(function (a, b) { return a.position - b.position; });
      return out;
    } catch (e) {
      return [];
    }
  }

  function readArticleAdsFromUI() {
    var list = qs("#ss-article-ads-list");
    if (!list) return [];
    var cards = list.querySelectorAll("[data-article-ad-card]");
    var out = [];
    cards.forEach(function (card) {
      var posEl = card.querySelector("[data-article-ad-position]");
      var contentEl = card.querySelector("[data-article-ad-content]");
      var pos = clampInt(posEl ? posEl.value : 50, 1, 99);
      var content = contentEl ? contentEl.value : "";
      out.push({ position: pos, content: content });
    });
    return out;
  }

  function setArticleAdsHidden(items) {
    var el = qs("#ss-article-ads-data");
    if (!el) return;
    var clean = (items || []).filter(function (it) {
      return it && String(it.content || "").trim() !== "";
    });
    el.value = clean.length ? JSON.stringify(clean) : "";
  }

  function renderArticleAdCards(count, seedItems, preserveExisting) {
    var list = qs("#ss-article-ads-list");
    var empty = qs("#ss-article-ads-empty");
    var tpl = qs("#ss-article-ads-item-tpl");
    if (!list || !tpl) return;
    count = clampInt(count, 0, 10);
    var previousUI = preserveExisting ? readArticleAdsFromUI() : null;
    list.innerHTML = "";
    if (count === 0) {
      if (empty) empty.classList.remove("d-none");
      return;
    }
    if (empty) empty.classList.add("d-none");
    for (var i = 0; i < count; i++) {
      var node = tpl.content.firstElementChild.cloneNode(true);
      var idxEl = node.querySelector("[data-article-ad-index]");
      var posEl = node.querySelector("[data-article-ad-position]");
      var contentEl = node.querySelector("[data-article-ad-content]");
      var pctLabel = node.querySelector("[data-article-ad-percent-label]");
      var defPos = Math.round(((i + 1) * 100) / (count + 1));
      if (defPos < 1) defPos = 1;
      if (defPos > 99) defPos = 99;
      var seed = (seedItems && seedItems[i]) || (previousUI && previousUI[i]) || null;
      if (idxEl) idxEl.textContent = String(i + 1);
      if (posEl) posEl.value = seed ? clampInt(seed.position, 1, 99) : defPos;
      if (contentEl) contentEl.value = seed ? String(seed.content || "") : "";
      if (pctLabel && posEl) pctLabel.textContent = String(posEl.value) + "%";
      if (posEl && pctLabel) {
        (function (input, label) {
          input.addEventListener("input", function () {
            var v = clampInt(input.value, 1, 99);
            label.textContent = String(v) + "%";
          });
        })(posEl, pctLabel);
      }
      list.appendChild(node);
    }
  }

  function buildArticleAdsPayload() {
    var items = readArticleAdsFromUI();
    return items.filter(function (it) {
      return it && String(it.content || "").trim() !== "";
    });
  }

  function buildPayload() {
    return {
      article_ads: buildArticleAdsPayload(),
      site_name: (qs("#ss-site-name") && qs("#ss-site-name").value) || "",
      footer_copy: (qs("#ss-footer-copy") && qs("#ss-footer-copy").value) || "",
      tagline: (qs("#ss-tagline") && qs("#ss-tagline").value) || "",
      logo_url: (qs("#ss-logo-url") && qs("#ss-logo-url").value) || "",
      favicon_url: (qs("#ss-favicon-url") && qs("#ss-favicon-url").value) || "",
      text_primary:
        (qs("#ss-text-primary") && qs("#ss-text-primary").value) || "",
      text_secondary:
        (qs("#ss-text-secondary") && qs("#ss-text-secondary").value) || "",
      background_primary:
        (qs("#ss-background-primary") && qs("#ss-background-primary").value) ||
        "",
      background_secondary:
        (qs("#ss-background-secondary") &&
          qs("#ss-background-secondary").value) ||
        "",
      primary_button:
        (qs("#ss-primary-button") && qs("#ss-primary-button").value) || "",
      secondary_button:
        (qs("#ss-secondary-button") && qs("#ss-secondary-button").value) || "",
      accent_color:
        (qs("#ss-accent-color") && qs("#ss-accent-color").value) || "",
      border_color:
        (qs("#ss-border-color") && qs("#ss-border-color").value) || "",
      font_family: (qs("#ss-font-family") && qs("#ss-font-family").value) || "",
      content_font_size:
        (qs("#ss-content-font-size") && qs("#ss-content-font-size").value) ||
        "",
      sidebar_font_size:
        (qs("#ss-sidebar-font-size") && qs("#ss-sidebar-font-size").value) ||
        "",
      sidebar_list_limit_popular: (function () {
        var el = qs("#ss-sidebar-list-limit-popular");
        var n = el ? parseInt(el.value, 10) : 5;
        if (isNaN(n) || n < 1) return 5;
        if (n > 20) return 20;
        return n;
      })(),
      sidebar_list_limit_latest: (function () {
        var el = qs("#ss-sidebar-list-limit-latest");
        var n = el ? parseInt(el.value, 10) : 5;
        if (isNaN(n) || n < 1) return 5;
        if (n > 20) return 20;
        return n;
      })(),
      image_caption_enabled: !!(
        qs("#ss-image-caption-enabled") &&
        qs("#ss-image-caption-enabled").checked
      ),
      image_caption_mode:
        (qs("#ss-image-caption-mode") && qs("#ss-image-caption-mode").value) ||
        "default",
      image_caption_template:
        (qs("#ss-image-caption-template") &&
          qs("#ss-image-caption-template").value) ||
        "",
      image_resize_enabled: !!(
        qs("#ss-image-resize-enabled") &&
        qs("#ss-image-resize-enabled").checked
      ),
      img_resize_enabled: !!(
        qs("#ss-img-resize-enabled") &&
        qs("#ss-img-resize-enabled").checked
      ),
      img_resize_width:
        (qs("#ss-img-resize-width") && qs("#ss-img-resize-width").value) || "",
      img_resize_height:
        (qs("#ss-img-resize-height") && qs("#ss-img-resize-height").value) || "",
      img_resize_quality:
        (qs("#ss-img-resize-quality") && qs("#ss-img-resize-quality").value) ||
        "",
      image_resize_width:
        (qs("#ss-image-resize-width") && qs("#ss-image-resize-width").value) ||
        "",
      image_resize_quality:
        (qs("#ss-image-resize-quality") &&
          qs("#ss-image-resize-quality").value) ||
        "",
      stored_image_max_width:
        (qs("#ss-stored-image-max-width") &&
          qs("#ss-stored-image-max-width").value) ||
        "",
      stored_image_quality:
        (qs("#ss-stored-image-quality") &&
          qs("#ss-stored-image-quality").value) ||
        "",
      avatar_resize_width:
        (qs("#ss-avatar-resize-width") &&
          qs("#ss-avatar-resize-width").value) ||
        "",
      avatar_resize_quality:
        (qs("#ss-avatar-resize-quality") &&
          qs("#ss-avatar-resize-quality").value) ||
        "",
      homepage_layout:
        (qs("#ss-homepage-layout") && qs("#ss-homepage-layout").value) ||
        "default",
      article_slug_mode:
        (qs("#ss-article-slug-mode") && qs("#ss-article-slug-mode").value) ||
        "direct",
      article_schema_type:
        (qs("#ss-article-schema-type") &&
          qs("#ss-article-schema-type").value) ||
        "Article",
      scraper_auto_tag: !!(
        qs("#ss-scraper-auto-tag") && qs("#ss-scraper-auto-tag").checked
      ),
      auto_href_enabled: !!(
        qs("#ss-auto-href-enabled") && qs("#ss-auto-href-enabled").checked
      ),
      also_read_enabled: !!(
        qs("#ss-also-read-enabled") && qs("#ss-also-read-enabled").checked
      ),
      also_read_max_inserts: clampAlsoReadMaxInserts(
        (qs("#ss-also-read-max-inserts") &&
          qs("#ss-also-read-max-inserts").value) ||
          6
      ),
      post_date_refresh_enabled: !!(
        qs("#ss-post-date-refresh-enabled") &&
        qs("#ss-post-date-refresh-enabled").checked
      ),
      post_date_refresh_days: Number(
        (qs("#ss-post-date-refresh-days") &&
          qs("#ss-post-date-refresh-days").value) ||
          30
      ),
      post_date_interval_mins: Number(
        (qs("#ss-post-date-interval-mins") &&
          qs("#ss-post-date-interval-mins").value) ||
          30
      ),
      post_date_url_recycle_enabled: !!(
        qs("#ss-post-date-url-recycle-enabled") &&
        qs("#ss-post-date-url-recycle-enabled").checked
      ),
      post_date_url_recycle_interval_mins: Number(
        (qs("#ss-post-date-url-recycle-interval-mins") &&
          qs("#ss-post-date-url-recycle-interval-mins").value) ||
          0
      ),
      source_disclaimer_mode:
        (qs("#ss-source-disclaimer-mode") &&
          qs("#ss-source-disclaimer-mode").value) ||
        "default",
      source_disclaimer_template:
        (qs("#ss-source-disclaimer-template") &&
          qs("#ss-source-disclaimer-template").value) ||
        "",
      color_theme:
        (qs("#ss-color-theme") && qs("#ss-color-theme").value) || "default",
      login_url: normalizeLoginURLInput(
        (qs("#ss-login-url") && qs("#ss-login-url").value) || ""
      ),
      not_found_redirect_mode:
        (qs("#ss-not-found-redirect-mode") &&
          qs("#ss-not-found-redirect-mode").value) ||
        "home",
      default_title:
        (qs("#sm-default-title") && qs("#sm-default-title").value) || "",
      default_description:
        (qs("#sm-default-description") &&
          qs("#sm-default-description").value) ||
        "",
      default_image:
        (qs("#sm-default-image") && qs("#sm-default-image").value) || "",
      default_keywords:
        (qs("#sm-default-keywords") && qs("#sm-default-keywords").value) || "",
      header_script:
        (qs("#ss-header-script") && qs("#ss-header-script").value) || "",
      footer_script: "",
      ads_top: (qs("#ss-ads-top") && qs("#ss-ads-top").value) || "",
      ads_center: (qs("#ss-ads-center") && qs("#ss-ads-center").value) || "",
      ads_bottom: (qs("#ss-ads-bottom") && qs("#ss-ads-bottom").value) || "",
      ads_txt: (qs("#ss-ads-txt") && qs("#ss-ads-txt").value) || "",
      robots_txt: (qs("#ss-robots-txt") && qs("#ss-robots-txt").value) || "",
      ads_lazy_load_enabled: !!(
        qs("#ss-ads-lazy-load-enabled") &&
        qs("#ss-ads-lazy-load-enabled").checked
      ),
      social_facebook_url:
        (qs("#ss-social-facebook-url") &&
          qs("#ss-social-facebook-url").value) ||
        "",
      social_x_url:
        (qs("#ss-social-x-url") && qs("#ss-social-x-url").value) || "",
      social_instagram_url:
        (qs("#ss-social-instagram-url") &&
          qs("#ss-social-instagram-url").value) ||
        "",
      social_youtube_url:
        (qs("#ss-social-youtube-url") && qs("#ss-social-youtube-url").value) ||
        "",
      social_tiktok_url:
        (qs("#ss-social-tiktok-url") && qs("#ss-social-tiktok-url").value) ||
        "",
    };
  }

  function buildCloudStoragePayload() {
    return {
      r2_account_id:
        (qs("#ss-r2-account-id") && qs("#ss-r2-account-id").value) || "",
      r2_access_key_id:
        (qs("#ss-r2-access-key-id") && qs("#ss-r2-access-key-id").value) || "",
      r2_secret_access_key:
        (qs("#ss-r2-secret-access-key") &&
          qs("#ss-r2-secret-access-key").value) ||
        "",
      r2_bucket: (qs("#ss-r2-bucket") && qs("#ss-r2-bucket").value) || "media",
      r2_public_base_url:
        (qs("#ss-r2-public-base-url") && qs("#ss-r2-public-base-url").value) ||
        "",
      r2_prefix:
        (qs("#ss-r2-prefix") && qs("#ss-r2-prefix").value) || "uploads",
    };
  }

  var WEB_IMAGE_PROVIDER_FIELDS = {
    unsplash: ["access_key"],
    pixabay: ["api_key"],
    openverse: ["client_id", "client_secret"],
    wikimedia: [],
    wordpress: [],
    pexels: ["api_key"],
  };
  var WEB_IMAGE_REQUIRED_CREDENTIALS = {
    unsplash: ["access_key"],
    pixabay: ["api_key"],
    pexels: ["api_key"],
  };
  var WEB_IMAGE_PROVIDER_ORDER = [
    "pexels",
    "unsplash",
    "pixabay",
    "openverse",
    "wikimedia",
    "wordpress",
  ];

  function fillWebImageProviderForm(data) {
    var providers = (data && data.providers) || [];
    var byProvider = {};
    providers.forEach(function (item) {
      if (!item || !item.provider) return;
      byProvider[String(item.provider).toLowerCase()] = item;
    });
    WEB_IMAGE_PROVIDER_ORDER.forEach(function (provider) {
      var item = byProvider[provider] || {};
      var enabled = qs('[data-wi-enabled="' + provider + '"]');
      if (enabled) enabled.checked = item.enabled === true;
      var credentials = item.credentials || {};
      (WEB_IMAGE_PROVIDER_FIELDS[provider] || []).forEach(function (field) {
        var input = qs('[data-wi-credential="' + provider + "." + field + '"]');
        if (input) input.value = credentials[field] || "";
      });
    });
    syncWebImageProviderConfigStatus();
  }

  function buildWebImageProviderPayload() {
    return {
      providers: WEB_IMAGE_PROVIDER_ORDER.map(function (provider) {
        var enabled = qs('[data-wi-enabled="' + provider + '"]');
        var credentials = {};
        (WEB_IMAGE_PROVIDER_FIELDS[provider] || []).forEach(function (field) {
          var input = qs('[data-wi-credential="' + provider + "." + field + '"]');
          credentials[field] = input ? input.value : "";
        });
        return {
          provider: provider,
          enabled: enabled ? !!enabled.checked : true,
          credentials: credentials,
        };
      }),
    };
  }

  function webImageProviderMissingRequiredKey(provider) {
    var enabled = qs('[data-wi-enabled="' + provider + '"]');
    if (enabled && !enabled.checked) return false;
    var fields = WEB_IMAGE_REQUIRED_CREDENTIALS[provider] || [];
    for (var i = 0; i < fields.length; i++) {
      var input = qs(
        '[data-wi-credential="' + provider + "." + fields[i] + '"]'
      );
      if (!input || String(input.value || "").trim() === "") return true;
    }
    return false;
  }

  function syncWebImageProviderConfigStatus() {
    var missing = [];
    Object.keys(WEB_IMAGE_REQUIRED_CREDENTIALS).forEach(function (provider) {
      var isMissing = webImageProviderMissingRequiredKey(provider);
      var badge = qs('[data-wi-status="' + provider + '"]');
      if (badge) badge.classList.toggle("d-none", !isMissing);
      if (isMissing) missing.push(provider.charAt(0).toUpperCase() + provider.slice(1));
    });
    var status = qs("#site-web-images-config-status");
    if (!status) return;
    if (!missing.length) {
      status.classList.add("d-none");
      status.textContent = "";
      return;
    }
    status.classList.remove("d-none");
    status.textContent =
      "Belum config: " +
      missing.join(", ") +
      ". Penyedia ini memerlukan API key sebelum bisa dipakai untuk pencarian gambar web.";
  }

  function normalizeSectionKey(raw) {
    var t = String(raw || "")
      .toLowerCase()
      .replace(/&/g, "and")
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "");
    if (!t) return "section";
    if (t.indexOf("theme") >= 0) return "theme";
    if (t.indexOf("image") >= 0) return "images";
    if (t.indexOf("brand") >= 0) return "assets";
    if (t.indexOf("access") >= 0 || t.indexOf("social") >= 0) return "access";
    if (t.indexOf("general") >= 0) return "general";
    return t;
  }

  function initSiteSettingSubtabs(form, onSaveSection) {
    if (!form) return [];
    var blocks = Array.prototype.slice.call(form.children || []);
    var sections = [];
    var current = null;
    blocks.forEach(function (node) {
      var titleEl = node.querySelector(".ss-section-divider__title");
      if (titleEl) {
        current = {
          key: normalizeSectionKey(titleEl.textContent || ""),
          label: String(titleEl.textContent || "").trim(),
          nodes: [],
        };
        sections.push(current);
      }
      if (current) current.nodes.push(node);
    });
    if (sections.length < 2) return [];

    var nav = document.createElement("div");
    nav.className = "ss-subtabs-nav nav nav-pills gap-2 mb-3";
    nav.setAttribute("role", "tablist");

    var panesWrap = document.createElement("div");
    panesWrap.className = "ss-subtabs-wrap";
    form.insertBefore(nav, form.firstChild);
    form.insertBefore(panesWrap, nav.nextSibling);

    sections.forEach(function (section, idx) {
      var btn = document.createElement("button");
      btn.type = "button";
      btn.className = "nav-link" + (idx === 0 ? " active" : "");
      btn.textContent = section.label;
      btn.setAttribute("data-ss-subtab", section.key);
      nav.appendChild(btn);

      var pane = document.createElement("div");
      pane.className = "row g-3 g-lg-4 ss-subtab-pane";
      if (idx !== 0) pane.classList.add("d-none");
      pane.setAttribute("data-ss-subpane", section.key);

      section.nodes.forEach(function (n) {
        pane.appendChild(n);
      });

      if (!pane.querySelector(".ss-actions")) {
        var action = document.createElement("div");
        action.className = "col-12 pt-2 ss-actions";
        var saveBtn = document.createElement("button");
        saveBtn.type = "button";
        saveBtn.className = "btn btn-primary px-4";
        saveBtn.setAttribute("data-ss-save-section", section.key);
        saveBtn.innerHTML =
          '<i class="bi bi-check-lg me-1" aria-hidden="true"></i>Simpan';
        action.appendChild(saveBtn);
        pane.appendChild(action);
      } else {
        var existingBtn = pane.querySelector(".ss-actions .btn");
        if (existingBtn) {
          existingBtn.type = "button";
          existingBtn.setAttribute("data-ss-save-section", section.key);
        }
      }

      panesWrap.appendChild(pane);
    });

    nav.addEventListener("click", function (e) {
      var targetBtn = e.target.closest("[data-ss-subtab]");
      if (!targetBtn) return;
      var key = targetBtn.getAttribute("data-ss-subtab");
      nav.querySelectorAll("[data-ss-subtab]").forEach(function (b) {
        b.classList.toggle("active", b === targetBtn);
      });
      panesWrap.querySelectorAll("[data-ss-subpane]").forEach(function (p) {
        p.classList.toggle("d-none", p.getAttribute("data-ss-subpane") !== key);
      });
    });

    form.querySelectorAll("[data-ss-save-section]").forEach(function (btn) {
      btn.addEventListener("click", function () {
        var key = btn.getAttribute("data-ss-save-section") || "section";
        var titleMap = {
          general: "Pengaturan umum",
          theme: "Pengaturan tema",
          images: "Pengaturan gambar",
          assets: "Pengaturan aset",
          access: "Pengaturan akses",
        };
        onSaveSection(btn, (titleMap[key] || "Pengaturan") + " berhasil disimpan.");
      });
    });

    return sections.map(function (s) {
      return s.key;
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    if (!getToken()) {
      window.location.href = loginRedirectUrl();
      return;
    }

    var errEl = qs("#site-settings-error");
    var form = qs("#form-site-settings");
    var submitBtn = qs("#ss-submit");
    var scriptsErrEl = qs("#site-scripts-error");
    var scriptsForm = qs("#form-site-scripts");
    var scriptsSubmitBtn = qs("#ss-scripts-submit");
    var articleAdsErrEl = qs("#site-article-ads-error");
    var articleAdsForm = qs("#form-article-ads");
    var articleAdsSubmitBtn = qs("#ss-article-ads-submit");
    var articleAdsCountEl = qs("#ss-article-ads-count");
    var metaErrEl = qs("#site-meta-error");
    var metaForm = qs("#form-site-meta");
    var metaSubmitBtn = qs("#sm-submit");
    var defaultTitleInput = qs("#sm-default-title");
    var defaultDescriptionInput = qs("#sm-default-description");
    var r2ErrEl = qs("#site-r2-error");
    var r2Form = qs("#form-site-r2");
    var r2SubmitBtn = qs("#ss-r2-submit");
    var webImagesErrEl = qs("#site-web-images-error");
    var webImagesForm = qs("#form-site-web-images");
    var webImagesSubmitBtn = qs("#ss-web-images-submit");
    if (defaultTitleInput) {
      defaultTitleInput.addEventListener("input", updateMetaCharacterCounts);
    }
    if (defaultDescriptionInput) {
      defaultDescriptionInput.addEventListener("input", updateMetaCharacterCounts);
    }
    updateMetaCharacterCounts();
    if (webImagesForm) {
      webImagesForm.addEventListener("input", syncWebImageProviderConfigStatus);
      webImagesForm.addEventListener("change", syncWebImageProviderConfigStatus);
      syncWebImageProviderConfigStatus();
    }
    wireFontFamilyControls();
    wireThemeColorPickers();
    wireAssetSourceRadios("ss-logo");
    wireAssetSourceRadios("ss-favicon");
    wireAssetSourceRadios("sm-default-image");
    ensureAssetPreviewUI("ss-logo", "rounded");
    ensureAssetPreviewUI("ss-favicon", "rounded");
    ensureAssetPreviewUI("sm-default-image", "rounded");
    wireSiteAssetFile("#ss-logo-file", "#ss-logo-url", "logo");
    wireSiteAssetFile("#ss-favicon-file", "#ss-favicon-url", "favicon");
    wireSiteAssetFile(
      "#sm-default-image-file",
      "#sm-default-image",
      "default_image"
    );
    var logoUrlInput = qs("#ss-logo-url");
    var faviconUrlInput = qs("#ss-favicon-url");
    var defaultImageInput = qs("#sm-default-image");
    if (logoUrlInput) {
      logoUrlInput.addEventListener("input", function () {
        renderAssetPreview("ss-logo");
      });
      renderAssetPreview("ss-logo");
    }
    if (faviconUrlInput) {
      faviconUrlInput.addEventListener("input", function () {
        renderAssetPreview("ss-favicon");
      });
      renderAssetPreview("ss-favicon");
    }
    if (defaultImageInput) {
      defaultImageInput.addEventListener("input", function () {
        renderAssetPreview("sm-default-image");
      });
      renderAssetPreview("sm-default-image");
    }
    var colorThemeSel = qs("#ss-color-theme");
    if (colorThemeSel) {
      colorThemeSel.addEventListener("change", function () {
        if (colorThemeSel.value === "custom") {
          toggleCustomColorsVisibility();
          cacheAdminThemeFromForm();
          return;
        }
        applyColorPresetToForm(colorThemeSel.value);
        toggleCustomColorsVisibility();
      });
    }
    fetchAuth(API, { method: "GET" })
      .then(function (res) {
        if (res.ok) return res.json();
        return null;
      })
      .then(function (data) {
        if (data) fillForm(data);
      })
      .catch(function (e) {
        if (e && e.message === "unauthorized") return;
      });
    fetchAuth(CLOUD_API, { method: "GET" })
      .then(function (res) {
        if (res.ok) return res.json();
        return null;
      })
      .then(function (data) {
        if (data) fillCloudStorageForm(data);
      })
      .catch(function (e) {
        if (e && e.message === "unauthorized") return;
      });
    fetchAuth(WEB_IMAGE_API, { method: "GET" })
      .then(function (res) {
        if (res.ok) return res.json();
        return null;
      })
      .then(function (data) {
        if (data) fillWebImageProviderForm(data);
      })
      .catch(function (e) {
        if (e && e.message === "unauthorized") return;
      });

    function clearAllTabErrors() {
      setGlobalError(errEl, null);
      setGlobalError(scriptsErrEl, null);
      setGlobalError(articleAdsErrEl, null);
      setGlobalError(metaErrEl, null);
      setGlobalError(r2ErrEl, null);
      setGlobalError(webImagesErrEl, null);
    }

    function saveMainSiteSettings(successToastMsg, actionBtn) {
      var body = buildPayload();
      if (actionBtn) {
        actionBtn.disabled = true;
        actionBtn.setAttribute("aria-busy", "true");
      }
      return fetchAuth(API, {
        method: "PUT",
        body: JSON.stringify(body),
      })
        .then(async function (res) {
          if (res.ok) {
            var data = await res.json().catch(function () {
              return null;
            });
            if (data) fillForm(data);
            if (successToastMsg) showSuccessToast(successToastMsg);
            return true;
          }
          var msg = "Gagal menyimpan (" + res.status + ")";
          try {
            var j = await res.json();
            if (j && j.error) msg = j.error;
          } catch (x) {}
          setGlobalError(errEl, msg);
          return false;
        })
        .catch(function (e) {
          if (e && e.message === "unauthorized") return false;
          setGlobalError(
            errEl,
            "Terjadi gangguan jaringan. Periksa koneksi lalu coba lagi."
          );
          return false;
        })
        .finally(function () {
          if (actionBtn) {
            actionBtn.disabled = false;
            actionBtn.removeAttribute("aria-busy");
          }
        });
    }

    initSiteSettingSubtabs(form, function (btn, message) {
      clearAllTabErrors();
      saveMainSiteSettings(message, btn);
    });
    syncPostDateRefreshFieldsVisibility();
    syncPostDateUrlRecycleFieldsVisibility();
    syncAlsoReadMaxFieldsVisibility();

    if (form) {
      form.addEventListener("submit", function (e) {
        e.preventDefault();
        clearAllTabErrors();

        saveMainSiteSettings("Pengaturan site berhasil disimpan.", submitBtn);
      });
    }

    var scraperAutoTagInput = qs("#ss-scraper-auto-tag");
    var autoHrefEnabledInput = qs("#ss-auto-href-enabled");
    var alsoReadEnabledInput = qs("#ss-also-read-enabled");
    function bindToggleAutoSave(inputEl, successMsg) {
      if (!inputEl) return;
      var timer = null;
      inputEl.addEventListener("change", function () {
        clearTimeout(timer);
        timer = setTimeout(function () {
          clearAllTabErrors();
          saveMainSiteSettings(successMsg).then(function (ok) {
            if (!ok) {
              fetchAuth(API, { method: "GET" })
                .then(function (res) {
                  if (res.ok) return res.json();
                  return null;
                })
                .then(function (data) {
                  if (data) fillForm(data);
                })
                .catch(function () {});
            }
          });
        }, 200);
      });
    }
    bindToggleAutoSave(
      scraperAutoTagInput,
      "Auto-tag scraper berhasil disimpan."
    );
    bindToggleAutoSave(
      autoHrefEnabledInput,
      "Pengaturan Auto-link berhasil disimpan."
    );
    if (alsoReadEnabledInput) {
      alsoReadEnabledInput.addEventListener("change", function () {
        syncAlsoReadMaxFieldsVisibility();
      });
    }
    bindToggleAutoSave(
      alsoReadEnabledInput,
      "Pengaturan Baca juga berhasil disimpan."
    );
    var alsoReadMaxInsertsInput = qs("#ss-also-read-max-inserts");
    if (alsoReadMaxInsertsInput) {
      var alsoReadMaxSaveTimer = null;
      alsoReadMaxInsertsInput.addEventListener("change", function () {
        alsoReadMaxInsertsInput.value = String(
          clampAlsoReadMaxInserts(alsoReadMaxInsertsInput.value)
        );
        clearTimeout(alsoReadMaxSaveTimer);
        alsoReadMaxSaveTimer = setTimeout(function () {
          clearAllTabErrors();
          saveMainSiteSettings(
            "Jumlah maksimal Baca juga berhasil disimpan."
          ).then(function (ok) {
            if (!ok) {
              fetchAuth(API, { method: "GET" })
                .then(function (res) {
                  if (res.ok) return res.json();
                  return null;
                })
                .then(function (data) {
                  if (data) fillForm(data);
                })
                .catch(function () {});
            }
          });
        }, 400);
      });
    }

    var imgResizeEnabledInput = qs("#ss-img-resize-enabled");
    if (imgResizeEnabledInput) {
      var imgResizeToggleSaveTimer = null;
      imgResizeEnabledInput.addEventListener("change", function () {
        syncImgProxyFieldsVisibility();
        clearTimeout(imgResizeToggleSaveTimer);
        imgResizeToggleSaveTimer = setTimeout(function () {
          clearAllTabErrors();
          saveMainSiteSettings("Pengaturan resize /img berhasil disimpan.").then(function (ok) {
            if (!ok) {
              fetchAuth(API, { method: "GET" })
                .then(function (res) {
                  if (res.ok) return res.json();
                  return null;
                })
                .then(function (data) {
                  if (data) fillForm(data);
                })
                .catch(function () {});
            }
          });
        }, 200);
      });
    }

    var imageResizeEnabledInput = qs("#ss-image-resize-enabled");
    if (imageResizeEnabledInput) {
      var imageResizeToggleSaveTimer = null;
      imageResizeEnabledInput.addEventListener("change", function () {
        syncCdnPostImageFieldsVisibility();
        clearTimeout(imageResizeToggleSaveTimer);
        imageResizeToggleSaveTimer = setTimeout(function () {
          clearAllTabErrors();
          saveMainSiteSettings("Pengaturan resize Cloudflare berhasil disimpan.").then(function (ok) {
            if (!ok) {
              fetchAuth(API, { method: "GET" })
                .then(function (res) {
                  if (res.ok) return res.json();
                  return null;
                })
                .then(function (data) {
                  if (data) fillForm(data);
                })
                .catch(function () {});
            }
          });
        }, 200);
      });
    }

    var postDateRefreshEnabledInput = qs("#ss-post-date-refresh-enabled");
    if (postDateRefreshEnabledInput) {
      var postDateRefreshToggleSaveTimer = null;
      postDateRefreshEnabledInput.addEventListener("change", function () {
        function revertToOff() {
          postDateRefreshEnabledInput.checked = false;
          syncPostDateRefreshFieldsVisibility();
        }
        function revertToOn() {
          postDateRefreshEnabledInput.checked = true;
          syncPostDateRefreshFieldsVisibility();
        }
        function proceedSave() {
          syncPostDateRefreshFieldsVisibility();
          clearTimeout(postDateRefreshToggleSaveTimer);
          postDateRefreshToggleSaveTimer = setTimeout(function () {
            clearAllTabErrors();
            saveMainSiteSettings("Pengaturan refresh tanggal post berhasil disimpan.").then(
              function (ok) {
                if (!ok) {
                  fetchAuth(API, { method: "GET" })
                    .then(function (res) {
                      if (res.ok) return res.json();
                      return null;
                    })
                    .then(function (data) {
                      if (data) fillForm(data);
                    })
                    .catch(function () {});
                }
              }
            );
          }, 200);
        }

        if (postDateRefreshEnabledInput.checked) {
          if (typeof Swal !== "undefined") {
            Swal.fire({
              title: "Aktifkan refresh tanggal post?",
              html: "Post terbit yang lebih lama dari umur yang Anda atur akan memiliki <code>updated_at</code> yang di-refresh secara terjadwal. Lanjutkan?",
              icon: "warning",
              showCancelButton: true,
              focusCancel: true,
              confirmButtonText: "Aktifkan",
              cancelButtonText: "Batal",
            }).then(function (result) {
              if (result.isConfirmed) {
                proceedSave();
              } else {
                revertToOff();
              }
            });
          } else {
            if (
              window.confirm(
                "Aktifkan refresh tanggal post? Post terbit yang lebih lama dari umur yang Anda atur akan di-refresh tanggal updatenya secara terjadwal."
              )
            ) {
              proceedSave();
            } else {
              revertToOff();
            }
          }
          return;
        }

        if (typeof Swal !== "undefined") {
          Swal.fire({
            title: "Nonaktifkan refresh tanggal post?",
            html: "Refresh otomatis <code>updated_at</code> untuk post terbit lama akan berhenti. Pengaturan hari dan interval tetap disimpan untuk dipakai lagi nanti.",
            icon: "question",
            showCancelButton: true,
            focusCancel: true,
            confirmButtonText: "Nonaktifkan",
            cancelButtonText: "Batal",
          }).then(function (result) {
            if (result.isConfirmed) {
              proceedSave();
            } else {
              revertToOn();
            }
          });
        } else {
          if (
            window.confirm(
              "Nonaktifkan refresh tanggal post? Post lama tidak akan mendapat update tanggal otomatis sampai Anda mengaktifkannya lagi."
            )
          ) {
            proceedSave();
          } else {
            revertToOn();
          }
        }
      });
    }

    var postDateUrlRecycleEnabledInput = qs("#ss-post-date-url-recycle-enabled");
    if (postDateUrlRecycleEnabledInput) {
      var postDateUrlRecycleToggleTimer = null;
      postDateUrlRecycleEnabledInput.addEventListener("change", function () {
        syncPostDateUrlRecycleFieldsVisibility();
        clearTimeout(postDateUrlRecycleToggleTimer);
        postDateUrlRecycleToggleTimer = setTimeout(function () {
          clearAllTabErrors();
          saveMainSiteSettings("Pengaturan recycle URL berhasil disimpan.").then(function (ok) {
            if (!ok) {
              fetchAuth(API, { method: "GET" })
                .then(function (res) {
                  if (res.ok) return res.json();
                  return null;
                })
                .then(function (data) {
                  if (data) fillForm(data);
                })
                .catch(function () {});
            }
          });
        }, 200);
      });
    }
    var postDateUrlRecycleIntervalInput = qs(
      "#ss-post-date-url-recycle-interval-mins"
    );
    if (postDateUrlRecycleIntervalInput) {
      var urlRecycleIntTimer = null;
      postDateUrlRecycleIntervalInput.addEventListener("change", function () {
        clearTimeout(urlRecycleIntTimer);
        urlRecycleIntTimer = setTimeout(function () {
          clearAllTabErrors();
          saveMainSiteSettings("Interval recycle URL berhasil disimpan.");
        }, 200);
      });
    }
    var postDateUrlRecycleRunBtn = qs("#ss-post-date-url-recycle-run");
    if (postDateUrlRecycleRunBtn) {
      postDateUrlRecycleRunBtn.addEventListener("click", function () {
        clearAllTabErrors();
        var urlsInput = qs("#ss-post-date-url-recycle-urls");
        var raw = (urlsInput && urlsInput.value) || "";
        if (!raw.trim()) {
          setGlobalError(errEl, "Tambahkan minimal satu URL (satu per baris) terlebih dulu.");
          return;
        }
        postDateUrlRecycleRunBtn.disabled = true;
        postDateUrlRecycleRunBtn.setAttribute("aria-busy", "true");
        fetchAuth(URL_RECYCLE_RUN_API, {
          method: "POST",
          body: JSON.stringify({ urls: raw }),
        })
          .then(async function (res) {
            if (res.status === 202 || res.ok) {
              var queued = 0;
              try {
                var data = await res.json();
                if (data && typeof data.queued === "number") queued = data.queued;
              } catch (x) {}
              showSuccessToast(
                queued > 0
                  ? queued + " URL masuk antrean recycle."
                  : "Recycle URL masuk antrean."
              );
              return;
            }
            var msg = "Gagal memasukkan ke antrean (" + res.status + ")";
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
              "Terjadi gangguan jaringan. Periksa koneksi lalu coba lagi."
            );
          })
          .finally(function () {
            postDateUrlRecycleRunBtn.disabled = false;
            postDateUrlRecycleRunBtn.removeAttribute("aria-busy");
          });
      });
    }

    var sourceDisclaimerModeInput = qs("#ss-source-disclaimer-mode");
    if (sourceDisclaimerModeInput) {
      sourceDisclaimerModeInput.addEventListener("change", function () {
        syncSourceDisclaimerTemplateVisibility();
      });
    }

    var imageCaptionEnabledInput = qs("#ss-image-caption-enabled");
    if (imageCaptionEnabledInput) {
      imageCaptionEnabledInput.addEventListener("change", function () {
        syncImageCaptionFieldsVisibility();
      });
    }
    var imageCaptionModeInput = qs("#ss-image-caption-mode");
    if (imageCaptionModeInput) {
      imageCaptionModeInput.addEventListener("change", function () {
        syncImageCaptionFieldsVisibility();
      });
    }

    if (scriptsForm) {
      scriptsForm.addEventListener("submit", function (e) {
        e.preventDefault();
        clearAllTabErrors();
        var body = buildPayload();

        if (scriptsSubmitBtn) {
          scriptsSubmitBtn.disabled = true;
          scriptsSubmitBtn.setAttribute("aria-busy", "true");
        }

        fetchAuth(SCRIPTS_API, {
          method: "PUT",
          body: JSON.stringify(body),
        })
          .then(async function (res) {
            if (res.ok) {
              var data = await res.json().catch(function () {
                return null;
              });
              if (data) fillForm(data);
              showSuccessToast("Script site berhasil disimpan.");
              return;
            }
            var msg = "Gagal menyimpan (" + res.status + ")";
            try {
              var j = await res.json();
              if (j && j.error) msg = j.error;
            } catch (x) {}
            setGlobalError(scriptsErrEl, msg);
          })
          .catch(function (e) {
            if (e && e.message === "unauthorized") return;
            setGlobalError(
              scriptsErrEl,
              "Terjadi gangguan jaringan. Periksa koneksi lalu coba lagi."
            );
          })
          .finally(function () {
            if (scriptsSubmitBtn) {
              scriptsSubmitBtn.disabled = false;
              scriptsSubmitBtn.removeAttribute("aria-busy");
            }
          });
      });
    }

    var initialArticleAds = readArticleAdsFromHidden();
    if (articleAdsCountEl) {
      articleAdsCountEl.value = String(initialArticleAds.length || 0);
    }
    renderArticleAdCards(initialArticleAds.length, initialArticleAds, false);

    if (articleAdsCountEl) {
      articleAdsCountEl.addEventListener("change", function () {
        var n = clampInt(articleAdsCountEl.value, 0, 10);
        renderArticleAdCards(n, null, true);
      });
    }

    if (articleAdsForm) {
      articleAdsForm.addEventListener("submit", function (e) {
        e.preventDefault();
        clearAllTabErrors();
        var body = buildPayload();
        var ads = buildArticleAdsPayload();
        var i;
        for (i = 0; i < ads.length; i++) {
          if (ads[i].position < 1 || ads[i].position > 99) {
            setGlobalError(
              articleAdsErrEl,
              "Ad position #" + (i + 1) + " must be between 1 and 99."
            );
            return;
          }
        }
        body.article_ads = ads;

        if (articleAdsSubmitBtn) {
          articleAdsSubmitBtn.disabled = true;
          articleAdsSubmitBtn.setAttribute("aria-busy", "true");
        }

        fetchAuth(SCRIPTS_API, {
          method: "PUT",
          body: JSON.stringify(body),
        })
          .then(async function (res) {
            if (res.ok) {
              var data = await res.json().catch(function () {
                return null;
              });
              if (data) fillForm(data);
              showSuccessToast("Iklan artikel berhasil disimpan.");
              return;
            }
            var msg = "Gagal menyimpan (" + res.status + ")";
            try {
              var j = await res.json();
              if (j && j.error) msg = j.error;
            } catch (x) {}
            setGlobalError(articleAdsErrEl, msg);
          })
          .catch(function (e) {
            if (e && e.message === "unauthorized") return;
            setGlobalError(
              articleAdsErrEl,
              "Terjadi gangguan jaringan. Periksa koneksi lalu coba lagi."
            );
          })
          .finally(function () {
            if (articleAdsSubmitBtn) {
              articleAdsSubmitBtn.disabled = false;
              articleAdsSubmitBtn.removeAttribute("aria-busy");
            }
          });
      });
    }

    if (metaForm) {
      metaForm.addEventListener("submit", function (e) {
        e.preventDefault();
        clearAllTabErrors();
        var body = buildPayload();

        if (metaSubmitBtn) {
          metaSubmitBtn.disabled = true;
          metaSubmitBtn.setAttribute("aria-busy", "true");
        }

        fetchAuth(API, {
          method: "PUT",
          body: JSON.stringify(body),
        })
          .then(async function (res) {
            if (res.ok) {
              var data = await res.json().catch(function () {
                return null;
              });
              if (data) fillForm(data);
              showSuccessToast("Meta site berhasil disimpan.");
              return;
            }
            var msg = "Gagal menyimpan (" + res.status + ")";
            try {
              var j = await res.json();
              if (j && j.error) msg = j.error;
            } catch (x) {}
            setGlobalError(metaErrEl, msg);
          })
          .catch(function (e) {
            if (e && e.message === "unauthorized") return;
            setGlobalError(
              metaErrEl,
              "Terjadi gangguan jaringan. Periksa koneksi lalu coba lagi."
            );
          })
          .finally(function () {
            if (metaSubmitBtn) {
              metaSubmitBtn.disabled = false;
              metaSubmitBtn.removeAttribute("aria-busy");
            }
          });
      });
    }

    if (r2Form) {
      r2Form.addEventListener("submit", function (e) {
        e.preventDefault();
        clearAllTabErrors();
        var body = buildCloudStoragePayload();

        if (r2SubmitBtn) {
          r2SubmitBtn.disabled = true;
          r2SubmitBtn.setAttribute("aria-busy", "true");
        }

        fetchAuth(CLOUD_API, {
          method: "PUT",
          body: JSON.stringify(body),
        })
          .then(async function (res) {
            if (res.ok) {
              var data = await res.json().catch(function () {
                return null;
              });
              if (data) fillCloudStorageForm(data);
              showSuccessToast("Pengaturan Cloudflare R2 berhasil disimpan.");
              return;
            }
            var msg = "Gagal menyimpan (" + res.status + ")";
            try {
              var j = await res.json();
              if (j && j.error) msg = j.error;
            } catch (x) {}
            setGlobalError(r2ErrEl, msg);
          })
          .catch(function (e) {
            if (e && e.message === "unauthorized") return;
            setGlobalError(
              r2ErrEl,
              "Terjadi gangguan jaringan. Periksa koneksi lalu coba lagi."
            );
          })
          .finally(function () {
            if (r2SubmitBtn) {
              r2SubmitBtn.disabled = false;
              r2SubmitBtn.removeAttribute("aria-busy");
            }
          });
      });
    }

    if (webImagesForm) {
      webImagesForm.addEventListener("submit", function (e) {
        e.preventDefault();
        clearAllTabErrors();
        var body = buildWebImageProviderPayload();

        if (webImagesSubmitBtn) {
          webImagesSubmitBtn.disabled = true;
          webImagesSubmitBtn.setAttribute("aria-busy", "true");
        }

        fetchAuth(WEB_IMAGE_API, {
          method: "PUT",
          body: JSON.stringify(body),
        })
          .then(async function (res) {
            if (res.ok) {
              var data = await res.json().catch(function () {
                return null;
              });
              if (data) fillWebImageProviderForm(data);
              showSuccessToast("Pengaturan API gambar web berhasil disimpan.");
              return;
            }
            var msg = "Gagal menyimpan (" + res.status + ")";
            try {
              var j = await res.json();
              if (j && j.error) msg = j.error;
            } catch (x) {}
            setGlobalError(webImagesErrEl, msg);
          })
          .catch(function (e) {
            if (e && e.message === "unauthorized") return;
            setGlobalError(
              webImagesErrEl,
              "Terjadi gangguan jaringan. Periksa koneksi lalu coba lagi."
            );
          })
          .finally(function () {
            if (webImagesSubmitBtn) {
              webImagesSubmitBtn.disabled = false;
              webImagesSubmitBtn.removeAttribute("aria-busy");
            }
          });
      });
    }
  });
})();
