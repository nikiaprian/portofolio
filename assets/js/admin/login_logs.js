(function () {
  "use strict";

  var LOGS_API = "/api/v1/admin/login-logs";
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

  function reasonBadge(success, reason) {
    var r = reason != null ? String(reason).trim() : "";
    if (!r) return "—";
    r = translateReason(r);
    if (success) {
      return (
        '<span class="badge text-bg-secondary">' +
        r.replace(/</g, "&lt;").replace(/>/g, "&gt;") +
        "</span>"
      );
    }
    return (
      '<span class="badge text-bg-danger">' +
      r.replace(/</g, "&lt;").replace(/>/g, "&gt;") +
      "</span>"
    );
  }

  function translateReason(reason) {
    var raw = String(reason || "").trim();
    var key = raw.toLowerCase();
    var map = {
      "invalid email or password": "Email atau kata sandi tidak valid",
      "lookup failed": "Gagal mencari akun",
      "account disabled": "Akun dinonaktifkan",
      "token generation failed": "Gagal membuat token",
      success: "Berhasil",
      failed: "Gagal",
    };
    return map[key] || raw;
  }

  function truncate(s, n) {
    if (s == null) return "";
    s = String(s);
    if (s.length <= n) return s;
    return s.slice(0, n) + "…";
  }

  function buildListURL(page, size, q, ctx) {
    var p = new URLSearchParams();
    p.set("page", String(page));
    p.set("size", String(size));
    if (q) p.set("q", q);
    var sf = ctx.successFilterEl;
    if (sf && sf.value !== "") p.set("success", sf.value);
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
        tbody: "#login-logs-tbody",
        error: "#login-logs-error",
        pageInfo: "#login-logs-page-info",
        pageSize: "#login-logs-page-size",
        prev: "#login-logs-prev-btn",
        next: "#login-logs-next-btn",
        searchInput: "#login-logs-search",
      },
      buildListURL: buildListURL,
      parseTotalPages: parseTotalPages,
      messages: {
        loadFailed: "Gagal memuat log login",
        unexpectedResponse: "Respons server tidak sesuai.",
        network: "Terjadi gangguan jaringan. Coba lagi.",
        pageOf: function (a, b) {
          return "Halaman " + a + " dari " + b;
        },
        noItems: "Tidak ada aktivitas login di halaman ini.",
      },
      renderEmpty: function (tbody) {
        if (!tbody) return;
        var tr = document.createElement("tr");
        var td = document.createElement("td");
        td.colSpan = 7;
        td.className = "text-secondary py-4 text-center small";
        td.textContent = "Tidak ada aktivitas login di halaman ini.";
        tr.appendChild(td);
        tbody.appendChild(tr);
      },
      renderRow: function (row) {
        var tr = document.createElement("tr");
        var t1 = document.createElement("td");
        t1.className = "text-nowrap small";
        t1.textContent = formatTime(row.created_at);
        var tEmail = document.createElement("td");
        tEmail.className = "small text-break";
        tEmail.textContent = row.email != null ? String(row.email) : "—";
        var tOk = document.createElement("td");
        tOk.className = "small";
        if (row.success) {
          tOk.innerHTML =
            '<span class="badge text-bg-success">Berhasil</span>';
        } else {
          tOk.innerHTML = '<span class="badge text-bg-danger">Gagal</span>';
        }
        var tUid = document.createElement("td");
        tUid.className = "small font-monospace";
        tUid.textContent =
          row.user_id != null && row.user_id !== ""
            ? "#" + row.user_id
            : "—";
        var tReason = document.createElement("td");
        tReason.className = "small text-break";
        tReason.innerHTML = reasonBadge(
          !!row.success,
          row.failure_reason != null ? row.failure_reason : ""
        );
        var tIp = document.createElement("td");
        tIp.className = "small font-monospace text-break";
        tIp.textContent =
          row.ip != null && String(row.ip).trim() !== ""
            ? String(row.ip)
            : "—";
        var tUa = document.createElement("td");
        tUa.className = "small text-break";
        tUa.textContent = truncate(row.user_agent, 120);
        tUa.title = row.user_agent != null ? String(row.user_agent) : "";
        tr.appendChild(t1);
        tr.appendChild(tEmail);
        tr.appendChild(tOk);
        tr.appendChild(tUid);
        tr.appendChild(tReason);
        tr.appendChild(tIp);
        tr.appendChild(tUa);
        return tr;
      },
      afterBind: function (ctx) {
        ctx.successFilterEl = qs("#login-logs-success-filter");
        function reloadFromFilter() {
          ctx.state.page = 1;
          ctx.loadList();
        }
        if (ctx.successFilterEl) {
          ctx.successFilterEl.addEventListener("change", reloadFromFilter);
        }
        var ref = qs("#login-logs-refresh");
        if (ref) {
          ref.addEventListener("click", function () {
            ctx.loadList();
          });
        }
        var clearBtn = qs("#login-logs-clear");
        if (clearBtn) {
          clearBtn.addEventListener("click", async function () {
            var ok = await AC.confirmAction({
              title: "Bersihkan semua log login?",
              text: "Tindakan ini akan menghapus permanen semua baris log login.",
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
              ctx.showSuccessToast((data.deleted || 0) + " log login berhasil dihapus.");
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
