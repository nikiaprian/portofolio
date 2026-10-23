/**
 * Shared TinyMCE init for admin post/page forms (self-hosted GPL bundle under /static/vendor/tinymce).
 * Same-origin assets work behind strict CSP and corporate proxies that block CDNs.
 * Optional: set window.TINYMCE_BASE_URL before init (no trailing slash) to override the base path.
 * Depends on global tinymce (7.x) and AdminCrud for JWT image uploads.
 */
(function (global) {
  function tinymceBaseURL() {
    var u =
      typeof global.TINYMCE_BASE_URL === "string"
        ? global.TINYMCE_BASE_URL.trim()
        : "";
    if (u) {
      return u.replace(/\/+$/, "");
    }
    return "/static/vendor/tinymce";
  }

  function readCssVar(name) {
    try {
      return String(
        getComputedStyle(document.documentElement).getPropertyValue(name) || ""
      ).trim();
    } catch (e) {
      return "";
    }
  }

  function parseToRgb(str) {
    if (!str) return null;
    str = String(str).trim();
    if (str.charAt(0) === "#") {
      var h = str.slice(1);
      if (h.length === 3) {
        h = h[0] + h[0] + h[1] + h[1] + h[2] + h[2];
      }
      if (h.length !== 6 || /[^0-9a-f]/i.test(h)) return null;
      return {
        r: parseInt(h.slice(0, 2), 16),
        g: parseInt(h.slice(2, 4), 16),
        b: parseInt(h.slice(4, 6), 16),
      };
    }
    var m = str.match(/^rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/i);
    if (m) {
      return { r: +m[1], g: +m[2], b: +m[3] };
    }
    return null;
  }

  function relLuminance(rgb) {
    function lin(c) {
      c = c / 255;
      return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
    }
    var R = lin(rgb.r);
    var G = lin(rgb.g);
    var B = lin(rgb.b);
    return 0.2126 * R + 0.7152 * G + 0.0722 * B;
  }

  /** True when site setting background_primary reads as “dark” (oxide-dark UI + matching status bar). */
  function adminEditorDarkChrome() {
    var bg = readCssVar("--admin-bg-primary");
    var rgb = parseToRgb(bg);
    if (!rgb) return false;
    return relLuminance(rgb) < 0.45;
  }

  /** Editor iframe body: match site_setting colors from :root (--admin-*). */
  function buildContentStyleFromSiteSettings() {
    var bg = readCssVar("--admin-bg-primary") || "#ffffff";
    var fg = readCssVar("--admin-text-primary") || "#2f3640";
    var sec = readCssVar("--admin-text-secondary") || "#6c757d";
    var border = readCssVar("--admin-border") || "#ced4da";
    var accent = readCssVar("--admin-accent") || "#0d6efd";
    return (
      "body { margin:10px; font-family:system-ui,-apple-system,'Segoe UI',Roboto,sans-serif;" +
      "font-size:16px;line-height:1.55;background-color:" +
      bg +
      ";color:" +
      fg +
      ";}" +
      "a{color:" +
      accent +
      ";}" +
      "table{border-collapse:collapse;}table,th,td{border:1px solid " +
      border +
      ";}" +
      "blockquote{border-left:4px solid " +
      accent +
      ";margin:1em 0;padding-left:1em;color:" +
      sec +
      ";}" +
      "h1,h2,h3,h4{color:" +
      fg +
      ";}" +
      "code,pre{border:1px solid " +
      border +
      ";border-radius:4px;padding:2px 6px;font-size:0.9em;}"
    );
  }

  global.TinyMCE = {
    /**
     * @param {object} options
     * @param {string} options.selector - e.g. '#pf-content'
     * @param {string} [options.uploadUrl]
     * @param {string} [options.tokenKey]
     * @param {string} [options.loginUrl]
     * @param {function(object)} [options.onSetup] - TinyMCE editor setup callback
     */
    init: function (options) {
      options = options || {};
      var AC = global.AdminCrud;
      if (!AC) {
        return Promise.reject(new Error("AdminCrud is not available"));
      }
      if (typeof global.tinymce === "undefined") {
        return Promise.reject(new Error("TinyMCE is not loaded"));
      }

      var selector = options.selector || "#pf-content";
      var uploadUrl = options.uploadUrl || "/api/v1/admin/posts/upload-image";
      var tokenKey = options.tokenKey || "tirtayasa_token";
      var loginUrl =
        options.loginUrl ||
        (typeof window !== "undefined" && window.TIRTAYASA_LOGIN_URL) ||
        "/login";

      var darkChrome = adminEditorDarkChrome();

      var config = {
        selector: selector,
        license_key: "gpl",
        promotion: false,
        branding: false,
        height: 480,
        min_height: 320,
        menubar: false,
        base_url: tinymceBaseURL(),
        suffix: ".min",
        skin: darkChrome ? "oxide-dark" : "oxide",
        content_css: false,
        plugins:
          "anchor autolink charmap code fullscreen help image link lists searchreplace table visualblocks wordcount",
        toolbar:
          "undo redo | blocks | bold italic underline strikethrough | alignleft aligncenter alignright alignjustify | bullist numlist outdent indent | link image table | blockquote | removeformat code",
        block_formats:
          "Paragraph=p; Heading 1=h1; Heading 2=h2; Heading 3=h3; Preformatted=pre",
        paste_data_images: true,
        automatic_uploads: true,
        relative_urls: false,
        remove_script_host: false,
        convert_urls: true,
        content_style: buildContentStyleFromSiteSettings(),
        images_upload_handler: function (blobInfo /*, progress */) {
          return new Promise(function (resolve, reject) {
            var xhr = new XMLHttpRequest();
            xhr.open("POST", uploadUrl);
            xhr.setRequestHeader(
              "Authorization",
              "Bearer " + AC.getToken(tokenKey)
            );
            xhr.onload = function () {
              if (xhr.status === 401) {
                global.location.href = loginUrl;
                reject(new Error("unauthorized"));
                return;
              }
              if (xhr.status < 200 || xhr.status >= 300) {
                reject(new Error("Upload failed (" + xhr.status + ")"));
                return;
              }
              try {
                var json = JSON.parse(xhr.responseText);
                if (json && json.image) {
                  resolve(String(json.image));
                } else {
                  reject(new Error("No image URL in response"));
                }
              } catch (e) {
                reject(e);
              }
            };
            xhr.onerror = function () {
              reject(new Error("Network error"));
            };
            var fd = new FormData();
            fd.append("image", blobInfo.blob(), blobInfo.filename());
            xhr.send(fd);
          });
        },
        setup: function (editor) {
          if (typeof options.onSetup === "function") {
            options.onSetup(editor);
          }
        },
      };

      return global.tinymce.init(config).then(function (editors) {
        if (editors && editors[0]) return editors[0];
        var id = selector.replace(/^#/, "");
        return global.tinymce.get(id);
      });
    },
  };
})(window);
