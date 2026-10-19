(function () {
  "use strict";

  var LOGS_API = "/api/v1/admin/bulk-post-logs";
  var AC = typeof window !== "undefined" ? window.AdminCrud : null;

  function qs(sel) {
    return document.querySelector(sel);
  }

  function formatTime(iso) {
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

  /** Bootstrap badge for AI provider: gemini, openai, groq, claude (+ auto, unknown). */
  function aiProviderBadge(code) {
    var c = (code || "").toLowerCase().trim();
    if (!c) {
      return "—";
    }
    var cls = "text-bg-secondary";
    if (c === "gemini") cls = "text-bg-primary";
    else if (c === "openai") cls = "text-bg-success";
    else if (c === "groq") cls = "text-bg-warning text-dark";
    else if (c === "claude") cls = "text-bg-dark";
    else if (c === "auto") cls = "text-bg-secondary";
    else cls = "text-bg-light text-dark border";
    return (
      '<span class="badge ' +
      cls +
      '">' +
      escapeHtml(c) +
      "</span>"
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

  function categoryCell(name) {
    var n = name != null ? String(name).trim() : "";
    if (!n) return "—";
    return (
      '<span class="badge text-bg-light text-dark border">' +
      n.replace(/</g, "&lt;").replace(/>/g, "&gt;") +
      "</span>"
    );
  }

  function postBadge(id) {
    if (id == null || id === "") return "—";
    return (
      '<span class="badge font-monospace text-bg-light text-dark border">#' +
      String(id) +
      "</span>"
    );
  }

  function translateEvent(event) {
    var raw = event != null ? String(event).trim() : "";
    var map = {
      suggest_titles_ok: "saran_judul_berhasil",
      suggest_titles_err: "saran_judul_gagal",
      regenerate_titles_ok: "buat_ulang_judul_berhasil",
      regenerate_titles_err: "buat_ulang_judul_gagal",
      scrape_xml_titles_ok: "ambil_judul_xml_berhasil",
      scrape_xml_titles_err: "ambil_judul_xml_gagal",
      generate_abort: "generate_dibatalkan",
      generate_start: "generate_dimulai",
      generate_canceled: "generate_dihentikan",
      generate_cancel_requested: "permintaan_batal_generate",
      generate_item_ok: "item_berhasil",
      generate_item_err: "item_gagal",
      generate_item_warn: "peringatan_item",
      generate_item_image_ok: "gambar_item_berhasil",
      generate_summary: "ringkasan_generate",
    };
    return map[raw] || raw || "—";
  }

  function translateLogMessage(message) {
    var s = message != null ? String(message).trim() : "";
    if (!s) return "";
    return s
      .replace(/^slugs length mismatch$/i, "Jumlah slug tidak sama")
      .replace(/^category ids length mismatch$/i, "Jumlah ID kategori tidak sama")
      .replace(/^category refs length mismatch$/i, "Jumlah referensi kategori tidak sama")
      .replace(/^no valid titles$/i, "Tidak ada judul yang valid")
      .replace(/^missing category$/i, "Kategori belum dipilih")
      .replace(/^bulk generation cancel requested$/i, "Permintaan pembatalan generate bulk diterima")
      .replace(/^empty AI response$/i, "Respons AI kosong")
      .replace(/^cover:/i, "sampul:")
      .replace(/^save:/i, "simpan:")
      .replace(/^stopped after (\d+)\/(\d+) titles$/i, "Dihentikan setelah $1/$2 judul")
      .replace(/^stopped during cooldown after (\d+)\/(\d+) titles$/i, "Dihentikan saat jeda setelah $1/$2 judul")
      .replace(/^stopped during AI content generation after (\d+)\/(\d+) titles$/i, "Dihentikan saat membuat konten AI setelah $1/$2 judul")
      .replace(/^stopped during AI meta description after (\d+)\/(\d+) titles$/i, "Dihentikan saat membuat meta deskripsi AI setelah $1/$2 judul")
      .replace(/^stopped during AI meta keywords after (\d+)\/(\d+) titles$/i, "Dihentikan saat membuat meta kata kunci AI setelah $1/$2 judul")
      .replace(/^stopped during cover image generation after (\d+)\/(\d+) titles$/i, "Dihentikan saat membuat gambar sampul setelah $1/$2 judul")
      .replace(/^stopped during in-content image generation after (\d+)\/(\d+) titles$/i, "Dihentikan saat membuat gambar dalam konten setelah $1/$2 judul")
      .replace(/\bkeyword=/g, "kata_kunci=")
      .replace(/\btitle_lines=/g, "baris_judul=")
      .replace(/\blines=/g, "baris=")
      .replace(/\bchunks=/g, "bagian=")
      .replace(/\btitles=/g, "judul=")
      .replace(/\bcreated=/g, "dibuat=")
      .replace(/\bfailed=/g, "gagal=")
      .replace(/\bbatch_titles=/g, "judul_batch=")
      .replace(/\bstatus=/g, "status=")
      .replace(/\bcategory_id=/g, "kategori_id=")
      .replace(/\bper_category=/g, "per_kategori=")
      .replace(/\bauthor_id=/g, "penulis_id=")
      .replace(/\binterval_min=/g, "interval_menit=")
      .replace(/\bgenerate_delay_min=/g, "jeda_generate_menit=")
      .replace(/\bslug=/g, "slug=")
      .replace(/\bpurpose=cover\b/g, "tujuan=sampul")
      .replace(/\bpurpose=content\b/g, "tujuan=konten")
      .replace(/\bprovider=/g, "penyedia=")
      .replace(/\bmodel=/g, "model=")
      .replace(/\burl=/g, "url=")
      .replace(/\berr=/g, "error=")
      .replace(/\bempty response\b/gi, "respons kosong");
  }

  function buildListURL(page, size, q, ctx) {
    var p = new URLSearchParams();
    p.set("page", String(page));
    p.set("size", String(size));
    if (q) p.set("q", q);
    var lv = ctx.levelFilterEl;
    if (lv && lv.value) p.set("level", lv.value);
    var evIn = ctx.eventFilterEl;
    var ev = evIn && evIn.value ? String(evIn.value).trim() : "";
    if (ev) p.set("event", ev);
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
        tbody: "#bulk-post-logs-tbody",
        error: "#bulk-post-logs-error",
        pageInfo: "#bulk-post-logs-page-info",
        pageSize: "#bulk-post-logs-page-size",
        prev: "#bulk-post-logs-prev-btn",
        next: "#bulk-post-logs-next-btn",
        searchInput: "#bulk-post-logs-search",
      },
      buildListURL: buildListURL,
      parseTotalPages: parseTotalPages,
      messages: {
        loadFailed: "Gagal memuat log bulk post",
        unexpectedResponse: "Respons server tidak sesuai.",
        network: "Terjadi gangguan jaringan. Coba lagi.",
        pageOf: function (a, b) {
          return "Halaman " + a + " dari " + b;
        },
        noItems: "Tidak ada aktivitas bulk post di halaman ini.",
      },
      renderEmpty: function (tbody) {
        if (!tbody) return;
        var tr = document.createElement("tr");
        var td = document.createElement("td");
        td.colSpan = 9;
        td.className = "text-secondary py-4 text-center small";
        td.textContent = "Tidak ada aktivitas bulk post di halaman ini.";
        tr.appendChild(td);
        tbody.appendChild(tr);
      },
      renderRow: function (row) {
        var tr = document.createElement("tr");
        var t1 = document.createElement("td");
        t1.className = "text-nowrap small";
        t1.textContent = formatTime(row.created_at);
        var tUser = document.createElement("td");
        tUser.className = "small text-break";
        var disp =
          row.user_display != null && String(row.user_display).trim() !== ""
            ? String(row.user_display)
            : row.user_id != null && row.user_id !== ""
              ? "#" + row.user_id
              : "—";
        tUser.textContent = disp;
        var tLv = document.createElement("td");
        tLv.className = "small";
        tLv.innerHTML = levelBadge(row.level);
        var tEv = document.createElement("td");
        tEv.className = "small font-monospace text-break";
        tEv.textContent = translateEvent(row.event);
        var tProvUsed = document.createElement("td");
        tProvUsed.className = "small text-nowrap";
        tProvUsed.innerHTML = aiProviderBadge(
          row.provider_used != null ? row.provider_used : ""
        );
        var tMsg = document.createElement("td");
        tMsg.className = "small text-break";
        tMsg.textContent = translateLogMessage(row.message);
        var tCat = document.createElement("td");
        tCat.className = "small text-break";
        tCat.innerHTML = categoryCell(
          row.category_name != null && String(row.category_name).trim() !== ""
            ? row.category_name
            : row.category_id != null && row.category_id !== ""
              ? "#" + row.category_id
              : ""
        );
        var tPost = document.createElement("td");
        tPost.className = "small";
        tPost.innerHTML =
          row.post_id != null && row.post_id !== ""
            ? postBadge(row.post_id)
            : "—";
        var tTitle = document.createElement("td");
        tTitle.className = "small text-break";
        tTitle.textContent =
          row.title != null && String(row.title).trim() !== ""
            ? String(row.title)
            : "—";
        tr.appendChild(t1);
        tr.appendChild(tUser);
        tr.appendChild(tLv);
        tr.appendChild(tEv);
        tr.appendChild(tProvUsed);
        tr.appendChild(tMsg);
        tr.appendChild(tCat);
        tr.appendChild(tPost);
        tr.appendChild(tTitle);
        return tr;
      },
      afterBind: function (ctx) {
        ctx.levelFilterEl = qs("#bulk-post-logs-level-filter");
        ctx.eventFilterEl = qs("#bulk-post-logs-event-filter");
        function reloadFromFilter() {
          ctx.state.page = 1;
          ctx.loadList();
        }
        if (ctx.levelFilterEl) {
          ctx.levelFilterEl.addEventListener("change", reloadFromFilter);
        }
        if (ctx.eventFilterEl) {
          var t = null;
          function schedule() {
            if (t) clearTimeout(t);
            t = setTimeout(function () {
              t = null;
              reloadFromFilter();
            }, 350);
          }
          ctx.eventFilterEl.addEventListener("input", schedule);
          ctx.eventFilterEl.addEventListener("change", function () {
            if (t) clearTimeout(t);
            reloadFromFilter();
          });
        }
        var ref = qs("#bulk-post-logs-refresh");
        if (ref) {
          ref.addEventListener("click", function () {
            ctx.loadList();
          });
        }
        var clearBtn = qs("#bulk-post-logs-clear");
        if (clearBtn) {
          clearBtn.addEventListener("click", async function () {
            var ok = await AC.confirmAction({
              title: "Bersihkan semua log bulk post?",
              text: "Tindakan ini akan menghapus permanen semua baris log bulk post.",
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
              ctx.showSuccessToast((data.deleted || 0) + " log bulk post berhasil dihapus.");
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
