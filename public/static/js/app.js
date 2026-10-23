(function () {
  "use strict";

  var api = window.TirtayasaAPI;
  if (!api) return;
  var BLANK_IMG =
    "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==";
  var initialMetaTitle = document.title || "";
  var localTimezone = resolvePublicTimezone();
  window.TirtayasaLocalTimezone = localTimezone;

  function browserTitleSuffix() {
    try {
      var cfg = window.TirtayasaPublicConfig || {};
      var sn = String(cfg.siteName || "").trim();
      return sn ? " - " + sn : "";
    } catch (e) {
      return "";
    }
  }

  function resolvePublicTimezone() {
    var fallback = "Asia/Jakarta";
    var tz = "";
    try {
      var cfg = window.TirtayasaPublicConfig || {};
      tz = String(cfg.localTimezone || "").trim();
    } catch (e) {}
    if (!tz) tz = fallback;
    try {
      // Validate IANA timezone supaya formatter tidak melempar error.
      Intl.DateTimeFormat("id-ID", { timeZone: tz }).format(new Date());
      return tz;
    } catch (e) {
      return fallback;
    }
  }

  function setMetaTitle(nextTitle) {
    var title = String(nextTitle || "").trim();
    if (!title) return;
    document.title = title;
    var selectors = [
      'meta[name="title"]',
      'meta[property="og:title"]',
      'meta[name="twitter:title"]',
    ];
    selectors.forEach(function (sel) {
      var node = document.querySelector(sel);
      if (node) node.setAttribute("content", title);
    });
  }

  function escapeHtml(s) {
    if (s == null) return "";
    var d = document.createElement("div");
    d.textContent = s;
    return d.innerHTML;
  }

  var webImageProviderLabels = {
    pexels: "Pexels",
    unsplash: "Unsplash",
    pixabay: "Pixabay",
    openverse: "Openverse",
    wikimedia: "Wikimedia Commons",
    wordpress: "WordPress",
  };

  function webImageProviderDisplayName(provider) {
    var p = String(provider || "")
      .trim()
      .toLowerCase();
    if (!p || p === "auto") return "";
    if (webImageProviderLabels[p]) return webImageProviderLabels[p];
    return p.charAt(0).toUpperCase() + p.slice(1);
  }

  function scrapeSourceLabel(articleSource, detail) {
    detail = String(detail || "").trim();
    if (detail) {
      try {
        var u = new URL(detail);
        if (u.host) return u.host;
      } catch (e) {
        return detail;
      }
    }
    articleSource = String(articleSource || "").trim();
    if (!articleSource) return "";
    try {
      return new URL(articleSource).host || articleSource;
    } catch (e) {
      return articleSource;
    }
  }

  function buildCustomHeroCaptionInnerHTML(post, title, tpl) {
    tpl = String(tpl || "").trim();
    if (!tpl) tpl = "Foto: Ilustrasi {{title}}.";
    title = String(title || (post && post.title) || "").trim();
    var provider = webImageProviderDisplayName(
      post && post.image_origin_detail ? post.image_origin_detail : ""
    );
    var source = scrapeSourceLabel(
      post && post.source ? post.source : "",
      post && post.image_origin_detail ? post.image_origin_detail : ""
    );
    var creditHTML = "";
    if (provider) {
      creditHTML =
        ' <span class="blog-article-hero-caption-credit">(Illustration by ' +
        escapeHtml(provider) +
        ")</span>";
    }
    var out = escapeHtml(tpl);
    out = out.split("{{title}}").join(escapeHtml(title));
    out = out.split("{{provider}}").join(escapeHtml(provider));
    out = out.split("{{source}}").join(escapeHtml(source));
    out = out.split("{{credit}}").join(creditHTML);
    return out;
  }

  function buildDefaultHeroCaptionInnerHTML(post, title) {
    var origin = String(post.image_origin || "")
      .trim()
      .toLowerCase();
    title = String(title || post.title || "").trim();
    if (!origin) {
      if (!title) return "";
      return "Foto: Ilustrasi " + escapeHtml(title) + ".";
    }
    if (origin === "web_search") {
      if (!title) return "";
      var provider = webImageProviderDisplayName(post.image_origin_detail);
      var inner = "Foto: " + escapeHtml(title) + ".";
      if (provider) {
        inner +=
          ' <span class="blog-article-hero-caption-credit">(Illustration by ' +
          escapeHtml(provider) +
          ")</span>";
      }
      return inner;
    }
    if (origin === "ai" || origin === "template" || origin === "manual") {
      if (!title) return "";
      return "Foto: Ilustrasi " + escapeHtml(title) + ".";
    }
    if (origin === "scrape") {
      if (!title) return "";
      return "Foto: " + escapeHtml(title) + ".";
    }
    return "";
  }

  function buildHeroCaptionInnerHTML(post, title) {
    try {
      var cfg = window.TirtayasaPublicConfig || {};
      if (!cfg.imageCaptionEnabled) return "";
      if (!post || !post.image) return "";
      if (
        String(cfg.imageCaptionMode || "")
          .toLowerCase()
          .trim() === "custom"
      ) {
        return buildCustomHeroCaptionInnerHTML(
          post,
          title,
          cfg.imageCaptionTemplate
        );
      }
      return buildDefaultHeroCaptionInnerHTML(post, title);
    } catch (e) {
      return "";
    }
  }

  /** Ringkasan meta / konten untuk kartu Megazine-home (SSR & muat lagi). */
  function postCardExcerpt(p, max) {
    max = max || 200;
    var md =
      p.meta_description != null ? String(p.meta_description).trim() : "";
    var raw = md;
    if (!raw) {
      raw = (p.content || "").replace(/<[^>]+>/g, " ");
      raw = raw.replace(/\s+/g, " ").trim();
    }
    if (raw.length > max) raw = raw.slice(0, max) + "\u2026";
    return escapeHtml(raw);
  }

  function postAuthorDisplayPublic(p) {
    var u = p.author || {};
    var nm = String(u.name != null ? u.name : "").trim();
    if (nm) return nm;
    return String(u.username != null ? u.username : "").trim();
  }

  function postCalendarLabelPublic(p) {
    var iso = postTimeIso(p);
    if (!iso) return "";
    var d = new Date(iso);
    if (isNaN(d.getTime())) return "";
    try {
      return d.toLocaleDateString("id-ID", {
        day: "numeric",
        month: "long",
        year: "numeric",
        timeZone: window.TirtayasaLocalTimezone || "Asia/Jakarta",
      });
    } catch (e) {
      return "";
    }
  }

  function formatViewLabel(n) {
    var v = n != null ? Number(n) : 0;
    if (!isFinite(v) || v <= 0) return "";
    var s = String(Math.floor(Math.abs(v)));
    var pre = s.length % 3;
    if (pre === 0) pre = 3;
    var out = "";
    while (s.length) {
      if (out) out += ".";
      var take = pre;
      if (take > s.length) take = s.length;
      out += s.slice(0, take);
      s = s.slice(take);
      pre = 3;
    }
    return out + " tayangan";
  }

  function isPublished(p) {
    return (p.status || "").toLowerCase() === "published";
  }

  /** URL artikel kanonik (sesuai site article_slug_mode). */
  function postCategoryPathPrefix(p, mode) {
    if (!p || !p.category) return "";
    var cat = p.category;
    var slug = cat.slug ? String(cat.slug).trim() : "";
    if (!slug) return "";
    var parentSlug = cat.parent_slug
      ? String(cat.parent_slug).trim()
      : cat.parentSlug
      ? String(cat.parentSlug).trim()
      : cat.parent && cat.parent.slug
      ? String(cat.parent.slug).trim()
      : "";
    if (mode === "category_nested" && parentSlug) {
      return encodeURIComponent(parentSlug) + "/" + encodeURIComponent(slug);
    }
    return encodeURIComponent(slug);
  }

  function postPublicPath(p) {
    if (!p || !p.slug) return "/";
    var s = String(p.slug).trim();
    if (!s) return "/";
    var mode = "";
    try {
      mode = String((window.TirtayasaPublicConfig || {}).articleSlugMode || "")
        .toLowerCase()
        .trim();
    } catch (e) {}
    if (mode === "category" || mode === "category_nested") {
      var prefix = postCategoryPathPrefix(p, mode);
      if (prefix) {
        return "/" + prefix + "/" + encodeURIComponent(s);
      }
    }
    return "/" + encodeURIComponent(s);
  }

  function categoryPublicPath(slug) {
    if (!slug) return "/";
    return "/" + encodeURIComponent(String(slug).trim());
  }

  function slugifyText(v) {
    return String(v || "")
      .toLowerCase()
      .trim()
      .replace(/['"]/g, "")
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "");
  }

  function authorSlugFromUser(u) {
    if (!u) return "";
    var un = slugifyText(u.username || "");
    if (un) return un;
    var em = String(u.email || "").trim();
    if (em) {
      var local = em.split("@")[0] || "";
      return slugifyText(local);
    }
    return "";
  }

  function authorPublicPath(u) {
    var s = authorSlugFromUser(u);
    if (!s) return "/posts";
    return "/author/" + encodeURIComponent(s);
  }

  function formatAuthorRole(role) {
    var r = String(role || "")
      .toLowerCase()
      .trim();
    if (r === "admin") return "Administrator";
    if (r === "writer") return "Penulis";
    if (!r) return "Kontributor";
    return String(role || "").trim();
  }

  function postAuthorMatchesSlug(p, slug) {
    if (!p || !p.author || !slug) return false;
    var target = slugifyText(slug);
    if (!target) return false;
    var u = p.author;
    var canonical = authorSlugFromUser(u);
    if (canonical && target === canonical) return true;
    var role = slugifyText(u.role || "");
    var name = slugifyText(u.name || "");
    var emailLocal = "";
    try {
      var em = String(u.email || "").trim();
      emailLocal = slugifyText((em.split("@")[0] || "").trim());
    } catch (e) {}
    return (
      target === role ||
      target === name ||
      (emailLocal && target === emailLocal)
    );
  }

  function normalizePathForCompare(s) {
    s = String(s || "").trim();
    if (!s || s === "/") return "/";
    return s.replace(/\/+$/, "");
  }

  function pathIsLoginPage(path) {
    var n = normalizePathForCompare(path);
    var lh = "";
    try {
      lh =
        (window.TirtayasaPublicConfig && window.TirtayasaPublicConfig.loginHref) || "";
    } catch (e) {}
    lh = String(lh || "").trim();
    if (!lh) lh = "/login";
    if (/^https?:\/\//i.test(lh)) return false;
    return n === normalizePathForCompare(lh);
  }

  function parseRoute() {
    var path = location.pathname || "/";
    var mPath = /^\/post\/([^/]+)\/?$/.exec(path);
    if (mPath) {
      return { name: "post", slug: decodeURIComponent(mPath[1]) };
    }

    if (/^\/posts\/?$/.test(path)) {
      var sp = new URLSearchParams(location.search);
      var pg = parseInt(sp.get("page") || "1", 10);
      if (isNaN(pg) || pg < 1) pg = 1;
      return { name: "posts", page: pg };
    }

    if (pathIsLoginPage(path)) {
      return { name: "login" };
    }

    var h = (location.hash || "").replace(/^#/, "");
    if (pathIsLoginPage(h)) return { name: "login" };
    var mHashNum = /^\/post\/(\d+)$/.exec(h);
    if (mHashNum) return { name: "post", id: parseInt(mHashNum[1], 10) };

    if (path === "/" || path === "") return { name: "home" };

    var mTwo = /^\/([^/]+)\/([^/]+)\/?$/.exec(path);
    if (mTwo) {
      var segA = decodeURIComponent(mTwo[1]);
      var segB = decodeURIComponent(mTwo[2]);
      if (
        segA === "posts" ||
        segA === "rss-feeds" ||
        segA === "login" ||
        segA === "tag" ||
        segA === "author" ||
        segA === "admin" ||
        segA === "api" ||
        segA === "static" ||
        segA === "rss" ||
        segA === "halaman" ||
        segA === "sitemap.xml" ||
        segA === "favicon.ico" ||
        segA === "robots.txt" ||
        segA === "ads.txt"
      ) {
        return { name: "home" };
      }
      return { name: "post", slug: segB, categorySlug: segA };
    }

    var mSeg = /^\/([^/]+)\/?$/.exec(path);
    if (mSeg) {
      var seg = decodeURIComponent(mSeg[1]);
      if (seg === "posts" || seg === "rss-feeds" || seg === "login") {
        return { name: "home" };
      }
      return { name: "segment", slug: seg };
    }

    return { name: "home" };
  }

  var PUBLIC_SEARCH_MAX_LEN = 100;

  function normalizeSearchQuery(raw) {
    var q = String(raw || "").trim();
    if (!q) return "";
    if (q.length > PUBLIC_SEARCH_MAX_LEN) {
      q = q.slice(0, PUBLIC_SEARCH_MAX_LEN);
    }
    return q;
  }

  function getQuery() {
    var p = new URLSearchParams(location.search);
    return {
      q: normalizeSearchQuery(p.get("q") || ""),
      category: (p.get("category") || "").trim(),
      tag: (p.get("tag") || "").trim(),
    };
  }

  function postsFilterBootstrapPath(query) {
    var url = "/posts?size=100&page=1";
    if (query && query.q) {
      url += "&q=" + encodeURIComponent(query.q);
    }
    return url;
  }

  function firstImgSrc(html) {
    if (!html || typeof html !== "string") return "";
    var m = html.match(/<img[^>]+src\s*=\s*["']([^"']+)["']/i);
    return m ? m[1].trim() : "";
  }

  function selfHostedImgResizeEnabled() {
    try {
      var cfg = window.TirtayasaPublicConfig || {};
      return cfg.imgResizeEnabled === true;
    } catch (e) {
      return false;
    }
  }

  function imageResizeProxyPath() {
    try {
      var p = (window.TirtayasaPublicConfig || {}).imageResizePath;
      if (p && String(p).trim()) return String(p).trim();
    } catch (e) {}
    return "/img";
  }

  function normalizeImgSrcWithConfig(src, width, quality, height) {
    if (!src) return BLANK_IMG;
    if (/^data:/i.test(String(src))) return src;
    var out = "";
    if (/^https?:\/\//i.test(src)) out = src;
    else if (src.charAt(0) === "/") out = src;
    else out = "/" + src;
    if (!selfHostedImgResizeEnabled()) {
      return stripImageResizePrefix(out);
    }
    height = String(height || "").trim();
    if (!width && !height && !quality) return out;
    quality = resolveImgResizeQuality(quality, src);
    if (/^\d+$/.test(width) && Number(width) > 0 && !height && !quality)
      return out;

    var defaultImg = BLANK_IMG;
    if (out === defaultImg || String(out).toLowerCase().endsWith(defaultImg)) {
      return out;
    }
    out = stripImageResizePrefix(out);
    if (!isResizableAsset(out)) return out;

    var params = [];
    if (/^\d+$/.test(width) && Number(width) > 0) params.push("w=" + width);
    if (/^\d+$/.test(height) && Number(height) > 0) params.push("h=" + height);
    if (/^\d+$/.test(quality) && Number(quality) > 0)
      params.push("q=" + quality);
    if (!params.length) return out;

    if (/^https?:\/\//i.test(out)) {
      try {
        var eu = new URL(out);
        var erel = eu.host;
        var epath = (eu.pathname || "").replace(/^\//, "");
        if (epath) erel += "/" + epath;
        var esegs = erel.split("/").map(function (seg) {
          return encodeURIComponent(seg);
        });
        return (
          imageResizeProxyPath() +
          "/" +
          esegs.join("/") +
          "?" +
          params.join("&")
        );
      } catch (eUrl) {
        return out;
      }
    }

    var rel = out.indexOf("/static/") === 0 ? out.slice("/static/".length) : "";
    if (!rel) return out;
    var segs = rel.split("/").map(function (seg) {
      return encodeURIComponent(seg);
    });
    return (
      imageResizeProxyPath() + "/" + segs.join("/") + "?" + params.join("&")
    );
  }

  function isResizableAsset(src) {
    var path = stripImageResizePrefix(src);
    if (/^https?:\/\//i.test(path)) {
      try {
        return !!new URL(path).hostname;
      } catch (e) {
        return false;
      }
    }
    return (
      path.indexOf("/static/uploads/") === 0 ||
      path.indexOf("/static/images/") === 0
    );
  }

  function isLocalStaticResizeRel(rel) {
    return rel.indexOf("uploads/") === 0 || rel.indexOf("images/") === 0;
  }

  function looksLikeRemoteHost(host) {
    host = String(host || "").toLowerCase();
    if (!host || host === "uploads" || host === "images") return false;
    if (host.indexOf(".") >= 0 || host.indexOf(":") >= 0) return true;
    return host === "localhost";
  }

  function unwrapHostPathRemoteRel(rel) {
    var slash = rel.indexOf("/");
    if (slash <= 0) return "";
    var host = rel.slice(0, slash);
    if (!looksLikeRemoteHost(host)) return "";
    var rpath = rel.slice(slash + 1);
    return "https://" + host + (rpath ? "/" + rpath : "");
  }

  function stripImageResizePrefix(raw) {
    var s = String(raw || "").trim();
    if (!s) return s;
    if (/^https?:\/\//i.test(s)) {
      try {
        var absU = new URL(s);
        var proxyPath = imageResizeProxyPath();
        var pn = absU.pathname || "";
        if (pn.toLowerCase().indexOf(proxyPath.toLowerCase() + "/") === 0) {
          return stripImageResizePrefix(
            proxyPath + pn.slice(proxyPath.length) + (absU.search || "")
          );
        }
      } catch (eAbs) {}
    }
    var low = s.toLowerCase();
    var proxy = imageResizeProxyPath().toLowerCase();
    if (low.indexOf(proxy + "/") === 0) {
      try {
        var u = new URL(s, location.origin);
        var rel = (u.pathname || "").slice(proxy.length + 1);
        if (rel) {
          rel = rel
            .split("/")
            .map(function (seg) {
              return decodeURIComponent(seg);
            })
            .join("/");
          if (rel.indexOf("remote/") === 0) {
            var rest = rel.slice("remote/".length);
            var parts = rest.split("/");
            if (parts.length >= 2) {
              var scheme = parts[0];
              var host = parts[1];
              var rpath = parts.slice(2).join("/");
              return scheme + "://" + host + (rpath ? "/" + rpath : "");
            }
          }
          if (isLocalStaticResizeRel(rel)) {
            return "/static/" + rel;
          }
          var remote = unwrapHostPathRemoteRel(rel);
          if (remote) return remote;
          return s;
        }
      } catch (e) {}
    }
    if (low.indexOf(proxy + "?") === 0) {
      try {
        var u2 = new URL(s, location.origin);
        var inner = (u2.searchParams.get("src") || "").trim();
        if (inner) return inner;
      } catch (e2) {}
    }
    var marker = "/cdn-cgi/image/";
    var idx = low.indexOf(marker);
    if (idx < 0) return s;
    var rest = s.slice(idx + marker.length);
    var slash = rest.indexOf("/");
    if (slash < 0) return s;
    return s.slice(0, idx) + rest.slice(slash);
  }

  function readCDNImageResizeOption(raw, key) {
    var s = String(raw || "").trim();
    if (!s) return "";
    var want = String(key || "")
      .toLowerCase()
      .trim();
    var proxy = imageResizeProxyPath().toLowerCase();
    if (
      s.toLowerCase().indexOf(proxy + "/") === 0 ||
      s.toLowerCase().indexOf(proxy + "?") === 0
    ) {
      try {
        var u = new URL(s, location.origin);
        if (want === "width" || want === "w")
          return (u.searchParams.get("w") || "").trim();
        if (want === "height" || want === "h")
          return (u.searchParams.get("h") || "").trim();
        if (want === "quality" || want === "q")
          return (u.searchParams.get("q") || "").trim();
      } catch (e) {}
    }
    var low = s.toLowerCase();
    var marker = "/cdn-cgi/image/";
    var idx = low.indexOf(marker);
    if (idx < 0) return "";
    var rest = s.slice(idx + marker.length);
    var slash = rest.indexOf("/");
    if (slash < 0) return "";
    var opts = rest.slice(0, slash).split(",");
    for (var i = 0; i < opts.length; i++) {
      var pair = String(opts[i] || "").split("=");
      if (pair.length < 2) continue;
      if (
        String(pair[0] || "")
          .toLowerCase()
          .trim() !== want
      )
        continue;
      var val = String(pair.slice(1).join("=") || "").trim();
      if (val) return val;
    }
    return "";
  }

  function normalizeImgSrc(src) {
    var quality = "";
    try {
      var cfg = window.TirtayasaPublicConfig || {};
      quality = String(
        cfg.imgResizeQuality || cfg.imageResizeQuality || ""
      ).trim();
    } catch (e) {}
    return normalizeImgSrcWithConfig(src, "", quality);
  }

  function configuredImgResizeMaxWidth() {
    try {
      var cfg = window.TirtayasaPublicConfig || {};
      return String(cfg.imgResizeWidth || "").trim();
    } catch (e) {
      return "";
    }
  }

  function clampImgResizeWidth(width) {
    if (!/^\d+$/.test(String(width || ""))) return "";
    var n = Number(width);
    if (!(n > 0)) return "";
    var max = configuredImgResizeMaxWidth();
    if (/^\d+$/.test(max) && Number(max) > 0) {
      n = Math.min(n, Number(max));
    }
    return String(Math.max(1, Math.ceil(n)));
  }

  function configuredImgResizeMaxHeight() {
    try {
      var cfg = window.TirtayasaPublicConfig || {};
      return String(cfg.imgResizeHeight || "").trim();
    } catch (e) {
      return "";
    }
  }

  function clampImgResizeHeight(height) {
    if (!/^\d+$/.test(String(height || ""))) return "";
    var n = Number(height);
    if (!(n > 0)) return "";
    var max = configuredImgResizeMaxHeight();
    if (/^\d+$/.test(max) && Number(max) > 0) {
      n = Math.min(n, Number(max));
    }
    return String(Math.max(1, Math.ceil(n)));
  }

  function configuredImgResizeQuality() {
    try {
      var cfg = window.TirtayasaPublicConfig || {};
      return String(
        cfg.imgResizeQuality ||
          cfg.cloudflareImageResizeQuality ||
          cfg.imageResizeQuality ||
          ""
      ).trim();
    } catch (e) {
      return "";
    }
  }

  function resolveImgResizeQuality(quality, rawSrc) {
    var q = String(quality || "").trim();
    if (/^\d+$/.test(q) && Number(q) > 0 && Number(q) <= 100) return q;
    q = configuredImgResizeQuality();
    if (/^\d+$/.test(q) && Number(q) > 0 && Number(q) <= 100) return q;
    q = readCDNImageResizeOption(String(rawSrc || ""), "q");
    if (/^\d+$/.test(q) && Number(q) > 0 && Number(q) <= 100) return q;
    return "";
  }

  function configuredCloudflareImageResizeWidth() {
    try {
      var cfg = window.TirtayasaPublicConfig || {};
      return String(cfg.cloudflareImageResizeWidth || "").trim();
    } catch (e) {
      return "";
    }
  }

  function configuredCloudflareImageResizeQuality() {
    try {
      var cfg = window.TirtayasaPublicConfig || {};
      return String(cfg.cloudflareImageResizeQuality || "").trim();
    } catch (e) {
      return "";
    }
  }

  function containerDisplayAspectRatio(container) {
    if (!container) return 0;
    var cw = container.clientWidth || 0;
    var ch = container.clientHeight || 0;
    if (cw > 0 && ch > 0) return ch / cw;
    try {
      var st = window.getComputedStyle(container);
      var ar = st && st.aspectRatio;
      if (ar && ar !== "auto") {
        var parts = String(ar).split("/");
        if (parts.length === 2) {
          var aw = parseFloat(parts[0]);
          var ah = parseFloat(parts[1]);
          if (aw > 0 && ah > 0) return ah / aw;
        }
        var n = parseFloat(ar);
        if (n > 0) return n;
      }
    } catch (e) {}
    return 0;
  }

  function hasFixedDisplayBox(container, fallbackHeight) {
    if (
      /^\d+$/.test(String(fallbackHeight || "")) &&
      Number(fallbackHeight) > 0
    ) {
      return true;
    }
    if (!container) return false;
    var h = 0;
    try {
      h = Math.ceil(container.getBoundingClientRect().height || 0);
    } catch (e) {}
    if (h > 0) return true;
    if (container.clientHeight > 0) return true;
    return containerDisplayAspectRatio(container) > 0;
  }

  function effectiveImageWidth(container, fallbackWidth, img) {
    var w = 0;
    if (container && container.getBoundingClientRect) {
      try {
        w = Math.ceil(container.getBoundingClientRect().width || 0);
      } catch (e) {}
    }
    if (!w && container && container.clientWidth) {
      w = Math.ceil(container.clientWidth || 0);
    }
    if (!w && img) {
      var attrW = parseInt(img.getAttribute("width") || "", 10);
      if (attrW > 0) w = attrW;
    }
    if (!w && /^\d+$/.test(String(fallbackWidth || ""))) {
      w = Number(fallbackWidth);
    }
    if (w < 1) return "";
    return String(w);
  }

  function effectiveImageHeight(container, fallbackHeight, img, knownWidth) {
    var h = 0;
    if (container && container.getBoundingClientRect) {
      try {
        h = Math.ceil(container.getBoundingClientRect().height || 0);
      } catch (e) {}
    }
    if (!h && container && container.clientHeight) {
      h = Math.ceil(container.clientHeight || 0);
    }
    if (!h && img) {
      var attrH = parseInt(img.getAttribute("height") || "", 10);
      if (attrH > 0) h = attrH;
    }
    if (!h && /^\d+$/.test(String(fallbackHeight || ""))) {
      h = Number(fallbackHeight);
    }
    if (
      !h &&
      /^\d+$/.test(String(knownWidth || "")) &&
      Number(knownWidth) > 0
    ) {
      var ratio = containerDisplayAspectRatio(container);
      if (ratio > 0) {
        h = Math.ceil(Number(knownWidth) * ratio);
      }
    }
    if (!h && img) {
      var attrW = parseInt(img.getAttribute("width") || "", 10);
      var attrH = parseInt(img.getAttribute("height") || "", 10);
      if (attrW > 0 && attrH > 0) {
        var refW2 = parseInt(String(knownWidth || ""), 10);
        if (refW2 > 0) h = Math.ceil((refW2 * attrH) / attrW);
        else h = attrH;
      }
    }
    if (h < 1) return "";
    return String(h);
  }

  function displayedBoxPixels(img, container, fallbackWidth, fallbackHeight) {
    var w = 0;
    var h = 0;
    if (img && img.getBoundingClientRect) {
      try {
        var ir = img.getBoundingClientRect();
        w = Math.ceil(ir.width || 0);
        h = Math.ceil(ir.height || 0);
      } catch (e) {}
    }
    var box = container || (img && img.parentElement);
    if (box && box.getBoundingClientRect) {
      try {
        var cr = box.getBoundingClientRect();
        if (!(w > 0)) w = Math.ceil(cr.width || 0);
        if (!(h > 0)) h = Math.ceil(cr.height || 0);
      } catch (e2) {}
    }
    if (!(w > 0)) {
      w = Number(effectiveImageWidth(box, fallbackWidth, img)) || 0;
    }
    if (!(h > 0)) {
      h =
        Number(
          effectiveImageHeight(box, fallbackHeight, img, w > 0 ? String(w) : "")
        ) || 0;
    }
    return { width: w > 0 ? w : 0, height: h > 0 ? h : 0 };
  }

  function resolveResizeDimensions(
    container,
    img,
    fallbackWidth,
    fallbackHeight,
    multiplier
  ) {
    var mult = Number(multiplier);
    if (!(mult > 0)) mult = 1;
    var ctn = container || (img && img.parentElement);
    var display = displayedBoxPixels(img, ctn, fallbackWidth, fallbackHeight);
    var width = "";
    var height = "";
    if (display.width > 0) {
      width = clampImgResizeWidth(Math.max(1, Math.ceil(display.width * mult)));
    }
    if (hasFixedDisplayBox(ctn, fallbackHeight) && display.height > 0) {
      height = clampImgResizeHeight(
        Math.max(1, Math.ceil(display.height * mult))
      );
    }
    return { width: width, height: height };
  }

  function bindResponsiveThumbCDN(
    img,
    container,
    fallbackWidth,
    widthMultiplier,
    fallbackHeight
  ) {
    if (!img) return;
    var raw = img.__cdnOriginalSrc || "";
    if (!raw) {
      var currentSrc = img.getAttribute("src") || "";
      var existingQuality = readCDNImageResizeOption(currentSrc, "quality");
      if (existingQuality) {
        img.__cdnOriginalQuality = existingQuality;
      }
      raw = currentSrc;
      raw = stripImageResizePrefix(raw);
      img.__cdnOriginalSrc = raw;
    }
    if (!raw) return;

    var currentSrc = img.getAttribute("src") || "";
    var fallback = String(fallbackWidth || "").trim();
    var multiplier = Number(widthMultiplier);
    if (!(multiplier > 0)) multiplier = 1;

    function update() {
      var dims = resolveResizeDimensions(
        container || img.parentElement,
        img,
        fallback,
        String(fallbackHeight || "").trim(),
        multiplier
      );
      var next = normalizeImgSrcWithConfig(
        raw,
        dims.width,
        resolveImgResizeQuality(
          String(img.__cdnOriginalQuality || "").trim(),
          currentSrc
        ),
        dims.height
      );
      if (next && next !== img.getAttribute("src"))
        img.setAttribute("src", next);
    }

    update();
    if (!img.__imgResizeLoadBound) {
      img.__imgResizeLoadBound = true;
      img.addEventListener("load", update, { passive: true });
    }
    if (typeof requestAnimationFrame !== "undefined") {
      requestAnimationFrame(function () {
        requestAnimationFrame(update);
      });
    } else {
      setTimeout(update, 0);
    }
    if (typeof ResizeObserver !== "undefined") {
      try {
        if (img.__imgResizeObserver) img.__imgResizeObserver.disconnect();
        var ro = new ResizeObserver(function () {
          update();
        });
        if (img) ro.observe(img);
        if (container && container !== img) ro.observe(container);
        img.__imgResizeObserver = ro;
      } catch (eRo) {}
    }
  }

  function bindPageResponsiveImages(scope) {
    if (typeof window.TirtayasaBindPageImages === "function") {
      window.TirtayasaBindPageImages(scope || document);
    }
  }

  function bindResponsiveArticleImages(article) {
    if (!article) return;
    var bodyEl = article.querySelector(".blog-article-body");
    if (bodyEl) {
      bodyEl.querySelectorAll("img[src]").forEach(function (img) {
        bindResponsiveThumbCDN(img, bodyEl, "", 1, "");
      });
    }
    var heroWrap = article.querySelector(".blog-article-hero");
    var heroImg = heroWrap && heroWrap.querySelector("img");
    if (heroImg) {
      bindResponsiveThumbCDN(heroImg, heroWrap, "", 1, "");
    }
  }

  function normalizeAvatarImgSrc(src) {
    var width = "";
    var quality = "";
    try {
      var cfg = window.TirtayasaPublicConfig || {};
      width = String(cfg.avatarResizeWidth || "").trim();
      quality = String(cfg.avatarResizeQuality || "").trim();
    } catch (e) {}
    return normalizeImgSrcWithConfig(src, width, quality);
  }

  /** Thumbnail listing: gambar unggahan admin (`image`), lalu gambar pertama di HTML konten. */
  function postThumbnailSrc(p) {
    if (!p) return BLANK_IMG;
    var u = p.image != null ? String(p.image).trim() : "";
    if (!u) u = firstImgSrc(p.content || "");
    return normalizeImgSrc(u);
  }

  /** Gambar utama halaman artikel: sama seperti thumbnail; kosong jika tidak ada gambar sama sekali. */
  function postHeroSrc(p) {
    if (!p) return "";
    var u = p.image != null ? String(p.image).trim() : "";
    if (!u) u = firstImgSrc(p.content || "");
    if (!u) return "";
    return normalizeImgSrc(u);
  }

  /** ISO tanggal untuk urutan & teks relatif: prioritas updated_at, lalu created_at. */
  function postTimeIso(p) {
    if (!p) return "";
    return (p.updated_at || p.created_at || "").toString().trim();
  }

  var homeRenderGen = 0;

  function dedupePostsById(posts) {
    var seen = Object.create(null);
    var out = [];
    (posts || []).forEach(function (p) {
      if (!p || p.id == null || seen[p.id]) return;
      seen[p.id] = true;
      out.push(p);
    });
    return out;
  }

  function sortByPublishedDesc(posts) {
    return posts.slice().sort(function (a, b) {
      var isoA = postTimeIso(a);
      var isoB = postTimeIso(b);
      var ta = isoA ? new Date(isoA).getTime() : 0;
      var tb = isoB ? new Date(isoB).getTime() : 0;
      return tb - ta;
    });
  }

  function sortPopularByViews(posts) {
    return posts.slice().sort(function (a, b) {
      var va = a.view != null ? Number(a.view) : 0;
      var vb = b.view != null ? Number(b.view) : 0;
      if (vb !== va) return vb - va;
      var isoA = postTimeIso(a);
      var isoB = postTimeIso(b);
      var ta = isoA ? new Date(isoA).getTime() : 0;
      var tb = isoB ? new Date(isoB).getTime() : 0;
      return tb - ta;
    });
  }

  /** Carousel beranda: Artikel terbaru (selain kartu "terbaru" utama jika ada). */
  function selectHomeCarouselSlides(published, latestPost) {
    var recent = (published || []).filter(function (p) {
      return !latestPost || p.id !== latestPost.id;
    });
    if (!recent.length) recent = (published || []).slice();
    return recent.slice(0, Math.min(5, recent.length));
  }

  function isBreakingCandidate(p) {
    var iso = postTimeIso(p);
    if (!iso) return false;
    var t = new Date(iso).getTime();
    if (isNaN(t)) return false;
    return Date.now() - t < 6 * 3600 * 1000;
  }

  function buildFeaturedCarousel(slides) {
    var el = document.createElement("div");
    el.className = "blog-feature-carousel";
    el.setAttribute("role", "region");
    el.setAttribute("aria-label", "Artikel unggulan");

    var viewport = document.createElement("div");
    viewport.className = "blog-feature-viewport";

    var track = document.createElement("div");
    track.className = "blog-feature-track";

    slides.forEach(function (p, i) {
      var slide = document.createElement("article");
      slide.className = "blog-feature-slide";
      slide.setAttribute("aria-hidden", i === 0 ? "false" : "true");

      var href = postPublicPath(p);
      var imgSrc = postThumbnailSrc(p);
      var catName = p.category && p.category.name ? p.category.name : "";
      var catUpper = escapeHtml(String(catName || "").toUpperCase());
      var rel = relativeTimeId(postTimeIso(p));
      var isHot = isBreakingCandidate(p);

      slide.innerHTML =
        '<div class="blog-feature-slide-media">' +
        '<img src="' +
        escapeHtml(imgSrc) +
        '" alt="" decoding="async" loading="' +
        (i === 0 ? "eager" : "lazy") +
        '" />' +
        (isHot
          ? '<span class="blog-feature-badge blog-feature-badge--breaking">BREAKING</span>'
          : "") +
        '<span class="blog-feature-slide-shade" aria-hidden="true"></span>' +
        "</div>" +
        '<a class="blog-feature-slide-hit" href="' +
        escapeHtml(href) +
        '"><span class="blog-visually-hidden">' +
        escapeHtml(p.title || "Buka artikel") +
        "</span></a>" +
        '<div class="blog-feature-slide-caption">' +
        (catName
          ? '<span class="blog-feature-cat">' + catUpper + "</span>"
          : "") +
        '<p class="blog-feature-slide-title">' +
        escapeHtml(p.title || "") +
        "</p>" +
        (rel
          ? '<p class="blog-feature-slide-time">' + escapeHtml(rel) + "</p>"
          : "") +
        "</div>";

      var img = slide.querySelector("img");
      var mediaWrap = slide.querySelector(".blog-feature-slide-media");
      if (img) {
        img.addEventListener(
          "error",
          function onImgErr() {
            img.removeEventListener("error", onImgErr);
            img.src = BLANK_IMG;
          },
          false
        );
        bindLazyImageReveal(mediaWrap, img);
      }

      track.appendChild(slide);
    });

    viewport.appendChild(track);
    el.appendChild(viewport);

    var prev = document.createElement("button");
    prev.type = "button";
    prev.className = "blog-feature-nav blog-feature-nav--prev";
    prev.setAttribute("aria-label", "Slide sebelumnya");
    prev.innerHTML =
      '<span class="blog-feature-nav-icon" aria-hidden="true">‹</span>';

    var next = document.createElement("button");
    next.type = "button";
    next.className = "blog-feature-nav blog-feature-nav--next";
    next.setAttribute("aria-label", "Slide berikutnya");
    next.innerHTML =
      '<span class="blog-feature-nav-icon" aria-hidden="true">›</span>';

    el.appendChild(prev);
    el.appendChild(next);

    return el;
  }

  /**
   * Slide dengan aria-hidden masih berisi <a> yang bisa di-tab — Lighthouse memperingatkan.
   * Nonaktifkan tab order pada konten di dalam slide yang disembunyikan.
   */
  function setCarouselSlideFocusable(slideEl, hidden) {
    if (!slideEl) return;
    var focusables = slideEl.querySelectorAll(
      "a[href], button:not([disabled]), [tabindex]"
    );
    focusables.forEach(function (el) {
      if (hidden) {
        el.setAttribute("tabindex", "-1");
      } else {
        el.removeAttribute("tabindex");
      }
    });
  }

  function bindFeaturedCarousel(root) {
    if (!root) return;
    var track = root.querySelector(".blog-feature-track");
    var slideEls = root.querySelectorAll(".blog-feature-slide");
    var n = slideEls.length;
    if (!track || n === 0) return;

    var prev = root.querySelector(".blog-feature-nav--prev");
    var next = root.querySelector(".blog-feature-nav--next");
    var dotEls = root.parentElement
      ? root.parentElement.querySelectorAll("[data-feature-dot-index]")
      : [];
    if (n <= 1) {
      if (prev) prev.hidden = true;
      if (next) next.hidden = true;
      if (dotEls && dotEls.length) {
        dotEls.forEach(function (dot, idx) {
          dot.classList.toggle("is-active", idx === 0);
          dot.setAttribute("aria-current", idx === 0 ? "true" : "false");
        });
      }
      return;
    }

    var i = 0;
    function apply() {
      track.style.transform = "translateX(-" + i * 100 + "%)";
      slideEls.forEach(function (s, j) {
        var hidden = j !== i;
        s.setAttribute("aria-hidden", hidden ? "true" : "false");
        setCarouselSlideFocusable(s, hidden);
      });
      if (dotEls && dotEls.length) {
        dotEls.forEach(function (dot, idx) {
          var on = idx === i;
          dot.classList.toggle("is-active", on);
          dot.setAttribute("aria-current", on ? "true" : "false");
        });
      }
    }

    function go(delta) {
      i = (i + delta + n) % n;
      apply();
    }

    if (prev)
      prev.addEventListener("click", function (e) {
        e.preventDefault();
        e.stopPropagation();
        go(-1);
        restartAuto();
      });
    if (next)
      next.addEventListener("click", function (e) {
        e.preventDefault();
        e.stopPropagation();
        go(1);
        restartAuto();
      });
    if (dotEls && dotEls.length) {
      dotEls.forEach(function (dot) {
        dot.addEventListener("click", function (e) {
          e.preventDefault();
          e.stopPropagation();
          var idx = parseInt(
            dot.getAttribute("data-feature-dot-index") || "",
            10
          );
          if (!isFinite(idx) || idx < 0 || idx >= n) return;
          i = idx;
          apply();
          restartAuto();
        });
      });
    }

    var sx = 0;
    var tracking = false;
    root.addEventListener(
      "touchstart",
      function (e) {
        if (!e.touches || !e.touches[0]) return;
        tracking = true;
        sx = e.touches[0].clientX;
      },
      { passive: true }
    );
    root.addEventListener(
      "touchend",
      function (e) {
        if (!tracking || !e.changedTouches || !e.changedTouches[0]) return;
        tracking = false;
        var ex = e.changedTouches[0].clientX;
        var d = ex - sx;
        if (Math.abs(d) > 48) {
          go(d < 0 ? 1 : -1);
          restartAuto();
        }
      },
      { passive: true }
    );

    var autoTimer = 0;
    var autoDelayMs = 5000;
    function stopAuto() {
      if (!autoTimer) return;
      clearInterval(autoTimer);
      autoTimer = 0;
    }
    function startAuto() {
      if (autoTimer || n <= 1) return;
      autoTimer = setInterval(function () {
        if (document.hidden) return;
        go(1);
      }, autoDelayMs);
    }
    function restartAuto() {
      stopAuto();
      startAuto();
    }
    root.addEventListener("mouseenter", stopAuto);
    root.addEventListener("mouseleave", startAuto);
    root.addEventListener(
      "focusin",
      function () {
        stopAuto();
      },
      true
    );
    root.addEventListener(
      "focusout",
      function () {
        startAuto();
      },
      true
    );
    document.addEventListener("visibilitychange", function () {
      if (document.hidden) {
        stopAuto();
      } else {
        startAuto();
      }
    });
    apply();
    startAuto();
  }

  function buildLatestPostSection(latest) {
    if (!latest) return null;
    var sec = document.createElement("section");
    sec.className = "blog-latest-section";
    var href = postPublicPath(latest);
    var thumb = postThumbnailSrc(latest);
    var catName =
      latest.category && latest.category.name ? latest.category.name : "";
    var catSlug =
      latest.category && latest.category.slug ? latest.category.slug : "";
    var rel = relativeTimeId(postTimeIso(latest));
    var catHtml = "";
    if (catName) {
      catHtml = catSlug
        ? '<a class="blog-latest-cat" href="' +
          escapeHtml(categoryPublicPath(catSlug)) +
          '">' +
          escapeHtml(catName) +
          "</a>"
        : '<span class="blog-latest-cat">' + escapeHtml(catName) + "</span>";
    }
    sec.innerHTML =
      '<div class="blog-section-head">' +
      '<h2 class="blog-section-title">' +
      '<span class="blog-section-title-bar" aria-hidden="true"></span>' +
      "Postingan terbaru" +
      "</h2>" +
      '<a class="blog-section-more" href="/posts">Lihat semua <span aria-hidden="true">→</span></a>' +
      "</div>" +
      '<article class="blog-latest-card">' +
      '<div class="blog-latest-thumb">' +
      '<img src="' +
      escapeHtml(thumb) +
      '" alt="" loading="eager" decoding="async" width="280" height="180" />' +
      "</div>" +
      '<div class="blog-latest-body">' +
      catHtml +
      '<h3 class="blog-latest-title">' +
      escapeHtml(latest.title || "") +
      "</h3>" +
      (rel ? '<p class="blog-latest-time">' + escapeHtml(rel) + "</p>" : "") +
      "</div>" +
      '<a class="blog-latest-overlay" href="' +
      escapeHtml(href) +
      '" aria-label="' +
      escapeHtml(latest.title || "Buka artikel") +
      '"></a>' +
      "</article>";
    var thumbWrap = sec.querySelector(".blog-latest-thumb");
    var img = thumbWrap && thumbWrap.querySelector("img");
    if (img) {
      bindResponsiveThumbCDN(img, thumbWrap, "", 1, "");
      img.addEventListener(
        "error",
        function onE() {
          img.removeEventListener("error", onE);
          img.src = BLANK_IMG;
        },
        false
      );
      bindLazyImageReveal(thumbWrap, img);
    }
    return sec;
  }

  function buildPicksGridCard(p) {
    var href = postPublicPath(p);
    var thumb = postThumbnailSrc(p);
    var catName = p.category && p.category.name ? p.category.name : "";
    var rel = relativeTimeId(postTimeIso(p));
    var a = document.createElement("a");
    a.className = "blog-picks-grid-card";
    a.href = href;
    a.innerHTML =
      '<div class="blog-picks-grid-media">' +
      '<img src="' +
      escapeHtml(thumb) +
      '" alt="" loading="lazy" decoding="async" />' +
      "</div>" +
      '<div class="blog-picks-grid-body">' +
      (catName
        ? '<span class="blog-picks-cat">' + escapeHtml(catName) + "</span>"
        : "") +
      '<h3 class="blog-picks-grid-title">' +
      escapeHtml(p.title || "") +
      "</h3>" +
      (rel ? '<p class="blog-picks-time">' + escapeHtml(rel) + "</p>" : "") +
      "</div>";
    var im = a.querySelector("img");
    if (im) {
      im.addEventListener(
        "error",
        function onE() {
          im.removeEventListener("error", onE);
          im.src = BLANK_IMG;
        },
        false
      );
    }
    return a;
  }

  /**
   * Observe an element, call onHit when it enters viewport.
   * If onHit returns false, keep observing (not "done").
   */
  function observeOnce(el, onHit) {
    if (!el || !onHit) return function () {};
    var done = false;
    if ("IntersectionObserver" in window) {
      var io = new IntersectionObserver(
        function (entries) {
          if (done) return;
          for (var i = 0; i < entries.length; i++) {
            if (entries[i] && entries[i].isIntersecting) {
              var handled = onHit();
              if (handled === false) return;
              done = true;
              try {
                io.disconnect();
              } catch (e) {}
              return;
            }
          }
        },
        { rootMargin: "200px 0px" }
      );
      io.observe(el);
      return function () {
        try {
          io.disconnect();
        } catch (e) {}
      };
    }
    // Fallback: eager
    onHit();
    return function () {};
  }

  function bindRecCarousel(root) {
    if (!root) return;
    var track = root.querySelector(".blog-rec-track");
    var slides = root.querySelectorAll(".blog-rec-slide");
    var n = slides.length;
    if (!track || n === 0) return;

    var prev = root.querySelector(".blog-rec-nav--prev");
    var next = root.querySelector(".blog-rec-nav--next");
    if (n <= 1) return;

    var i = 0;
    function apply() {
      track.style.transform = "translateX(-" + i * 100 + "%)";
      slides.forEach(function (s, j) {
        var hidden = j !== i;
        s.setAttribute("aria-hidden", hidden ? "true" : "false");
        setCarouselSlideFocusable(s, hidden);
      });
    }
    function go(delta) {
      i = (i + delta + n) % n;
      apply();
    }
    if (prev)
      prev.addEventListener("click", function (e) {
        e.preventDefault();
        e.stopPropagation();
        go(-1);
      });
    if (next)
      next.addEventListener("click", function (e) {
        e.preventDefault();
        e.stopPropagation();
        go(1);
      });
    var sx = 0;
    var tracking = false;
    root.addEventListener(
      "touchstart",
      function (e) {
        if (!e.touches || !e.touches[0]) return;
        tracking = true;
        sx = e.touches[0].clientX;
      },
      { passive: true }
    );
    root.addEventListener(
      "touchend",
      function (e) {
        if (!tracking || !e.changedTouches || !e.changedTouches[0]) return;
        tracking = false;
        var d = e.changedTouches[0].clientX - sx;
        if (Math.abs(d) > 48) go(d < 0 ? 1 : -1);
      },
      { passive: true }
    );
    apply();
  }

  function relativeTimeId(iso) {
    if (!iso) return "";
    var t = new Date(iso).getTime();
    if (isNaN(t)) return "";
    var sec = Math.floor((Date.now() - t) / 1000);
    if (sec < 0) sec = 0;
    if (sec < 3600) return Math.max(1, Math.floor(sec / 60)) + " menit lalu";
    if (sec < 86400) return Math.floor(sec / 3600) + " jam lalu";
    var days = Math.floor(sec / 86400);
    if (days < 30) return Math.max(1, days) + " hari lalu";
    var months = Math.floor(days / 30);
    if (months < 12) return Math.max(1, months) + " bulan lalu";
    var years = Math.floor(days / 365);
    return Math.max(1, years) + " tahun lalu";
  }

  function setPopularVisible(show) {
    var wrap = document.getElementById("blog-popular-wrap");
    if (!wrap) return;
    wrap.hidden = !show;
    if (!show) wrap.style.minHeight = "";
  }

  function canonicalOrigin() {
    // Prefer server-provided canonical URL from og:url when present.
    try {
      var og = document.querySelector('meta[property="og:url"]');
      var c =
        og && og.getAttribute ? (og.getAttribute("content") || "").trim() : "";
      if (c) {
        var u = new URL(c);
        if (u && u.origin) return u.origin;
      }
    } catch (e) {}
    try {
      return location.origin;
    } catch (e2) {}
    return "";
  }

  function openSharePopup(url, w, h) {
    try {
      var sw = w || 640;
      var sh = h || 560;
      var left = Math.max(0, Math.floor((window.screen.width - sw) / 2));
      var top = Math.max(0, Math.floor((window.screen.height - sh) / 3));
      window.open(
        url,
        "share",
        "noopener,noreferrer,width=" +
          sw +
          ",height=" +
          sh +
          ",left=" +
          left +
          ",top=" +
          top
      );
    } catch (e) {
      location.href = url;
    }
  }

  async function copyToClipboard(text) {
    var t = String(text || "");
    if (!t) return false;
    try {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(t);
        return true;
      }
    } catch (e) {}
    try {
      var ta = document.createElement("textarea");
      ta.value = t;
      ta.setAttribute("readonly", "readonly");
      ta.style.position = "fixed";
      ta.style.left = "-9999px";
      document.body.appendChild(ta);
      ta.select();
      var ok = document.execCommand("copy");
      ta.remove();
      return !!ok;
    } catch (e2) {}
    return false;
  }

  function buildShareURL(kind, pageURL, title) {
    var u = encodeURIComponent(String(pageURL || ""));
    var t = encodeURIComponent(String(title || ""));
    switch (kind) {
      case "wa":
        return "https://wa.me/?text=" + t + "%20" + u;
      case "fb":
        // Facebook primarily uses OG tags; prefilling user text is often ignored.
        return "https://www.facebook.com/sharer/sharer.php?u=" + u;
      case "x":
        return "https://twitter.com/intent/tweet?text=" + t + "&url=" + u;
      case "tg":
        return "https://t.me/share/url?url=" + u + "&text=" + t;
      case "li":
        return "https://www.linkedin.com/sharing/share-offsite/?url=" + u;
      default:
        return "";
    }
  }

  function ensureShareMenu(rootEl, opts) {
    if (!rootEl || !opts) return function () {};
    var open = false;

    var overlay = document.createElement("div");
    overlay.className = "blog-share-overlay";
    overlay.hidden = true;

    var modal = document.createElement("div");
    modal.className = "blog-share-modal";
    modal.hidden = true;
    modal.setAttribute("role", "dialog");
    modal.setAttribute("aria-modal", "true");
    modal.setAttribute("aria-label", "Share");

    modal.innerHTML =
      '<div class="blog-share-modal-head">' +
      '<div class="blog-share-modal-title">Share</div>' +
      '<button type="button" class="blog-share-close" aria-label="Tutup">×</button>' +
      "</div>" +
      '<div class="blog-share-modal-body">' +
      '<div class="blog-share-grid">' +
      '<button type="button" class="blog-share-chip" data-share="fb" aria-label="Share Facebook">' +
      '<span class="blog-share-ico" aria-hidden="true">' +
      '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M22 12a10 10 0 1 0-11.56 9.87v-6.99H7.9V12h2.54V9.8c0-2.5 1.49-3.88 3.77-3.88 1.09 0 2.23.2 2.23.2v2.46h-1.25c-1.23 0-1.61.76-1.61 1.54V12h2.74l-.44 2.88h-2.3v6.99A10 10 0 0 0 22 12z"/></svg>' +
      "</span><span>Facebook</span></button>" +
      '<button type="button" class="blog-share-chip" data-share="x" aria-label="Share X">' +
      '<span class="blog-share-ico" aria-hidden="true">' +
      '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M18.9 2H22l-6.76 7.73L23 22h-6.1l-4.78-7.05L6.04 22H2.9l7.23-8.26L1 2h6.26l4.32 6.35L18.9 2zm-1.07 18h1.7L7.18 3.93H5.37L17.83 20z"/></svg>' +
      "</span><span>X</span></button>" +
      '<button type="button" class="blog-share-chip" data-share="tg" aria-label="Share Telegram">' +
      '<span class="blog-share-ico" aria-hidden="true">' +
      '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M9.7 14.7 9.4 19c.4 0 .6-.2.9-.4l2.1-2 4.3 3.1c.8.4 1.3.2 1.5-.7l2.7-12.7c.3-1.1-.4-1.5-1.1-1.3L2.7 9.7c-1 .4-1 1 .1 1.3l4.4 1.4 10.2-6.4c.5-.3.9-.1.6.2L9.7 14.7z"/></svg>' +
      "</span><span>Telegram</span></button>" +
      '<button type="button" class="blog-share-chip" data-share="li" aria-label="Share LinkedIn">' +
      '<span class="blog-share-ico" aria-hidden="true">' +
      '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M4.98 3.5C4.98 4.88 3.87 6 2.5 6S0 4.88 0 3.5 1.12 1 2.5 1s2.48 1.12 2.48 2.5zM.5 23.5h4V7.98h-4V23.5zM8 7.98h3.8v2.12h.05c.53-1 1.83-2.12 3.77-2.12 4.03 0 4.78 2.65 4.78 6.1v9.42h-4v-8.35c0-2-.04-4.57-2.79-4.57-2.79 0-3.22 2.18-3.22 4.43v8.49H8V7.98z"/></svg>' +
      "</span><span>LinkedIn</span></button>" +
      '<button type="button" class="blog-share-chip" data-share="copy" aria-label="Copy link">' +
      '<span class="blog-share-ico" aria-hidden="true">' +
      '<svg viewBox="0 0 24 24" fill="currentColor"><path d="M16 1H6c-1.1 0-2 .9-2 2v12h2V3h10V1zm3 4H10c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h9c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2zm0 16H10V7h9v14z"/></svg>' +
      "</span><span>Copy link</span></button>" +
      "</div>" +
      "</div>";

    document.body.appendChild(overlay);
    document.body.appendChild(modal);

    function setOpen(v) {
      open = !!v;
      overlay.hidden = !open;
      modal.hidden = !open;
      document.body.classList.toggle("blog-modal-open", open);
    }

    function close() {
      setOpen(false);
    }

    overlay.addEventListener("click", function () {
      close();
    });
    var closeBtn = modal.querySelector(".blog-share-close");
    if (closeBtn) closeBtn.addEventListener("click", close);

    modal.addEventListener("click", function (e) {
      var btn =
        e.target && e.target.closest ? e.target.closest("[data-share]") : null;
      if (!btn) return;
      e.preventDefault();
      var kind = btn.getAttribute("data-share");
      if (kind === "copy") {
        copyToClipboard(opts.url).then(function (ok) {
          btn.textContent = ok ? "Copied" : "Copy failed";
          setTimeout(function () {
            btn.textContent = "Copy link";
          }, 900);
        });
        return;
      }
      if (kind === "fb") {
        // FB doesn't reliably support prefilling text; copy first so user can paste.
        copyToClipboard((opts.title ? opts.title + " " : "") + opts.url).then(
          function () {}
        );
      }
      var shareURL = buildShareURL(kind, opts.url, opts.title);
      if (shareURL) openSharePopup(shareURL);
    });

    // Close on ESC
    window.addEventListener("keydown", function onKey(e) {
      if (!open) return;
      if (e.key === "Escape") close();
    });

    return function toggle() {
      setOpen(!open);
    };
  }

  function insertAfter(parent, node, afterNode) {
    if (!parent || !node) return;
    if (!afterNode || afterNode.parentNode !== parent) {
      parent.appendChild(node);
      return;
    }
    if (afterNode.nextSibling) parent.insertBefore(node, afterNode.nextSibling);
    else parent.appendChild(node);
  }

  function placePopularForViewport(routeName) {
    var wrap = document.getElementById("blog-popular-wrap");
    if (!wrap) return;
    var root = document.getElementById("public-root");
    if (!root) return;

    if (
      wrap.getAttribute("data-neo-feattab-rail") === "1" &&
      root.contains(wrap)
    ) {
      return;
    }

    var isMobile = false;
    try {
      isMobile = !!(
        window.matchMedia && window.matchMedia("(max-width: 899px)").matches
      );
    } catch (e) {}

    /*
     * Desktop: jangan sisipkan node komentar anchor — itu menambah anak ketiga di .blog-home-layout
     * (grid 2 kolom), sehingga kolom 2 terisi komentar kosong dan aside Populer terdorong baris
     * lain → CLS sangat besar. Cukup pastikan aside tepat setelah #public-root (2 sel grid).
     */
    if (!isMobile) {
      var layout = root.parentNode;
      if (!layout) return;
      if (wrap.parentNode === layout && root.nextSibling === wrap) return;
      insertAfter(layout, wrap, root);
      return;
    }

    // Mobile beranda + /posts: biarkan Populer sebagai sibling setelah #public-root.
    // Memindahkan ke dalam #public-root memicu CLS besar (footer terdorong).
    if (routeName === "home" || routeName === "posts") {
      var layoutHome = root.parentNode;
      if (layoutHome) insertAfter(layoutHome, wrap, root);
      return;
    }

    // Mobile: move into main flow at a sensible place per route.
    if (routeName === "post") {
      var mv = root.querySelector(".blog-mostviewed");
      if (mv) {
        insertAfter(root, wrap, mv);
        return;
      }
      var article = root.querySelector(".blog-article-single");
      if (article) {
        insertAfter(root, wrap, article);
        return;
      }
      root.appendChild(wrap);
      return;
    }
  }

  function ensurePopularDetachedFromRoot(keepHomeSSR) {
    var wrap = document.getElementById("blog-popular-wrap");
    if (!wrap) return;
    var root = document.getElementById("public-root");
    if (!root || !root.contains(wrap)) return;

    /*
     * SSR home neo Mag: rail #blog-popular-wrap di dalam #public-root — biarkan di tempatnya
     * supaya layout feattab+tautan tidak meloncat. Keluar beranda: angkat ke kontainer layout
     * sebelum root.innerHTML dibersihkan.
     */
    if (keepHomeSSR && wrap.getAttribute("data-neo-feattab-rail") === "1") {
      return;
    }

    var layout = root.parentNode;
    if (layout) layout.appendChild(wrap);
  }

  var popularPlaceT = 0;
  function requestPopularPlacement(routeName) {
    try {
      if (popularPlaceT) clearTimeout(popularPlaceT);
    } catch (e) {}
    popularPlaceT = setTimeout(function () {
      placePopularForViewport(routeName);
    }, 0);
  }

  function renderPopularSidebar(payload) {
    var wrap = document.getElementById("blog-popular-wrap");
    var ol = document.getElementById("blog-popular-list");
    var olLatest = document.getElementById("blog-latest-list");
    if (!wrap || !ol || !olLatest) return;

    /*
     * Beranda neo (feattab): rail Terkini di-server; jangan timpa/sembunyikan rail dari API.
     */
    if (
      wrap.getAttribute("data-neo-feattab-rail") === "1" &&
      wrap.getAttribute("data-sidebar-ssr") === "1"
    ) {
      return;
    }

    if (
      wrap.classList.contains("blog-popular-wrap--neo-mag-home-only-latest") &&
      wrap.getAttribute("data-sidebar-ssr") === "1" &&
      wrap.querySelector(".neo-mag-rail-section--latest .neo-mag-mp-card")
    ) {
      return;
    }

    var popularPosts = payload && payload.popular ? payload.popular : [];
    var latestPosts = payload && payload.latest ? payload.latest : [];
    var neoMag = wrap.classList.contains("blog-popular-wrap--neo-mag");
    var neoLatestSection = wrap.querySelector(".neo-mag-rail-section--latest");

    function renderNeoTerkiniRailCards(posts) {
      return (posts || [])
        .slice(0, 5)
        .map(function (p) {
          var href = postPublicPath(p);
          var thumb = escapeHtml(postThumbnailSrc(p));
          var rel = relativeTimeId(postTimeIso(p));
          var meta = rel
            ? '<p class="neo-mag-mp-meta-line"><span class="neo-mag-meta-time">' +
              escapeHtml(rel) +
              "</span></p>"
            : "";
          return (
            '<article class="neo-mag-mp-card">' +
            '<a class="neo-mag-mp-thumb-ring" href="' +
            escapeHtml(href) +
            '" aria-hidden="true" tabindex="-1">' +
            '<img src="' +
            thumb +
            '" alt="" width="72" height="72" loading="lazy" decoding="async" />' +
            "</a>" +
            '<div class="neo-mag-mp-text">' +
            '<h4 class="neo-mag-mp-title"><a href="' +
            escapeHtml(href) +
            '">' +
            escapeHtml(p.title || "") +
            "</a></h4>" +
            meta +
            "</div></article>"
          );
        })
        .join("");
    }

    function renderList(targetOl, posts, listKind) {
      if (!targetOl) return;
      targetOl.classList.remove("blog-popular-list--skeleton");
      var neoMag = wrap.classList.contains("blog-popular-wrap--neo-mag");
      targetOl.innerHTML = (posts || [])
        .slice(0, 5)
        .map(function (p) {
          var cat = p.category && p.category.name ? p.category.name : "";
          var rel = relativeTimeId(postTimeIso(p));
          var href = postPublicPath(p);
          if (neoMag) {
            var thumb = escapeHtml(postThumbnailSrc(p));
            var meta = "";
            var cal = postCalendarLabelPublic(p);
            var views =
              listKind === "popular" ? formatViewLabel(p.view) : "";
            if (listKind === "popular") {
              if (cal) {
                meta +=
                  '<span class="blog-popular-date">' +
                  escapeHtml(cal) +
                  "</span>";
              }
              if (views) {
                meta +=
                  '<span class="blog-popular-views"><span class="neo-mag-ico neo-mag-ico--eye" aria-hidden="true"></span>' +
                  escapeHtml(views) +
                  "</span>";
              } else if (rel) {
                meta +=
                  '<span class="blog-popular-time">' +
                  escapeHtml(rel) +
                  "</span>";
              }
            } else {
              if (cal) {
                meta +=
                  '<span class="blog-popular-date"><span class="neo-mag-ico neo-mag-ico--cal" aria-hidden="true"></span>' +
                  escapeHtml(cal) +
                  "</span>";
              } else if (rel) {
                meta +=
                  '<span class="blog-popular-time">' +
                  escapeHtml(rel) +
                  "</span>";
              }
            }
            return (
              '<li class="blog-popular-item blog-popular-item--neo-mag">' +
              '<a class="blog-popular-link blog-popular-link--neo-mag" href="' +
              escapeHtml(href) +
              '">' +
              '<span class="blog-popular-thumb-ring" aria-hidden="true">' +
              '<img src="' +
              thumb +
              '" alt="" width="72" height="72" loading="lazy" decoding="async" />' +
              "</span>" +
              '<span class="blog-popular-body-neo">' +
              '<span class="blog-popular-title">' +
              escapeHtml(p.title || "") +
              "</span>" +
              '<span class="blog-popular-meta-row">' +
              meta +
              "</span>" +
              "</span>" +
              "</a>" +
              "</li>"
            );
          }
          return (
            '<li class="blog-popular-item">' +
            '<a class="blog-popular-link" href="' +
            escapeHtml(href) +
            '">' +
            '<span class="blog-popular-title">' +
            escapeHtml(p.title || "") +
            "</span>" +
            '<span class="blog-popular-meta">' +
            escapeHtml(cat) +
            (cat && rel ? " · " : "") +
            escapeHtml(rel) +
            "</span>" +
            "</a>" +
            "</li>"
          );
        })
        .join("");
    }

    if (
      (!popularPosts || !popularPosts.length) &&
      (!latestPosts || !latestPosts.length)
    ) {
      wrap.hidden = true;
      ol.innerHTML = "";
      olLatest.innerHTML = "";
      ol.classList.remove("blog-popular-list--skeleton");
      olLatest.classList.remove("blog-popular-list--skeleton");
      wrap.setAttribute("aria-busy", "false");
      wrap.style.minHeight = "";
      return;
    }
    wrap.hidden = false;
    wrap.setAttribute("aria-busy", "false");
    renderList(ol, popularPosts, "popular");
    if (neoMag && neoLatestSection) {
      neoLatestSection.innerHTML =
        '<h3 class="neo-mag-rail-heading">Terkini</h3>' +
        renderNeoTerkiniRailCards(latestPosts);
      olLatest.innerHTML = "";
      olLatest.classList.remove("blog-popular-list--skeleton");
    } else {
      renderList(olLatest, latestPosts, "latest");
    }
    bindPageResponsiveImages(wrap);
  }

  var popularCache = { at: 0, posts: null };
  var latestCache = { at: 0, posts: null };

  /**
   * Tunda isi Populer sampai setelah browser sempat melukis skeleton (dua frame).
   * Tanpa ini, respons API dari cache bisa mengganti DOM sebelum paint pertama → CLS tidak stabil antar run.
   */
  function queuePopularSidebarRender(payload) {
    requestAnimationFrame(function () {
      requestAnimationFrame(function () {
        renderPopularSidebar(payload);
      });
    });
  }

  /**
   * Sidebar dari template (data-sidebar-ssr): jangan fetch/ganti DOM di beranda.
   * Populer 24 jam bisa kosong sementara Terkini isi — guard lama (dua daftar >0) memicu
   * refresh blok bawah (sidebar di bawah main di mobile).
   */
  function sidebarListLimitFromDataAttr(dataAttr, metaName) {
    var wrap = document.getElementById("blog-popular-wrap");
    if (wrap) {
      var fromWrap = parseInt(wrap.getAttribute(dataAttr), 10);
      if (!isNaN(fromWrap) && fromWrap > 0) {
        return Math.min(20, fromWrap);
      }
      var legacyWrap = parseInt(
        wrap.getAttribute("data-sidebar-list-limit"),
        10
      );
      if (!isNaN(legacyWrap) && legacyWrap > 0) {
        return Math.min(20, legacyWrap);
      }
    }
    var meta = document.querySelector('meta[name="' + metaName + '"]');
    if (meta) {
      var fromMeta = parseInt(meta.getAttribute("content"), 10);
      if (!isNaN(fromMeta) && fromMeta > 0) {
        return Math.min(20, fromMeta);
      }
    }
    var legacyMeta = document.querySelector(
      'meta[name="tirtayasa-sidebar-list-limit"]'
    );
    if (legacyMeta) {
      var fromLegacy = parseInt(legacyMeta.getAttribute("content"), 10);
      if (!isNaN(fromLegacy) && fromLegacy > 0) {
        return Math.min(20, fromLegacy);
      }
    }
    return 5;
  }

  function sidebarListLimitPopular() {
    return sidebarListLimitFromDataAttr(
      "data-sidebar-list-limit-popular",
      "tirtayasa-sidebar-list-limit-popular"
    );
  }

  function sidebarListLimitLatest() {
    return sidebarListLimitFromDataAttr(
      "data-sidebar-list-limit-latest",
      "tirtayasa-sidebar-list-limit-latest"
    );
  }

  function shouldSkipPopularSidebarFetchOnHome() {
    try {
      if (normalizePathname(location.pathname || "") !== "/") return false;
    } catch (e) {
      return false;
    }
    var w = document.getElementById("blog-popular-wrap");
    return !!(w && w.getAttribute("data-sidebar-ssr") === "1");
  }

  async function ensurePopularSidebarLoaded() {
    var wrap = document.getElementById("blog-popular-wrap");
    var ol = document.getElementById("blog-popular-list");
    var olLatest = document.getElementById("blog-latest-list");
    if (!wrap || !ol || !olLatest) return;

    if (
      wrap.classList.contains("blog-popular-wrap--neo-mag-home-only-latest") &&
      wrap.getAttribute("data-sidebar-ssr") === "1"
    ) {
      return;
    }

    if (shouldSkipPopularSidebarFetchOnHome()) {
      return;
    }

    var now = Date.now();
    try {
      var useCachePopular =
        popularCache.posts && now - popularCache.at < 5 * 60 * 1000;
      var useCacheLatest =
        latestCache.posts && now - latestCache.at < 5 * 60 * 1000;
      if (useCachePopular && useCacheLatest) {
        queuePopularSidebarRender({
          popular: popularCache.posts,
          latest: latestCache.posts,
        });
        return;
      }

      var reqs = [];
      reqs.push(
        useCachePopular
          ? Promise.resolve(popularCache.posts)
          : api
              .getPublic(
                "/posts/popular?hours=24&limit=" + sidebarListLimitPopular()
              )
              .then(function (raw) {
                var items =
                  raw && Array.isArray(raw.items)
                    ? raw.items
                    : Array.isArray(raw)
                    ? raw
                    : [];
                items = dedupePostsById(items).filter(isPublished);
                popularCache = { at: Date.now(), posts: items };
                return items;
              })
      );
      reqs.push(
        useCacheLatest
          ? Promise.resolve(latestCache.posts)
          : api
              .getPublic("/posts?page=1&size=" + sidebarListLimitLatest())
              .then(function (raw2) {
                var items2 = Array.isArray(raw2)
                  ? raw2
                  : raw2 && Array.isArray(raw2.items)
                  ? raw2.items
                  : [];
                items2 = dedupePostsById(items2).filter(isPublished);
                items2 = sortByPublishedDesc(items2);
                latestCache = { at: Date.now(), posts: items2 };
                return items2;
              })
      );

      var both = await Promise.all(reqs);
      queuePopularSidebarRender({
        popular: both[0] || [],
        latest: both[1] || [],
      });
    } catch (e) {
      // keep sidebar hidden on failure
      queuePopularSidebarRender({ popular: [], latest: [] });
    }
  }

  /** Ukuran halaman feed beranda (API + "Muat lagi"); selaras dengan server pagination. */
  var FEED_PAGE_SIZE = 15;
  /** Posting untuk carousel, posting terbaru, rekomendasi, tag, populer — tidak memuat ratusan artikel. */
  var HOME_BOOTSTRAP_SIZE = 36;
  function publicAlsoReadMaxInserts() {
    var n = parseInt((window.TirtayasaPublicConfig || {}).alsoReadMaxInserts, 10);
    if (isNaN(n) || n < 1) return 6;
    if (n > 20) return 20;
    return n;
  }

  function publicAlsoReadRelatedFetch() {
    var cap = publicAlsoReadMaxInserts() * 6;
    return cap < 36 ? 36 : cap > 100 ? 100 : cap;
  }

  var relatedByCategoryCache = Object.create(null);
  async function getRelatedPostsByCategory(catSlug, excludeID, fetchLimit) {
    var slug = (catSlug || "").trim();
    if (!slug) return [];
    var cap =
      fetchLimit != null && fetchLimit > 0
        ? fetchLimit
        : publicAlsoReadRelatedFetch();
    if (cap > 100) cap = 100;
    var cacheKey = slug + "|bcat";
    var cached = relatedByCategoryCache[cacheKey];
    var now = Date.now();
    if (cached && cached.posts && now - cached.at < 5 * 60 * 1000) {
      return cached.posts
        .filter(function (p) {
          return !excludeID || p.id !== excludeID;
        })
        .slice(0, cap);
    }
    try {
      var raw = await api.getPublic(
        "/posts?page=1&size=" +
          cap +
          "&category_slug=" +
          encodeURIComponent(slug)
      );
      var items = Array.isArray(raw)
        ? raw
        : raw && Array.isArray(raw.items)
        ? raw.items
        : [];
      items = dedupePostsById(items).filter(isPublished);
      items = sortPopularByViews(items);
      relatedByCategoryCache[cacheKey] = { at: now, posts: items };
      return items
        .filter(function (p) {
          return !excludeID || p.id !== excludeID;
        })
        .slice(0, cap);
    } catch (e) {
      return [];
    }
  }

  function buildAlsoReadBlock(p) {
    if (!p) return null;
    var href = postPublicPath(p);
    var el = document.createElement("aside");
    el.className = "blog-also-read";
    el.setAttribute("aria-label", "Baca juga");
    el.innerHTML =
      '<div class="blog-also-read-inner">' +
      '<div class="blog-also-read-label">Baca juga</div>' +
      '<a class="blog-also-read-link" href="' +
      escapeHtml(href) +
      '">' +
      '<span class="blog-also-read-title">' +
      escapeHtml(p.title || "") +
      "</span>" +
      "</a>" +
      "</div>";
    return el;
  }

  function elementStartsWithTableOrList(el) {
    if (!el || el.nodeType !== Node.ELEMENT_NODE) return false;
    var child = el.firstChild;
    while (child) {
      if (child.nodeType === Node.TEXT_NODE) {
        if (!(child.textContent || "").trim()) {
          child = child.nextSibling;
          continue;
        }
        return false;
      }
      if (child.nodeType === Node.COMMENT_NODE) {
        child = child.nextSibling;
        continue;
      }
      if (child.nodeType !== Node.ELEMENT_NODE) {
        child = child.nextSibling;
        continue;
      }
      var tag = (child.nodeName || "").toLowerCase();
      if (tag === "table" || tag === "ol" || tag === "ul") return true;
      if (tag === "div" || tag === "figure") {
        if (elementStartsWithTableOrList(child)) return true;
      }
      return false;
    }
    return false;
  }

  function nextStructuredBlockSiblingEl(cur) {
    if (!cur) return null;
    var n = cur.nextSibling;
    while (n) {
      if (n.nodeType === Node.TEXT_NODE) {
        if (!(n.textContent || "").trim()) {
          n = n.nextSibling;
          continue;
        }
        return null;
      }
      if (n.nodeType === Node.COMMENT_NODE) {
        n = n.nextSibling;
        continue;
      }
      if (n.nodeType !== Node.ELEMENT_NODE) {
        n = n.nextSibling;
        continue;
      }
      var tag = (n.nodeName || "").toLowerCase();
      if (tag === "table" || tag === "ol" || tag === "ul") return n;
      if (tag === "div" || tag === "figure") {
        if (elementStartsWithTableOrList(n)) return n;
      }
      return null;
    }
    return null;
  }

  function relocateMisplacedAlsoReadBlocks(root) {
    if (!root) return;
    var list = root.querySelectorAll("aside.blog-also-read");
    for (var i = 0; i < list.length; i++) {
      var as = list[i];
      var block = nextStructuredBlockSiblingEl(as);
      if (!block || !as.parentNode || !block.parentNode) continue;
      if (as.parentNode !== block.parentNode) continue;
      block.parentNode.insertBefore(as, block.nextSibling);
    }
  }

  function publicAlsoReadEnabled() {
    return (window.TirtayasaPublicConfig || {}).alsoReadEnabled !== false;
  }

  function injectAlsoReadEvery3Paragraphs(bodyEl, posts) {
    if (!publicAlsoReadEnabled()) return;
    if (!bodyEl || !posts || !posts.length) return;
    var paras = Array.prototype.slice.call(bodyEl.querySelectorAll("p"));
    if (!paras.length) return;

    var insertIdx = 0;
    var inserted = 0;
    var maxInsert = publicAlsoReadMaxInserts();
    var seen = Object.create(null);

    var count = 0;
    for (var i = 0; i < paras.length; i++) {
      var pEl = paras[i];
      if (!pEl) continue;
      var txt = (pEl.textContent || "").trim();
      if (!txt) continue;
      count++;
      if (count % 3 !== 0) continue;
      if (inserted >= maxInsert) break;

      while (
        insertIdx < posts.length &&
        posts[insertIdx] &&
        seen[posts[insertIdx].id]
      ) {
        insertIdx++;
      }
      if (insertIdx >= posts.length) break;

      var post = posts[insertIdx++];
      if (!post) continue;
      if (post.id != null) seen[post.id] = true;

      var block = buildAlsoReadBlock(post);
      if (!block) continue;

      var anchor = pEl;
      var afterBlock = nextStructuredBlockSiblingEl(pEl);
      if (afterBlock && (pEl.nodeName || "").toLowerCase() === "p") {
        anchor = afterBlock;
      }

      // Insert after anchor (paragraph or following table/list wrapper).
      if (anchor.parentNode) {
        anchor.insertAdjacentElement("afterend", block);
      } else {
        bodyEl.appendChild(block);
      }
      inserted++;
    }
    relocateMisplacedAlsoReadBlocks(bodyEl);
  }

  function bindThumbFallback(card) {
    var img = card.querySelector(".blog-article-thumb img");
    if (!img) return;
    img.addEventListener(
      "error",
      function onErr() {
        img.removeEventListener("error", onErr);
        img.src = BLANK_IMG;
      },
      false
    );
  }

  /** Shimmer pada wrapper gambar sampai img load (bukan skeleton seluruh halaman). */
  function bindLazyImageReveal(wrap, img) {
    if (!wrap || !img) return;
    function done() {
      wrap.classList.remove("blog-media--img-loading");
    }
    if (img.complete && img.naturalWidth > 0) {
      done();
      return;
    }
    if (!wrap.classList.contains("blog-media--img-loading")) {
      wrap.classList.add("blog-media--img-loading");
    }
    img.addEventListener("load", done, { once: true });
    img.addEventListener("error", done, { once: true });
  }

  function bindImgFallbackDefault(img) {
    if (!img) return;
    img.addEventListener(
      "error",
      function onE() {
        img.removeEventListener("error", onE);
        img.src = BLANK_IMG;
      },
      false
    );
  }

  function bindPopularTagStripElement(wrap) {
    if (!wrap) return;
    var strip = wrap.querySelector(".blog-tag-popular-strip");
    var prev = wrap.querySelector(".blog-tag-scroll-btn--prev");
    var next = wrap.querySelector(".blog-tag-scroll-btn--next");
    var step = 220;
    if (prev && strip) {
      prev.addEventListener("click", function () {
        strip.scrollBy({ left: -step, behavior: "smooth" });
      });
    }
    if (next && strip) {
      next.addEventListener("click", function () {
        strip.scrollBy({ left: step, behavior: "smooth" });
      });
    }
  }

  /**
   * Beranda SSR (sama seperti kategori/tag): HTML dari server, hanya hidrasi + muat lagi.
   */
  function hydrateHomeSSR(root) {
    if (!root) return;
    if (root.getAttribute("data-home-hydrated") === "1") return;
    root.setAttribute("data-home-hydrated", "1");
    setPopularVisible(true);
    var carousel = root.querySelector(".blog-feature-carousel");
    if (carousel) bindFeaturedCarousel(carousel);
    bindPopularTagStripElement(root.querySelector(".blog-tag-popular"));

    root
      .querySelectorAll(".blog-feature-slide-media")
      .forEach(function (media) {
        var img = media.querySelector("img");
        if (!img) return;
        bindImgFallbackDefault(img);
        bindLazyImageReveal(media, img);
      });

    var latestThumb = root.querySelector(".blog-latest-thumb");
    if (latestThumb) {
      var limg = latestThumb.querySelector("img");
      bindImgFallbackDefault(limg);
      bindResponsiveThumbCDN(limg, latestThumb, "", 1, "");
      bindLazyImageReveal(latestThumb, limg);
    }

    root
      .querySelectorAll(".blog-latest-stack-thumb")
      .forEach(function (stackThumb) {
        var simg = stackThumb.querySelector("img");
        bindImgFallbackDefault(simg);
        bindResponsiveThumbCDN(simg, stackThumb, "", 1, "");
        bindLazyImageReveal(stackThumb, simg);
      });

    root
      .querySelectorAll(".blog-posts-item [data-thumb-skeleton]")
      .forEach(function (thumb) {
        var img = thumb.querySelector("img");
        function done() {
          thumb.classList.remove("blog-posts-thumb--loading");
        }
        if (!img) {
          done();
          return;
        }
        if (img.complete) {
          done();
          return;
        }
        img.addEventListener("load", done, { once: true });
        img.addEventListener("error", done, { once: true });
      });

    root.querySelectorAll(".blog-posts-item").forEach(function (a) {
      bindThumbFallback(a);
      var ptw = a.querySelector(".blog-posts-thumb");
      var ptim = ptw && ptw.querySelector("img");
      bindResponsiveThumbCDN(ptim, ptw, "", 1, "");
      bindLazyImageReveal(ptw, ptim);
    });

    root.querySelectorAll(".blog-mostviewed-media").forEach(function (media) {
      var img = media.querySelector("img");
      bindResponsiveThumbCDN(img, media);
    });

    /*
     * Neo SSR wrappers (Featured Category lead/stack, Most Popular, Sorotan utama) memakai
     * .blog-media--img-loading langsung pada <a> sehingga tidak tertangkap selector spesifik
     * di atas. Tanpa reveal, .blog-media--img-loading img { opacity: 0 } membuat gambar tak
     * pernah terlihat — pass generik berikut mengikat fallback + reveal untuk sisanya.
     */
    bindPageResponsiveImages(document);

    root.querySelectorAll(".blog-media--img-loading").forEach(function (wrap) {
      if (wrap.getAttribute("data-img-reveal-bound") === "1") return;
      var img = wrap.querySelector("img");
      if (!img) {
        wrap.classList.remove("blog-media--img-loading");
        return;
      }
      bindImgFallbackDefault(img);
      bindLazyImageReveal(wrap, img);
      wrap.setAttribute("data-img-reveal-bound", "1");
    });

    var loadBtn = root.querySelector("[data-home-feed-more]");
    if (loadBtn) {
      var listEl = root.querySelector("#blog-posts-list-ssr");
      var totalPages = parseInt(
        loadBtn.getAttribute("data-total-pages") || "1",
        10
      );
      var visibleStep = parseInt(
        loadBtn.getAttribute("data-home-visible-step") || "0",
        10
      );
      var pendingPosts = [];
      loadBtn.addEventListener("click", async function () {
        if (!listEl) return;
        var page = parseInt(loadBtn.getAttribute("data-next-page") || "2", 10);
        var added = 0;
        loadBtn.disabled = true;
        try {
          if (visibleStep > 0) {
            while (added < visibleStep) {
              if (!pendingPosts.length) {
                if (page > totalPages) break;
                var raw = await api.getPublic(
                  "/posts?page=" + page + "&size=" + FEED_PAGE_SIZE + "&skip=1"
                );
                var chunk = Array.isArray(raw)
                  ? raw
                  : raw && Array.isArray(raw.items)
                  ? raw.items
                  : [];
                chunk = dedupePostsById(chunk).filter(isPublished);
                for (var ci = 0; ci < chunk.length; ci++) {
                  pendingPosts.push(chunk[ci]);
                }
                page += 1;
                if (!pendingPosts.length && page > totalPages) break;
              }
              var p = pendingPosts.shift();
              if (!p) break;
              appendPostsCompactItem(listEl, p);
              added += 1;
            }
          } else {
            var raw = await api.getPublic(
              "/posts?page=" + page + "&size=" + FEED_PAGE_SIZE + "&skip=1"
            );
            var chunk = Array.isArray(raw)
              ? raw
              : raw && Array.isArray(raw.items)
              ? raw.items
              : [];
            chunk = dedupePostsById(chunk).filter(isPublished);
            chunk.forEach(function (p) {
              appendPostsCompactItem(listEl, p);
            });
            page += 1;
          }
          if (
            page > totalPages &&
            (!visibleStep || pendingPosts.length === 0)
          ) {
            var row = loadBtn.closest(".blog-feed-loadmore");
            if (row) row.remove();
            return;
          }
          loadBtn.setAttribute("data-next-page", String(page));
        } catch (e) {
          alert(e.message || "Gagal memuat artikel.");
        } finally {
          loadBtn.disabled = false;
        }
      });
    }

    try {
      var pa = window.PublicSiteAds;
      if (pa && typeof pa.activateSlotsIn === "function") {
        pa.activateSlotsIn(root);
      } else if (pa && typeof pa.pushUnhandledIn === "function") {
        root.querySelectorAll(".site-ad").forEach(function (slot) {
          pa.pushUnhandledIn(slot);
        });
      }
    } catch (eHomeAds) {}

    requestPopularPlacement("home");
    ensurePopularSidebarLoaded();
  }

  /** Tombol "Muat lagi": teks + ikon panah ke bawah. */
  function fillLoadMoreButton(btn) {
    if (!btn) return;
    btn.innerHTML =
      '<span class="blog-feed-loadmore-inner">' +
      '<span class="blog-feed-loadmore-label">Muat lagi</span>' +
      '<svg class="blog-feed-loadmore-arrow" viewBox="0 0 24 24" width="17" height="17" aria-hidden="true" focusable="false">' +
      '<path fill="none" stroke="currentColor" stroke-width="2.25" stroke-linecap="round" stroke-linejoin="round" d="m7 10 5 5 5-5"/>' +
      "</svg>" +
      "</span>";
  }

  function appendPostsCompactItem(list, p) {
    var href = postPublicPath(p);
    var thumb = postThumbnailSrc(p);
    var titleEsc = escapeHtml(p.title || "");
    var rel = relativeTimeId(postTimeIso(p));
    var pr = document.getElementById("public-root");
    var neoMag =
      isNeoSiteLayout() ||
      (pr && pr.getAttribute("data-home-layout") === "neo");
    var a = document.createElement("a");
    if (neoMag) {
      a.className = "blog-posts-item neo-mag-feed-item";
      a.href = href;
      a.innerHTML =
        '<span class="blog-posts-thumb neo-mag-feed-thumb blog-media--img-loading" aria-hidden="true" data-thumb-skeleton>' +
        '<img src="' +
        escapeHtml(thumb) +
        '" alt="" width="88" height="66" loading="lazy" decoding="async" />' +
        "</span>" +
        '<span class="blog-posts-text neo-mag-feed-text">' +
        '<span class="blog-posts-title neo-mag-feed-title">' +
        titleEsc +
        "</span>" +
        (rel
          ? '<span class="neo-mag-feed-byline"><span class="neo-mag-feed-byline-i blog-posts-time">' +
            escapeHtml(rel) +
            "</span></span>"
          : "") +
        "</span>";
    } else {
      a.className = "blog-posts-item";
      a.href = href;
      a.innerHTML =
        '<span class="blog-posts-thumb" aria-hidden="true">' +
        '<img src="' +
        escapeHtml(thumb) +
        '" alt="" width="88" height="66" loading="lazy" decoding="async" />' +
        "</span>" +
        '<span class="blog-posts-text">' +
        '<span class="blog-posts-title">' +
        titleEsc +
        "</span>" +
        (rel
          ? '<span class="blog-posts-time">' + escapeHtml(rel) + "</span>"
          : "") +
        "</span>";
    }
    list.appendChild(a);
    bindThumbFallback(a);
    var ptw = a.querySelector(".blog-posts-thumb");
    var ptim = ptw && ptw.querySelector("img");
    bindResponsiveThumbCDN(ptim, ptw, "", 1, "");
    bindLazyImageReveal(ptw, ptim);
  }

  function refreshCompactListThumbCDN(list) {
    if (!list) return;
    function apply() {
      list.querySelectorAll(".blog-posts-thumb").forEach(function (ptw) {
        var img = ptw.querySelector("img");
        bindResponsiveThumbCDN(img, ptw, "", 1, "");
      });
    }
    if (typeof requestAnimationFrame !== "undefined") {
      requestAnimationFrame(function () {
        requestAnimationFrame(apply);
      });
      return;
    }
    setTimeout(apply, 0);
  }

  function renderNav() {
    if (window.TirtayasaHeader && window.TirtayasaHeader.refresh) {
      window.TirtayasaHeader.refresh();
    }
  }

  /** Halaman statis (Page) di URL /slug — prioritas di atas kategori & artikel. */
  function renderStaticPage(root, page) {
    setPopularVisible(false);
    var main = document.createElement("main");
    main.className = "container py-4 py-md-5";
    var article = document.createElement("article");
    article.className = "mx-auto";
    article.style.maxWidth = "860px";
    var head = document.createElement("header");
    head.className = "mb-3 mb-md-4";
    head.innerHTML =
      '<h1 class="h2 mb-0">' + escapeHtml(page.title || "") + "</h1>";
    var section = document.createElement("section");
    section.className = "blog-page-content";
    section.innerHTML = page.content || "";
    applyExternalAnchorNofollowNewTab(section);
    article.appendChild(head);
    article.appendChild(section);
    main.appendChild(article);
    root.replaceChildren(main);
    var staticTitle = String(page.title || "").trim() || "Halaman";
    setMetaTitle(staticTitle + browserTitleSuffix());
  }

  async function renderSegment(root, slug) {
    try {
      var pageRow = null;
      try {
        pageRow = await api.getPublic(
          "/pages/slug/" + encodeURIComponent(slug)
        );
      } catch (e) {
        if (e.status !== 404) throw e;
      }
      if (pageRow && pageRow.slug) {
        renderStaticPage(root, pageRow);
        return;
      }

      await renderPost(root, { name: "post", slug: slug });
    } catch (err) {
      root.innerHTML =
        '<div class="notice notice-error">' +
        escapeHtml(err.message || "Gagal memuat halaman.") +
        '</div><p class="back-link"><a href="/">← Kembali ke beranda</a></p>';
    }
  }

  async function renderHome(root, opts) {
    opts = opts || {};
    setPopularVisible(true);
    var gen = ++homeRenderGen;

    try {
      var query = getQuery();
      var categoryFromOpts = opts.categorySlug
        ? String(opts.categorySlug).trim()
        : "";
      var tagFromOpts = opts.tagSlug ? String(opts.tagSlug).trim() : "";
      var authorFromOpts = opts.authorSlug
        ? String(opts.authorSlug).trim()
        : "";
      var effectiveCategory = categoryFromOpts || (query.category || "").trim();
      var effectiveTag = tagFromOpts || (query.tag || "").trim();
      var effectiveAuthor = authorFromOpts;
      var hasFilter = !!(
        effectiveCategory ||
        query.q ||
        effectiveTag ||
        effectiveAuthor
      );

      if (hasFilter) {
        var raw = await api.getPublic(postsFilterBootstrapPath(query));
        if (gen !== homeRenderGen) return;
        // no loading text

        var posts = Array.isArray(raw)
          ? raw
          : raw && Array.isArray(raw.items)
          ? raw.items
          : [];
        posts = dedupePostsById(posts);
        if (!Array.isArray(posts) || posts.length === 0) {
          renderPopularSidebar([]);
          root.appendChild(document.createTextNode("Belum ada artikel."));
          requestPopularPlacement("home");
          return;
        }
        var allPublished = sortByPublishedDesc(posts.filter(isPublished));
        // Populer harus global: ambil dari endpoint populer (24 jam) bukan dari hasil filter.
        ensurePopularSidebarLoaded();

        var published = allPublished.slice();
        if (effectiveCategory) {
          published = published.filter(function (p) {
            return p.category && p.category.slug === effectiveCategory;
          });
        }
        if (query.q) {
          var ql = query.q.toLowerCase();
          published = published.filter(function (p) {
            return (p.title || "").toLowerCase().indexOf(ql) !== -1;
          });
        }
        if (effectiveTag) {
          var qt = effectiveTag;
          published = published.filter(function (p) {
            return (p.tags || []).some(function (t) {
              if (!t) return false;
              return (t.slug || "") === qt;
            });
          });
        }
        if (effectiveAuthor) {
          published = published.filter(function (p) {
            return postAuthorMatchesSlug(p, effectiveAuthor);
          });
        }
        if (published.length === 0) {
          root.appendChild(
            document.createTextNode(
              effectiveCategory || query.q
                ? "Tidak ada artikel yang cocok dengan filter ini."
                : "Belum ada artikel yang dipublikasikan."
            )
          );
          return;
        }

        if (
          categoryFromOpts &&
          effectiveCategory &&
          !query.q &&
          !query.tag &&
          location.pathname !== "/"
        ) {
          var catTitle = "";
          try {
            for (var ci = 0; ci < published.length; ci++) {
              var pc =
                published[ci] && published[ci].category
                  ? published[ci].category
                  : null;
              if (pc && (pc.slug || "") === effectiveCategory && pc.name) {
                catTitle = String(pc.name);
                break;
              }
            }
          } catch (e) {}

          var catHead = document.createElement("div");
          catHead.className = "blog-feed-head";
          catHead.innerHTML =
            '<h1 class="blog-section-title blog-section-title--feed">' +
            '<span class="blog-section-title-bar" aria-hidden="true"></span>' +
            escapeHtml(catTitle || effectiveCategory) +
            "</h1>";
          root.appendChild(catHead);

          var catList = document.createElement("div");
          catList.className = "blog-posts-list";
          published.forEach(function (p) {
            appendPostsCompactItem(catList, p);
          });
          root.appendChild(catList);
          refreshCompactListThumbCDN(catList);
          requestPopularPlacement("home");
          setMetaTitle((catTitle || effectiveCategory) + browserTitleSuffix());
          return;
        }

        if (
          tagFromOpts &&
          effectiveTag &&
          !query.q &&
          !effectiveCategory &&
          !effectiveAuthor
        ) {
          var tagTitle = "";
          try {
            for (var ti = 0; ti < published.length; ti++) {
              var tags =
                published[ti] && Array.isArray(published[ti].tags)
                  ? published[ti].tags
                  : [];
              for (var tj = 0; tj < tags.length; tj++) {
                var tt = tags[tj] || null;
                if (tt && (tt.slug || "") === effectiveTag && tt.name) {
                  tagTitle = String(tt.name);
                  break;
                }
              }
              if (tagTitle) break;
            }
          } catch (e) {}

          var tagHead = document.createElement("div");
          tagHead.className = "blog-feed-head";
          tagHead.innerHTML =
            '<h1 class="blog-section-title blog-section-title--feed">' +
            '<span class="blog-section-title-bar" aria-hidden="true"></span>' +
            "Tag: " +
            escapeHtml(tagTitle || effectiveTag) +
            "</h1>";
          root.appendChild(tagHead);

          var tagList = document.createElement("div");
          tagList.className = "blog-posts-list";
          published.forEach(function (tp) {
            appendPostsCompactItem(tagList, tp);
          });
          root.appendChild(tagList);
          refreshCompactListThumbCDN(tagList);
          requestPopularPlacement("home");
          setMetaTitle(
            ("Tag: " + (tagTitle || effectiveTag)).trim() + browserTitleSuffix()
          );
          return;
        }

        if (
          authorFromOpts &&
          effectiveAuthor &&
          !query.q &&
          !effectiveCategory &&
          !effectiveTag
        ) {
          var authorRef = null;
          try {
            authorRef = await api.getPublic(
              "/authors/" + encodeURIComponent(effectiveAuthor)
            );
          } catch (e) {}
          for (var ai = 0; ai < published.length; ai++) {
            if (postAuthorMatchesSlug(published[ai], effectiveAuthor)) {
              if (!authorRef) authorRef = published[ai].author || null;
              break;
            }
          }
          var authorName =
            authorRef && authorRef.name
              ? String(authorRef.name)
              : effectiveAuthor;
          var authorProfile =
            authorRef && authorRef.profile ? String(authorRef.profile) : "";
          var authorImg =
            authorRef && authorRef.image
              ? normalizeAvatarImgSrc(authorRef.image)
              : "";
          var authorPostsCount = published.length;
          var authorRoleLabel = formatAuthorRole(
            authorRef && authorRef.role ? authorRef.role : ""
          );

          var authorCard = document.createElement("section");
          authorCard.className = "blog-author-card";
          authorCard.setAttribute("aria-label", "Profil author");
          authorCard.innerHTML =
            '<div class="blog-author-card-banner" aria-hidden="true"></div>' +
            '<div class="blog-author-card-body">' +
            '<div class="blog-author-card-avatar">' +
            (authorImg
              ? '<img class="blog-author-card-img" src="' +
                escapeHtml(authorImg) +
                '" alt="' +
                escapeHtml(authorName) +
                '" loading="lazy" decoding="async" width="120" height="120" />'
              : '<span class="blog-author-card-img blog-author-card-img--fallback" aria-hidden="true">' +
                escapeHtml((authorName || "?").charAt(0).toUpperCase()) +
                "</span>") +
            "</div>" +
            '<div class="blog-author-card-main">' +
            '<h1 class="blog-author-card-name">' +
            escapeHtml(authorName) +
            "</h1>" +
            '<p class="blog-author-card-headline">' +
            escapeHtml(authorRoleLabel) +
            "</p>" +
            (authorProfile
              ? '<p class="blog-author-card-bio">' +
                escapeHtml(authorProfile) +
                "</p>"
              : '<p class="blog-author-card-bio blog-author-card-bio--empty">Belum ada deskripsi profil.</p>') +
            '<p class="blog-author-card-metric"><strong>' +
            escapeHtml(String(authorPostsCount)) +
            "</strong> artikel dipublikasikan</p>" +
            '<div class="blog-author-card-actions">' +
            '<a class="blog-author-card-btn blog-author-card-btn--primary" href="#author-posts">Lihat artikel</a>' +
            '<a class="blog-author-card-btn blog-author-card-btn--outline" href="/">Beranda</a>' +
            "</div>" +
            "</div>" +
            "</div>";
          root.appendChild(authorCard);

          var postsAnchor = document.createElement("div");
          postsAnchor.id = "author-posts";
          postsAnchor.className = "blog-author-posts-anchor";
          postsAnchor.innerHTML =
            '<h2 class="blog-author-posts-title">Artikel</h2>';
          root.appendChild(postsAnchor);

          var authorList = document.createElement("div");
          authorList.className = "blog-posts-list";
          published.forEach(function (pp) {
            appendPostsCompactItem(authorList, pp);
          });
          root.appendChild(authorList);
          refreshCompactListThumbCDN(authorList);
          requestPopularPlacement("home");
          setMetaTitle(
            ("Author: " + (authorName || effectiveAuthor)).trim() +
              browserTitleSuffix()
          );
          return;
        }

        var featureBlock = document.createElement("div");
        featureBlock.className = "blog-home-feature-block";

        var latestPost = published.length ? published[0] : null;
        var slidePosts = selectHomeCarouselSlides(published, latestPost);
        if (slidePosts.length) {
          var carousel = buildFeaturedCarousel(slidePosts);
          featureBlock.appendChild(carousel);
          bindFeaturedCarousel(carousel);
        }

        var latestEl = buildLatestPostSection(latestPost);
        if (latestEl) featureBlock.appendChild(latestEl);

        // blog-picks rekomendasi dihapus

        if (featureBlock.childNodes.length) root.appendChild(featureBlock);

        var feedPosts = latestPost
          ? published.filter(function (p) {
              return p.id !== latestPost.id;
            })
          : published.slice();

        var feedHead = document.createElement("div");
        feedHead.className = "blog-feed-head";
        feedHead.id = "blog-article-feed";
        feedHead.innerHTML =
          '<h2 class="blog-section-title blog-section-title--feed">' +
          '<span class="blog-section-title-bar" aria-hidden="true"></span>' +
          "Artikel terbaru" +
          "</h2>";
        root.appendChild(feedHead);

        var list = document.createElement("div");
        list.className = "blog-posts-list";
        var feedTotal = feedPosts.length;
        var idx = 0;
        var firstBatch = Math.min(FEED_PAGE_SIZE, feedTotal);
        for (; idx < firstBatch; idx++) {
          appendPostsCompactItem(list, feedPosts[idx]);
        }
        root.appendChild(list);

        if (feedTotal > FEED_PAGE_SIZE) {
          var loadRow = document.createElement("div");
          loadRow.className = "blog-feed-loadmore";
          var loadBtn = document.createElement("button");
          loadBtn.type = "button";
          loadBtn.className = "btn btn-ghost blog-feed-loadmore-btn";
          fillLoadMoreButton(loadBtn);
          loadBtn.addEventListener("click", function () {
            if (gen !== homeRenderGen) return;
            var end = Math.min(idx + FEED_PAGE_SIZE, feedTotal);
            for (; idx < end; idx++) {
              appendPostsCompactItem(list, feedPosts[idx]);
            }
            if (idx >= feedTotal) {
              loadRow.remove();
            }
          });
          loadRow.appendChild(loadBtn);
          root.appendChild(loadRow);
        }
        requestPopularPlacement("home");
        return;
      }

      /* Tanpa filter: bootstrap ringan + feed lewat pagination API (15 per permintaan). */
      var results = await Promise.all([
        api.getPublic("/posts?page=1&size=" + HOME_BOOTSTRAP_SIZE),
        api.getPublic("/posts?page=1&size=" + FEED_PAGE_SIZE + "&skip=1"),
      ]);
      if (gen !== homeRenderGen) return;
      // no loading text

      var bootRaw = results[0];
      var feedRaw = results[1];

      var bootPosts = Array.isArray(bootRaw)
        ? bootRaw
        : bootRaw && Array.isArray(bootRaw.items)
        ? bootRaw.items
        : [];
      bootPosts = dedupePostsById(bootPosts);
      if (!bootPosts.length) {
        renderPopularSidebar([]);
        root.appendChild(document.createTextNode("Belum ada artikel."));
        return;
      }

      var published = sortByPublishedDesc(bootPosts.filter(isPublished));
      // Populer harus global: ambil dari endpoint populer (24 jam) bukan dari bootstrap beranda.
      ensurePopularSidebarLoaded();

      var featureBlock = document.createElement("div");
      featureBlock.className = "blog-home-feature-block";

      var latestPost2 = published.length ? published[0] : null;
      var slidePosts2 = selectHomeCarouselSlides(published, latestPost2);
      if (slidePosts2.length) {
        var carousel2 = buildFeaturedCarousel(slidePosts2);
        featureBlock.appendChild(carousel2);
        bindFeaturedCarousel(carousel2);
      }

      var latestEl2 = buildLatestPostSection(latestPost2);
      if (latestEl2) featureBlock.appendChild(latestEl2);

      // blog-picks rekomendasi dihapus

      if (featureBlock.childNodes.length) root.appendChild(featureBlock);

      var feedHead2 = document.createElement("div");
      feedHead2.className = "blog-feed-head";
      feedHead2.id = "blog-article-feed";
      feedHead2.innerHTML =
        '<h2 class="blog-section-title blog-section-title--feed">' +
        '<span class="blog-section-title-bar" aria-hidden="true"></span>' +
        "Artikel terbaru" +
        "</h2>";
      root.appendChild(feedHead2);

      var feedItems = Array.isArray(feedRaw)
        ? feedRaw
        : feedRaw && Array.isArray(feedRaw.items)
        ? feedRaw.items
        : [];
      feedItems = dedupePostsById(feedItems);
      var feedTotalPages =
        feedRaw && feedRaw.total_pages != null
          ? Number(feedRaw.total_pages)
          : feedItems.length
          ? 1
          : 0;

      var list2 = document.createElement("div");
      list2.className = "blog-posts-list";
      feedItems.forEach(function (p) {
        appendPostsCompactItem(list2, p);
      });
      root.appendChild(list2);

      if (feedTotalPages > 1) {
        var loadRow2 = document.createElement("div");
        loadRow2.className = "blog-feed-loadmore";
        var loadBtn2 = document.createElement("button");
        loadBtn2.type = "button";
        loadBtn2.className = "btn btn-ghost blog-feed-loadmore-btn";
        fillLoadMoreButton(loadBtn2);
        var feedPage = 1;
        loadBtn2.addEventListener("click", async function () {
          if (gen !== homeRenderGen) return;
          feedPage += 1;
          loadBtn2.disabled = true;
          try {
            var next = await api.getPublic(
              "/posts?page=" + feedPage + "&size=" + FEED_PAGE_SIZE + "&skip=1"
            );
            if (gen !== homeRenderGen) return;
            var chunk = Array.isArray(next)
              ? next
              : next && Array.isArray(next.items)
              ? next.items
              : [];
            chunk = dedupePostsById(chunk);
            chunk.forEach(function (p) {
              appendPostsCompactItem(list2, p);
            });
            var tp =
              next && next.total_pages != null
                ? Number(next.total_pages)
                : feedTotalPages;
            if (feedPage >= tp) {
              loadRow2.remove();
              return;
            }
          } catch (e) {
            alert(e.message || "Gagal memuat artikel.");
            feedPage -= 1;
          } finally {
            if (loadBtn2.parentNode) loadBtn2.disabled = false;
          }
        });
        loadRow2.appendChild(loadBtn2);
        root.appendChild(loadRow2);
      }
      requestPopularPlacement("home");
    } catch (err) {
      if (gen === homeRenderGen) {
        renderPopularSidebar([]);
        var errBox = document.createElement("div");
        errBox.className = "notice notice-error";
        errBox.textContent = err.message || "Gagal memuat artikel.";
        root.appendChild(errBox);
        requestPopularPlacement("home");
      }
    }
  }

  var POSTS_PAGE_SIZE = 12;

  async function renderPostsList(root, route) {
    setPopularVisible(true);
    var gen = ++homeRenderGen;
    var neoMag = isNeoSiteLayout();

    try {
      var page = route.page || 1;
      var raw = await api.getPublic(
        "/posts?size=" + POSTS_PAGE_SIZE + "&page=" + page
      );
      if (gen !== homeRenderGen) return;
      // no loading text

      if (!neoMag) {
        ensurePopularSidebarLoaded();
      }

      var items = Array.isArray(raw)
        ? raw
        : raw && Array.isArray(raw.items)
        ? raw.items
        : [];
      items = dedupePostsById(items);
      var total = raw && raw.total != null ? Number(raw.total) : items.length;
      var totalPages =
        raw && raw.total_pages != null
          ? Number(raw.total_pages)
          : Math.max(1, Math.ceil(total / POSTS_PAGE_SIZE));

      var head = document.createElement("div");
      head.className =
        "blog-feed-head" + (neoMag ? " neo-mag-feed-head" : "");
      head.innerHTML =
        '<h1 class="blog-section-title blog-section-title--feed">' +
        '<span class="blog-section-title-bar" aria-hidden="true"></span>' +
        "Semua artikel" +
        "</h1>";
      root.appendChild(head);

      if (!items.length) {
        root.appendChild(
          document.createTextNode("Belum ada artikel yang dipublikasikan.")
        );
        requestPopularPlacement("posts");
        return;
      }

      var list = document.createElement("div");
      list.className =
        "blog-posts-list" + (neoMag ? " neo-mag-feed-list" : "");
      items.forEach(function (p) {
        appendPostsCompactItem(list, p);
      });
      root.appendChild(list);
      refreshCompactListThumbCDN(list);

      if (totalPages > 1) {
        var nav = document.createElement("nav");
        nav.className = "blog-pagination";
        nav.setAttribute("aria-label", "Navigasi halaman");
        var ul = document.createElement("div");
        ul.className = "blog-pagination-inner";

        function appendLink(pnum, label, opts) {
          opts = opts || {};
          var active = opts.active;
          var disabled = opts.disabled;
          if (disabled) {
            var s = document.createElement("span");
            s.className = "blog-pagination-btn is-disabled";
            s.setAttribute("aria-disabled", "true");
            s.textContent = label;
            ul.appendChild(s);
            return;
          }
          var a = document.createElement("a");
          a.className = "blog-pagination-btn" + (active ? " is-active" : "");
          a.href =
            pnum === 1
              ? "/posts"
              : "/posts?page=" + encodeURIComponent(String(pnum));
          a.textContent = label;
          ul.appendChild(a);
        }

        appendLink(page - 1, "←", { disabled: page <= 1 });

        function appendEllipsis() {
          var ell = document.createElement("span");
          ell.className = "blog-pagination-ellipsis";
          ell.setAttribute("aria-hidden", "true");
          ell.textContent = "…";
          ul.appendChild(ell);
        }

        var T = totalPages;
        var p = page;
        if (T <= 5) {
          for (var n = 1; n <= T; n++) {
            appendLink(n, String(n), { active: p === n });
          }
        } else if (p <= 3) {
          appendLink(1, "1", { active: p === 1 });
          appendLink(2, "2", { active: p === 2 });
          appendLink(3, "3", { active: p === 3 });
          appendEllipsis();
          appendLink(T, String(T), { active: p === T });
        } else if (p >= T - 2) {
          appendLink(1, "1", { active: p === 1 });
          appendEllipsis();
          appendLink(T - 2, String(T - 2), { active: p === T - 2 });
          appendLink(T - 1, String(T - 1), { active: p === T - 1 });
          appendLink(T, String(T), { active: p === T });
        } else {
          appendLink(1, "1", { active: p === 1 });
          appendEllipsis();
          appendLink(p - 1, String(p - 1), { active: false });
          appendLink(p, String(p), { active: true });
          appendLink(p + 1, String(p + 1), { active: false });
          appendEllipsis();
          appendLink(T, String(T), { active: p === T });
        }

        appendLink(page + 1, "→", { disabled: page >= totalPages });

        nav.appendChild(ul);
        root.appendChild(nav);
      }
      requestPopularPlacement("posts");
    } catch (err) {
      if (gen === homeRenderGen) {
        renderPopularSidebar([]);
        var errBox = document.createElement("div");
        errBox.className = "notice notice-error";
        errBox.textContent = err.message || "Gagal memuat artikel.";
        root.appendChild(errBox);
        requestPopularPlacement("posts");
      }
    }
  }

  function normalizeLinkHostname(host) {
    return String(host || "")
      .toLowerCase()
      .replace(/\.$/, "");
  }

  function mergeRelAttrTokens(existing) {
    var extra = Array.prototype.slice.call(arguments, 1);
    var have = Object.create(null);
    var out = [];
    function addToken(tok) {
      var s = String(tok || "").trim();
      if (!s) return;
      var k = s.toLowerCase();
      if (have[k]) return;
      have[k] = true;
      out.push(s);
    }
    String(existing || "")
      .split(/\s+/)
      .forEach(addToken);
    extra.forEach(addToken);
    return out.join(" ");
  }

  /** Tautan ke host selain situs: nofollow + tab baru (selaras dengan SSR). */
  function applyExternalAnchorNofollowNewTab(rootEl) {
    if (!rootEl) return;
    var origin = canonicalOrigin();
    if (!origin) {
      try {
        origin = location.origin;
      } catch (e) {
        return;
      }
    }
    var siteHost = "";
    try {
      siteHost = normalizeLinkHostname(new URL(origin).hostname);
    } catch (e2) {
      return;
    }
    if (!siteHost) return;
    var links = rootEl.querySelectorAll("a[href]");
    for (var i = 0; i < links.length; i++) {
      var a = links[i];
      var href = (a.getAttribute("href") || "").trim();
      if (!href) continue;
      var low = href.toLowerCase();
      if (
        low.indexOf("javascript:") === 0 ||
        low.indexOf("mailto:") === 0 ||
        low.indexOf("tel:") === 0 ||
        low.indexOf("data:") === 0
      ) {
        continue;
      }
      var u;
      try {
        u = new URL(href, origin);
      } catch (e3) {
        continue;
      }
      if (!u.host) continue;
      if (normalizeLinkHostname(u.hostname) === siteHost) continue;
      var rel = (a.getAttribute("rel") || "").trim();
      a.setAttribute(
        "rel",
        mergeRelAttrTokens(rel, "nofollow", "noopener", "noreferrer")
      );
      a.setAttribute("target", "_blank");
    }
  }

  /** Hilangkan judul duplikat di awal HTML konten (sering sama dengan &lt;h1&gt; halaman). */
  function stripLeadingDuplicateTitleHtml(html, title) {
    if (!html || typeof html !== "string") return html || "";
    var t = (title || "").trim().toLowerCase();
    if (!t) return html;
    var re = /^\s*<h1\b[^>]*>([\s\S]*?)<\/h1>\s*/i;
    var m = html.match(re);
    if (!m) return html;
    var inner = m[1]
      .replace(/<[^>]+>/g, " ")
      .replace(/\s+/g, " ")
      .trim()
      .toLowerCase();
    if (inner === t || inner.indexOf(t) === 0) {
      return html.replace(re, "");
    }
    return html;
  }

  function normalizePathname(p) {
    if (p == null || p === "") return "/";
    var s = String(p).replace(/\/+$/, "");
    return s === "" ? "/" : s;
  }

  /** Cari target anchor (#id) di dokumen; mendukung id dengan karakter khusus. */
  function findElForHashId(rawId) {
    if (rawId == null || rawId === "") return null;
    var id;
    try {
      id = decodeURIComponent(String(rawId).replace(/^#/, ""));
    } catch (e) {
      return null;
    }
    if (!id) return null;
    var el = document.getElementById(id);
    if (el) return el;
    if (typeof CSS !== "undefined" && CSS.escape) {
      try {
        el = document.querySelector('[id="' + CSS.escape(id) + '"]');
      } catch (e2) {
        el = null;
      }
    }
    return el;
  }

  function scrollToArticleHashIfPresent(behavior) {
    var raw = (location.hash || "").replace(/^#/, "");
    if (!raw) return;
    var el = findElForHashId(raw);
    if (!el) return;
    var b = behavior === "smooth" ? "smooth" : "auto";
    el.scrollIntoView({ behavior: b, block: "start" });
  }

  /** Tunda pekerjaan non-kritis setelah paint pertama (mengurangi CLS pada badan artikel). */
  function scheduleAfterPaintIdle(fn) {
    if (typeof requestAnimationFrame !== "undefined") {
      requestAnimationFrame(function () {
        requestAnimationFrame(function () {
          if (typeof requestIdleCallback !== "undefined") {
            requestIdleCallback(fn, { timeout: 2000 });
          } else {
            setTimeout(fn, 150);
          }
        });
      });
    } else {
      setTimeout(fn, 150);
    }
  }

  function attachArticleFontSize(article, bodyEl) {
    var input = article.querySelector("#blog-fontsize-range");
    if (!input || !bodyEl) return;
    var isNeo =
      document.body &&
      document.body.classList.contains("site-layout-neo");
    var neoBodyScale = 1.03;

    function readBaseFontPx() {
      try {
        var inline = bodyEl.style.getPropertyValue("--content-font-size");
        if (inline) {
          var fromVar = parseFloat(String(inline).replace(/px$/i, ""));
          if (!isNaN(fromVar)) return fromVar;
        }
        var px = parseFloat(window.getComputedStyle(bodyEl).fontSize);
        if (isNaN(px)) return null;
        if (isNeo) px = px / neoBodyScale;
        return px;
      } catch (e) {
        return null;
      }
    }

    function syncSliderFromCss() {
      var px = readBaseFontPx();
      if (px == null || isNaN(px)) return;
      var v = Math.round(px);
      if (v >= 12 && v <= 28) input.value = String(v);
    }

    if (typeof requestAnimationFrame !== "undefined") {
      requestAnimationFrame(function () {
        requestAnimationFrame(syncSliderFromCss);
      });
    } else {
      setTimeout(syncSliderFromCss, 0);
    }

    var apply = function () {
      var n = parseInt(input.value || "17", 10);
      if (isNaN(n) || n < 12 || n > 28) return;
      bodyEl.style.setProperty("--content-font-size", n + "px");
      bodyEl.style.removeProperty("font-size");
    };

    input.addEventListener("input", apply, { passive: true });
    input.addEventListener("change", apply, { passive: true });
  }

  function attachArticleShare(article, shareURL, shareTitle) {
    var shareRoot = article.querySelector(".blog-share");
    if (!shareRoot) return;
    var toggleMore = ensureShareMenu(article, {
      url: shareURL,
      title: shareTitle,
    });

    shareRoot.addEventListener("click", function (e) {
      var btn =
        e.target && e.target.closest
          ? e.target.closest("button[data-share]")
          : null;
      if (!btn) return;
      e.preventDefault();
      var kind = btn.getAttribute("data-share");
      if (kind === "more") {
        toggleMore();
        return;
      }
      if (kind === "copy") {
        copyToClipboard(shareURL).then(function (ok) {
          btn.textContent = ok ? "✓" : "×";
          setTimeout(function () {
            btn.textContent = "⧉";
          }, 900);
        });
        return;
      }
      if (kind === "fb") {
        copyToClipboard((shareTitle ? shareTitle + " " : "") + shareURL).then(
          function (ok) {
            btn.textContent = ok ? "✓" : "f";
            setTimeout(function () {
              btn.textContent = "f";
            }, 900);
          }
        );
      }
      var u = buildShareURL(kind, shareURL, shareTitle);
      if (u) openSharePopup(u);
    });
  }

  function buildArticleTOC(article, bodyEl) {
    var toc = article.querySelector("#blog-article-toc");
    if (!toc || !bodyEl) return;
    var heads = bodyEl.querySelectorAll("h2, h3");
    if (!heads || !heads.length) return;

    var used = Object.create(null);
    function slugifyId(s) {
      var t = String(s || "")
        .toLowerCase()
        .trim()
        .replace(/['"]/g, "")
        .replace(/[^a-z0-9]+/g, "-")
        .replace(/^-+|-+$/g, "");
      if (!t) t = "section";
      if (used[t]) {
        used[t] += 1;
        t = t + "-" + used[t];
      } else {
        used[t] = 1;
      }
      return t;
    }

    var items = [];
    heads.forEach(function (h) {
      var level = String(h.tagName || "").toLowerCase() === "h3" ? 3 : 2;
      var label = (h.textContent || "").trim();
      if (!label) return;
      var id = (h.getAttribute("id") || "").trim();
      if (!id) {
        id = slugifyId(label);
        h.setAttribute("id", id);
      } else {
        if (used[id]) {
          used[id] += 1;
          id = id + "-" + used[id];
          h.setAttribute("id", id);
        } else {
          used[id] = 1;
        }
      }
      items.push({ id: id, label: label, level: level });
    });
    if (!items.length) return;

    var groups = [];
    var current = null;
    items.forEach(function (it) {
      if (it.level === 2) {
        current = { h2: it, h3: [] };
        groups.push(current);
        return;
      }
      if (it.level === 3) {
        if (!current) {
          current = { h2: null, h3: [] };
          groups.push(current);
        }
        current.h3.push(it);
      }
    });

    var listHtml = groups
      .map(function (g) {
        var h2Html = g.h2
          ? '<a class="blog-toc-link" href="#' +
            escapeHtml(g.h2.id) +
            '">' +
            escapeHtml(g.h2.label) +
            "</a>"
          : "";
        var h3Html =
          g.h3 && g.h3.length
            ? '<ul class="blog-toc-sublist">' +
              g.h3
                .map(function (c) {
                  return (
                    '<li class="blog-toc-subitem"><a class="blog-toc-sublink" href="#' +
                    escapeHtml(c.id) +
                    '">' +
                    escapeHtml(c.label) +
                    "</a></li>"
                  );
                })
                .join("") +
              "</ul>"
            : "";
        return '<li class="blog-toc-item">' + h2Html + h3Html + "</li>";
      })
      .join("");

    toc.innerHTML =
      '<div class="blog-toc-head">' +
      '<div class="blog-toc-title">Daftar Isi</div>' +
      '<button type="button" class="blog-toc-toggle" aria-expanded="false">Tampilkan</button>' +
      "</div>" +
      '<ul class="blog-toc-list" hidden>' +
      listHtml +
      "</ul>";
    toc.hidden = false;

    var toggle = toc.querySelector(".blog-toc-toggle");
    if (toggle) {
      toggle.addEventListener("click", function (e) {
        if (e && e.preventDefault) e.preventDefault();
        if (e && e.stopPropagation) e.stopPropagation();
        var list = toc.querySelector(".blog-toc-list");
        var expanded = toggle.getAttribute("aria-expanded") !== "false";
        if (expanded) {
          toggle.setAttribute("aria-expanded", "false");
          toggle.textContent = "Tampilkan";
          if (list) {
            list.hidden = true;
            list.style.display = "none";
          }
        } else {
          toggle.setAttribute("aria-expanded", "true");
          toggle.textContent = "Sembunyikan";
          if (list) {
            list.hidden = false;
            list.style.display = "";
          }
        }
      });
    }
  }

  function isNeoSiteLayout() {
    try {
      return !!(
        document.body &&
        document.body.classList.contains("site-layout-neo")
      );
    } catch (e) {
      return false;
    }
  }

  var ARTICLE_META_ICO_TIME =
    '<svg class="blog-article-meta-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>';
  var ARTICLE_META_ICO_VIEWS =
    '<svg class="blog-article-meta-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" d="M4 19V9M10 19V5M16 19v-6M22 19V3"/></svg>';

  var ARTICLE_FONTSIZE_HTML =
    '<div class="blog-article-fontsize blog-article-fontsize--neo" aria-label="Pengaturan ukuran teks">' +
    '<span class="blog-article-fontsize-a blog-article-fontsize-a--sm" aria-hidden="true">A</span>' +
    '<input id="blog-fontsize-range" class="blog-article-fontsize-range" type="range" min="14" max="24" step="1" value="17" aria-label="Atur ukuran teks artikel" />' +
    '<span class="blog-article-fontsize-a blog-article-fontsize-a--lg" aria-hidden="true">A</span>' +
    "</div>";

  function buildNeoArticleTopHTML(p, breadcrumbHtml, heroHtml, opts) {
    var catName = opts.catName || "";
    var catPath = opts.catPath || "";
    var metaLine = opts.metaLine || "";
    var viewCount = opts.viewCount || "";
    var authorName = opts.authorName || "";
    var authorImgSrc = opts.authorImgSrc || "";
    var avatarLetter = opts.avatarLetter || "?";
    var authorPath = opts.authorPath || "#";
    var timeIso = opts.timeIso || "";
    var draftBadge = opts.draftBadge || "";

    var categoryHtml = "";
    if (catName) {
      if (catPath) {
        categoryHtml =
          '<div class="blog-article-category-row"><a href="' +
          escapeHtml(catPath) +
          '" class="blog-article-category blog-article-category--neo">' +
          escapeHtml(catName) +
          "</a></div>";
      } else {
        categoryHtml =
          '<div class="blog-article-category-row"><span class="blog-article-category blog-article-category--neo">' +
          escapeHtml(catName) +
          "</span></div>";
      }
    }

    var metaNeoHtml = "";
    if (metaLine || viewCount) {
      var bits = [];
      if (metaLine) {
        bits.push(
          '<span class="blog-article-meta-item blog-article-meta-item--time">' +
            ARTICLE_META_ICO_TIME +
            "<time" +
            (timeIso ? ' datetime="' + escapeHtml(timeIso) + '"' : "") +
            ">" +
            escapeHtml(metaLine) +
            "</time></span>"
        );
      }
      if (viewCount) {
        bits.push(
          '<span class="blog-article-meta-item blog-article-meta-item--views">' +
            ARTICLE_META_ICO_VIEWS +
            "<span>" +
            escapeHtml(viewCount) +
            "</span></span>"
        );
      }
      metaNeoHtml =
        '<div class="blog-article-byline-meta-neo" role="group" aria-label="Informasi artikel">' +
        bits.join("") +
        "</div>";
    }

    return (
      heroHtml +
      '<header class="blog-article-head blog-article-head--neo">' +
      breadcrumbHtml +
      categoryHtml +
      "<h1>" +
      escapeHtml(p.title || "") +
      draftBadge +
      "</h1>" +
      '<div class="blog-article-byline blog-article-byline--neo">' +
      '<span class="blog-article-avatar" aria-hidden="true">' +
      (authorImgSrc
        ? '<img class="blog-article-avatar-img" src="' +
          escapeHtml(authorImgSrc) +
          '" alt="" loading="lazy" decoding="async" />'
        : escapeHtml(avatarLetter)) +
      "</span>" +
      '<a class="blog-article-byline-name" href="' +
      escapeHtml(authorPath) +
      '">' +
      escapeHtml(authorName) +
      "</a>" +
      metaNeoHtml +
      "</div></header>"
    );
  }

  async function renderPost(root, route) {
    try {
      setPopularVisible(true);
      ensurePopularSidebarLoaded();

      var p;
      if (route.slug) {
        p = await api.getPublic(
          "/posts/slug/" + encodeURIComponent(route.slug)
        );
      } else if (route.id != null) {
        p = await api.getPublic("/posts/" + route.id);
      } else {
        throw new Error("Rute artikel tidak valid.");
      }
      if (route.id != null && p && p.slug && history.replaceState) {
        history.replaceState(null, "", postPublicPath(p));
        if (location.hash) location.hash = "";
      }
      if (!isPublished(p) && !api.getToken()) {
        root.innerHTML =
          '<div class="notice notice-info">Artikel ini belum dipublikasikan atau tidak tersedia untuk pengunjung.</div>' +
          '<p class="back-link"><a href="/">← Kembali ke beranda</a></p>';
        return;
      }
      var tabTitle = String(p.title || "").trim();
      setMetaTitle(tabTitle || "Artikel");
      root.innerHTML = "";
      var wrap = document.createElement("div");
      wrap.className = "blog-article-layout";
      var isNeo = isNeoSiteLayout();
      var article = document.createElement("article");
      article.className =
        "blog-article-single" + (isNeo ? " blog-article-single--neo" : "");
      var metaLine = "";
      var pti = postTimeIso(p);
      if (pti) {
        try {
          metaLine = new Date(pti).toLocaleString("id-ID", {
            dateStyle: "long",
            timeStyle: "short",
            timeZone: localTimezone,
          });
        } catch (e) {}
      }
      // Hilangkan kata "pukul" (format locale Indonesia sering menambahkan ini).
      if (metaLine) metaLine = metaLine.replace(/\s+pukul\s+/i, " ");
      if (isNeo && pti) {
        try {
          metaLine = new Date(pti).toLocaleDateString("id-ID", {
            day: "2-digit",
            month: "short",
            year: "numeric",
            timeZone: localTimezone,
          });
        } catch (eNeoDate) {}
      }
      var author = p.author && p.author.name ? p.author.name : "";
      var authorImg =
        p.author && p.author.image ? String(p.author.image || "").trim() : "";
      var catName = p.category && p.category.name ? p.category.name : "";
      var catSlug = p.category && p.category.slug ? p.category.slug : "";
      var relatedPosts = catSlug
        ? await getRelatedPostsByCategory(
            catSlug,
            p.id,
            publicAlsoReadRelatedFetch()
          )
        : [];
      var draftBadge =
        !isPublished(p) && api.getToken()
          ? ' <span class="draft-badge">Draf</span>'
          : "";

      var authorName = author || "Tim redaksi";
      var avatarLetter = authorName.charAt(0)
        ? authorName.charAt(0).toUpperCase()
        : "?";
      var authorImgSrc = authorImg ? normalizeAvatarImgSrc(authorImg) : "";
      var viewLine = "";
      var viewCountNeo = "";
      if (p.view != null && p.view !== "") {
        viewCountNeo = Number(p.view).toLocaleString("id-ID");
        viewLine = viewCountNeo + " dibaca";
      }
      var bylineMetaTop = "";
      if (metaLine || viewLine) {
        bylineMetaTop = (metaLine || "-") + " · " + (viewLine || "0 dibaca");
      }

      var shareURL = canonicalOrigin() + postPublicPath(p);
      var shareTitle = String(p.title || "");

      var breadcrumbBits = ['<li><a href="/">Home</a></li>'];
      var slugMode = "";
      try {
        slugMode = String(
          (window.TirtayasaPublicConfig || {}).articleSlugMode || ""
        )
          .toLowerCase()
          .trim();
      } catch (e) {}
      var catObj = p.category || null;
      var parentSlug = "";
      var parentName = "";
      if (catObj) {
        parentSlug = catObj.parent_slug
          ? String(catObj.parent_slug).trim()
          : catObj.parentSlug
          ? String(catObj.parentSlug).trim()
          : catObj.parent && catObj.parent.slug
          ? String(catObj.parent.slug).trim()
          : "";
        parentName = catObj.parent_name
          ? String(catObj.parent_name).trim()
          : catObj.parentName
          ? String(catObj.parentName).trim()
          : catObj.parent && catObj.parent.name
          ? String(catObj.parent.name).trim()
          : "";
      }
      if (slugMode === "category_nested" && parentSlug && parentName) {
        breadcrumbBits.push(
          '<li><a href="' +
            escapeHtml("/" + encodeURIComponent(parentSlug)) +
            '">' +
            escapeHtml(parentName) +
            "</a></li>"
        );
      }
      var catPath = "";
      if (catName && catSlug) {
        catPath =
          slugMode === "category_nested" && parentSlug && parentName
            ? "/" +
              encodeURIComponent(parentSlug) +
              "/" +
              encodeURIComponent(catSlug)
            : categoryPublicPath(catSlug);
        breadcrumbBits.push(
          '<li><a href="' +
            escapeHtml(catPath) +
            '">' +
            escapeHtml(catName) +
            "</a></li>"
        );
      } else if (catName) {
        breadcrumbBits.push("<li>" + escapeHtml(catName) + "</li>");
      }
      breadcrumbBits.push(
        '<li><span aria-current="page">' +
          escapeHtml(p.title || "") +
          "</span></li>"
      );
      var breadcrumbHtml =
        '<nav class="blog-article-breadcrumb" aria-label="Breadcrumb"><ol class="blog-article-breadcrumb-list">' +
        breadcrumbBits.join("") +
        "</ol></nav>";

      var bylineHtml =
        (bylineMetaTop
          ? '<div class="blog-article-byline-meta-top">' +
            escapeHtml(bylineMetaTop) +
            "</div>"
          : "") +
        '<div class="blog-article-byline">' +
        '<span class="blog-article-avatar" aria-hidden="true">' +
        (authorImgSrc
          ? '<img class="blog-article-avatar-img" src="' +
            escapeHtml(authorImgSrc) +
            '" alt="" loading="lazy" decoding="async" />'
          : escapeHtml(avatarLetter)) +
        "</span>" +
        '<div class="blog-article-byline-main">' +
        '<div class="blog-article-byline-name">' +
        escapeHtml(authorName) +
        "</div>" +
        '<div class="blog-article-byline-meta">Author</div>' +
        "</div>" +
        '<div class="blog-share" aria-label="Share">' +
        '<button type="button" class="blog-share-btn blog-share-btn--wa" data-share="wa" aria-label="Share WhatsApp">' +
        '<svg class="blog-share-btn-ico" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M20.52 3.48A11.86 11.86 0 0 0 12.06 0C5.48 0 .12 5.36.12 11.94c0 2.1.55 4.15 1.6 5.97L0 24l6.25-1.64a11.9 11.9 0 0 0 5.7 1.45h.01c6.58 0 11.94-5.36 11.94-11.94 0-3.19-1.24-6.19-3.38-8.39zM11.95 21.5h-.01a9.9 9.9 0 0 1-5.05-1.38l-.36-.21-3.71.97.99-3.62-.24-.37a9.88 9.88 0 0 1-1.52-5.25C2.05 6.5 6.5 2.06 12.06 2.06c2.62 0 5.08 1.02 6.93 2.87a9.74 9.74 0 0 1 2.87 6.93c0 5.56-4.45 9.64-9.91 9.64zm5.44-7.39c-.3-.15-1.76-.87-2.03-.97-.27-.1-.47-.15-.67.15-.2.3-.77.97-.94 1.17-.17.2-.35.22-.65.07-.3-.15-1.25-.46-2.38-1.47-.88-.78-1.47-1.74-1.64-2.04-.17-.3-.02-.46.13-.61.13-.13.3-.35.45-.52.15-.17.2-.3.3-.5.1-.2.05-.37-.03-.52-.08-.15-.67-1.61-.92-2.2-.24-.58-.49-.5-.67-.5h-.57c-.2 0-.52.07-.79.37-.27.3-1.04 1.02-1.04 2.49 0 1.47 1.07 2.89 1.22 3.09.15.2 2.11 3.22 5.1 4.51.71.31 1.27.49 1.7.63.71.23 1.36.2 1.87.12.57-.09 1.76-.72 2.01-1.42.25-.7.25-1.3.17-1.42-.08-.12-.27-.2-.57-.35z"/></svg>' +
        "</button>" +
        '<button type="button" class="blog-share-btn blog-share-btn--fb" data-share="fb" aria-label="Share Facebook">' +
        '<svg class="blog-share-btn-ico" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M22 12a10 10 0 1 0-11.56 9.87v-6.99H7.9V12h2.54V9.8c0-2.5 1.49-3.88 3.77-3.88 1.09 0 2.23.2 2.23.2v2.46h-1.25c-1.23 0-1.61.76-1.61 1.54V12h2.74l-.44 2.88h-2.3v6.99A10 10 0 0 0 22 12z"/></svg>' +
        "</button>" +
        '<button type="button" class="blog-share-btn blog-share-btn--copy" data-share="copy" aria-label="Copy link">' +
        '<svg class="blog-share-btn-ico" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M16 1H6c-1.1 0-2 .9-2 2v12h2V3h10V1zm3 4H10c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h9c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2zm0 16H10V7h9v14z"/></svg>' +
        "</button>" +
        '<button type="button" class="blog-share-btn blog-share-btn--more" data-share="more" aria-label="Menu share lainnya">' +
        '<svg class="blog-share-btn-ico" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M5 10.5a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3zm7 0a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3zm7 0a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3z"/></svg>' +
        "</button>" +
        "</div>" +
        "</div>";

      var authorCardHtml =
        '<section class="blog-article-author-card" aria-label="Tentang penulis">' +
        (authorImgSrc
          ? '<img class="blog-article-author-card-img" src="' +
            escapeHtml(authorImgSrc) +
            '" alt="' +
            escapeHtml(authorName) +
            '" loading="lazy" decoding="async" />'
          : '<span class="blog-article-author-card-img blog-article-author-card-img--fallback" aria-hidden="true">' +
            escapeHtml(avatarLetter) +
            "</span>") +
        '<div class="blog-article-author-card-main">' +
        '<h2 class="blog-article-author-card-name">' +
        '<a href="' +
        escapeHtml(authorPublicPath(p.author || null)) +
        '">' +
        escapeHtml(authorName) +
        "</a>" +
        "</h2>" +
        '<p class="blog-article-author-card-bio">' +
        escapeHtml(
          (p.author && p.author.profile ? p.author.profile : "").trim() ||
            "Belum ada deskripsi profil."
        ) +
        "</p>" +
        "</div>" +
        "</section>";

      var heroSrc = postHeroSrc(p);
      var heroHtml = "";
      if (heroSrc) {
        var heroCaption = buildHeroCaptionInnerHTML(p, p.title || "");
        heroHtml =
          '<figure class="blog-article-hero' +
          (isNeo ? " blog-article-hero--neo" : "") +
          '">' +
          '<img src="' +
          escapeHtml(heroSrc) +
          '" alt="' +
          escapeHtml(p.title || "") +
          '" loading="eager" decoding="async" />' +
          (heroCaption
            ? '<figcaption class="blog-article-hero-caption">' +
              heroCaption +
              "</figcaption>"
            : "") +
          "</figure>";
      }

      var sourceHtml = "";
      var sourceUrl = (p.source || "").trim();
      if (sourceUrl) {
        var sourceLabel = sourceUrl;
        try {
          sourceLabel = new URL(sourceUrl).host || sourceUrl;
        } catch (e) {
          // keep raw string
        }
        sourceHtml =
          '<p class="blog-article-source"><strong>Sumber:</strong> ' +
          '<a href="' +
          escapeHtml(sourceUrl) +
          '" target="_blank" rel="nofollow noopener noreferrer">' +
          escapeHtml(sourceLabel) +
          "</a></p>";
      }

      if (isNeo) {
        article.innerHTML =
          buildNeoArticleTopHTML(p, breadcrumbHtml, heroHtml, {
            catName: catName,
            catPath: catPath,
            metaLine: metaLine,
            viewCount: viewCountNeo,
            authorName: authorName,
            authorImgSrc: authorImgSrc,
            avatarLetter: avatarLetter,
            authorPath: authorPublicPath(p.author || null),
            timeIso: pti || "",
            draftBadge: draftBadge,
          }) +
          ARTICLE_FONTSIZE_HTML +
          '<div class="blog-article-toc" id="blog-article-toc" hidden></div>' +
          '<div class="blog-article-body post-body"></div>' +
          sourceHtml +
          authorCardHtml;
      } else {
        article.innerHTML =
          '<header class="blog-article-head">' +
          breadcrumbHtml +
          "<h1>" +
          escapeHtml(p.title) +
          draftBadge +
          "</h1>" +
          bylineHtml +
          "</header>" +
          heroHtml +
          '<div class="blog-article-fontsize" aria-label="Pengaturan ukuran teks">' +
          '<span class="blog-article-fontsize-a blog-article-fontsize-a--sm" aria-hidden="true">A</span>' +
          '<input id="blog-fontsize-range" class="blog-article-fontsize-range" type="range" min="14" max="24" step="1" value="17" aria-label="Atur ukuran teks artikel" />' +
          '<span class="blog-article-fontsize-a blog-article-fontsize-a--lg" aria-hidden="true">A</span>' +
          "</div>" +
          '<div class="blog-article-toc" id="blog-article-toc" hidden></div>' +
          '<div class="blog-article-body post-body"></div>' +
          sourceHtml +
          authorCardHtml;
      }

      var bodyEl = article.querySelector(".blog-article-body");
      bodyEl.innerHTML = stripLeadingDuplicateTitleHtml(
        p.content || "",
        p.title || ""
      );
      if (
        window.PublicSiteAds &&
        typeof window.PublicSiteAds.activateSlotsIn === "function"
      ) {
        window.PublicSiteAds.activateSlotsIn(article);
      } else if (
        window.PublicSiteAds &&
        typeof window.PublicSiteAds.pushUnhandledIn === "function"
      ) {
        window.PublicSiteAds.pushUnhandledIn(article);
      }
      applyExternalAnchorNofollowNewTab(bodyEl);

      attachArticleFontSize(article, bodyEl);
      attachArticleShare(article, shareURL, shareTitle);

      buildArticleTOC(article, bodyEl);
      if (typeof requestAnimationFrame !== "undefined") {
        requestAnimationFrame(function () {
          scrollToArticleHashIfPresent("auto");
        });
      }
      scheduleAfterPaintIdle(function () {
        if (!publicAlsoReadEnabled()) return;
        if (bodyEl.querySelector("aside.blog-also-read")) {
          return;
        }
        injectAlsoReadEvery3Paragraphs(bodyEl, relatedPosts);
        try {
          var pa2 = window.PublicSiteAds;
          if (pa2 && typeof pa2.activateSlotsIn === "function") {
            pa2.activateSlotsIn(article);
          } else if (pa2 && typeof pa2.pushUnhandledIn === "function") {
            pa2.pushUnhandledIn(article);
          }
        } catch (eAlsoReadAds) {}
      });
      bindResponsiveArticleImages(article);
      var heroImg = article.querySelector(".blog-article-hero img");
      if (heroImg) {
        heroImg.addEventListener(
          "error",
          function onHeroErr() {
            heroImg.removeEventListener("error", onHeroErr);
            var fig = heroImg.closest(".blog-article-hero");
            if (fig) fig.remove();
          },
          false
        );
      }
      wrap.appendChild(article);
      root.replaceChildren(wrap);
      requestPopularPlacement("post");
    } catch (err) {
      root.innerHTML =
        '<div class="notice notice-error">' +
        escapeHtml(err.message || "Artikel tidak ditemukan.") +
        '</div><p class="back-link"><a href="/">← Kembali ke beranda</a></p>';
    }
  }

  function renderLogin(root) {
    setMetaTitle("Login" + browserTitleSuffix());
    root.innerHTML =
      '<div class="card card-narrow">' +
      "<h1>Masuk</h1>" +
      '<p class="muted">Login untuk mengelola konten (token dipakai di <a href="/admin/dashboard">Dashboard</a>). Membaca artikel di beranda tidak perlu login.</p>' +
      '<form id="login-form" class="stack">' +
      '<label>Email <input type="email" name="email" required autocomplete="username"></label>' +
      '<label>Password <input type="password" name="password" required autocomplete="current-password"></label>' +
      '<button type="submit" class="btn btn-primary">Login</button>' +
      "</form>" +
      '<p class="muted"><a href="/">← Kembali</a></p>' +
      "</div>";

    document
      .getElementById("login-form")
      .addEventListener("submit", async function (e) {
        e.preventDefault();
        var fd = new FormData(e.target);
        var email = fd.get("email");
        var password = fd.get("password");
        try {
          var data = await api.login(email, password);
          if (data.token) api.setSession(data.token, data.user);
          renderNav();
          if (history.replaceState) {
            history.replaceState(null, "", location.pathname + location.search);
          }
          if (location.hash) location.hash = "";
          render();
        } catch (err) {
          alert(err.message || "Login gagal");
        }
      });
  }

  function scrollToTopInstant() {
    try {
      window.scrollTo(0, 0);
    } catch (e) {}
  }

  var lastNavKey = null;
  function render() {
    var root = document.getElementById("public-root");
    if (!root) return;

    var navKey = (location.pathname || "") + (location.search || "");
    // Legacy hash routes (#/post/123, #/login) should also trigger scroll reset.
    if (location.hash && location.hash.indexOf("#/") === 0) {
      navKey += location.hash;
    }
    if (lastNavKey !== navKey) {
      scrollToTopInstant();
      lastNavKey = navKey;
    }

    var route = parseRoute();
    var hq = getQuery();
    var homeLayout = (root.getAttribute("data-home-layout") || "").trim();

    var keepHomeSSR =
      route.name === "home" &&
      root.getAttribute("data-home-ssr") === "1" &&
      !hq.category &&
      !hq.q &&
      !hq.tag;

    ensurePopularDetachedFromRoot(keepHomeSSR);

    var keepContentUntilReady =
      route.name === "segment" || route.name === "post";
    if (!keepHomeSSR && !keepContentUntilReady) {
      root.innerHTML = "";
    }

    if (
      route.name !== "home" &&
      route.name !== "posts" &&
      route.name !== "post" &&
      route.name !== "segment"
    ) {
      setPopularVisible(false);
    }

    if (route.name === "home") {
      if (
        homeLayout === "detik" &&
        !hq.category &&
        !hq.q &&
        !hq.tag &&
        !hq.author &&
        root.getAttribute("data-home-ssr") !== "1"
      ) {
        var alreadyCanonicalHome =
          location.pathname === "/" && !location.search && !location.hash;
        if (!alreadyCanonicalHome) {
          location.assign("/");
          return;
        }
        // Hindari loop reload pada detik layout saat SSR home tidak tersedia.
        // Jika sudah di "/", biarkan renderHome berjalan normal.
        renderHome(root);
        return;
      }
      if (hq.category && !hq.q && !hq.tag && history.replaceState) {
        history.replaceState(null, "", "/" + encodeURIComponent(hq.category));
        renderNav();
        render();
        return;
      }
      setMetaTitle(initialMetaTitle);
      if (keepHomeSSR) {
        hydrateHomeSSR(root);
        return;
      }
      renderHome(root);
      return;
    }
    if (route.name === "segment") {
      renderSegment(root, route.slug);
      return;
    }
    if (route.name === "posts") {
      setMetaTitle("Semua postingan" + browserTitleSuffix());
      renderPostsList(root, route);
      return;
    }
    if (route.name === "post") {
      renderPost(root, route);
      return;
    }
    if (route.name === "login") {
      renderLogin(root);
    }
  }

  document.addEventListener("click", function (e) {
    var a = e.target && e.target.closest ? e.target.closest("a") : null;
    if (!a || a.getAttribute("target") === "_blank") return;
    var hrefAttr = a.getAttribute("href");
    if (!hrefAttr) return;
    var url;
    try {
      url = new URL(a.href);
    } catch (err) {
      return;
    }
    if (url.origin !== location.origin) return;

    /* Halaman statis (footer, legal, dll.) tidak punya #public-root — biarkan navigasi penuh (location). */
    if (!document.getElementById("public-root")) {
      return;
    }

    /* TOC / anchor #id: href="#..." selalu di halaman ini; hindari banding pathname yang rawan salah. */
    var hrefTrim = (hrefAttr || "").trim();
    var frag = "";
    if (hrefTrim.charAt(0) === "#" && hrefTrim.length > 1) {
      frag = hrefTrim.slice(1);
    } else if (url.hash && url.hash.length > 1) {
      if (
        normalizePathname(url.pathname) ===
          normalizePathname(location.pathname) &&
        String(url.search || "") === String(location.search || "")
      ) {
        frag = url.hash.replace(/^#/, "");
      }
    }
    if (frag) {
      var targetEl = findElForHashId(frag);
      if (targetEl) {
        e.preventDefault();
        targetEl.scrollIntoView({ behavior: "smooth", block: "start" });
        if (history.replaceState) {
          var bar =
            hrefTrim.charAt(0) === "#" ? hrefTrim : url.hash || "#" + frag;
          history.replaceState(
            null,
            "",
            location.pathname + location.search + bar
          );
        }
        return;
      }
    }

    /* Beranda + query (?category=, ?q=): jangan anggap semua "/" sebagai home kosong — itu memutus filter kategori. */
    if (url.pathname === "/") {
      e.preventDefault();
      var next = url.pathname + url.search;
      if (history.pushState) history.pushState(null, "", next);
      else {
        location.href = next;
        return;
      }
      if (!url.search && location.hash) location.hash = "";
      renderNav();
      render();
      return;
    }
    if (/^\/post\//.test(url.pathname)) {
      e.preventDefault();
      var next = url.pathname + url.search;
      if (history.pushState) history.pushState(null, "", next);
      else location.href = next;
      renderNav();
      render();
      return;
    }
    if (url.pathname === "/posts") {
      e.preventDefault();
      var nextPosts = url.pathname + url.search;
      if (history.pushState) history.pushState(null, "", nextPosts);
      else location.href = nextPosts;
      renderNav();
      render();
    }
  });

  window.TirtayasaPublicRefresh = function () {
    renderNav();
    render();
  };

  window.addEventListener("hashchange", function () {
    // Ignore in-page anchors (TOC) like #heading-id; only handle legacy hash routes.
    if (location.hash && location.hash.indexOf("#/") === 0) {
      renderNav();
      render();
    }
  });
  window.addEventListener("popstate", function () {
    renderNav();
    render();
  });
  document.addEventListener("DOMContentLoaded", function () {
    renderNav();
    render();
  });
})();
