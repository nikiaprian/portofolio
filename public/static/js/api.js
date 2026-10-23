(function () {
  "use strict";

  var PREFIX = "/api/v1";
  try {
    if (
      window.TirtayasaAdminConfig &&
      typeof window.TirtayasaAdminConfig.apiPrefix === "string" &&
      window.TirtayasaAdminConfig.apiPrefix.trim()
    ) {
      PREFIX = window.TirtayasaAdminConfig.apiPrefix.trim();
    } else if (
      window.TirtayasaPublicConfig &&
      typeof window.TirtayasaPublicConfig.apiPrefix === "string" &&
      window.TirtayasaPublicConfig.apiPrefix.trim()
    ) {
      PREFIX = window.TirtayasaPublicConfig.apiPrefix.trim();
    }
  } catch (e) {}
  var TOKEN_KEY = "tirtayasa_token";
  var USER_KEY = "tirtayasa_user";

  function getToken() {
    return sessionStorage.getItem(TOKEN_KEY);
  }

  function setSession(token, user) {
    if (token) {
      sessionStorage.setItem(TOKEN_KEY, token);
      document.cookie =
        TOKEN_KEY + "=" + encodeURIComponent(token) + "; Path=/; SameSite=Lax";
    } else {
      sessionStorage.removeItem(TOKEN_KEY);
      document.cookie = TOKEN_KEY + "=; Path=/; Max-Age=0; SameSite=Lax";
    }
    if (user) sessionStorage.setItem(USER_KEY, JSON.stringify(user));
    else sessionStorage.removeItem(USER_KEY);
  }

  function getUser() {
    try {
      var raw = sessionStorage.getItem(USER_KEY);
      return raw ? JSON.parse(raw) : null;
    } catch (e) {
      return null;
    }
  }

  function headers(jsonBody) {
    var h = { Accept: "application/json" };
    var t = getToken();
    if (t) h.Authorization = "Bearer " + t;
    if (jsonBody) h["Content-Type"] = "application/json";
    return h;
  }

  async function request(method, path, body) {
    var opts = { method: method, headers: headers(!!body) };
    if (body !== undefined && body !== null) opts.body = JSON.stringify(body);
    var res = await fetch(PREFIX + path, opts);
    var text = await res.text();
    var data = null;
    if (text) {
      try {
        data = JSON.parse(text);
      } catch (e) {
        data = { raw: text };
      }
    }
    if (!res.ok) {
      var err = new Error(
        (data && data.error) || res.statusText || "Request failed"
      );
      err.status = res.status;
      err.body = data;
      throw err;
    }
    return data;
  }

  /** GET tanpa header Authorization — untuk konten publik (mis. daftar & baca artikel di /). */
  async function requestPublic(method, path) {
    var opts = { method: method, headers: { Accept: "application/json" } };
    var res = await fetch(PREFIX + path, opts);
    var text = await res.text();
    var data = null;
    if (text) {
      try {
        data = JSON.parse(text);
      } catch (e) {
        data = { raw: text };
      }
    }
    if (!res.ok) {
      var err = new Error(
        (data && data.error) || res.statusText || "Request failed"
      );
      err.status = res.status;
      err.body = data;
      throw err;
    }
    return data;
  }

  /**
   * Hapus tepat satu <h1> di awal isi artikel jika sama dengan judul (tanpa
   * read/write innerHTML seluruh body — menghindiri skrip/iframe AdSense rusak).
   */
  function stripLeadingDuplicateH1Dom(bodyEl, plainTitle) {
    if (!bodyEl || plainTitle == null) return;
    var t = String(plainTitle).trim().toLowerCase();
    if (!t) return;
    var node = bodyEl.firstChild;
    while (node) {
      if (node.nodeType === 3) {
        if (String(node.textContent || "").trim() !== "") return;
        node = node.nextSibling;
        continue;
      }
      if (node.nodeType !== 1) return;
      if (node.tagName !== "H1") return;
      var inner = String(node.textContent || "")
        .replace(/\s+/g, " ")
        .trim()
        .toLowerCase();
      if (inner === t || inner.indexOf(t) === 0) {
        node.parentNode.removeChild(node);
      }
      return;
    }
  }

  function adSlotUsableWidth(el) {
    if (!el || el.nodeType !== 1) return 0;
    try {
      if (typeof el.getBoundingClientRect === "function") {
        var r = el.getBoundingClientRect();
        if (r.width >= 1) return r.width;
      }
    } catch (eW) {}
    try {
      var ow = el.offsetWidth;
      if (ow >= 1) return ow;
    } catch (eW2) {}
    return 0;
  }

  function ensureAdsenseSlotHasWidth(ins) {
    if (!ins || ins.nodeType !== 1) return 0;
    var w = adSlotUsableWidth(ins);
    if (w >= 1) return w;
    try {
      var p = ins.parentElement;
      for (var steps = 0; p && steps < 10; steps++) {
        var pw = adSlotUsableWidth(p);
        if (pw >= 1) {
          if (!ins.style.display) ins.style.display = "block";
          if (!ins.style.width) ins.style.width = "100%";
          return adSlotUsableWidth(ins) || pw;
        }
        p = p.parentElement;
      }
    } catch (ePrep) {}
    return adSlotUsableWidth(ins);
  }

  function pushAdsenseElement(ins, attempt) {
    attempt = attempt || 0;
    if (!ins || !ins.getAttribute) return;
    if (ins.getAttribute("data-adsbygoogle-status")) return;
    if (ensureAdsenseSlotHasWidth(ins) < 1) {
      if (attempt < 120) {
        requestAnimationFrame(function () {
          pushAdsenseElement(ins, attempt + 1);
        });
      }
      return;
    }
    try {
      (window.adsbygoogle = window.adsbygoogle || []).push({ element: ins });
    } catch (eEl) {}
  }

  function pushAdsenseUnhandledIn(root) {
    if (!root || !root.querySelectorAll) return;
    try {
      var nodes = root.querySelectorAll(
        "ins.adsbygoogle:not([data-adsbygoogle-status])",
      );
      var i = 0;
      for (; i < nodes.length; i++) {
        pushAdsenseElement(nodes[i], 0);
      }
    } catch (e1) {}
  }

  function adsLazyLoadEnabled() {
    try {
      return !!(
        window.TirtayasaPublicConfig &&
        window.TirtayasaPublicConfig.adsLazyLoadEnabled
      );
    } catch (eLazyCfg) {
      return false;
    }
  }

  function activateAdSlot(slot) {
    if (!slot || slot.nodeType !== 1) return;
    if (!slot.classList || !slot.classList.contains("site-ad")) return;
    if (!adsLazyLoadEnabled()) {
      pushAdsenseUnhandledIn(slot);
      return;
    }
    if (slot.getAttribute("data-tirtayasa-lazy-activated") === "1") return;
    if (slot.getAttribute("data-tirtayasa-lazy-observing") === "1") return;
    if (typeof IntersectionObserver === "undefined") {
      pushAdsenseUnhandledIn(slot);
      slot.setAttribute("data-tirtayasa-lazy-activated", "1");
      return;
    }
    slot.setAttribute("data-tirtayasa-lazy-observing", "1");
    var obs = new IntersectionObserver(
      function (entries, o) {
        var j = 0;
        for (; j < entries.length; j++) {
          var ent = entries[j];
          if (!ent.isIntersecting) continue;
          var el = ent.target;
          try {
            o.unobserve(el);
          } catch (eUn) {}
          el.removeAttribute("data-tirtayasa-lazy-observing");
          el.setAttribute("data-tirtayasa-lazy-activated", "1");
          pushAdsenseUnhandledIn(el);
        }
      },
      { root: null, rootMargin: "280px 0px", threshold: 0 },
    );
    try {
      obs.observe(slot);
    } catch (eObs) {
      slot.removeAttribute("data-tirtayasa-lazy-observing");
      pushAdsenseUnhandledIn(slot);
      slot.setAttribute("data-tirtayasa-lazy-activated", "1");
    }
  }

  function activateAdSlotsIn(root) {
    if (!root || !root.querySelectorAll) return;
    var list = [];
    if (root.matches && root.matches(".site-ad")) list.push(root);
    root.querySelectorAll(".site-ad").forEach(function (el) {
      list.push(el);
    });
    var i = 0;
    for (; i < list.length; i++) {
      activateAdSlot(list[i]);
    }
  }

  function runSiteAdSlotsPass() {
    try {
      document.querySelectorAll(".site-ad").forEach(function (slot) {
        activateAdSlot(slot);
      });
    } catch (e2) {}
  }

  window.PublicSiteAds = {
    stripLeadingDuplicateH1Dom: stripLeadingDuplicateH1Dom,
    pushUnhandledIn: pushAdsenseUnhandledIn,
    activateSlotsIn: activateAdSlotsIn,
  };

  document.addEventListener("DOMContentLoaded", function () {
    requestAnimationFrame(function () {
      requestAnimationFrame(runSiteAdSlotsPass);
    });
  });
  window.addEventListener("load", function () {
    runSiteAdSlotsPass();
    setTimeout(runSiteAdSlotsPass, 80);
    setTimeout(runSiteAdSlotsPass, 400);
  });

  window.TirtayasaAPI = {
    PREFIX: PREFIX,
    getToken: getToken,
    setSession: setSession,
    getUser: getUser,
    logout: function () {
      setSession(null, null);
    },
    login: function (email, password) {
      return request("POST", "/auth/login", {
        email: email,
        password: password,
      });
    },
    register: function (payload) {
      return request("POST", "/auth/register", payload);
    },
    get: function (path) {
      return request("GET", path, null);
    },
    getPublic: function (path) {
      return requestPublic("GET", path);
    },
    post: function (path, body) {
      return request("POST", path, body);
    },
    put: function (path, body) {
      return request("PUT", path, body);
    },
    delete: function (path) {
      return request("DELETE", path, null);
    },
  };
})();
