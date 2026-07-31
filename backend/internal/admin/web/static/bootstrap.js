(async function () {
  function loadApplication() {
    var script = document.createElement("script");
    script.src = "/admin/static/app.js?v=20260731-2";
    script.async = false;
    document.body.appendChild(script);
  }

  function currentCSRF() {
    const stored = sessionStorage.getItem("claw_admin_csrf") || "";
    if (stored) return stored;
    const prefix = "claw_admin_csrf=";
    const cookie = document.cookie.split(";").map((item) => item.trim())
      .find((item) => item.indexOf(prefix) === 0);
    return cookie ? decodeURIComponent(cookie.slice(prefix.length)) : "";
  }

  if (!currentCSRF()) {
    try {
      const response = await fetch("/admin/api/csrf", {
        method: "POST",
        credentials: "same-origin"
      });
      const result = await response.json();
      if (response.ok && result.code === 0 && result.data && result.data.csrf_token) {
        sessionStorage.setItem("claw_admin_csrf", result.data.csrf_token);
      }
    } catch (_) {
      // The authenticated API call below will redirect when the session expired.
    }
  }

  if (window.layui && typeof window.layui.use === "function") {
    window.layui.use(["layer"], loadApplication);
    return;
  }
  loadApplication();
})();
