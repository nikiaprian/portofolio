(function () {
  "use strict";

  var api = window.__CLIENT_API_ACCESS__;
  if (!api) return;

  function escapeHtml(s) {
    if (s == null) return "";
    var d = document.createElement("div");
    d.textContent = s;
    return d.innerHTML;
  }

  function slugify(s) {
    return String(s || "")
      .trim()
      .toLowerCase()
      .replace(/[^\w\s-]/g, "")
      .replace(/[\s_]+/g, "-")
      .replace(/^-+|-+$/g, "");
  }

  var state = { tab: "posts", categories: [], tags: [], posts: [] };

  function showLoginForm(root) {
    root.innerHTML =
      '<div class="admin-login card card-narrow">' +
      "<h1>Admin</h1>" +
      '<p class="muted">Sign in to manage posts, categories, and tags.</p>' +
      '<form id="admin-login-form" class="stack">' +
      '<label>Email <input type="email" name="email" required autocomplete="username"></label>' +
      '<label>Password <input type="password" name="password" required autocomplete="current-password"></label>' +
      '<button type="submit" class="btn btn-primary">Login</button>' +
      "</form>" +
      '<p class="muted"><a href="/">← Back to site</a></p>' +
      "</div>";

    document.getElementById("admin-login-form").addEventListener("submit", async function (e) {
      e.preventDefault();
      var fd = new FormData(e.target);
      try {
        var data = await api.login(fd.get("email"), fd.get("password"));
        if (data.token) api.setSession(data.token, data.user);
        updateShell();
        await loadAll();
        renderPanel(document.getElementById("admin-root"));
      } catch (err) {
        alert(err.message || "Sign-in failed");
      }
    });
  }

  function updateShell() {
    var u = api.getUser();
    var nameEl = document.getElementById("admin-user-name");
    var outBtn = document.getElementById("admin-logout");
    if (nameEl) nameEl.textContent = u && u.name ? u.name : "";
    if (outBtn) outBtn.style.display = api.getToken() ? "inline-block" : "none";
  }

  async function loadAll() {
    var cats = await api.get("/admin/categories");
    var tags = await api.get("/admin/tags");
    var posts = await api.get("/admin/posts");
    state.categories = Array.isArray(cats) ? cats : [];
    state.tags = Array.isArray(tags) ? tags : [];
    state.posts = Array.isArray(posts) ? posts : [];
  }

  function renderPosts(container) {
    var rows = state.posts
      .map(function (p) {
        return (
          "<tr data-id=\"" +
          p.id +
          "\">" +
          "<td>" +
          escapeHtml(p.title) +
          "</td>" +
          "<td><code>" +
          escapeHtml(p.status || "") +
          "</code></td>" +
          "<td>" +
          escapeHtml((p.category && p.category.name) || "") +
          "</td>" +
          "<td class=\"actions\">" +
          '<button type="button" class="btn btn-sm btn-edit" data-id="' +
          p.id +
          '">Edit</button> ' +
          '<button type="button" class="btn btn-sm btn-danger btn-del" data-id="' +
          p.id +
          '">Delete</button>' +
          "</td>" +
          "</tr>"
        );
      })
      .join("");

    container.innerHTML =
      '<div class="admin-toolbar">' +
      '<button type="button" class="btn btn-primary" id="btn-new-post">New post</button>' +
      "</div>" +
      '<div class="table-wrap"><table class="data-table"><thead><tr>' +
      "<th>Title</th><th>Status</th><th>Category</th><th></th>" +
      "</tr></thead><tbody>" +
      (rows || '<tr><td colspan="4" class="muted">No data yet</td></tr>') +
      "</tbody></table></div>";

    container.querySelector("#btn-new-post").addEventListener("click", function () {
      openPostModal(null);
    });
    container.querySelectorAll(".btn-edit").forEach(function (btn) {
      btn.addEventListener("click", function () {
        var id = parseInt(btn.getAttribute("data-id"), 10);
        var post = state.posts.find(function (x) {
          return x.id === id;
        });
        openPostModal(post);
      });
    });
    container.querySelectorAll(".btn-del").forEach(function (btn) {
      btn.addEventListener("click", async function () {
        var id = parseInt(btn.getAttribute("data-id"), 10);
        if (!confirm("Delete this post?")) return;
        try {
          await api.delete("/admin/posts/" + id);
          await loadAll();
          renderPanel(document.getElementById("admin-root"));
        } catch (err) {
          alert(err.message);
        }
      });
    });
  }

  function openPostModal(post) {
    var u = api.getUser();
    var authorId = u && u.id ? u.id : 0;
    var catOptions = state.categories
      .map(function (c) {
        return (
          '<option value="' +
          c.id +
          '"' +
          (post && post.category_id === c.id ? " selected" : "") +
          ">" +
          escapeHtml(c.name) +
          "</option>"
        );
      })
      .join("");
    var tagChecks = state.tags
      .map(function (t) {
        var sel =
          post &&
          Array.isArray(post.tags) &&
          post.tags.some(function (x) {
            return x.id === t.id;
          });
        return (
          "<label class=\"tag-check\"><input type=\"checkbox\" name=\"tag\" value=\"" +
          t.id +
          '"' +
          (sel ? " checked" : "") +
          "> " +
          escapeHtml(t.name) +
          "</label>"
        );
      })
      .join(" ");

    var wrap = document.createElement("div");
    wrap.className = "modal-overlay";
    wrap.innerHTML =
      '<div class="modal card">' +
      "<h2>" +
      (post ? "Edit post" : "New post") +
      "</h2>" +
      '<form id="post-editor-form" class="stack">' +
      "<label>Judul <input name=\"title\" required value=\"" +
      escapeHtml(post ? post.title : "") +
      '"></label>' +
      "<label>Slug <input name=\"slug\" value=\"" +
      escapeHtml(post ? post.slug : "") +
      '"></label>' +
      "<label>Category <select name=\"category_id\" required>" +
      '<option value="">— select —</option>' +
      catOptions +
      "</select></label>" +
      "<label>Status <select name=\"status\">" +
      '<option value="draft"' +
      (!post || post.status === "draft" ? " selected" : "") +
      ">Draft</option>" +
      '<option value="published"' +
      (post && post.status === "published" ? " selected" : "") +
      ">Published</option>" +
      "</select></label>" +
      "<label>Content <textarea name=\"content\" rows=\"12\" required>" +
      escapeHtml(post ? post.content : "") +
      "</textarea></label>" +
      "<div><span class=\"label\">Tag</span><div class=\"tag-grid\">" +
      (tagChecks || '<span class="muted">No tags yet — create some in the Tags tab</span>') +
      "</div></div>" +
      '<div class="form-actions">' +
      '<button type="button" class="btn btn-ghost" id="modal-cancel">Cancel</button> ' +
      '<button type="submit" class="btn btn-primary">Save</button>' +
      "</div>" +
      "</form>" +
      "</div>";

    document.body.appendChild(wrap);

    var titleInput = wrap.querySelector('input[name="title"]');
    var slugInput = wrap.querySelector('input[name="slug"]');
    titleInput.addEventListener("blur", function () {
      if (!post && !slugInput.value) slugInput.value = slugify(titleInput.value);
    });

    function close() {
      wrap.remove();
    }
    wrap.querySelector("#modal-cancel").addEventListener("click", close);
    wrap.addEventListener("click", function (e) {
      if (e.target === wrap) close();
    });

    wrap.querySelector("#post-editor-form").addEventListener("submit", async function (e) {
      e.preventDefault();
      var fd = new FormData(e.target);
      var title = fd.get("title");
      var slugVal = fd.get("slug") || slugify(title);
      var categoryId = parseInt(fd.get("category_id"), 10);
      var tagIds = [];
      wrap.querySelectorAll('input[name="tag"]:checked').forEach(function (cb) {
        tagIds.push(parseInt(cb.value, 10));
      });
      var tagsPayload = tagIds.map(function (id) {
        return { id: id };
      });

      var payload = {
        title: title,
        slug: slugVal,
        content: fd.get("content"),
        status: fd.get("status"),
        author_id: post && post.author_id ? post.author_id : authorId,
        category_id: categoryId,
        tags: tagsPayload,
      };

      if (!payload.author_id) {
        alert("Missing author. Please sign in again.");
        return;
      }

      try {
        if (post) {
          await api.put("/admin/posts/" + post.id, payload);
        } else {
          await api.post("/admin/posts", payload);
        }
        close();
        await loadAll();
        renderPanel(document.getElementById("admin-root"));
      } catch (err) {
        alert(err.message || "Failed to save");
      }
    });
  }

  function renderCategories(container) {
    var rows = state.categories
      .map(function (c) {
        return (
          "<tr data-id=\"" +
          c.id +
          "\">" +
          "<td>" +
          escapeHtml(c.name) +
          "</td>" +
          "<td><code>" +
          escapeHtml(c.slug) +
          "</code></td>" +
          '<td class="actions">' +
          '<button type="button" class="btn btn-sm btn-cat-edit" data-id="' +
          c.id +
          '">Edit</button> ' +
          '<button type="button" class="btn btn-sm btn-danger btn-cat-del" data-id="' +
          c.id +
          '">Delete</button>' +
          "</td>" +
          "</tr>"
        );
      })
      .join("");

    container.innerHTML =
      '<div class="admin-toolbar">' +
      '<button type="button" class="btn btn-primary" id="btn-new-cat">New category</button>' +
      "</div>" +
      '<div class="table-wrap"><table class="data-table"><thead><tr>' +
      "<th>Name</th><th>Slug</th><th></th>" +
      "</tr></thead><tbody>" +
      (rows || '<tr><td colspan="3" class="muted">No data yet</td></tr>') +
      "</tbody></table></div>";

    container.querySelector("#btn-new-cat").addEventListener("click", function () {
      openCatModal(null);
    });
    container.querySelectorAll(".btn-cat-edit").forEach(function (btn) {
      btn.addEventListener("click", function () {
        var id = parseInt(btn.getAttribute("data-id"), 10);
        var c = state.categories.find(function (x) {
          return x.id === id;
        });
        openCatModal(c);
      });
    });
    container.querySelectorAll(".btn-cat-del").forEach(function (btn) {
      btn.addEventListener("click", async function () {
        var id = parseInt(btn.getAttribute("data-id"), 10);
        if (!confirm("Delete this category?")) return;
        try {
          await api.delete("/admin/categories/" + id);
          await loadAll();
          renderPanel(document.getElementById("admin-root"));
        } catch (err) {
          alert(err.message);
        }
      });
    });
  }

  function openCatModal(cat) {
    var wrap = document.createElement("div");
    wrap.className = "modal-overlay";
    wrap.innerHTML =
      '<div class="modal card">' +
      "<h2>" +
      (cat ? "Edit category" : "New category") +
      "</h2>" +
      '<form id="cat-form" class="stack">' +
      "<label>Name <input name=\"name\" required value=\"" +
      escapeHtml(cat ? cat.name : "") +
      '"></label>' +
      "<label>Slug <input name=\"slug\" required value=\"" +
      escapeHtml(cat ? cat.slug : "") +
      '"></label>' +
      '<div class="form-actions">' +
      '<button type="button" class="btn btn-ghost" id="cat-cancel">Cancel</button> ' +
      '<button type="submit" class="btn btn-primary">Save</button>' +
      "</div>" +
      "</form></div>";
    document.body.appendChild(wrap);
    function close() {
      wrap.remove();
    }
    wrap.querySelector("#cat-cancel").addEventListener("click", close);
    wrap.addEventListener("click", function (e) {
      if (e.target === wrap) close();
    });
    var nameIn = wrap.querySelector('input[name="name"]');
    var slugIn = wrap.querySelector('input[name="slug"]');
    nameIn.addEventListener("blur", function () {
      if (!cat && !slugIn.value) slugIn.value = slugify(nameIn.value);
    });
    wrap.querySelector("#cat-form").addEventListener("submit", async function (e) {
      e.preventDefault();
      var fd = new FormData(e.target);
      var body = { name: fd.get("name"), slug: fd.get("slug") || slugify(fd.get("name")) };
      try {
        if (cat) await api.put("/admin/categories/" + cat.id, body);
        else await api.post("/admin/categories", body);
        close();
        await loadAll();
        renderPanel(document.getElementById("admin-root"));
      } catch (err) {
        alert(err.message);
      }
    });
  }

  function renderTags(container) {
    var rows = state.tags
      .map(function (t) {
        return (
          "<tr data-id=\"" +
          t.id +
          "\">" +
          "<td>" +
          escapeHtml(t.name) +
          "</td>" +
          "<td><code>" +
          escapeHtml(t.slug) +
          "</code></td>" +
          '<td class="actions">' +
          '<button type="button" class="btn btn-sm btn-tag-edit" data-id="' +
          t.id +
          '">Edit</button> ' +
          '<button type="button" class="btn btn-sm btn-danger btn-tag-del" data-id="' +
          t.id +
          '">Delete</button>' +
          "</td>" +
          "</tr>"
        );
      })
      .join("");

    container.innerHTML =
      '<div class="admin-toolbar">' +
      '<button type="button" class="btn btn-primary" id="btn-new-tag">New tag</button>' +
      "</div>" +
      '<div class="table-wrap"><table class="data-table"><thead><tr>' +
      "<th>Name</th><th>Slug</th><th></th>" +
      "</tr></thead><tbody>" +
      (rows || '<tr><td colspan="3" class="muted">No data yet</td></tr>') +
      "</tbody></table></div>";

    container.querySelector("#btn-new-tag").addEventListener("click", function () {
      openTagModal(null);
    });
    container.querySelectorAll(".btn-tag-edit").forEach(function (btn) {
      btn.addEventListener("click", function () {
        var id = parseInt(btn.getAttribute("data-id"), 10);
        var t = state.tags.find(function (x) {
          return x.id === id;
        });
        openTagModal(t);
      });
    });
    container.querySelectorAll(".btn-tag-del").forEach(function (btn) {
      btn.addEventListener("click", async function () {
        var id = parseInt(btn.getAttribute("data-id"), 10);
        if (!confirm("Delete this tag?")) return;
        try {
          await api.delete("/admin/tags/" + id);
          await loadAll();
          renderPanel(document.getElementById("admin-root"));
        } catch (err) {
          alert(err.message);
        }
      });
    });
  }

  function openTagModal(tag) {
    var wrap = document.createElement("div");
    wrap.className = "modal-overlay";
    wrap.innerHTML =
      '<div class="modal card">' +
      "<h2>" +
      (tag ? "Edit tag" : "New tag") +
      "</h2>" +
      '<form id="tag-form" class="stack">' +
      "<label>Name <input name=\"name\" required value=\"" +
      escapeHtml(tag ? tag.name : "") +
      '"></label>' +
      "<label>Slug <input name=\"slug\" required value=\"" +
      escapeHtml(tag ? tag.slug : "") +
      '"></label>' +
      '<div class="form-actions">' +
      '<button type="button" class="btn btn-ghost" id="tag-cancel">Cancel</button> ' +
      '<button type="submit" class="btn btn-primary">Save</button>' +
      "</div>" +
      "</form></div>";
    document.body.appendChild(wrap);
    function close() {
      wrap.remove();
    }
    wrap.querySelector("#tag-cancel").addEventListener("click", close);
    wrap.addEventListener("click", function (e) {
      if (e.target === wrap) close();
    });
    var nameIn = wrap.querySelector('input[name="name"]');
    var slugIn = wrap.querySelector('input[name="slug"]');
    nameIn.addEventListener("blur", function () {
      if (!tag && !slugIn.value) slugIn.value = slugify(nameIn.value);
    });
    wrap.querySelector("#tag-form").addEventListener("submit", async function (e) {
      e.preventDefault();
      var fd = new FormData(e.target);
      var body = { name: fd.get("name"), slug: fd.get("slug") || slugify(fd.get("name")) };
      try {
        if (tag) await api.put("/admin/tags/" + tag.id, body);
        else await api.post("/admin/tags", body);
        close();
        await loadAll();
        renderPanel(document.getElementById("admin-root"));
      } catch (err) {
        alert(err.message);
      }
    });
  }

  function renderPanel(root) {
    var inner = document.createElement("div");
    inner.className = "admin-panel-inner";

    var nav = document.createElement("div");
    nav.className = "admin-tabs";
    nav.innerHTML =
      '<button type="button" data-tab="posts" class="' +
      (state.tab === "posts" ? "active" : "") +
      '">Posts</button>' +
      '<button type="button" data-tab="categories" class="' +
      (state.tab === "categories" ? "active" : "") +
      '">Categories</button>' +
      '<button type="button" data-tab="tags" class="' +
      (state.tab === "tags" ? "active" : "") +
      '">Tag</button>';

    var content = document.createElement("div");
    content.id = "admin-tab-content";

    nav.querySelectorAll("button").forEach(function (b) {
      b.addEventListener("click", function () {
        state.tab = b.getAttribute("data-tab");
        renderPanel(root);
      });
    });

    inner.appendChild(nav);
    inner.appendChild(content);
    root.innerHTML = "";
    root.appendChild(inner);

    if (state.tab === "posts") renderPosts(content);
    else if (state.tab === "categories") renderCategories(content);
    else renderTags(content);
  }

  async function init() {
    var root = document.getElementById("admin-root");
    if (!root) return;
    updateShell();
    if (!api.getToken()) {
      showLoginForm(root);
      return;
    }
    try {
      await loadAll();
      renderPanel(root);
    } catch (err) {
      root.innerHTML =
        '<div class="notice notice-error">' +
        escapeHtml(err.message || "Failed to load data") +
        " — your session may have expired. Please sign in again.</div>";
      api.logout();
      updateShell();
      showLoginForm(root);
    }
  }

  document.addEventListener("DOMContentLoaded", init);

  document.addEventListener("click", function (e) {
    if (e.target && e.target.id === "admin-logout") {
      e.preventDefault();
      api.logout();
      updateShell();
      var root = document.getElementById("admin-root");
      if (root) showLoginForm(root);
    }
  });
})();
