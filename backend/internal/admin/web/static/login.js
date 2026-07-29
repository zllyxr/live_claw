(function () {
  const form = document.getElementById("login-form");
  const errorBox = document.getElementById("login-error");

  form.addEventListener("submit", async function (event) {
    event.preventDefault();
    errorBox.textContent = "";
    const button = form.querySelector("button");
    button.disabled = true;
    button.textContent = "登录中…";
    try {
      const fields = new FormData(form);
      const response = await fetch("/admin/api/login", {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        credentials: "same-origin",
        body: JSON.stringify({
          username: fields.get("username"),
          password: fields.get("password")
        })
      });
      const result = await response.json();
      if (!response.ok || result.code !== 0) {
        throw new Error(result.message || "登录失败");
      }
      sessionStorage.setItem("claw_admin_csrf", result.data.csrf_token);
      window.location.replace("/admin/app");
    } catch (error) {
      errorBox.textContent = error.message || "登录暂不可用";
    } finally {
      button.disabled = false;
      button.textContent = "安全登录";
    }
  });
})();
