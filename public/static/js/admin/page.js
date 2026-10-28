(function () {
  var AC = window.AdminCrud;
  if (!AC) return;

  var API = "/api/v1/admin/pages";
  var API_BULK_STATUS = "/api/v1/admin/pages/bulk-status";
  var p = "pages";
  var msg = {
    loadFailed: "Gagal memuat halaman",
    unexpectedResponse: "Respons server tidak sesuai.",
    network: "Terjadi gangguan jaringan. Coba lagi.",
    noItems: "Halaman tidak ditemukan",
    emptyPage: "Tidak ada halaman di halaman ini.",
    deleteSuccess: "Halaman berhasil dihapus.",
    deleteFailed: "Gagal menghapus halaman",
    loadOneFailed: "Gagal memuat halaman",
  };

  function el(id) {
    return document.getElementById(id);
  }

  var modalDeleteEl = el("modalPageDelete");
  var deleteItemId = null;

  function itemUrl(id) {
    return AC.itemUrl(API, id);
  }

  function consumePageSavedFlash() {
    try {
      var raw = sessionStorage.getItem("tirtayasa_page_saved_flash");
      if (!raw) return;
      sessionStorage.removeItem("tirtayasa_page_saved_flash");
      var info = JSON.parse(raw);
      var text =
        info && info.action === "created"
          ? "Halaman berhasil dibuat."
          : "Halaman berhasil diperbarui.";
      AC.showSuccessToast(text);
    } catch (e) {
      /* ignore */
    }
  }

  function statusBadgeClass(status) {
    return status === "published"
      ? "badge rounded-pill bg-success-subtle text-success-emphasis border border-success-subtle"
      : "badge rounded-pill bg-secondary-subtle text-secondary-emphasis border border-secondary-subtle";
  }

  function selectedRowIdsFromTbody(tbodyId) {
    var root = document.getElementById(tbodyId);
    if (!root) return [];
    return Array.prototype.slice
      .call(root.querySelectorAll(".admin-crud-row-select:checked"))
      .map(function (el) {
        return (el.getAttribute("data-item-id") || "").trim();
      })
      .filter(Boolean);
  }

  async function updatePageStatus(ctx, itemId, nextStatus) {
    var resGet = await fetch(itemUrl(itemId), {
      method: "GET",
      headers: {
        Authorization: "Bearer " + AC.getToken("tirtayasa_token"),
        Accept: "application/json",
      },
    });
    if (resGet.status === 401) {
      window.location.href =
        (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) || "/login";
      throw new Error("unauthorized");
    }
    if (!resGet.ok) throw new Error("Gagal memuat halaman.");
    var p = await resGet.json();
    var payload = {
      title: p && p.title ? String(p.title) : "",
      slug: p && p.slug ? String(p.slug) : "",
      content: p && p.content ? String(p.content) : "",
      status: nextStatus,
    };
    var resPut = await ctx.fetchJSON("PUT", itemUrl(itemId), payload);
    if (!resPut.ok) {
      var data = await resPut.json().catch(function () {
        return null;
      });
      throw new Error((data && data.error) || "Gagal memperbarui halaman (" + resPut.status + ")");
    }
  }

  AC.bindPage({
    tokenKey: "tirtayasa_token",
    loginUrl:
      (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) || "/login",
    initialPageSize: 10,
    selectors: {
      tbody: "#" + p + "-tbody",
      error: "#" + p + "-error",
      pageInfo: "#" + p + "-page-info",
      pageSize: "#" + p + "-page-size",
      prev: "#" + p + "-prev-btn",
      next: "#" + p + "-next-btn",
      searchInput: "#" + p + "-search",
    },
    api: {
      list: function (page, size, q) {
        return (
          API +
          "?page=" +
          encodeURIComponent(page) +
          "&size=" +
          encodeURIComponent(size) +
          "&q=" +
          encodeURIComponent(q || "")
        );
      },
    },
    messages: {
      loadFailed: msg.loadFailed,
      unexpectedResponse: msg.unexpectedResponse,
      network: msg.network,
      pageOf: function (a, b) {
        return "Halaman " + a + " dari " + b;
      },
      noItems: msg.noItems,
    },
    bulkActions: {
      confirmTitle: "Hapus halaman yang dipilih?",
      confirmText: function (count) {
        return "Tindakan ini akan menghapus permanen " + count + " halaman yang dipilih.";
      },
      successText: function (deleted, failed) {
        if (failed > 0) {
          return deleted + " halaman terhapus, " + failed + " gagal.";
        }
        return deleted + " halaman berhasil dihapus.";
      },
      failedPrefix: msg.deleteFailed,
      deleteItem: async function (id, helpers) {
        var res = await helpers.fetchJSON("DELETE", itemUrl(id), null);
        if (res.ok) return null;
        return AC.responseErrorMessage(res, msg.deleteFailed);
      },
    },
    renderEmpty: function (tbody) {
      if (!tbody) return;
      var tr = document.createElement("tr");
      var td = document.createElement("td");
      td.colSpan = 7;
      td.className = "text-secondary py-5 text-center";
      td.innerHTML =
        '<div class="d-flex flex-column align-items-center gap-2">' +
        '<i class="bi bi-folder2 fs-2 opacity-25"></i>' +
        "<span>" +
        msg.emptyPage +
        "</span></div>";
      tr.appendChild(td);
      tbody.appendChild(tr);
    },
    renderRow: function (item) {
      var id = item && item.id != null ? item.id : "";
      var tr = document.createElement("tr");
      tr.setAttribute("data-indexing-status", String((item && item.indexing_status) || "not_indexed"));

      var tdMain = document.createElement("td");
      var rowFlex = document.createElement("div");
      rowFlex.className = "d-flex align-items-center gap-3";
      var av = document.createElement("div");
      av.className = "admin-crud-avatar";
      av.setAttribute("aria-hidden", "true");
      av.textContent = AC.initials(item && item.title, item && item.slug);
      var textCol = document.createElement("div");
      textCol.className = "min-w-0 flex-grow-1";
      var nameLine = document.createElement("div");
      nameLine.className = "d-flex flex-wrap align-items-center gap-2";
      var slug = item && item.slug ? String(item.slug) : "";
      var pubHref = slug ? AC.publicArticleHref(slug) : "";
      var titleEl = document.createElement(pubHref ? "a" : "span");
      titleEl.className = "fw-semibold text-body";
      titleEl.textContent = item && item.title ? item.title : "—";
      if (pubHref) {
        titleEl.href = pubHref;
        titleEl.target = "_blank";
        titleEl.rel = "noopener";
        titleEl.className += " text-decoration-none admin-crud-title-link";
      }
      var idPill = document.createElement("span");
      idPill.className =
        "badge rounded-pill bg-body-secondary text-body-secondary border small fw-normal";
      idPill.textContent = "#" + id;
      nameLine.appendChild(titleEl);
      nameLine.appendChild(idPill);
      textCol.appendChild(nameLine);
      rowFlex.appendChild(av);
      rowFlex.appendChild(textCol);
      tdMain.appendChild(rowFlex);

      var tdSlug = document.createElement("td");
      tdSlug.className = "admin-crud-meta font-monospace small";
      tdSlug.textContent = item && item.slug ? item.slug : "—";

      var tdStatus = document.createElement("td");
      tdStatus.className = "admin-crud-meta";
      var st = item && item.status ? String(item.status) : "—";
      if (st !== "—") {
        var b = document.createElement("span");
        b.className = statusBadgeClass(st);
        b.textContent = st === "published" ? "Terbit" : st === "draft" ? "Draf" : st;
        tdStatus.appendChild(b);
      } else {
        tdStatus.textContent = "—";
      }

      var tdIndexing = document.createElement("td");
      tdIndexing.className = "admin-crud-meta";
      var idxStatus = String((item && item.indexing_status) || "not_indexed").toLowerCase();
      var idxMsg = item && item.indexing_message ? String(item.indexing_message) : "";
      var idxAt = item && item.indexed_at ? AC.formatDate(item.indexed_at) : "";
      var idxWrap = document.createElement("div");
      idxWrap.className = "d-flex align-items-center gap-1";
      var idxBadge = document.createElement(idxStatus === "indexed" && pubHref ? "a" : "span");
      idxBadge.className = "badge rounded-pill small ";
      if (idxStatus === "indexed") {
        idxBadge.className += "text-bg-success";
        idxBadge.textContent = "Terindeks";
        if (idxAt) idxBadge.title = "Terindeks di Google: " + idxAt;
        if (pubHref) {
          idxBadge.href = "https://www.google.com/search?q=site:" + encodeURIComponent(pubHref);
          idxBadge.target = "_blank";
          idxBadge.rel = "noopener";
          idxBadge.style.textDecoration = "none";
        }
      } else if (idxStatus === "submitted") {
        idxBadge.className += "text-bg-info";
        idxBadge.textContent = "Terkirim";
        if (idxAt) idxBadge.title = "Terkirim: " + idxAt;
      } else if (idxStatus === "failed") {
        idxBadge.className += "text-bg-danger";
        idxBadge.textContent = "Gagal";
        if (idxMsg) idxBadge.title = idxMsg;
      } else {
        idxBadge.className += "text-bg-secondary";
        idxBadge.textContent = "Belum Dikirim";
      }
      idxWrap.appendChild(idxBadge);
      if (idxStatus !== "indexed") {
        var indexBtn = document.createElement("button");
        indexBtn.type = "button";
        indexBtn.className = "btn btn-sm btn-light border admin-crud-btn-index-gsc ms-1 p-1 lh-1";
        indexBtn.setAttribute("data-item-id", String(id));
        indexBtn.setAttribute("title", "Kirim Request Indexing ke Google Search Console");
        indexBtn.innerHTML = '<i class="bi bi-google text-primary small"></i>';
        idxWrap.appendChild(indexBtn);
      }
      tdIndexing.appendChild(idxWrap);

      var tdUpdated = document.createElement("td");
      tdUpdated.className = "admin-crud-meta";
      var w = document.createElement("div");
      w.className = "d-flex align-items-center gap-2 text-nowrap";
      var cal = document.createElement("i");
      cal.className = "bi bi-calendar3 opacity-50";
      var t = document.createElement("span");
      t.textContent = AC.formatDate(item && item.updated_at) || "—";
      w.appendChild(cal);
      w.appendChild(t);
      tdUpdated.appendChild(w);

      var tdActions = document.createElement("td");
      tdActions.className = "text-end admin-crud-actions";

      var curStatus = String((item && item.status) || "").toLowerCase();
      var isPublished = curStatus === "published";
      var toggleBtn = document.createElement("button");
      toggleBtn.type = "button";
      toggleBtn.className =
        "btn btn-sm btn-light border admin-crud-btn-toggle-status me-1";
      toggleBtn.setAttribute("data-item-id", String(id));
      toggleBtn.setAttribute("data-status", isPublished ? "published" : "draft");
      toggleBtn.setAttribute(
        "title",
        isPublished ? "Jadikan draf" : "Terbitkan"
      );
      toggleBtn.innerHTML = isPublished
        ? '<i class="bi bi-toggle-on text-success"></i>'
        : '<i class="bi bi-toggle-off text-secondary"></i>';

      var editBtn = document.createElement("a");
      editBtn.href = "/admin/pages/" + encodeURIComponent(id) + "/edit";
      editBtn.className =
        "btn btn-sm btn-light border me-1 text-decoration-none";
      editBtn.setAttribute("title", "Edit");
      editBtn.innerHTML = '<i class="bi bi-pencil"></i>';
      var delBtn = document.createElement("button");
      delBtn.type = "button";
      delBtn.className =
        "btn btn-sm btn-light border text-danger admin-crud-btn-delete";
      delBtn.setAttribute("data-item-id", String(id));
      delBtn.setAttribute("title", "Hapus");
      delBtn.innerHTML = '<i class="bi bi-trash"></i>';
      tdActions.appendChild(toggleBtn);
      tdActions.appendChild(editBtn);
      tdActions.appendChild(delBtn);

      tr.appendChild(tdMain);
      tr.appendChild(tdSlug);
      tr.appendChild(tdStatus);
      tr.appendChild(tdIndexing);
      tr.appendChild(tdUpdated);
      tr.appendChild(tdActions);
      return tr;
    },
    onDeleteClick: function (id) {
      openDeleteModal(id);
    },
    afterBind: function (ctx) {
      consumePageSavedFlash();

      var pagesTbody = document.getElementById("pages-tbody");
      if (pagesTbody) {
        pagesTbody.addEventListener("click", async function (ev) {
          var btn = ev.target.closest(".admin-crud-btn-index-gsc");
          if (!btn) return;
          var id = btn.getAttribute("data-item-id");
          if (!id) return;
          btn.disabled = true;
          var origHTML = btn.innerHTML;
          btn.innerHTML = '<span class="spinner-border spinner-border-sm" role="status"></span>';
          try {
            var res = await ctx.fetchJSON("POST", API + "/" + encodeURIComponent(id) + "/index-gsc", {});
            var data = await res.json().catch(function () { return null; });
            if (!res.ok) throw new Error((data && data.error) || "Gagal mengirim request indexing");
            var msgStr = (data && data.message) || "Pengindeksan Google Search Console berhasil dikirim";
            if (typeof Swal !== "undefined") {
              await Swal.fire({ icon: "success", title: "Berhasil Terkirim!", text: msgStr, confirmButtonText: "OK" });
            } else {
              alert("Berhasil Terkirim!\n" + msgStr);
            }
            await ctx.loadList();
          } catch (err) {
            var errMsg = (err && err.message) || "Terjadi kesalahan saat pengiriman indexing.";
            if (typeof Swal !== "undefined") {
              await Swal.fire({ icon: "error", title: "Indexing GSC Gagal", text: errMsg, confirmButtonText: "Tutup" });
            } else {
              alert("Indexing GSC Gagal:\n" + errMsg);
            }
          } finally {
            btn.disabled = false;
            btn.innerHTML = origHTML;
          }
        });
      }

      var bulkIndexBtn = document.querySelector('[data-admin-crud-bulk-index-gsc="pages-tbody"]');
      if (bulkIndexBtn) {
        bulkIndexBtn.classList.remove("d-none");
        bulkIndexBtn.disabled = false;
        bulkIndexBtn.addEventListener("click", async function () {
          var checkboxes = pagesTbody ? pagesTbody.querySelectorAll(".admin-crud-row-select:checked") : [];
          var ids = [];
          var skipped = 0;
          Array.prototype.slice.call(checkboxes).forEach(function (cb) {
            var tr = cb.closest("tr");
            if (tr && tr.getAttribute("data-indexing-status") === "indexed") { skipped++; return; }
            var rowId = (cb.getAttribute("data-item-id") || "").trim();
            if (rowId) ids.push(Number(rowId));
          });
          if (!ids.length) {
            if (skipped > 0) AC.showSuccessToast("Semua item yang dipilih sudah terindeks.");
            return;
          }
          AC.openBulkLoadingModal("Mengirim Request Indexing GSC Halaman", ids.length);
          bulkIndexBtn.disabled = true;
          try {
            var res = await ctx.fetchJSON("POST", API + "/bulk-index-gsc", { ids: ids });
            AC.closeBulkLoadingModal();
            var data = await res.json().catch(function () { return null; });
            if (!res.ok) throw new Error((data && data.error) || "Gagal mengirim request indexing massal");
            var msgStr = (data && data.message) || "Request indexing massal berhasil dikirim.";
            if (typeof Swal !== "undefined") {
              await Swal.fire({ icon: "success", title: "Berhasil Terkirim!", text: msgStr, confirmButtonText: "OK" });
            } else {
              alert("Berhasil Terkirim!\n" + msgStr);
            }
          } catch (e) {
            AC.closeBulkLoadingModal();
            var errMsg = (e && e.message) || "Gagal meng-index secara massal.";
            if (typeof Swal !== "undefined") {
              await Swal.fire({ icon: "error", title: "Indexing GSC Gagal", text: errMsg, confirmButtonText: "Tutup" });
            } else {
              alert("Indexing GSC Gagal:\n" + errMsg);
            }
          } finally {
            bulkIndexBtn.disabled = false;
          }
          await ctx.loadList();
        });
      }

      var bulkPublishBtn = document.querySelector('[data-admin-crud-bulk-publish="pages-tbody"]');
      var bulkDraftBtn = document.querySelector('[data-admin-crud-bulk-draft="pages-tbody"]');
      if (bulkPublishBtn) {
        bulkPublishBtn.classList.remove("d-none");
        bulkPublishBtn.disabled = false;
        bulkPublishBtn.addEventListener("click", async function () {
          var ids = selectedRowIdsFromTbody("pages-tbody");
          if (!ids.length) return;
          bulkPublishBtn.disabled = true;
          if (bulkDraftBtn) bulkDraftBtn.disabled = true;
          try {
            var res = await ctx.fetchJSON("POST", API_BULK_STATUS, {
              ids: ids.map(function (x) { return Number(x); }),
              status: "published",
            });
            if (!res.ok) {
              throw new Error(await AC.responseErrorMessage(res, "Gagal menerbitkan halaman secara massal"));
            }
            AC.showSuccessToast(ids.length + " halaman berhasil diterbitkan.");
          } catch (e) {
            AC.showErrorToast((e && e.message) || "Gagal menerbitkan secara massal.");
          } finally {
            bulkPublishBtn.disabled = false;
            if (bulkDraftBtn) bulkDraftBtn.disabled = false;
          }
          await ctx.loadList();
        });
      }
      if (bulkDraftBtn) {
        bulkDraftBtn.classList.remove("d-none");
        bulkDraftBtn.disabled = false;
        bulkDraftBtn.addEventListener("click", async function () {
          var ids = selectedRowIdsFromTbody("pages-tbody");
          if (!ids.length) return;
          bulkDraftBtn.disabled = true;
          if (bulkPublishBtn) bulkPublishBtn.disabled = true;
          try {
            var res = await ctx.fetchJSON("POST", API_BULK_STATUS, {
              ids: ids.map(function (x) { return Number(x); }),
              status: "draft",
            });
            if (!res.ok) {
              throw new Error(await AC.responseErrorMessage(res, "Gagal menjadikan halaman sebagai draf secara massal"));
            }
            AC.showSuccessToast(ids.length + " halaman berhasil dijadikan draf.");
          } catch (e) {
            AC.showErrorToast((e && e.message) || "Gagal mengubah draf secara massal.");
          } finally {
            bulkDraftBtn.disabled = false;
            if (bulkPublishBtn) bulkPublishBtn.disabled = false;
          }
          await ctx.loadList();
        });
      }

      // One-time binding: quick publish/draft toggle from list table.
      if (!window.__tirtayasaPageStatusToggleBound) {
        window.__tirtayasaPageStatusToggleBound = true;
        document.addEventListener("click", async function (e) {
          var btn =
            e && e.target && e.target.closest
              ? e.target.closest(".admin-crud-btn-toggle-status")
              : null;
          if (!btn) return;
          var itemId = btn.getAttribute("data-item-id");
          if (!itemId) return;
          e.preventDefault();
          e.stopPropagation();
          if (btn.disabled) return;

          var cur = btn.getAttribute("data-status") === "published" ? "published" : "draft";
          var next = cur === "published" ? "draft" : "published";

          btn.disabled = true;
          try {
            await updatePageStatus(ctx, itemId, next);
            AC.showSuccessToast(next === "published" ? "Halaman berhasil diterbitkan." : "Halaman berhasil dijadikan draf.");
            ctx.loadList();
          } catch (err) {
            AC.showErrorToast("Terjadi gangguan jaringan.");
          } finally {
            btn.disabled = false;
          }
        }, true);
      }

      var delBtn = el("cd-confirm");
      if (delBtn) {
        delBtn.addEventListener("click", async function () {
          if (!deleteItemId) return;
          AC.setInlineError(el("page-delete-error"), null);
          try {
            var res = await ctx.fetchJSON(
              "DELETE",
              itemUrl(deleteItemId),
              null
            );
            if (!res.ok) {
              AC.setInlineError(
                el("page-delete-error"),
                await AC.responseErrorMessage(res, msg.deleteFailed)
              );
              return;
            }
            AC.hideModal(modalDeleteEl);
            deleteItemId = null;
            AC.showSuccessToast(msg.deleteSuccess);
            ctx.loadList();
          } catch (err) {
            if (err && err.message === "unauthorized") return;
            AC.setInlineError(el("page-delete-error"), msg.network);
          }
        });
      }
    },
  });

  function openDeleteModal(itemId) {
    if (!itemId || !modalDeleteEl) return;
    deleteItemId = itemId;
    AC.setInlineError(el("page-delete-error"), null);
    el("cd-id").textContent = "";
    el("cd-name").textContent = "";
    el("cd-slug").textContent = "";
    bootstrap.Modal.getOrCreateInstance(modalDeleteEl).show();

    (async function () {
      try {
        var res = await fetch(itemUrl(itemId), {
          method: "GET",
          headers: {
            Authorization: "Bearer " + AC.getToken("tirtayasa_token"),
          },
        });
        if (res.status === 401) {
          window.location.href =
        (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) || "/login";
          return;
        }
        if (!res.ok) {
          AC.setInlineError(
            el("page-delete-error"),
            await AC.responseErrorMessage(res, msg.loadOneFailed)
          );
          return;
        }
        var row = await res.json();
        el("cd-id").textContent = row && row.id != null ? row.id : "";
        el("cd-name").textContent = row && row.title ? row.title : "";
        el("cd-slug").textContent = row && row.slug ? row.slug : "";
      } catch (e) {
        AC.setInlineError(el("page-delete-error"), msg.network);
      }
    })();
  }
})();
