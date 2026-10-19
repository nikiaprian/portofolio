(function () {
  "use strict";

  var API = "/api/v1/admin/ai-setting";
  var UPLOAD_BULK_COVER_API =
    "/api/v1/admin/ai-setting/upload-bulk-cover-background";
  var TOKEN_KEY = "bebii_token";
  var LOGIN_URL =
    (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login";

  function qs(sel) {
    return document.querySelector(sel);
  }

  function wirePasswordToggle(inputId, buttonId) {
    var input = document.getElementById(inputId);
    var btn = document.getElementById(buttonId);
    if (!input || !btn) return;

    function sync() {
      var hidden = input.type === "password";
      var icon = btn.querySelector("i");
      if (icon) {
        icon.className = hidden ? "bi bi-eye" : "bi bi-eye-slash";
      }
      btn.setAttribute("aria-label", hidden ? "Tampilkan password" : "Sembunyikan password");
      btn.setAttribute("aria-pressed", hidden ? "false" : "true");
      btn.setAttribute("title", hidden ? "Tampilkan password" : "Sembunyikan password");
    }

    sync();
    btn.addEventListener("click", function (e) {
      e.preventDefault();
      input.type = input.type === "password" ? "text" : "password";
      sync();
    });
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
    var oc = qs("#ais-opencode");
    var ocm = qs("#ais-opencode-model");
    var oclHost = qs("#ais-opencode-local-host");
    var oclPort = qs("#ais-opencode-local-port");
    var oclUser = qs("#ais-opencode-local-username");
    var oclPass = qs("#ais-opencode-local-password");
    var oclModel = qs("#ais-opencode-local-model");
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
    var gemSearch = qs("#ais-gemini-search-mode");
    var oaiSearch = qs("#ais-openai-search-mode");
    var grqSearch = qs("#ais-groq-search-mode");
    var claSearch = qs("#ais-claude-search-mode");
    var ocSearch = qs("#ais-opencode-search-mode");
    if (g && data.gemini != null) g.value = data.gemini;
    if (gm && data.gemini_model != null) setSelectValue(gm, data.gemini_model);
    if (o && data.openai != null) o.value = data.openai;
    if (om && data.openai_model != null) setSelectValue(om, data.openai_model);
    if (q && data.groq != null) q.value = data.groq;
    if (qm && data.groq_model != null) setSelectValue(qm, data.groq_model);
    if (cl && data.claude != null) cl.value = data.claude;
    if (clm && data.claude_model != null) setSelectValue(clm, data.claude_model);
    if (oc && data.opencode != null) oc.value = data.opencode;
    if (ocm && data.opencode_model != null) setSelectValue(ocm, data.opencode_model);
    var ocl = data.opencode_local || {};
    if (oclHost && ocl.host != null) oclHost.value = ocl.host;
    if (oclPort && ocl.port != null) oclPort.value = ocl.port;
    if (oclUser && ocl.username != null) oclUser.value = ocl.username;
    if (oclPass && ocl.password != null) oclPass.value = ocl.password;
    if (oclModel) setSelectValue(oclModel, ocl.model || "auto");
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
    if (gemSearch) gemSearch.checked = !!data.gemini_search_mode;
    if (oaiSearch) oaiSearch.checked = !!data.openai_search_mode;
    if (grqSearch) grqSearch.checked = !!data.groq_search_mode;
    if (claSearch) claSearch.checked = !!data.claude_search_mode;
    if (ocSearch) ocSearch.checked = !!data.opencode_search_mode;
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

  var tinyInstances = {};

  function destroyTiny(provider) {
    var ed = tinyInstances[provider];
    if (ed) {
      try { ed.remove(); } catch (x) {}
      delete tinyInstances[provider];
    }
  }

  function getResponseTA(provider) {
    return document.querySelector(
      '.ais-test-result[data-provider="' + provider + '"]'
    );
  }

  async function testChat(provider, btn, inputEl) {
    var resultEl = getResponseTA(provider);
    if (provider === "opencode_local") {
      var hostEl = qs("#ais-opencode-local-host");
      var portEl = qs("#ais-opencode-local-port");
      var hostVal = hostEl && String(hostEl.value || "").trim();
      var portVal = portEl && String(portEl.value || "").trim();
      if (!hostVal && !portVal) {
        showTestError(provider, "Isi host atau port terlebih dahulu.");
        return;
      }
    } else {
      var keyInput = qs("#ais-" + provider);
      if (!keyInput || !String(keyInput.value || "").trim()) {
        showTestError(provider, "Isi API key terlebih dahulu.");
        return;
      }
    }
    var prompt = String(inputEl.value || "").trim();
    if (!prompt) {
      showTestError(provider, "Tulis pesan terlebih dahulu.");
      return;
    }

    destroyTiny(provider);
    resultEl.classList.add("d-none");
    resultEl.value = "";

    btn.disabled = true;
    var origHTML = btn.innerHTML;
    btn.innerHTML =
      '<span class="spinner-border spinner-border-sm" aria-hidden="true"></span>';
    try {
      var res = await fetchAuth(API + "/test-chat", {
        method: "POST",
        body: JSON.stringify({ provider: provider, prompt: prompt }),
      });
      var data = await res.json();
      if (!res.ok) {
        showTestError(provider, (data && data.error) || "Gagal terhubung ke " + provider);
        return;
      }
      var text = data.response || "(respon kosong)";
      resultEl.value = text;
      resultEl.classList.remove("d-none");
      resultEl.style.height = "120px";
      resultEl.value = text;
      resultEl.classList.remove("d-none");
      resultEl.style.height = "120px";
      if (typeof tinymce !== "undefined" && window.TinyMCE) {
        window.TinyMCE.init({
          selector: '.ais-test-result[data-provider="' + provider + '"]',
          height: 160,
          menubar: false,
          toolbar: false,
          readonly: true,
          statusbar: false,
          plugins: "",
          onSetup: function (ed) {
            tinyInstances[provider] = ed;
          },
        }).catch(function () {});
      }
    } catch (e) {
      if (e && e.message === "unauthorized") return;
      showTestError(provider, "Gagal terhubung ke " + provider);
    } finally {
      btn.disabled = false;
      btn.innerHTML = origHTML;
    }
  }

  function showTestError(provider, msg) {
    var el = getResponseTA(provider);
    destroyTiny(provider);
    el.classList.remove("d-none");
    el.style.height = "";
    el.className = "ais-test-result form-control form-control-sm border border-danger mt-1 text-danger small";
    el.value = msg;
  }

  function modelOptionValue(m) {
    if (!m) return "";
    var pid = String(m.providerID || "").trim();
    var id = String(m.id || "").trim();
    if (!id) return "";
    return pid ? pid + "/" + id : id;
  }

  function populateOpenCodeLocalModels(data, preferred) {
    var sel = qs("#ais-opencode-local-model");
    if (!sel) return;
    var keep = preferred != null ? String(preferred) : String(sel.value || "auto");
    if (!keep) keep = "auto";
    sel.innerHTML = "";
    var autoOpt = document.createElement("option");
    autoOpt.value = "auto";
    autoOpt.textContent = "Auto (free models, fallback)";
    sel.appendChild(autoOpt);

    var models = (data && data.models) || [];
    var freeGroup = document.createElement("optgroup");
    freeGroup.label = "Free Models";
    var paidGroup = document.createElement("optgroup");
    paidGroup.label = "Other Models";
    var hasFree = false;
    var hasPaid = false;
    for (var i = 0; i < models.length; i++) {
      var m = models[i];
      var val = modelOptionValue(m);
      if (!val) continue;
      var opt = document.createElement("option");
      opt.value = val;
      var label = String(m.name || m.id || val);
      if (m.free) label += " (free)";
      opt.textContent = label + " — " + val;
      if (m.free) {
        freeGroup.appendChild(opt);
        hasFree = true;
      } else {
        paidGroup.appendChild(opt);
        hasPaid = true;
      }
    }
    if (hasFree) sel.appendChild(freeGroup);
    if (hasPaid) sel.appendChild(paidGroup);
    setSelectValue(sel, keep);
  }

  async function refreshOpenCodeLocalModels(opts) {
    opts = opts || {};
    var btn = qs("#ais-opencode-local-models-refresh");
    var hostEl = qs("#ais-opencode-local-host");
    var portEl = qs("#ais-opencode-local-port");
    var userEl = qs("#ais-opencode-local-username");
    var passEl = qs("#ais-opencode-local-password");
    var host = hostEl && String(hostEl.value || "").trim();
    var port = portEl && String(portEl.value || "").trim();
    if (!host && !port && !opts.allowSaved) {
      if (!opts.silent) {
        setGlobalError(
          qs("#ai-setting-error"),
          "Isi host/port OpenCode local terlebih dahulu."
        );
      }
      return;
    }
    var origHTML = btn ? btn.innerHTML : "";
    if (btn) {
      btn.disabled = true;
      btn.innerHTML =
        '<span class="spinner-border spinner-border-sm" aria-hidden="true"></span>';
    }
    try {
      var res = await fetchAuth(API + "/opencode-local/models", {
        method: "POST",
        body: JSON.stringify({
          host: host || "",
          port: port || "",
          username: (userEl && userEl.value) || "",
          password: (passEl && passEl.value) || "",
        }),
      });
      var data = await res.json();
      if (!res.ok) {
        if (!opts.silent) {
          setGlobalError(
            qs("#ai-setting-error"),
            (data && data.error) || "Gagal memuat daftar model OpenCode local."
          );
        }
        return;
      }
      var preferred =
        opts.preferred != null
          ? opts.preferred
          : qs("#ais-opencode-local-model") && qs("#ais-opencode-local-model").value;
      populateOpenCodeLocalModels(data, preferred || "auto");
      if (!opts.silent) {
        setGlobalError(qs("#ai-setting-error"), null);
        showSuccessToast("Daftar model OpenCode local dimuat.");
      }
    } catch (e) {
      if (e && e.message === "unauthorized") return;
      if (!opts.silent) {
        setGlobalError(qs("#ai-setting-error"), "Gagal memuat daftar model OpenCode local.");
      }
    } finally {
      if (btn) {
        btn.disabled = false;
        btn.innerHTML = origHTML;
      }
    }
  }

  document.addEventListener("DOMContentLoaded", function () {
    if (!getToken()) {
      window.location.href = LOGIN_URL;
      return;
    }

    wirePasswordToggle("ais-opencode-local-password", "ais-opencode-local-password-toggle");

    var errEl = qs("#ai-setting-error");
    var form = qs("#form-ai-setting");
    var generateTitlePrompt = qs("#ais-generate-prompt-title");
    var generateMetaDescriptionPrompt = qs("#ais-generate-meta-desc");

    var testBtns = document.querySelectorAll(".ais-test-send");
    for (var i = 0; i < testBtns.length; i++) {
      testBtns[i].addEventListener("click", function (ev) {
        var btn = ev.currentTarget;
        var provider = btn.getAttribute("data-provider");
        var inputEl = document.querySelector(
          '.ais-test-input[data-provider="' + provider + '"]'
        );
        testChat(provider, btn, inputEl);
      });
    }

    var oclRefresh = qs("#ais-opencode-local-models-refresh");
    if (oclRefresh) {
      oclRefresh.addEventListener("click", function () {
        refreshOpenCodeLocalModels({ allowSaved: true });
      });
    }

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
          var ocl = (data && data.opencode_local) || {};
          var hasLocal = !!(ocl.host || ocl.port);
          if (hasLocal) {
            refreshOpenCodeLocalModels({
              silent: true,
              allowSaved: true,
              preferred: ocl.model || "auto",
            });
          }
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
          opencode: (qs("#ais-opencode") && qs("#ais-opencode").value) || "",
          opencode_local: {
            host:
              (qs("#ais-opencode-local-host") && qs("#ais-opencode-local-host").value) || "",
            port:
              (qs("#ais-opencode-local-port") && qs("#ais-opencode-local-port").value) || "",
            username:
              (qs("#ais-opencode-local-username") && qs("#ais-opencode-local-username").value) ||
              "",
            password:
              (qs("#ais-opencode-local-password") && qs("#ais-opencode-local-password").value) ||
              "",
            model:
              (qs("#ais-opencode-local-model") && qs("#ais-opencode-local-model").value) ||
              "auto",
          },
          gemini_model: (qs("#ais-gemini-model") && qs("#ais-gemini-model").value) || "",
          groq_model: (qs("#ais-groq-model") && qs("#ais-groq-model").value) || "",
          openai_model: (qs("#ais-openai-model") && qs("#ais-openai-model").value) || "",
          claude_model: (qs("#ais-claude-model") && qs("#ais-claude-model").value) || "",
          opencode_model: (qs("#ais-opencode-model") && qs("#ais-opencode-model").value) || "",
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
          gemini_search_mode: !!(qs("#ais-gemini-search-mode") && qs("#ais-gemini-search-mode").checked),
          openai_search_mode: !!(qs("#ais-openai-search-mode") && qs("#ais-openai-search-mode").checked),
          groq_search_mode: !!(qs("#ais-groq-search-mode") && qs("#ais-groq-search-mode").checked),
          claude_search_mode: !!(qs("#ais-claude-search-mode") && qs("#ais-claude-search-mode").checked),
          opencode_search_mode: !!(qs("#ais-opencode-search-mode") && qs("#ais-opencode-search-mode").checked),
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
