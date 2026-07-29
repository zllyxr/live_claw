(function () {
  "use strict";

  const form = document.getElementById("login-form");
  const username = document.getElementById("username");
  const password = document.getElementById("password");
  const submit = document.getElementById("login-submit");
  const errorBox = document.getElementById("login-error");
  const passwordToggle = document.getElementById("password-toggle");
  const csrfStorageKey = "claw_support_csrf";

  function showError(message) {
    errorBox.textContent = message || "";
  }

  function setBusy(busy) {
    submit.disabled = busy;
    username.disabled = busy;
    password.disabled = busy;
    submit.textContent = busy ? "正在登录…" : "安全登录";
  }

  passwordToggle.addEventListener("click", function () {
    const show = password.type === "password";
    password.type = show ? "text" : "password";
    passwordToggle.textContent = show ? "隐藏" : "显示";
    passwordToggle.setAttribute("aria-label", show ? "隐藏密码" : "显示密码");
    passwordToggle.setAttribute("aria-pressed", String(show));
    password.focus();
  });

  form.addEventListener("submit", async function (event) {
    event.preventDefault();
    showError("");

    const account = username.value.trim();
    const secret = password.value;
    if (!account || !secret) {
      showError("请输入座席账号和登录密码");
      (account ? password : username).focus();
      return;
    }

    setBusy(true);
    try {
      const response = await fetch("/support-console/api/login", {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: account, password: secret })
      });
      const envelope = await response.json().catch(function () {
        return {};
      });
      if (response.status === 429) {
        throw new Error("登录尝试过于频繁，请稍后再试");
      }
      if (!response.ok || Number(envelope.code) !== 0) {
        throw new Error(envelope.message || "账号或密码错误");
      }
      const data = envelope.data || {};
      if (data.csrf_token) {
        sessionStorage.setItem(csrfStorageKey, String(data.csrf_token));
      }
      window.location.replace("/support-console/app");
    } catch (error) {
      showError(error && error.message ? error.message : "登录暂不可用，请稍后重试");
      password.select();
    } finally {
      setBusy(false);
    }
  });
})();
