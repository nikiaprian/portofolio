(function () {
  "use strict";

  var API = "/api/v1/admin/site-connections";
  var API_TEST = "/api/v1/admin/site-connections/test-autopost";
  var TOKEN_KEY = "tirtayasa_token";
  var LOGIN_URL =
    (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) || "/login";

  var AHREFS_HELP = "https://yoa.st/ahrefs-verification-code";
  var YANDEX_HELP = "https://webmaster.yandex.com/sites/add/";
  var OAUTH_DEFAULTS = {
    facebook: {
      token_url: "https://graph.facebook.com/v24.0/oauth/access_token",
      publish_url: "https://graph.facebook.com/v24.0/{account_id}/feed",
      scope: "pages_manage_posts pages_read_engagement",
    },
    instagram: {
      token_url: "https://graph.facebook.com/v24.0/oauth/access_token",
      publish_url: "https://graph.facebook.com/v24.0/{account_id}/media",
      scope: "instagram_basic instagram_content_publish pages_show_list",
    },
    x: {
      token_url: "https://api.twitter.com/2/oauth2/token",
      publish_url: "https://api.twitter.com/2/tweets",
      scope: "tweet.write users.read offline.access",
    },
    tiktok: {
      token_url: "https://open.tiktokapis.com/v2/oauth/token/",
      publish_url: "https://open.tiktokapis.com/v2/post/publish/content/init/",
      scope: "user.info.basic video.publish",
    },
  };

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
    if (
      options.body &&
      m !== "GET" &&
      m !== "HEAD" &&
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

  /** Normalized site URL for webmaster dashboards from current runtime host. */
  function currentSiteURL() {
    if (typeof window !== "undefined" && window.location && window.location.origin) {
      return String(window.location.origin).trim();
    }
    return "";
  }

  function updateHelpLinks() {
    var siteUrl = currentSiteURL();

    var aAhrefs = qs("#sc-link-ahrefs");
    if (aAhrefs) {
      aAhrefs.href = AHREFS_HELP;
    }

    var aYandex = qs("#sc-link-yandex");
    if (aYandex) {
      aYandex.href = YANDEX_HELP;
    }

    var aGoogle = qs("#sc-link-google");
    var aBing = qs("#sc-link-bing");

    if (siteUrl) {
      if (aGoogle) {
        aGoogle.href =
          "https://search.google.com/search-console/users?hl=en&tid=alternate&siteUrl=" +
          encodeURIComponent(siteUrl);
        aGoogle.removeAttribute("aria-disabled");
      }
      if (aBing) {
        aBing.href =
          "https://www.bing.com/toolbox/webmaster/#/admin/dashboard/?url=" +
          encodeURIComponent(siteUrl);
        aBing.removeAttribute("aria-disabled");
      }
    } else {
      if (aGoogle) {
        aGoogle.href = "#";
        aGoogle.setAttribute("aria-disabled", "true");
      }
      if (aBing) {
        aBing.href = "#";
        aBing.setAttribute("aria-disabled", "true");
      }
    }
  }

  function fillForm(data) {
    if (!data) return;
    var ahrefs = qs("#sc-ahrefs");
    var google = qs("#sc-google");
    var bing = qs("#sc-bing");
    var yandex = qs("#sc-yandex");
    var autoPostEnabled = qs("#sc-auto-post-enabled");
    var autoPostTemplate = qs("#sc-auto-post-template");
    var facebookEnabled = qs("#sc-facebook-enabled");
    var facebookClientID = qs("#sc-facebook-client-id");
    var facebookClientSecret = qs("#sc-facebook-client-secret");
    var facebookTokenURL = qs("#sc-facebook-token-url");
    var facebookPublishURL = qs("#sc-facebook-publish-url");
    var facebookScope = qs("#sc-facebook-scope");
    var facebookAccountID = qs("#sc-facebook-account-id");
    var xEnabled = qs("#sc-x-enabled");
    var xClientID = qs("#sc-x-client-id");
    var xClientSecret = qs("#sc-x-client-secret");
    var xTokenURL = qs("#sc-x-token-url");
    var xPublishURL = qs("#sc-x-publish-url");
    var xScope = qs("#sc-x-scope");
    var xAccountID = qs("#sc-x-account-id");
    var tiktokEnabled = qs("#sc-tiktok-enabled");
    var tiktokClientID = qs("#sc-tiktok-client-id");
    var tiktokClientSecret = qs("#sc-tiktok-client-secret");
    var tiktokTokenURL = qs("#sc-tiktok-token-url");
    var tiktokPublishURL = qs("#sc-tiktok-publish-url");
    var tiktokScope = qs("#sc-tiktok-scope");
    var tiktokAccountID = qs("#sc-tiktok-account-id");
    var instagramEnabled = qs("#sc-instagram-enabled");
    var instagramClientID = qs("#sc-instagram-client-id");
    var instagramClientSecret = qs("#sc-instagram-client-secret");
    var instagramTokenURL = qs("#sc-instagram-token-url");
    var instagramPublishURL = qs("#sc-instagram-publish-url");
    var instagramScope = qs("#sc-instagram-scope");
    var instagramAccountID = qs("#sc-instagram-account-id");
    if (ahrefs && data.ahrefs != null) ahrefs.value = data.ahrefs;
    if (google && data.google != null) google.value = data.google;
    if (bing && data.bing != null) bing.value = data.bing;
    if (yandex && data.yandex != null) yandex.value = data.yandex;
    if (autoPostEnabled) autoPostEnabled.checked = !!data.auto_post_enabled;
    if (autoPostTemplate && data.auto_post_template != null)
      autoPostTemplate.value = data.auto_post_template;
    if (facebookEnabled) facebookEnabled.checked = !!data.facebook_enabled;
    if (facebookClientID && data.facebook_client_id != null)
      facebookClientID.value = data.facebook_client_id;
    if (facebookClientSecret && data.facebook_client_secret != null)
      facebookClientSecret.value = data.facebook_client_secret;
    if (facebookTokenURL && data.facebook_token_url != null)
      facebookTokenURL.value = data.facebook_token_url;
    if (facebookPublishURL && data.facebook_publish_url != null)
      facebookPublishURL.value = data.facebook_publish_url;
    if (facebookScope && data.facebook_scope != null)
      facebookScope.value = data.facebook_scope;
    if (facebookAccountID && data.facebook_account_id != null)
      facebookAccountID.value = data.facebook_account_id;
    if (xEnabled) xEnabled.checked = !!data.x_enabled;
    if (xClientID && data.x_client_id != null)
      xClientID.value = data.x_client_id;
    if (xClientSecret && data.x_client_secret != null)
      xClientSecret.value = data.x_client_secret;
    if (xTokenURL && data.x_token_url != null)
      xTokenURL.value = data.x_token_url;
    if (xPublishURL && data.x_publish_url != null)
      xPublishURL.value = data.x_publish_url;
    if (xScope && data.x_scope != null) xScope.value = data.x_scope;
    if (xAccountID && data.x_account_id != null)
      xAccountID.value = data.x_account_id;
    if (tiktokEnabled) tiktokEnabled.checked = !!data.tiktok_enabled;
    if (tiktokClientID && data.tiktok_client_id != null)
      tiktokClientID.value = data.tiktok_client_id;
    if (tiktokClientSecret && data.tiktok_client_secret != null)
      tiktokClientSecret.value = data.tiktok_client_secret;
    if (tiktokTokenURL && data.tiktok_token_url != null)
      tiktokTokenURL.value = data.tiktok_token_url;
    if (tiktokPublishURL && data.tiktok_publish_url != null)
      tiktokPublishURL.value = data.tiktok_publish_url;
    if (tiktokScope && data.tiktok_scope != null)
      tiktokScope.value = data.tiktok_scope;
    if (tiktokAccountID && data.tiktok_account_id != null)
      tiktokAccountID.value = data.tiktok_account_id;
    if (instagramEnabled) instagramEnabled.checked = !!data.instagram_enabled;
    if (instagramClientID && data.instagram_client_id != null)
      instagramClientID.value = data.instagram_client_id;
    if (instagramClientSecret && data.instagram_client_secret != null)
      instagramClientSecret.value = data.instagram_client_secret;
    if (instagramTokenURL && data.instagram_token_url != null)
      instagramTokenURL.value = data.instagram_token_url;
    if (instagramPublishURL && data.instagram_publish_url != null)
      instagramPublishURL.value = data.instagram_publish_url;
    if (instagramScope && data.instagram_scope != null)
      instagramScope.value = data.instagram_scope;
    if (instagramAccountID && data.instagram_account_id != null)
      instagramAccountID.value = data.instagram_account_id;
    updateHelpLinks();
  }

  function applyOAuthDefaults() {
    ["facebook", "instagram", "x", "tiktok"].forEach(function (platform) {
      var tokenEl = qs("#sc-" + platform + "-token-url");
      var publishEl = qs("#sc-" + platform + "-publish-url");
      var scopeEl = qs("#sc-" + platform + "-scope");
      if (tokenEl && !String(tokenEl.value || "").trim()) {
        tokenEl.value = OAUTH_DEFAULTS[platform].token_url;
      }
      if (publishEl && !String(publishEl.value || "").trim()) {
        publishEl.value = OAUTH_DEFAULTS[platform].publish_url;
      }
      if (scopeEl && !String(scopeEl.value || "").trim()) {
        scopeEl.value = OAUTH_DEFAULTS[platform].scope;
      }
    });
  }

  function lockHardcodedOAuthFields() {
    ["facebook", "instagram", "x", "tiktok"].forEach(function (platform) {
      ["token-url", "publish-url", "scope"].forEach(function (field) {
        var el = qs("#sc-" + platform + "-" + field);
        if (!el) return;
        el.setAttribute("readonly", "readonly");
        el.classList.add("bg-light");
      });
    });
  }

  function renderPublishPreview(platform) {
    var previewEl = qs("#sc-" + platform + "-publish-preview");
    if (!previewEl) return;
    var template = (OAUTH_DEFAULTS[platform] && OAUTH_DEFAULTS[platform].publish_url) || "";
    var accountIdEl = qs("#sc-" + platform + "-account-id");
    var accountId = (accountIdEl && accountIdEl.value) || "";
    accountId = String(accountId).trim();
    if (!template) {
      previewEl.textContent = "-";
      return;
    }
    if (!accountId) {
      previewEl.textContent = template;
      return;
    }
    previewEl.textContent = template
      .replace(/\{account_id\}/g, encodeURIComponent(accountId))
      .replace(/\{\{account_id\}\}/g, encodeURIComponent(accountId));
  }

  function bindPublishPreviewListeners() {
    ["facebook", "instagram", "x", "tiktok"].forEach(function (platform) {
      var input = qs("#sc-" + platform + "-account-id");
      if (!input) return;
      input.addEventListener("input", function () {
        renderPublishPreview(platform);
      });
      input.addEventListener("change", function () {
        renderPublishPreview(platform);
      });
      renderPublishPreview(platform);
    });
  }

  function validateEnabledPlatformOAuth() {
    var checks = [
      { key: "facebook", label: "Facebook" },
      { key: "instagram", label: "Instagram" },
      { key: "x", label: "X" },
      { key: "tiktok", label: "TikTok" },
    ];
    for (var i = 0; i < checks.length; i++) {
      var it = checks[i];
      var enabled = !!(
        qs("#sc-" + it.key + "-enabled") &&
        qs("#sc-" + it.key + "-enabled").checked
      );
      if (!enabled) continue;
      var missing = [];
      var cid =
        (qs("#sc-" + it.key + "-client-id") &&
          qs("#sc-" + it.key + "-client-id").value) ||
        "";
      var sec =
        (qs("#sc-" + it.key + "-client-secret") &&
          qs("#sc-" + it.key + "-client-secret").value) ||
        "";
      if (!cid.trim()) missing.push("client_id");
      if (!sec.trim()) missing.push("client_secret");
      if (missing.length > 0) {
        return it.label + " is enabled but missing: " + missing.join(", ");
      }
    }
    return "";
  }

  document.addEventListener("DOMContentLoaded", function () {
    if (!getToken()) {
      return;
    }

    var form = qs("#form-site-connections");
    if (!form) return;

    var errEl = qs("#site-connections-error");
    var submitBtn = qs("#sc-submit");
    var testBtn = qs("#sc-test-autopost");
    fetchAuth(API, { method: "GET" })
      .then(function (res) {
        if (res.ok) return res.json();
        return null;
      })
      .then(function (data) {
        if (data) fillForm(data);
        else updateHelpLinks();
        applyOAuthDefaults();
        lockHardcodedOAuthFields();
        bindPublishPreviewListeners();
      })
      .catch(function (e) {
        if (e && e.message === "unauthorized") return;
        updateHelpLinks();
        applyOAuthDefaults();
        lockHardcodedOAuthFields();
        bindPublishPreviewListeners();
      });

    function blockEmptyDashboardClick(ev) {
      if (!currentSiteURL()) {
        ev.preventDefault();
      }
    }
    var aGoogle = qs("#sc-link-google");
    var aBing = qs("#sc-link-bing");
    if (aGoogle) aGoogle.addEventListener("click", blockEmptyDashboardClick);
    if (aBing) aBing.addEventListener("click", blockEmptyDashboardClick);

    if (testBtn) {
      testBtn.addEventListener("click", function () {
        setGlobalError(errEl, null);
        testBtn.disabled = true;
        testBtn.setAttribute("aria-busy", "true");
        fetchAuth(API_TEST, {
          method: "POST",
          body: JSON.stringify({
            title: "Manual autopost test",
            slug: "test-autopost-manual",
            content:
              "Test content to verify the autopost webhook is delivered.",
          }),
        })
          .then(async function (res) {
            if (res.ok) {
              showSuccessToast("Test autopost sent successfully.");
              return;
            }
            var msg = "Failed to test autopost (" + res.status + ")";
            try {
              var j = await res.json();
              if (j && j.error) msg = j.error;
            } catch (x) {}
            setGlobalError(errEl, msg);
          })
          .catch(function (e) {
            if (e && e.message === "unauthorized") return;
            setGlobalError(errEl, "Network error. Please try again.");
          })
          .finally(function () {
            testBtn.disabled = false;
            testBtn.removeAttribute("aria-busy");
          });
      });
    }

    form.addEventListener("submit", function (e) {
      e.preventDefault();
      setGlobalError(errEl, null);
      applyOAuthDefaults();
      var validationErr = validateEnabledPlatformOAuth();
      if (validationErr) {
        setGlobalError(errEl, validationErr);
        return;
      }

      var body = {
        ahrefs: (qs("#sc-ahrefs") && qs("#sc-ahrefs").value) || "",
        google: (qs("#sc-google") && qs("#sc-google").value) || "",
        bing: (qs("#sc-bing") && qs("#sc-bing").value) || "",
        yandex: (qs("#sc-yandex") && qs("#sc-yandex").value) || "",
        auto_post_enabled: !!(
          qs("#sc-auto-post-enabled") && qs("#sc-auto-post-enabled").checked
        ),
        auto_post_template:
          (qs("#sc-auto-post-template") &&
            qs("#sc-auto-post-template").value) ||
          "",
        facebook_enabled: !!(
          qs("#sc-facebook-enabled") && qs("#sc-facebook-enabled").checked
        ),
        facebook_client_id:
          (qs("#sc-facebook-client-id") &&
            qs("#sc-facebook-client-id").value) ||
          "",
        facebook_client_secret:
          (qs("#sc-facebook-client-secret") &&
            qs("#sc-facebook-client-secret").value) ||
          "",
        facebook_token_url:
          (qs("#sc-facebook-token-url") &&
            qs("#sc-facebook-token-url").value) ||
          "",
        facebook_publish_url:
          (qs("#sc-facebook-publish-url") &&
            qs("#sc-facebook-publish-url").value) ||
          "",
        facebook_scope:
          (qs("#sc-facebook-scope") && qs("#sc-facebook-scope").value) || "",
        facebook_account_id:
          (qs("#sc-facebook-account-id") &&
            qs("#sc-facebook-account-id").value) ||
          "",
        x_enabled: !!(qs("#sc-x-enabled") && qs("#sc-x-enabled").checked),
        x_client_id:
          (qs("#sc-x-client-id") && qs("#sc-x-client-id").value) || "",
        x_client_secret:
          (qs("#sc-x-client-secret") && qs("#sc-x-client-secret").value) || "",
        x_token_url:
          (qs("#sc-x-token-url") && qs("#sc-x-token-url").value) || "",
        x_publish_url:
          (qs("#sc-x-publish-url") && qs("#sc-x-publish-url").value) || "",
        x_scope: (qs("#sc-x-scope") && qs("#sc-x-scope").value) || "",
        x_account_id:
          (qs("#sc-x-account-id") && qs("#sc-x-account-id").value) || "",
        tiktok_enabled: !!(
          qs("#sc-tiktok-enabled") && qs("#sc-tiktok-enabled").checked
        ),
        tiktok_client_id:
          (qs("#sc-tiktok-client-id") && qs("#sc-tiktok-client-id").value) ||
          "",
        tiktok_client_secret:
          (qs("#sc-tiktok-client-secret") &&
            qs("#sc-tiktok-client-secret").value) ||
          "",
        tiktok_token_url:
          (qs("#sc-tiktok-token-url") && qs("#sc-tiktok-token-url").value) ||
          "",
        tiktok_publish_url:
          (qs("#sc-tiktok-publish-url") &&
            qs("#sc-tiktok-publish-url").value) ||
          "",
        tiktok_scope:
          (qs("#sc-tiktok-scope") && qs("#sc-tiktok-scope").value) || "",
        tiktok_account_id:
          (qs("#sc-tiktok-account-id") && qs("#sc-tiktok-account-id").value) ||
          "",
        instagram_enabled: !!(
          qs("#sc-instagram-enabled") && qs("#sc-instagram-enabled").checked
        ),
        instagram_client_id:
          (qs("#sc-instagram-client-id") &&
            qs("#sc-instagram-client-id").value) ||
          "",
        instagram_client_secret:
          (qs("#sc-instagram-client-secret") &&
            qs("#sc-instagram-client-secret").value) ||
          "",
        instagram_token_url:
          (qs("#sc-instagram-token-url") &&
            qs("#sc-instagram-token-url").value) ||
          "",
        instagram_publish_url:
          (qs("#sc-instagram-publish-url") &&
            qs("#sc-instagram-publish-url").value) ||
          "",
        instagram_scope:
          (qs("#sc-instagram-scope") && qs("#sc-instagram-scope").value) || "",
        instagram_account_id:
          (qs("#sc-instagram-account-id") &&
            qs("#sc-instagram-account-id").value) ||
          "",
      };

      if (submitBtn) {
        submitBtn.disabled = true;
        submitBtn.setAttribute("aria-busy", "true");
      }

      fetchAuth(API, {
        method: "PUT",
        body: JSON.stringify(body),
      })
        .then(async function (res) {
          if (res.ok) {
            var data = await res.json().catch(function () {
              return null;
            });
            if (data) fillForm(data);
            showSuccessToast("Site connections saved.");
            return;
          }
          var msg = "Failed to save (" + res.status + ")";
          try {
            var j = await res.json();
            if (j && j.error) msg = j.error;
          } catch (x) {}
          setGlobalError(errEl, msg);
        })
        .catch(function (e) {
          if (e && e.message === "unauthorized") return;
          setGlobalError(errEl, "Network error. Please try again.");
        })
        .finally(function () {
          if (submitBtn) {
            submitBtn.disabled = false;
            submitBtn.removeAttribute("aria-busy");
          }
        });
    });
  });
})();
