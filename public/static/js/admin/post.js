(function () {
  var AC = window.AdminCrud;
  if (!AC) {
    return;
  }

  var API_POSTS = "/api/v1/admin/posts";
  var API_POSTS_BULK_STATUS = "/api/v1/admin/posts/bulk-status";
  var API_CATEGORIES = "/api/v1/admin/categories";
  var API_USERS = "/api/v1/admin/users";

  var modalDeleteEl = document.getElementById("modalPostDelete");
  var deletePostId = null;

  function postUrl(id) {
    return AC.itemUrl(API_POSTS, id);
  }

  var activeFilters = {
    status: "",
    category_id: "",
    author_id: "",
  };

  function selectOption(value, label) {
    var opt = document.createElement("option");
    opt.value = String(value || "");
    opt.textContent = String(label || "");
    return opt;
  }

  function buildFilterSelect(id, placeholder) {
    var select = document.createElement("select");
    select.className = "form-select form-select-sm admin-post-filter__select";
    select.id = id;
    select.setAttribute("aria-label", placeholder);
    select.appendChild(selectOption("", placeholder));
    return select;
  }

  async function fetchFilterOptions(ctx, endpoint) {
    var url = endpoint + "?page=1&size=500";
    var res = await ctx.fetchJSON("GET", url, null);
    if (!res.ok) return [];
    var data = await res.json().catch(function () {
      return null;
    });
    return data && Array.isArray(data.items) ? data.items : [];
  }

  function scheduledTimestamp(item) {
    var raw = item && item.publish_at;
    var t = raw ? new Date(raw).getTime() : NaN;
    return isNaN(t) ? Number.MAX_SAFE_INTEGER : t;
  }

  function postUpdatedTimestamp(item) {
    var raw = (item && (item.updated_at || item.created_at)) || "";
    var t = raw ? new Date(raw).getTime() : NaN;
    return isNaN(t) ? 0 : t;
  }

  function parsePostItems(payload) {
    var items = Array.isArray(payload)
      ? payload
      : payload && Array.isArray(payload.items)
        ? payload.items
        : null;
    if (!Array.isArray(items)) return items;
    if (String(activeFilters.status || "").toLowerCase() !== "scheduled") {
      if (String(activeFilters.status || "").trim() !== "") {
        return items;
      }
      return items.slice().sort(function (a, b) {
        var aScheduled = String((a && a.status) || "").toLowerCase() === "scheduled";
        var bScheduled = String((b && b.status) || "").toLowerCase() === "scheduled";
        if (aScheduled && bScheduled) {
          var scheduledDiff = scheduledTimestamp(a) - scheduledTimestamp(b);
          if (scheduledDiff !== 0) return scheduledDiff;
        } else if (aScheduled !== bScheduled) {
          return aScheduled ? -1 : 1;
        }
        var updatedDiff = postUpdatedTimestamp(b) - postUpdatedTimestamp(a);
        if (updatedDiff !== 0) return updatedDiff;
        var aidAll = a && a.id != null ? Number(a.id) : 0;
        var bidAll = b && b.id != null ? Number(b.id) : 0;
        return bidAll - aidAll;
      });
    }
    return items.slice().sort(function (a, b) {
      var diff = scheduledTimestamp(a) - scheduledTimestamp(b);
      if (diff !== 0) return diff;
      var aid = a && a.id != null ? Number(a.id) : 0;
      var bid = b && b.id != null ? Number(b.id) : 0;
      return bid - aid;
    });
  }

  async function mountPostFilters(ctx) {
    var headerBottom = document.querySelector(".admin-crud-header__bottom");
    var searchWrap = document.querySelector(".admin-crud-search");
    if (!headerBottom || !searchWrap) return;
    if (document.getElementById("posts-filter-status")) return;
    headerBottom.classList.add("admin-crud-header__bottom--post-filters");

    var filterWrap = document.createElement("div");
    filterWrap.className = "admin-post-filters";
    filterWrap.setAttribute("aria-label", "Filter post");

    var statusSel = buildFilterSelect("posts-filter-status", "Semua status");
    statusSel.appendChild(selectOption("published", "Terbit"));
    statusSel.appendChild(selectOption("draft", "Draf"));
    statusSel.appendChild(selectOption("scheduled", "Terjadwal"));

    var categorySel = buildFilterSelect("posts-filter-category", "Semua kategori");
    var authorSel = buildFilterSelect("posts-filter-author", "Semua penulis");

    function buildFilterField(label, inputEl) {
      var field = document.createElement("label");
      field.className = "admin-post-filter";
      var text = document.createElement("span");
      text.className = "admin-post-filter__label";
      text.textContent = label;
      field.appendChild(text);
      field.appendChild(inputEl);
      return field;
    }

    filterWrap.appendChild(buildFilterField("Status", statusSel));
    filterWrap.appendChild(buildFilterField("Kategori", categorySel));
    filterWrap.appendChild(buildFilterField("Penulis", authorSel));

    headerBottom.insertBefore(filterWrap, searchWrap);

    var categories = await fetchFilterOptions(ctx, API_CATEGORIES);
    categories.forEach(function (row) {
      var id = row && row.id != null ? row.id : null;
      var name = row && row.name ? row.name : "";
      if (id == null || !name) return;
      categorySel.appendChild(selectOption(String(id), name));
    });

    var users = await fetchFilterOptions(ctx, API_USERS);
    users.forEach(function (row) {
      var id = row && row.id != null ? row.id : null;
      var name = row && row.name ? row.name : "";
      if (id == null || !name) return;
      authorSel.appendChild(selectOption(String(id), name));
    });

    function onFilterChange() {
      activeFilters.status = (statusSel.value || "").trim();
      activeFilters.category_id = (categorySel.value || "").trim();
      activeFilters.author_id = (authorSel.value || "").trim();
      ctx.state.page = 1;
      ctx.loadList();
    }
    statusSel.addEventListener("change", onFilterChange);
    categorySel.addEventListener("change", onFilterChange);
    authorSel.addEventListener("change", onFilterChange);
  }

  function consumePostSavedFlash() {
    try {
      var raw = sessionStorage.getItem("tirtayasa_post_saved_flash");
      if (!raw) return;
      sessionStorage.removeItem("tirtayasa_post_saved_flash");
      var info = JSON.parse(raw);
      var msg =
        info && info.action === "created"
          ? "Post berhasil dibuat."
          : "Post berhasil diperbarui.";
      AC.showSuccessToast(msg);
    } catch (e) {
      /* ignore */
    }
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

  async function updatePostStatus(ctx, itemId, nextStatus) {
    var resGet = await fetch(postUrl(itemId), {
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
    if (!resGet.ok) {
      throw new Error("Gagal memuat post.");
    }
    var p = await resGet.json();
    var tagIds = (p && p.tags ? p.tags : [])
      .map(function (t) {
        return t && t.id != null ? Number(t.id) : null;
      })
      .filter(function (x) {
        return x != null && !isNaN(x) && x > 0;
      });
    var payload = {
      title: p && p.title ? String(p.title) : "",
      slug: p && p.slug ? String(p.slug) : "",
      content: p && p.content ? String(p.content) : "",
      status: nextStatus,
      author_id: p && p.author_id != null ? Number(p.author_id) : 0,
      category_id: p && p.category_id != null ? Number(p.category_id) : 0,
      tags: tagIds,
    };
    if (p && p.image != null) payload.image = String(p.image);
    else payload.image = null;
    if (p && p.source != null) payload.source = String(p.source);
    else payload.source = null;
    var resPut = await ctx.fetchJSON("PUT", postUrl(itemId), payload);
    if (!resPut.ok) {
      var data = await resPut.json().catch(function () {
        return null;
      });
      throw new Error((data && data.error) || "Gagal memperbarui post (" + resPut.status + ")");
    }
  }

  AC.bindPage({
    tokenKey: "tirtayasa_token",
    loginUrl:
      (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) || "/login",
    initialPageSize: 10,
    selectors: {
      tbody: "#posts-tbody",
      error: "#posts-error",
      pageInfo: "#posts-page-info",
      pageSize: "#posts-page-size",
      prev: "#posts-prev-btn",
      next: "#posts-next-btn",
      searchInput: "#posts-search",
    },
    api: {
      list: function (page, size, q) {
        var url =
          API_POSTS +
          "?page=" +
          encodeURIComponent(page) +
          "&size=" +
          encodeURIComponent(size) +
          "&q=" +
          encodeURIComponent(q || "");
        if (activeFilters.status) {
          url += "&status=" + encodeURIComponent(activeFilters.status);
        }
        if (activeFilters.category_id) {
          url += "&category_id=" + encodeURIComponent(activeFilters.category_id);
        }
        if (activeFilters.author_id) {
          url += "&author_id=" + encodeURIComponent(activeFilters.author_id);
        }
        return url;
      },
    },
    parseItems: parsePostItems,
    messages: {
      loadFailed: "Gagal memuat post",
      unexpectedResponse: "Respons server tidak sesuai.",
      network: "Terjadi gangguan jaringan. Coba lagi.",
      pageOf: function (a, b) {
        return "Halaman " + a + " dari " + b;
      },
      noItems: "Post belum ada",
    },
    bulkActions: {
      confirmTitle: "Hapus post yang dipilih?",
      confirmText: function (count) {
        return "Tindakan ini akan menghapus permanen " + count + " post yang dipilih.";
      },
      successText: function (deleted, failed) {
        if (failed > 0) {
          return deleted + " post terhapus, " + failed + " gagal.";
        }
        return deleted + " post berhasil dihapus.";
      },
      failedPrefix: "Gagal menghapus",
      deleteItem: async function (id, helpers) {
        var res = await helpers.fetchJSON("DELETE", postUrl(id), null);
        if (res.ok) return null;
        return AC.responseErrorMessage(res, "Gagal menghapus post");
      },
    },
    renderEmpty: function (tbody) {
      if (!tbody) return;
      var tr = document.createElement("tr");
      var td = document.createElement("td");
      td.colSpan = 8;
      td.className = "text-secondary py-5 text-center";
      td.innerHTML =
        '<div class="d-flex flex-column align-items-center gap-2">' +
        '<i class="bi bi-file-earmark-text fs-2 opacity-25"></i>' +
        "<span>Tidak ada post di halaman ini.</span></div>";
      tr.appendChild(td);
      tbody.appendChild(tr);
    },
    renderRow: function (item) {
      var id = item && item.id != null ? item.id : "";
      var tr = document.createElement("tr");
      tr.setAttribute("data-indexing-status", String((item && item.indexing_status) || "not_indexed"));

      var tdTitle = document.createElement("td");
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
      var catSlug =
        item && item.category && item.category.slug
          ? String(item.category.slug).trim()
          : "";
      var pubHref = slug ? AC.publicArticleHref(slug, catSlug) : "";
      var titleEl = document.createElement(pubHref ? "a" : "span");
      titleEl.className = "fw-semibold text-body text-truncate d-inline-block";
      titleEl.style.maxWidth = "280px";
      titleEl.textContent = item && item.title ? item.title : "—";
      titleEl.title = item && item.title ? item.title : "";
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
      tdTitle.appendChild(rowFlex);

      var tdStatus = document.createElement("td");
      var badge = document.createElement("span");
      var st = (item && item.status) || "";
      badge.className = "badge rounded-pill small ";
      if (st === "published") {
        badge.className += "text-bg-success";
        badge.textContent = "Terbit";
      } else if (st === "scheduled") {
        badge.className += "text-bg-info";
        var pa = item && item.publish_at;
        badge.textContent = pa
          ? "Terjadwal · " + (AC.formatDate(pa) || String(pa))
          : "Terjadwal";
      } else {
        badge.className += "text-bg-secondary";
        badge.textContent = "Draf";
      }
      tdStatus.appendChild(badge);

      var tdCat = document.createElement("td");
      tdCat.className = "admin-crud-meta";
      tdCat.textContent =
        item && item.category && item.category.name
          ? item.category.name
          : "—";

      var tdAuthor = document.createElement("td");
      tdAuthor.className = "admin-crud-meta";
      tdAuthor.textContent =
        item && item.author && item.author.name ? item.author.name : "—";

      var tdIndexing = document.createElement("td");
      tdIndexing.className = "admin-crud-meta";
      var idxWrap = document.createElement("div");
      idxWrap.className = "d-flex align-items-center gap-1 text-nowrap";
      var idxSt = (item && item.indexing_status) || "not_indexed";
      var idxMsg = (item && item.indexing_message) || "";
      var idxTime = item && item.indexed_at ? AC.formatDate(item.indexed_at) : "";
      var idxBadge = document.createElement(idxSt === "indexed" && pubHref ? "a" : "span");
      idxBadge.className = "badge rounded-pill small ";
      if (idxSt === "indexed") {
        idxBadge.className += "bg-success-subtle text-success border border-success-subtle";
        idxBadge.textContent = "Terindeks";
        if (idxTime) idxBadge.title = "Terindeks di Google: " + idxTime;
        if (pubHref) {
          idxBadge.href = "https://www.google.com/search?q=site:" + encodeURIComponent(pubHref);
          idxBadge.target = "_blank";
          idxBadge.rel = "noopener";
          idxBadge.style.textDecoration = "none";
        }
      } else if (idxSt === "submitted") {
        idxBadge.className += "bg-info-subtle text-info border border-info-subtle";
        idxBadge.textContent = "Terkirim";
        if (idxTime) idxBadge.title = "Terkirim: " + idxTime;
      } else if (idxSt === "failed") {
        idxBadge.className += "bg-danger-subtle text-danger border border-danger-subtle";
        idxBadge.textContent = "Gagal";
        if (idxMsg) idxBadge.title = idxMsg;
      } else {
        idxBadge.className += "bg-secondary-subtle text-secondary border border-secondary-subtle";
        idxBadge.textContent = "Belum Dikirim";
      }
      idxWrap.appendChild(idxBadge);
      if (idxSt !== "indexed") {
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
      var uAt = item && item.updated_at;
      var cAt = item && item.created_at;
      var showAt = uAt;
      if (
        !showAt ||
        (typeof showAt === "string" && showAt.slice(0, 10) === "0001-01-01")
      ) {
        showAt = cAt || uAt;
      }
      t.textContent = AC.formatDate(showAt) || "—";
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
      toggleBtn.setAttribute(
        "data-status",
        isPublished ? "published" : "unpublished"
      );
      toggleBtn.setAttribute(
        "title",
        isPublished ? "Jadikan draf" : "Terbitkan"
      );
      toggleBtn.innerHTML = isPublished
        ? '<i class="bi bi-toggle-on text-success"></i>'
        : '<i class="bi bi-toggle-off text-secondary"></i>';

      var editBtn = document.createElement("a");
      editBtn.href = "/admin/posts/" + encodeURIComponent(id) + "/edit";
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

      tr.appendChild(tdTitle);
      tr.appendChild(tdStatus);
      tr.appendChild(tdCat);
      tr.appendChild(tdAuthor);
      tr.appendChild(tdIndexing);
      tr.appendChild(tdUpdated);
      tr.appendChild(tdActions);
      return tr;
    },
    onDeleteClick: function (id) {
      openDeleteModal(id);
    },
    afterBind: function (ctx) {
      consumePostSavedFlash();
      mountPostFilters(ctx);

      var tbodyEl = document.querySelector("#posts-tbody");
      if (tbodyEl) {
        tbodyEl.addEventListener("click", async function (ev) {
          var btn = ev.target.closest(".admin-crud-btn-index-gsc");
          if (!btn) return;
          var id = btn.getAttribute("data-item-id");
          if (!id) return;
          btn.disabled = true;
          var origHTML = btn.innerHTML;
          btn.innerHTML = '<span class="spinner-border spinner-border-sm" role="status"></span>';
          try {
            var res = await ctx.fetchJSON("POST", "/api/v1/admin/posts/" + encodeURIComponent(id) + "/index-gsc", {});
            var data = await res.json().catch(function () { return null; });
            if (!res.ok) {
              throw new Error((data && data.error) || "Gagal mengirim request indexing");
            }
            var msg = (data && data.message) || "Pengindeksan Google Search Console berhasil dikirim";
            if (typeof Swal !== "undefined") {
              await Swal.fire({ icon: "success", title: "Berhasil Terkirim!", text: msg, confirmButtonText: "OK" });
            } else {
              alert("Berhasil Terkirim!\n" + msg);
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

      var bulkIndexBtn = document.querySelector('[data-admin-crud-bulk-index-gsc="posts-tbody"]');
      if (bulkIndexBtn) {
        bulkIndexBtn.classList.remove("d-none");
        bulkIndexBtn.disabled = false;
        bulkIndexBtn.addEventListener("click", async function () {
          var root = document.getElementById("posts-tbody");
          var checkboxes = root ? root.querySelectorAll(".admin-crud-row-select:checked") : [];
          var ids = [];
          var skipped = 0;
          Array.prototype.slice.call(checkboxes).forEach(function (cb) {
            var tr = cb.closest("tr");
            if (tr && tr.getAttribute("data-indexing-status") === "indexed") { skipped++; return; }
            var id = (cb.getAttribute("data-item-id") || "").trim();
            if (id) ids.push(id);
          });
          if (!ids.length) {
            if (skipped > 0) AC.showSuccessToast("Semua item yang dipilih sudah terindeks.");
            return;
          }
          AC.openBulkLoadingModal("Mengirim Request Indexing GSC", 0);
          bulkIndexBtn.disabled = true;
          try {
            var res = await ctx.fetchJSON("POST", "/api/v1/admin/posts/bulk-index-gsc", {
              ids: ids.map(function (x) { return Number(x); }),
            });
            AC.closeBulkLoadingModal();
            var data = await res.json().catch(function () { return null; });
            if (!res.ok) {
              throw new Error((data && data.error) || "Gagal mengirim request indexing massal");
            }
            var msg = (data && data.message) || "Request indexing massal berhasil dikirim.";
            if (typeof Swal !== "undefined") {
              await Swal.fire({ icon: "success", title: "Berhasil Terkirim!", text: msg, confirmButtonText: "OK" });
            } else {
              alert("Berhasil Terkirim!\n" + msg);
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

      var bulkPublishBtn = document.querySelector('[data-admin-crud-bulk-publish="posts-tbody"]');
      var bulkDraftBtn = document.querySelector('[data-admin-crud-bulk-draft="posts-tbody"]');
      if (bulkPublishBtn) {
        bulkPublishBtn.classList.remove("d-none");
        bulkPublishBtn.disabled = false;
        bulkPublishBtn.addEventListener("click", async function () {
          var ids = selectedRowIdsFromTbody("posts-tbody");
          if (!ids.length) return;
          bulkPublishBtn.disabled = true;
          if (bulkDraftBtn) bulkDraftBtn.disabled = true;
          try {
            var res = await ctx.fetchJSON("POST", API_POSTS_BULK_STATUS, {
              ids: ids.map(function (x) { return Number(x); }),
              status: "published",
            });
            if (!res.ok) {
              throw new Error(await AC.responseErrorMessage(res, "Gagal menerbitkan post secara massal"));
            }
            AC.showSuccessToast(ids.length + " post berhasil diterbitkan.");
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
          var ids = selectedRowIdsFromTbody("posts-tbody");
          if (!ids.length) return;
          bulkDraftBtn.disabled = true;
          if (bulkPublishBtn) bulkPublishBtn.disabled = true;
          try {
            var res = await ctx.fetchJSON("POST", API_POSTS_BULK_STATUS, {
              ids: ids.map(function (x) { return Number(x); }),
              status: "draft",
            });
            if (!res.ok) {
              throw new Error(await AC.responseErrorMessage(res, "Gagal menjadikan post sebagai draf secara massal"));
            }
            AC.showSuccessToast(ids.length + " post berhasil dijadikan draf.");
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
      if (!window.__tirtayasaPostStatusToggleBound) {
        window.__tirtayasaPostStatusToggleBound = true;
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

          var cur =
            btn.getAttribute("data-status") === "published"
              ? "published"
              : "unpublished";
          var next = cur === "published" ? "draft" : "published";

          btn.disabled = true;
          try {
            await updatePostStatus(ctx, itemId, next);
            AC.showSuccessToast(next === "published" ? "Post berhasil diterbitkan." : "Post berhasil dijadikan draf.");
            ctx.loadList();
          } catch (err) {
            AC.showErrorToast("Terjadi gangguan jaringan.");
          } finally {
            btn.disabled = false;
          }
        }, true);
      }

      var delBtn = document.getElementById("pfd-confirm");
      if (delBtn) {
        delBtn.addEventListener("click", async function () {
          if (!deletePostId) return;
          AC.setInlineError(document.getElementById("post-delete-error"), null);
          try {
            var res = await ctx.fetchJSON(
              "DELETE",
              postUrl(deletePostId),
              null
            );
            if (!res.ok) {
              AC.setInlineError(
                document.getElementById("post-delete-error"),
                await AC.responseErrorMessage(res, "Gagal menghapus post")
              );
              return;
            }
            AC.hideModal(modalDeleteEl);
            deletePostId = null;
            AC.showSuccessToast("Post berhasil dihapus.");
            ctx.loadList();
          } catch (err) {
            if (err && err.message === "unauthorized") return;
            AC.setInlineError(
              document.getElementById("post-delete-error"),
              "Terjadi gangguan jaringan."
            );
          }
        });
      }
    },
  });

  function openDeleteModal(itemId) {
    if (!itemId || !modalDeleteEl) return;
    deletePostId = itemId;
    AC.setInlineError(document.getElementById("post-delete-error"), null);
    document.getElementById("pfd-id").textContent = "";
    document.getElementById("pfd-title").textContent = "";
    document.getElementById("pfd-slug").textContent = "";
    bootstrap.Modal.getOrCreateInstance(modalDeleteEl).show();

    (async function () {
      try {
        var res = await fetch(postUrl(itemId), {
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
            document.getElementById("post-delete-error"),
            await AC.responseErrorMessage(res, "Gagal memuat post")
          );
          return;
        }
        var row = await res.json();
        document.getElementById("pfd-id").textContent =
          row && row.id != null ? row.id : "";
        document.getElementById("pfd-title").textContent =
          row && row.title ? row.title : "";
        document.getElementById("pfd-slug").textContent =
          row && row.slug ? row.slug : "";
      } catch (e) {
        AC.setInlineError(
          document.getElementById("post-delete-error"),
          "Terjadi gangguan jaringan."
        );
      }
    })();
  }
})();
