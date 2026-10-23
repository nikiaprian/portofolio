/**
 * Shared admin CRUD for API resources with { name, slug, id, updated_at } (categories, tags).
 * Page must load after crud.js (defer). Call AdminNameSlugCrud.bind({ ... }) from category.js / tag.js.
 */
(function (global) {
  "use strict";

  /** Match API slug rule: lowercase, digits, single hyphens between segments. */
  function slugify(str) {
    var s = String(str || "")
      .toLowerCase()
      .normalize("NFD")
      .replace(/[\u0300-\u036f]/g, "");
    s = s.replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
    return s;
  }

  /**
   * Keeps slug in sync with name until the user edits the slug field.
   * @returns {{ reset: function() }}
   */
  function wireSlugAutofill(nameInput, slugInput) {
    if (!nameInput || !slugInput) {
      return { reset: function () {} };
    }
    var slugTouched = false;
    function onNameInput() {
      if (slugTouched) return;
      slugInput.value = slugify(nameInput.value);
    }
    function onSlugInput() {
      slugTouched = true;
    }
    nameInput.addEventListener("input", onNameInput);
    slugInput.addEventListener("input", onSlugInput);
    return {
      reset: function () {
        slugTouched = false;
      },
    };
  }

  /**
   * @param {object} cfg
   * @param {string} cfg.apiBase e.g. "/api/v1/admin/categories"
   * @param {string} cfg.listPrefix e.g. "categories" → #categories-tbody, #categories-error, …
   * @param {object} cfg.ids - DOM ids for modals/forms (see category.js)
   * @param {object} cfg.messages - user-facing strings
   * @param {string} cfg.emptyIcon - bootstrap-icons class without "bi " prefix e.g. "bi-folder2"
   * @param {boolean} [cfg.slugAutofill=true] - create modal: slug follows name until slug is edited (edit modal: manual only)
   */
  function bind(cfg) {
    var AC = global.AdminCrud;
    if (!AC) return;

    var API = cfg.apiBase.replace(/\/+$/, "");
    var p = cfg.listPrefix;
    var ids = cfg.ids || {};
    var msg = cfg.messages || {};
    var emptyIcon = cfg.emptyIcon || "bi-inbox";
    var enableDescription = !!cfg.enableDescription;
    // Category list always needs title + keyword columns (must match server thead).
    // Also treat as enabled when listPrefix is "categories" so older cached category.min.js
    // (without enableCategorySEO in bind cfg) still renders and saves the extra fields.
    var enableCategorySEO =
      !!cfg.enableCategorySEO || String(p || "").toLowerCase() === "categories";
    var enableCategoryTitle = enableCategorySEO && cfg.enableCategoryTitle !== false;
    var enableCategorySort =
      String(p || "").toLowerCase() === "categories" && cfg.enableCategorySort !== false;
    var enableCategoryDragSort =
      String(p || "").toLowerCase() === "categories" &&
      cfg.enableCategoryDragSort === true;
    var enableCategoryParent =
      !!cfg.enableCategoryParent ||
      (String(p || "").toLowerCase() === "categories" && cfg.enableCategoryParent !== false);
    var publicHrefForItem =
      cfg && typeof cfg.publicHrefForItem === "function"
        ? cfg.publicHrefForItem
        : null;
    var rowCacheByID = Object.create(null);

    var modalCreateEl = document.getElementById(ids.createModal);
    var modalEditEl = document.getElementById(ids.editModal);
    var modalDeleteEl = document.getElementById(ids.deleteModal);
    var deleteItemId = null;

    var createAutofill = { reset: function () {} };

    function itemUrl(id) {
      return AC.itemUrl(API, id);
    }

    function el(id) {
      return document.getElementById(id);
    }

    /** Fallback when cfg.ids omits title/keyword (e.g. cached category.min.js). */
    function categorySeoField(which, part) {
      if (String(p || "").toLowerCase() !== "categories") return null;
      var bid =
        which === "create"
          ? part === "title"
            ? "cc-title"
            : "cc-keyword"
          : part === "title"
            ? "ce-title"
            : "ce-keyword";
      return document.getElementById(bid);
    }

    function categoryParentField(which) {
      if (!enableCategoryParent) return null;
      if (which === "create") {
        return (ids.createParent && el(ids.createParent)) || el("cc-parent");
      }
      return (ids.editParent && el(ids.editParent)) || el("ce-parent");
    }

    function categoryRootsForParentSelect(excludeId) {
      var roots = [];
      Object.keys(rowCacheByID).forEach(function (key) {
        var c = rowCacheByID[key];
        if (!c || c.id == null) return;
        if (excludeId != null && String(c.id) === String(excludeId)) return;
        if (c.parent_id != null && Number(c.parent_id) > 0) return;
        roots.push(c);
      });
      roots.sort(function (a, b) {
        var na = (a && a.name) || "";
        var nb = (b && b.name) || "";
        return na.localeCompare(nb, undefined, { sensitivity: "base" });
      });
      return roots;
    }

    function fillParentSelect(selectEl, excludeId, selectedParentId) {
      if (!selectEl) return;
      var prev = selectedParentId != null ? String(selectedParentId) : "";
      selectEl.innerHTML = "";
      var ph = document.createElement("option");
      ph.value = "";
      ph.textContent = "— Kategori utama (tanpa induk) —";
      selectEl.appendChild(ph);
      categoryRootsForParentSelect(excludeId).forEach(function (c) {
        var o = document.createElement("option");
        o.value = String(c.id);
        o.textContent = c.name || c.slug || "#" + c.id;
        if (prev && String(c.id) === prev) o.selected = true;
        selectEl.appendChild(o);
      });
      if (prev && !selectEl.value) {
        var missing = document.createElement("option");
        missing.value = prev;
        missing.textContent = "Kategori induk #" + prev;
        missing.selected = true;
        selectEl.appendChild(missing);
      }
    }

    function categoryHasChildren(categoryId) {
      if (categoryId == null || categoryId === "") return false;
      var found = false;
      Object.keys(rowCacheByID).forEach(function (key) {
        var c = rowCacheByID[key];
        if (!c || c.parent_id == null) return;
        if (String(c.parent_id) === String(categoryId)) found = true;
      });
      return found;
    }

    function readParentPayload(selectEl) {
      if (!selectEl) return {};
      var raw = (selectEl.value || "").trim();
      if (raw === "") return { clear_parent: true };
      var pid = parseInt(raw, 10);
      if (!pid || pid < 1) return {};
      return { parent_id: pid };
    }

    function categoryDeleteSeoEl(part) {
      if (String(p || "").toLowerCase() !== "categories") return null;
      return document.getElementById(
        part === "title" ? "cd-title" : "cd-keyword"
      );
    }

    AC.bindPage({
      tokenKey: cfg.tokenKey || "tirtayasa_token",
      loginUrl:
        cfg.loginUrl ||
        (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) ||
        "/login",
      initialPageSize: cfg.initialPageSize || 10,
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
        loadFailed: msg.loadFailed || "Gagal memuat data",
        unexpectedResponse:
          msg.unexpectedResponse || "Respons server tidak sesuai.",
        network: msg.network || "Terjadi gangguan jaringan. Coba lagi.",
        pageOf: function (a, b) {
          if (typeof msg.pageOf === "function") return msg.pageOf(a, b);
          return "Halaman " + a + " dari " + b;
        },
        noItems: msg.noItems || "Data tidak ditemukan",
      },
      bulkActions: {
        confirmTitle: msg.bulkDeleteTitle || "Hapus data yang dipilih?",
        confirmText: function (count) {
          if (typeof msg.bulkDeleteText === "function") return msg.bulkDeleteText(count);
          return "Tindakan ini akan menghapus permanen " + count + " data yang dipilih.";
        },
        successText: function (deleted, failed) {
          if (typeof msg.bulkDeleteSuccess === "function") return msg.bulkDeleteSuccess(deleted, failed);
          if (failed > 0) {
            return deleted + " data terhapus, " + failed + " gagal.";
          }
          return deleted + " data berhasil dihapus.";
        },
        failedPrefix: msg.deleteFailed || "Gagal menghapus",
        deleteItem: async function (id, helpers) {
          var res = await helpers.fetchJSON("DELETE", itemUrl(id), null);
          if (res.ok) return null;
          return AC.responseErrorMessage(res, msg.deleteFailed || "Gagal menghapus");
        },
      },
      renderEmpty: function (tbody) {
        if (!tbody) return;
        var tr = document.createElement("tr");
        var td = document.createElement("td");
        var colSpan = enableDescription ? 6 : 5;
        if (enableCategorySEO) colSpan += enableCategoryTitle ? 2 : 1;
        if (enableCategorySort) colSpan += 1;
        td.colSpan = colSpan;
        td.className = "text-secondary py-5 text-center";
        td.innerHTML =
          '<div class="d-flex flex-column align-items-center gap-2">' +
          '<i class="bi ' +
          emptyIcon +
          ' fs-2 opacity-25"></i>' +
          "<span>" +
          (msg.emptyPage || "Belum ada data.") +
          "</span></div>";
        tr.appendChild(td);
        tbody.appendChild(tr);
      },
      renderRow: function (item) {
        var id = item && item.id != null ? item.id : "";
        if (id !== "") rowCacheByID[String(id)] = item || {};
        var tr = document.createElement("tr");
        tr.setAttribute("data-item-id", String(id));

        if (enableCategoryDragSort) {
          tr.classList.add("admin-crud-row-draggable");
          tr.draggable = true;
        }

        var tdDrag = null;
        if (enableCategoryDragSort) {
          tdDrag = document.createElement("td");
          tdDrag.className = "admin-crud-meta text-center";
          var dragTitle = msg.dragReorderTitle || "Geser untuk mengatur ulang";
          tdDrag.innerHTML =
            '<button type="button" class="btn btn-sm btn-light border admin-crud-drag-handle" title="' +
            dragTitle +
            '" aria-label="' +
            dragTitle +
            '">' +
            '<i class="bi bi-grip-vertical"></i>' +
            "</button>";
        }

        var tdMain = document.createElement("td");
        var rowFlex = document.createElement("div");
        rowFlex.className = "d-flex align-items-center gap-3";
        var av = document.createElement("div");
        av.className = "admin-crud-avatar";
        av.setAttribute("aria-hidden", "true");
        av.textContent = AC.initials(item && item.name, item && item.slug);
        var textCol = document.createElement("div");
        textCol.className = "min-w-0 flex-grow-1";
        var nameLine = document.createElement("div");
        nameLine.className = "d-flex flex-wrap align-items-center gap-2";
        var pubHref = "";
        try {
          pubHref = publicHrefForItem ? String(publicHrefForItem(item) || "") : "";
        } catch (e) {
          pubHref = "";
        }
        var nameEl = document.createElement(pubHref ? "a" : "span");
        nameEl.className = "fw-semibold text-body";
        nameEl.textContent = item && item.name ? item.name : "—";
        if (pubHref) {
          nameEl.href = pubHref;
          nameEl.target = "_blank";
          nameEl.rel = "noopener";
          nameEl.className += " text-decoration-none admin-crud-title-link";
        }
        var idPill = document.createElement("span");
        idPill.className =
          "badge rounded-pill bg-body-secondary text-body-secondary border small fw-normal";
        idPill.textContent = "#" + id;
        nameLine.appendChild(nameEl);
        nameLine.appendChild(idPill);
        if (enableCategoryParent && item && item.parent_id != null && Number(item.parent_id) > 0) {
          var parentRow = rowCacheByID[String(item.parent_id)];
          var subBadge = document.createElement("span");
          subBadge.className =
            "badge rounded-pill bg-info-subtle text-info-emphasis border small fw-normal";
          subBadge.textContent = parentRow && parentRow.name
            ? "Sub: " + parentRow.name
            : "Subkategori";
          nameLine.appendChild(subBadge);
        }
        textCol.appendChild(nameLine);
        rowFlex.appendChild(av);
        rowFlex.appendChild(textCol);
        tdMain.appendChild(rowFlex);

        var tdSlug = document.createElement("td");
        tdSlug.className = "admin-crud-meta font-monospace small";
        tdSlug.textContent = item && item.slug ? item.slug : "—";

        var tdTitle = null;
        var tdKeyword = null;
        if (enableCategorySEO && enableCategoryTitle) {
          tdTitle = document.createElement("td");
          tdTitle.className = "admin-crud-meta small";
          tdTitle.textContent =
            item && item.title ? String(item.title).trim() : "—";
        }
        if (enableCategorySEO) {
          tdKeyword = document.createElement("td");
          tdKeyword.className = "admin-crud-meta small";
          tdKeyword.textContent =
            item && item.keyword ? String(item.keyword).trim() : "—";
        }

        var tdDescription = null;
        if (enableDescription) {
          tdDescription = document.createElement("td");
          tdDescription.className = "admin-crud-meta small";
          var rawDesc = item && item.description ? String(item.description).trim() : "";
          tdDescription.textContent = rawDesc || "—";
        }

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
        var editBtn = document.createElement("button");
        editBtn.type = "button";
        editBtn.className =
          "btn btn-sm btn-light border admin-crud-btn-edit me-1";
        editBtn.setAttribute("data-item-id", String(id));
        editBtn.setAttribute("title", "Edit");
        editBtn.innerHTML = '<i class="bi bi-pencil"></i>';
        var delBtn = document.createElement("button");
        delBtn.type = "button";
        delBtn.className =
          "btn btn-sm btn-light border text-danger admin-crud-btn-delete";
        delBtn.setAttribute("data-item-id", String(id));
        delBtn.setAttribute("title", "Hapus");
        delBtn.innerHTML = '<i class="bi bi-trash"></i>';
        tdActions.appendChild(editBtn);
        tdActions.appendChild(delBtn);

        if (tdDrag) tr.appendChild(tdDrag);
        tr.appendChild(tdMain);
        tr.appendChild(tdSlug);
        if (enableCategorySEO && tdTitle) tr.appendChild(tdTitle);
        if (enableCategorySEO && tdKeyword) tr.appendChild(tdKeyword);
        if (enableDescription && tdDescription) tr.appendChild(tdDescription);
      tr.appendChild(tdUpdated);
        tr.appendChild(tdActions);
        return tr;
      },
      onEditClick: function (id) {
        openEditModal(id);
      },
      onDeleteClick: function (id) {
        openDeleteModal(id);
      },
      afterBind: function (ctx) {
        if (cfg.slugAutofill !== false) {
          createAutofill = wireSlugAutofill(
            el(ids.createName),
            el(ids.createSlug)
          );
        }

        if (modalCreateEl) {
          modalCreateEl.addEventListener("show.bs.modal", function () {
            createAutofill.reset();
            if (enableCategoryParent) {
              fillParentSelect(categoryParentField("create"), null, null);
            }
          });
          modalCreateEl.addEventListener("hidden.bs.modal", function () {
            var f = el(ids.formCreate);
            if (f) f.reset();
            createAutofill.reset();
            AC.setInlineError(el(ids.createError), null);
          });
        }

        var formCreate = el(ids.formCreate);
        if (formCreate) {
          formCreate.addEventListener("submit", async function (e) {
            e.preventDefault();
            AC.setInlineError(el(ids.createError), null);
            var body = {
              name: el(ids.createName).value.trim(),
              slug: el(ids.createSlug).value.trim(),
            };
            if (enableCategorySort && ids.createSort) {
              var createSortRaw = (el(ids.createSort).value || "").trim();
              if (createSortRaw !== "") body.sort = Number(createSortRaw);
            }
            var titleElC =
              (ids.createTitle && el(ids.createTitle)) ||
              categorySeoField("create", "title");
            var keyElC =
              (ids.createKeyword && el(ids.createKeyword)) ||
              categorySeoField("create", "keyword");
            if (enableCategoryTitle && titleElC) body.title = (titleElC.value || "").trim();
            if (keyElC) body.keyword = (keyElC.value || "").trim();
            if (enableDescription && ids.createDescription) {
              body.description = (el(ids.createDescription).value || "").trim();
            }
            if (enableCategoryParent) {
              var parentCreate = categoryParentField("create");
              var rawParent = parentCreate ? (parentCreate.value || "").trim() : "";
              if (rawParent !== "") {
                var pidCreate = parseInt(rawParent, 10);
                if (pidCreate > 0) body.parent_id = pidCreate;
              }
            }
            try {
              var res = await ctx.fetchJSON("POST", API, body);
              if (!res.ok) {
                AC.setInlineError(
                  el(ids.createError),
                  await AC.responseErrorMessage(
                    res,
                    msg.createFailed || "Gagal membuat data"
                  )
                );
                return;
              }
              AC.hideModal(modalCreateEl);
              AC.showSuccessToast(
                msg.createSuccess || "Data berhasil dibuat."
              );
              ctx.state.page = 1;
              ctx.loadList();
            } catch (err) {
              if (err && err.message === "unauthorized") return;
              AC.setInlineError(el(ids.createError), msg.network || "Terjadi gangguan jaringan.");
            }
          });
        }

        var formEdit = el(ids.formEdit);
        if (formEdit) {
          formEdit.addEventListener("submit", async function (e) {
            e.preventDefault();
            AC.setInlineError(el(ids.editError), null);
            var itemId = el(ids.editId).value;
            var payload = {
              name: el(ids.editName).value.trim(),
              slug: el(ids.editSlug).value.trim(),
            };
            if (enableCategorySort && ids.editSort) {
              var editSortRaw = (el(ids.editSort).value || "").trim();
              if (editSortRaw !== "") payload.sort = Number(editSortRaw);
            }
            var titleElE =
              (ids.editTitle && el(ids.editTitle)) ||
              categorySeoField("edit", "title");
            var keyElE =
              (ids.editKeyword && el(ids.editKeyword)) ||
              categorySeoField("edit", "keyword");
            if (enableCategoryTitle && titleElE) payload.title = (titleElE.value || "").trim();
            if (keyElE) payload.keyword = (keyElE.value || "").trim();
            if (enableDescription && ids.editDescription) {
              payload.description = (el(ids.editDescription).value || "").trim();
            }
            if (enableCategoryParent) {
              var parentEdit = categoryParentField("edit");
              if (parentEdit && !parentEdit.disabled) {
                var parentPayload = readParentPayload(parentEdit);
                if (parentPayload.parent_id != null) {
                  payload.parent_id = parentPayload.parent_id;
                } else if (parentPayload.clear_parent) {
                  payload.clear_parent = true;
                }
              }
            }
            try {
              var res = await ctx.fetchJSON("PUT", itemUrl(itemId), payload);
              if (!res.ok) {
                AC.setInlineError(
                  el(ids.editError),
                  await AC.responseErrorMessage(
                    res,
                    msg.updateFailed || "Gagal memperbarui data"
                  )
                );
                return;
              }
              AC.hideModal(modalEditEl);
              AC.showSuccessToast(
                msg.updateSuccess || "Data berhasil diperbarui."
              );
              ctx.loadList();
            } catch (err) {
              if (err && err.message === "unauthorized") return;
              AC.setInlineError(el(ids.editError), msg.network || "Terjadi gangguan jaringan.");
            }
          });
        }

        var tbody = document.getElementById(p + "-tbody");
        if (enableCategoryDragSort && tbody) {
          bindCategoryDragSort(ctx, tbody);
        }

        if (enableCategoryParent) {
          (async function () {
            try {
              var resAll = await ctx.fetchJSON(
                "GET",
                API + "?page=1&size=500"
              );
              if (!resAll.ok) return;
              var payload = await resAll.json();
              (payload.items || []).forEach(function (it) {
                if (it && it.id != null) rowCacheByID[String(it.id)] = it;
              });
            } catch (e) {}
          })();
        }

        var delBtn = el(ids.deleteConfirm);
        if (delBtn) {
          delBtn.addEventListener("click", async function () {
            if (!deleteItemId) return;
            AC.setInlineError(el(ids.deleteError), null);
            try {
              var res = await ctx.fetchJSON(
                "DELETE",
                itemUrl(deleteItemId),
                null
              );
              if (!res.ok) {
                AC.setInlineError(
                  el(ids.deleteError),
                  await AC.responseErrorMessage(
                    res,
                    msg.deleteFailed || "Gagal menghapus data"
                  )
                );
                return;
              }
              AC.hideModal(modalDeleteEl);
              deleteItemId = null;
              AC.showSuccessToast(
                msg.deleteSuccess || "Data berhasil dihapus."
              );
              ctx.loadList();
            } catch (err) {
              if (err && err.message === "unauthorized") return;
              AC.setInlineError(el(ids.deleteError), msg.network || "Terjadi gangguan jaringan.");
            }
          });
        }
      },
    });

    function openEditModal(itemId) {
      if (!itemId || !modalEditEl) return;
      AC.setInlineError(el(ids.editError), null);
      el(ids.editId).value = itemId;
      el(ids.editName).value = "";
      el(ids.editSlug).value = "";
      if (enableCategorySort && ids.editSort) el(ids.editSort).value = "";
      var etClear =
        (ids.editTitle && el(ids.editTitle)) ||
        categorySeoField("edit", "title");
      var ekClear =
        (ids.editKeyword && el(ids.editKeyword)) ||
        categorySeoField("edit", "keyword");
      if (etClear) etClear.value = "";
      if (ekClear) ekClear.value = "";
      if (enableDescription && ids.editDescription) {
        el(ids.editDescription).value = "";
      }
      if (enableCategoryParent) {
        fillParentSelect(categoryParentField("edit"), itemId, null);
      }
      bootstrap.Modal.getOrCreateInstance(modalEditEl).show();

      (async function () {
        try {
          var res = await fetch(itemUrl(itemId), {
            method: "GET",
            headers: {
              Authorization:
                "Bearer " + AC.getToken(cfg.tokenKey || "tirtayasa_token"),
            },
          });
          if (res.status === 401) {
            window.location.href =
              cfg.loginUrl ||
              (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) ||
              "/login";
            return;
          }
          if (!res.ok) {
            AC.setInlineError(
              el(ids.editError),
              await AC.responseErrorMessage(
                res,
                msg.loadOneFailed || "Gagal memuat data"
              )
            );
            return;
          }
          var row = await res.json();
          el(ids.editName).value = row && row.name ? row.name : "";
          el(ids.editSlug).value = row && row.slug ? row.slug : "";
          if (enableCategorySort && ids.editSort) {
            el(ids.editSort).value =
              row && Number(row.sort) > 0 ? String(Number(row.sort)) : "";
          }
          var etFill =
            (ids.editTitle && el(ids.editTitle)) ||
            categorySeoField("edit", "title");
          var ekFill =
            (ids.editKeyword && el(ids.editKeyword)) ||
            categorySeoField("edit", "keyword");
          if (enableCategoryTitle && etFill) etFill.value = row && row.title ? row.title : "";
          if (ekFill) ekFill.value = row && row.keyword ? row.keyword : "";
          if (enableDescription && ids.editDescription) {
            el(ids.editDescription).value =
              row && row.description ? row.description : "";
          }
          if (enableCategoryParent) {
            var parentSel = categoryParentField("edit");
            var selectedParent =
              row && row.parent_id != null && Number(row.parent_id) > 0
                ? row.parent_id
                : null;
            fillParentSelect(parentSel, itemId, selectedParent);
            var hasKids = categoryHasChildren(itemId);
            if (parentSel) {
              parentSel.disabled = !!hasKids;
              if (hasKids) parentSel.value = "";
            }
            var hintEl = document.getElementById("ce-parent-hint");
            if (hintEl) {
              hintEl.textContent = hasKids
                ? "Kategori ini memiliki subkategori; induk tidak dapat diubah."
                : "Subkategori hanya boleh berada di bawah satu kategori utama.";
            }
          }
        } catch (e) {
          AC.setInlineError(el(ids.editError), msg.network || "Terjadi gangguan jaringan.");
        }
      })();
    }

    function openDeleteModal(itemId) {
      if (!itemId || !modalDeleteEl) return;
      deleteItemId = itemId;
      AC.setInlineError(el(ids.deleteError), null);
      el(ids.deleteDisplayId).textContent = "";
      el(ids.deleteDisplayName).textContent = "";
      el(ids.deleteDisplaySlug).textContent = "";
      if (enableCategorySort && ids.deleteDisplaySort) {
        el(ids.deleteDisplaySort).textContent = "";
      }
      var dtClear =
        (ids.deleteDisplayTitle && el(ids.deleteDisplayTitle)) ||
        categoryDeleteSeoEl("title");
      var dkClear =
        (ids.deleteDisplayKeyword && el(ids.deleteDisplayKeyword)) ||
        categoryDeleteSeoEl("keyword");
      if (dtClear) dtClear.textContent = "";
      if (dkClear) dkClear.textContent = "";
      if (enableDescription && ids.deleteDisplayDescription) {
        el(ids.deleteDisplayDescription).textContent = "";
      }
      bootstrap.Modal.getOrCreateInstance(modalDeleteEl).show();

      (async function () {
        try {
          var res = await fetch(itemUrl(itemId), {
            method: "GET",
            headers: {
              Authorization:
                "Bearer " + AC.getToken(cfg.tokenKey || "tirtayasa_token"),
            },
          });
          if (res.status === 401) {
            window.location.href =
              cfg.loginUrl ||
              (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) ||
              "/login";
            return;
          }
          if (!res.ok) {
            AC.setInlineError(
              el(ids.deleteError),
              await AC.responseErrorMessage(
                res,
                msg.loadOneFailed || "Gagal memuat data"
              )
            );
            return;
          }
          var row = await res.json();
          el(ids.deleteDisplayId).textContent =
            row && row.id != null ? row.id : "";
          el(ids.deleteDisplayName).textContent =
            row && row.name ? row.name : "";
          el(ids.deleteDisplaySlug).textContent =
            row && row.slug ? row.slug : "";
          if (enableCategorySort && ids.deleteDisplaySort) {
            el(ids.deleteDisplaySort).textContent =
              row && Number(row.sort) > 0 ? String(Number(row.sort)) : "";
          }
          var dtFill =
            (ids.deleteDisplayTitle && el(ids.deleteDisplayTitle)) ||
            categoryDeleteSeoEl("title");
          var dkFill =
            (ids.deleteDisplayKeyword && el(ids.deleteDisplayKeyword)) ||
            categoryDeleteSeoEl("keyword");
          if (enableCategoryTitle && dtFill) dtFill.textContent = row && row.title ? row.title : "";
          if (dkFill) dkFill.textContent = row && row.keyword ? row.keyword : "";
          if (enableDescription && ids.deleteDisplayDescription) {
            el(ids.deleteDisplayDescription).textContent =
              row && row.description ? row.description : "";
          }
        } catch (e) {
          AC.setInlineError(el(ids.deleteError), msg.network || "Terjadi gangguan jaringan.");
        }
      })();
    }

    function bindCategoryDragSort(ctx, tbody) {
      var draggingRow = null;
      var isSaving = false;

      tbody.addEventListener("dragstart", function (e) {
        var row = e.target.closest("tr.admin-crud-row-draggable");
        if (!row || isSaving) return;
        if (e.target.closest("input,button,a,label,textarea,select")) {
          if (!e.target.closest(".admin-crud-drag-handle")) {
            e.preventDefault();
            return;
          }
        }
        draggingRow = row;
        row.classList.add("is-dragging");
        if (e.dataTransfer) {
          e.dataTransfer.effectAllowed = "move";
          e.dataTransfer.setData("text/plain", row.getAttribute("data-item-id") || "");
        }
      });

      tbody.addEventListener("dragend", function () {
        tbody.querySelectorAll("tr.admin-crud-row-draggable").forEach(function (r) {
          r.classList.remove("is-dragging", "is-drop-target");
        });
        draggingRow = null;
      });

      tbody.addEventListener("dragover", function (e) {
        if (!draggingRow || isSaving) return;
        var target = e.target.closest("tr.admin-crud-row-draggable");
        if (!target || target === draggingRow) return;
        e.preventDefault();
        tbody.querySelectorAll("tr.admin-crud-row-draggable").forEach(function (r) {
          if (r !== target) r.classList.remove("is-drop-target");
        });
        target.classList.add("is-drop-target");
      });

      tbody.addEventListener("drop", function (e) {
        if (!draggingRow || isSaving) return;
        var target = e.target.closest("tr.admin-crud-row-draggable");
        if (!target || target === draggingRow) return;
        e.preventDefault();
        target.classList.remove("is-drop-target");
        var targetRect = target.getBoundingClientRect();
        var dropAfter = e.clientY > targetRect.top + targetRect.height / 2;
        if (dropAfter) {
          target.parentNode.insertBefore(draggingRow, target.nextSibling);
        } else {
          target.parentNode.insertBefore(draggingRow, target);
        }
        persistCategoryDragSort(ctx, tbody).catch(function () {
          ctx.loadList();
        });
      });

      async function persistCategoryDragSort(ctx2, tbody2) {
        if (isSaving) return;
        var rows = Array.prototype.slice.call(
          tbody2.querySelectorAll("tr.admin-crud-row-draggable")
        );
        if (!rows.length) return;

        var page = Number(ctx2.state && ctx2.state.page) || 1;
        var size = Number(ctx2.state && ctx2.state.size) || rows.length;
        var startSort = (Math.max(1, page) - 1) * Math.max(1, size) + 1;
        var updates = [];

        rows.forEach(function (row, idx) {
          var id = (row.getAttribute("data-item-id") || "").trim();
          if (!id) return;
          var cached = rowCacheByID[id];
          if (!cached) return;
          var nextSort = startSort + idx;
          var curSort = Number(cached.sort) || 0;
          if (curSort !== nextSort) {
            updates.push({ id: id, item: cached, sort: nextSort });
          }
        });
        if (!updates.length) return;

        isSaving = true;
        tbody2.classList.add("admin-crud-drag-saving");
        try {
          for (var i = 0; i < updates.length; i++) {
            var u = updates[i];
            var payload = {
              name: String(u.item.name || "").trim(),
              slug: String(u.item.slug || "").trim(),
              sort: u.sort,
              title: String(u.item.title || "").trim(),
              keyword: String(u.item.keyword || "").trim(),
              description: String(u.item.description || "").trim(),
            };
            var res = await ctx2.fetchJSON("PUT", itemUrl(u.id), payload);
            if (!res.ok) {
              throw new Error("failed");
            }
            if (rowCacheByID[u.id]) rowCacheByID[u.id].sort = u.sort;
          }
          AC.showSuccessToast(msg.orderUpdated || "Urutan berhasil diperbarui.");
          ctx2.loadList();
        } finally {
          isSaving = false;
          tbody2.classList.remove("admin-crud-drag-saving");
        }
      }
    }
  }

  global.AdminNameSlugCrud = { bind: bind, slugify: slugify };
})(window);
