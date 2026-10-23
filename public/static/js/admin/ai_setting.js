(function () {
  "use strict";

  var API = "/api/v1/admin/ai-setting";
  var UPLOAD_BULK_COVER_API =
    "/api/v1/admin/ai-setting/upload-bulk-cover-background";
  var TOKEN_KEY = "tirtayasa_token";
  var LOGIN_URL =
    (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) || "/login";

  function qs(sel) {
    return document.querySelector(sel);
  }

  function getToken() {
    try {
      return localStorage.getItem(TOKEN_KEY);
    } catch (e) {
      return null;
    }
  }

  function setGlobalError(el, message) {
    if (!el) return;
    if (!message) {
      el.classList.add("d-none");
      el.textContent = "";
      return;
    }
    el.classList.remove("d-none");
    el.textContent = message;
  }

  function updateCharacterCount(inputSelector, countSelector) {
    var input = qs(inputSelector);
    var counter = qs(countSelector);
    if (!input || !counter) return;
    counter.textContent = String(input.value || "").length + " karakter";
  }

  function updateGeneratePromptCharacterCounts() {
    updateCharacterCount(
      "#ais-generate-prompt-title",
      "#ais-generate-prompt-title-count"
    );
    updateCharacterCount(
      "#ais-generate-meta-desc",
      "#ais-generate-meta-desc-count"
    );
  }

  function showSuccessToast(message) {
    if (!message || typeof Swal === "undefined") return;
    Swal.mixin({
      toast: true,
      position: "top-end",
      showConfirmButton: false,
      timer: 3200,
      timerProgressBar: true,
    }).fire({
      icon: "success",
      title: message,
    });
  }

  async function fetchAuth(url, options) {
    var token = getToken();
    options = options || {};
    options.headers = Object.assign({}, options.headers || {});
    options.headers.Authorization = "Bearer " + token;
    var m = (options.method || "GET").toUpperCase();
    if (options.body && m !== "GET" && m !== "HEAD" && !options.headers["Content-Type"]) {
      options.headers["Content-Type"] = "application/json";
    }
    var res = await fetch(url, options);
    if (res.status === 401) {
      window.location.href = LOGIN_URL;
      throw new Error("unauthorized");
    }
    return res;
  }

  function clearCustomOptions(sel) {
    if (!sel) return;
    var opts = sel.querySelectorAll("option[data-ais-custom]");
    for (var i = 0; i < opts.length; i++) opts[i].remove();
  }

  function setSelectValue(sel, value) {
    if (!sel) return;
    if (value == null) value = "";
    value = String(value);
    clearCustomOptions(sel);
    sel.value = value;
    if (value !== "" && sel.value !== value) {
      var opt = document.createElement("option");
      opt.value = value;
      opt.textContent = value + " (custom)";
      opt.setAttribute("data-ais-custom", "1");
      sel.appendChild(opt);
      sel.value = value;
    }
  }

  function parseBulkCoverURLs(raw) {
    if (!raw) return [];
    var parts = String(raw).split(/[\n\r,;]+/);
    var seen = {};
    var out = [];
    for (var i = 0; i < parts.length; i++) {
      var v = String(parts[i] || "").trim();
      if (!v || seen[v]) continue;
      seen[v] = true;
      out.push(v);
    }
    return out;
  }

  function setBulkCoverURLs(urls) {
    var inp = qs("#ais-bulk-cover-bg-url");
    if (!inp) return;
    inp.value = (urls || []).join("\n");
    renderBulkCoverPreview(urls || []);
  }

  function addBulkCoverURL(url) {
    var inp = qs("#ais-bulk-cover-bg-url");
    if (!inp) return;
    var urls = parseBulkCoverURLs(inp.value);
    var next = String(url || "").trim();
    if (!next) return;
    if (urls.indexOf(next) === -1) urls.push(next);
    setBulkCoverURLs(urls);
  }

  function renderBulkCoverPreview(urls) {
    var root = qs("#ais-bulk-cover-preview-list");
    if (!root) return;
    root.innerHTML = "";
    if (!urls || !urls.length) return;
    for (var i = 0; i < urls.length; i++) {
      var col = document.createElement("div");
      col.className = "col-6 col-md-3";
      var card = document.createElement("div");
      card.className = "border rounded p-1 h-100 bg-body-tertiary position-relative";
      var img = document.createElement("img");
      img.src = urls[i];
      img.alt = "Template sampul bulk " + (i + 1);
      img.loading = "lazy";
      img.className = "img-fluid rounded";
      img.style.aspectRatio = "3 / 2";
      img.style.objectFit = "cover";
      var cap = document.createElement("div");
      cap.className = "small text-truncate mt-1 px-1";
      cap.title = urls[i];
      cap.textContent = urls[i];
      var removeBtn = document.createElement("button");
      removeBtn.type = "button";
      removeBtn.className =
        "btn btn-sm btn-secondary rounded-circle position-absolute top-0 end-0 m-1 d-inline-flex align-items-center justify-content-center";
      removeBtn.style.width = "20px";
      removeBtn.style.height = "20px";
      removeBtn.setAttribute("aria-label", "Hapus template sampul bulk");
      removeBtn.innerHTML = '<i class="bi bi-x-lg" aria-hidden="true"></i>';
      removeBtn.setAttribute("data-url", urls[i]);
      removeBtn.addEventListener("click", function (ev) {
        var url = ev.currentTarget
          ? String(ev.currentTarget.getAttribute("data-url") || "").trim()
          : "";
        if (!url) return;
        var inp = qs("#ais-bulk-cover-bg-url");
        if (!inp) return;
        var next = parseBulkCoverURLs(inp.value).filter(function (it) {
          return String(it).trim() !== url;
        });
        setBulkCoverURLs(next);
      });
      card.appendChild(removeBtn);
      card.appendChild(img);
      card.appendChild(cap);
      col.appendChild(card);
      root.appendChild(col);
    }
  }

  function fillForm(data) {
    if (!data) return;
    var g = qs("#ais-gemini");
    var gm = qs("#ais-gemini-model");
    var o = qs("#ais-openai");
    var om = qs("#ais-openai-model");
    var q = qs("#ais-groq");
    var qm = qs("#ais-groq-model");
    var cl = qs("#ais-claude");
    var clm = qs("#ais-claude-model");
    var scraperProv = qs("#ais-scraper-provider");
    var genProv = qs("#ais-generate-provider");
    var sTitle = qs("#ais-scraper-prompt-title");
    var sContent = qs("#ais-scraper-prompt-content");
    var gTitle = qs("#ais-generate-prompt-title");
    var gContentName = qs("#ais-generate-prompt-content-name");
    var gContent = qs("#ais-generate-prompt-content");
    var gContentName2 = qs("#ais-generate-prompt-content-name-2");
    var gContent2 = qs("#ais-generate-prompt-content-2");
    var gContentName3 = qs("#ais-generate-prompt-content-name-3");
    var gContent3 = qs("#ais-generate-prompt-content-3");
    var gMetaD = qs("#ais-generate-meta-desc");
    var gMetaK = qs("#ais-generate-meta-kw");
    var gImage = qs("#ais-generate-prompt-image");
    var gImageContent = qs("#ais-generate-prompt-image-to-content");
    if (g && data.gemini != null) g.value = data.gemini;
    if (gm && data.gemini_model != null) setSelectValue(gm, data.gemini_model);
    if (o && data.openai != null) o.value = data.openai;
    if (om && data.openai_model != null) setSelectValue(om, data.openai_model);
    if (q && data.groq != null) q.value = data.groq;
    if (qm && data.groq_model != null) setSelectValue(qm, data.groq_model);
    if (cl && data.claude != null) cl.value = data.claude;
    if (clm && data.claude_model != null) setSelectValue(clm, data.claude_model);
    if (scraperProv) scraperProv.value = data.scraper_ai_provider || "auto";
    if (genProv) genProv.value = data.generate_ai_provider || "auto";
    if (sTitle && data.scraper_prompt_title != null) sTitle.value = data.scraper_prompt_title;
    if (sContent && data.scraper_prompt_content != null) {
      sContent.value = data.scraper_prompt_content;
    }
    if (gTitle && data.generate_prompt_title != null) gTitle.value = data.generate_prompt_title;
    if (gContentName && data.generate_prompt_content_name != null) {
      gContentName.value = data.generate_prompt_content_name;
    }
    if (gContent && data.generate_prompt_content != null) {
      gContent.value = data.generate_prompt_content;
    }
    if (gContentName2 && data.generate_prompt_content_name_2 != null) {
      gContentName2.value = data.generate_prompt_content_name_2;
    }
    if (gContent2 && data.generate_prompt_content_2 != null) {
      gContent2.value = data.generate_prompt_content_2;
    }
    if (gContentName3 && data.generate_prompt_content_name_3 != null) {
      gContentName3.value = data.generate_prompt_content_name_3;
    }
    if (gContent3 && data.generate_prompt_content_3 != null) {
      gContent3.value = data.generate_prompt_content_3;
    }
    if (gMetaD && data.generate_prompt_meta_description != null) {
      gMetaD.value = data.generate_prompt_meta_description;
    }
    if (gMetaK && data.generate_prompt_meta_keywords != null) {
      gMetaK.value = data.generate_prompt_meta_keywords;
    }
    updateGeneratePromptCharacterCounts();
    if (gImage && data.generate_prompt_image != null) {
      gImage.value = data.generate_prompt_image;
    }
    if (gImageContent && data.generate_prompt_image_to_content != null) {
      gImageContent.value = data.generate_prompt_image_to_content;
    }
    var bulkBg = qs("#ais-bulk-cover-bg-url");
    if (bulkBg && data.bulk_cover_background_url != null) {
      setBulkCoverURLs(parseBulkCoverURLs(data.bulk_cover_background_url));
    }
  }

  async function uploadBulkCoverFile(file) {
    var fd = new FormData();
    fd.append("image", file);
    var token = getToken();
    var res = await fetch(UPLOAD_BULK_COVER_API, {
      method: "POST",
      headers: { Authorization: "Bearer " + token },
      body: fd,
    });
    if (res.status === 401) {
      window.location.href = LOGIN_URL;
      throw new Error("unauthorized");
    }
    if (!res.ok) {
      var msg = "Upload gagal (" + res.status + ")";
      try {
        var j = await res.json();
        if (j && j.error) msg = j.error;
      } catch (x) {}
      throw new Error(msg);
    }
    var body = await res.json();
    if (!body || body.image == null || String(body.image).trim() === "") {
      throw new Error("Respons upload tidak valid");
    }
    return String(body.image).trim();
  }

  document.addEventListener("DOMContentLoaded", function () {
    if (!getToken()) {
      window.location.href = LOGIN_URL;
      return;
    }

    var errEl = qs("#ai-setting-error");
    var form = qs("#form-ai-setting");
    var generateTitlePrompt = qs("#ais-generate-prompt-title");
    var generateMetaDescriptionPrompt = qs("#ais-generate-meta-desc");

    if (generateTitlePrompt) {
      generateTitlePrompt.addEventListener(
        "input",
        updateGeneratePromptCharacterCounts
      );
    }
    if (generateMetaDescriptionPrompt) {
      generateMetaDescriptionPrompt.addEventListener(
        "input",
        updateGeneratePromptCharacterCounts
      );
    }
    updateGeneratePromptCharacterCounts();

    var bulkCoverFile = qs("#ais-bulk-cover-bg-file");
    if (bulkCoverFile) {
      bulkCoverFile.addEventListener("change", function () {
        var files = bulkCoverFile.files ? Array.from(bulkCoverFile.files) : [];
        if (!files.length) return;
        (async function () {
          var uploaded = 0;
          var failed = [];
          for (var i = 0; i < files.length; i++) {
            try {
              var url = await uploadBulkCoverFile(files[i]);
              addBulkCoverURL(url);
              uploaded++;
            } catch (e) {
              if (e && e.message === "unauthorized") return;
              failed.push(files[i] && files[i].name ? files[i].name : "file-" + (i + 1));
            }
          }
          bulkCoverFile.value = "";
          if (uploaded > 0) {
            showSuccessToast(
              uploaded +
                " template berhasil diunggah. Simpan pengaturan AI agar tersimpan."
            );
          }
          if (failed.length > 0) {
            setGlobalError(
              errEl,
              "Beberapa upload gagal: " + failed.join(", ")
            );
          } else {
            setGlobalError(errEl, null);
          }
        })();
      });
    }

    var bulkBgInput = qs("#ais-bulk-cover-bg-url");
    if (bulkBgInput) {
      bulkBgInput.addEventListener("input", function () {
        renderBulkCoverPreview(parseBulkCoverURLs(bulkBgInput.value));
      });
      renderBulkCoverPreview(parseBulkCoverURLs(bulkBgInput.value));
    }

    fetchAuth(API, { method: "GET" })
      .then(function (res) {
        return res.json().then(function (data) {
          if (!res.ok) {
            setGlobalError(errEl, (data && data.error) || "Gagal memuat pengaturan AI.");
            return;
          }
          fillForm(data);
        });
      })
      .catch(function (e) {
        if (e && e.message === "unauthorized") return;
        setGlobalError(errEl, "Terjadi gangguan jaringan saat memuat pengaturan AI.");
      });

    if (form) {
      form.addEventListener("submit", function (ev) {
        ev.preventDefault();
        setGlobalError(errEl, null);
        var body = {
          gemini: (qs("#ais-gemini") && qs("#ais-gemini").value) || "",
          groq: (qs("#ais-groq") && qs("#ais-groq").value) || "",
          openai: (qs("#ais-openai") && qs("#ais-openai").value) || "",
          claude: (qs("#ais-claude") && qs("#ais-claude").value) || "",
          gemini_model: (qs("#ais-gemini-model") && qs("#ais-gemini-model").value) || "",
          groq_model: (qs("#ais-groq-model") && qs("#ais-groq-model").value) || "",
          openai_model: (qs("#ais-openai-model") && qs("#ais-openai-model").value) || "",
          claude_model: (qs("#ais-claude-model") && qs("#ais-claude-model").value) || "",
          scraper_ai_provider:
            (qs("#ais-scraper-provider") && qs("#ais-scraper-provider").value) || "auto",
          generate_ai_provider:
            (qs("#ais-generate-provider") && qs("#ais-generate-provider").value) || "auto",
          scraper_prompt_title:
            (qs("#ais-scraper-prompt-title") && qs("#ais-scraper-prompt-title").value) || "",
          scraper_prompt_content:
            (qs("#ais-scraper-prompt-content") && qs("#ais-scraper-prompt-content").value) || "",
          generate_prompt_title:
            (qs("#ais-generate-prompt-title") && qs("#ais-generate-prompt-title").value) || "",
          generate_prompt_content_name:
            (qs("#ais-generate-prompt-content-name") &&
              qs("#ais-generate-prompt-content-name").value) ||
            "",
          generate_prompt_content:
            (qs("#ais-generate-prompt-content") && qs("#ais-generate-prompt-content").value) || "",
          generate_prompt_content_name_2:
            (qs("#ais-generate-prompt-content-name-2") &&
              qs("#ais-generate-prompt-content-name-2").value) ||
            "",
          generate_prompt_content_2:
            (qs("#ais-generate-prompt-content-2") &&
              qs("#ais-generate-prompt-content-2").value) ||
            "",
          generate_prompt_content_name_3:
            (qs("#ais-generate-prompt-content-name-3") &&
              qs("#ais-generate-prompt-content-name-3").value) ||
            "",
          generate_prompt_content_3:
            (qs("#ais-generate-prompt-content-3") &&
              qs("#ais-generate-prompt-content-3").value) ||
            "",
          generate_prompt_meta_description:
            (qs("#ais-generate-meta-desc") && qs("#ais-generate-meta-desc").value) || "",
          generate_prompt_meta_keywords:
            (qs("#ais-generate-meta-kw") && qs("#ais-generate-meta-kw").value) || "",
          generate_prompt_image:
            (qs("#ais-generate-prompt-image") && qs("#ais-generate-prompt-image").value) || "",
          generate_prompt_image_to_content:
            (qs("#ais-generate-prompt-image-to-content") &&
              qs("#ais-generate-prompt-image-to-content").value) ||
            "",
          bulk_cover_background_url:
            (qs("#ais-bulk-cover-bg-url") && qs("#ais-bulk-cover-bg-url").value) ||
            "",
        };
        fetchAuth(API, {
          method: "PUT",
          body: JSON.stringify(body),
        })
          .then(function (res) {
            return res.json().then(function (data) {
              if (!res.ok) {
                setGlobalError(errEl, (data && data.error) || "Gagal menyimpan.");
                return;
              }
              fillForm(data);
              showSuccessToast("Pengaturan AI berhasil disimpan.");
            });
          })
          .catch(function (e) {
            if (e && e.message === "unauthorized") return;
            setGlobalError(errEl, "Terjadi gangguan jaringan.");
          });
      });
    }
  });
})();
