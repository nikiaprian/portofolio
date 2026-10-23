(function () {
  "use strict";

  var api = window.TirtayasaAPI;
  /** Kategori dari SSR (header.html); dipakai untuk state aktif & heuristik path. */
  var lastCats = [];

  function debounce(fn, ms) {
    var t;
    return function () {
      var self = this;
      var args = arguments;
      clearTimeout(t);
      t = setTimeout(function () {
        fn.apply(self, args);
      }, ms);
    };
  }

  function readCategoriesFromDOM() {
    var primary = document.getElementById("header-nav-cats");
    if (!primary) return [];
    var out = [];
    primary.querySelectorAll("a[data-cat-slug]").forEach(function (a) {
      var slug = (a.getAttribute("data-cat-slug") || "").trim();
      if (!slug) return;
      var parentSlug = (a.getAttribute("data-parent-slug") || "").trim();
      out.push({
        slug: slug,
        parentSlug: parentSlug,
        pathKey: parentSlug ? parentSlug + "/" + slug : slug,
        name: (a.textContent || "").trim(),
      });
    });
    return out;
  }

  function updateNavActive() {
    var params = new URLSearchParams(location.search);
    var cat = params.get("category") || "";
    var tag = params.get("tag") || "";
    var hash = location.hash || "";
    var pathSeg = /^\/([^/]+)\/?$/.exec(location.pathname);
    var pathTwo = /^\/([^/]+)\/([^/]+)\/?$/.exec(location.pathname);
    var pathCat = "";
    var pathCatKey = "";
    if (pathTwo && lastCats && lastCats.length) {
      var p1 = pathTwo[1];
      var p2 = pathTwo[2];
      if (
        p1 &&
        p2 &&
        !/^(posts|rss-feeds|login)$/.test(p1) &&
        lastCats.some(function (c) {
          return (c.pathKey || c.slug || "") === p1 + "/" + p2;
        })
      ) {
        pathCatKey = p1 + "/" + p2;
        pathCat = p2;
      }
    }
    if (!pathCatKey && pathSeg && lastCats && lastCats.length) {
      var ps = pathSeg[1];
      if (
        ps &&
        !/^(posts|rss-feeds|login)$/.test(ps) &&
        lastCats.some(function (c) {
          return !c.parentSlug && (c.slug || "") === ps;
        })
      ) {
        pathCat = ps;
        pathCatKey = ps;
      }
    }
    var catActive = cat || pathCat;
    var catActiveKey = cat || pathCatKey;
    var onPost =
      /^\/post\//.test(location.pathname) ||
      /^#\/post\//.test(hash) ||
      (!!pathSeg &&
        pathSeg[1] &&
        !/^(posts|rss-feeds|login)$/.test(pathSeg[1]) &&
        (!lastCats.length ||
          !lastCats.some(function (c) {
            return (c.slug || "") === pathSeg[1];
          })));
    var home = document.querySelector(".header-nav-link--home");
    var homeActive =
      location.pathname === "/" &&
      !cat &&
      !tag &&
      !pathCat &&
      !onPost &&
      hash.indexOf("#/login") !== 0;

    if (home) home.classList.toggle("is-active", homeActive);

    document
      .querySelectorAll(
        ".header-nav-link[data-cat-slug], .header-nav-dropdown-item[data-cat-slug]"
      )
      .forEach(function (a) {
        var slug = a.getAttribute("data-cat-slug") || "";
        var parentSlug = a.getAttribute("data-parent-slug") || "";
        var key = parentSlug ? parentSlug + "/" + slug : slug;
        var active =
          !!catActiveKey &&
          (key === catActiveKey || (!parentSlug && slug === catActive));
        a.classList.toggle("is-active", active);
        if (parentSlug && active) {
          var wrap = a.closest(".header-nav-dropdown");
          if (wrap) wrap.classList.add("is-active");
          var parentLink = wrap && wrap.querySelector(".header-nav-link--has-children");
          if (parentLink) parentLink.classList.add("is-active");
        }
      });

    document.querySelectorAll(".header-nav-dropdown").forEach(function (wrap) {
      wrap.classList.remove("is-active");
      var parentSlug = "";
      var plink = wrap.querySelector(".header-nav-link--has-children");
      if (plink) parentSlug = (plink.getAttribute("data-cat-slug") || "").trim();
      var childActive =
        !!catActiveKey && !!parentSlug && catActiveKey.indexOf(parentSlug + "/") === 0;
      if (childActive || (catActive && parentSlug && catActive === parentSlug)) {
        wrap.classList.add("is-active");
      }
    });
  }

  var navDropdownCloseTimer = null;
  var navDropdownOpenWrap = null;
  var navDropdownDocBound = false;

  /** Panel di layer `display:none` tidak bisa ditampilkan; pindah ke body sekali saat init. */
  function ensureNavDropdownPanelsPortaled() {
    var layer = document.getElementById("header-nav-dropdown-layer");
    var panels = layer
      ? layer.querySelectorAll(".header-nav-dropdown-panel")
      : document.querySelectorAll(".header-nav-dropdown-panel");
    panels.forEach(function (panel) {
      if (panel.parentNode !== document.body) {
        document.body.appendChild(panel);
      }
    });
  }

  function navDropdownPanelForWrap(wrap) {
    if (!wrap) return null;
    if (wrap._navDropdownPanel) return wrap._navDropdownPanel;
    var id = wrap.getAttribute("data-dropdown-panel");
    if (!id) return null;
    var panel = document.getElementById(id);
    if (panel) wrap._navDropdownPanel = panel;
    return panel;
  }

  function navDropdownTrigger(wrap) {
    return wrap ? wrap.querySelector(".header-nav-link--has-children") : null;
  }

  function positionNavDropdownPanel(wrap) {
    var panel = navDropdownPanelForWrap(wrap);
    var trigger = navDropdownTrigger(wrap);
    if (!panel || !trigger) return;
    var r = trigger.getBoundingClientRect();
    var vw = window.innerWidth || document.documentElement.clientWidth || 0;
    var minW = Math.max(Math.round(r.width), 200);
    var left = Math.round(r.left);
    if (left + minW > vw - 8) {
      left = Math.max(8, vw - minW - 8);
    }
    panel.style.position = "fixed";
    panel.style.zIndex = "10001";
    panel.style.top = Math.max(0, Math.round(r.bottom) - 2) + "px";
    panel.style.left = left + "px";
    panel.style.minWidth = minW + "px";
  }

  function hideNavDropdownPanel(panel) {
    if (!panel) return;
    panel.classList.remove("is-visible");
    panel.classList.add("is-collapsed");
    panel.setAttribute("aria-hidden", "true");
    panel.style.removeProperty("display");
    panel.style.removeProperty("position");
    panel.style.removeProperty("z-index");
    panel.style.removeProperty("top");
    panel.style.removeProperty("left");
    panel.style.removeProperty("min-width");
  }

  function closeNavDropdown(wrap) {
    if (!wrap) return;
    var trigger = navDropdownTrigger(wrap);
    var panel = navDropdownPanelForWrap(wrap);
    wrap.classList.remove("is-open");
    wrap.classList.remove("is-open-touch");
    if (trigger) trigger.setAttribute("aria-expanded", "false");
    hideNavDropdownPanel(panel);
    if (navDropdownOpenWrap === wrap) navDropdownOpenWrap = null;
  }

  function closeAllNavDropdowns(except) {
    document.querySelectorAll(".header-nav-dropdown").forEach(function (w) {
      if (except && w === except) return;
      closeNavDropdown(w);
    });
  }

  function scheduleCloseNavDropdown(wrap) {
    clearTimeout(navDropdownCloseTimer);
    navDropdownCloseTimer = setTimeout(function () {
      closeNavDropdown(wrap);
    }, 200);
  }

  function cancelCloseNavDropdown() {
    clearTimeout(navDropdownCloseTimer);
  }

  function openNavDropdown(wrap) {
    var trigger = navDropdownTrigger(wrap);
    var panel = navDropdownPanelForWrap(wrap);
    if (!trigger || !panel) return;
    cancelCloseNavDropdown();
    closeAllNavDropdowns(wrap);
    if (panel.parentNode !== document.body) {
      document.body.appendChild(panel);
    }
    navDropdownOpenWrap = wrap;
    wrap.classList.add("is-open");
    trigger.setAttribute("aria-expanded", "true");
    panel.classList.remove("is-collapsed");
    panel.classList.add("is-visible");
    panel.setAttribute("aria-hidden", "false");
    panel.style.setProperty("display", "block", "important");
    panel.style.setProperty("visibility", "visible", "important");
    panel.style.setProperty("opacity", "1", "important");
    positionNavDropdownPanel(wrap);
  }

  function pointerWithinDropdown(wrap, panel, target) {
    if (!target) return false;
    if (wrap && wrap.contains(target)) return true;
    if (panel && panel.contains(target)) return true;
    return false;
  }

  function bindNavDropdownPanelHover(wrap, panel) {
    if (!panel || panel._navHoverBound) return;
    panel._navHoverBound = true;
    panel.addEventListener("mouseenter", cancelCloseNavDropdown);
    panel.addEventListener("mouseleave", function () {
      scheduleCloseNavDropdown(wrap);
    });
  }

  function initNavDropdowns() {
    ensureNavDropdownPanelsPortaled();
    var root = document.getElementById("header-nav-cats");
    if (!root) return;

    var canHover = window.matchMedia("(hover: hover)").matches;

    root.querySelectorAll(".header-nav-dropdown").forEach(function (wrap) {
      if (wrap.getAttribute("data-nav-dd-bound") === "1") return;
      wrap.setAttribute("data-nav-dd-bound", "1");

      var trigger = navDropdownTrigger(wrap);
      if (!trigger) return;

      var panel = navDropdownPanelForWrap(wrap);
      if (panel) bindNavDropdownPanelHover(wrap, panel);

      function onOpen() {
        openNavDropdown(wrap);
      }

      function onMaybeClose(e) {
        var panelRef = navDropdownPanelForWrap(wrap);
        if (pointerWithinDropdown(wrap, panelRef, e.relatedTarget)) return;
        scheduleCloseNavDropdown(wrap);
      }

      wrap.addEventListener("mouseenter", onOpen);
      wrap.addEventListener("pointerenter", onOpen);
      trigger.addEventListener("focus", onOpen);

      wrap.addEventListener("mouseleave", onMaybeClose);
      trigger.addEventListener("mouseleave", onMaybeClose);

      if (panel) {
        panel.addEventListener("mouseleave", onMaybeClose);
      }

      trigger.addEventListener("click", function (e) {
        if (canHover && !wrap.classList.contains("is-open-touch")) return;
        if (wrap.classList.contains("is-open")) {
          e.preventDefault();
          closeNavDropdown(wrap);
          return;
        }
        e.preventDefault();
        openNavDropdown(wrap);
        wrap.classList.add("is-open-touch");
      });
    });

    if (navDropdownDocBound) return;
    navDropdownDocBound = true;

    var catsScroll = document.getElementById("header-nav-cats");
    if (catsScroll) {
      catsScroll.addEventListener(
        "scroll",
        function () {
          if (navDropdownOpenWrap) positionNavDropdownPanel(navDropdownOpenWrap);
        },
        { passive: true }
      );
    }

    window.addEventListener(
      "resize",
      debounce(function () {
        if (navDropdownOpenWrap) positionNavDropdownPanel(navDropdownOpenWrap);
      }, 100)
    );

    document.addEventListener("click", function (e) {
      if (e.target.closest(".header-nav-dropdown")) return;
      if (e.target.closest(".header-nav-dropdown-panel")) return;
      closeAllNavDropdowns();
    });

    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape") closeAllNavDropdowns();
    });
  }

  function syncSearchInputs() {
    var q = new URLSearchParams(location.search).get("q") || "";
    document
      .querySelectorAll(
        '.header-search input[name="q"], .offcanvas-search input[name="q"]'
      )
      .forEach(function (inp) {
        inp.value = q;
      });
    syncIconOnlySearchOpenState(q);
  }

  function syncIconOnlySearchOpenState(q) {
    var form = document.getElementById("header-search-form");
    if (!form) return;
    if (
      !document.body.classList.contains("site-layout-detik") &&
      !document.body.classList.contains("site-layout-tirtayasa")
    ) {
      return;
    }
    var input = form.querySelector('input[name="q"]');
    var hasValue = (q || "").trim() !== "";
    if (hasValue) {
      form.classList.add("header-search--open");
    } else if (!input || document.activeElement !== input) {
      form.classList.remove("header-search--open");
    }
  }

  function navScrollStep() {
    var el = document.getElementById("header-nav-cats");
    if (!el) return 160;
    return Math.max(120, Math.floor(el.clientWidth * 0.78));
  }

  function refreshNavScrollArrows() {
    var cats = document.getElementById("header-nav-cats");
    var next = document.getElementById("header-nav-scroll-next");
    if (!cats || !next) return;

    var sw = cats.scrollWidth;
    var cw = cats.clientWidth;
    var sl = cats.scrollLeft;
    var overflow = sw > cw + 2;
    var atEnd = sl + cw >= sw - 2;

    if (!overflow || atEnd) {
      next.hidden = true;
      next.disabled = false;
      return;
    }

    next.hidden = false;
    next.disabled = false;
  }

  var navScrollBound = false;
  function initNavScrollControls() {
    if (navScrollBound) return;
    var cats = document.getElementById("header-nav-cats");
    var next = document.getElementById("header-nav-scroll-next");
    if (!cats || !next) return;
    navScrollBound = true;

    next.addEventListener("click", function () {
      cats.scrollBy({ left: navScrollStep(), behavior: "smooth" });
    });
    cats.addEventListener("scroll", refreshNavScrollArrows, { passive: true });
    window.addEventListener("resize", debounce(refreshNavScrollArrows, 120));
  }

  /** Isi menu kategori dari markup SSR (tabel categories di server); tanpa fetch API. */
  function initNavCategoriesFromSSR() {
    var primary = document.getElementById("header-nav-cats");
    if (!primary) return;

    initNavDropdowns();
    lastCats = readCategoriesFromDOM();
    updateNavActive();
    syncSearchInputs();

    requestAnimationFrame(function () {
      requestAnimationFrame(function () {
        refreshNavScrollArrows();
      });
    });
  }

  function closeOffcanvas() {
    var panel = document.getElementById("site-offcanvas");
    var back = document.getElementById("site-offcanvas-backdrop");
    var btn = document.getElementById("site-menu-toggle");
    if (panel) {
      panel.hidden = true;
      panel.setAttribute("aria-hidden", "true");
    }
    if (back) back.hidden = true;
    if (btn) btn.setAttribute("aria-expanded", "false");
    document.body.classList.remove("offcanvas-open");
  }

  function openOffcanvas() {
    var panel = document.getElementById("site-offcanvas");
    var back = document.getElementById("site-offcanvas-backdrop");
    var btn = document.getElementById("site-menu-toggle");
    if (panel) {
      panel.hidden = false;
      panel.setAttribute("aria-hidden", "false");
    }
    if (back) back.hidden = false;
    if (btn) btn.setAttribute("aria-expanded", "true");
    document.body.classList.add("offcanvas-open");
  }

  function initOffcanvas() {
    var toggle = document.getElementById("site-menu-toggle");
    var closeBtn = document.getElementById("site-menu-close");
    var back = document.getElementById("site-offcanvas-backdrop");
    var panel = document.getElementById("site-offcanvas");
    if (toggle)
      toggle.addEventListener("click", function () {
        var p = document.getElementById("site-offcanvas");
        if (p && p.hidden) openOffcanvas();
        else closeOffcanvas();
      });
    if (closeBtn) closeBtn.addEventListener("click", closeOffcanvas);
    if (back) back.addEventListener("click", closeOffcanvas);
    if (panel) {
      panel.addEventListener("click", function (e) {
        if (e.target.closest("a")) closeOffcanvas();
      });
    }
  }

  /**
   * Logo gambar + fallback teks: sinkronkan dengan gambar dari cache (complete sebelum onload),
   * dan satu jalur untuk load/error tanpa race dengan inline handler.
   */
  function initSiteLogoImages() {
    document
      .querySelectorAll(".site-logo-img-wrap .site-logo-img")
      .forEach(function (img) {
        var wrap = img.closest(".site-logo-img-wrap");
        var fallback = img.previousElementSibling;

        function finishOk() {
          if (wrap) wrap.classList.add("site-logo-img-wrap--has-image");
          img.classList.remove("site-logo-img--pending");
          img.classList.add("site-logo-img--ready");
        }

        function finishErr() {
          if (wrap) wrap.classList.add("site-logo-img-wrap--image-failed");
          img.classList.remove("site-logo-img--pending");
          img.style.display = "none";
          if (
            fallback &&
            fallback.classList &&
            fallback.classList.contains("site-logo-text--fallback")
          ) {
            fallback.removeAttribute("aria-hidden");
          }
        }

        if (img.complete) {
          if (img.naturalWidth > 0) {
            finishOk();
          } else {
            finishErr();
          }
          return;
        }

        img.addEventListener(
          "load",
          function () {
            finishOk();
          },
          { once: true }
        );
        img.addEventListener(
          "error",
          function () {
            finishErr();
          },
          { once: true }
        );
      });
  }

  /**
   * Mobile: jika header baris atas overflow (nabrak logo), sembunyikan elemen secara bertahap.
   * Prioritas: sembunyikan hamburger dulu, lalu ikon sosial (RSS paling akhir).
   */
  function shrinkHeaderSocialToFit() {
    if (!window.matchMedia("(max-width: 720px)").matches) {
      var socAll = document.querySelector(".header-social");
      var menuAll = document.getElementById("site-menu-toggle");
      if (socAll) {
        socAll.querySelectorAll("a.social-btn").forEach(function (a) {
          a.style.removeProperty("display");
        });
      }
      if (menuAll) menuAll.style.removeProperty("display");
      return;
    }

    var inner = document.querySelector(".header-top-inner");
    var social = document.querySelector(".header-social");
    var menu = document.getElementById("site-menu-toggle");
    if (!inner || !social) return;

    var anchors = Array.prototype.slice.call(social.querySelectorAll("a.social-btn"));
    if (!anchors.length) return;

    anchors.forEach(function (a) {
      a.style.removeProperty("display");
    });
    if (menu) menu.style.removeProperty("display");

    var guard = 0;
    function rowOverflows() {
      var sw = inner.scrollWidth > inner.clientWidth + 2;
      var h = inner.getBoundingClientRect().height;
      // Single-row header mobile biasanya masih bisa 58–66px tergantung font & padding.
      // Ambang 54px terlalu agresif dan bisa memicu "sembunyikan semuanya".
      var tooTall = h > 68;
      return sw || tooTall;
    }

    // Prioritas: jika hamburger nabrak logo di layar sempit, hilangkan dulu.
    if (rowOverflows() && menu) {
      menu.style.display = "none";
    }

    // Jika masih overflow, hilangkan ikon sosial satu per satu.
    // Pertahankan RSS paling akhir (jangan hilang duluan).
    var rss = null;
    var hideOrder = [];
    anchors.forEach(function (a) {
      if (a.classList && a.classList.contains("social-rss")) rss = a;
      else hideOrder.push(a);
    });
    if (rss) hideOrder.push(rss);

    while (rowOverflows() && guard < hideOrder.length) {
      // Jangan sampai semuanya hilang: usahakan tetap ada setidaknya RSS jika memungkinkan.
      if (rss && hideOrder[guard] === rss && hideOrder.length > 1) {
        // Jika cuma RSS yang tersisa dan masih overflow, biarkan RSS tetap tampil.
        // Overflow ekstrem lebih baik diselesaikan oleh ellipsis logo daripada menghilangkan semua aksi.
        break;
      }
      hideOrder[guard].style.display = "none";
      guard++;
    }
  }

  function initHeaderSocialShrink() {
    var inner = document.querySelector(".header-top-inner");
    if (!inner) return;

    var run = debounce(function () {
      shrinkHeaderSocialToFit();
    }, 80);

    window.addEventListener("resize", run, { passive: true });
    window.addEventListener("orientationchange", run, { passive: true });
    window.addEventListener(
      "load",
      function () {
        run();
      },
      { once: true }
    );

    // Hindari loop visual: perubahan display pada shrink bisa memicu ResizeObserver
    // berulang (terlihat seperti halaman berkedip), terutama di layout detik.
    // Cukup andalkan resize/orientation/load/fonts untuk trigger recalculation.

    if (document.fonts && document.fonts.ready) {
      document.fonts.ready.then(function () {
        run();
      });
    }

    requestAnimationFrame(function () {
      requestAnimationFrame(run);
    });
  }

  function fitTitleToThreeLines(el) {
    if (!el || !el.isConnected) return;
    el.style.removeProperty("font-size");
    var cs = window.getComputedStyle(el);
    var base = parseFloat(cs.fontSize || "");
    if (!isFinite(base) || base <= 0) return;
    var lhRaw = String(cs.lineHeight || "").trim();
    var lh = parseFloat(lhRaw);
    var lineFactor = 1.35;
    if (isFinite(lh) && lh > 0) {
      lineFactor = lh / base;
    }
    var minRatio = 0.74;
    if (
      el.classList &&
      (el.classList.contains("blog-mostviewed-title") ||
        el.classList.contains("blog-also-read-title") ||
        el.classList.contains("blog-latest-stack-title"))
    ) {
      minRatio = 0.62;
    }
    var min = Math.max(10, base * minRatio);
    var size = base;
    var maxHeight = size * lineFactor * 3 + 1;
    var guard = 0;
    while (el.scrollHeight > maxHeight && size > min && guard < 24) {
      size -= 0.25;
      if (size < min) size = min;
      el.style.fontSize = size.toFixed(2) + "px";
      maxHeight = size * lineFactor * 3 + 1;
      guard++;
    }
  }

  function fitAllArticleTitles() {
    var selectors = [
      ".blog-popular-title",
      ".blog-posts-title",
      ".blog-latest-title",
      ".blog-latest-stack-title",
      ".blog-mostviewed-title",
      ".blog-also-read-title",
      ".blog-article-title",
      ".blog-feature-slide-title",
    ];
    document.querySelectorAll(selectors.join(",")).forEach(function (el) {
      fitTitleToThreeLines(el);
    });
  }

  function initTitleAutoFit() {
    var run = debounce(function () {
      fitAllArticleTitles();
    }, 80);
    run();
    window.addEventListener("resize", run, { passive: true });
    window.addEventListener("orientationchange", run, { passive: true });
    if (typeof MutationObserver !== "undefined") {
      var mo = new MutationObserver(run);
      mo.observe(document.body, { childList: true, subtree: true });
    }
  }

  function initSearchForm() {
    var form = document.getElementById("header-search-form");
    if (!form || !document.getElementById("public-root")) return;

    var input = form.querySelector('input[name="q"]');
    var submitBtn = form.querySelector(".header-search-submit");

    function isIconOnlySearch() {
      return (
        document.body.classList.contains("site-layout-detik") ||
        document.body.classList.contains("site-layout-tirtayasa")
      );
    }
    function isOpen() {
      return form.classList.contains("header-search--open");
    }
    function openSearch() {
      form.classList.add("header-search--open");
      if (input) {
        requestAnimationFrame(function () {
          try {
            input.focus({ preventScroll: true });
          } catch (_) {
            input.focus();
          }
        });
      }
    }
    function closeSearch() {
      form.classList.remove("header-search--open");
    }

    // Detik / neo nav: search collapsed to icon-only. Clicking the submit icon
    // while collapsed (and empty) expands the field instead of submitting.
    if (submitBtn) {
      submitBtn.addEventListener("click", function (e) {
        if (!isIconOnlySearch() || isOpen()) return;
        var v = input && input.value ? String(input.value).trim() : "";
        if (!v) {
          e.preventDefault();
          openSearch();
        }
      });
    }

    // Outside click collapses the field again when empty.
    document.addEventListener("click", function (e) {
      if (!isIconOnlySearch() || !isOpen()) return;
      if (form.contains(e.target)) return;
      var v = input && input.value ? String(input.value).trim() : "";
      if (!v) closeSearch();
    });

    if (input) {
      input.addEventListener("keydown", function (e) {
        var k = e.key || "";
        if (k === "Escape" || e.keyCode === 27) {
          if (isIconOnlySearch() && isOpen()) {
            closeSearch();
            if (submitBtn) submitBtn.focus();
          }
        }
      });
      input.addEventListener("blur", function () {
        if (!isIconOnlySearch() || !isOpen()) return;
        setTimeout(function () {
          var active = document.activeElement;
          if (active && form.contains(active)) return;
          var v = (input.value || "").trim();
          if (!v) closeSearch();
        }, 140);
      });
    }

    form.addEventListener("submit", function (e) {
      e.preventDefault();
      var fd = new FormData(form);
      var q = (fd.get("q") || "").toString().trim();
      if (q.length > 100) q = q.slice(0, 100);
      var cur = new URLSearchParams(location.search);
      var cat = (cur.get("category") || "").trim();
      var tag = (cur.get("tag") || "").trim();
      var params = new URLSearchParams();
      if (cat) params.set("category", cat);
      if (tag) params.set("tag", tag);
      if (q) params.set("q", q);
      var qs = params.toString();
      var url = "/" + (qs ? "?" + qs : "");
      if (history.replaceState) history.replaceState(null, "", url);
      else location.href = url;
      if (window.TirtayasaPublicRefresh) window.TirtayasaPublicRefresh();
    });

    // Auto-open when arriving with ?q= on icon-only search layouts.
    if (isIconOnlySearch() && input && (input.value || "").trim() !== "") {
      openSearch();
    }
  }

  function syncNavUser() {
    var el = document.getElementById("nav-user");
    if (!el || !api) return;
    var u = api.getUser();
    el.textContent = u && u.name ? "Masuk sebagai " + u.name : "";
  }

  function syncHeaderDate() {
    var el = document.getElementById("site-header-date");
    if (!el) return;
    var now = new Date();
    var text = now.toLocaleDateString("id-ID", {
      weekday: "long",
      day: "2-digit",
      month: "long",
      year: "numeric",
    });
    el.textContent = text;
    var y = now.getFullYear();
    var m = String(now.getMonth() + 1).padStart(2, "0");
    var d = String(now.getDate()).padStart(2, "0");
    el.setAttribute("datetime", y + "-" + m + "-" + d);
  }

  window.TirtayasaHeader = {
    refresh: function () {
      ensureNavDropdownPanelsPortaled();
      initNavDropdowns();
      lastCats = readCategoriesFromDOM();
      updateNavActive();
      syncSearchInputs();
      syncNavUser();
      syncHeaderDate();
      refreshNavScrollArrows();
      shrinkHeaderSocialToFit();
    },
  };

  document.addEventListener("DOMContentLoaded", function () {
    initSiteLogoImages();
    initHeaderSocialShrink();
    initNavCategoriesFromSSR();
    initOffcanvas();
    initNavScrollControls();
    initSearchForm();
    initTitleAutoFit();
    syncNavUser();
    syncHeaderDate();
  });
})();
