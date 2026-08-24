(function () {
  const state = { page: 1, pageSize: 20, total: 0, hasMore: false, query: "", status: "", me: null };
  const rows = document.getElementById("team-member-rows");
  const errorBox = document.getElementById("team-error");
  let searchTimer = null;

  function esc(value) {
    return String(value === undefined || value === null ? "" : value)
      .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }

  function csrfToken() {
    const stored = sessionStorage.getItem("claw_team_csrf") || "";
    if (stored) return stored;
    const cookie = document.cookie.split(";").map((item) => item.trim())
      .find((item) => item.indexOf("claw_team_csrf=") === 0);
    return cookie ? decodeURIComponent(cookie.slice("claw_team_csrf=".length)) : "";
  }

  async function api(path, options) {
    const config = Object.assign({credentials: "same-origin"}, options || {});
    config.headers = Object.assign({}, config.headers || {});
    if (config.body && typeof config.body !== "string") {
      config.headers["Content-Type"] = "application/json";
      config.body = JSON.stringify(config.body);
    }
    if (config.method && config.method !== "GET") {
      config.headers["X-CSRF-Token"] = csrfToken();
    }
    const response = await fetch(path, config);
    if (response.status === 401) {
      sessionStorage.removeItem("claw_team_csrf");
      window.location.replace("/team-console/login");
      throw new Error("登录已失效");
    }
    const result = await response.json();
    if (!response.ok || result.code !== 0) throw new Error(result.message || "操作失败");
    return result.data;
  }

  function statusTag(status) {
    const labels = {1: ["正常", "ok"], 2: ["冻结", "warn"], 3: ["已关闭", "bad"]};
    const item = labels[Number(status)] || ["未知", ""];
    return '<span class="tag ' + item[1] + '">' + item[0] + "</span>";
  }

  function formatTime(seconds) {
    const date = new Date(Number(seconds || 0) * 1000);
    return Number.isNaN(date.getTime()) ? "—" : date.toLocaleString("zh-CN", {hour12: false});
  }

  function renderMetrics() {
    const team = state.me.team;
    document.getElementById("team-title").textContent = team.name;
    document.getElementById("team-metrics").innerHTML =
      '<article class="metric-card"><span>团队前缀</span><strong>' + esc(team.code) +
      '</strong><small>邀请码前三位</small></article>' +
      '<article class="metric-card"><span>当前在队人数</span><strong>' + esc(team.member_count) +
      '</strong><small>人</small></article>' +
      '<article class="metric-card"><span>团队负责人</span><strong>' + esc(state.me.nickname) +
      '</strong><small>ID ' + esc(state.me.user_id) + "</small></article>";
  }

  async function loadMembers() {
    errorBox.textContent = "";
    rows.innerHTML = '<tr><td colspan="5" class="table-empty-cell">加载中…</td></tr>';
    const params = new URLSearchParams({page: String(state.page), page_size: String(state.pageSize)});
    if (state.query) params.set("q", state.query);
    if (state.status) params.set("status", state.status);
    try {
      const data = await api("/team-console/api/members?" + params.toString());
      state.total = Number(data.total || 0);
      state.hasMore = Boolean(data.has_more);
      rows.innerHTML = (data.items || []).map((member) =>
        "<tr><td><strong>" + esc(member.id) + "</strong></td><td>" + esc(member.nickname) +
        "</td><td>" + statusTag(member.status) + "</td><td>" + formatTime(member.joined_at) +
        "</td><td>" + (member.is_owner ? '<span class="tag ok">负责人</span>' : "成员") + "</td></tr>"
      ).join("") || '<tr><td colspan="5" class="table-empty-cell">暂无符合条件的成员</td></tr>';
      const start = state.total ? (state.page - 1) * state.pageSize + 1 : 0;
      const end = Math.min(state.page * state.pageSize, state.total);
      document.getElementById("team-member-total").textContent = "共 " + state.total + " 人";
      document.getElementById("team-member-range").textContent = "第 " + start + "–" + end + " 条";
      document.getElementById("team-page-status").textContent = "第 " + state.page + " 页";
      document.getElementById("team-prev").disabled = state.page <= 1;
      document.getElementById("team-next").disabled = !state.hasMore;
    } catch (error) {
      rows.innerHTML = '<tr><td colspan="5" class="table-empty-cell">加载失败</td></tr>';
      errorBox.textContent = error.message || "读取团队成员失败";
    }
  }

  async function initialize() {
    try {
      state.me = await api("/team-console/api/me");
      renderMetrics();
      await loadMembers();
    } catch (error) {
      errorBox.textContent = error.message || "加载团队后台失败";
    }
  }

  document.getElementById("team-refresh").addEventListener("click", loadMembers);
  document.getElementById("team-member-search").addEventListener("input", function (event) {
    window.clearTimeout(searchTimer);
    searchTimer = window.setTimeout(function () {
      state.query = String(event.target.value || "").trim();
      state.page = 1;
      void loadMembers();
    }, 250);
  });
  document.getElementById("team-member-status").addEventListener("change", function (event) {
    state.status = String(event.target.value || "");
    state.page = 1;
    void loadMembers();
  });
  document.getElementById("team-prev").addEventListener("click", function () {
    if (state.page > 1) { state.page -= 1; void loadMembers(); }
  });
  document.getElementById("team-next").addEventListener("click", function () {
    if (state.hasMore) { state.page += 1; void loadMembers(); }
  });
  document.getElementById("team-logout").addEventListener("click", async function () {
    try { await api("/team-console/api/logout", {method: "POST"}); } catch (_) { /* redirect anyway */ }
    sessionStorage.removeItem("claw_team_csrf");
    window.location.replace("/team-console/login");
  });

  void initialize();
})();
