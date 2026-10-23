/**
 * Halaman artikel SSR (post.html) — hidrasi sidebar Populer/Terkini, share, TOC, font size,
 * blok "Baca juga", dan section rekomendasi. Tidak memuat app.js (SPA beranda).
 */
(function () {
  "use strict";

  var api = window.TirtayasaAPI;
  if (!api) return;
  var BLANK_IMG =
    "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==";

  function escapeHtml(s) {
    if (s == null) return "";
    var d = document.createElement("div");
    d.textContent = s;
    return d.innerHTML;
  }

  function isPublished(p) {
    return (p.status || "").toLowerCase() === "published";
  }

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
      return (
        encodeURIComponent(parentSlug) +
        "/" +
        encodeURIComponent(slug)
      );
    }
    return encodeURIComponent(slug);
  }

  function postPublicPath(p) {
    if (!p || !p.slug) return "/";
    var s = String(p.slug).trim();
    if (!s) return "/";
    var mode = "";
    try {
      mode = String(
        (window.TirtayasaPublicConfig || {}).articleSlugMode || ""
      )
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
    if (/^\d+$/.test(width) && Number(width) > 0 && !height && !quality) return out;

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
        return imageResizeProxyPath() + "/" + esegs.join("/") + "?" + params.join("&");
      } catch (eUrl) {
        return out;
      }
    }

    var rel =
      out.indexOf("/static/") === 0 ? out.slice("/static/".length) : "";
    if (!rel) return out;
    var segs = rel.split("/").map(function (seg) {
      return encodeURIComponent(seg);
    });
    return imageResizeProxyPath() + "/" + segs.join("/") + "?" + params.join("&");
  }

  function isResizableAsset(src) {
    var path = stripImageResizePrefix(src);
    if (/^https?:\/\//i.test(path)) {
      try {
        return !!(new URL(path).hostname);
      } catch (e) {
        return false;
      }
    }
    return (
      path.indexOf("/static/uploads/") === 0 ||
      path.indexOf("/static/images/") === 0
    );
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
    if (/^\d+$/.test(String(fallbackHeight || "")) && Number(fallbackHeight) > 0) {
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
    if (!h && /^\d+$/.test(String(knownWidth || "")) && Number(knownWidth) > 0) {
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
      w =
        Number(
          effectiveImageWidth(box, fallbackWidth, img)
        ) || 0;
    }
    if (!(h > 0)) {
      h =
        Number(
          effectiveImageHeight(
            box,
            fallbackHeight,
            img,
            w > 0 ? String(w) : ""
          )
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
    var display = displayedBoxPixels(
      img,
      ctn,
      fallbackWidth,
      fallbackHeight
    );
    var width = "";
    var height = "";
    if (display.width > 0) {
      width = clampImgResizeWidth(
        Math.max(1, Math.ceil(display.width * mult))
      );
    }
    if (hasFixedDisplayBox(ctn, fallbackHeight) && display.height > 0) {
      height = clampImgResizeHeight(
        Math.max(1, Math.ceil(display.height * mult))
      );
    }
    return { width: width, height: height };
  }

  function bindResponsiveThumbCDN(img, container, fallbackWidth, widthMultiplier, fallbackHeight) {
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
              return (
                scheme +
                "://" +
                host +
                (rpath ? "/" + rpath : "")
              );
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

  function postThumbnailSrc(p) {
    if (!p) return BLANK_IMG;
    var u = p.image != null ? String(p.image).trim() : "";
    if (!u) u = firstImgSrc(p.content || "");
    return normalizeImgSrc(u);
  }

  function postTimeIso(p) {
    if (!p) return "";
    return (p.updated_at || p.created_at || "").toString().trim();
  }

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
      var va = a && a.view != null ? Number(a.view) : 0;
      var vb = b && b.view != null ? Number(b.view) : 0;
      if (vb !== va) return vb - va;
      var isoA = postTimeIso(a);
      var isoB = postTimeIso(b);
      var ta = isoA ? new Date(isoA).getTime() : 0;
      var tb = isoB ? new Date(isoB).getTime() : 0;
      return tb - ta;
    });
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
      fetchLimit != null && fetchLimit > 0 ? fetchLimit : publicAlsoReadRelatedFetch();
    if (cap < 1) cap = 1;
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

      if (anchor.parentNode) {
        anchor.insertAdjacentElement("afterend", block);
      } else {
        bodyEl.appendChild(block);
      }
      inserted++;
    }
    relocateMisplacedAlsoReadBlocks(bodyEl);
  }

  function bindMostViewedSlider(root) {
    if (!root) return;
    var slider = root.querySelector(".blog-mostviewed-slider");
    var track = root.querySelector(".blog-mostviewed-track");
    var prev = root.querySelector(".blog-mostviewed-nav--prev");
    var next = root.querySelector(".blog-mostviewed-nav--next");
    if (!slider || !track || !prev || !next) return;

    var items = Array.prototype.slice.call(
      track.querySelectorAll(".blog-mostviewed-item")
    );
    if (!items.length) return;

    var slides = [];
    var n = 0;
    var i = 0;
    var perPage = 3;

    function getPerPage() {
      return window.matchMedia("(max-width: 768px)").matches ? 2 : 3;
    }

    function clearTrack() {
      while (track.firstChild) track.removeChild(track.firstChild);
    }

    function rebuildPages() {
      perPage = getPerPage();
      clearTrack();
      slides = [];
      for (var j = 0; j < items.length; j += perPage) {
        var page = document.createElement("div");
        page.className = "blog-mostviewed-page blog-mostviewed-slide";
        page.setAttribute("aria-hidden", "true");
        items.slice(j, j + perPage).forEach(function (it) {
          page.appendChild(it);
        });
        slides.push(page);
        track.appendChild(page);
      }
      n = slides.length;
      i = Math.max(0, Math.min(i, n - 1));
      apply();
    }

    function apply() {
      track.style.transform = "translateX(-" + i * 100 + "%)";
      prev.disabled = i <= 0;
      next.disabled = i >= n - 1;
      slides.forEach(function (slide, idx) {
        slide.setAttribute("aria-hidden", idx === i ? "false" : "true");
      });
    }

    function go(delta) {
      i = Math.max(0, Math.min(n - 1, i + delta));
      apply();
    }

    prev.addEventListener("click", function (e) {
      e.preventDefault();
      e.stopPropagation();
      go(-1);
    });
    next.addEventListener("click", function (e) {
      e.preventDefault();
      e.stopPropagation();
      go(1);
    });

    var startX = 0;
    var tracking = false;
    slider.addEventListener(
      "touchstart",
      function (e) {
        if (!e.touches || !e.touches[0]) return;
        tracking = true;
        startX = e.touches[0].clientX;
      },
      { passive: true }
    );
    slider.addEventListener(
      "touchend",
      function (e) {
        if (!tracking || !e.changedTouches || !e.changedTouches[0]) return;
        tracking = false;
        var delta = e.changedTouches[0].clientX - startX;
        if (Math.abs(delta) > 48) go(delta < 0 ? 1 : -1);
      },
      { passive: true }
    );

    var resizeTimer = 0;
    window.addEventListener("resize", function () {
      clearTimeout(resizeTimer);
      resizeTimer = setTimeout(function () {
        if (perPage !== getPerPage()) rebuildPages();
      }, 120);
    });

    rebuildPages();
  }

  function initArticleRelatedLazyList(section) {
    if (!section || section.getAttribute("data-lazy-bound") === "1") return;
    var list = section.querySelector("[data-lazy-list]");
    var loadBtn = section.querySelector("[data-lazy-loadmore]");
    if (!list || !loadBtn) return;
    section.setAttribute("data-lazy-bound", "1");
    var batch = parseInt(list.getAttribute("data-batch-size") || "5", 10);
    if (!batch || batch < 1) batch = 5;
    var hiddenSelector = "[data-lazy-item].is-lazy-hidden";
    loadBtn.addEventListener("click", function () {
      var hidden = list.querySelectorAll(hiddenSelector);
      if (!hidden.length) {
        var wrap = loadBtn.closest(".blog-feed-loadmore");
        if (wrap) wrap.remove();
        return;
      }
      for (var i = 0; i < hidden.length && i < batch; i++) {
        hidden[i].hidden = false;
        hidden[i].classList.remove("is-lazy-hidden");
      }
      if (!list.querySelector(hiddenSelector)) {
        var wrap2 = loadBtn.closest(".blog-feed-loadmore");
        if (wrap2) wrap2.remove();
      }
    });
  }

  function bindArticleMoreSectionsSSR(root) {
    if (!root) return;
    root.querySelectorAll(".blog-mostviewed").forEach(function (sec) {
      if (sec.getAttribute("data-slider-bound") === "1") return;
      sec.setAttribute("data-slider-bound", "1");
      bindMostViewedSlider(sec);
    });
    root.querySelectorAll(".blog-otherlatest").forEach(initArticleRelatedLazyList);
    root.querySelectorAll(".blog-otherlatest .blog-posts-item").forEach(function (a) {
      var ptw = a.querySelector(".blog-posts-thumb");
      var ptim = ptw && ptw.querySelector("img");
      bindResponsiveThumbCDN(ptim, ptw, "", 1, "");
    });
    root.querySelectorAll(".blog-mostviewed-media img").forEach(function (im) {
      bindResponsiveThumbCDN(im, im.closest(".blog-mostviewed-media"));
    });
  }

  function setPopularVisible(show) {
    var wrap = document.getElementById("blog-popular-wrap");
    if (!wrap) return;
    wrap.hidden = !show;
    if (!show) wrap.style.minHeight = "";
  }

  function canonicalOrigin() {
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
        copyToClipboard((opts.title ? opts.title + " " : "") + opts.url).then(
          function () {}
        );
      }
      var shareURL = buildShareURL(kind, opts.url, opts.title);
      if (shareURL) openSharePopup(shareURL);
    });

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

    var isMobile = false;
    try {
      isMobile = !!(
        window.matchMedia && window.matchMedia("(max-width: 899px)").matches
      );
    } catch (e) {}

    if (!isMobile) {
      var layout = root.parentNode;
      if (!layout) return;
      if (wrap.parentNode === layout && root.nextSibling === wrap) return;
      insertAfter(layout, wrap, root);
      return;
    }

    if (routeName === "home" || routeName === "posts") {
      var layoutHome = root.parentNode;
      if (layoutHome) insertAfter(layoutHome, wrap, root);
      return;
    }

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

  function ensurePopularDetachedFromRoot() {
    var wrap = document.getElementById("blog-popular-wrap");
    if (!wrap) return;
    var root = document.getElementById("public-root");
    if (!root) return;
    if (wrap.parentNode !== root) return;

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

  function renderPopularSidebar(payload) {
    var wrap = document.getElementById("blog-popular-wrap");
    var ol = document.getElementById("blog-popular-list");
    var olLatest = document.getElementById("blog-latest-list");
    if (!wrap || !ol || !olLatest) return;

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

    function renderList(targetOl, posts) {
      if (!targetOl) return;
      targetOl.classList.remove("blog-popular-list--skeleton");
      targetOl.innerHTML = (posts || [])
        .slice(0, 5)
        .map(function (p) {
          var cat = p.category && p.category.name ? p.category.name : "";
          var rel = relativeTimeId(postTimeIso(p));
          var href = postPublicPath(p);
          if (neoMag) {
            var thumb = escapeHtml(postThumbnailSrc(p));
            var meta = "";
            if (rel) {
              meta +=
                '<span class="blog-popular-time">' + escapeHtml(rel) + "</span>";
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
    renderList(ol, popularPosts);
    if (neoMag && neoLatestSection) {
      neoLatestSection.innerHTML =
        '<h3 class="neo-mag-rail-heading">Terkini</h3>' +
        renderNeoTerkiniRailCards(latestPosts);
      olLatest.innerHTML = "";
      olLatest.classList.remove("blog-popular-list--skeleton");
      if (typeof window.TirtayasaBindPageImages === "function") {
        window.TirtayasaBindPageImages(wrap);
      }
    } else {
      renderList(olLatest, latestPosts);
    }
  }

  var popularCache = { at: 0, posts: null };
  var latestCache = { at: 0, posts: null };

  function queuePopularSidebarRender(payload) {
    requestAnimationFrame(function () {
      requestAnimationFrame(function () {
        renderPopularSidebar(payload);
      });
    });
  }

  function sidebarListLimitFromDataAttr(dataAttr, metaName) {
    var wrap = document.getElementById("blog-popular-wrap");
    if (wrap) {
      var fromWrap = parseInt(wrap.getAttribute(dataAttr), 10);
      if (!isNaN(fromWrap) && fromWrap > 0) {
        return Math.min(20, fromWrap);
      }
      var legacyWrap = parseInt(wrap.getAttribute("data-sidebar-list-limit"), 10);
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
    var legacyMeta = document.querySelector('meta[name="tirtayasa-sidebar-list-limit"]');
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

  async function ensurePopularSidebarLoaded() {
    var wrap = document.getElementById("blog-popular-wrap");
    var ol = document.getElementById("blog-popular-list");
    var olLatest = document.getElementById("blog-latest-list");
    if (!wrap || !ol || !olLatest) return;

    /* Sudah diisi server (seperti kategori/tag): jangan fetch ulang — hindari kedip/refresh DOM. */
    var skel =
      ol.classList.contains("blog-popular-list--skeleton") ||
      olLatest.classList.contains("blog-popular-list--skeleton");
    var hasNeoTerkini =
      wrap.classList.contains("blog-popular-wrap--neo-mag-home-only-latest") &&
      wrap.querySelector(".neo-mag-rail-section--latest .neo-mag-mp-card");
    var hasItems =
      hasNeoTerkini ||
      (ol.querySelector(".blog-popular-item") &&
        olLatest.querySelector(".blog-popular-item"));
    if (hasItems && !skel) {
      wrap.setAttribute("aria-busy", "false");
      wrap.hidden = false;
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
              .getPublic("/posts/popular?hours=24&limit=" + sidebarListLimitPopular())
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
          : api.getPublic("/posts?page=1&size=" + sidebarListLimitLatest()).then(function (raw2) {
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
      queuePopularSidebarRender({ popular: [], latest: [] });
    }
  }

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
    var decBtn = article.querySelector("#blog-fontsize-dec");
    var incBtn = article.querySelector("#blog-fontsize-inc");
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
      /* Neo memakai calc(var(--content-font-size) * 1.03); cukup ubah variabel. */
      bodyEl.style.removeProperty("font-size");
    };

    input.addEventListener("input", apply, { passive: true });
    input.addEventListener("change", apply, { passive: true });

    function stepFont(delta) {
      var n = parseInt(input.value || "17", 10);
      if (isNaN(n)) n = 17;
      n += delta;
      if (n < 12) n = 12;
      if (n > 28) n = 28;
      input.value = String(n);
      apply();
    }
    if (decBtn) {
      decBtn.addEventListener("click", function () {
        stepFont(-1);
      });
    }
    if (incBtn) {
      incBtn.addEventListener("click", function () {
        stepFont(1);
      });
    }
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

    // Neo layout: place TOC right before the first H2 in article body.
    try {
      var isNeo = document.body && document.body.classList.contains("site-layout-neo");
      if (isNeo && bodyEl) {
        var firstH2 = bodyEl.querySelector("h2");
        if (firstH2 && firstH2.parentNode) {
          firstH2.parentNode.insertBefore(toc, firstH2);
        }
      }
    } catch (e) {}

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

  async function hydrateArticlePage(root, slug) {
    if (root.getAttribute("data-article-more-ssr") === "1") {
      bindArticleMoreSectionsSSR(root);
    }
    if (root.getAttribute("data-article-hydrated") === "1") {
      ensurePopularSidebarLoaded();
      requestPopularPlacement("post");
      return;
    }
    setPopularVisible(true);
    ensurePopularSidebarLoaded();
    var article = root.querySelector(".blog-article-single");
    if (!article) {
      root.setAttribute("data-article-hydrated", "1");
      return;
    }
    var h1 = article.querySelector("h1");
    var title = h1 ? (h1.textContent || "").trim() : "";
    var bodyEl = article.querySelector(".blog-article-body");
    if (bodyEl && title) {
      if (
        window.PublicSiteAds &&
        typeof window.PublicSiteAds.stripLeadingDuplicateH1Dom === "function"
      ) {
        window.PublicSiteAds.stripLeadingDuplicateH1Dom(bodyEl, title);
      } else {
        bodyEl.innerHTML = stripLeadingDuplicateTitleHtml(
          bodyEl.innerHTML,
          title
        );
      }
    }
    attachArticleFontSize(article, bodyEl);
    var pathForShare = (function () {
      try {
        var pth = (location.pathname || "").trim();
        if (pth) return pth;
      } catch (e) {}
      return "/" + encodeURIComponent(String(slug || "").trim());
    })();
    var shareURL = canonicalOrigin() + pathForShare;
    var shareTitle = title;
    attachArticleShare(article, shareURL, shareTitle);

    var hasAlsoReadInBody =
      bodyEl && !!bodyEl.querySelector("aside.blog-also-read");

    var p = null;
    if (!hasAlsoReadInBody && publicAlsoReadEnabled()) {
      try {
        p = await api.getPublic("/posts/slug/" + encodeURIComponent(slug));
      } catch (e) {
        p = null;
      }
    }

    buildArticleTOC(article, bodyEl);
    if (typeof requestAnimationFrame !== "undefined") {
      requestAnimationFrame(function () {
        scrollToArticleHashIfPresent("auto");
      });
    }
    if (!hasAlsoReadInBody && publicAlsoReadEnabled() && p && bodyEl && p.category && p.category.slug) {
      scheduleAfterPaintIdle(function () {
        getRelatedPostsByCategory(
          p.category.slug,
          p.id,
          publicAlsoReadRelatedFetch()
        ).then(function (relatedPosts) {
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
      });
    }

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

    root.setAttribute("data-article-hydrated", "1");
    requestPopularPlacement("post");
    if (root.getAttribute("data-article-more-ssr") === "1") {
      bindArticleMoreSectionsSSR(root);
    }
  }

  function bootstrapPostPage() {
    if (typeof window.TirtayasaBindPageImages === "function") {
      window.TirtayasaBindPageImages(document);
    }
    var root = document.getElementById("public-root");
    if (!root || root.getAttribute("data-ssr-article") !== "1") return;
    var dataSlug = (root.getAttribute("data-slug") || "").trim();
    if (!dataSlug) return;
    var path = location.pathname || "/";
    var routePostSlug = "";
    var mTwo = /^\/([^/]+)\/([^/]+)\/?$/.exec(path);
    if (mTwo) {
      var s1 = decodeURIComponent(mTwo[1] || "");
      var s2 = decodeURIComponent(mTwo[2] || "");
      if (
        s1 === "posts" ||
        s1 === "rss-feeds" ||
        s1 === "login" ||
        s1 === "tag" ||
        s1 === "author"
      ) {
        return;
      }
      routePostSlug = s2;
    } else {
      var mSeg = /^\/([^/]+)\/?$/.exec(path);
      if (!mSeg) return;
      routePostSlug = decodeURIComponent(mSeg[1]);
      if (
        routePostSlug === "posts" ||
        routePostSlug === "rss-feeds" ||
        routePostSlug === "login"
      ) {
        return;
      }
    }
    if (routePostSlug !== dataSlug) return;
    if (root.getAttribute("data-article-more-ssr") === "1") {
      bindArticleMoreSectionsSSR(root);
    }
    ensurePopularDetachedFromRoot();
    hydrateArticlePage(root, dataSlug).catch(function () {});
  }

  document.addEventListener("DOMContentLoaded", bootstrapPostPage);
})();
