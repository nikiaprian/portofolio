(function () {
  "use strict";

  var DEFAULT_IMG_RESIZE_QUALITY = "82";

  function confTruthy(val) {
    return val === true || val === "true" || val === 1 || val === "1";
  }

  function selfHostedImgResizeEnabled() {
    try {
      var cfg = window.__CLIENT_PUB_CONF__ || {};
      return confTruthy(cfg.imgResizeEnabled);
    } catch (e) {
      return false;
    }
  }

  function cloudflareImageResizeEnabled() {
    try {
      var cfg = window.__CLIENT_PUB_CONF__ || {};
      if (cfg.imageResizeEnabled === false || cfg.imageResizeEnabled === "false") {
        return false;
      }
      return true;
    } catch (e) {
      return true;
    }
  }

  function readCDNImageResizeOption(raw, key) {
    var s = String(raw || "").trim();
    if (!s) return "";
    var want = String(key || "").toLowerCase().trim();
    var marker = "/cdn-cgi/image/";
    var low = s.toLowerCase();
    var idx = low.indexOf(marker);
    if (idx < 0) return "";
    var rest = s.slice(idx + marker.length);
    var slash = rest.indexOf("/");
    if (slash < 0) return "";
    var opts = rest.slice(0, slash).split(",");
    for (var i = 0; i < opts.length; i++) {
      var pair = String(opts[i] || "").split("=");
      if (pair.length < 2) continue;
      if (String(pair[0] || "").toLowerCase().trim() !== want) continue;
      var val = String(pair.slice(1).join("=") || "").trim();
      if (val) return val;
    }
    return "";
  }

  function buildCloudflareResizedSrc(raw, width, quality) {
    if (!raw) return "";
    var w = Math.max(1, Math.ceil(Number(width) || 0));
    if (!(w > 0)) return raw;
    var opts = ["width=" + String(w)];
    if (/^\d+$/.test(String(quality || "")) && Number(quality) > 0) {
      opts.push("quality=" + quality);
    }
    if (/^https?:\/\//i.test(raw)) {
      try {
        var u = new URL(raw);
        if (u.pathname) {
          u.pathname = "/cdn-cgi/image/" + opts.join(",") + u.pathname;
          return u.toString();
        }
      } catch (e) {}
      return raw;
    }
    var path = raw.charAt(0) === "/" ? raw : "/" + raw;
    return "/cdn-cgi/image/" + opts.join(",") + path;
  }

  function imageResizeProxyPath() {
    try {
      var p = (window.__CLIENT_PUB_CONF__ || {}).imageResizePath;
      if (p && String(p).trim()) return String(p).trim();
    } catch (e) {}
    return "/img";
  }

  function imgResizePublicBase() {
    try {
      var base = String((window.__CLIENT_PUB_CONF__ || {}).publicBase || "").trim();
      if (base) return base.replace(/\/$/, "");
    } catch (e) {}
    if (typeof location !== "undefined" && location.origin) {
      return String(location.origin).replace(/\/$/, "");
    }
    return "";
  }

  function absolutizeImgResizeProxyURL(path) {
    path = String(path || "").trim();
    if (!path || /^https?:\/\//i.test(path)) return path;
    var base = imgResizePublicBase();
    if (!base) return path;
    return base + (path.charAt(0) === "/" ? path : "/" + path);
  }

  function isLocalStaticPath(path) {
    path = String(path || "");
    return (
      path.indexOf("/static/uploads/") === 0 ||
      path.indexOf("/static/images/") === 0
    );
  }

  function localStaticPathFromAbsoluteURL(src) {
    if (!/^https?:\/\//i.test(String(src || ""))) return "";
    try {
      var p = new URL(src).pathname || "";
      return isLocalStaticPath(p) ? p : "";
    } catch (e) {
      return "";
    }
  }

  function configuredImgResizeQuality() {
    try {
      var cfg = window.__CLIENT_PUB_CONF__ || {};
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
    q = readResizeQueryOption(String(rawSrc || ""), "q");
    if (/^\d+$/.test(q) && Number(q) > 0 && Number(q) <= 100) return q;
    q = readCDNImageResizeOption(String(rawSrc || ""), "quality");
    if (/^\d+$/.test(q) && Number(q) > 0 && Number(q) <= 100) return q;
    return DEFAULT_IMG_RESIZE_QUALITY;
  }

  function configuredImgResizeMaxWidth() {
    try {
      var cfg = window.__CLIENT_PUB_CONF__ || {};
      return String(cfg.imgResizeWidth || "").trim();
    } catch (e) {
      return "";
    }
  }

  function configuredImgResizeMaxHeight() {
    try {
      var cfg = window.__CLIENT_PUB_CONF__ || {};
      return String(cfg.imgResizeHeight || "").trim();
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

  function stripStaticResizeQuery(path) {
    var s = String(path || "").trim();
    if (!s) return s;
    var base = s.split("?")[0].split("#")[0];
    if (
      base.indexOf("/static/uploads/") !== 0 &&
      base.indexOf("/static/images/") !== 0
    ) {
      return s;
    }
    try {
      var u = new URL(s, location.origin);
      if (!u.searchParams.has("w") && !u.searchParams.has("h") && !u.searchParams.has("q")) {
        return s;
      }
      return u.pathname;
    } catch (e) {
      return base;
    }
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
    if (idx < 0) return stripStaticResizeQuery(s);
    var rest = s.slice(idx + marker.length);
    var slash = rest.indexOf("/");
    if (slash < 0) return s;
    return stripStaticResizeQuery(s.slice(0, idx) + rest.slice(slash));
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

  function normalizeImgSrcWithConfigInner(src, width, quality, height) {
    if (!src) return src;
    if (/^data:/i.test(String(src))) return src;
    var keepAbsolute = /^https?:\/\//i.test(String(src || ""));
    var out = "";
    if (/^https?:\/\//i.test(src)) {
      var localPath = localStaticPathFromAbsoluteURL(src);
      out = localPath || src;
    } else if (src.charAt(0) === "/") out = src;
    else out = "/" + src;
    if (!selfHostedImgResizeEnabled()) {
      return stripImageResizePrefix(out);
    }
    height = String(height || "").trim();
    width = clampImgResizeWidth(String(width || "").trim());
    height = clampImgResizeHeight(height);
    if (!width && !height && !quality) {
      return keepAbsolute && isLocalStaticPath(out)
        ? absolutizeImgResizeProxyURL(out)
        : out;
    }
    quality = resolveImgResizeQuality(quality, src);
    out = stripImageResizePrefix(out);
    var localFromOut = localStaticPathFromAbsoluteURL(out);
    if (localFromOut) {
      out = localFromOut;
      keepAbsolute = true;
    }
    if (out.indexOf("?") >= 0) out = out.split("?")[0].split("#")[0];
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
        return absolutizeImgResizeProxyURL(
          imageResizeProxyPath() + "/" + esegs.join("/") + "?" + params.join("&")
        );
      } catch (eUrl) {
        return out;
      }
    }
    var rel =
      out.indexOf("/static/") === 0 ? out.slice("/static/".length) : "";
    if (!rel) return out;
    var resized = out + "?" + params.join("&");
    return keepAbsolute ? absolutizeImgResizeProxyURL(resized) : resized;
  }

  function normalizeImgSrcWithConfig(src, width, quality, height) {
    return normalizeImgSrcWithConfigInner(src, width, quality, height);
  }

  function readResizeQueryOption(raw, key) {
    var s = String(raw || "").trim();
    if (!s) return "";
    var want = String(key || "").toLowerCase().trim();
    var proxy = imageResizeProxyPath().toLowerCase();
    var low = s.toLowerCase();
    var readFromURL = function (u) {
      if (want === "width" || want === "w")
        return (u.searchParams.get("w") || "").trim();
      if (want === "height" || want === "h")
        return (u.searchParams.get("h") || "").trim();
      if (want === "quality" || want === "q")
        return (u.searchParams.get("q") || "").trim();
      return "";
    };
    try {
      var uAny = new URL(s, location.origin);
      var pn = (uAny.pathname || "").toLowerCase();
      if (
        pn.indexOf("/static/uploads/") === 0 ||
        pn.indexOf("/static/images/") === 0 ||
        pn.indexOf(proxy + "/") === 0 ||
        pn === proxy ||
        low.indexOf(proxy + "?") === 0
      ) {
        var got = readFromURL(uAny);
        if (got) return got;
      }
    } catch (eAny) {}
    if (
      low.indexOf(proxy + "/") === 0 ||
      low.indexOf(proxy + "?") === 0
    ) {
      try {
        var u = new URL(s, location.origin);
        return readFromURL(u);
      } catch (e) {}
    }
    if (/^https?:\/\//i.test(s)) {
      try {
        var absU = new URL(s);
        var pn2 = (absU.pathname || "").toLowerCase();
        if (
          pn2.indexOf("/static/uploads/") === 0 ||
          pn2.indexOf("/static/images/") === 0 ||
          pn2.indexOf(proxy + "/") === 0 ||
          pn2 === proxy
        ) {
          return readFromURL(absU);
        }
      } catch (eAbs) {}
    }
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

  // Pixel size of the image slot as rendered (img rect, then container, then fallbacks).
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

  function bindResponsiveImg(img, container, fallbackWidth, widthMultiplier, fallbackHeight) {
    if (!img || img.getAttribute("data-img-resize-bound") === "1") return;
    img.setAttribute("data-img-resize-bound", "1");
    var raw = img.__cdnOriginalSrc || "";
    if (!raw) {
      var currentSrc = img.getAttribute("src") || "";
      var existingQuality = readResizeQueryOption(currentSrc, "quality");
      if (existingQuality) img.__cdnOriginalQuality = existingQuality;
      raw = stripImageResizePrefix(currentSrc);
      img.__cdnOriginalSrc = raw;
    }
    if (!raw) return;
    var currentSrc = img.getAttribute("src") || "";
    var fallback = String(fallbackWidth || "").trim();
    var multiplier = Number(widthMultiplier);
    if (!(multiplier > 0)) multiplier = 1;

    function update() {
      var liveSrc = img.getAttribute("src") || "";
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
          liveSrc
        ),
        dims.height
      );
      if (next && next !== liveSrc) {
        img.setAttribute("src", next);
      }
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
    var root = scope || document;
    root.querySelectorAll(".blog-posts-item").forEach(function (a) {
      var ptw = a.querySelector(".blog-posts-thumb");
      var img = ptw && ptw.querySelector("img");
      bindResponsiveImg(img, ptw, "", 1, "");
    });
    root.querySelectorAll(".blog-popular-thumb-ring img").forEach(function (img) {
      bindResponsiveImg(img, img.closest(".blog-popular-thumb-ring"), "", 1, "");
    });
    root.querySelectorAll(".blog-latest-thumb img").forEach(function (img) {
      bindResponsiveImg(img, img.closest(".blog-latest-thumb"), "", 1, "");
    });
    root.querySelectorAll(".blog-latest-stack-thumb img").forEach(function (img) {
      bindResponsiveImg(img, img.closest(".blog-latest-stack-thumb"), "", 1, "");
    });
    root.querySelectorAll(".blog-feature-slide-media img").forEach(function (img) {
      bindResponsiveImg(img, img.closest(".blog-feature-slide-media"), "", 1, "");
    });
    root.querySelectorAll(".blog-mostviewed-media img").forEach(function (img) {
      bindResponsiveImg(img, img.closest(".blog-mostviewed-media"), "", 1, "");
    });
    root.querySelectorAll(".neo-mag-box-media img").forEach(function (img) {
      bindResponsiveImg(img, img.closest(".neo-mag-box-media"), "", 1, "");
    });
    root.querySelectorAll(".neo-mag-stack-item img").forEach(function (img) {
      bindResponsiveImg(img, img.closest(".neo-mag-stack-item"), "", 1, "");
    });
    root.querySelectorAll(".neo-mag-box-thumb img").forEach(function (img) {
      bindResponsiveImg(img, img.closest(".neo-mag-box-thumb"), "", 1, "");
    });
    root.querySelectorAll(".neo-mag-list-thumb-hit img").forEach(function (img) {
      bindResponsiveImg(img, img.closest(".neo-mag-list-thumb-hit"), "", 1, "");
    });
    root.querySelectorAll(".neo-mag-mp-thumb-ring img").forEach(function (img) {
      bindResponsiveImg(img, img.closest(".neo-mag-mp-thumb-ring"), "", 1, "");
    });
    root.querySelectorAll(".neo-mag-feed-thumb img").forEach(function (img) {
      bindResponsiveImg(img, img.closest(".neo-mag-feed-thumb"), "", 1, "");
    });
    root.querySelectorAll(".blog-picks-grid-media img").forEach(function (img) {
      bindResponsiveImg(img, img.closest(".blog-picks-grid-media"), "", 1, "");
    });
    root.querySelectorAll(".blog-article-hero img").forEach(function (img) {
      bindResponsiveImg(img, img.closest(".blog-article-hero"), "", 1, "");
    });
    root.querySelectorAll(".home-category-strip-media img").forEach(function (img) {
      bindResponsiveImg(img, img.closest(".home-category-strip-media"), "", 1, "");
    });
    root.querySelectorAll(".blog-article-body img[src]").forEach(function (img) {
      var body = img.closest(".blog-article-body");
      bindResponsiveImg(img, body || img.parentElement, "", 1, "");
    });
  }

  window.ClientBindPageImg = bindPageResponsiveImages;

  function run() {
    bindPageResponsiveImages(document);
  }
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", run);
  } else {
    run();
  }
})();
