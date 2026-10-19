(function () {
  "use strict";

  var LOGS_API = "/api/v1/admin/scraper-logs";
  var AC = typeof window !== "undefined" ? window.AdminCrud : null;

  function qs(sel) {
    return document.querySelector(sel);
  }

  function formatLogTime(iso) {
    if (
      typeof AdminLogFormat !== "undefined" &&
      AdminLogFormat.formatTimestamp
    ) {
      return AdminLogFormat.formatTimestamp(iso);
    }
    if (!iso) return "—";
    try {
      var d = new Date(iso);
      if (isNaN(d.getTime())) return String(iso);
      return d.toISOString();
    } catch (e) {
      return String(iso);
    }
  }

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function aiProviderBadge(code) {
    var c = (code || "").toLowerCase().trim();
    if (!c) return "—";
    var cls = "text-bg-secondary";
    if (c === "gemini") cls = "text-bg-primary";
    else if (c === "openai") cls = "text-bg-success";
    else if (c === "groq") cls = "text-bg-warning text-dark";
    else if (c === "claude") cls = "text-bg-dark";
    else if (c === "auto") cls = "text-bg-secondary";
    else cls = "text-bg-light text-dark border";
    return (
      '<span class="badge ' + cls + '">' + escapeHtml(c) + "</span>"
    );
  }

  function levelBadge(level) {
    var l = (level || "").toLowerCase();
    if (l === "error") {
      return '<span class="badge text-bg-danger">error</span>';
    }
    if (l === "warn" || l === "warning") {
      return '<span class="badge text-bg-warning text-dark">warn</span>';
    }
    if (l === "info") {
      return '<span class="badge text-bg-info text-dark">info</span>';
    }
    return (
      '<span class="badge text-bg-secondary">' +
      String(level || "—")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;") +
      "</span>"
    );
  }

  function categoryBadge(slug) {
    var s = slug != null ? String(slug).trim() : "";
    if (!s) return "—";
    return (
      '<span class="badge rounded-pill text-bg-light text-dark border font-monospace small">' +
      s.replace(/</g, "&lt;").replace(/>/g, "&gt;") +
      "</span>"
    );
  }

  function translateLogMessage(message) {
    var s = message != null ? String(message).trim() : "";
    if (!s) return "";
    s = s.replace(/^\[scraper\]\s*/i, "");
    var replacements = [
      [/^GET bad request\b/i, "Request GET tidak valid"],
      [/^GET cancelled\b/i, "Request GET dibatalkan"],
      [/^GET failed\b/i, "Request GET gagal"],
      [/^GET image failed\b/i, "Gagal mengambil gambar"],
      [/^GET feed read body failed\b/i, "Gagal membaca isi feed"],
      [/^GET read body failed\b/i, "Gagal membaca isi halaman"],
      [/^GET parse HTML failed\b/i, "Gagal mem-parsing HTML"],
      [/^GET sitemap read body failed\b/i, "Gagal membaca isi sitemap"],
      [/^GET sitemap XML decode failed\b/i, "Gagal membaca XML sitemap"],
      [/^GET url=/i, "GET url="],
      [/^started\b/i, "Scraper dimulai"],
      [/^preflight check failed\b/i, "Pemeriksaan awal gagal"],
      [/^load jobs for run budget failed\b/i, "Gagal memuat job untuk batas run"],
      [/^run failed\b/i, "Run gagal"],
      [/^nothing to scrape\b/i, "Tidak ada yang bisa di-scrape"],
      [/^skipped\b/i, "Dilewati"],
      [/^run start\b/i, "Run dimulai"],
      [/^stopped mid-run\b/i, "Berhenti di tengah run"],
      [/^stopped during inter-category pause\b/i, "Berhenti saat jeda antar kategori"],
      [/^skip category\b/i, "Lewati kategori"],
      [/^feed error\b/i, "Error feed"],
      [/^inter-category pause\b/i, "Jeda antar kategori"],
      [/^completed\b/i, "Selesai"],
      [/^sitemap article error\b/i, "Error artikel sitemap"],
      [/^sitemap create error\b/i, "Gagal membuat post dari sitemap"],
      [/^sitemap index child error\b/i, "Error child sitemap index"],
      [/^sitemap index child scrape error\b/i, "Gagal scrape child sitemap index"],
      [/^sitemap urlset article error\b/i, "Error artikel urlset sitemap"],
      [/^sitemap urlset create error\b/i, "Gagal membuat post dari urlset sitemap"],
      [/^item error\b/i, "Error item"],
      [/^entry error\b/i, "Error entry"],
      [/^AI API call start\b/i, "Panggilan API AI dimulai"],
      [/^AI API call ok\b/i, "Panggilan API AI berhasil"],
      [/^AI API call failed\b/i, "Panggilan API AI gagal"],
      [/^skip candidate\b/i, "Lewati kandidat"],
      [/^skip duplicate\b/i, "Lewati duplikat"],
      [/^web image source=/i, "Sumber gambar web="],
      [/^web image failed\b/i, "Gambar web gagal"],
      [/^AI image generated\b/i, "Gambar AI berhasil dibuat"],
      [/^AI image failed\b/i, "Gambar AI gagal"],
      [/^AI image empty\b/i, "Gambar AI kosong"],
      [/^AI cover R2 upload failed, using local disk\b/i, "Upload sampul AI ke R2 gagal, memakai disk lokal"],
    ];
    replacements.forEach(function (pair) {
      s = s.replace(pair[0], pair[1]);
    });
    return s
      .replace(/\burl=/g, "url=")
      .replace(/\berr=/g, "error=")
      .replace(/\bhttp_status=/g, "status_http=")
      .replace(/\bcategory=/g, "kategori=")
      .replace(/\btitle=/g, "judul=")
      .replace(/\blink=/g, "link=")
      .replace(/\bjobs=/g, "job=")
      .replace(/\btotal_feeds=/g, "total_feed=")
      .replace(/\bprocessed_feeds=/g, "feed_diproses=")
      .replace(/\bfailed_feeds=/g, "feed_gagal=")
      .replace(/\bnew_posts=/g, "post_baru=")
      .replace(/\bphase=/g, "fase=")
      .replace(/\bduration_ms=/g, "durasi_ms=")
      .replace(/\bout_runes=/g, "output_rune=")
      .replace(/\bout_words=/g, "output_kata=")
      .replace(/\bsource_words=/g, "kata_sumber=")
      .replace(/\bdisabled\b/g, "dinonaktifkan")
      .replace(/\bno enabled categories with feed URLs\b/gi, "tidak ada kategori aktif dengan URL feed")
      .replace(/\bAI provider is disabled\b/gi, "Penyedia AI dinonaktifkan")
      .replace(/\bempty title\/link\b/gi, "judul/link kosong")
      .replace(/\bscraped body too short\b/gi, "isi hasil scrape terlalu pendek")
      .replace(/\bfinal body too short after paraphrase\b/gi, "isi akhir terlalu pendek setelah parafrase")
      .replace(/\bno image\b/gi, "tidak ada gambar");
  }

  function buildListURL(page, size, q, ctx) {
    var p = new URLSearchParams();
    p.set("page", String(page));
    p.set("size", String(size));
    if (q) p.set("q", q);
    var lv = ctx.levelFilterEl;
    if (lv && lv.value) p.set("level", lv.value);
    return LOGS_API + "?" + p.toString();
  }

  function parseTotalPages(payload) {
    if (!payload || typeof payload.total !== "number") return 0;
    if (payload.total === 0) return 0;
    var sz = payload.size || 10;
    if (sz <= 0) return 0;
    return Math.ceil(payload.total / sz);
  }

  document.addEventListener("DOMContentLoaded", function () {
    if (!AC || !AC.bindPage) return;

    AC.bindPage({
      tokenKey: "bebii_token",
      loginUrl:
        (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login",
      initialPageSize: 50,
      selectors: {
        tbody: "#scr-logs-tbody",
        error: "#scr-logs-error",
        pageInfo: "#scr-logs-page-info",
        pageSize: "#scr-logs-page-size",
        prev: "#scr-logs-prev-btn",
        next: "#scr-logs-next-btn",
        searchInput: "#scr-logs-search",
      },
      buildListURL: buildListURL,
      parseTotalPages: parseTotalPages,
      messages: {
        loadFailed: "Gagal memuat log scraper",
        unexpectedResponse: "Respons server tidak sesuai.",
        network: "Terjadi gangguan jaringan. Coba lagi.",
        pageOf: function (a, b) {
          return "Halaman " + a + " dari " + b;
        },
        noItems: "Tidak ada log di halaman ini.",
      },
      renderEmpty: function (tbody) {
        if (!tbody) return;
        var tr = document.createElement("tr");
        var td = document.createElement("td");
        td.colSpan = 6;
        td.className = "text-secondary py-4 text-center small";
        td.textContent = "Tidak ada log di halaman ini.";
        tr.appendChild(td);
        tbody.appendChild(tr);
      },
      renderRow: function (row) {
        var tr = document.createElement("tr");
        var t1 = document.createElement("td");
        t1.className = "text-nowrap small";
        t1.textContent = formatLogTime(row.created_at);
        var t2 = document.createElement("td");
        t2.className = "small";
        t2.innerHTML = levelBadge(row.level);
        var tProvUsed = document.createElement("td");
        tProvUsed.className = "small text-nowrap";
        tProvUsed.innerHTML = aiProviderBadge(
          row.provider_used != null ? row.provider_used : ""
        );
        var t3 = document.createElement("td");
        t3.className = "small font-monospace text-break";
        t3.textContent = translateLogMessage(row.message);
        var tUrl = document.createElement("td");
        tUrl.className = "small text-break";
        var fu =
          row.feed_url != null && String(row.feed_url).trim() !== ""
            ? String(row.feed_url).trim()
            : "";
        if (fu) {
          var a = document.createElement("a");
          a.href = fu;
          a.target = "_blank";
          a.rel = "noopener noreferrer";
          a.className = "text-break";
          a.textContent = fu.length > 80 ? fu.slice(0, 80) + "…" : fu;
          a.title = fu;
          tUrl.appendChild(a);
        } else {
          tUrl.textContent = "—";
        }
        var t4 = document.createElement("td");
        t4.className = "small text-break";
        t4.innerHTML = categoryBadge(row.category_slug);
        tr.appendChild(t1);
        tr.appendChild(t2);
        tr.appendChild(tProvUsed);
        tr.appendChild(t3);
        tr.appendChild(tUrl);
        tr.appendChild(t4);
        return tr;
      },
      afterBind: function (ctx) {
        ctx.levelFilterEl = qs("#scr-logs-level-filter");
        function reloadFromFilter() {
          ctx.state.page = 1;
          ctx.loadList();
        }
        if (ctx.levelFilterEl) {
          ctx.levelFilterEl.addEventListener("change", reloadFromFilter);
        }
        var ref = qs("#scr-logs-refresh");
        if (ref) {
          ref.addEventListener("click", function () {
            ctx.loadList();
          });
        }
        var clearBtn = qs("#scr-logs-clear");
        if (clearBtn) {
          clearBtn.addEventListener("click", async function () {
            var ok = await AC.confirmAction({
              title: "Bersihkan semua log scraper?",
              text: "Tindakan ini akan menghapus permanen semua baris log scraper.",
              icon: "warning",
              confirmButtonText: "Bersihkan log",
            });
            if (!ok) return;
            clearBtn.disabled = true;
            try {
              var res = await ctx.fetchAuth(LOGS_API, { method: "DELETE" });
              if (!res.ok) {
                var msg = await AC.responseErrorMessage(res, "Gagal membersihkan log");
                throw new Error(msg);
              }
              var data = await res.json().catch(function () {
                return {};
              });
              ctx.showSuccessToast((data.deleted || 0) + " log scraper berhasil dihapus.");
              ctx.state.page = 1;
              ctx.loadList();
            } catch (e) {
              ctx.showErrorToast(e && e.message ? e.message : "Gagal membersihkan log");
            } finally {
              clearBtn.disabled = false;
            }
          });
        }
      },
    });
  });
})();
