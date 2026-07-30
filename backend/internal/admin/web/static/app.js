(function () {
  const layer = window.layui && window.layui.layer;
  const content = document.getElementById("page-content");
  const heading = document.getElementById("page-heading");
  const description = document.getElementById("page-description");
  const topTitle = document.getElementById("top-title");
  const actions = document.getElementById("page-actions");
  const state = {
    me: null,
    route: "",
    cache: {},
    tableSequence: 0,
    tables: {},
    tablePreferences: {},
    routeLoadSerial: 0
  };

  const pages = {
    dashboard: ["数据统计", "平台概览", "关键业务数据、资金与待处理事项"],
    users: ["用户管理", "用户与团队", "账号状态、团队归属和邀请码体系"],
    wallet: ["资金管理", "资金审核与流水", "充值、提现、调账及逐场游戏输赢"],
    payments: ["支付管理", "支付通道与充值", "BEpusdt 通道、充值商品、回调订单与异常人工处置"],
    games: ["游戏管理", "游戏与捕鱼场", "固定 300 桌、每桌 4 座，倍率 1 / 5 / 10"],
    live: ["抖音直播", "抖音直播间", "v2 仅允许经过审核的抖音 PAGE 直播"],
    lottery: ["彩票管理", "彩种、玩法与期号", "开奖先封盘，所有变更写入审计日志"],
    sports: ["体育管理", "赛事与盘口", "维护赛事、赔率、赛果并提交 Scheduler 结算"],
    bets: ["投注管理", "全平台投注", "统一查看彩票、体育和游戏投注与派彩"],
    im: ["IM 管理", "会话与群组", "原生单聊、群聊、成员、禁言和消息处置"],
    app: ["App 管理", "客户端版本", "原生强制更新与 WGT 静默热更新"],
    rbac: ["角色与权限", "管理员权限", "最小权限角色和管理员授权"],
    system: ["系统设置", "系统设置与审计", "配置版本控制、密钥掩码和完整审计"]
  };

  const metricLabels = [
    ["users", "用户总数", "人"],
    ["active_users", "正常用户", "人"],
    ["wallet_coin", "可用星币", "币"],
    ["frozen_coin", "冻结星币", "币"],
    ["pending_topups", "待支付充值", "笔"],
    ["pending_withdrawals", "待处理提现", "笔"],
    ["live_rooms", "在线直播间", "间"],
    ["im_groups", "有效群组", "个"],
    ["today_game_settlements", "今日游戏结算", "场"],
    ["sports_matches", "进行中/待开赛事", "场"],
    ["pending_lottery_bets", "待结算彩票注单", "笔"],
    ["pending_sports_bets", "待结算体育注单", "笔"]
  ];

  function esc(value) {
    return String(value === undefined || value === null ? "" : value)
      .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }

  function safeHTTPURL(value) {
    try {
      const url = new URL(String(value || "").trim());
      return url.protocol === "http:" || url.protocol === "https:" ? url.href : "";
    } catch (_) {
      return "";
    }
  }

  function formatNumber(value) {
    return new Intl.NumberFormat("zh-CN").format(Number(value || 0));
  }

  function formatTime(value) {
    const numeric = Number(value || 0);
    if (!numeric) return "—";
    return new Date(numeric * 1000).toLocaleString("zh-CN", { hour12: false });
  }

  function utf8ByteLength(value) {
    if (typeof TextEncoder === "function") {
      return new TextEncoder().encode(String(value || "")).length;
    }
    return unescape(encodeURIComponent(String(value || ""))).length;
  }

  function decimalEntityID(value, allowZero) {
    const normalized = String(value === undefined || value === null ? "" : value).trim();
    const pattern = allowZero ? /^(0|[1-9]\d*)$/ : /^[1-9]\d*$/;
    if (!pattern.test(normalized)) return "";
    const unsigned = normalized.replace(/^0+(?=\d)/, "");
    const maximum = "9223372036854775807";
    if (unsigned.length > maximum.length ||
        (unsigned.length === maximum.length && unsigned > maximum)) {
      return "";
    }
    return unsigned;
  }

  function requireDecimalEntityID(value, label, allowZero) {
    const normalized = decimalEntityID(value, Boolean(allowZero));
    if (!normalized) {
      throw new Error((label || "编号") + "格式无效");
    }
    return normalized;
  }

  function statusTag(value, labels) {
    const item = labels[String(value)] || [String(value), ""];
    return '<span class="tag ' + esc(item[1]) + '">' + esc(item[0]) + "</span>";
  }

  function has(permission) {
    return Boolean(state.me && state.me.permissions && state.me.permissions.includes(permission));
  }

  function csrfToken() {
    const stored = sessionStorage.getItem("claw_admin_csrf") || "";
    if (stored) return stored;
    const prefix = "claw_admin_csrf=";
    const cookie = document.cookie.split(";").map((item) => item.trim())
      .find((item) => item.indexOf(prefix) === 0);
    return cookie ? decodeURIComponent(cookie.slice(prefix.length)) : "";
  }

  async function api(path, options) {
    const config = Object.assign({ credentials: "same-origin" }, options || {});
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
      window.location.replace("/admin/login");
      throw new Error("登录已失效");
    }
    const result = await response.json();
    if (!response.ok || result.code !== 0) {
      throw new Error(result.message || "操作失败");
    }
    return result.data;
  }

  function uploadToSignedURL(url, file, headers) {
    return new Promise((resolve, reject) => {
      const request = new XMLHttpRequest();
      request.open("PUT", url, true);
      Object.keys(headers || {}).forEach((name) => request.setRequestHeader(name, headers[name]));
      request.onload = () => {
        if (request.status >= 200 && request.status < 300) {
          resolve();
          return;
        }
        reject(new Error("安装包上传失败（" + request.status + "）"));
      };
      request.onerror = () => reject(new Error("安装包上传失败，请检查存储服务与跨域配置"));
      request.send(file);
    });
  }

  function notify(message, error) {
    const text = String(message || "");
    if (layer) {
      layer.msg(esc(text), { icon: error ? 2 : 1 });
    } else {
      window.alert(text);
    }
  }

  async function mutateAndRefresh(path, options, successMessage) {
    const result = await api(path, options);
    notify(successMessage || "操作成功");
    const refreshed = await loadRoute();
    if (!refreshed) {
      notify("操作已成功，但列表刷新失败，请点击“刷新数据”重试", true);
    }
    return result;
  }

  function errorPanel(error) {
    content.innerHTML = '<div class="panel-error">' + esc(error.message || "加载失败") + "</div>";
  }

  function panel(title, subtitle, body) {
    return '<section class="panel data-panel"><div class="data-panel-head"><div><h2>' +
      esc(title) + "</h2><p>" + esc(subtitle || "") +
      '</p></div></div>' + body + "</section>";
  }

  function stripHTML(value) {
    return String(value || "")
      .replace(/<[^>]*>/g, " ")
      .replace(/&nbsp;/gi, " ")
      .replace(/&amp;/gi, "&")
      .replace(/&lt;/gi, "<")
      .replace(/&gt;/gi, ">")
      .replace(/&quot;/gi, '"')
      .replace(/&#39;/gi, "'");
  }

  function searchableRowText(row, columns) {
    const rendered = columns.map((column) => {
      if (column.label === "操作") return "";
      try {
        return typeof column.render === "function" ?
          stripHTML(column.render(row)) : String(row[column.key] ?? "");
      } catch (_) {
        return "";
      }
    });
    let serialized = "";
    try {
      serialized = JSON.stringify(row);
    } catch (_) {
      serialized = String(row);
    }
    return (rendered.join(" ") + " " + serialized).toLocaleLowerCase("zh-CN");
  }

  function tablePageNumbers(page, totalPages) {
    const pages = [];
    const start = Math.max(1, Math.min(page - 2, totalPages - 4));
    const end = Math.min(totalPages, Math.max(page + 2, 5));
    for (let value = start; value <= end; value += 1) pages.push(value);
    return pages;
  }

  function tablePreferenceKey(key, route) {
    return String(route || state.route) + ":" + key;
  }

  function remoteTableURL(key, path, extraParams, route) {
    const preferences = state.tablePreferences[tablePreferenceKey(key, route)] || {};
    const params = new URLSearchParams();
    Object.entries(extraParams || {}).forEach(([name, value]) => {
      if (value !== undefined && value !== null && String(value) !== "") {
        params.set(name, String(value));
      }
    });
    params.set("page", String(Math.max(1, Number(preferences.page || 1))));
    params.set("page_size", String([10, 20, 50, 100].includes(Number(preferences.pageSize)) ?
      Number(preferences.pageSize) : 10));
    const query = String(preferences.query || "").trim();
    if (query) params.set("q", query);
    return path + (path.includes("?") ? "&" : "?") + params.toString();
  }

  async function remoteTableData(key, path, extraParams) {
    const requestRoute = state.route;
    const preferenceKey = tablePreferenceKey(key, requestRoute);
    const data = await api(remoteTableURL(key, path, extraParams, requestRoute));
    const pageSize = Math.max(1, Number(data.page_size || 10));
    const lastPage = Math.max(1, Math.ceil(Number(data.total || 0) / pageSize));
    if (Number(data.page || 1) <= lastPage) return data;
    const preferences = state.tablePreferences[preferenceKey] || {};
    state.tablePreferences[preferenceKey] = Object.assign({}, preferences, { page: lastPage });
    return api(remoteTableURL(key, path, extraParams, requestRoute));
  }

  async function fetchAllRemoteItems(path, extraParams) {
    const items = [];
    let page = 1;
    while (true) {
      const params = new URLSearchParams();
      Object.entries(extraParams || {}).forEach(([name, value]) => {
        if (value !== undefined && value !== null && String(value) !== "") {
          params.set(name, String(value));
        }
      });
      params.set("page", String(page));
      params.set("page_size", "100");
      const data = await api(path + (path.includes("?") ? "&" : "?") + params.toString());
      const batch = Array.isArray(data.items) ? data.items : [];
      items.push(...batch);
      if (!data.has_more) return items;
      page += 1;
      if (page > 10000 || !batch.length) {
        throw new Error("全量列表加载异常，请稍后重试");
      }
    }
  }

  function tableViewButton(tableID, rowIndex) {
    return '<button type="button" class="layui-btn layui-btn-sm layui-btn-primary table-view-button" ' +
      'data-action="table-view" data-table-id="' + esc(tableID) +
      '" data-row-index="' + esc(rowIndex) + '">查看</button>';
  }

  function withTableViewAction(value, tableID, rowIndex) {
    const view = tableViewButton(tableID, rowIndex);
    const rendered = String(value || "").trim();
    if (!rendered || rendered === "—") {
      return '<div class="row-actions">' + view + "</div>";
    }
    if (/^<div class="row-actions">[\s\S]*<\/div>$/.test(rendered)) {
      return rendered.replace(/<\/div>$/, view + "</div>");
    }
    return '<div class="row-actions">' + rendered + view + "</div>";
  }

  function renderTable(model) {
    const query = String(model.query || "").trim().toLocaleLowerCase("zh-CN");
    const indexedRows = model.rows.map((row, rowIndex) => ({ row, rowIndex }));
    const filteredRows = model.remote ? indexedRows : query ? indexedRows.filter((item) =>
      searchableRowText(item.row, model.columns).includes(query)) : indexedRows;
    const total = model.remote ? Number(model.total || 0) : filteredRows.length;
    const totalPages = Math.max(1, Math.ceil(total / model.pageSize));
    model.page = Math.max(1, Math.min(Number(model.page || 1), totalPages));
    const offset = (model.page - 1) * model.pageSize;
    const pageRows = model.remote ? filteredRows : filteredRows.slice(offset, offset + model.pageSize);
    state.tablePreferences[model.preferenceKey] = {
      query: model.query,
      page: model.page,
      pageSize: model.pageSize
    };

    const actionColumnIndex = model.columns.findIndex((column) => column.label === "操作");
    const displayColumns = actionColumnIndex === -1 ?
      model.columns.concat([{ label: "操作", generatedAction: true }]) : model.columns;
    const head = displayColumns.map((column) => "<th>" + esc(column.label) + "</th>").join("");
    const body = pageRows.length ? pageRows.map((item) => {
      return "<tr>" + displayColumns.map((column, columnIndex) => {
        let value;
        if (column.generatedAction) {
          value = '<div class="row-actions">' +
            tableViewButton(model.id, item.rowIndex) + "</div>";
        } else {
          value = typeof column.render === "function" ?
            column.render(item.row) : esc(item.row[column.key]);
          if (columnIndex === actionColumnIndex) {
            value = withTableViewAction(value, model.id, item.rowIndex);
          }
        }
        return '<td class="' + esc(column.className || "") + '">' + value + "</td>";
      }).join("") + "</tr>";
    }).join("") : '<tr><td class="table-empty-cell" colspan="' +
      esc(displayColumns.length) + '">' + (query ? "没有符合检索条件的数据" : "暂无数据") + "</td></tr>";

    const pageButtons = tablePageNumbers(model.page, totalPages).map((page) =>
      '<button type="button" class="table-page-number' + (page === model.page ? " active" : "") +
      '" data-action="table-page" data-table-id="' + esc(model.id) + '" data-page="' +
      esc(page) + '" aria-current="' + (page === model.page ? "page" : "false") + '">' +
      esc(page) + "</button>").join("");
    const from = total && pageRows.length ? offset + 1 : 0;
    const to = total && pageRows.length ? Math.min(offset + pageRows.length, total) : 0;
    const sizeOptions = [10, 20, 50, 100].map((size) =>
      '<option value="' + size + '"' + (size === model.pageSize ? " selected" : "") +
      ">" + size + "</option>").join("");

    return '<div class="admin-data-table" id="' + esc(model.id) +
      '" data-admin-table="' + esc(model.id) + '">' +
      '<div class="table-toolbar"><label class="table-search">' +
      '<span>检索</span><input type="search" data-table-search="' + esc(model.id) +
      '" value="' + esc(model.query) + '" autocomplete="off" placeholder="' +
      (model.remote ? "检索全部记录" : "检索当前已加载列表") + '" ' +
      'aria-label="检索当前列表"></label>' +
      '<label class="table-page-size"><span>每页</span><select data-table-page-size="' +
      esc(model.id) + '" aria-label="每页显示数量">' + sizeOptions +
      '</select><span>条</span></label>' +
      '<span class="table-result-count">' + (model.remote ? "共 " : "当前已加载 ") +
      '<strong>' + esc(total) + "</strong> 条" +
      (query ? "（已检索）" : "") + "</span></div>" +
      '<div class="table-wrap"><table class="admin-table"><thead><tr>' +
      head + "</tr></thead><tbody>" + body + "</tbody></table></div>" +
      '<div class="table-pagination"><span class="table-range">显示 ' + esc(from) +
      " - " + esc(to) + "，共 " + esc(total) + " 条</span>" +
      '<div class="table-page-controls">' +
      '<button type="button" data-action="table-page" data-table-id="' + esc(model.id) +
      '" data-page="' + esc(model.page - 1) + '"' + (model.page <= 1 ? " disabled" : "") +
      '>上一页</button>' + pageButtons +
      '<button type="button" data-action="table-page" data-table-id="' + esc(model.id) +
      '" data-page="' + esc(model.page + 1) + '"' +
      (model.page >= totalPages ? " disabled" : "") + '>下一页</button></div>' +
      '<span class="table-page-status">第 <strong>' + esc(model.page) +
      "</strong> / " + esc(totalPages) + " 页</span></div></div>";
  }

  function table(columns, rows, options) {
    const config = options || {};
    const sequence = ++state.tableSequence;
    const stableKey = String(config.key || ("table-" + sequence));
    const preferenceKey = tablePreferenceKey(stableKey);
    const preferences = state.tablePreferences[preferenceKey] || {};
    const tableID = "admin-table-" + state.route + "-" + sequence;
    const pageSize = [10, 20, 50, 100].includes(Number(preferences.pageSize)) ?
      Number(preferences.pageSize) :
      [10, 20, 50, 100].includes(Number(config.pageSize)) ? Number(config.pageSize) : 10;
    const model = {
      id: tableID,
      key: stableKey,
      preferenceKey,
      columns: Array.isArray(columns) ? columns : [],
      rows: Array.isArray(rows) ? rows : [],
      query: String(preferences.query || ""),
      page: Number(preferences.page || config.page || 1),
      pageSize,
      total: Number(config.total ?? (Array.isArray(rows) ? rows.length : 0)),
      hasMore: Boolean(config.hasMore),
      remote: config.remote || null,
      searchTimer: null,
      requestSerial: 0
    };
    model.confirmed = {
      rows: model.rows,
      query: model.query,
      page: model.page,
      pageSize: model.pageSize,
      total: model.total,
      hasMore: model.hasMore
    };
    state.tables[tableID] = model;
    return renderTable(model);
  }

  function refreshTable(tableID, focusSearch) {
    const model = state.tables[tableID];
    const root = document.getElementById(tableID);
    if (!model || !root) return;
    root.outerHTML = renderTable(model);
    if (!focusSearch) return;
    const input = document.querySelector('[data-table-search="' + tableID + '"]');
    if (input) {
      input.focus();
      const end = input.value.length;
      input.setSelectionRange(end, end);
    }
  }

  async function loadRemoteTable(model, focusSearch) {
    if (!model || !model.remote) return;
    const serial = ++model.requestSerial;
    const root = document.getElementById(model.id);
    root?.classList.add("loading");
    const params = Object.assign({}, model.remote.params || {}, {
      page: model.page,
      page_size: model.pageSize
    });
    const query = String(model.query || "").trim();
    if (query) params.q = query;
    const search = new URLSearchParams();
    Object.entries(params).forEach(([name, value]) => {
      if (value !== undefined && value !== null && String(value) !== "") {
        search.set(name, String(value));
      }
    });
    try {
      const data = await api(model.remote.path +
        (model.remote.path.includes("?") ? "&" : "?") + search.toString());
      if (serial !== model.requestSerial) return;
      model.rows = Array.isArray(data.items) ? data.items : [];
      model.page = Math.max(1, Number(data.page || model.page));
      model.pageSize = [10, 20, 50, 100].includes(Number(data.page_size)) ?
        Number(data.page_size) : model.pageSize;
      model.total = Number(data.total ?? model.rows.length);
      model.hasMore = Boolean(data.has_more);
      const lastPage = Math.max(1, Math.ceil(model.total / model.pageSize));
      if (model.page > lastPage) {
        model.page = lastPage;
        await loadRemoteTable(model, focusSearch);
        return;
      }
      model.confirmed = {
        rows: model.rows,
        query: model.query,
        page: model.page,
        pageSize: model.pageSize,
        total: model.total,
        hasMore: model.hasMore
      };
      if (model.remote.cacheName) {
        state.cache[model.remote.cacheName] = model.rows;
      }
      refreshTable(model.id, focusSearch);
    } catch (error) {
      if (serial !== model.requestSerial) return;
      Object.assign(model, model.confirmed);
      refreshTable(model.id, focusSearch);
      notify(error.message || "列表加载失败", true);
    }
  }

  function openTableRowDetails(tableID, rowIndex) {
    const model = state.tables[tableID];
    const row = model && model.rows[Number(rowIndex)];
    if (!model || !row) {
      notify("该条数据已刷新，请重新打开", true);
      return;
    }
    const fields = model.columns.filter((column) => column.label !== "操作").map((column) => {
      let value;
      try {
        value = typeof column.render === "function" ?
          column.render(row) : esc(row[column.key]);
      } catch (_) {
        value = "—";
      }
      return '<div class="table-detail-field"><dt>' + esc(column.label) +
        "</dt><dd>" + (String(value).trim() || "—") + "</dd></div>";
    }).join("");
    let record = "";
    try {
      record = JSON.stringify(row, null, 2);
    } catch (_) {
      record = String(row);
    }
    const html = '<div class="table-detail-modal"><dl>' + fields +
      '</dl><details><summary>查看完整记录</summary><pre class="json-block">' +
      esc(record) + "</pre></details></div>";
    if (!layer) {
      window.alert(stripHTML(fields) + "\n\n" + record);
      return;
    }
    layer.open({
      type: 1,
      title: "查看详情",
      skin: "table-detail-layer",
      area: ["720px", "min(760px, calc(100vh - 48px))"],
      content: html,
      btn: ["关闭"]
    });
  }

  function button(label, action, id, style) {
    return '<button type="button" class="layui-btn layui-btn-sm ' + (style || "layui-btn-primary") +
      '" data-action="' + esc(action) + '" data-id="' + esc(id) + '">' + esc(label) + "</button>";
  }

  function userActionButtons(row) {
    const buttons = [];
    if (has("users.write")) {
      buttons.push(button("编辑资料", "user-edit", row.id, "layui-btn-normal"));
      buttons.push(button("重置密码", "user-password", row.id, "layui-btn-warm"));
      buttons.push(button("状态", "user-status", row.id));
      buttons.push(button("团队", "user-team", row.id));
    }
    if (has("wallet.adjust")) {
      buttons.push(button("调账", "user-wallet-adjustment", row.id, "layui-btn-normal"));
    }
    return buttons.length ? '<div class="row-actions">' + buttons.join("") + "</div>" : "—";
  }

  function setHeader(route) {
    const meta = pages[route] || pages.dashboard;
    topTitle.textContent = meta[0];
    heading.textContent = meta[1];
    description.textContent = meta[2];
    document.querySelectorAll(".menu-item").forEach((item) => {
      item.classList.toggle("active", item.getAttribute("href") === "#" + route);
    });
    actions.innerHTML = '<button class="layui-btn layui-btn-primary" data-action="refresh">刷新数据</button>';
  }

  function isCurrentRouteLoad(loadContext) {
    return Boolean(loadContext &&
      loadContext.serial === state.routeLoadSerial &&
      loadContext.route === state.route);
  }

  async function dashboard(loadContext) {
    const data = await api("/admin/api/dashboard");
    if (!isCurrentRouteLoad(loadContext)) return;
    const metrics = '<section class="metric-grid">' + metricLabels.map((item) =>
      '<article class="metric-card"><span>' + item[1] + "</span><strong>" +
      formatNumber(data[item[0]]) + "</strong><small>" + item[2] + "</small></article>"
    ).join("") + "</section>";
    const architecture = panel("v2 本地架构状态", "前端与后台均读取 claw_v2，旧业务接口不再参与运行", [
      ["聚合首页 API", "已接入"], ["不可变资金账本", "已接入"],
      ["抖音单一直播源", "已接入"], ["原生 Go IM", "已接入"],
      ["彩票/体育账本", "已接入"], ["后台 RBAC 与审计", "已接入"]
    ].map((item) => '<div><span>' + item[0] + "</span><strong>" + item[1] + "</strong></div>")
      .join("").replace(/^/, '<div class="migration-list">').concat("</div>"));
    content.innerHTML = metrics + architecture;
  }

  async function users(loadContext) {
    const result = await Promise.all([
      remoteTableData("users", "/admin/api/users"),
      remoteTableData("teams", "/admin/api/teams")
    ]);
    if (!isCurrentRouteLoad(loadContext)) return;
    state.cache.users = result[0].items;
    state.cache.teams = result[1].items;
    state.cache.teamOptions = null;
    state.cache.teamOptionsPromise = null;
    if (has("users.write")) {
      actions.insertAdjacentHTML("afterbegin",
        '<button class="layui-btn" data-action="team-create">新建团队</button>');
    }
    const userTable = table([
      { label: "用户", render: (row) => "<strong>" + esc(row.nickname || row.username) +
        "</strong><br><small>ID " + esc(row.id) + " · " + esc(row.username) + "</small>", className: "wrap" },
      { label: "联系方式", render: (row) => esc(row.mobile || row.email || "—") },
      { label: "团队", render: (row) => row.team_code ? esc(row.team_code + " · " + row.team_name) : "未分组" },
      { label: "可用 / 冻结", render: (row) => formatNumber(row.available) + " / " + formatNumber(row.frozen) },
      { label: "状态", render: (row) => statusTag(row.status, {
        1: ["正常", "ok"], 2: ["冻结", "warn"], 3: ["关闭", "bad"]
      }) },
      { label: "注册时间", render: (row) => formatTime(row.created_at) },
      { label: "操作", render: userActionButtons }
    ], result[0].items, {
      key: "users",
      page: result[0].page,
      pageSize: result[0].page_size,
      total: result[0].total,
      hasMore: result[0].has_more,
      remote: { path: "/admin/api/users", cacheName: "users" }
    });
    const teamTable = table([
      { label: "代码", render: (row) => "<strong>" + esc(row.code) + "</strong>" },
      { label: "名称", key: "name" }, { label: "成员", render: (row) => formatNumber(row.member_count) },
      { label: "负责人", render: (row) =>
        String(row.owner_user_id || "0") === "0" ? "—" : esc(row.owner_user_id) },
      { label: "状态", render: (row) => statusTag(row.status, { 1: ["启用", "ok"], 0: ["停用", "bad"] }) },
      { label: "操作", render: (row) => has("users.write") ?
        button("编辑", "team-edit", row.id, "layui-btn-normal") : "—" }
    ], result[1].items, {
      key: "teams",
      page: result[1].page,
      pageSize: result[1].page_size,
      total: result[1].total,
      hasMore: result[1].has_more,
      remote: { path: "/admin/api/teams", cacheName: "teams" }
    });
    content.innerHTML = panel("用户列表", "余额、团队和账号状态", userTable) +
      panel("团队列表", "邀请码前三位即团队代码", teamTable);
  }

  async function walletView(loadContext) {
    const result = await Promise.all([
      remoteTableData("wallet-ledger", "/admin/api/wallet/ledger"),
      remoteTableData("wallet-recharges", "/admin/api/wallet/recharges"),
      remoteTableData("wallet-withdrawals", "/admin/api/wallet/withdrawals"),
      remoteTableData("wallet-adjustments", "/admin/api/wallet/adjustments")
    ]);
    if (!isCurrentRouteLoad(loadContext)) return;
    state.cache.adjustments = result[3].items;
    state.cache.recharges = result[1].items;
    state.cache.withdrawals = result[2].items;
    if (has("wallet.review")) {
      actions.insertAdjacentHTML("afterbegin",
        '<button class="layui-btn" data-action="adjustment-create">发起调账</button>');
    }
    const ledger = table([
      { label: "流水号", render: (row) => esc(row.entry_no) },
      { label: "用户", key: "user_id" },
      { label: "可用变动", render: (row) => (Number(row.delta_available) > 0 ? "+" : "") + formatNumber(row.delta_available) },
      { label: "冻结变动", render: (row) => (Number(row.delta_frozen) > 0 ? "+" : "") + formatNumber(row.delta_frozen) },
      { label: "业务", render: (row) => esc(row.business_type + " / " + row.business_id), className: "wrap" },
      { label: "游戏局", render: (row) => esc([row.game_code, row.venue_code, row.table_no, row.round_no].filter(Boolean).join(" · ") || "—") },
      { label: "时间", render: (row) => formatTime(row.created_at) }
    ], result[0].items, {
      key: "wallet-ledger",
      page: result[0].page,
      pageSize: result[0].page_size,
      total: result[0].total,
      hasMore: result[0].has_more,
      remote: { path: "/admin/api/wallet/ledger" }
    });
    const adjustments = table([
      { label: "申请号", key: "adjustment_no" }, { label: "用户", key: "user_id" },
      { label: "金额", render: (row) => (Number(row.amount) > 0 ? "+" : "") + formatNumber(row.amount) },
      { label: "原因", key: "reason", className: "wrap" },
      { label: "状态", render: (row) => statusTag(row.status, {
        0: ["待审核", "warn"], 2: ["已驳回", "bad"], 3: ["已入账", "ok"]
      }) },
      { label: "操作", render: (row) => has("wallet.review") && Number(row.status) === 0 ?
        '<div class="row-actions">' + button("通过", "adjustment-approve", row.id) +
        button("驳回", "adjustment-reject", row.id, "layui-btn-danger") + "</div>" : "—" }
    ], result[3].items, {
      key: "wallet-adjustments",
      page: result[3].page,
      pageSize: result[3].page_size,
      total: result[3].total,
      hasMore: result[3].has_more,
      remote: { path: "/admin/api/wallet/adjustments", cacheName: "adjustments" }
    });
    const recharge = table([
      { label: "订单", key: "order_no" }, { label: "用户", key: "user_id" },
      { label: "金额(分)", render: (row) => formatNumber(row.amount_minor) },
      { label: "星币", render: (row) => formatNumber(row.coin_amount) },
      { label: "状态", render: (row) => statusTag(row.status, {
        0: ["已创建", "warn"], 1: ["支付中", "warn"], 2: ["已支付", "ok"],
        3: ["失败", "bad"], 4: ["已关闭", "bad"], 5: ["已退款", "warn"]
      }) },
      { label: "操作", render: (row) => has("wallet.review") && [0, 1].includes(Number(row.status)) ?
        button("异常人工入账", "recharge-mark-paid", row.id, "layui-btn-danger") : "—" }
    ], result[1].items, {
      key: "wallet-recharges",
      page: result[1].page,
      pageSize: result[1].page_size,
      total: result[1].total,
      hasMore: result[1].has_more,
      remote: { path: "/admin/api/wallet/recharges", cacheName: "recharges" }
    });
    const withdrawal = table([
      { label: "订单", key: "order_no" }, { label: "用户", key: "user_id" },
      { label: "提现星币", render: (row) => formatNumber(row.coin_amount) },
      { label: "到账(分)", render: (row) => formatNumber(row.amount_minor) },
      { label: "状态", render: (row) => statusTag(row.status, {
        0: ["待审核", "warn"], 1: ["已通过", "ok"], 2: ["打款中", "warn"],
        3: ["已到账", "ok"], 4: ["已驳回", "bad"], 6: ["失败", "bad"]
      }) },
      { label: "操作", render: (row) => has("wallet.review") && [0, 1, 2].includes(Number(row.status)) ?
        button("审核", "withdraw-review", row.id) : "—" }
    ], result[2].items, {
      key: "wallet-withdrawals",
      page: result[2].page,
      pageSize: result[2].page_size,
      total: result[2].total,
      hasMore: result[2].has_more,
      remote: { path: "/admin/api/wallet/withdrawals", cacheName: "withdrawals" }
    });
    content.innerHTML = panel("资金流水", "账本不可修改；游戏流水精确到场、桌和局", ledger) +
      panel("后台调账", "申请人与审核人必须为不同管理员", adjustments) +
      '<div class="subgrid">' + panel("充值订单", "", recharge) + panel("提现订单", "", withdrawal) + "</div>";
  }

  async function paymentsView(loadContext) {
    const result = await Promise.all([
      remoteTableData("payment-channels", "/admin/api/payments/channels"),
      remoteTableData("payment-products", "/admin/api/payments/products"),
      remoteTableData("payment-recharges", "/admin/api/payments/recharges")
    ]);
    if (!isCurrentRouteLoad(loadContext)) return;
    state.cache.paymentChannels = result[0].items;
    state.cache.paymentProducts = result[1].items;
    state.cache.paymentRecharges = result[2].items;
    if (has("payments.write")) {
      actions.insertAdjacentHTML("afterbegin",
        '<button class="layui-btn" data-action="payment-product-create">新增充值商品</button>');
    }

    const channels = table([
      {
        label: "通道",
        render: (row) => "<strong>" + esc(row.name) + "</strong><br><small>" +
          esc(row.channel_key) + "</small>",
        className: "wrap"
      },
      { label: "服务商", render: (row) => statusTag(row.provider, {
        bepusdt: ["BEpusdt", "ok"]
      }) },
      {
        label: "配置",
        render: (row) => '<div class="payment-config-state">' +
          statusTag(row.config_valid ? 1 : 0, {
            1: ["配置有效", "ok"], 0: [row.config_error || "未配置", "bad"]
          }) +
          statusTag(row.token_configured ? 1 : 0, {
            1: ["Token 已配置", "ok"], 0: ["Token 未配置", "warn"]
          }) +
          statusTag(row.config_verified ? 1 : 0, {
            1: ["签名检查通过", "ok"], 0: ["待签名检查", "warn"]
          }) + "</div>"
      },
      {
        label: "接口 / 类型",
        render: (row) => esc(row.api_base_url || "—") + "<br><small>" +
          esc([row.trade_type, row.fiat].filter(Boolean).join(" · ") || "—") + "</small>",
        className: "wrap"
      },
      {
        label: "金额范围(最小单位)",
        render: (row) => formatNumber(row.min_amount_minor) + " - " +
          formatNumber(row.max_amount_minor)
      },
      { label: "排序", key: "sort_order" },
      { label: "状态", render: (row) => statusTag(row.status, {
        1: ["启用", "ok"], 0: ["停用", "bad"]
      }) },
      {
        label: "操作",
        render: (row) => {
          if (!has("payments.write")) return "—";
          const controls = [];
          if (row.provider === "bepusdt") {
            controls.push(button("编辑配置", "payment-channel-edit", row.id, "layui-btn-normal"));
            if (row.config_valid) {
              controls.push(button("协议检查", "payment-channel-check", row.id, "layui-btn-warm"));
            }
            if (Number(row.status) === 1 || row.config_verified) {
              controls.push(button(Number(row.status) === 1 ? "停用" : "启用",
                "payment-channel-status", row.id,
                Number(row.status) === 1 ? "layui-btn-danger" : "layui-btn-primary"));
            }
          } else if (Number(row.status) === 1) {
            controls.push(button("停用", "payment-channel-status", row.id, "layui-btn-danger"));
          }
          return controls.length ? '<div class="row-actions">' + controls.join("") + "</div>" : "—";
        }
      }
    ], result[0].items, {
      key: "payment-channels",
      page: result[0].page,
      pageSize: result[0].page_size,
      total: result[0].total,
      hasMore: result[0].has_more,
      remote: { path: "/admin/api/payments/channels", cacheName: "paymentChannels" }
    });

    const products = table([
      {
        label: "商品",
        render: (row) => "<strong>" + esc(row.name) + "</strong><br><small>ID " +
          esc(row.id) + "</small>"
      },
      {
        label: "支付金额",
        render: (row) => formatNumber(row.amount_minor) + " " +
          esc(row.fiat_currency) + "（精度 " + esc(row.currency_scale) + "）"
      },
      {
        label: "到账星币",
        render: (row) => formatNumber(row.coin_amount) +
          (Number(row.bonus_coin) > 0 ? " +" + formatNumber(row.bonus_coin) : "")
      },
      { label: "排序", key: "sort_order" },
      { label: "状态", render: (row) => statusTag(row.status, {
        1: ["启用", "ok"], 0: ["停用", "bad"]
      }) },
      {
        label: "操作",
        render: (row) => has("payments.write") ?
          '<div class="row-actions">' +
          button("编辑", "payment-product-edit", row.id, "layui-btn-normal") +
          button(Number(row.status) === 1 ? "停用" : "启用",
            "payment-product-status", row.id,
            Number(row.status) === 1 ? "layui-btn-danger" : "layui-btn-primary") +
          "</div>" : "—"
      }
    ], result[1].items, {
      key: "payment-products",
      page: result[1].page,
      pageSize: result[1].page_size,
      total: result[1].total,
      hasMore: result[1].has_more,
      remote: { path: "/admin/api/payments/products", cacheName: "paymentProducts" }
    });

    const recharges = table([
      {
        label: "平台订单",
        render: (row) => "<strong>" + esc(row.order_no) + "</strong><br><small>用户 " +
          esc(row.user_id) + "</small>",
        className: "wrap"
      },
      {
        label: "通道 / 服务商订单",
        render: (row) => esc(row.channel || row.channel_key || "—") +
          "<br><small>" + esc(row.provider_trade_id || "尚未生成") + "</small>",
        className: "wrap"
      },
      {
        label: "应付 / 实付",
        render: (row) => formatNumber(row.amount_minor) + " " + esc(row.fiat_currency) +
          "<br><small>" + esc(row.actual_amount || "—") + "</small>"
      },
      {
        label: "链上交易",
        render: (row) => esc(row.block_transaction_id || "—"),
        className: "wrap"
      },
      {
        label: "支付链接 / 到期",
        render: (row) => {
          const paymentURL = safeHTTPURL(row.payment_url);
          const link = paymentURL ? '<a href="' + esc(paymentURL) +
            '" target="_blank" rel="noopener noreferrer">打开支付页</a>' : "—";
          return link + "<br><small>" + formatTime(row.expires_at) + "</small>";
        },
        className: "wrap"
      },
      {
        label: "回调",
        render: (row) => formatNumber(row.callback_count) + " 次<br><small>" +
          formatTime(row.last_callback_at) + "</small>"
      },
      { label: "状态", render: (row) => statusTag(row.status, {
        0: ["已创建", "warn"], 1: ["支付中", "warn"], 2: ["已支付", "ok"],
        3: ["失败", "bad"], 4: ["已关闭", "bad"], 5: ["已退款", "warn"]
      }) },
      {
        label: "操作",
        render: (row) => has("wallet.review") && row.provider === "bepusdt" &&
          row.provider_trade_id && [0, 1].includes(Number(row.status)) ?
          button("核验补账", "payment-recharge-mark-paid", row.id, "layui-btn-danger") : "—"
      }
    ], result[2].items, {
      key: "payment-recharges",
      page: result[2].page,
      pageSize: result[2].page_size,
      total: result[2].total,
      hasMore: result[2].has_more,
      remote: { path: "/admin/api/payments/recharges", cacheName: "paymentRecharges" }
    });

    content.innerHTML =
      panel("支付通道", "密钥不回传；配置保存后会自动停用，必须通过签名与 Token 检查才可启用", channels) +
      panel("充值商品", "前端只展示启用商品；金额按法币最小单位存储", products) +
      panel("充值订单", "BEpusdt 回调自动入账；异常补账必须先由服务端核验 BEpusdt 已支付状态", recharges);
  }

  async function games(loadContext) {
    const data = await api("/admin/api/games");
    if (!isCurrentRouteLoad(loadContext)) return;
    state.cache.games = data.items;
    state.cache.venues = data.venues;
    const gameTable = table([
      { label: "游戏", render: (row) => "<strong>" + esc(row.name) + "</strong><br><small>" + esc(row.game_code) + "</small>" },
      { label: "分类", key: "category" }, { label: "玩家", render: (row) => esc(row.min_players + " - " + row.max_players) },
      { label: "在线会话", render: (row) => formatNumber(row.active_sessions) },
      { label: "状态", render: (row) => statusTag(row.status, { 1: ["上架", "ok"], 0: ["下架", "bad"] }) },
      { label: "操作", render: (row) => has("games.write") ? button("编辑", "game-edit", row.id) : "—" }
    ], data.items);
    const venueTable = table([
      { label: "场次", render: (row) => "<strong>" + esc(row.name) + "</strong><br><small>" + esc(row.venue_code) + "</small>" },
      { label: "倍率", render: (row) => "×" + esc(row.multiplier) },
      { label: "桌 / 座", render: (row) => esc(row.table_count + " / " + row.seats_per_table) },
      { label: "最低余额", render: (row) => formatNumber(row.min_balance) },
      { label: "入场冻结", render: (row) => formatNumber(row.escrow_amount) },
      { label: "在线", render: (row) => formatNumber(row.active_sessions) },
      { label: "RTP", render: (row) => (Number(row.target_rtp_ppm) / 10000).toFixed(2) + "%" },
      { label: "操作", render: (row) => has("games.write") ? button("配置", "venue-edit", row.id) : "—" }
    ], data.venues);
    content.innerHTML = panel("游戏目录", "前端入口与游戏开关", gameTable) +
      panel("深海猎手场次", "固定 300 桌、每桌 4 座，随机分配空座", venueTable);
  }

  async function liveView(loadContext) {
    const data = await remoteTableData("live-rooms", "/admin/api/live/rooms");
    if (!isCurrentRouteLoad(loadContext)) return;
    state.cache.live = data.items;
    if (has("live.write")) {
      actions.insertAdjacentHTML("afterbegin",
        '<button class="layui-btn" data-action="live-create">新增抖音房间</button>');
    }
    const roomTable = table([
      { label: "直播间", render: (row) => "<strong>" + esc(row.title) + "</strong><br><small>" + esc(row.room_no) + "</small>", className: "wrap" },
      { label: "主播", render: (row) => esc(row.nickname || row.host_name) + "<br><small>ID " + esc(row.host_user_id) + "</small>" },
      { label: "抖音房间", render: (row) => {
        const providerPage = safeHTTPURL(row.provider_page);
        const label = row.provider_room_id || row.provider_page || "—";
        return providerPage ? '<a href="' + esc(providerPage) +
          '" target="_blank" rel="noopener noreferrer">' + esc(label) + "</a>" : esc(label);
      } },
      { label: "解析", render: (row) => statusTag(row.last_resolve_status, {
        0: ["未解析", ""], 1: ["正常", "ok"], 2: ["失败", "bad"]
      }) },
      { label: "状态", render: (row) => statusTag(row.status, {
        0: ["离线", "warn"], 1: ["在线", "ok"], 2: ["停用", "bad"]
      }) },
      { label: "操作", render: (row) => has("live.write") ? button("编辑", "live-edit", row.id) : "—" }
    ], data.items, {
      key: "live-rooms",
      page: data.page,
      pageSize: data.page_size,
      total: data.total,
      hasMore: data.has_more,
      remote: { path: "/admin/api/live/rooms", cacheName: "live" }
    });
    content.innerHTML = panel("直播间列表", "数据库和接口双重限制 provider=douyin", roomTable);
  }

  async function lotteryView(loadContext) {
    const result = await Promise.all([
      api("/admin/api/lottery/catalog"),
      remoteTableData("lottery-issues", "/admin/api/lottery/issues")
    ]);
    if (!isCurrentRouteLoad(loadContext)) return;
    const data = result[0];
    const issueData = result[1];
    state.cache.lottery = data;
    state.cache.lotteryGames = data.games || [];
    state.cache.lotteryCategories = data.categories || [];
    state.cache.lotteryPlays = data.plays || [];
    state.cache.lotteryOptions = data.options || [];
    state.cache.lotteryIssues = issueData.items || [];
    const activeTab = state.cache.lotteryTab || "games";
    if (has("lottery.write")) {
      const actionByTab = {
        games: '<button class="layui-btn" data-action="lottery-game">新增彩种</button>',
        categories: '<button class="layui-btn" data-action="lottery-category">新增分类</button>',
        issues: '<button class="layui-btn layui-btn-warm" data-action="lottery-issue">新建期号</button>'
      };
      actions.insertAdjacentHTML("afterbegin", actionByTab[activeTab] || "");
    }
    const gameTable = table([
      { label: "彩种", render: (row) => "<strong>" + esc(row.name) + "</strong><br><small>" + esc(row.game_code) + "</small>" },
      { label: "分类", key: "category_name" },
      { label: "周期", render: (row) => esc(row.issue_interval_seconds) + " 秒" },
      { label: "单注范围", render: (row) => formatNumber(row.min_bet) + " - " + formatNumber(row.max_bet) },
      { label: "状态", render: (row) => statusTag(row.status, {
        1: ["启用", "ok"], 0: ["停用", "bad"]
      }) },
      { label: "最新期号", render: (row) => row.latest_issue ?
        esc(row.latest_issue.issue_no) + "<br><small>" + formatTime(row.latest_issue.draw_at) + "</small>" : "—" },
      { label: "玩法", render: (row) => formatNumber((data.plays || []).filter((play) =>
        String(play.game_id) === String(row.id)).length) + " 个" },
      { label: "操作", render: (row) => has("lottery.write") ?
        '<div class="row-actions">' + button("玩法配置", "lottery-config", row.id) +
        button("编辑", "lottery-game-edit", row.id, "layui-btn-normal") +
        button(Number(row.status) === 1 ? "停用" : "恢复",
          "lottery-game-status", row.id,
          Number(row.status) === 1 ? "layui-btn-danger" : "layui-btn-warm") + "</div>" :
        button("查看玩法", "lottery-config", row.id) }
    ], data.games);
    const categoryTable = table([
      { label: "标识", key: "category_key" }, { label: "名称", key: "name" },
      { label: "彩种数量", render: (row) => formatNumber((data.games || []).filter((game) =>
        String(game.category_id) === String(row.id)).length) },
      { label: "排序", key: "sort_order" },
      { label: "操作", render: (row) => has("lottery.write") ?
        button("编辑", "lottery-category-edit", row.id) : "—" }
    ], data.categories);
    const issueTable = table([
      { label: "彩种 / 期号", render: (row) => "<strong>" + esc(row.game_name) +
        "</strong><br><small>" + esc(row.issue_no) + "</small>", className: "wrap" },
      { label: "封盘 / 开奖", render: (row) => formatTime(row.sale_close_at) +
        "<br><small>" + formatTime(row.draw_at) + "</small>", className: "wrap" },
      { label: "订单", render: (row) => formatNumber(row.order_count) },
      { label: "投注 / 派彩", render: (row) => formatNumber(row.total_bet) +
        " / " + formatNumber(row.total_payout) },
      { label: "状态", render: (row) => statusTag(row.status, {
        0: ["待开售", ""], 1: ["销售中", "ok"], 2: ["已封盘", "warn"],
        3: ["待结算", "warn"], 4: ["已结算", "ok"], 5: ["已取消", "bad"]
      }) },
      { label: "操作", render: (row) => has("lottery.write") ?
        '<div class="row-actions">' +
        ([0, 1].includes(Number(row.status)) ? button("封盘", "lottery-close", row.id) : "") +
        (Number(row.status) === 2 ? button("开奖", "lottery-draw", row.id, "layui-btn-warm") : "") +
        "</div>" : "—" }
    ], issueData.items, {
      key: "lottery-issues",
      page: issueData.page,
      pageSize: issueData.page_size,
      total: issueData.total,
      hasMore: issueData.has_more,
      remote: { path: "/admin/api/lottery/issues", cacheName: "lotteryIssues" }
    });
    const tabs = '<div class="admin-tabs" role="tablist">' + [
      ["games", "彩种列表"], ["categories", "彩票分类"], ["issues", "期号管理"]
    ].map((item) => '<button class="admin-tab ' + (activeTab === item[0] ? "active" : "") +
      '" data-action="lottery-tab" data-id="' + item[0] + '">' + item[1] + "</button>").join("") + "</div>";
    const bodyByTab = {
      games: panel("彩种列表", "玩法只在彩种配置弹窗中维护，不铺满主页面", gameTable),
      categories: panel("彩票分类", "分类与彩种列表独立维护", categoryTable),
      issues: panel("彩票期号", "封盘、开奖和结算状态", issueTable)
    };
    content.innerHTML = tabs + (bodyByTab[activeTab] || bodyByTab.games);
  }

  async function sportsView(loadContext) {
    const results = await Promise.all([
      remoteTableData("sports-matches", "/admin/api/sports/matches"),
      api("/admin/api/sports/sync")
    ]);
    if (!isCurrentRouteLoad(loadContext)) return;
    const data = results[0];
    const sync = results[1];
    state.cache.sports = data.items;
    if (has("sports.write")) {
      actions.insertAdjacentHTML("afterbegin",
        '<button class="layui-btn" data-action="sports-create">新增赛事</button>');
    }
    const matchTable = table([
      { label: "赛事", render: (row) => "<strong>" + esc(row.competition) +
        "</strong><br><small>" + esc(row.public_match_id) + "</small>", className: "wrap" },
      { label: "对阵", render: (row) => esc(row.home_name) + " <strong>VS</strong> " +
        esc(row.away_name) + "<br><small>" + esc(row.home_score + " - " + row.away_score) + "</small>", className: "wrap" },
      { label: "开赛 / 封盘", render: (row) => formatTime(row.kickoff_at) +
        "<br><small>封盘 " + formatTime(row.bet_close_at) + "</small>", className: "wrap" },
      { label: "状态", render: (row) => statusTag(row.match_status, {
        NS: ["未开始", ""], LIVE: ["进行中", "warn"], HT: ["中场", "warn"],
        FT: ["已完赛", "ok"], CANCELLED: ["已取消", "bad"]
      }) + "<br><small>" + (Number(row.bet_status) === 1 ? "可投注" : "已停盘") + "</small>" },
      { label: "盘口 / 选项", render: (row) => esc(row.market_count + " / " + row.option_count) },
      { label: "投注", render: (row) => formatNumber(row.order_count) + " 单<br><small>" +
        formatNumber(row.total_bet) + " / 派彩 " + formatNumber(row.total_payout) + "</small>", className: "wrap" },
      { label: "结算", render: (row) => statusTag(row.settle_status, {
        0: ["未提交", ""], 1: ["待结算", "warn"], 2: ["已结算", "ok"]
      }) },
      { label: "操作", render: (row) => '<div class="row-actions">' +
        button("盘口", "sports-markets", row.id) +
        (has("sports.write") ? button("编辑", "sports-edit", row.id) +
          ([ "FT", "CANCELLED" ].includes(row.match_status) && Number(row.settle_status) !== 2 ?
            button("提交结算", "sports-settle", row.id, "layui-btn-warm") : "") : "") + "</div>" }
    ], data.items, {
      key: "sports-matches",
      page: data.page,
      pageSize: data.page_size,
      total: data.total,
      hasMore: data.has_more,
      remote: { path: "/admin/api/sports/matches", cacheName: "sports" }
    });
    const syncTable = table([
      { label: "状态", render: () => statusTag(sync.state, {
        "同步正常": ["同步正常", "ok"],
        "同步异常": ["同步异常", "bad"],
        "未配置或尚未同步": ["未配置或尚未同步", "warn"]
      }) },
      { label: "近期赛事", render: () => formatNumber(sync.future_matches) },
      { label: "有效盘口", render: () => formatNumber(sync.active_markets) },
      { label: "有效选项", render: () => formatNumber(sync.active_options) },
      { label: "最近同步", render: () => sync.logs && sync.logs.length ?
        formatTime(sync.logs[0].created_at) : "—" },
      { label: "说明", render: () => sync.logs && sync.logs.length && sync.logs[0].error_message ?
        esc(sync.logs[0].error_message) : "API-Football 真实赛事与赔率" }
    ], [sync]);
    content.innerHTML =
      panel("体育数据同步", "未配置 V2_SPORTS_API_KEY 时不会生成模拟赛事或赔率", syncTable) +
      panel("体育赛事", "后台维护赛事、封盘、赛果和结算状态", matchTable);
  }

  function betOrderTable(rows, statusLabels, options) {
    return table([
      { label: "订单", render: (row) => "<strong>" + esc(row.order_no) +
        "</strong><br><small>ID " + esc(row.id) + "</small>", className: "wrap" },
      { label: "用户", render: (row) => esc(row.nickname || "用户 " + row.user_id) +
        "<br><small>ID " + esc(row.user_id) + "</small>", className: "wrap" },
      { label: "项目 / 场次", render: (row) => esc(row.event), className: "wrap" },
      { label: "投注", render: (row) => formatNumber(row.total_bet) },
      { label: "派彩", render: (row) => formatNumber(row.total_payout) },
      { label: "净输赢", render: (row) => {
        const net = Number(row.total_payout || 0) - Number(row.total_bet || 0);
        return (net > 0 ? "+" : "") + formatNumber(net);
      } },
      { label: "状态", render: (row) => statusTag(row.status, statusLabels) },
      { label: "时间", render: (row) => formatTime(row.created_at) }
    ], rows, options);
  }

  async function betsView(loadContext) {
    const result = await Promise.all([
      api("/admin/api/bets/dashboard?page_size=1"),
      remoteTableData("bets-lottery", "/admin/api/bets/lottery"),
      remoteTableData("bets-sports", "/admin/api/bets/sports"),
      remoteTableData("bets-game", "/admin/api/bets/game")
    ]);
    if (!isCurrentRouteLoad(loadContext)) return;
    const data = result[0];
    const lotteryOrders = result[1];
    const sportsOrders = result[2];
    const gameOrders = result[3];
    const groups = [
      ["lottery", "彩票投注"], ["sports", "体育投注"], ["games", "游戏结算"]
    ];
    const metrics = '<section class="metric-grid">' + groups.map((group) => {
      const item = data.summary[group[0]] || {};
      return '<article class="metric-card"><span>' + esc(group[1]) + '</span><strong>' +
        formatNumber(item.orders) + '</strong><small>投注 ' + formatNumber(item.total_bet) +
        ' · 派彩 ' + formatNumber(item.total_payout) + ' · 净 ' + formatNumber(item.net) +
        '</small></article>';
    }).join("") + "</section>";
    const betLabels = {
      0: ["待结算", "warn"], 1: ["赢单", "ok"], 2: ["输单", "bad"],
      3: ["已退款", ""], 4: ["已取消", ""]
    };
    const gameLabels = {
      0: ["处理中", "warn"], 1: ["已结算", "ok"], 2: ["失败", "bad"]
    };
    content.innerHTML = metrics +
      panel("彩票投注订单", "按期号记录投注和派彩", betOrderTable(
        lotteryOrders.items, betLabels, {
          key: "bets-lottery",
          page: lotteryOrders.page,
          pageSize: lotteryOrders.page_size,
          total: lotteryOrders.total,
          hasMore: lotteryOrders.has_more,
          remote: { path: "/admin/api/bets/lottery" }
        })) +
      panel("体育投注订单", "按赛事记录投注和派彩", betOrderTable(
        sportsOrders.items, betLabels, {
          key: "bets-sports",
          page: sportsOrders.page,
          pageSize: sportsOrders.page_size,
          total: sportsOrders.total,
          hasMore: sportsOrders.has_more,
          remote: { path: "/admin/api/bets/sports" }
        })) +
      panel("游戏逐场结算", "精确到游戏、场次、桌号与会话", betOrderTable(
        gameOrders.items, gameLabels, {
          key: "bets-game",
          page: gameOrders.page,
          pageSize: gameOrders.page_size,
          total: gameOrders.total,
          hasMore: gameOrders.has_more,
          remote: { path: "/admin/api/bets/game" }
        }));
  }

  async function imView(loadContext) {
    const data = await remoteTableData("im-conversations", "/admin/api/im/conversations");
    if (!isCurrentRouteLoad(loadContext)) return;
    state.cache.im = data.items;
    const conversationTable = table([
      { label: "会话", render: (row) => "<strong>" + esc(row.title || row.group_no || row.id) +
        "</strong><br><small>" + esc(row.id) + "</small>", className: "wrap" },
      { label: "类型", render: (row) => ({ 1: "单聊", 2: "群聊", 3: "直播群" }[row.conversation_type] || row.conversation_type) },
      { label: "成员", render: (row) => esc(row.member_count + " / " + row.max_members) },
      { label: "消息序号", render: (row) => formatNumber(row.message_seq) },
      { label: "全员禁言", render: (row) => row.all_muted ? statusTag(1, { 1: ["已开启", "warn"] }) : "否" },
      { label: "更新时间", render: (row) => formatTime(row.updated_at) },
      { label: "操作", render: (row) => '<div class="row-actions">' +
        button("成员", "im-members", row.id) +
        (has("im.moderate") && Number(row.conversation_type) === 2 ?
          button(row.all_muted ? "解除禁言" : "全员禁言", "im-all-mute", row.id) : "") + "</div>" }
    ], data.items, {
      key: "im-conversations",
      page: data.page,
      pageSize: data.page_size,
      total: data.total,
      hasMore: data.has_more,
      remote: { path: "/admin/api/im/conversations", cacheName: "im" }
    });
    content.innerHTML = panel("会话列表", "消息记录可追溯，管理员操作全部审计", conversationTable);
  }

  async function appView(loadContext) {
    const data = await remoteTableData("app-releases", "/admin/api/app/releases");
    if (!isCurrentRouteLoad(loadContext)) return;
    state.cache.appReleases = data.items;
    if (has("app.write")) {
      actions.insertAdjacentHTML("afterbegin",
        '<button class="layui-btn" data-action="app-create">上传新版本</button>');
    }
    const releaseTable = table([
      { label: "平台", key: "platform" }, { label: "类型", key: "release_type" },
      { label: "版本", render: (row) => esc(row.version_name + " (" + row.version_code + ")") },
      { label: "最低原生版本", key: "min_native_code" },
      { label: "强制 / 静默", render: (row) => (row.force_update ? "强制" : "可选") + " / " + (row.silent_update ? "静默" : "提示") },
      { label: "灰度", render: (row) => esc(row.rollout_percent) + "%" },
      { label: "包大小", render: (row) => (Number(row.package_size || 0) / 1024 / 1024).toFixed(2) + " MB" },
      { label: "状态", render: (row) => statusTag(row.status, {
        0: ["草稿", "warn"], 1: ["已发布", "ok"], 2: ["暂停", "warn"], 3: ["已归档", ""]
      }) },
      { label: "发布时间", render: (row) => formatTime(row.published_at) },
      { label: "操作", render: (row) => {
        if (!has("app.write")) return "—";
        const status = Number(row.status);
        if (status === 0) {
          return '<div class="row-actions">' +
            button("编辑", "app-edit", row.id, "layui-btn-normal") +
            button("发布", "app-publish", row.id) +
            button("归档", "app-archive", row.id, "layui-btn-danger") + "</div>";
        }
        if (status === 1) {
          return '<div class="row-actions">' +
            button("暂停", "app-pause", row.id, "layui-btn-warm") +
            button("归档", "app-archive", row.id, "layui-btn-danger") + "</div>";
        }
        if (status === 2) {
          return '<div class="row-actions">' +
            button("恢复", "app-resume", row.id) +
            button("归档", "app-archive", row.id, "layui-btn-danger") + "</div>";
        }
        return "—";
      } }
    ], data.items, {
      key: "app-releases",
      page: data.page,
      pageSize: data.page_size,
      total: data.total,
      hasMore: data.has_more,
      remote: { path: "/admin/api/app/releases", cacheName: "appReleases" }
    });
    content.innerHTML = panel("版本记录", "安装包保存在 MinIO，发布前校验 SHA-256", releaseTable);
  }

  async function rbacView(loadContext) {
    const data = await api("/admin/api/rbac");
    if (!isCurrentRouteLoad(loadContext)) return;
    state.cache.rbac = data;
    if (has("rbac.write")) {
      actions.insertAdjacentHTML("afterbegin",
        '<button class="layui-btn" data-action="role-create">新建角色</button>' +
        '<button class="layui-btn layui-btn-normal" data-action="admin-create">新建管理员</button>');
    }
    const adminTable = table([
      { label: "账号", render: (row) => "<strong>" + esc(row.display_name || row.username) +
        "</strong><br><small>" + esc(row.username) + "</small>" },
      { label: "邮箱", render: (row) => esc(row.email || "—") },
      { label: "角色", render: (row) => esc((row.roles || []).join("、") || "未授权"), className: "wrap" },
      { label: "状态", render: (row) => statusTag(row.status, { 1: ["启用", "ok"], 0: ["停用", "bad"] }) },
      { label: "最后登录", render: (row) => formatTime(row.last_login_at) },
      { label: "操作", render: (row) => has("rbac.write") ?
        '<div class="row-actions">' +
        button("编辑授权", "admin-edit", row.id, "layui-btn-normal") +
        button("重置密码", "admin-password", row.id, "layui-btn-warm") + "</div>" : "—" }
    ], data.admins);
    const roleTable = table([
      { label: "角色", render: (row) => "<strong>" + esc(row.name) + "</strong><br><small>" + esc(row.role_key) + "</small>" },
      { label: "数据范围", render: (row) => ({ 1: "全部", 2: "团队", 3: "本人" }[row.data_scope] || row.data_scope) },
      { label: "权限", render: (row) => esc((row.permissions || []).join("、")), className: "wrap" },
      { label: "状态", render: (row) => statusTag(row.status, { 1: ["启用", "ok"], 0: ["停用", "bad"] }) },
      { label: "操作", render: (row) => has("rbac.write") &&
        !["super_admin", "support_agent", "support_supervisor"].includes(row.role_key) ?
        button("权限配置", "role-edit", row.id, "layui-btn-normal") : "—" }
    ], data.roles);
    const permissionTable = table([
      { label: "权限 ID", key: "id" },
      { label: "权限标识", render: (row) => "<strong>" + esc(row.permission_key) + "</strong>" },
      { label: "名称", key: "name" },
      { label: "模块 / 动作", render: (row) => esc(row.module + " / " + row.action) },
      { label: "说明", render: (row) => esc(row.description || "—"), className: "wrap" }
    ], data.permissions);
    content.innerHTML = panel("管理员", "管理员会话 12 小时过期，变更权限立即生效", adminTable) +
      panel("角色", "超级管理员及客服系统角色受保护，其他角色可配置权限", roleTable) +
      panel("权限字典", "角色授权时可按权限 ID 选择", permissionTable);
  }

  async function systemView(loadContext) {
    const result = await Promise.all([
      api("/admin/api/system/settings"),
      remoteTableData("system-audit", "/admin/api/system/audit")
    ]);
    if (!isCurrentRouteLoad(loadContext)) return;
    state.cache.settings = result[0].items;
    const settingTable = table([
      { label: "设置项", render: (row) => "<strong>" + esc(row.key) + "</strong>" },
      { label: "值", render: (row) => '<pre class="json-block">' + esc(JSON.stringify(row.value, null, 2)) + "</pre>", className: "wrap" },
      { label: "版本", key: "version" }, { label: "更新时间", render: (row) => formatTime(row.updated_at) },
      { label: "操作", render: (row) => has("system.write") ? button("编辑", "setting-edit", row.key) : "—" }
    ], result[0].items);
    const auditTable = table([
      { label: "时间", render: (row) => formatTime(row.created_at) },
      { label: "管理员", render: (row) => esc(row.actor_name || row.actor_id) },
      { label: "动作", key: "action" }, { label: "资源", render: (row) => esc(row.resource_type + " / " + row.resource_id) },
      { label: "请求", key: "request_id" }, { label: "IP", key: "ip" }
    ], result[1].items, {
      key: "system-audit",
      page: result[1].page,
      pageSize: result[1].page_size,
      total: result[1].total,
      hasMore: result[1].has_more,
      remote: { path: "/admin/api/system/audit" }
    });
    content.innerHTML = panel("系统设置", "密钥只显示是否已配置，不回传明文", settingTable) +
      panel("审计日志", "后台重要操作不可静默执行", auditTable);
  }

  const loaders = {
    dashboard, users, wallet: walletView, payments: paymentsView, games, live: liveView,
    lottery: lotteryView, sports: sportsView, bets: betsView,
    im: imView, app: appView, rbac: rbacView, system: systemView
  };

  function resetTableRegistry() {
    Object.values(state.tables).forEach((model) => {
      if (model.searchTimer) window.clearTimeout(model.searchTimer);
      model.requestSerial += 1;
    });
    state.tables = {};
    state.tableSequence = 0;
  }

  async function loadRoute() {
    const requestedRoute = (window.location.hash || "#dashboard").slice(1);
    const route = pages[requestedRoute] ? requestedRoute : "dashboard";
    const loadContext = { route, serial: ++state.routeLoadSerial };
    state.route = route;
    if (layer) layer.closeAll("page");
    resetTableRegistry();
    setHeader(route);
    content.innerHTML = '<div class="empty-state">正在加载…</div>';
    try {
      await loaders[route](loadContext);
      if (!isCurrentRouteLoad(loadContext)) return true;
      return true;
    } catch (error) {
      if (!isCurrentRouteLoad(loadContext)) return true;
      errorPanel(error);
      return false;
    }
  }

  function openForm(title, fields, submit) {
    if (!layer) return;
    const formID = "modal-form-" + Date.now();
    const html = '<form id="' + formID + '" class="modal-form"><div class="form-grid">' +
      fields.map((field) => {
        const selectedValues = Array.isArray(field.value) ?
          field.value.map(String) : [String(field.value ?? "")];
        const options = field.type === "checkboxes" ?
          '<div class="form-check-grid">' + (field.options || []).map((item) =>
            '<label class="form-check-option"><input type="checkbox" name="' +
            esc(field.name) + '" value="' + esc(item[0]) + '"' +
            (selectedValues.includes(String(item[0])) ? " checked" : "") +
            '><span>' + esc(item[1]) + "</span></label>").join("") + "</div>" :
          field.options ? '<select name="' + esc(field.name) + '">' +
          field.options.map((item) => '<option value="' + esc(item[0]) + '"' +
            (selectedValues.includes(String(item[0])) ? " selected" : "") + ">" + esc(item[1]) + "</option>").join("") +
          "</select>" : field.type === "textarea" ?
          '<textarea name="' + esc(field.name) + '">' + esc(field.value || "") + "</textarea>" :
          field.type === "file" ?
          '<input name="' + esc(field.name) + '" type="file" accept="' + esc(field.accept || "") + '">' :
          '<input name="' + esc(field.name) + '" type="' + esc(field.type || "text") +
          '" value="' + esc(field.value || "") + '" placeholder="' + esc(field.placeholder || "") + '"' +
          (field.inputmode ? ' inputmode="' + esc(field.inputmode) + '"' : "") +
          (field.readonly ? " readonly" : "") + ">";
        if (field.type === "checkboxes") {
          return '<div class="form-check-field ' + (field.wide ? "wide" : "") +
            '"><span class="form-check-label">' + esc(field.label) + "</span>" + options + "</div>";
        }
        return '<label class="' + (field.wide ? "wide" : "") + '">' +
          esc(field.label) + options + "</label>";
      }).join("") + "</div></form>";
    layer.open({
      type: 1, title: esc(title), area: ["620px", "auto"], content: html, btn: ["保存", "取消"],
      yes: async function (index) {
        const form = document.getElementById(formID);
        const values = {};
        new FormData(form).forEach((value, key) => {
          const normalized = value instanceof File ? value : String(value);
          if (Object.prototype.hasOwnProperty.call(values, key)) {
            values[key] = Array.isArray(values[key]) ?
              values[key].concat([normalized]) : [values[key], normalized];
            return;
          }
          values[key] = normalized;
        });
        let outcome;
        try {
          outcome = await submit(values);
        } catch (error) {
          notify(error.message || "操作失败", true);
          return;
        }
        layer.close(index);
        notify(outcome && outcome.__formMessage ? outcome.__formMessage : "操作成功");
        if (outcome && outcome.__skipRefresh) {
          if (outcome.__redirect) window.location.replace(outcome.__redirect);
          return;
        }
        try {
          const refreshed = await loadRoute();
          if (!refreshed) {
            notify("操作已成功，但列表刷新失败，请点击“刷新数据”重试", true);
          }
        } catch (_) {
          notify("操作已成功，但列表刷新失败，请点击“刷新数据”重试", true);
        }
      }
    });
  }

  function walletAdjustmentRequestID() {
    if (window.crypto && typeof window.crypto.randomUUID === "function") {
      return window.crypto.randomUUID();
    }
    if (window.crypto && typeof window.crypto.getRandomValues === "function") {
      const bytes = new Uint8Array(16);
      window.crypto.getRandomValues(bytes);
      bytes[6] = (bytes[6] & 15) | 64;
      bytes[8] = (bytes[8] & 63) | 128;
      const hex = Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
      return [hex.slice(0, 8), hex.slice(8, 12), hex.slice(12, 16),
        hex.slice(16, 20), hex.slice(20)].join("-");
    }
    return "admin-" + Date.now() + "-" + Math.random().toString(16).slice(2);
  }

  function availableBalanceFromAdjustment(result) {
    const values = [
      result && result.available,
      result && result.available_balance,
      result && result.new_balance,
      result && result.balance,
      result && result.balance && result.balance.available,
      result && result.wallet && result.wallet.available,
      result && result.wallet && result.wallet.balance,
      result && result.user && result.user.available
    ];
    return values.find((value) => value !== undefined && value !== null &&
      value !== "" && typeof value !== "object");
  }

  function openUserWalletAdjustment(user) {
    if (!layer || !user) return;
    const suffix = String(Date.now());
    const modalID = "wallet-adjustment-" + suffix;
    const formID = "wallet-adjustment-form-" + suffix;
    const amountID = "wallet-adjustment-amount-" + suffix;
    const reasonID = "wallet-adjustment-reason-" + suffix;
    let closed = false;
    let confirming = false;
    let submitting = false;
    let confirmIndex = null;
    let requestID = "";
    let requestSignature = "";
    const displayName = user.nickname || user.username || ("用户 " + user.id);
    const currentBalance = formatNumber(user.available);
    const html = '<div id="' + modalID + '" class="wallet-adjustment-modal">' +
      '<section class="wallet-adjustment-user">' +
      '<span class="wallet-adjustment-user-name">' + esc(displayName) + "</span>" +
      '<span class="wallet-adjustment-user-meta">账号 ' + esc(user.username || "—") +
      " · ID " + esc(user.id) + "</span>" +
      '<span class="tag ok wallet-adjustment-balance">当前可用余额 ' +
      esc(currentBalance) + " 星币</span></section>" +
      '<form id="' + formID + '" class="wallet-adjustment-form">' +
      '<fieldset class="wallet-adjustment-directions"><legend>调账类型</legend>' +
      '<label class="wallet-direction-option credit">' +
      '<input type="radio" name="direction" value="credit" checked>' +
      '<span><strong>充值</strong><small>增加用户可用余额</small></span></label>' +
      '<label class="wallet-direction-option debit">' +
      '<input type="radio" name="direction" value="debit">' +
      '<span><strong>扣款</strong><small>减少用户可用余额</small></span></label></fieldset>' +
      '<label class="wallet-adjustment-field" for="' + amountID + '">' +
      '<span>调整金额（正整数）</span><input id="' + amountID +
      '" name="amount" type="number" inputmode="numeric" min="1" step="1" required ' +
      'autocomplete="off" placeholder="请输入大于 0 的整数"></label>' +
      '<label class="wallet-adjustment-field" for="' + reasonID + '">' +
      '<span>调账原因（必填）</span><textarea id="' + reasonID +
      '" name="reason" maxlength="500" required placeholder="请填写本次充值或扣款的原因"></textarea></label>' +
      '<span class="wallet-adjustment-note">提交前会再次显示用户、方向、金额和原因，请认真核对。</span>' +
      "</form></div>";

    layer.open({
      type: 1,
      title: "用户调账",
      skin: "wallet-adjustment-layer",
      area: ["620px", "auto"],
      content: html,
      btn: ["核对并提交", "取消"],
      success: function () {
        const amountInput = document.getElementById(amountID);
        if (amountInput) amountInput.focus();
      },
      yes: function (index, layerElement) {
        if (closed || confirming || submitting) return;
        const form = document.getElementById(formID);
        if (!form) return;
        const values = new FormData(form);
        const direction = String(values.get("direction") || "");
        const amountText = String(values.get("amount") || "").trim();
        const reason = String(values.get("reason") || "").trim();
        const amount = Number(amountText);
        if (!["credit", "debit"].includes(direction)) {
          notify("请选择充值或扣款", true);
          return;
        }
        if (!/^[1-9]\d*$/.test(amountText) || !Number.isSafeInteger(amount)) {
          notify("金额必须是大于 0 的正整数", true);
          document.getElementById(amountID)?.focus();
          return;
        }
        if (!reason) {
          notify("请填写调账原因", true);
          document.getElementById(reasonID)?.focus();
          return;
        }
        if (reason.length > 500) {
          notify("调账原因不能超过 500 个字", true);
          document.getElementById(reasonID)?.focus();
          return;
        }
        const isCredit = direction === "credit";
        const directionText = isCredit ? "充值" : "扣款";
        const signedAmount = (isCredit ? "+" : "−") + formatNumber(amount);
        const confirmHTML = '<div class="wallet-adjustment-confirmation">' +
          '<span class="tag ' + (isCredit ? "ok" : "bad") + '">' + directionText + "</span>" +
          '<strong class="' + (isCredit ? "credit" : "debit") + '">' +
          esc(signedAmount) + " 星币</strong>" +
          '<span class="wallet-confirm-user">' + esc(displayName) + " · ID " + esc(user.id) + "</span>" +
          '<span class="wallet-confirm-label">调账原因</span><p>' + esc(reason) + "</p>" +
          '<span class="wallet-confirm-warning">确认后将立即影响用户可用余额，请再次核对。</span></div>';
        confirming = true;
        const parentLayer = layerElement && layerElement[0] ? layerElement[0] : null;
        parentLayer?.classList.add("is-confirming");
        confirmIndex = layer.confirm(confirmHTML, {
          title: "二次确认",
          skin: "wallet-adjustment-confirm-layer",
          area: ["460px", "auto"],
          btn: ["确认" + directionText, "返回修改"],
          yes: async function (confirmationIndex, confirmationElement) {
            if (submitting) return;
            submitting = true;
            const signature = [direction, amountText, reason].join("\n");
            if (!requestID || requestSignature !== signature) {
              requestID = walletAdjustmentRequestID();
              requestSignature = signature;
            }
            const confirmationLayer = confirmationElement && confirmationElement[0] ?
              confirmationElement[0] : null;
            parentLayer?.classList.add("is-submitting");
            confirmationLayer?.classList.add("is-submitting");
            const confirmationButton = confirmationLayer?.querySelector(".layui-layer-btn0");
            if (confirmationButton) confirmationButton.textContent = "正在提交…";
            const loadingIndex = layer.load(2, { shade: [0.18, "#fff"] });
            try {
              const result = await api("/admin/api/users/" + encodeURIComponent(user.id) +
                "/wallet-adjustments", {
                method: "POST",
                body: {
                  direction,
                  amount,
                  reason,
                  request_id: requestID
                }
              });
              const latestBalance = availableBalanceFromAdjustment(result);
              layer.close(confirmationIndex);
              layer.close(index);
              notify(directionText + "成功" + (latestBalance !== undefined ?
                "，最新可用余额：" + formatNumber(latestBalance) + " 星币" : "，用户余额已更新"));
              try {
                const refreshed = await loadRoute();
                if (!refreshed) {
                  notify("调账已成功，但列表刷新失败，请点击“刷新数据”重试", true);
                }
              } catch (refreshError) {
                notify("调账已成功，但列表刷新失败，请点击“刷新数据”重试", true);
              }
            } catch (error) {
              notify(error.message || "调账失败", true);
            } finally {
              submitting = false;
              parentLayer?.classList.remove("is-submitting");
              confirmationLayer?.classList.remove("is-submitting");
              if (confirmationButton && document.body.contains(confirmationButton)) {
                confirmationButton.textContent = "确认" + directionText;
              }
              layer.close(loadingIndex);
            }
          },
          end: function () {
            confirming = false;
            confirmIndex = null;
            parentLayer?.classList.remove("is-confirming");
          }
        });
      },
      end: function () {
        closed = true;
        if (confirmIndex !== null) layer.close(confirmIndex);
      }
    });
  }

  function openLiveRoomForm(room) {
    if (!layer) return;
    const editing = Boolean(room);
    const suffix = String(Date.now());
    const modalID = "live-create-modal-" + suffix;
    const searchFormID = "live-host-search-" + suffix;
    const searchInputID = "live-host-keyword-" + suffix;
    const resultsID = "live-host-results-" + suffix;
    const selectedID = "live-host-selected-" + suffix;
    const roomInputID = "live-provider-room-" + suffix;
    const statusInputID = "live-room-status-" + suffix;
    const sortInputID = "live-room-sort-" + suffix;
    let hosts = [];
    let selectedHost = editing ? {
      id: String(room.host_user_id || ""),
      username: String(room.host_username || room.unique_id || ""),
      nickname: String(room.nickname || room.host_name || ""),
      avatar_url: String(room.avatar_url || ""),
      cover_url: String(room.cover_url || ""),
      title: String((room.nickname || room.host_name || "") + "的直播间"),
      is_virtual: Boolean(room.host_is_virtual)
    } : null;
    let searchTimer;
    let requestSerial = 0;
    let closed = false;
    let saving = false;
    const roomStatus = editing ? Number(room.status || 0) : 1;
    const editFields = editing ? '<div class="live-edit-fields">' +
      '<label for="' + statusInputID + '"><span>状态</span><select id="' + statusInputID + '">' +
      '<option value="1"' + (roomStatus === 1 ? " selected" : "") + '>在线</option>' +
      '<option value="0"' + (roomStatus === 0 ? " selected" : "") + '>离线</option>' +
      '<option value="2"' + (roomStatus === 2 ? " selected" : "") + '>停用</option></select></label>' +
      '<label for="' + sortInputID + '"><span>排序</span><input id="' + sortInputID +
      '" type="number" value="' + esc(room.sort_order || 0) + '"></label></div>' : "";

    const html = '<div id="' + modalID + '" class="live-create-modal">' +
      '<form id="' + searchFormID + '" class="live-host-search">' +
      '<label for="' + searchInputID + '"><span>选择现有主播</span>' +
      '<span class="live-host-search-row"><input id="' + searchInputID +
      '" type="search" autocomplete="off" placeholder="搜索昵称、账号或完整用户 ID">' +
      '<button type="submit" class="layui-btn layui-btn-primary"><span>搜索</span></button></span></label></form>' +
      '<span class="inline-note live-create-note">仅显示启用用户；选择后昵称、头像、封面和标题由服务器自动带出。</span>' +
      '<div id="' + resultsID + '" class="live-host-results"><span class="live-host-loading">正在加载主播…</span></div>' +
      '<div id="' + selectedID + '" class="live-host-selected"><span>尚未选择主播</span></div>' +
      '<label class="live-room-field" for="' + roomInputID + '"><span>抖音房间 ID</span>' +
      '<input id="' + roomInputID + '" type="text" inputmode="numeric" maxlength="128" ' +
      'autocomplete="off" value="' + esc(editing ? room.provider_room_id : "") +
      '" placeholder="例如 826694648629"></label>' + editFields +
      '<span class="inline-note live-create-note">' + (editing ?
        "更换主播、房间 ID 或设为在线时会先探测直播流，再保存资料。" :
        "这里只填写房间 ID，不要粘贴整条网址；探测成功后会立即上线。") + "</span>" +
      "</div>";

    function hostAvatar(host) {
      const avatar = String(host.avatar_url || "");
      if (avatar) {
        return '<img src="' + esc(avatar) + '" alt="">';
      }
      return '<span class="live-host-avatar-fallback">' +
        esc(String(host.nickname || host.username || "主").slice(0, 1)) + "</span>";
    }

    function renderSelected() {
      const target = document.getElementById(selectedID);
      if (!target) return;
      if (!selectedHost) {
        target.innerHTML = "<span>尚未选择主播</span>";
        return;
      }
      target.innerHTML = '<span class="live-selected-label">已选主播</span>' +
        '<span class="live-selected-avatar">' + hostAvatar(selectedHost) + "</span>" +
        '<span class="live-selected-copy"><span class="live-selected-name">' +
        esc(selectedHost.nickname || selectedHost.username) + '</span><span class="live-selected-meta">@' +
        esc(selectedHost.username) + " · ID " + esc(selectedHost.id) + "</span><span class=\"live-selected-title\">" +
        esc(selectedHost.title || "") + "</span></span>" +
        (selectedHost.cover_url ? '<span class="live-selected-cover"><img src="' +
          esc(selectedHost.cover_url) + '" alt=""></span>' : "");
    }

    function renderHosts(data) {
      const target = document.getElementById(resultsID);
      if (!target) return;
      hosts = Array.isArray(data.items) ? data.items : [];
      if (!hosts.length) {
        target.innerHTML = '<span class="live-host-empty">没有找到符合条件的启用用户</span>';
        return;
      }
      target.innerHTML = hosts.map((host) => {
        const active = selectedHost && String(selectedHost.id) === String(host.id);
        return '<button type="button" class="live-host-option' + (active ? " selected" : "") +
          '" data-host-id="' + esc(host.id) + '" aria-pressed="' + (active ? "true" : "false") + '">' +
          '<span class="live-host-avatar">' + hostAvatar(host) + '</span><span class="live-host-copy">' +
          '<span class="live-host-name">' + esc(host.nickname || host.username) + "</span>" +
          '<span class="live-host-meta">@' + esc(host.username) + " · ID " + esc(host.id) + "</span></span>" +
          '<span class="tag ' + (host.is_virtual ? "ok" : "") + '">' +
          (host.is_virtual ? "虚拟主播" : "真实用户") + "</span></button>";
      }).join("");
      const total = Number(data.total || hosts.length);
      target.insertAdjacentHTML("beforeend", '<span class="live-host-count">共 ' +
        esc(total) + " 位，当前显示 " + esc(hosts.length) + " 位</span>");
    }

    async function loadHosts() {
      const input = document.getElementById(searchInputID);
      const target = document.getElementById(resultsID);
      if (!input || !target || closed) return;
      const serial = ++requestSerial;
      target.innerHTML = '<span class="live-host-loading">正在搜索主播…</span>';
      try {
        const data = await api("/admin/api/live/hosts?page=1&page_size=20&q=" +
          encodeURIComponent(input.value.trim()));
        if (closed || serial !== requestSerial) return;
        renderHosts(data || {});
      } catch (error) {
        if (closed || serial !== requestSerial) return;
        target.innerHTML = '<span class="live-host-error">' +
          esc(error.message || "主播加载失败") + "</span>";
      }
    }

    layer.open({
      type: 1,
      title: editing ? "编辑抖音直播间" : "新增抖音直播间",
      skin: "live-create-layer",
      area: ["760px", "min(780px, calc(100vh - 60px))"],
      content: html,
      btn: [editing ? "校验并保存" : "探测并上线", "取消"],
      success: function () {
        const root = document.getElementById(modalID);
        const searchForm = document.getElementById(searchFormID);
        const searchInput = document.getElementById(searchInputID);
        if (!root || !searchForm || !searchInput) return;
        searchForm.addEventListener("submit", function (event) {
          event.preventDefault();
          clearTimeout(searchTimer);
          void loadHosts();
        });
        searchInput.addEventListener("input", function () {
          clearTimeout(searchTimer);
          searchTimer = setTimeout(() => void loadHosts(), 300);
        });
        root.addEventListener("click", function (event) {
          const option = event.target.closest(".live-host-option");
          if (!option) return;
          selectedHost = hosts.find((host) => String(host.id) === String(option.dataset.hostId)) || null;
          root.querySelectorAll(".live-host-option").forEach((item) => {
            const active = selectedHost && String(item.dataset.hostId) === String(selectedHost.id);
            item.classList.toggle("selected", Boolean(active));
            item.setAttribute("aria-pressed", active ? "true" : "false");
          });
          renderSelected();
        });
        renderSelected();
        void loadHosts();
      },
      yes: async function (index, layerElement) {
        if (saving) return;
        const roomInput = document.getElementById(roomInputID);
        const providerRoomID = roomInput ? roomInput.value.trim() : "";
        if (!selectedHost) {
          notify("请先选择主播", true);
          return;
        }
        if (!/^[0-9A-Za-z_-]{3,128}$/.test(providerRoomID)) {
          notify("请输入正确的抖音房间 ID", true);
          roomInput?.focus();
          return;
        }
        const statusInput = document.getElementById(statusInputID);
        const sortInput = document.getElementById(sortInputID);
        const status = editing ? Number(statusInput?.value || 0) : 1;
        const sortOrder = editing ? Number(sortInput?.value || 0) : 0;
        if (![0, 1, 2].includes(status) || !Number.isInteger(sortOrder)) {
          notify("直播间状态或排序无效", true);
          return;
        }
        saving = true;
        const layerNode = layerElement && layerElement[0] ? layerElement[0] : null;
        layerNode?.classList.add("is-saving");
        const loadingIndex = layer.load(2, { shade: [0.18, "#fff"] });
        try {
          const body = {
            host_user_id: String(selectedHost.id),
            provider_room_id: providerRoomID
          };
          if (editing) {
            body.status = status;
            body.sort_order = sortOrder;
          }
          await api(editing ? "/admin/api/live/rooms/" + encodeURIComponent(room.id) :
            "/admin/api/live/rooms", {
            method: "POST",
            body
          });
          layer.close(index);
          notify(editing ? "直播间已更新" : "直播间已通过流探测并上线");
          const refreshed = await loadRoute();
          if (!refreshed) {
            notify("操作已成功，但列表刷新失败，请点击“刷新数据”重试", true);
          }
        } catch (error) {
          notify(error.message || (editing ? "更新直播间失败" : "创建直播间失败"), true);
        } finally {
          saving = false;
          layerNode?.classList.remove("is-saving");
          layer.close(loadingIndex);
        }
      },
      end: function () {
        closed = true;
        requestSerial += 1;
        clearTimeout(searchTimer);
      }
    });
  }

  function cached(name, id) {
    return (state.cache[name] || []).find((item) => String(item.id ?? item.key) === String(id));
  }

  function decimalFormIDs(value) {
    const values = Array.isArray(value) ? value : String(value || "").split(",");
    const result = [];
    const seen = new Set();
    values.map((item) => String(item).trim()).filter(Boolean).forEach((item) => {
      const id = requireDecimalEntityID(item, "编号");
      if (!seen.has(id)) {
        seen.add(id);
        result.push(id);
      }
    });
    return result;
  }

  function requirePaymentBaseURL(value, label) {
    let parsed;
    try {
      parsed = new URL(String(value || "").trim());
    } catch (_) {
      throw new Error(label + "格式无效");
    }
    if (!["http:", "https:"].includes(parsed.protocol) || !parsed.hostname ||
        parsed.username || parsed.password || parsed.search || parsed.hash ||
        (parsed.pathname !== "" && parsed.pathname !== "/")) {
      throw new Error(label + "必须是无账号、查询参数和路径的 HTTP(S) 根地址");
    }
    return parsed.href.replace(/\/$/, "");
  }

  function paymentProductForm(product) {
    const editing = Boolean(product);
    return openForm(editing ? "编辑充值商品" : "新增充值商品", [
      { name: "name", label: "商品名称", value: product?.name || "", wide: true },
      {
        name: "fiat_currency", label: "法币",
        options: [["CNY", "CNY"], ["USD", "USD"], ["EUR", "EUR"], ["GBP", "GBP"], ["JPY", "JPY"]],
        value: product?.fiat_currency || "CNY"
      },
      {
        name: "currency_scale", label: "法币精度", type: "number",
        value: product?.currency_scale ?? 2
      },
      {
        name: "amount_minor", label: "支付金额（法币最小单位）", type: "number",
        value: product?.amount_minor || ""
      },
      {
        name: "coin_amount", label: "到账星币", type: "number",
        value: product?.coin_amount || ""
      },
      {
        name: "bonus_coin", label: "赠送星币", type: "number",
        value: product?.bonus_coin ?? 0
      },
      { name: "sort_order", label: "排序", type: "number", value: product?.sort_order ?? 0 },
      {
        name: "status", label: "状态",
        options: [[1, "启用"], [0, "停用"]], value: product?.status ?? 1
      }
    ], (values) => {
      const name = String(values.name || "").trim();
      const fiatCurrency = String(values.fiat_currency || "").trim().toUpperCase();
      const currencyScale = Number(values.currency_scale);
      const amountMinor = Number(values.amount_minor);
      const coinAmount = Number(values.coin_amount);
      const bonusCoin = Number(values.bonus_coin);
      const sortOrder = Number(values.sort_order);
      const status = Number(values.status);
      const expectedScale = fiatCurrency === "JPY" ? 0 : 2;
      if (!name || [...name].length > 100) {
        throw new Error("商品名称必填且最多 100 个字");
      }
      if (!/^[A-Z][A-Z0-9]{2,7}$/.test(fiatCurrency) ||
          !["CNY", "USD", "EUR", "GBP", "JPY"].includes(fiatCurrency) ||
          !Number.isInteger(currencyScale) || currencyScale !== expectedScale ||
          !Number.isSafeInteger(amountMinor) || amountMinor < 1 ||
          !Number.isSafeInteger(coinAmount) || coinAmount < 1 ||
          !Number.isSafeInteger(bonusCoin) || bonusCoin < 0 ||
          !Number.isSafeInteger(sortOrder) || Math.abs(sortOrder) > 1000000 ||
          ![0, 1].includes(status)) {
        throw new Error("请检查法币、精度（CNY/USD/EUR/GBP 为 2，JPY 为 0）、金额、星币、排序和状态");
      }
      const path = editing ?
        "/admin/api/payments/products/" + encodeURIComponent(product.id) :
        "/admin/api/payments/products";
      return api(path, {
        method: "POST",
        body: {
          name, fiat_currency: fiatCurrency, currency_scale: currencyScale,
          amount_minor: amountMinor, coin_amount: coinAmount, bonus_coin: bonusCoin,
          sort_order: sortOrder, status
        }
      });
    });
  }

  async function handleAction(action, id, target) {
    if (action === "table-view") {
      return openTableRowDetails(target?.dataset.tableId || "", target?.dataset.rowIndex || "");
    }
    if (action === "table-page") {
      const model = state.tables[target?.dataset.tableId || ""];
      const page = Number(target?.dataset.page || 1);
      if (!model || !Number.isInteger(page)) return;
      if (model.searchTimer) {
        window.clearTimeout(model.searchTimer);
        model.searchTimer = null;
      }
      model.page = page;
      state.tablePreferences[model.preferenceKey] = {
        query: model.query,
        page: model.page,
        pageSize: model.pageSize
      };
      if (model.remote) {
        void loadRemoteTable(model, false);
        return;
      }
      refreshTable(model.id, false);
      return;
    }
    if (action === "refresh") return loadRoute();
    if (action === "lottery-tab") {
      state.cache.lotteryTab = id || "games";
      return loadRoute();
    }
    if (action === "user-edit") {
      const row = cached("users", id);
      if (!row) {
        notify("用户数据已刷新，请重试", true);
        return;
      }
      return openForm("编辑用户资料", [
        { name: "username", label: "登录账号（必填）", value: row.username, wide: true },
        { name: "nickname", label: "昵称", value: row.nickname },
        { name: "country_code", label: "国家区号", value: row.country_code || "86" },
        { name: "mobile", label: "手机号", value: row.mobile || "" },
        { name: "email", label: "邮箱", type: "email", value: row.email || "", wide: true }
      ], (values) => {
        const username = String(values.username || "").trim();
        const nickname = String(values.nickname || "").trim();
        const countryCode = String(values.country_code || "").trim();
        const mobile = String(values.mobile || "").trim();
        const email = String(values.email || "").trim();
        const controlCharacters = /[\u0000-\u001f\u007f-\u009f]/;
        if (!username || [...username].length > 120 ||
            /\s/.test(username) || controlCharacters.test(username)) {
          throw new Error("登录账号必填、最多 120 个字，且不能包含空白或控制字符");
        }
        if ([...nickname].length > 100 || controlCharacters.test(nickname)) {
          throw new Error("昵称最多 100 个字，且不能包含控制字符");
        }
        if (!/^\d{1,8}$/.test(countryCode)) {
          throw new Error("国家区号必须是 1 到 8 位数字");
        }
        if (mobile && !/^\d{5,20}$/.test(mobile)) {
          throw new Error("手机号必须是 5 到 20 位数字");
        }
        if (email && (!/^[^\s<>@]+@[^\s<>@]+$/.test(email) ||
            utf8ByteLength(email) > 190)) {
          throw new Error("请输入有效且不超过 190 字节的邮箱");
        }
        return api("/admin/api/users/" + encodeURIComponent(id), {
          method: "POST",
          body: {
            username,
            nickname,
            country_code: countryCode,
            mobile,
            email
          }
        });
      });
    }
    if (action === "user-password") {
      const row = cached("users", id);
      if (!row) {
        notify("用户数据已刷新，请重试", true);
        return;
      }
      return openForm("重置用户密码", [
        {
          name: "password",
          label: "新密码（12–128 个字符）",
          type: "password",
          placeholder: "请输入新密码",
          wide: true
        },
        {
          name: "password_confirm",
          label: "再次输入新密码",
          type: "password",
          placeholder: "请再次输入新密码",
          wide: true
        },
        {
          name: "reason",
          label: "重置原因（必填；保存后该用户现有登录会话会全部注销）",
          type: "textarea",
          placeholder: "请填写管理员重置密码的原因",
          wide: true
        }
      ], (values) => {
        const password = String(values.password || "");
        const confirmation = String(values.password_confirm || "");
        const reason = String(values.reason || "").trim();
        const passwordLength = [...password].length;
        if (passwordLength < 12 || passwordLength > 128 || utf8ByteLength(password) > 512) {
          throw new Error("新密码长度必须为 12 到 128 个字符");
        }
        if (password !== confirmation) {
          throw new Error("两次输入的新密码不一致");
        }
        if (!reason || reason.length > 500) {
          throw new Error("重置原因必填，且不能超过 500 个字");
        }
        return api("/admin/api/users/" + encodeURIComponent(id) + "/password", {
          method: "POST",
          body: { password, reason }
        });
      });
    }
    if (action === "user-status") {
      return openForm("修改用户状态", [
        { name: "status", label: "状态", options: [[1, "正常"], [2, "冻结"], [3, "关闭"]], value: 2 },
        { name: "reason", label: "原因", wide: true }
      ], (values) => api("/admin/api/users/" + id + "/status", {
        method: "POST", body: { status: Number(values.status), reason: values.reason }
      }));
    }
    if (action === "user-team") {
      if (!state.cache.teamOptionsPromise) {
        state.cache.teamOptionsPromise = fetchAllRemoteItems("/admin/api/teams")
          .then((items) => {
            state.cache.teamOptions = items;
            return items;
          })
          .catch((error) => {
            state.cache.teamOptionsPromise = null;
            throw error;
          });
      }
      const allTeams = state.cache.teamOptions ||
        await state.cache.teamOptionsPromise;
      const options = allTeams.filter((team) => Number(team.status) === 1)
        .map((team) => [team.id, team.code + " · " + team.name]);
      if (!options.length) {
        notify("没有可分配的启用团队，请先创建或启用团队", true);
        return;
      }
      return openForm("调整用户团队", [
        { name: "team_id", label: "目标团队", options },
        { name: "reason", label: "调整原因（必填）", type: "textarea", wide: true }
      ], (values) => {
        const reason = String(values.reason || "").trim();
        if (!reason || reason.length > 500) {
          throw new Error("调整原因必填，且不能超过 500 个字");
        }
        return api("/admin/api/users/" + id + "/team", {
          method: "POST",
          body: { team_id: requireDecimalEntityID(values.team_id, "团队编号"), reason }
        });
      });
    }
    if (action === "user-wallet-adjustment") {
      return openUserWalletAdjustment(cached("users", id));
    }
    if (action === "team-create") {
      return openForm("新建团队", [
        { name: "code", label: "三位团队代码", placeholder: "0-9 / a-z" },
        { name: "name", label: "团队名称" }, { name: "owner_user_id", label: "负责人用户 ID", value: "0" }
      ], (values) => {
        const ownerUserID = String(values.owner_user_id || "0").trim();
        if (!/^(0|[1-9]\d*)$/.test(ownerUserID)) {
          throw new Error("请填写有效的负责人用户 ID");
        }
        return api("/admin/api/teams", {
          method: "POST", body: {
            code: values.code, name: values.name, owner_user_id: ownerUserID
          }
        });
      });
    }
    if (action === "team-edit") {
      const row = cached("teams", id);
      if (!row) {
        notify("团队数据已刷新，请重试", true);
        return;
      }
      return openForm("编辑团队 " + row.code, [
        { name: "name", label: "团队名称", value: row.name, wide: true },
        { name: "owner_user_id", label: "负责人用户 ID（0 表示不设置）",
          value: row.owner_user_id || "0", inputmode: "numeric" },
        { name: "status", label: "状态", options: [[1, "启用"], [0, "停用"]], value: row.status }
      ], (values) => {
        const name = String(values.name || "").trim();
        const ownerUserID = String(values.owner_user_id || "0").trim();
        if (!name || name.length > 100 || !/^(0|[1-9]\d*)$/.test(ownerUserID)) {
          throw new Error("请填写有效的团队名称和负责人用户 ID");
        }
        return api("/admin/api/teams/" + encodeURIComponent(id), {
          method: "POST",
          body: {
            name,
            owner_user_id: ownerUserID,
            status: Number(values.status)
          }
        });
      });
    }
    if (action === "payment-channel-edit") {
      const row = cached("paymentChannels", id);
      if (!row) {
        notify("支付通道数据已刷新，请重试", true);
        return;
      }
      return openForm("编辑 BEpusdt 通道", [
        { name: "name", label: "通道名称", value: row.name, wide: true },
        {
          name: "api_base_url", label: "BEpusdt API 根地址",
          value: row.api_base_url || "", placeholder: "http://bepusdt:8080", wide: true
        },
        {
          name: "public_base_url", label: "用户可访问的支付根地址",
          value: row.public_base_url || "", placeholder: "https://pay.example.com", wide: true
        },
        {
          name: "api_token", label: row.token_configured ?
            "API Token（留空保留原值）" : "API Token（首次配置必填）",
          type: "password", placeholder: row.token_configured ? "留空不会覆盖" : "至少 8 个字符",
          wide: true
        },
        {
          name: "trade_type", label: "交易类型",
          value: row.trade_type || "usdt.trc20", placeholder: "usdt.trc20"
        },
        {
          name: "fiat", label: "法币",
          options: [["CNY", "CNY"], ["USD", "USD"], ["EUR", "EUR"], ["GBP", "GBP"], ["JPY", "JPY"]],
          value: row.fiat || "CNY"
        },
        {
          name: "timeout_seconds", label: "订单超时（180–3600 秒）", type: "number",
          value: row.timeout_seconds || 1200
        },
        {
          name: "currency_scale", label: "法币精度", type: "number",
          value: row.currency_scale ?? 2
        },
        {
          name: "min_amount_minor", label: "最小金额（最小单位）", type: "number",
          value: row.min_amount_minor || 1
        },
        {
          name: "max_amount_minor", label: "最大金额（最小单位）", type: "number",
          value: row.max_amount_minor || 1000000
        },
        { name: "sort_order", label: "排序", type: "number", value: row.sort_order ?? 0 }
      ], (values) => {
        const name = String(values.name || "").trim();
        const apiBaseURL = requirePaymentBaseURL(values.api_base_url, "API 根地址");
        const publicBaseURL = requirePaymentBaseURL(values.public_base_url, "支付根地址");
        const apiToken = String(values.api_token || "").trim();
        const tradeType = String(values.trade_type || "").trim().toLowerCase();
        const fiat = String(values.fiat || "").trim().toUpperCase();
        const timeoutSeconds = Number(values.timeout_seconds);
        const currencyScale = Number(values.currency_scale);
        const minAmountMinor = Number(values.min_amount_minor);
        const maxAmountMinor = Number(values.max_amount_minor);
        const sortOrder = Number(values.sort_order);
        const expectedScale = fiat === "JPY" ? 0 : 2;
        if (!name || [...name].length > 100) {
          throw new Error("通道名称必填且最多 100 个字");
        }
        if ((!row.token_configured && apiToken.length < 8) || apiToken.length > 512) {
          throw new Error("首次配置 Token 必须为 8–512 个字符；已配置时可留空保留");
        }
        if (!/^[a-z0-9][a-z0-9._-]{1,39}$/.test(tradeType) ||
            !["CNY", "USD", "EUR", "GBP", "JPY"].includes(fiat) ||
            !Number.isInteger(timeoutSeconds) || timeoutSeconds < 180 || timeoutSeconds > 3600 ||
            !Number.isInteger(currencyScale) || currencyScale !== expectedScale ||
            !Number.isSafeInteger(minAmountMinor) || minAmountMinor < 1 ||
            !Number.isSafeInteger(maxAmountMinor) || maxAmountMinor < minAmountMinor ||
            !Number.isSafeInteger(sortOrder) || Math.abs(sortOrder) > 1000000) {
          throw new Error("请检查交易类型、法币、超时、法币精度（CNY 为 2）、金额范围和排序");
        }
        return api("/admin/api/payments/channels/" + encodeURIComponent(id), {
          method: "POST",
          body: {
            name, api_base_url: apiBaseURL, public_base_url: publicBaseURL,
            api_token: apiToken, trade_type: tradeType, fiat,
            timeout_seconds: timeoutSeconds, currency_scale: currencyScale,
            min_amount_minor: minAmountMinor, max_amount_minor: maxAmountMinor,
            sort_order: sortOrder, status: 0
          }
        });
      });
    }
    if (action === "payment-channel-check") {
      const row = cached("paymentChannels", id);
      if (!row) {
        notify("支付通道数据已刷新，请重试", true);
        return;
      }
      const result = await api(
        "/admin/api/payments/channels/" + encodeURIComponent(id) + "/check",
        { method: "POST", body: {} }
      );
      notify("签名与 Token 检查通过：" + (result.provider_message || "BEpusdt 响应正常"));
      return loadRoute();
    }
    if (action === "payment-channel-status") {
      const row = cached("paymentChannels", id);
      if (!row) {
        notify("支付通道数据已刷新，请重试", true);
        return;
      }
      const status = Number(row.status) === 1 ? 0 : 1;
      if (!window.confirm(status === 1 ?
        "确认启用该支付通道？启用后会立即出现在客户端。" :
        "确认停用该支付通道？停用后将不能创建新订单。")) {
        return;
      }
      return mutateAndRefresh(
        "/admin/api/payments/channels/" + encodeURIComponent(id) + "/status",
        { method: "POST", body: { status } },
        status === 1 ? "支付通道已启用" : "支付通道已停用"
      );
    }
    if (action === "payment-product-create") {
      return paymentProductForm(null);
    }
    if (action === "payment-product-edit") {
      const row = cached("paymentProducts", id);
      if (!row) {
        notify("充值商品数据已刷新，请重试", true);
        return;
      }
      return paymentProductForm(row);
    }
    if (action === "payment-product-status") {
      const row = cached("paymentProducts", id);
      if (!row) {
        notify("充值商品数据已刷新，请重试", true);
        return;
      }
      const status = Number(row.status) === 1 ? 0 : 1;
      if (!window.confirm(status === 1 ?
        "确认启用该充值商品？启用后会立即出现在客户端。" :
        "确认停用该充值商品？已有订单不受影响。")) {
        return;
      }
      return mutateAndRefresh(
        "/admin/api/payments/products/" + encodeURIComponent(id) + "/status",
        { method: "POST", body: { status } },
        status === 1 ? "充值商品已启用" : "充值商品已停用"
      );
    }
    if (action === "payment-recharge-mark-paid") {
      const row = cached("paymentRecharges", id);
      if (!row) {
        notify("充值订单数据已刷新，请重试", true);
        return;
      }
      return openForm("BEpusdt 已支付订单核验补账", [
        {
          name: "order_no", label: "平台订单（不可修改）",
          value: row.order_no, readonly: true, wide: true
        },
        {
          name: "provider_order_no", label: "BEpusdt 交易号（必填）",
          value: row.provider_trade_id || "", wide: true
        },
        {
          name: "reason",
          label: "异常核实说明（必填；服务端核验已支付后才会入账）",
          type: "textarea", wide: true
        }
      ], (values) => {
        const providerOrderNo = String(values.provider_order_no || "").trim();
        const reason = String(values.reason || "").trim();
        if (!providerOrderNo || providerOrderNo.length > 190) {
          throw new Error("BEpusdt 交易号必填，且不能超过 190 个字符");
        }
        if (!reason || reason.length > 500) {
          throw new Error("异常核实说明必填，且不能超过 500 个字");
        }
        const credited = Number(row.coin_amount || 0) + Number(row.bonus_coin || 0);
        if (!window.confirm(
          "二次确认：该操作会立即向用户 ID " + String(row.user_id || "—") +
          " 入账 " + formatNumber(credited) +
          " 星币；服务端将再次核验 BEpusdt 已支付状态。确认继续？"
        )) {
          throw new Error("操作已取消，本次未入账");
        }
        return api(
          "/admin/api/payments/recharges/" + encodeURIComponent(id) + "/mark-paid",
          { method: "POST", body: { provider_order_no: providerOrderNo, reason } }
        );
      });
    }
    if (action === "recharge-mark-paid") {
      const row = cached("recharges", id);
      if (!row) {
        notify("充值订单数据已刷新，请重试", true);
        return;
      }
      return openForm("异常订单人工入账", [
        {
          name: "order_no",
          label: "平台订单（不可修改）",
          value: row.order_no,
          readonly: true,
          wide: true
        },
        { name: "provider_order_no", label: "渠道交易号（必填）", value: row.provider_trade_id || "", wide: true },
        {
          name: "reason",
          label: "异常核实说明（必填；立即入账并写入支付审计）",
          type: "textarea",
          wide: true
        }
      ], (values) => {
        const providerOrderNo = String(values.provider_order_no || "").trim();
        const reason = String(values.reason || "").trim();
        if (!providerOrderNo || providerOrderNo.length > 190) {
          throw new Error("渠道交易号必填，且不能超过 190 个字符");
        }
        if (!reason || reason.length > 500) {
          throw new Error("异常核实说明必填，且不能超过 500 个字");
        }
        const credited = Number(row.coin_amount || 0) + Number(row.bonus_coin || 0);
        if (!window.confirm(
          "二次确认：该操作会立即向用户 ID " + String(row.user_id || "—") +
          " 入账 " + formatNumber(credited) + " 星币，且写入资金审计。确认继续？"
        )) {
          throw new Error("操作已取消，本次未入账");
        }
        return api("/admin/api/wallet/recharges/" + encodeURIComponent(id) + "/mark-paid", {
          method: "POST",
          body: { provider_order_no: providerOrderNo, reason }
        });
      });
    }
    if (action === "adjustment-create") {
      return openForm("发起调账", [
        { name: "user_id", label: "用户 ID", inputmode: "numeric" },
        { name: "amount", label: "星币变动（可为负）", type: "number" },
        { name: "reason", label: "调账原因", type: "textarea", wide: true }
      ], (values) => {
        const userID = String(values.user_id || "").trim();
        if (!/^[1-9]\d*$/.test(userID)) {
          throw new Error("请填写有效的用户 ID");
        }
        return api("/admin/api/wallet/adjustments", {
          method: "POST", body: {
            user_id: userID, amount: Number(values.amount),
            reason: values.reason, evidence_asset_id: 0
          }
        });
      });
    }
    if (action === "adjustment-approve") {
      return mutateAndRefresh(
        "/admin/api/wallet/adjustments/" + encodeURIComponent(id) + "/approve",
        { method: "POST", body: {} },
        "调账已入账"
      ).catch((error) => notify(error.message, true));
    }
    if (action === "adjustment-reject") {
      return openForm("驳回调账", [{ name: "reason", label: "驳回原因", type: "textarea", wide: true }],
        (values) => api("/admin/api/wallet/adjustments/" + id + "/reject", {
          method: "POST", body: { reason: values.reason }
        }));
    }
    if (action === "withdraw-review") {
      return openForm("提现审核", [
        { name: "action", label: "动作", options: [["approve", "通过"], ["paying", "进入打款"], ["paid", "确认到账"], ["reject", "驳回"]] },
        { name: "provider_order_no", label: "渠道订单号" },
        { name: "reason", label: "原因", type: "textarea", wide: true }
      ], (values) => api("/admin/api/wallet/withdrawals/" + id + "/review", {
        method: "POST", body: values
      }));
    }
    if (action === "game-edit") {
      const row = cached("games", id);
      return openForm("编辑游戏", [
        { name: "name", label: "名称", value: row.name },
        { name: "status", label: "状态", options: [[1, "上架"], [0, "下架"]], value: row.status },
        { name: "sort_order", label: "排序", type: "number", value: row.sort_order }
      ], (values) => api("/admin/api/games/" + id, {
        method: "POST", body: { name: values.name, status: Number(values.status), sort_order: Number(values.sort_order) }
      }));
    }
    if (action === "venue-edit") {
      const row = cached("venues", id);
      return openForm("配置捕鱼场", [
        { name: "name", label: "名称", value: row.name },
        { name: "min_balance", label: "最低余额", type: "number", value: row.min_balance },
        { name: "escrow_amount", label: "入场冻结", type: "number", value: row.escrow_amount },
        { name: "bet_levels", label: "下注档位（逗号分隔）", value: (row.bet_levels || []).join(",") },
        { name: "target_rtp_ppm", label: "目标 RTP（百万分比）", type: "number", value: row.target_rtp_ppm },
        { name: "status", label: "状态", options: [[1, "启用"], [0, "停用"]], value: row.status },
        { name: "sort_order", label: "排序", type: "number", value: row.sort_order }
      ], (values) => api("/admin/api/games/venues/" + id, {
        method: "POST", body: {
          name: values.name, min_balance: Number(values.min_balance),
          escrow_amount: Number(values.escrow_amount),
          bet_levels: values.bet_levels.split(",").map(Number).filter((value) => value > 0),
          target_rtp_ppm: Number(values.target_rtp_ppm), status: Number(values.status),
          sort_order: Number(values.sort_order)
        }
      }));
    }
    if (action === "live-create") {
      return openLiveRoomForm(null);
    }
    if (action === "live-edit") {
      const row = cached("live", id);
      return openLiveRoomForm(row);
    }
    if (action === "lottery-category") {
      return openForm("新增彩票分类", [
        { name: "category_key", label: "英文标识" }, { name: "name", label: "名称" },
        { name: "sort_order", label: "排序", type: "number", value: "0" }
      ], (values) => api("/admin/api/lottery/categories", {
        method: "POST", body: {
          category_key: values.category_key, name: values.name, status: 1, sort_order: Number(values.sort_order)
        }
      }));
    }
    if (action === "lottery-category-edit") {
      const row = cached("lotteryCategories", id);
      return openForm("编辑彩票分类", [
        { name: "category_key", label: "英文标识（不可修改）", value: row.category_key },
        { name: "name", label: "名称", value: row.name },
        { name: "sort_order", label: "排序", type: "number", value: row.sort_order }
      ], (values) => api("/admin/api/lottery/categories/" + id, {
        method: "POST", body: {
          name: values.name, sort_order: Number(values.sort_order)
        }
      }));
    }
    if (action === "lottery-game") {
      const options = (state.cache.lottery.categories || []).map((item) => [item.id, item.name]);
      return openForm("新增彩票游戏", [
        { name: "category_id", label: "分类", options }, { name: "game_code", label: "英文标识" },
        { name: "name", label: "名称" }, { name: "issue_interval_seconds", label: "每期秒数", type: "number", value: "300" },
        { name: "sale_close_seconds", label: "提前封盘秒数", type: "number", value: "10" },
        { name: "min_bet", label: "最低投注", type: "number", value: "1" },
        { name: "max_bet", label: "最高投注", type: "number", value: "1000000" }
      ], (values) => api("/admin/api/lottery/games", {
        method: "POST", body: {
          category_id: requireDecimalEntityID(values.category_id, "彩票分类编号"),
          game_code: values.game_code, name: values.name,
          issue_interval_seconds: Number(values.issue_interval_seconds),
          sale_close_seconds: Number(values.sale_close_seconds), min_bet: Number(values.min_bet),
          max_bet: Number(values.max_bet), status: 1, sort_order: 0, config: {}
        }
      }));
    }
    if (action === "lottery-game-edit") {
      const row = cached("lotteryGames", id);
      const options = (state.cache.lottery.categories || []).map((item) => [item.id, item.name]);
      return openForm("编辑彩票游戏", [
        { name: "category_id", label: "分类", options, value: row.category_id },
        { name: "game_code", label: "英文标识", value: row.game_code },
        { name: "name", label: "名称", value: row.name },
        { name: "issue_interval_seconds", label: "每期秒数", type: "number", value: row.issue_interval_seconds },
        { name: "sale_close_seconds", label: "提前封盘秒数", type: "number", value: row.sale_close_seconds },
        { name: "min_bet", label: "最低投注", type: "number", value: row.min_bet },
        { name: "max_bet", label: "最高投注", type: "number", value: row.max_bet },
        { name: "sort_order", label: "排序", type: "number", value: row.sort_order }
      ], (values) => api("/admin/api/lottery/games/" + id, {
        method: "POST", body: {
          category_id: requireDecimalEntityID(values.category_id, "彩票分类编号"),
          game_code: values.game_code,
          name: values.name, issue_interval_seconds: Number(values.issue_interval_seconds),
          sale_close_seconds: Number(values.sale_close_seconds),
          min_bet: Number(values.min_bet), max_bet: Number(values.max_bet),
          sort_order: Number(values.sort_order)
        }
      }));
    }
    if (action === "lottery-game-status") {
      const row = cached("lotteryGames", id);
      if (!row) {
        notify("彩票数据已刷新，请重试", true);
        return;
      }
      const status = Number(row.status) === 1 ? 0 : 1;
      return mutateAndRefresh("/admin/api/lottery/games/" + encodeURIComponent(id) + "/status", {
        method: "POST",
        body: { status }
      }, status === 1 ? "彩票已恢复" : "彩票已停用");
    }
    if (action === "lottery-config") {
      const row = cached("lotteryGames", id);
      const plays = (state.cache.lotteryPlays || []).filter((play) =>
        String(play.game_id) === String(id));
      const playTable = table([
        { label: "玩法", render: (play) => "<strong>" + esc(play.name) +
          "</strong><br><small>" + esc(play.play_code) + "</small>" },
        { label: "结算规则", key: "settlement_rule" },
        { label: "选项", render: (play) => formatNumber(play.option_count) + " 个" },
        { label: "操作", render: (play) => has("lottery.write") ?
          '<div class="row-actions">' + button("选项", "lottery-play-options", play.id) +
          button("编辑", "lottery-play-edit", play.id, "layui-btn-normal") + "</div>" :
          button("查看选项", "lottery-play-options", play.id) }
      ], plays);
      const toolbar = has("lottery.write") ?
        '<div class="modal-toolbar"><button class="layui-btn" data-action="lottery-play-add" data-id="' +
        esc(id) + '">新增玩法</button></div>' : "";
      layer.open({
        type: 1, title: esc(row.name) + " · 玩法配置", area: ["860px", "620px"],
        content: '<div class="modal-content"><div class="lottery-config-summary">' +
          "<strong>" + esc(row.name) + "</strong><span>" + esc(row.game_code) +
          " · " + esc(row.category_name) + " · " + esc(row.issue_interval_seconds) +
          " 秒/期</span></div>" + toolbar + playTable + "</div>"
      });
      return;
    }
    if (action === "lottery-issue") {
      const options = (state.cache.lottery.games || [])
        .filter((item) => Number(item.status) === 1)
        .map((item) => [item.id, item.name]);
      if (!options.length) {
        notify("没有可创建期号的启用彩票，请先恢复彩票", true);
        return;
      }
      const now = Math.floor(Date.now() / 1000);
      return openForm("新建彩票期号", [
        { name: "game_id", label: "彩票游戏", options },
        { name: "issue_no", label: "期号" },
        { name: "sale_open_at", label: "开售 Unix 秒", type: "number", value: now },
        { name: "sale_close_at", label: "封盘 Unix 秒", type: "number", value: now + 300 },
        { name: "draw_at", label: "开奖 Unix 秒", type: "number", value: now + 310 }
      ], (values) => api("/admin/api/lottery/issues", {
        method: "POST", body: {
          game_id: requireDecimalEntityID(values.game_id, "彩票游戏编号"),
          issue_no: values.issue_no,
          sale_open_at: Number(values.sale_open_at), sale_close_at: Number(values.sale_close_at),
          draw_at: Number(values.draw_at)
        }
      }));
    }
    if (action === "lottery-play-add") {
      const game = cached("lotteryGames", id);
      return openForm("新增彩票玩法", [
        { name: "game_name", label: "彩票游戏", value: game.name },
        { name: "play_code", label: "玩法标识" },
        { name: "name", label: "玩法名称" },
        { name: "settlement_rule", label: "结算规则", value: "winner_option_ids" },
        { name: "sort_order", label: "排序", type: "number", value: "0" }
      ], (values) => api("/admin/api/lottery/plays", {
        method: "POST", body: {
          game_id: requireDecimalEntityID(id, "彩票游戏编号"),
          play_code: values.play_code,
          name: values.name, settlement_rule: values.settlement_rule,
          status: 1, sort_order: Number(values.sort_order), config: {}
        }
      }));
    }
    if (action === "lottery-play-edit") {
      const row = cached("lotteryPlays", id);
      return openForm("编辑彩票玩法", [
        { name: "play_code", label: "玩法标识（不可修改）", value: row.play_code },
        { name: "name", label: "玩法名称", value: row.name },
        { name: "settlement_rule", label: "结算规则", value: row.settlement_rule },
        { name: "sort_order", label: "排序", type: "number", value: row.sort_order }
      ], (values) => api("/admin/api/lottery/plays/" + id, {
        method: "POST", body: {
          name: values.name, settlement_rule: values.settlement_rule,
          sort_order: Number(values.sort_order)
        }
      }));
    }
    if (action === "lottery-play-options") {
      const play = cached("lotteryPlays", id);
      const options = (state.cache.lotteryOptions || []).filter((item) =>
        String(item.play_id) === String(id));
      const optionTable = table([
        { label: "选项", render: (item) => "<strong>" + esc(item.name) +
          "</strong><br><small>" + esc(item.option_code) + "</small>" },
        { label: "赔率", render: (item) => (Number(item.odds_scaled) / 1000000).toFixed(3) },
        { label: "排序", key: "sort_order" },
        { label: "操作", render: (item) => has("lottery.write") ?
          button("编辑", "lottery-option-edit", item.id) : "—" }
      ], options);
      const toolbar = has("lottery.write") ?
        '<div class="modal-toolbar"><button class="layui-btn" data-action="lottery-option-add" data-id="' +
        esc(id) + '">新增选项</button></div>' : "";
      layer.open({
        type: 1, title: esc(play.game_name) + " · " + esc(play.name),
        area: ["760px", "580px"],
        content: '<div class="modal-content">' + toolbar + optionTable + "</div>"
      });
      return;
    }
    if (action === "lottery-option-add") {
      const play = cached("lotteryPlays", id);
      return openForm("新增彩票投注选项", [
        { name: "play_name", label: "彩票玩法", value: play.game_name + " · " + play.name },
        { name: "option_code", label: "选项标识" },
        { name: "name", label: "选项名称" },
        { name: "odds", label: "十进制赔率", type: "number", value: "2.0" },
        { name: "sort_order", label: "排序", type: "number", value: "0" }
      ], (values) => api("/admin/api/lottery/options", {
        method: "POST", body: {
          play_id: requireDecimalEntityID(id, "彩票玩法编号"),
          option_code: values.option_code,
          name: values.name, odds_scaled: Math.round(Number(values.odds) * 1000000),
          status: 1, sort_order: Number(values.sort_order), config: {}
        }
      }));
    }
    if (action === "lottery-option-edit") {
      const row = cached("lotteryOptions", id);
      return openForm("编辑彩票投注选项", [
        { name: "option_code", label: "选项标识（不可修改）", value: row.option_code },
        { name: "name", label: "选项名称", value: row.name },
        { name: "odds", label: "十进制赔率", type: "number", value: Number(row.odds_scaled) / 1000000 },
        { name: "sort_order", label: "排序", type: "number", value: row.sort_order }
      ], (values) => api("/admin/api/lottery/options/" + id, {
        method: "POST", body: {
          name: values.name, odds_scaled: Math.round(Number(values.odds) * 1000000),
          sort_order: Number(values.sort_order)
        }
      }));
    }
    if (action === "lottery-close") {
      return mutateAndRefresh(
        "/admin/api/lottery/issues/" + encodeURIComponent(id) + "/close",
        { method: "POST", body: {} },
        "期号已封盘"
      ).catch((error) => notify(error.message, true));
    }
    if (action === "lottery-draw") {
      return openForm("录入开奖结果", [
        { name: "winner_option_ids", label: "中奖选项 ID（逗号分隔）" },
        { name: "source", label: "结果来源", value: "manual_reviewed" }
      ], (values) => api("/admin/api/lottery/issues/" + id + "/draw", {
        method: "POST", body: {
          result: { winner_option_ids: decimalFormIDs(values.winner_option_ids) },
          source: values.source
        }
      }));
    }
    if (action === "sports-create") {
      const now = Math.floor(Date.now() / 1000);
      return openForm("新增体育赛事", [
        { name: "competition", label: "联赛" },
        { name: "competition_type", label: "项目", value: "football" },
        { name: "home_name", label: "主队" }, { name: "away_name", label: "客队" },
        { name: "kickoff_at", label: "开赛 Unix 秒", type: "number", value: now + 3600 },
        { name: "bet_close_at", label: "封盘 Unix 秒", type: "number", value: now + 3300 },
        { name: "min_bet", label: "最低投注", type: "number", value: "1" },
        { name: "max_bet", label: "最高投注", type: "number", value: "1000000" }
      ], (values) => api("/admin/api/sports/matches", {
        method: "POST", body: {
          competition: values.competition, competition_type: values.competition_type,
          home_name: values.home_name, away_name: values.away_name,
          kickoff_at: Number(values.kickoff_at), bet_close_at: Number(values.bet_close_at),
          home_score: 0, away_score: 0, match_status: "NS", bet_status: 1,
          min_bet: Number(values.min_bet), max_bet: Number(values.max_bet)
        }
      }));
    }
    if (action === "sports-edit") {
      const row = cached("sports", id);
      return openForm("编辑体育赛事", [
        { name: "competition", label: "联赛", value: row.competition },
        { name: "competition_type", label: "项目", value: row.competition_type },
        { name: "home_name", label: "主队", value: row.home_name },
        { name: "away_name", label: "客队", value: row.away_name },
        { name: "kickoff_at", label: "开赛 Unix 秒", type: "number", value: row.kickoff_at },
        { name: "bet_close_at", label: "封盘 Unix 秒", type: "number", value: row.bet_close_at },
        { name: "home_score", label: "主队比分", type: "number", value: row.home_score },
        { name: "away_score", label: "客队比分", type: "number", value: row.away_score },
        { name: "match_status", label: "赛事状态", options: [
          ["NS", "未开始"], ["LIVE", "进行中"], ["HT", "中场"],
          ["FT", "已完赛"], ["CANCELLED", "已取消"]
        ], value: row.match_status },
        { name: "bet_status", label: "投注状态", options: [[1, "开放"], [0, "停盘"]], value: row.bet_status },
        { name: "min_bet", label: "最低投注", type: "number", value: row.min_bet },
        { name: "max_bet", label: "最高投注", type: "number", value: row.max_bet }
      ], (values) => api("/admin/api/sports/matches/" + id, {
        method: "POST", body: {
          competition: values.competition, competition_type: values.competition_type,
          home_name: values.home_name, away_name: values.away_name,
          kickoff_at: Number(values.kickoff_at), bet_close_at: Number(values.bet_close_at),
          home_score: Number(values.home_score), away_score: Number(values.away_score),
          match_status: values.match_status, bet_status: Number(values.bet_status),
          min_bet: Number(values.min_bet), max_bet: Number(values.max_bet)
        }
      }));
    }
    if (action === "sports-markets") {
      const data = await api("/admin/api/sports/matches/" + id + "/markets");
      const markets = data.items || [];
      const options = [];
      markets.forEach((market) => {
        (market.options || []).forEach((marketOption) => options.push({
          ...marketOption,
          market_id: market.id,
          market_name: market.name,
          market_code: market.market_code
        }));
      });
      state.cache.sportsMarkets = markets;
      state.cache.sportsOptions = options;
      const marketTable = table([
        { label: "盘口", render: (row) => "<strong>" + esc(row.name) +
          "</strong><br><small>" + esc(row.market_code) + "</small>" },
        { label: "结算规则", render: (row) => esc(row.settlement_rule) },
        { label: "状态", render: (row) => statusTag(row.status, {
          1: ["启用", "ok"], 0: ["停用", "bad"]
        }) },
        { label: "排序", key: "sort_order" },
        { label: "选项数", render: (row) => formatNumber((row.options || []).length) },
        { label: "操作", render: (row) => has("sports.write") ?
          button("编辑盘口", "sports-market-edit", row.id, "layui-btn-normal") : "—" }
      ], markets);
      const optionTable = table([
        { label: "盘口", render: (row) => "<strong>" + esc(row.market_name) +
          "</strong><br><small>" + esc(row.market_code) + "</small>" },
        { label: "选项", render: (row) => esc(row.name) + "<br><small>" + esc(row.option_code) + "</small>" },
        { label: "赔率", render: (row) => (Number(row.odds_scaled) / 1000000).toFixed(3) },
        { label: "赛果", render: (row) => statusTag(row.result, {
          0: ["未录入", "warn"], 1: ["赢", "ok"], 2: ["输", "bad"]
        }) },
        { label: "状态", render: (row) => Number(row.status) === 1 ? "启用" : "停用" },
        { label: "操作", render: (row) => has("sports.write") ?
          button("赔率/赛果", "sports-option-edit", row.id) : "—" }
      ], options);
      const addButton = has("sports.write") ?
        '<div class="modal-toolbar"><button class="layui-btn" data-action="sports-market-create" data-id="' +
        esc(id) + '">新增盘口</button></div>' : "";
      layer.open({
        type: 1, title: "赛事盘口", area: ["900px", "620px"],
        content: '<div class="modal-content">' + addButton +
          panel("盘口列表", "可维护盘口标识、结算规则、状态与排序", marketTable) +
          panel("盘口选项", "赔率、赛果与选项启停保持独立维护", optionTable) + "</div>"
      });
      return;
    }
    if (action === "sports-market-create") {
      return openForm("新增体育盘口", [
        { name: "market_code", label: "盘口标识", value: "1x2" },
        { name: "name", label: "盘口名称", value: "胜平负" },
        { name: "settlement_rule", label: "结算规则", value: "result_option" },
        {
          name: "options", label: "选项 JSON", type: "textarea", wide: true,
          value: JSON.stringify([
            { option_code: "home", name: "主胜", odds_scaled: 1800000 },
            { option_code: "draw", name: "平局", odds_scaled: 3200000 },
            { option_code: "away", name: "客胜", odds_scaled: 4100000 }
          ], null, 2)
        }
      ], (values) => api("/admin/api/sports/markets", {
        method: "POST", body: {
          match_id: requireDecimalEntityID(id, "体育赛事编号"),
          market_code: values.market_code, name: values.name,
          settlement_rule: values.settlement_rule, status: 1, sort_order: 0,
          options: JSON.parse(values.options)
        }
      }));
    }
    if (action === "sports-market-edit") {
      const row = cached("sportsMarkets", id);
      if (!row) {
        notify("盘口数据已刷新，请重试", true);
        return;
      }
      return openForm("编辑体育盘口", [
        { name: "market_code", label: "盘口标识", value: row.market_code },
        { name: "name", label: "盘口名称", value: row.name },
        { name: "settlement_rule", label: "结算规则", value: row.settlement_rule },
        { name: "status", label: "状态", options: [[1, "启用"], [0, "停用"]], value: row.status },
        { name: "sort_order", label: "排序", type: "number", value: row.sort_order }
      ], (values) => api("/admin/api/sports/markets/" + encodeURIComponent(id), {
        method: "POST",
        body: {
          market_code: String(values.market_code || "").trim(),
          name: String(values.name || "").trim(),
          settlement_rule: String(values.settlement_rule || "").trim(),
          status: Number(values.status),
          sort_order: Number(values.sort_order || 0)
        }
      }));
    }
    if (action === "sports-option-edit") {
      const row = cached("sportsOptions", id);
      return openForm("编辑赔率与赛果", [
        { name: "odds", label: "十进制赔率", type: "number", value: Number(row.odds_scaled) / 1000000 },
        { name: "result", label: "赛果", options: [[0, "未录入"], [1, "赢"], [2, "输"]], value: row.result },
        { name: "status", label: "状态", options: [[1, "启用"], [0, "停用"]], value: row.status }
      ], (values) => api("/admin/api/sports/options/" + id, {
        method: "POST", body: {
          odds_scaled: Math.round(Number(values.odds) * 1000000),
          result: Number(values.result), status: Number(values.status)
        }
      }));
    }
    if (action === "sports-settle") {
      return mutateAndRefresh(
        "/admin/api/sports/matches/" + encodeURIComponent(id) + "/settle",
        { method: "POST", body: {} },
        "赛事已提交结算队列"
      ).catch((error) => notify(error.message, true));
    }
    if (action === "im-members") {
      const messageKey = "im-messages-" + id;
      const messagePath = "/admin/api/im/conversations/" + encodeURIComponent(id) + "/messages";
      const result = await Promise.all([
        api("/admin/api/im/conversations/" + encodeURIComponent(id) + "/members"),
        remoteTableData(messageKey, messagePath)
      ]);
      const data = result[0];
      const messages = result[1];
      state.cache.imMembers = data.items || [];
      state.cache.imMessages = messages.items || [];
      state.cache.imModalConversation = id;
      const memberTable = table([
        { label: "用户", render: (row) => esc(row.nickname + " (" + row.user_id + ")") },
        { label: "角色", render: (row) => ({ 100: "群主", 50: "管理员", 1: "成员" }[row.role] || row.role) },
        { label: "状态", render: (row) => statusTag(row.member_status, {
          1: ["有效", "ok"], 2: ["退出", "warn"], 3: ["已移出", "bad"]
        }) },
        { label: "禁言至", render: (row) => formatTime(row.mute_until) },
        { label: "加入时间", render: (row) => formatTime(row.joined_at) },
        { label: "操作", render: (row) => {
          if (!has("im.moderate") || Number(row.member_status) !== 1) return "—";
          const payload = encodeURIComponent(id) + "~" + row.user_id;
          const muted = Number(row.mute_until || 0) > Math.floor(Date.now() / 1000);
          return '<div class="row-actions">' +
            button(muted ? "解除禁言" : "禁言", muted ? "im-member-unmute" : "im-member-mute", payload) +
            (Number(row.role) < 100 ?
              button("移出", "im-member-remove", payload, "layui-btn-danger") : "") + "</div>";
        } }
      ], data.items);
      const messageTable = table([
        { label: "序号", key: "sequence" },
        { label: "发送人", render: (row) => esc(row.sender_name) +
          "<br><small>ID " + esc(row.sender_user_id) + "</small>", className: "wrap" },
        { label: "类型", render: (row) => ({
          1: "文字", 2: "图片", 3: "语音", 4: "视频", 5: "文件", 100: "系统"
        }[row.message_type] || ("类型 " + row.message_type)) },
        { label: "内容", render: (row) => esc(row.text_content || "资源 ID " + row.asset_id), className: "wrap" },
        { label: "状态", render: (row) => statusTag(row.status, {
          1: ["正常", "ok"], 2: ["撤回", "warn"], 3: ["已删除", "bad"]
        }) },
        { label: "时间", render: (row) => formatTime(row.created_at) },
        { label: "操作", render: (row) => has("im.moderate") && Number(row.status) !== 3 ?
          button("删除消息", "im-message-delete", row.id, "layui-btn-danger") : "—" }
      ], messages.items, {
        key: messageKey,
        page: messages.page,
        pageSize: messages.page_size,
        total: messages.total,
        hasMore: messages.has_more,
        remote: { path: messagePath, cacheName: "imMessages" }
      });
      layer.open({
        type: 1,
        title: "会话成员与消息",
        skin: "im-management-layer",
        area: ["min(1120px, calc(100vw - 36px))", "min(820px, calc(100vh - 48px))"],
        content: '<div class="modal-content">' +
          panel("成员列表", "可对有效成员禁言、解除禁言或移出群组", memberTable) +
          panel("消息列表", "管理员删除消息必须填写处置原因", messageTable) + "</div>"
      });
      return;
    }
    if (["im-member-mute", "im-member-unmute", "im-member-remove"].includes(action)) {
      const separator = id.lastIndexOf("~");
      const conversationID = decodeURIComponent(id.slice(0, separator));
      const userID = id.slice(separator + 1);
      if (separator < 1 || !conversationID || !userID) {
        notify("群成员数据无效，请刷新后重试", true);
        return;
      }
      const moderationAction = action === "im-member-mute" ? "mute" :
        action === "im-member-unmute" ? "unmute" : "remove";
      const fields = moderationAction === "mute" ? [
        { name: "duration_minutes", label: "禁言时长（分钟）", type: "number", value: "60" },
        { name: "reason", label: "管理原因（必填）", type: "textarea", wide: true }
      ] : [
        { name: "reason", label: moderationAction === "remove" ? "移出原因（必填）" : "解除原因（必填）",
          type: "textarea", wide: true }
      ];
      return openForm({
        mute: "禁言群成员", unmute: "解除成员禁言", remove: "移出群成员"
      }[moderationAction], fields, async (values) => {
        const minutes = moderationAction === "mute" ? Number(values.duration_minutes) : 0;
        if (moderationAction === "mute" &&
            (!Number.isInteger(minutes) || minutes < 1 || minutes > 525600)) {
          throw new Error("禁言时长必须是 1 到 525600 分钟");
        }
        const reason = String(values.reason || "").trim();
        if (!reason || reason.length > 500) {
          throw new Error("管理原因必填，且不能超过 500 个字");
        }
        await api("/admin/api/im/conversations/" + encodeURIComponent(conversationID) +
          "/members/" + encodeURIComponent(userID), {
          method: "POST",
          body: {
            action: moderationAction,
            duration_seconds: minutes * 60,
            reason
          }
        });
        layer.closeAll("page");
      });
    }
    if (action === "im-message-delete") {
      return openForm("删除 IM 消息", [
        { name: "reason", label: "删除原因（必填）", type: "textarea", wide: true }
      ], async (values) => {
        const reason = String(values.reason || "").trim();
        if (!reason || reason.length > 500) {
          throw new Error("删除原因必填，且不能超过 500 个字");
        }
        await api("/admin/api/im/messages/" + encodeURIComponent(id) + "/delete", {
          method: "POST",
          body: { reason }
        });
        layer.closeAll("page");
      });
    }
    if (action === "im-all-mute") {
      const row = cached("im", id);
      return mutateAndRefresh(
        "/admin/api/im/conversations/" + encodeURIComponent(id),
        {
          method: "POST",
          body: { action: "all_mute", value: !row.all_muted, reason: "后台管理操作" }
        },
        "群组状态已更新"
      ).catch((error) => notify(error.message, true));
    }
    if (action === "app-create") {
      return openForm("上传 App 新版本", [
        {
          name: "package_file", label: "更新包（WGT / APK / IPA）",
          type: "file", accept: ".wgt,.apk,.ipa", wide: true
        },
        {
          name: "platform", label: "发布平台",
          options: [["app", "Android + iOS"], ["android", "Android"], ["ios", "iOS"]],
          value: "app"
        },
        {
          name: "release_type", label: "更新类型",
          options: [["wgt", "WGT 热更新"], ["native", "原生整包更新"]], value: "wgt"
        },
        { name: "version_name", label: "版本名称", placeholder: "例如 8.1.1" },
        { name: "version_code", label: "版本号", type: "number", placeholder: "必须递增" },
        {
          name: "min_native_code", label: "最低原生壳版本",
          type: "number", value: "0"
        },
        {
          name: "force_update", label: "是否强制",
          options: [[0, "可选更新"], [1, "强制更新"]], value: "0"
        },
        {
          name: "silent_update", label: "WGT 安装方式",
          options: [[1, "后台无感更新"], [0, "弹窗询问"]], value: "1"
        },
        {
          name: "rollout_percent", label: "发布比例（%）",
          type: "number", value: "100"
        },
        {
          name: "publish_now", label: "上传后",
          options: [[1, "立即发布"], [0, "保存为草稿"]], value: "1"
        },
        {
          name: "release_notes", label: "更新说明",
          type: "textarea", placeholder: "本次修复和功能说明", wide: true
        }
      ], async (values) => {
        const file = values.package_file;
        if (!(file instanceof File) || !file.name || file.size < 1) {
          throw new Error("请选择更新包");
        }
        const releaseType = String(values.release_type || "");
        const platform = String(values.platform || "");
        const extension = file.name.toLowerCase().slice(file.name.lastIndexOf("."));
        if (releaseType === "wgt" && extension !== ".wgt") {
          throw new Error("WGT 热更新只能上传 .wgt 文件");
        }
        if (releaseType === "native" && platform === "android" && extension !== ".apk") {
          throw new Error("Android 整包更新需要可直接安装的 APK");
        }
        if (releaseType === "native" && platform === "ios" && extension !== ".ipa") {
          throw new Error("iOS 整包更新需要 IPA");
        }
        if (releaseType === "native" && platform === "app") {
          throw new Error("原生整包更新必须明确选择 Android 或 iOS");
        }
        const versionCode = Number(values.version_code || 0);
        const versionName = String(values.version_name || "").trim();
        const rollout = Number(values.rollout_percent || 0);
        if (!versionName) {
          throw new Error("请填写版本名称");
        }
        if (!Number.isInteger(versionCode) || versionCode < 1) {
          throw new Error("版本号必须是大于 0 的整数");
        }
        if (releaseType === "wgt" && file.name !== versionName + "_" + versionCode + ".wgt") {
          throw new Error("WGT 文件名必须为 " + versionName + "_" + versionCode + ".wgt");
        }
        if (!Number.isInteger(rollout) || rollout < 1 || rollout > 100) {
          throw new Error("发布比例必须是 1 到 100");
        }

        const loadingIndex = layer.load(2, { shade: [0.18, "#fff"] });
        try {
          const contentType = file.type || "application/octet-stream";
          const prepared = await api("/admin/api/app/uploads/prepare", {
            method: "POST",
            body: { file_name: file.name, content_type: contentType, size: file.size }
          });
          await uploadToSignedURL(prepared.upload_url, file, prepared.headers || {});
          const finalized = await api("/admin/api/app/uploads/finalize", {
            method: "POST",
            body: { object_key: prepared.object_key, sha256: "", size: file.size }
          });
          const release = await api("/admin/api/app/releases", {
            method: "POST",
            body: {
              platform,
              release_type: releaseType,
              version_name: versionName,
              version_code: versionCode,
              min_native_code: Number(values.min_native_code || 0),
              force_update: String(values.force_update) === "1",
              silent_update: releaseType === "wgt" && String(values.silent_update) === "1",
              rollout_percent: rollout,
              asset_id: requireDecimalEntityID(finalized.asset_id, "安装包素材编号"),
              release_notes: String(values.release_notes || "").trim()
            }
          });
          if (String(values.publish_now) === "1") {
            await api("/admin/api/app/releases/" + release.release_id + "/publish", {
              method: "POST", body: {}
            });
          }
        } finally {
          layer.close(loadingIndex);
        }
      });
    }
    if (action === "app-edit") {
      const row = cached("appReleases", id);
      if (!row) {
        notify("版本数据已刷新，请重试", true);
        return;
      }
      return openForm("编辑草稿版本 · " + row.version_name, [
        { name: "version_name", label: "版本名称", value: row.version_name },
        { name: "version_code", label: "版本号", type: "number", value: row.version_code },
        {
          name: "min_native_code",
          label: "最低原生版本",
          type: "number",
          value: row.min_native_code
        },
        {
          name: "force_update",
          label: "更新方式",
          options: [[0, "用户可选"], [1, "强制更新"]],
          value: row.force_update ? 1 : 0
        },
        {
          name: "silent_update",
          label: "安装提示",
          options: [[0, "显示提示"], [1, "静默更新"]],
          value: row.silent_update ? 1 : 0
        },
        {
          name: "rollout_percent",
          label: "灰度比例（0–100）",
          type: "number",
          value: row.rollout_percent
        },
        {
          name: "release_notes",
          label: "更新说明",
          type: "textarea",
          value: row.release_notes || "",
          wide: true
        }
      ], (values) => {
        const versionName = String(values.version_name || "").trim();
        const versionCode = Number(values.version_code);
        const minNativeCode = Number(values.min_native_code);
        const rolloutPercent = Number(values.rollout_percent);
        const releaseNotes = String(values.release_notes || "").trim();
        if (!versionName || utf8ByteLength(versionName) > 40) {
          throw new Error("版本名称必填，且不能超过 40 字节");
        }
        if (!Number.isInteger(versionCode) || versionCode < 1) {
          throw new Error("版本号必须是大于 0 的整数");
        }
        if (!Number.isInteger(minNativeCode) || minNativeCode < 0) {
          throw new Error("最低原生版本必须是大于或等于 0 的整数");
        }
        if (!Number.isInteger(rolloutPercent) || rolloutPercent < 0 || rolloutPercent > 100) {
          throw new Error("灰度比例必须是 0 到 100 的整数");
        }
        if (utf8ByteLength(releaseNotes) > 2000) {
          throw new Error("更新说明不能超过 2000 字节");
        }
        return api("/admin/api/app/releases/" + encodeURIComponent(id), {
          method: "PATCH",
          body: {
            version_name: versionName,
            version_code: versionCode,
            min_native_code: minNativeCode,
            force_update: String(values.force_update) === "1",
            silent_update: row.release_type === "wgt" &&
              String(values.silent_update) === "1",
            rollout_percent: rolloutPercent,
            release_notes: releaseNotes
          }
        });
      });
    }
    if (["app-publish", "app-pause", "app-resume", "app-archive"].includes(action)) {
      const lifecycle = {
        "app-publish": ["publish", "版本已发布"],
        "app-pause": ["pause", "版本已暂停"],
        "app-resume": ["resume", "版本已恢复"],
        "app-archive": ["archive", "版本已归档"]
      }[action];
      return mutateAndRefresh(
        "/admin/api/app/releases/" + encodeURIComponent(id) + "/" + lifecycle[0],
        { method: "POST", body: {} },
        lifecycle[1]
      ).catch((error) => notify(error.message, true));
    }
    if (action === "setting-edit") {
      const row = cached("settings", id);
      return openForm("编辑系统设置", [
        {
          name: "value",
          label: row.is_secret ? "新的 JSON 密钥值" : "JSON 值",
          type: "textarea",
          value: row.is_secret ? "" : JSON.stringify(row.value, null, 2),
          wide: true
        },
        { name: "is_secret", label: "类型", options: [[0, "普通"], [1, "密钥"]], value: row.is_secret ? 1 : 0 }
      ], (values) => api("/admin/api/system/settings/" + encodeURIComponent(id), {
        method: "POST", body: {
          value: JSON.parse(values.value),
          is_secret: values.is_secret === "1", version: Number(row.version)
        }
      }));
    }
    if (action === "role-edit") {
      const row = (state.cache.rbac.roles || []).find((item) => String(item.id) === String(id));
      if (!row) {
        notify("角色数据已刷新，请重试", true);
        return;
      }
      const permissionIDs = (state.cache.rbac.permissions || [])
        .filter((permission) => (row.permissions || []).includes(permission.permission_key))
        .map((permission) => permission.id);
      return openForm("配置角色权限 · " + row.name, [
        {
          name: "permission_ids",
          label: "勾选角色权限",
          type: "checkboxes",
          options: (state.cache.rbac.permissions || []).map((permission) => [
            permission.id,
            permission.module + " · " + permission.name + "（" + permission.permission_key + "）"
          ]),
          value: permissionIDs,
          wide: true
        },
        { name: "status", label: "角色状态", options: [[1, "启用"], [0, "停用"]], value: row.status }
      ], (values) => {
        const uniqueIDs = decimalFormIDs(values.permission_ids);
        if (uniqueIDs.length > 100) throw new Error("角色权限不能超过 100 项");
        return api("/admin/api/rbac/roles/" + encodeURIComponent(id), {
          method: "POST",
          body: { permission_ids: uniqueIDs, status: Number(values.status) }
        });
      });
    }
    if (action === "admin-edit") {
      const row = (state.cache.rbac.admins || []).find((item) => String(item.id) === String(id));
      if (!row) {
        notify("管理员数据已刷新，请重试", true);
        return;
      }
      const roleIDs = (state.cache.rbac.roles || [])
        .filter((role) => (row.roles || []).includes(role.role_key))
        .map((role) => role.id);
      return openForm("编辑管理员 · " + row.username, [
        { name: "display_name", label: "显示名称", value: row.display_name, wide: true },
        {
          name: "role_ids",
          label: "勾选管理员角色",
          type: "checkboxes",
          options: (state.cache.rbac.roles || []).map((role) => [
            role.id, role.name + "（" + role.role_key + "）"
          ]),
          value: roleIDs,
          wide: true
        },
        { name: "status", label: "账号状态", options: [[1, "启用"], [0, "停用"]], value: row.status }
      ], (values) => {
        const displayName = String(values.display_name || "").trim();
        const roleIDsValue = decimalFormIDs(values.role_ids);
        if (!displayName || displayName.length > 100) {
          throw new Error("显示名称必填，且不能超过 100 个字");
        }
        if (!roleIDsValue.length || roleIDsValue.length > 20) {
          throw new Error("请至少分配一个角色，且不能超过 20 个");
        }
        return api("/admin/api/rbac/admins/" + encodeURIComponent(id), {
          method: "POST",
          body: {
            display_name: displayName,
            role_ids: roleIDsValue,
            status: Number(values.status)
          }
        });
      });
    }
    if (action === "admin-password") {
      const row = (state.cache.rbac.admins || []).find((item) => String(item.id) === String(id));
      if (!row) {
        notify("管理员数据已刷新，请重试", true);
        return;
      }
      return openForm("重置管理员密码 · " + row.username, [
        {
          name: "password",
          label: "新密码（12–128 个字符）",
          type: "password",
          placeholder: "请输入新密码",
          wide: true
        },
        {
          name: "password_confirm",
          label: "再次输入新密码",
          type: "password",
          placeholder: "请再次输入新密码",
          wide: true
        },
        {
          name: "reason",
          label: "重置原因（必填；保存后撤销该管理员后台与客服座席全部登录会话）",
          type: "textarea",
          placeholder: "请填写管理员重置密码的原因",
          wide: true
        }
      ], async (values) => {
        const password = String(values.password || "");
        const confirmation = String(values.password_confirm || "");
        const reason = String(values.reason || "").trim();
        const passwordLength = [...password].length;
        if (passwordLength < 12 || passwordLength > 128 || utf8ByteLength(password) > 512) {
          throw new Error("新密码长度必须为 12 到 128 个字符");
        }
        if (password !== confirmation) {
          throw new Error("两次输入的新密码不一致");
        }
        if (!reason || utf8ByteLength(reason) > 500) {
          throw new Error("重置原因必填，且不能超过 500 字节");
        }
        const result = await api(
          "/admin/api/rbac/admins/" + encodeURIComponent(id) + "/password",
          { method: "POST", body: { password, reason } }
        );
        const revokedSessions = Number(result.revoked_sessions || 0);
        const isCurrentAdministrator = String(state.me && state.me.id || "") === String(id);
        return {
          __formMessage: "管理员密码已重置，已撤销 " + revokedSessions + " 个后台/客服会话",
          __skipRefresh: true,
          __redirect: isCurrentAdministrator ? "/admin/login" : ""
        };
      });
    }
    if (action === "role-create") {
      const permissions = (state.cache.rbac.permissions || []).map((item) => [
        item.id, item.module + " · " + item.name + "（" + item.permission_key + "）"
      ]);
      return openForm("新建角色", [
        { name: "role_key", label: "角色标识" }, { name: "name", label: "名称" },
        { name: "description", label: "说明", wide: true },
        { name: "data_scope", label: "数据范围", options: [[1, "全部"], [2, "团队"], [3, "本人"]] },
        {
          name: "permission_ids",
          label: "勾选角色权限",
          type: "checkboxes",
          options: permissions,
          value: permissions.map((item) => item[0]),
          wide: true
        }
      ], (values) => api("/admin/api/rbac/roles", {
        method: "POST", body: {
          role_key: values.role_key, name: values.name, description: values.description,
          data_scope: Number(values.data_scope),
          permission_ids: decimalFormIDs(values.permission_ids)
        }
      }));
    }
    if (action === "admin-create") {
      const roles = (state.cache.rbac.roles || []).filter((item) => Number(item.status) === 1);
      return openForm("新建管理员", [
        { name: "username", label: "登录账号" }, { name: "display_name", label: "显示名称" },
        { name: "password", label: "初始密码（至少 12 位）", type: "password" },
        { name: "email", label: "邮箱" },
        {
          name: "role_ids",
          label: "勾选管理员角色",
          type: "checkboxes",
          options: roles.map((item) => [item.id, item.name + "（" + item.role_key + "）"]),
          value: roles.map((item) => item.id),
          wide: true
        }
      ], (values) => api("/admin/api/rbac/admins", {
        method: "POST", body: {
          username: values.username, display_name: values.display_name, password: values.password,
          email: values.email, role_ids: decimalFormIDs(values.role_ids)
        }
      }));
    }
  }

  document.addEventListener("click", function (event) {
    const target = event.target.closest("[data-action]");
    if (!target) return;
    event.preventDefault();
    void handleAction(target.dataset.action, target.dataset.id || "", target)
      .catch((error) => notify(error.message || "操作失败", true));
  });

  document.addEventListener("input", function (event) {
    const input = event.target.closest("[data-table-search]");
    if (!input) return;
    const model = state.tables[input.dataset.tableSearch || ""];
    if (!model) return;
    model.query = input.value;
    model.page = 1;
    if (model.remote) model.requestSerial += 1;
    state.tablePreferences[model.preferenceKey] = {
      query: model.query,
      page: model.page,
      pageSize: model.pageSize
    };
    if (model.searchTimer) window.clearTimeout(model.searchTimer);
    model.searchTimer = window.setTimeout(function () {
      model.searchTimer = null;
      if (model.remote) {
        void loadRemoteTable(model, true);
        return;
      }
      refreshTable(model.id, true);
    }, 180);
  });

  document.addEventListener("change", function (event) {
    const select = event.target.closest("[data-table-page-size]");
    if (!select) return;
    const model = state.tables[select.dataset.tablePageSize || ""];
    const pageSize = Number(select.value);
    if (!model || ![10, 20, 50, 100].includes(pageSize)) return;
    if (model.searchTimer) {
      window.clearTimeout(model.searchTimer);
      model.searchTimer = null;
    }
    model.pageSize = pageSize;
    model.page = 1;
    state.tablePreferences[model.preferenceKey] = {
      query: model.query,
      page: model.page,
      pageSize: model.pageSize
    };
    if (model.remote) {
      model.requestSerial += 1;
      void loadRemoteTable(model, false);
      return;
    }
    refreshTable(model.id, false);
  });

  document.getElementById("logout").addEventListener("click", async function () {
    await fetch("/admin/api/logout", {
      method: "POST", credentials: "same-origin", headers: { "X-CSRF-Token": csrfToken() }
    });
    sessionStorage.removeItem("claw_admin_csrf");
    window.location.replace("/admin/login");
  });

  window.addEventListener("hashchange", loadRoute);
  api("/admin/api/me").then(function (me) {
    state.me = me;
    document.querySelectorAll("[data-permission]").forEach((item) => {
      item.hidden = !has(item.dataset.permission);
    });
    return loadRoute();
  }).catch(errorPanel);
})();
