(function () {
  var AC = window.AdminCrud;
  if (!AC) return;

  var root = document.getElementById("admin-custom-layout-form-root");
  if (!root) return;

  var API = "/api/v1/admin/site-layouts";
  var API_GEN_JOBS = API + "/generate-jobs";
  var TOKEN_KEY = "bebii_token";
  var LOGIN_URL =
    (typeof window !== "undefined" && window.BEBII_LOGIN_URL) || "/login";
  var POLL_MS = 2200;

  var FRAGMENTS = [
    { key: "header", field: "cl-header", label: "Header" },
    { key: "footer", field: "cl-footer", label: "Footer" },
    { key: "home_body", field: "cl-home-body", label: "Home body" },
    { key: "article_body", field: "cl-article-body", label: "Article body" },
    { key: "sidebar", field: "cl-sidebar", label: "Sidebar" },
    { key: "rss", field: "cl-rss", label: "RSS" },
    { key: "author_profile", field: "cl-author-profile", label: "Author profile" },
    { key: "tag", field: "cl-tag", label: "Tag" },
    { key: "kategori", field: "cl-kategori", label: "Kategori" },
    { key: "search_page", field: "cl-search-page", label: "Search page" },
  ];

  var trackedJobs = {};
  var pollTimer = null;
  var generateAllBusy = false;

  function el(id) {
    return document.getElementById(id);
  }

  function clearFormError() {
    AC.setInlineError(el("custom-layout-form-error"), null);
  }

  function setFormError(message) {
    AC.setInlineError(el("custom-layout-form-error"), message);
    if (message && typeof AC.showErrorToast === "function") {
      AC.showErrorToast(message);
    }
  }

  function getLayoutId() {
    var raw = (root.getAttribute("data-layout-id") || "").trim();
    var id = parseInt(raw, 10);
    return isNaN(id) || id < 1 ? 0 : id;
  }

  async function fetchAuth(url, options) {
    var token = AC.getToken(TOKEN_KEY);
    options = options || {};
    options.headers = Object.assign({}, options.headers || {});
    options.headers.Authorization = "Bearer " + token;
    var m = (options.method || "GET").toUpperCase();
    var isFormData =
      typeof FormData !== "undefined" && options.body instanceof FormData;
    if (
      options.body &&
      m !== "GET" &&
      m !== "HEAD" &&
      !isFormData &&
      !options.headers["Content-Type"]
    ) {
      options.headers["Content-Type"] = "application/json";
    }
    var res = await fetch(url, options);
    if (res.status === 401) {
      window.location.href = LOGIN_URL;
      throw new Error("unauthorized");
    }
    return res;
  }

  async function fetchJSON(method, url, body) {
    var opt = { method: method || "GET" };
    opt.headers = Object.assign({}, opt.headers || {}, {
      Accept: "application/json",
    });
    var m = (method || "GET").toUpperCase();
    if (body !== undefined && body !== null && m !== "GET" && m !== "HEAD") {
      opt.body = typeof body === "string" ? body : JSON.stringify(body);
    }
    return fetchAuth(url, opt);
  }

  function parseImageRefRaw(raw) {
    if (!raw) return [];
    if (Array.isArray(raw)) {
      return raw.map(function (u) {
        return String(u || "").trim();
      }).filter(Boolean);
    }
    var s = String(raw).trim();
    if (!s) return [];
    try {
      var parsed = JSON.parse(s);
      if (Array.isArray(parsed)) {
        return parsed.map(function (u) {
          return String(u || "").trim();
        }).filter(Boolean);
      }
    } catch (e) {}
    return [];
  }

  function getImageRefs() {
    var hidden = el("cl-image-ref");
    return parseImageRefRaw(hidden ? hidden.value : "[]");
  }

  function getProvider() {
    var sel = el("cl-ai-provider");
    return sel ? String(sel.value || "auto").trim() : "auto";
  }

  function getCurrentPrompt() {
    var ta = el("cl-current-prompt");
    return ta ? String(ta.value || "").trim() : "";
  }

  function getLayoutName() {
    var inp = el("cl-layout-name");
    return inp ? String(inp.value || "").trim() : "";
  }

  function fragmentLabel(fragmentKey) {
    var meta = FRAGMENTS.find(function (f) {
      return f.key === fragmentKey;
    });
    return meta ? meta.label : fragmentKey;
  }

  function validateBeforeGenerate(requireSaved) {
    clearFormError();
    if (requireSaved && !getLayoutId()) {
      setFormError("Simpan layout terlebih dahulu agar hasil generate tersimpan ke database.");
      return false;
    }
    var name = getLayoutName();
    if (!name) {
      setFormError("Nama layout wajib diisi sebelum generate.");
      return false;
    }
    var refs = getImageRefs();
    if (!refs.length) {
      setFormError("Unggah minimal satu gambar referensi di Image ref.");
      return false;
    }
    return true;
  }

  function jobProgressPct(job) {
    var total = parseInt(String(job && job.total), 10) || 0;
    var done = parseInt(String(job && job.done), 10) || 0;
    if (total < 1) return job && job.status === "running" ? 8 : 0;
    if (job && (job.status === "done" || job.status === "failed" || job.status === "canceled")) {
      return 100;
    }
    return Math.max(4, Math.min(96, Math.round((done / total) * 100)));
  }

  function jobStatusText(job) {
    if (!job) return "";
    var st = String(job.status || "").toLowerCase();
    if (st === "queued") return "Menunggu antrian…";
    if (st === "running") {
      var cur = String(job.current_label || "").trim();
      var done = parseInt(String(job.done), 10) || 0;
      var total = parseInt(String(job.total), 10) || 0;
      if (cur) return "Membuat " + cur + " (" + done + "/" + total + ")…";
      return "Sedang memproses…";
    }
    if (st === "done") {
      var ok = parseInt(String(job.succeeded), 10) || 0;
      var fail = parseInt(String(job.failed), 10) || 0;
      if (fail > 0) return "Selesai (" + ok + " berhasil, " + fail + " gagal)";
      return "Selesai (" + ok + " fragment tersimpan)";
    }
    if (st === "canceled") return "Dibatalkan";
    if (st === "failed") return String(job.error || "Gagal");
    return st;
  }

  function statusBadgeClass(status) {
    var st = String(status || "").toLowerCase();
    if (st === "done") return "text-bg-success";
    if (st === "failed") return "text-bg-danger";
    if (st === "canceled") return "text-bg-secondary";
    return "text-bg-primary";
  }

  function renderJobsPanel() {
    var panel = el("cl-ai-jobs-panel");
    if (!panel) return;

    var ids = Object.keys(trackedJobs);
    var active = ids.filter(function (id) {
      var st = String((trackedJobs[id] && trackedJobs[id].status) || "").toLowerCase();
      return st === "queued" || st === "running";
    });
    var visible = ids.filter(function (id) {
      var job = trackedJobs[id];
      if (!job) return false;
      var st = String(job.status || "").toLowerCase();
      return st === "queued" || st === "running" || st === "done" || st === "failed" || st === "canceled";
    });

    if (!visible.length) {
      panel.classList.add("d-none");
      panel.textContent = "";
      return;
    }

    panel.classList.remove("d-none");
    var html =
      '<div class="card border shadow-sm">' +
      '<div class="card-body py-3 px-3 px-md-4">' +
      '<div class="d-flex justify-content-between align-items-center mb-2">' +
      '<div class="fw-semibold small"><i class="bi bi-cpu me-1" aria-hidden="true"></i>Generate layout (background)</div>' +
      '<span class="badge text-bg-light border">' +
      active.length +
      " aktif</span></div>";

    visible.forEach(function (jobId) {
      var job = trackedJobs[jobId];
      if (!job) return;
      var pct = jobProgressPct(job);
      var st = String(job.status || "").toLowerCase();
      var canCancel = st === "queued" || st === "running";
      html +=
        '<div class="border rounded-3 p-2 mb-2 bg-body-tertiary bg-opacity-25" data-job-id="' +
        jobId +
        '">' +
        '<div class="d-flex justify-content-between align-items-start gap-2 mb-1">' +
        '<div class="small"><span class="badge ' +
        statusBadgeClass(st) +
        ' me-1">' +
        (job.label || "Generate") +
        "</span>" +
        '<span class="text-secondary">' +
        jobStatusText(job) +
        "</span></div>";
      if (canCancel) {
        html +=
          '<button type="button" class="btn btn-sm btn-light border py-0 px-2 cl-ai-job-cancel" data-job-id="' +
          jobId +
          '" title="Batalkan">×</button>';
      }
      html += "</div>" +
        '<div class="progress" style="height:0.65rem" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="' +
        pct +
        '">' +
        '<div class="progress-bar progress-bar-striped' +
        (canCancel ? " progress-bar-animated" : "") +
        '" style="width:' +
        pct +
        '%"></div></div></div>';
    });

    html += "</div></div>";
    panel.innerHTML = html;
  }

  function trackJob(job) {
    if (!job || !job.job_id) return;
    trackedJobs[job.job_id] = job;
    renderJobsPanel();
    ensurePolling();
  }

  function mergeJobUpdate(job) {
    if (!job || !job.job_id) return;
    trackedJobs[job.job_id] = job;
    renderJobsPanel();
  }

  function hasActiveJobs() {
    return Object.keys(trackedJobs).some(function (id) {
      var st = String((trackedJobs[id] && trackedJobs[id].status) || "").toLowerCase();
      return st === "queued" || st === "running";
    });
  }

  function ensurePolling() {
    if (pollTimer) return;
    pollTimer = setInterval(pollJobs, POLL_MS);
    pollJobs();
  }

  function stopPollingIfIdle() {
    if (hasActiveJobs()) return;
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  async function pollJobs() {
    var layoutId = getLayoutId();
    var url = API_GEN_JOBS + (layoutId ? "?layout_id=" + layoutId : "");
    try {
      var res = await fetchJSON("GET", url);
      var data = {};
      try {
        data = await res.json();
      } catch (e) {}
      if (!res.ok) return;

      var items = (data && data.items) || [];
      var sawActive = false;
      var needReload = false;

      items.forEach(function (job) {
        if (!job || !job.job_id) return;
        var prev = trackedJobs[job.job_id];
        trackedJobs[job.job_id] = job;
        var st = String(job.status || "").toLowerCase();
        if (st === "queued" || st === "running") sawActive = true;
        if (
          prev &&
          (prev.status === "queued" || prev.status === "running") &&
          st === "done" &&
          (parseInt(String(job.succeeded), 10) || 0) > 0
        ) {
          needReload = true;
        }
      });

      renderJobsPanel();

      if (needReload) {
        await reloadLayoutFromServer();
        if (typeof AC.showSuccessToast === "function") {
          AC.showSuccessToast("Fragment layout tersimpan ke database.");
        }
      }

      if (!sawActive) stopPollingIfIdle();
    } catch (e) {
      if (String(e && e.message) === "unauthorized") return;
    }
  }

  async function reloadLayoutFromServer() {
    var layoutId = getLayoutId();
    if (!layoutId) return;
    var res = await fetchJSON("GET", API + "/" + layoutId);
    if (!res.ok) return;
    var row = await res.json().catch(function () {
      return null;
    });
    if (!row) return;

    FRAGMENTS.forEach(function (f) {
      var field = el(f.field);
      if (!field) return;
      var val = row[f.key];
      if (val == null && f.key === "home_body") val = row.home_body;
      if (val != null) field.value = String(val);
    });

    document.dispatchEvent(
      new CustomEvent("cl-layout-ai-reloaded", { detail: { layout: row } })
    );
  }

  async function enqueueGenerateJob(payload) {
    var res = await fetchJSON("POST", API_GEN_JOBS, payload);
    var data = {};
    try {
      data = await res.json();
    } catch (e) {}
    if (res.status === 202 || res.ok) {
      if (data && data.job_id) {
        trackJob({
          job_id: data.job_id,
          status: data.status || "queued",
          total: data.total || 1,
          done: 0,
          label: data.label || payload.fragment || "Generate",
          layout_id: payload.layout_id,
          layout_name: payload.layout_name,
          mode: payload.mode,
          succeeded: 0,
          failed: 0,
        });
        ensurePolling();
        if (typeof AC.showSuccessToast === "function") {
          AC.showSuccessToast("Generate berjalan di background.");
        }
        return data;
      }
    }
    throw new Error((data && data.error) || "Gagal mengantrekan generate.");
  }

  function buildJobPayload(mode, fragmentKey) {
    return {
      layout_id: getLayoutId(),
      mode: mode,
      fragment: fragmentKey || "",
      current_prompt: getCurrentPrompt(),
      layout_name: getLayoutName(),
      provider: getProvider(),
      image_refs: getImageRefs(),
    };
  }

  async function runSingleGenerate(fragmentKey) {
    if (!validateBeforeGenerate(true)) return;
    try {
      await enqueueGenerateJob(buildJobPayload("single", fragmentKey));
    } catch (err) {
      if (err && err.message === "unauthorized") return;
      setFormError((err && err.message) || "Gagal memulai generate.");
    }
  }

  async function runGenerateAll() {
    if (!validateBeforeGenerate(true) || generateAllBusy) return;
    generateAllBusy = true;
    var allBtn = el("cl-generate-all-btn");
    if (allBtn) allBtn.disabled = true;
    try {
      await enqueueGenerateJob(buildJobPayload("all", ""));
    } catch (err) {
      if (err && err.message === "unauthorized") return;
      setFormError((err && err.message) || "Gagal memulai generate semua.");
    } finally {
      generateAllBusy = false;
      if (allBtn) allBtn.disabled = false;
    }
  }

  async function cancelJob(jobId) {
    if (!jobId) return;
    try {
      var res = await fetchJSON("POST", API_GEN_JOBS + "/" + encodeURIComponent(jobId) + "/cancel");
      var data = {};
      try {
        data = await res.json();
      } catch (e) {}
      if (res.ok && data && data.job_id) {
        mergeJobUpdate(data);
      }
    } catch (e) {
      if (String(e && e.message) === "unauthorized") return;
    }
  }

  function bindGenerateButtons() {
    root.querySelectorAll(".cl-generate-fragment-btn").forEach(function (btn) {
      btn.addEventListener("click", function () {
        var key = btn.getAttribute("data-fragment");
        if (!key) return;
        runSingleGenerate(key);
      });
    });

    var allBtn = el("cl-generate-all-btn");
    if (allBtn) {
      allBtn.addEventListener("click", function () {
        runGenerateAll();
      });
    }

    var panel = el("cl-ai-jobs-panel");
    if (panel) {
      panel.addEventListener("click", function (ev) {
        var btn = ev.target.closest(".cl-ai-job-cancel");
        if (!btn) return;
        cancelJob(btn.getAttribute("data-job-id") || "");
      });
    }
  }

  async function bootstrapJobs() {
    if (!getLayoutId()) return;
    try {
      await pollJobs();
      if (hasActiveJobs()) ensurePolling();
    } catch (e) {}
  }

  bindGenerateButtons();
  bootstrapJobs();
})();
