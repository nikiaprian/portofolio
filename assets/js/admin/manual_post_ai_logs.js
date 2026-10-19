(function () {
  "use strict";

  var LOGS_API = "/api/v1/admin/manual-post-ai-logs";
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

  function providerBadge(code) {
    var c = (code || "").toLowerCase().trim();
    if (!c) return "—";
    var cls = "text-bg-secondary";
    if (c === "gemini") cls = "text-bg-primary";
    else if (c === "openai") cls = "text-bg-success";
    else if (c === "groq") cls = "text-bg-warning text-dark";
    else if (c === "claude") cls = "text-bg-dark";
    else if (c === "auto") cls = "text-bg-secondary";
    else cls = "text-bg-light text-dark border";
    return '<span class="badge ' + cls + '">' + escapeHtml(c) + "</span>";
  }

  function levelBadge(level) {
    var l = (level || "").toLowerCase();
    if (l === "error") return '<span class="badge text-bg-danger">error</span>';
    if (l === "warn" || l === "warning") {
      return '<span class="badge text-bg-warning text-dark">warn</span>';
    }
    if (l === "info") return '<span class="badge text-bg-info text-dark">info</span>';
    return '<span class="badge text-bg-secondary">' + escapeHtml(level || "—") + "</span>";
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
      generate_draft_ok: "draf_berhasil",
      generate_draft_err: "draf_gagal",
      generate_image_ok: "gambar_berhasil",
      generate_image_err: "gambar_gagal",
    };
    return map[raw] || raw || "—";
  }

  function translateLogMessage(message) {
    var s = message != null ? String(message).trim() : "";
    if (!s) return "";
    return s
      .replace(/\bai_requests=/g, "permintaan_ai=")
      .replace(/\bword_count=/g, "jumlah_kata=")
      .replace(/\bpurpose=cover\b/g, "tujuan=sampul")
      .replace(/\bpurpose=content\b/g, "tujuan=konten")
      .replace(/\bprovider=/g, "penyedia=")
      .replace(/\bmodel=/g, "model=")
      .replace(/\burl=/g, "url=")
      .replace(/\berr=/g, "error=")
      .replace(/\bempty response\b/gi, "respons kosong")
      .replace(/\bno web image found\b/gi, "gambar web tidak ditemukan");
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
        tbody: "#manual-post-ai-logs-tbody",
        error: "#manual-post-ai-logs-error",
        pageInfo: "#manual-post-ai-logs-page-info",
        pageSize: "#manual-post-ai-logs-page-size",
        prev: "#manual-post-ai-logs-prev-btn",
        next: "#manual-post-ai-logs-next-btn",
        searchInput: "#manual-post-ai-logs-search",
      },
      buildListURL: buildListURL,
      parseTotalPages: parseTotalPages,
      messages: {
        loadFailed: "Gagal memuat log AI post manual",
        unexpectedResponse: "Respons server tidak sesuai.",
        network: "Terjadi gangguan jaringan. Coba lagi.",
        pageOf: function (a, b) {
          return "Halaman " + a + " dari " + b;
        },
        noItems: "Tidak ada aktivitas AI post manual di halaman ini.",
      },
      renderEmpty: function (tbody) {
        if (!tbody) return;
        var tr = document.createElement("tr");
        var td = document.createElement("td");
        td.colSpan = 9;
        td.className = "text-secondary py-4 text-center small";
        td.textContent = "Tidak ada aktivitas AI post manual di halaman ini.";
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

        var tProvIn = document.createElement("td");
        tProvIn.className = "small text-nowrap";
        tProvIn.innerHTML = providerBadge(row.provider_input);

        var tProvUsed = document.createElement("td");
        tProvUsed.className = "small text-nowrap";
        tProvUsed.innerHTML = providerBadge(row.provider_used);

        var tMsg = document.createElement("td");
        tMsg.className = "small text-break";
        tMsg.textContent = translateLogMessage(row.message);

        var tPost = document.createElement("td");
        tPost.className = "small";
        tPost.innerHTML = row.post_id != null && row.post_id !== "" ? postBadge(row.post_id) : "—";

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
        tr.appendChild(tProvIn);
        tr.appendChild(tProvUsed);
        tr.appendChild(tMsg);
        tr.appendChild(tPost);
        tr.appendChild(tTitle);
        return tr;
      },
      afterBind: function (ctx) {
        ctx.levelFilterEl = qs("#manual-post-ai-logs-level-filter");
        ctx.eventFilterEl = qs("#manual-post-ai-logs-event-filter");
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
        var ref = qs("#manual-post-ai-logs-refresh");
        if (ref) {
          ref.addEventListener("click", function () {
            ctx.loadList();
          });
        }
        var clearBtn = qs("#manual-post-ai-logs-clear");
        if (clearBtn) {
          clearBtn.addEventListener("click", async function () {
            var ok = await AC.confirmAction({
              title: "Bersihkan semua log AI post manual?",
              text: "Tindakan ini akan menghapus permanen semua baris log AI post manual.",
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
              ctx.showSuccessToast((data.deleted || 0) + " log AI post manual berhasil dihapus.");
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
