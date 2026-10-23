(function () {
  "use strict";

  var api = window.TirtayasaAPI;
  if (!api) return;

  var form = document.getElementById("admin-profile-form");
  if (!form) return;

  function getToken() {
    var t = api.getToken();
    if (t) return t;
    try {
      return localStorage.getItem("tirtayasa_token") || "";
    } catch (e) {
      return "";
    }
  }

  var token = getToken();
  if (!token) {
    window.location.href =
      (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) || "/login";
    return;
  }

  var errorEl = document.getElementById("profile-error");
  var successEl = document.getElementById("profile-success");
  var imageInput = document.getElementById("profile-image-file");
  var imageUrlInput = document.getElementById("profile-image-url");
  var imagePreview = document.getElementById("profile-image-preview");
  var imageRemoved = false;
  var DEFAULT_PROFILE_ICON =
    "data:image/svg+xml;utf8,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 64 64'%3E%3Ccircle cx='32' cy='32' r='32' fill='%23e9ecef'/%3E%3Ccircle cx='32' cy='24' r='12' fill='%2399a1aa'/%3E%3Cpath d='M12 54c3-10 12-16 20-16s17 6 20 16' fill='%2399a1aa'/%3E%3C/svg%3E";

  function normalizeProfileImageSrc(src) {
    var s = src != null ? String(src).trim() : "";
    if (!s) return DEFAULT_PROFILE_ICON;
    return s;
  }

  function setError(msg) {
    if (!errorEl) return;
    if (!msg) {
      errorEl.classList.add("d-none");
      errorEl.textContent = "";
      return;
    }
    errorEl.textContent = msg;
    errorEl.classList.remove("d-none");
  }

  function setSuccess(msg) {
    if (!successEl) return;
    if (!msg) {
      successEl.classList.add("d-none");
      successEl.textContent = "";
      return;
    }
    successEl.textContent = msg;
    successEl.classList.remove("d-none");
  }

  function headers(withJSON) {
    var h = { Accept: "application/json", Authorization: "Bearer " + getToken() };
    if (withJSON) h["Content-Type"] = "application/json";
    return h;
  }

  async function fetchProfile() {
    var res = await fetch("/api/v1/admin/profile", { method: "GET", headers: headers(false) });
    if (res.status === 401) {
      window.location.href =
      (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) || "/login";
      return null;
    }
    var data = await res.json();
    if (!res.ok) throw new Error((data && data.error) || "Failed to load profile");
    return data;
  }

  async function uploadImage(file) {
    var fd = new FormData();
    fd.append("image", file);
    var res = await fetch("/api/v1/admin/profile/upload-image", {
      method: "POST",
      headers: { Authorization: "Bearer " + getToken() },
      body: fd,
    });
    if (res.status === 401) {
      window.location.href =
      (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) || "/login";
      return null;
    }
    var data = await res.json();
    if (!res.ok) throw new Error((data && data.error) || "Failed to upload image");
    return data && data.image ? String(data.image) : "";
  }

  async function saveProfile(payload) {
    var res = await fetch("/api/v1/admin/profile", {
      method: "PUT",
      headers: headers(true),
      body: JSON.stringify(payload),
    });
    if (res.status === 401) {
      window.location.href =
      (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) || "/login";
      return null;
    }
    var data = await res.json();
    if (!res.ok) throw new Error((data && data.error) || "Failed to save profile");
    return data;
  }

  function bindPreviewFile() {
    if (!imageInput) return;
    imageInput.addEventListener("change", function () {
      var f = imageInput.files && imageInput.files[0];
      if (!f || !imagePreview) return;
      imageRemoved = false;
      imagePreview.src = URL.createObjectURL(f);
    });
  }

  function ensureRemoveImageButton() {
    if (!imagePreview || !imagePreview.parentElement) return;
    var parent = imagePreview.parentElement;
    if (parent.querySelector("#profile-image-remove")) return;
    parent.classList.add("position-relative", "d-inline-block");
    var btn = document.createElement("button");
    btn.type = "button";
    btn.id = "profile-image-remove";
    btn.className =
      "btn btn-sm btn-secondary rounded-circle position-absolute top-0 end-0 m-1 d-inline-flex align-items-center justify-content-center";
    btn.style.width = "20px";
    btn.style.height = "20px";
    btn.setAttribute("aria-label", "Remove profile image");
    btn.innerHTML = '<i class="bi bi-x-lg" aria-hidden="true"></i>';
    btn.addEventListener("click", function () {
      imageRemoved = true;
      if (imageInput) imageInput.value = "";
      if (imageUrlInput) imageUrlInput.value = "";
      if (imagePreview) imagePreview.src = DEFAULT_PROFILE_ICON;
    });
    parent.appendChild(btn);
  }

  if (imagePreview) {
    imagePreview.addEventListener("error", function () {
      imagePreview.src = DEFAULT_PROFILE_ICON;
    });
  }

  form.addEventListener("submit", async function (e) {
    e.preventDefault();
    setError("");
    setSuccess("");
    try {
      var uploaded = imageUrlInput.value.trim();
      var file = imageInput && imageInput.files ? imageInput.files[0] : null;
      if (imageRemoved) {
        uploaded = "";
      } else if (file) {
        uploaded = await uploadImage(file);
        imageUrlInput.value = uploaded;
      }

      var payload = {
        name: document.getElementById("profile-name").value.trim(),
        email: document.getElementById("profile-email").value.trim(),
        profile: document.getElementById("profile-bio").value.trim(),
        image: uploaded,
      };
      var password = document.getElementById("profile-password").value.trim();
      if (password) payload.password = password;

      var updated = await saveProfile(payload);
      api.setSession(getToken(), updated);
      if (imagePreview) imagePreview.src = normalizeProfileImageSrc(updated && updated.image);
      imageRemoved = false;
      setSuccess("Profile updated successfully.");
      document.getElementById("profile-password").value = "";
    } catch (err) {
      setError(err && err.message ? err.message : "Network error");
    }
  });

  (async function init() {
    bindPreviewFile();
    ensureRemoveImageButton();
    setError("");
    setSuccess("");
    try {
      var cached = api.getUser();
      if (cached) {
        document.getElementById("profile-name").value = cached.name || "";
        document.getElementById("profile-email").value = cached.email || "";
        document.getElementById("profile-bio").value = cached.profile || "";
        imageUrlInput.value = cached.image || "";
        if (imagePreview) imagePreview.src = normalizeProfileImageSrc(cached.image);
      }
    } catch (e) {}
    try {
      var u = await fetchProfile();
      if (!u) return;
      document.getElementById("profile-name").value = u.name || "";
      document.getElementById("profile-email").value = u.email || "";
      document.getElementById("profile-bio").value = u.profile || "";
      imageUrlInput.value = u.image || "";
      if (imagePreview) imagePreview.src = normalizeProfileImageSrc(u.image);
    } catch (err) {
      setError(err && err.message ? err.message : "Network error");
    }
  })();
})();
