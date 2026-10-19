(function () {
  var AC = window.AdminCrud;
  if (!AC) return;

  var modalCreateEl = document.getElementById("modalUserCreate");
  var modalEditEl = document.getElementById("modalUserEdit");
  var modalDeleteEl = document.getElementById("modalUserDelete");
  var deleteUserId = null;

  var API_USERS = "/api/v1/admin/users";
  var DEFAULT_PROFILE_ICON =
    "data:image/svg+xml;utf8,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 64 64'%3E%3Ccircle cx='32' cy='32' r='32' fill='%23e9ecef'/%3E%3Ccircle cx='32' cy='24' r='12' fill='%2399a1aa'/%3E%3Cpath d='M12 54c3-10 12-16 20-16s17 6 20 16' fill='%2399a1aa'/%3E%3C/svg%3E";

  function currentUserId() {
    try {
      var raw = sessionStorage.getItem("bebii_user");
      if (raw) {
        var parsed = JSON.parse(raw);
        if (parsed && parsed.id != null && String(parsed.id).trim() !== "") {
          return String(parsed.id);
        }
      }
    } catch (e) {}
    try {
      var legacy = localStorage.getItem("bebii_user_id");
      if (legacy != null && String(legacy).trim() !== "") return String(legacy).trim();
    } catch (e2) {}
    return "";
  }

  function isCurrentUser(userId) {
    return String(userId || "") !== "" && String(userId) === currentUserId();
  }

  function normalizeProfileImageSrc(src) {
    var s = src != null ? String(src).trim() : "";
    if (!s) return DEFAULT_PROFILE_ICON;
    return s;
  }

  function syncCurrentUserSessionAndNavbar(updatedUser) {
    if (!updatedUser || !isCurrentUser(updatedUser.id)) return;
    try {
      sessionStorage.setItem("bebii_user", JSON.stringify(updatedUser));
    } catch (e) {}
    var navbarName = document.querySelector(".user-menu .d-none.d-md-inline");
    if (navbarName && updatedUser.name) {
      navbarName.textContent = String(updatedUser.name);
    }
    var navbarImgs = document.querySelectorAll(".user-menu img");
    if (navbarImgs && navbarImgs.length) {
      var src = normalizeProfileImageSrc(updatedUser.image);
      navbarImgs.forEach(function (img) {
        img.onerror = function () {
          img.src = DEFAULT_PROFILE_ICON;
        };
        img.src = src;
      });
    }
  }

  function showImageTab(triggerId) {
    if (typeof bootstrap === "undefined" || !triggerId) return;
    var trigger = document.getElementById(triggerId);
    if (!trigger) return;
    var tab = bootstrap.Tab.getOrCreateInstance(trigger);
    tab.show();
  }

  function normalizeImageUrlInput(input) {
    if (!input) return;
    input.type = "text";
    input.removeAttribute("pattern");
    input.setCustomValidity("");
    input.classList.remove("is-invalid");
  }

  function bindImageUrlInput(input) {
    if (!input || input.__imageUrlBound) return;
    input.__imageUrlBound = true;
    normalizeImageUrlInput(input);
    input.addEventListener("invalid", function (e) {
      e.preventDefault();
      normalizeImageUrlInput(input);
    });
  }

  function syncUserImageFieldState(prefix) {
    var uploadTab = document.getElementById(prefix + "-image-upload-tab");
    var fileInput = document.getElementById(prefix + "-image-file");
    var urlInput = document.getElementById(prefix + "-image");
    var uploadActive = !!(uploadTab && uploadTab.classList.contains("active"));

    if (fileInput) {
      fileInput.disabled = !uploadActive;
    }
    if (urlInput) {
      normalizeImageUrlInput(urlInput);
      urlInput.disabled = uploadActive;
    }
  }

  function bindUserImageTabSync(tabsId, prefix) {
    var tabs = document.getElementById(tabsId);
    if (!tabs) return;
    tabs.addEventListener("shown.bs.tab", function () {
      syncUserImageFieldState(prefix);
    });
    syncUserImageFieldState(prefix);
  }

  var ueImageRemoved = false;

  function ensureEditImageRemoveButton() {
    var preview = document.getElementById("ue-image-preview");
    if (!preview || !preview.parentElement) return;
    if (document.getElementById("ue-image-remove")) return;
    var parent = preview.parentElement;
    parent.classList.add("position-relative", "d-inline-block");
    var btn = document.createElement("button");
    btn.type = "button";
    btn.id = "ue-image-remove";
    btn.className =
      "btn btn-sm btn-secondary rounded-circle position-absolute top-0 end-0 m-1 d-inline-flex align-items-center justify-content-center";
    btn.style.width = "20px";
    btn.style.height = "20px";
    btn.style.padding = "0";
    btn.setAttribute("aria-label", "Hapus gambar pengguna");
    btn.innerHTML = '<i class="bi bi-x-lg" aria-hidden="true"></i>';
    btn.addEventListener("click", function () {
      var imageInput = document.getElementById("ue-image");
      var imageFile = document.getElementById("ue-image-file");
      ueImageRemoved = true;
      if (imageInput) imageInput.value = "";
      if (imageFile) imageFile.value = "";
      preview.src = DEFAULT_PROFILE_ICON;
    });
    parent.appendChild(btn);
  }

  async function uploadProfileImageFile(file) {
    var fd = new FormData();
    fd.append("image", file);
    var res = await fetch("/api/v1/admin/profile/upload-image", {
      method: "POST",
      headers: { Authorization: "Bearer " + AC.getToken("bebii_token") },
      body: fd,
    });
    if (res.status === 401) {
      window.location.href =
        (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login";
      throw new Error("unauthorized");
    }
    var data = await res.json().catch(function () {
      return null;
    });
    if (!res.ok) {
      throw new Error((data && data.error) || "Gagal mengunggah gambar.");
    }
    return data && data.image ? String(data.image) : "";
  }

  function userUrl(id) {
    return AC.itemUrl(API_USERS, id);
  }

  function slugifyAuthorSegment(v) {
    return String(v || "")
      .toLowerCase()
      .trim()
      .replace(/['"]/g, "")
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "");
  }

  /** Matches server authorPublicSlug: username, else email local-part (slugified). */
  function authorPublicHref(u) {
    var seg = slugifyAuthorSegment(u && u.username ? u.username : "");
    if (!seg && u && u.email) {
      var email = String(u.email).trim();
      var at = email.indexOf("@");
      if (at > 0) seg = slugifyAuthorSegment(email.slice(0, at));
    }
    if (!seg) return "";
    return "/author/" + encodeURIComponent(seg);
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

  async function updateUserEnabled(ctx, userId, nextEnabled) {
    var getRes = await fetch(userUrl(userId), {
      method: "GET",
      headers: { Authorization: "Bearer " + AC.getToken("bebii_token") },
    });
    if (getRes.status === 401) {
      window.location.href =
        (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login";
      throw new Error("unauthorized");
    }
    if (!getRes.ok) throw new Error("Gagal memuat pengguna.");
    var u = await getRes.json();
    var payload = {
      name: u && u.name ? String(u.name) : "",
      username: u && u.username ? String(u.username) : "",
      email: u && u.email ? String(u.email) : "",
      role: u && u.role ? String(u.role) : "writer",
      enabled: !!nextEnabled,
    };
    if (u && u.profile != null) payload.profile = String(u.profile);
    if (u && u.image != null) payload.image = String(u.image);
    var putRes = await ctx.fetchJSON("PUT", userUrl(userId), payload);
    if (!putRes.ok) {
      var data = await putRes.json().catch(function () {
        return null;
      });
      throw new Error((data && data.error) || "Gagal memperbarui pengguna (" + putRes.status + ")");
    }
  }

  AC.bindPage({
    tokenKey: "bebii_token",
    loginUrl:
      (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login",
    initialPageSize: 10,
    selectors: {
      tbody: "#users-tbody",
      error: "#users-error",
      pageInfo: "#users-page-info",
      pageSize: "#users-page-size",
      prev: "#users-prev-btn",
      next: "#users-next-btn",
      searchInput: "#users-search",
    },
    api: {
      list: function (page, size, q) {
        return (
          "/api/v1/admin/users?page=" +
          encodeURIComponent(page) +
          "&size=" +
          encodeURIComponent(size) +
          "&q=" +
          encodeURIComponent(q || "")
        );
      },
    },
    messages: {
      loadFailed: "Gagal memuat pengguna",
      unexpectedResponse: "Respons server tidak sesuai.",
      network: "Terjadi gangguan jaringan. Coba lagi.",
      pageOf: function (p, t) {
        return "Halaman " + p + " dari " + t;
      },
      noItems: "Pengguna tidak ditemukan",
    },
    bulkActions: {
      confirmTitle: "Hapus pengguna yang dipilih?",
      confirmText: function (count) {
        return "Tindakan ini akan menghapus permanen " + count + " pengguna yang dipilih.";
      },
      successText: function (deleted, failed) {
        if (failed > 0) {
          return deleted + " pengguna terhapus, " + failed + " gagal.";
        }
        return deleted + " pengguna berhasil dihapus.";
      },
      failedPrefix: "Gagal menghapus",
      deleteItem: async function (id, helpers) {
        var res = await helpers.fetchJSON("DELETE", userUrl(id), null);
        if (res.ok) return null;
        return AC.responseErrorMessage(res, "Gagal menghapus pengguna");
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
        '<i class="bi bi-people fs-2 opacity-25"></i>' +
        "<span>Tidak ada pengguna di halaman ini.</span></div>";
      tr.appendChild(td);
      tbody.appendChild(tr);
    },
    renderRow: function (u) {
      var id = u && u.id != null ? u.id : "";
      var tr = document.createElement("tr");

      var tdUser = document.createElement("td");
      var rowFlex = document.createElement("div");
      rowFlex.className = "d-flex align-items-center gap-3";

      var av = document.createElement("div");
      av.className = "admin-crud-avatar";
      av.setAttribute("aria-hidden", "true");
      av.textContent = AC.initials(u && u.name, u && u.email);

      var textCol = document.createElement("div");
      textCol.className = "min-w-0 flex-grow-1";

      var nameLine = document.createElement("div");
      nameLine.className = "d-flex flex-wrap align-items-center gap-2";

      var href = authorPublicHref(u);
      var nameEl = document.createElement(href ? "a" : "span");
      nameEl.className = "fw-semibold text-body";
      nameEl.textContent = u && u.name ? u.name : "—";
      if (href) {
        nameEl.href = href;
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

      var emailLine = document.createElement("div");
      emailLine.className = "small text-secondary text-truncate mt-1";
      emailLine.style.maxWidth = "min(22rem, 72vw)";
      emailLine.textContent = u && u.email ? u.email : "";

      textCol.appendChild(nameLine);
      textCol.appendChild(emailLine);
      rowFlex.appendChild(av);
      rowFlex.appendChild(textCol);
      tdUser.appendChild(rowFlex);

      var tdUsername = document.createElement("td");
      tdUsername.className = "admin-crud-meta font-monospace small";
      tdUsername.textContent =
        "@" + (u && u.username ? String(u.username) : "—");

      var tdUpdated = document.createElement("td");
      tdUpdated.className = "admin-crud-meta";
      var updatedWrap = document.createElement("div");
      updatedWrap.className = "d-flex align-items-center gap-2 text-nowrap";
      var calIcon = document.createElement("i");
      calIcon.className = "bi bi-calendar3 opacity-50";
      var updatedText = document.createElement("span");
      updatedText.textContent = AC.formatDate(u && u.updated_at) || "—";
      updatedWrap.appendChild(calIcon);
      updatedWrap.appendChild(updatedText);
      tdUpdated.appendChild(updatedWrap);

      var tdRole = document.createElement("td");
      tdRole.className = "admin-crud-meta";
      var roleBadge = document.createElement("span");
      roleBadge.className = "badge rounded-pill small text-bg-secondary";
      var role = u && u.role ? String(u.role) : "";
      roleBadge.textContent = role === "writer" ? "Penulis" : role || "—";
      tdRole.appendChild(roleBadge);

      var tdStatus = document.createElement("td");
      tdStatus.className = "admin-crud-meta";
      var enabled = !(u && u.enabled === false);
      var st = document.createElement("span");
      st.className =
        "badge rounded-pill small " + (enabled ? "text-bg-success" : "text-bg-danger");
      st.textContent = enabled ? "Aktif" : "Nonaktif";
      tdStatus.appendChild(st);

      var tdActions = document.createElement("td");
      tdActions.className = "text-end admin-crud-actions";

      var userIsSelf = isCurrentUser(id);
      var toggleBtn = null;
      if (!userIsSelf) {
        toggleBtn = document.createElement("button");
        toggleBtn.type = "button";
        toggleBtn.className =
          "btn btn-sm btn-light border admin-crud-btn-toggle-enabled me-1";
        toggleBtn.setAttribute("data-item-id", String(id));
        toggleBtn.setAttribute("data-enabled", enabled ? "1" : "0");
        toggleBtn.setAttribute("title", enabled ? "Nonaktifkan pengguna" : "Aktifkan pengguna");
        toggleBtn.innerHTML = enabled
          ? '<i class="bi bi-toggle-on text-success"></i>'
          : '<i class="bi bi-toggle-off text-danger"></i>';
      }

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

      if (toggleBtn) tdActions.appendChild(toggleBtn);
      tdActions.appendChild(editBtn);
      if (!userIsSelf) {
        tdActions.appendChild(delBtn);
      }

      tr.appendChild(tdUser);
      tr.appendChild(tdUsername);
      tr.appendChild(tdRole);
      tr.appendChild(tdStatus);
      tr.appendChild(tdUpdated);
      tr.appendChild(tdActions);
      return tr;
    },
    onEditClick: function (userId) {
      openEditModal(userId);
    },
    onDeleteClick: function (userId) {
      openDeleteModal(userId);
    },
    afterBind: function (ctx) {
      var bulkEnableBtn = document.querySelector('[data-admin-crud-bulk-enable="users-tbody"]');
      var bulkDisableBtn = document.querySelector('[data-admin-crud-bulk-disable="users-tbody"]');
      if (bulkEnableBtn) {
        bulkEnableBtn.classList.remove("d-none");
        bulkEnableBtn.disabled = false;
        bulkEnableBtn.addEventListener("click", async function () {
          var ids = selectedRowIdsFromTbody("users-tbody").filter(function (id) {
            return !isCurrentUser(id);
          });
          if (!ids.length) return;
          var confirmedEnable = await AC.confirmAction({
            title: "Aktifkan pengguna yang dipilih?",
            text: "Tindakan ini akan mengaktifkan " + ids.length + " pengguna yang dipilih.",
            confirmButtonText: "Aktifkan",
          });
          if (!confirmedEnable) return;
          var totalEnable = ids.length;
          AC.openBulkLoadingModal("Aktifkan Pengguna", totalEnable);
          bulkEnableBtn.disabled = true;
          if (bulkDisableBtn) bulkDisableBtn.disabled = true;
          var ok = 0;
          var fail = 0;
          try {
            for (var i = 0; i < ids.length; i += 1) {
              try {
                await updateUserEnabled(ctx, ids[i], true);
                ok += 1;
              } catch (e) {
                fail += 1;
              }
              var done = ok + fail;
              var pct = Math.round((done / totalEnable) * 100);
              if (done < totalEnable) {
                if (pct < 1) pct = 1;
                if (pct > 99) pct = 99;
              }
              AC.updateBulkLoadingModal(pct, "Memproses " + done + " / " + totalEnable + " pengguna…");
            }
          } finally {
            bulkEnableBtn.disabled = false;
            if (bulkDisableBtn) bulkDisableBtn.disabled = false;
          }
          AC.updateBulkLoadingModal(100, "Selesai. " + ok + " berhasil" + (fail ? ", " + fail + " gagal" : "") + ".");
          await ctx.loadList();
          AC.closeBulkLoadingModal();
          if (ok) AC.showSuccessToast(ok + " pengguna berhasil diaktifkan.");
          if (fail) AC.showErrorToast(fail + " pengguna gagal diproses.");
        });
      }
      if (bulkDisableBtn) {
        bulkDisableBtn.classList.remove("d-none");
        bulkDisableBtn.disabled = false;
        bulkDisableBtn.addEventListener("click", async function () {
          var ids = selectedRowIdsFromTbody("users-tbody").filter(function (id) {
            return !isCurrentUser(id);
          });
          if (!ids.length) return;
          var confirmedDisable = await AC.confirmAction({
            title: "Nonaktifkan pengguna yang dipilih?",
            text: "Tindakan ini akan menonaktifkan " + ids.length + " pengguna yang dipilih.",
            confirmButtonText: "Nonaktifkan",
          });
          if (!confirmedDisable) return;
          var totalDisable = ids.length;
          AC.openBulkLoadingModal("Nonaktifkan Pengguna", totalDisable);
          bulkDisableBtn.disabled = true;
          if (bulkEnableBtn) bulkEnableBtn.disabled = true;
          var ok = 0;
          var fail = 0;
          try {
            for (var i = 0; i < ids.length; i += 1) {
              try {
                await updateUserEnabled(ctx, ids[i], false);
                ok += 1;
              } catch (e) {
                fail += 1;
              }
              var done = ok + fail;
              var pct = Math.round((done / totalDisable) * 100);
              if (done < totalDisable) {
                if (pct < 1) pct = 1;
                if (pct > 99) pct = 99;
              }
              AC.updateBulkLoadingModal(pct, "Memproses " + done + " / " + totalDisable + " pengguna…");
            }
          } finally {
            bulkDisableBtn.disabled = false;
            if (bulkEnableBtn) bulkEnableBtn.disabled = false;
          }
          AC.updateBulkLoadingModal(100, "Selesai. " + ok + " berhasil" + (fail ? ", " + fail + " gagal" : "") + ".");
          await ctx.loadList();
          AC.closeBulkLoadingModal();
          if (ok) AC.showSuccessToast(ok + " pengguna berhasil dinonaktifkan.");
          if (fail) AC.showErrorToast(fail + " pengguna gagal diproses.");
        });
      }

      // Quick enable/disable toggle from list table.
      document.addEventListener("click", async function (e) {
        var btn =
          e && e.target && e.target.closest
            ? e.target.closest(".admin-crud-btn-toggle-enabled")
            : null;
        if (!btn) return;
        var userId = btn.getAttribute("data-item-id");
        if (!userId) return;
        e.preventDefault();
        e.stopPropagation();
        if (btn.disabled) return;

        var currentEnabled = btn.getAttribute("data-enabled") === "1";
        var nextEnabled = !currentEnabled;
        var confirmedSingleToggle = await AC.confirmAction({
          title: nextEnabled ? "Aktifkan pengguna ini?" : "Nonaktifkan pengguna ini?",
          text: nextEnabled
            ? "Pengguna ini akan bisa mengakses sistem."
            : "Pengguna ini tidak akan bisa mengakses sistem lagi.",
          confirmButtonText: nextEnabled ? "Aktifkan" : "Nonaktifkan",
        });
        if (!confirmedSingleToggle) return;

        btn.disabled = true;
        try {
          await updateUserEnabled(ctx, userId, nextEnabled);
          AC.showSuccessToast(nextEnabled ? "Pengguna berhasil diaktifkan." : "Pengguna berhasil dinonaktifkan.");
          ctx.loadList();
        } catch (err) {
          if (err && err.message === "unauthorized") return;
          AC.showErrorToast("Terjadi gangguan jaringan.");
        } finally {
          btn.disabled = false;
        }
      });

      if (modalCreateEl) {
        modalCreateEl.addEventListener("hidden.bs.modal", function () {
          var f = document.getElementById("form-user-create");
          if (f) f.reset();
          var ucImageFile = document.getElementById("uc-image-file");
          if (ucImageFile) ucImageFile.value = "";
          var ucImagePreview = document.getElementById("uc-image-preview");
          if (ucImagePreview) ucImagePreview.src = DEFAULT_PROFILE_ICON;
          showImageTab("uc-image-upload-tab");
          syncUserImageFieldState("uc");
          AC.setInlineError(document.getElementById("user-create-error"), null);
        });
      }

      var formCreate = document.getElementById("form-user-create");
      if (formCreate) {
        formCreate.noValidate = true;
        formCreate.addEventListener("submit", async function (e) {
          e.preventDefault();
          AC.setInlineError(
            document.getElementById("user-create-error"),
            null
          );
          var body = {
            name: document.getElementById("uc-name").value.trim(),
            username: document.getElementById("uc-username").value.trim(),
            email: document.getElementById("uc-email").value.trim(),
            password: document.getElementById("uc-password").value,
            role: document.getElementById("uc-role").value,
            enabled: !!document.getElementById("uc-enabled").checked,
            profile: document.getElementById("uc-profile").value.trim(),
            image: document.getElementById("uc-image").value.trim(),
          };
          try {
            var createImageFileInput = document.getElementById("uc-image-file");
            var createImagePreview = document.getElementById("uc-image-preview");
            var createFile =
              createImageFileInput && createImageFileInput.files
                ? createImageFileInput.files[0]
                : null;
            if (createFile) {
              body.image = await uploadProfileImageFile(createFile);
              document.getElementById("uc-image").value = body.image;
              if (createImagePreview) {
                createImagePreview.src = normalizeProfileImageSrc(body.image);
              }
            }
            var res = await ctx.fetchJSON("POST", "/api/v1/admin/users", body);
            if (!res.ok) {
              var data = await res.json().catch(function () {
                return null;
              });
              AC.setInlineError(
                document.getElementById("user-create-error"),
                (data && data.error) ||
                  "Gagal membuat pengguna (" + res.status + ")"
              );
              return;
            }
            AC.hideModal(modalCreateEl);
            AC.showSuccessToast("Pengguna berhasil dibuat.");
            ctx.state.page = 1;
            ctx.loadList();
          } catch (err) {
            if (err && err.message === "unauthorized") return;
            AC.setInlineError(
              document.getElementById("user-create-error"),
              "Terjadi gangguan jaringan."
            );
          }
        });
      }

      var formEdit = document.getElementById("form-user-edit");
      if (formEdit) {
        formEdit.noValidate = true;
        formEdit.addEventListener("submit", async function (e) {
          e.preventDefault();
          AC.setInlineError(document.getElementById("user-edit-error"), null);
          var userId = document.getElementById("ue-id").value;
          var payload = {
            name: document.getElementById("ue-name").value.trim(),
            username: document.getElementById("ue-username").value.trim(),
            email: document.getElementById("ue-email").value.trim(),
            role: document.getElementById("ue-role").value,
            profile: document.getElementById("ue-profile").value.trim(),
            image: document.getElementById("ue-image").value.trim(),
          };
          var enabledInput = document.getElementById("ue-enabled");
          if (enabledInput && !enabledInput.closest(".d-none")) {
            payload.enabled = !!enabledInput.checked;
          }
          var imageFileInput = document.getElementById("ue-image-file");
          var imagePreview = document.getElementById("ue-image-preview");
          var pw = document.getElementById("ue-password").value.trim();
          if (pw) payload.password = pw;
          try {
            var file =
              imageFileInput && imageFileInput.files
                ? imageFileInput.files[0]
                : null;
            if (ueImageRemoved) {
              payload.image = "";
              document.getElementById("ue-image").value = "";
            } else if (file) {
              payload.image = await uploadProfileImageFile(file);
              document.getElementById("ue-image").value = payload.image;
              if (imagePreview) {
                imagePreview.src = normalizeProfileImageSrc(payload.image);
              }
            }
            var res = await ctx.fetchJSON(
              "PUT",
              userUrl(userId),
              payload
            );
            if (!res.ok) {
              var data = await res.json().catch(function () {
                return null;
              });
              AC.setInlineError(
                document.getElementById("user-edit-error"),
                (data && data.error) ||
                  "Gagal memperbarui pengguna (" + res.status + ")"
              );
              return;
            }
            var updatedUser = await res.json().catch(function () {
              return null;
            });
            if (updatedUser) syncCurrentUserSessionAndNavbar(updatedUser);
            ueImageRemoved = false;
            AC.hideModal(modalEditEl);
            AC.showSuccessToast("Pengguna berhasil diperbarui.");
            ctx.loadList();
          } catch (err) {
            if (err && err.message === "unauthorized") return;
            AC.setInlineError(
              document.getElementById("user-edit-error"),
              "Terjadi gangguan jaringan."
            );
          }
        });
      }

      var udConfirm = document.getElementById("ud-confirm");
      if (udConfirm) {
        udConfirm.addEventListener("click", async function () {
          if (!deleteUserId) return;
          AC.setInlineError(
            document.getElementById("user-delete-error"),
            null
          );
          try {
            var res = await ctx.fetchJSON(
              "DELETE",
              userUrl(deleteUserId),
              null
            );
            if (!res.ok) {
              var data = await res.json().catch(function () {
                return null;
              });
              AC.setInlineError(
                document.getElementById("user-delete-error"),
                (data && data.error) ||
                  "Gagal menghapus pengguna (" + res.status + ")"
              );
              return;
            }
            AC.hideModal(modalDeleteEl);
            deleteUserId = null;
            AC.showSuccessToast("Pengguna berhasil dihapus.");
            ctx.loadList();
          } catch (err) {
            if (err && err.message === "unauthorized") return;
            AC.setInlineError(
              document.getElementById("user-delete-error"),
              "Terjadi gangguan jaringan."
            );
          }
        });
      }
    },
  });

  function openEditModal(userId) {
    if (!userId || !modalEditEl) return;
    AC.setInlineError(document.getElementById("user-edit-error"), null);
    document.getElementById("ue-id").value = userId;
    document.getElementById("ue-name").value = "";
    document.getElementById("ue-username").value = "";
    document.getElementById("ue-email").value = "";
    document.getElementById("ue-password").value = "";
    document.getElementById("ue-role").value = "writer";
    document.getElementById("ue-enabled").checked = true;
    document.getElementById("ue-profile").value = "";
    document.getElementById("ue-image").value = "";
    ueImageRemoved = false;
    var imageFileInput = document.getElementById("ue-image-file");
    if (imageFileInput) imageFileInput.value = "";
    var imagePreview = document.getElementById("ue-image-preview");
    if (imagePreview) imagePreview.src = DEFAULT_PROFILE_ICON;

    var enabledWrap = document.getElementById("ue-enabled")
      ? document.getElementById("ue-enabled").closest(".js-user-enabled-wrap")
      : null;
    if (enabledWrap) enabledWrap.classList.remove("d-none");
    showImageTab("ue-image-upload-tab");
    syncUserImageFieldState("ue");

    var modal = bootstrap.Modal.getOrCreateInstance(modalEditEl);
    modal.show();

    (async function () {
      try {
        var res = await fetch(userUrl(userId), {
          method: "GET",
          headers: { Authorization: "Bearer " + AC.getToken("bebii_token") },
        });
        if (res.status === 401) {
          window.location.href =
        (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login";
          return;
        }
        if (!res.ok) {
          AC.setInlineError(
            document.getElementById("user-edit-error"),
            await AC.responseErrorMessage(res, "Gagal memuat pengguna")
          );
          return;
        }
        var u = await res.json();
        document.getElementById("ue-name").value = u && u.name ? u.name : "";
        document.getElementById("ue-username").value =
          u && u.username ? u.username : "";
        document.getElementById("ue-email").value =
          u && u.email ? u.email : "";
        document.getElementById("ue-role").value =
          u && u.role ? u.role : "writer";
        document.getElementById("ue-enabled").checked = !(u && u.enabled === false);
        if (enabledWrap && isCurrentUser(userId)) {
          enabledWrap.classList.add("d-none");
        }
        document.getElementById("ue-profile").value =
          u && u.profile ? u.profile : "";
        document.getElementById("ue-image").value =
          u && u.image ? u.image : "";
        normalizeImageUrlInput(document.getElementById("ue-image"));
        ueImageRemoved = false;
        if (imagePreview) {
          imagePreview.src = normalizeProfileImageSrc(u && u.image ? u.image : "");
        }
      } catch (err) {
        AC.setInlineError(
          document.getElementById("user-edit-error"),
          "Terjadi gangguan jaringan."
        );
      }
    })();
  }

  var ueImageFile = document.getElementById("ue-image-file");
  var ueImagePreview = document.getElementById("ue-image-preview");
  if (ueImageFile && ueImagePreview) {
    ensureEditImageRemoveButton();
    ueImagePreview.src = DEFAULT_PROFILE_ICON;
    ueImageFile.addEventListener("change", function () {
      var f = ueImageFile.files && ueImageFile.files[0];
      if (!f) return;
      ueImageRemoved = false;
      showImageTab("ue-image-upload-tab");
      syncUserImageFieldState("ue");
      ueImagePreview.src = URL.createObjectURL(f);
    });
    ueImagePreview.addEventListener("error", function () {
      ueImagePreview.src = DEFAULT_PROFILE_ICON;
    });
  }

  var ucImageFile = document.getElementById("uc-image-file");
  var ucImagePreview = document.getElementById("uc-image-preview");
  if (ucImageFile && ucImagePreview) {
    ucImagePreview.src = DEFAULT_PROFILE_ICON;
    ucImageFile.addEventListener("change", function () {
      var f = ucImageFile.files && ucImageFile.files[0];
      if (!f) return;
      showImageTab("uc-image-upload-tab");
      syncUserImageFieldState("uc");
      ucImagePreview.src = URL.createObjectURL(f);
    });
    ucImagePreview.addEventListener("error", function () {
      ucImagePreview.src = DEFAULT_PROFILE_ICON;
    });
  }

  var ucImageUrl = document.getElementById("uc-image");
  bindImageUrlInput(ucImageUrl);
  if (ucImageUrl) {
    ucImageUrl.addEventListener("focus", function () {
      showImageTab("uc-image-url-tab");
    });
    ucImageUrl.addEventListener("input", function () {
      normalizeImageUrlInput(ucImageUrl);
      var v = ucImageUrl.value.trim();
      if (v && ucImagePreview) ucImagePreview.src = normalizeProfileImageSrc(v);
      if (!v && ucImagePreview && !(ucImageFile && ucImageFile.files && ucImageFile.files[0])) {
        ucImagePreview.src = DEFAULT_PROFILE_ICON;
      }
    });
  }

  var ueImageUrl = document.getElementById("ue-image");
  bindImageUrlInput(ueImageUrl);
  if (ueImageUrl) {
    ueImageUrl.addEventListener("focus", function () {
      showImageTab("ue-image-url-tab");
    });
    ueImageUrl.addEventListener("input", function () {
      normalizeImageUrlInput(ueImageUrl);
      var v = ueImageUrl.value.trim();
      ueImageRemoved = false;
      if (v && ueImagePreview) ueImagePreview.src = normalizeProfileImageSrc(v);
      if (!v && ueImagePreview && !(ueImageFile && ueImageFile.files && ueImageFile.files[0])) {
        ueImagePreview.src = DEFAULT_PROFILE_ICON;
      }
    });
  }

  bindUserImageTabSync("ue-image-tabs", "ue");
  bindUserImageTabSync("uc-image-tabs", "uc");

  function openDeleteModal(userId) {
    if (!userId || !modalDeleteEl) return;
    deleteUserId = userId;
    AC.setInlineError(document.getElementById("user-delete-error"), null);
    document.getElementById("ud-id").textContent = "";
    document.getElementById("ud-name").textContent = "";
    document.getElementById("ud-email").textContent = "";

    var modal = bootstrap.Modal.getOrCreateInstance(modalDeleteEl);
    modal.show();

    (async function () {
      try {
        var res = await fetch(userUrl(userId), {
          method: "GET",
          headers: { Authorization: "Bearer " + AC.getToken("bebii_token") },
        });
        if (res.status === 401) {
          window.location.href =
        (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login";
          return;
        }
        if (!res.ok) {
          AC.setInlineError(
            document.getElementById("user-delete-error"),
            await AC.responseErrorMessage(res, "Gagal memuat pengguna")
          );
          return;
        }
        var u = await res.json();
        document.getElementById("ud-id").textContent =
          u && u.id != null ? u.id : "";
        document.getElementById("ud-name").textContent =
          u && u.name ? u.name : "";
        document.getElementById("ud-email").textContent =
          u && u.email ? u.email : "";
      } catch (err) {
        AC.setInlineError(
          document.getElementById("user-delete-error"),
          "Terjadi gangguan jaringan."
        );
      }
    })();
  }
})();
