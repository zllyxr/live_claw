(function () {
  const layer = window.layui && window.layui.layer;
  const content = document.getElementById("page-content");
  const heading = document.getElementById("page-heading");
  const description = document.getElementById("page-description");
  const topTitle = document.getElementById("top-title");
  const actions = document.getElementById("page-actions");
  const consoleConfig = {
    apiBase: document.body.dataset.consoleApiBase || "/admin/api",
    base: document.body.dataset.consoleBase || "/admin",
    csrfKey: document.body.dataset.consoleCsrfKey || "claw_admin_csrf",
    defaultRoute: document.body.dataset.consoleDefaultRoute || "dashboard"
  };
  const state = {
    me: null,
    route: "",
    section: "",
    routeKey: "",
    cache: {},
    tableSequence: 0,
    tables: {},
    tablePreferences: {},
    remoteFilters: { online: "", permission: "", permission_granted: "1" },
    routeLoadSerial: 0
  };

  const pages = {
    "agent-teams": ["团队前缀", "团队前缀", "仅显示指定前缀与当前在队人数"],
    dashboard: ["数据统计", "平台概览", "关键业务数据、资金与待处理事项"],
    users: ["用户管理", "用户与团队", "账号状态、团队归属和邀请码体系"],
    agents: ["代理管理", "代理账号与团队前缀", "独立代理端授权及团队前缀归属"],
    wallet: ["资金管理", "资金审核与流水", "充值、提现、调账及逐场游戏输赢"],
    payments: ["支付管理", "支付通道与充值", "BEpusdt、银行卡收款、充值商品与订单审核"],
    games: ["游戏管理", "游戏与捕鱼场", "固定 300 桌、每桌 4 座，倍率 1 / 5 / 10"],
    live: ["抖音直播", "抖音直播间", "v2 仅允许经过审核的抖音 PAGE 直播"],
    lottery: ["彩票管理", "彩种、玩法与期号", "开奖先封盘，所有变更写入审计日志"],
    sports: ["体育管理", "赛事与盘口", "维护赛事、赔率、赛果并提交 Scheduler 结算"],
    bets: ["投注管理", "全平台投注", "统一查看彩票、体育和游戏投注与派彩"],
    im: ["IM 管理", "会话与群组", "原生单聊、群聊、成员、禁言和消息处置"],
    app: ["App 管理", "客户端版本", "原生强制更新与 WGT 静默热更新"],
    remote: ["远程设备", "远程协助设备", "用户主动授权、在线状态与一次性协助凭据"],
    rbac: ["角色与权限", "管理员权限", "最小权限角色和管理员授权"],
    system: ["系统设置", "系统设置与审计", "配置版本控制、密钥掩码和完整审计"]
  };

  const pageSections = {
    users: [["users", "用户列表"], ["teams", "团队管理"]],
    wallet: [["ledger", "资金流水"], ["adjustments", "后台调账"],
      ["recharges", "充值订单"], ["withdrawals", "提现订单"]],
    payments: [["channels", "支付通道"], ["banks", "收款银行卡"],
      ["products", "充值商品"], ["orders", "充值订单"]],
    games: [["catalog", "游戏目录"], ["venues", "捕鱼场次"]],
    lottery: [["games", "彩种列表"], ["categories", "彩票分类"], ["issues", "期号管理"]],
    sports: [["matches", "赛事管理"], ["sync", "同步状态"]],
    bets: [["lottery", "彩票投注"], ["sports", "体育投注"], ["games", "游戏结算"]],
    rbac: [["admins", "管理员"], ["roles", "角色"], ["permissions", "权限字典"]],
    system: [["settings", "系统设置"], ["audit", "审计日志"]]
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
    const date = numeric ? new Date(numeric * 1000) : new Date(String(value || ""));
    if (Number.isNaN(date.getTime())) return "—";
    return date.toLocaleString("zh-CN", { hour12: false });
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

  function apiPath(path) {
    const value = String(path || "");
    return value.indexOf("/admin/api") === 0 && consoleConfig.apiBase !== "/admin/api" ?
      consoleConfig.apiBase + value.slice("/admin/api".length) : value;
  }

  function csrfToken() {
    const stored = sessionStorage.getItem(consoleConfig.csrfKey) || "";
    if (stored) return stored;
    const prefix = consoleConfig.csrfKey + "=";
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
    const response = await fetch(apiPath(path), config);
    if (response.status === 401) {
      window.location.replace(consoleConfig.base + "/login");
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
    return String(route || state.routeKey || state.route) + ":" + key;
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
    const requestRoute = state.routeKey || state.route;
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

  function disabledButton(label, reason, style) {
    return '<span class="disabled-action"><button type="button" class="layui-btn layui-btn-sm ' +
      (style || "layui-btn-primary") + '" disabled aria-disabled="true" title="' + esc(reason) + '">' +
      esc(label) + '</button><small>' + esc(reason) + "</small></span>";
  }

  function routeButton(label, route, section, style) {
    return '<a class="layui-btn layui-btn-sm ' + (style || "layui-btn-primary") + '" href="#' +
      esc(route) + "/" + esc(section) + '">' + esc(label) + "</a>";
  }

  function normalizedSection(route, requestedSection) {
    const sections = pageSections[route] || [];
    const requested = String(requestedSection || "");
    return (sections.find((item) => item[0] === requested) || sections[0] || [""])[0];
  }

  function sectionNavigation(route, activeSection) {
    const sections = pageSections[route] || [];
    if (!sections.length) return "";
    return '<nav class="admin-tabs" aria-label="' + esc((pages[route] || pages.dashboard)[1]) +
      '二级导航" role="tablist">' + sections.map((item) =>
        '<a class="admin-tab ' + (activeSection === item[0] ? "active" : "") +
        '" role="tab" aria-selected="' + (activeSection === item[0] ? "true" : "false") +
        '" href="#' + esc(route) + "/" + esc(item[0]) + '">' + esc(item[1]) + "</a>"
      ).join("") + "</nav>";
  }

  function sectionBody(route, activeSection, bodies) {
    const fallback = normalizedSection(route, "");
    return sectionNavigation(route, activeSection) + (bodies[activeSection] || bodies[fallback] || "");
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
      loadContext.route === state.route &&
      loadContext.section === state.section);
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

  async function agentTeams(loadContext) {
    const data = await remoteTableData("agent-team-prefixes", "/admin/api/team-prefixes");
    if (!isCurrentRouteLoad(loadContext)) return;
    actions.insertAdjacentHTML("afterbegin",
      '<button class="layui-btn" data-action="agent-team-generate">生成团队前缀</button>');
    const prefixTable = table([
      { label: "邀请码前缀", render: (row) => "<strong>" + esc(row.code) + "</strong>" },
      { label: "当前在队人数", render: (row) => formatNumber(row.member_count) },
      { label: "操作", render: (row) => button("查看成员", "agent-team-members", row.code) }
    ], data.items, {
      key: "agent-team-prefixes", page: data.page, pageSize: data.page_size,
      total: data.total, hasMore: data.has_more,
      remote: { path: "/admin/api/team-prefixes" }
    });
    content.innerHTML = panel("团队前缀", "仅显示已分配的邀请码前三位和当前在队人数", prefixTable);
  }

  const agentPermissionLabels = {
    "games.read": "查看游戏", "games.write": "管理游戏",
    "live.read": "查看直播", "live.write": "管理直播",
    "lottery.read": "查看彩票", "lottery.write": "管理彩票",
    "sports.read": "查看体育", "sports.write": "管理体育",
    "bets.read": "查看投注", "app.read": "查看 App 版本", "app.write": "管理 App 版本"
  };

  function agentPermissionOptions(keys) {
    return (keys || []).map((key) => [key, agentPermissionLabels[key] || key]);
  }

  async function agentsView(loadContext) {
    const data = await remoteTableData("platform-agents", "/admin/api/agents");
    if (!isCurrentRouteLoad(loadContext)) return;
    state.cache.agents = data.items;
    state.cache.agentAllowedPermissions = data.allowed_permissions || [];
    if (has("agents.write")) {
      actions.insertAdjacentHTML("afterbegin",
        '<button class="layui-btn" data-action="agent-create">新建代理</button>');
    }
    const agentTable = table([
      { label: "代理", render: (row) => "<strong>" + esc(row.display_name || row.username) +
        "</strong><br><small>" + esc(row.username) + " · " + esc(row.agent_no) + "</small>", className: "wrap" },
      { label: "授权", render: (row) => esc((row.permissions || []).map((key) =>
        agentPermissionLabels[key] || key).join("、") || "仅团队前缀"), className: "wrap" },
      { label: "前缀数", render: (row) => formatNumber(row.prefix_count) },
      { label: "状态", render: (row) => statusTag(row.status, { 1: ["启用", "ok"], 0: ["停用", "bad"] }) },
      { label: "最后登录", render: (row) => formatTime(row.last_login_at) },
      { label: "操作", render: (row) => has("agents.write") ? '<div class="row-actions">' +
        button("编辑", "agent-edit", row.id, "layui-btn-normal") +
        button("重置密码", "agent-password", row.id, "layui-btn-warm") +
        button("查看前缀", "agent-prefixes", row.id) +
        button("分配前缀", "agent-prefix-assign", row.id, "layui-btn-primary") + "</div>" :
        button("查看前缀", "agent-prefixes", row.id) }
    ], data.items, {
      key: "platform-agents", page: data.page, pageSize: data.page_size,
      total: data.total, hasMore: data.has_more,
      remote: { path: "/admin/api/agents", cacheName: "agents" }
    });
    content.innerHTML = panel("代理账号", "代理使用独立入口登录；团队前缀与成员不受账号停用影响", agentTable);
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
    const activeSection = loadContext.section;
    if (has("users.write") && activeSection === "teams") {
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
    content.innerHTML = sectionBody("users", activeSection, {
      users: panel("用户列表", "余额、团队和账号状态", userTable),
      teams: panel("团队管理", "邀请码前三位即团队代码", teamTable)
    });
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
    const activeSection = loadContext.section;
    if (has("wallet.review") && activeSection === "adjustments") {
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
    content.innerHTML = sectionBody("wallet", activeSection, {
      ledger: panel("资金流水", "账本不可修改；游戏流水精确到场、桌和局", ledger),
      adjustments: panel("后台调账", "申请人与审核人必须为不同管理员", adjustments),
      recharges: panel("充值订单", "异常人工入账需要资金审核权限", recharge),
      withdrawals: panel("提现订单", "提现审核和打款状态独立管理", withdrawal)
    });
  }

  async function paymentsView(loadContext) {
    const result = await Promise.all([
      remoteTableData("payment-channels", "/admin/api/payments/channels"),
      remoteTableData("payment-products", "/admin/api/payments/products"),
      remoteTableData("payment-recharges", "/admin/api/payments/recharges"),
      remoteTableData("payment-bank-accounts", "/admin/api/payments/bank-accounts")
    ]);
    if (!isCurrentRouteLoad(loadContext)) return;
    state.cache.paymentChannels = result[0].items;
    state.cache.paymentProducts = result[1].items;
    state.cache.paymentRecharges = result[2].items;
    state.cache.paymentBankAccounts = result[3].items;
    const activeSection = loadContext.section;
    if (has("payments.write")) {
      const actionBySection = {
        banks: '<button class="layui-btn layui-btn-normal" data-action="payment-bank-create">新增收款银行卡</button>',
        products: '<button class="layui-btn" data-action="payment-product-create">新增充值商品</button>'
      };
      actions.insertAdjacentHTML("afterbegin", actionBySection[activeSection] || "");
    }

    const channels = table([
      {
        label: "通道",
        render: (row) => "<strong>" + esc(row.name) + "</strong><br><small>" +
          esc(row.channel_key) + "</small>",
        className: "wrap"
      },
      { label: "服务商", render: (row) => statusTag(row.provider, {
        bepusdt: ["BEpusdt", "ok"], manual_bank: ["银行卡转账", "warn"]
      }) },
      {
        label: "配置",
        render: (row) => row.provider === "manual_bank" ?
          '<div class="payment-config-state">' + statusTag(
            state.cache.paymentBankAccounts.some((account) => Number(account.status) === 1) ? 1 : 0,
            { 1: ["收款卡可用", "ok"], 0: ["请先启用收款卡", "warn"] }
          ) + "</div>" : row.provider === "bepusdt" ? '<div class="payment-config-state">' +
          statusTag(row.config_valid ? 1 : 0, {
            1: ["配置有效", "ok"], 0: [row.config_error || "未配置", "bad"]
          }) +
          statusTag(row.token_configured ? 1 : 0, {
            1: ["Token 已配置", "ok"], 0: ["Token 未配置", "warn"]
          }) +
          statusTag(row.config_verified ? 1 : 0, {
            1: ["签名检查通过", "ok"], 0: ["待签名检查", "warn"]
          }) + "</div>" : '<div class="payment-config-state">' +
          statusTag(0, { 0: ["服务端尚未接入", "warn"] }) + "</div>"
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
          if (!has("payments.write")) {
            return '<span class="read-only-reason">只读：缺少 payments.write 权限</span>';
          }
          const controls = [];
          if (row.provider === "bepusdt") {
            controls.push(button("编辑配置", "payment-channel-edit", row.id, "layui-btn-normal"));
            if (row.config_valid) {
              controls.push(button("协议检查", "payment-channel-check", row.id, "layui-btn-warm"));
            } else {
              controls.push(disabledButton("协议检查", "请先完成通道配置", "layui-btn-warm"));
            }
            if (Number(row.status) === 1) {
              controls.push(button("停用", "payment-channel-status", row.id, "layui-btn-danger"));
            } else if (row.config_verified) {
              controls.push(button(Number(row.status) === 1 ? "停用" : "启用",
                "payment-channel-status", row.id,
                Number(row.status) === 1 ? "layui-btn-danger" : "layui-btn-primary"));
            } else {
              controls.push(disabledButton("启用", "配置并通过协议检查后可启用"));
            }
          } else if (row.provider === "manual_bank") {
            controls.push(routeButton("管理银行卡", "payments", "banks", "layui-btn-normal"));
            if (Number(row.status) === 1) {
              controls.push(button("停用", "payment-channel-status", row.id, "layui-btn-danger"));
            } else if (state.cache.paymentBankAccounts.some((account) => Number(account.status) === 1)) {
              controls.push(button("启用", "payment-channel-status", row.id));
            } else {
              controls.push(disabledButton("启用", "请先新增并启用收款银行卡"));
            }
          } else {
            controls.push(disabledButton("编辑", "该服务商尚未接入服务端", "layui-btn-normal"));
            controls.push(disabledButton("启用", "该服务商尚未接入服务端"));
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

    const bankAccounts = table([
      {
        label: "收款卡",
        render: (row) => "<strong>" + esc(row.display_name) + "</strong><br><small>" +
          esc(row.card_number_masked) + "</small>",
        className: "wrap"
      },
      { label: "银行 / 支行", render: (row) => esc(row.bank_name) +
        "<br><small>" + esc(row.branch_name || "—") + "</small>", className: "wrap" },
      { label: "付款说明", render: (row) => esc(row.instructions || "—"), className: "wrap" },
      { label: "排序", key: "sort_order" },
      { label: "状态", render: (row) => statusTag(row.status, {
        1: ["启用", "ok"], 0: ["停用", "bad"]
      }) },
      {
        label: "操作",
        render: (row) => has("payments.write") ? '<div class="row-actions">' +
          button("编辑", "payment-bank-edit", row.id, "layui-btn-normal") +
          button(Number(row.status) === 1 ? "停用" : "启用", "payment-bank-status", row.id,
            Number(row.status) === 1 ? "layui-btn-danger" : "layui-btn-primary") +
          "</div>" : "—"
      }
    ], result[3].items, {
      key: "payment-bank-accounts",
      page: result[3].page,
      pageSize: result[3].page_size,
      total: result[3].total,
      hasMore: result[3].has_more,
      remote: { path: "/admin/api/payments/bank-accounts", cacheName: "paymentBankAccounts" }
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
        render: (row) => esc(row.channel || row.channel_key || "—") + "<br><small>" +
          esc(row.provider === "manual_bank" ?
            (row.bank_account_name ? row.bank_account_name + " · " + row.bank_account_masked : "尚未分配收款卡") :
            (row.provider_trade_id || "尚未生成")) + "</small>",
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
        0: [row.provider === "manual_bank" ? "待分配" : "已创建", "warn"],
        1: [row.bank_stage === "review_pending" ? "凭证待审核" : "支付中", "warn"], 2: ["已支付", "ok"],
        3: ["失败", "bad"], 4: ["已关闭", "bad"], 5: ["已退款", "warn"]
      }) },
      {
        label: "操作",
        render: (row) => {
          if (!has("wallet.review")) return "—";
          if (row.provider === "bepusdt" && row.provider_trade_id && [0, 1].includes(Number(row.status))) {
            return button("核验补账", "payment-recharge-mark-paid", row.id, "layui-btn-danger");
          }
          if (row.provider !== "manual_bank") return "—";
          const controls = [];
          if (row.bank_stage === "waiting_assignment") {
            controls.push(button("分配银行卡", "payment-bank-assign", row.id, "layui-btn-normal"));
            controls.push(button("关闭", "payment-bank-close", row.id, "layui-btn-danger"));
          } else if (row.bank_stage === "awaiting_payment") {
            controls.push(button("关闭", "payment-bank-close", row.id, "layui-btn-danger"));
          } else if (row.bank_stage === "review_pending") {
            controls.push(button("查看凭证", "payment-bank-proof-view", row.id, "layui-btn-normal"));
            controls.push(button("确认到账", "payment-bank-proof-approve", row.id, "layui-btn-warm"));
            controls.push(button("驳回关单", "payment-bank-proof-reject", row.id, "layui-btn-danger"));
          }
          return controls.length ? '<div class="row-actions">' + controls.join("") + "</div>" : "—";
        }
      }
    ], result[2].items, {
      key: "payment-recharges",
      page: result[2].page,
      pageSize: result[2].page_size,
      total: result[2].total,
      hasMore: result[2].has_more,
      remote: { path: "/admin/api/payments/recharges", cacheName: "paymentRecharges" }
    });

    const permissionNotice = '<div class="permission-notice ' +
      (has("payments.write") ? "ok" : "warn") + '"><strong>' +
      (has("payments.write") ? "当前账号可编辑支付配置" : "当前账号仅有查看权限") +
      '</strong><span>' + (has("payments.write") ?
        "不满足启用条件的按钮会保留并显示原因。" :
        "需要管理员授予 payments.write 后才能编辑或启停。") + "</span></div>";
    content.innerHTML = sectionNavigation("payments", activeSection) + permissionNotice + ({
      channels: panel("支付通道", "BEpusdt 需签名检查；银行卡通道至少需要一张启用的收款卡", channels),
      banks: panel("收款银行卡", "卡号与持卡人加密保存；列表仅显示掩码，已分配订单使用不可变快照", bankAccounts),
      products: panel("充值商品", "前端只展示启用商品；金额按法币最小单位存储", products),
      orders: panel("充值订单", "银行卡凭证确认后才入账；驳回凭证会立即关闭订单", recharges)
    }[activeSection] || "");
  }

  async function games(loadContext) {
    const data = await api("/admin/api/games");
    if (!isCurrentRouteLoad(loadContext)) return;
    state.cache.games = data.items;
    state.cache.venues = data.venues;
    const activeSection = loadContext.section;
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
      { label: "钱包模式", render: () => '<span class="tag ok">统一钱包</span>' },
      { label: "在线", render: (row) => formatNumber(row.active_sessions) },
      { label: "RTP", render: (row) => (Number(row.target_rtp_ppm) / 10000).toFixed(2) + "%" },
      { label: "操作", render: (row) => has("games.write") ? button("配置", "venue-edit", row.id) : "—" }
    ], data.venues);
    content.innerHTML = sectionBody("games", activeSection, {
      catalog: panel("游戏目录", "前端入口与游戏开关", gameTable),
      venues: panel("深海猎手场次", "固定 300 桌、每桌 4 座，随机分配空座", venueTable)
    });
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
    const activeSection = loadContext.section;
    if (has("lottery.write")) {
      const actionByTab = {
        games: '<button class="layui-btn" data-action="lottery-game">新增彩种</button>',
        categories: '<button class="layui-btn" data-action="lottery-category">新增分类</button>',
        issues: '<button class="layui-btn layui-btn-warm" data-action="lottery-issue">新建期号</button>'
      };
      actions.insertAdjacentHTML("afterbegin", actionByTab[activeSection] || "");
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
    const bodyByTab = {
      games: panel("彩种列表", "玩法只在彩种配置弹窗中维护，不铺满主页面", gameTable),
      categories: panel("彩票分类", "分类与彩种列表独立维护", categoryTable),
      issues: panel("彩票期号", "封盘、开奖和结算状态", issueTable)
    };
    content.innerHTML = sectionBody("lottery", activeSection, bodyByTab);
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
    const activeSection = loadContext.section;
    if (has("sports.write") && activeSection === "matches") {
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
    content.innerHTML = sectionBody("sports", activeSection, {
      matches: panel("体育赛事", "后台维护赛事、封盘、赛果和结算状态", matchTable),
      sync: panel("体育数据同步", "未配置 V2_SPORTS_API_KEY 时不会生成模拟赛事或赔率", syncTable)
    });
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
    const activeSection = loadContext.section;
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
    const bodies = {
      lottery: panel("彩票投注订单", "按期号记录投注和派彩", betOrderTable(
        lotteryOrders.items, betLabels, {
          key: "bets-lottery",
          page: lotteryOrders.page,
          pageSize: lotteryOrders.page_size,
          total: lotteryOrders.total,
          hasMore: lotteryOrders.has_more,
          remote: { path: "/admin/api/bets/lottery" }
        })),
      sports: panel("体育投注订单", "按赛事记录投注和派彩", betOrderTable(
        sportsOrders.items, betLabels, {
          key: "bets-sports",
          page: sportsOrders.page,
          pageSize: sportsOrders.page_size,
          total: sportsOrders.total,
          hasMore: sportsOrders.has_more,
          remote: { path: "/admin/api/bets/sports" }
        })),
      games: panel("游戏逐场结算", "精确到游戏、场次、桌号与会话", betOrderTable(
        gameOrders.items, gameLabels, {
          key: "bets-game",
          page: gameOrders.page,
          pageSize: gameOrders.page_size,
          total: gameOrders.total,
          hasMore: gameOrders.has_more,
          remote: { path: "/admin/api/bets/game" }
        }))
    };
    content.innerHTML = sectionNavigation("bets", activeSection) + metrics +
      (bodies[activeSection] || bodies.lottery);
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
    const activeSection = loadContext.section;
    if (has("rbac.write")) {
      const actionBySection = {
        admins: '<button class="layui-btn layui-btn-normal" data-action="admin-create">新建管理员</button>',
        roles: '<button class="layui-btn" data-action="role-create">新建角色</button>'
      };
      actions.insertAdjacentHTML("afterbegin", actionBySection[activeSection] || "");
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
    content.innerHTML = sectionBody("rbac", activeSection, {
      admins: panel("管理员", "管理员会话 12 小时过期，变更权限立即生效", adminTable),
      roles: panel("角色", "超级管理员及客服系统角色受保护，其他角色可配置权限", roleTable),
      permissions: panel("权限字典", "角色授权时可按权限 ID 选择", permissionTable)
    });
  }

  const systemSettingCatalog = {
    "platform.brand": {
      title: "平台品牌",
      group: "基础与内容",
      description: "设置客户端显示的名称、客服入口和维护状态。",
      fields: [
        { path: "name", label: "平台名称", type: "text", required: true, placeholder: "例如：星域" },
        { path: "support_url", label: "客服地址", type: "url", placeholder: "https://...", help: "可留空；填写后必须是 http 或 https 地址。" },
        { path: "maintenance", label: "维护模式", type: "boolean", help: "仅在需要暂停普通用户访问时开启。" }
      ]
    },
    "content.pages": {
      title: "协议与说明",
      group: "基础与内容",
      description: "维护客户端向用户展示的协议标题和正文。",
      fields: [
        { path: "recharge_agreement.title", label: "充值协议标题", type: "text", required: true },
        { path: "recharge_agreement.content", label: "充值协议正文", type: "textarea", required: true, wide: true, help: "支持正常换行，不需要添加 HTML 或其他代码。" }
      ]
    },
    "security.session": {
      title: "登录与会话",
      group: "账户与资金",
      description: "控制用户和管理员登录保持时间，以及是否限制单设备登录。",
      fields: [
        { path: "user_ttl_seconds", label: "用户登录有效期", type: "number", scale: 86400, unit: "天", min: 1, max: 365, step: 1, integer: true, required: true },
        { path: "admin_ttl_seconds", label: "管理员登录有效期", type: "number", scale: 3600, unit: "小时", min: 1, max: 168, step: 1, integer: true, required: true },
        { path: "single_device", label: "只允许一台设备登录", type: "boolean", help: "开启后，新设备登录会使旧设备会话失效。" }
      ]
    },
    "invite.policy": {
      title: "邀请码规则",
      group: "账户与资金",
      description: "设置团队邀请码和个人邀请码的组成及保留时间。",
      fields: [
        { path: "alphabet", label: "邀请码可用字符", type: "text", required: true, wide: true, help: "建议只使用不易混淆的数字和小写字母。" },
        { path: "format", label: "邀请码格式", type: "text", required: true, placeholder: "xxx-xxxx", help: "x 代表一个字符，短横线会原样显示。" },
        { path: "team_prefix_length", label: "团队前缀长度", type: "number", min: 1, max: 12, step: 1, integer: true, required: true },
        { path: "personal_code_length", label: "个人码长度", type: "number", min: 1, max: 20, step: 1, integer: true, required: true },
        { path: "alias_retention_days", label: "旧邀请码保留时间", type: "number", unit: "天", min: 0, max: 3650, step: 1, integer: true, required: true }
      ]
    },
    "wallet.policy": {
      title: "钱包规则",
      group: "账户与资金",
      description: "设置钱包币种、充值提现开关、最低提现金额和手续费。",
      fields: [
        { path: "currency", label: "钱包币种代码", type: "text", required: true, placeholder: "COIN" },
        { path: "recharge_enabled", label: "允许充值", type: "boolean" },
        { path: "withdraw_enabled", label: "允许提现", type: "boolean" },
        { path: "min_withdraw_coin", label: "最低提现金额", type: "number", unit: "星币", min: 0, step: 1, required: true },
        { path: "withdraw_fee_coin", label: "每笔提现手续费", type: "number", unit: "星币", min: 0, step: 1, required: true }
      ]
    },
    "game.fishing": {
      title: "捕鱼房间规则",
      group: "业务功能",
      description: "控制捕鱼房间的分配方式、桌椅数量和场次倍率。",
      fields: [
        { path: "allocation", label: "入场分配方式", type: "select", required: true, options: [["random_table_random_seat", "随机分桌并随机座位"]] },
        { path: "tables_per_venue", label: "每个场次桌数", type: "number", unit: "桌", min: 1, max: 10000, step: 1, integer: true, required: true },
        { path: "seats_per_table", label: "每桌座位数", type: "number", unit: "座", min: 1, max: 20, step: 1, integer: true, required: true },
        { path: "venues", label: "场次与倍率", type: "venues", required: true, wide: true, placeholder: "novice: 1\nexpert: 5\nmaster: 10", help: "每行填写“场次代码: 倍率”，例如 novice: 1。" }
      ]
    },
    "live.provider": {
      title: "直播来源",
      group: "业务功能",
      description: "设置允许接入的直播来源和解析结果缓存时间。",
      fields: [
        { path: "allowed", label: "允许的直播来源", type: "list", required: true, placeholder: "douyin", help: "多个来源用逗号或换行分隔。" },
        { path: "resolve_cache_seconds", label: "解析缓存时间", type: "number", unit: "秒", min: 0, max: 3600, step: 1, integer: true, required: true }
      ]
    },
    "im.policy": {
      title: "聊天功能",
      group: "业务功能",
      description: "分别控制私聊、群聊、直播群聊和群成员上限。",
      fields: [
        { path: "direct_enabled", label: "允许私聊", type: "boolean" },
        { path: "group_enabled", label: "允许群聊", type: "boolean" },
        { path: "live_group_enabled", label: "允许直播群聊", type: "boolean" },
        { path: "max_group_members", label: "每个群最多人数", type: "number", unit: "人", min: 2, max: 100000, step: 1, integer: true, required: true }
      ]
    },
    "lottery.policy": {
      title: "彩票总开关",
      group: "业务功能",
      description: "控制彩票业务是否开放，以及手动开奖是否必须经过审计。",
      fields: [
        { path: "enabled", label: "开放彩票业务", type: "boolean" },
        { path: "manual_draw_requires_audit", label: "手动开奖必须审计", type: "boolean", help: "建议保持开启，便于追踪所有人工开奖操作。" }
      ]
    },
    "app.update": {
      title: "客户端更新",
      group: "客户端",
      description: "控制强制更新、静默热更新和新版本灰度比例。",
      fields: [
        { path: "force_update_enabled", label: "允许强制更新", type: "boolean" },
        { path: "silent_hot_update_enabled", label: "允许静默热更新", type: "boolean" },
        { path: "rollout_percent", label: "新版本覆盖比例", type: "number", unit: "%", min: 0, max: 100, step: 1, integer: true, required: true }
      ]
    }
  };

  const systemSettingGroupOrder = ["基础与内容", "账户与资金", "业务功能", "客户端", "其他设置"];

  function systemSettingValueAtPath(value, path) {
    return String(path || "").split(".").reduce((current, segment) =>
      current && typeof current === "object" ? current[segment] : undefined, value);
  }

  function assignSystemSettingValue(target, path, value) {
    const segments = String(path || "").split(".");
    let current = target;
    segments.forEach((segment, index) => {
      if (index === segments.length - 1) {
        current[segment] = value;
        return;
      }
      if (!current[segment] || typeof current[segment] !== "object" || Array.isArray(current[segment])) {
        current[segment] = {};
      }
      current = current[segment];
    });
  }

  function cloneSystemSettingValue(value) {
    if (!value || typeof value !== "object") return {};
    return JSON.parse(JSON.stringify(value));
  }

  function systemSettingInputValue(field, value) {
    if (field.type === "boolean") return value ? "1" : "0";
    if (field.type === "list") return Array.isArray(value) ? value.join(", ") : "";
    if (field.type === "venues") {
      return Array.isArray(value) ? value.map((venue) =>
        String(venue.code || "") + ": " + String(venue.multiplier ?? "")).join("\n") : "";
    }
    if (field.scale && Number.isFinite(Number(value))) return Number(value) / field.scale;
    return value ?? "";
  }

  function systemSettingPreview(field, value) {
    if (field.type === "boolean") {
      return '<span class="setting-boolean ' + (value ? "on" : "off") + '">' +
        (value ? "已开启" : "已关闭") + "</span>";
    }
    if (field.type === "list") {
      const values = Array.isArray(value) ? value : [];
      return esc(values.length ? values.join("、") : "未设置");
    }
    if (field.type === "venues") {
      const venues = Array.isArray(value) ? value : [];
      return esc(venues.length ? venues.map((venue) =>
        String(venue.code || "未命名") + "（" + String(venue.multiplier ?? 0) + " 倍）").join("、") : "未设置");
    }
    if (field.type === "select") {
      const option = (field.options || []).find((item) => String(item[0]) === String(value));
      return esc(option ? option[1] : (value || "未设置"));
    }
    let displayValue = field.scale && Number.isFinite(Number(value)) ? Number(value) / field.scale : value;
    if (displayValue === undefined || displayValue === null || displayValue === "") return "未设置";
    displayValue = String(displayValue);
    if (displayValue.length > 120) displayValue = displayValue.slice(0, 120) + "…";
    return esc(displayValue) + (field.unit ? '<span class="setting-unit">' + esc(field.unit) + "</span>" : "");
  }

  function systemSettingCard(row) {
    const definition = systemSettingCatalog[row.key];
    const title = definition ? definition.title : row.key;
    const description = definition ? definition.description :
      "该配置暂未提供普通表单，请交由熟悉系统的技术人员维护。";
    let summary;
    if (row.is_secret) {
      const configured = Boolean(row.value && row.value.configured);
      summary = '<div class="setting-secret-state"><span class="setting-boolean ' +
        (configured ? "on" : "off") + '">' + (configured ? "密钥已配置" : "密钥未配置") +
        "</span><small>出于安全原因，密钥内容不会在此显示</small></div>";
    } else if (definition) {
      summary = '<dl class="setting-summary">' + definition.fields.map((field) =>
        '<div class="' + (field.wide ? "wide" : "") + '"><dt>' + esc(field.label) +
        "</dt><dd>" + systemSettingPreview(field, systemSettingValueAtPath(row.value, field.path)) +
        "</dd></div>").join("") + "</dl>";
    } else {
      summary = '<div class="setting-advanced-state">当前配置已加载，但不在页面直接展示代码。</div>';
    }
    const action = has("system.write") ? button(definition ? "编辑设置" : "高级编辑", "setting-edit", row.key,
      definition ? "layui-btn-normal" : "layui-btn-primary") : '<span class="read-only-reason">只读：缺少 system.write 权限</span>';
    return '<article class="system-setting-card"><header><div><h3>' + esc(title) +
      "</h3><p>" + esc(description) + '</p></div><span class="setting-version">版本 ' +
      esc(row.version) + "</span></header>" + summary + '<footer><div><span>最后更新：' +
      esc(formatTime(row.updated_at)) + '</span><code title="配置编号">' + esc(row.key) +
      "</code></div>" + action + "</footer></article>";
  }

  function systemSettingCards(items) {
    const groups = {};
    (items || []).forEach((row) => {
      const group = systemSettingCatalog[row.key]?.group || "其他设置";
      if (!groups[group]) groups[group] = [];
      groups[group].push(row);
    });
    const body = systemSettingGroupOrder.filter((group) => groups[group]?.length).map((group) =>
      '<section class="system-setting-group"><div class="system-setting-group-title"><h3>' +
      esc(group) + "</h3><span>" + esc(groups[group].length) + " 项</span></div>" +
      '<div class="system-setting-grid">' + groups[group].map(systemSettingCard).join("") +
      "</div></section>").join("");
    return '<div class="system-settings-shell">' +
      (body || '<div class="empty-state">暂无系统设置</div>') + "</div>";
  }

  function systemSettingFormFields(row) {
    const definition = systemSettingCatalog[row.key];
    if (definition) {
      return definition.fields.map((field, index) => ({
        name: "setting_field_" + index,
        label: field.label + (field.unit ? "（" + field.unit + "）" : ""),
        type: field.type === "boolean" || field.type === "select" ? undefined :
          ["venues", "list"].includes(field.type) ? "textarea" : field.type,
        options: field.type === "boolean" ? [[1, "开启"], [0, "关闭"]] : field.options,
        value: systemSettingInputValue(field, systemSettingValueAtPath(row.value, field.path)),
        placeholder: field.placeholder || "",
        help: field.help || "",
        min: field.min,
        max: field.max,
        step: field.step,
        required: field.required,
        wide: field.wide
      }));
    }
    if (row.is_secret) {
      return [{
        name: "secret_value",
        label: "新的密钥值",
        type: "password",
        required: true,
        wide: true,
        help: "保存后不会再次显示，请确认内容正确后再提交。"
      }];
    }
    return [{
      name: "advanced_value",
      label: "高级配置内容（JSON）",
      type: "textarea",
      value: JSON.stringify(row.value, null, 2),
      required: true,
      wide: true,
      help: "该设置还没有可视化表单，仅建议技术人员修改。"
    }];
  }

  function parseSystemSettingField(field, rawValue) {
    const raw = String(rawValue ?? "");
    const trimmed = raw.trim();
    if (field.required && !trimmed) throw new Error("请填写“" + field.label + "”");
    if (field.type === "boolean") return trimmed === "1";
    if (field.type === "number") {
      const value = Number(trimmed);
      if (!Number.isFinite(value)) throw new Error("“" + field.label + "”必须是有效数字");
      if (field.integer && !Number.isInteger(value)) throw new Error("“" + field.label + "”必须是整数");
      if (field.min !== undefined && value < field.min) throw new Error("“" + field.label + "”不能小于 " + field.min);
      if (field.max !== undefined && value > field.max) throw new Error("“" + field.label + "”不能大于 " + field.max);
      return field.scale ? value * field.scale : value;
    }
    if (field.type === "list") {
      const values = trimmed.split(/[,，\n]/).map((item) => item.trim()).filter(Boolean);
      if (field.required && !values.length) throw new Error("请至少填写一个“" + field.label + "”");
      return Array.from(new Set(values));
    }
    if (field.type === "venues") {
      const venues = trimmed.split(/\n/).map((line) => line.trim()).filter(Boolean).map((line) => {
        const match = line.match(/^([^:：,，\s]+)\s*[:：,，]\s*(\d+(?:\.\d+)?)$/);
        if (!match || Number(match[2]) <= 0) {
          throw new Error("场次格式不正确，请按“场次代码: 倍率”每行填写一项");
        }
        return { code: match[1], multiplier: Number(match[2]) };
      });
      if (field.required && !venues.length) throw new Error("请至少填写一个捕鱼场次");
      return venues;
    }
    if (field.type === "url" && trimmed && !safeHTTPURL(trimmed)) {
      throw new Error("“" + field.label + "”必须是 http 或 https 地址");
    }
    return trimmed;
  }

  function saveSystemSettingForm(row, values) {
    const definition = systemSettingCatalog[row.key];
    let value;
    if (definition) {
      value = cloneSystemSettingValue(row.value);
      definition.fields.forEach((field, index) => {
        assignSystemSettingValue(value, field.path,
          parseSystemSettingField(field, values["setting_field_" + index]));
      });
    } else if (row.is_secret) {
      const secret = String(values.secret_value || "").trim();
      if (!secret) throw new Error("请填写新的密钥值");
      value = secret;
    } else {
      try {
        value = JSON.parse(values.advanced_value);
      } catch (_) {
        throw new Error("高级配置格式不正确，请检查后重试");
      }
    }
    return api("/admin/api/system/settings/" + encodeURIComponent(row.key), {
      method: "POST",
      body: { value, is_secret: Boolean(row.is_secret), version: Number(row.version) }
    });
  }

  async function systemView(loadContext) {
    const result = await Promise.all([
      api("/admin/api/system/settings"),
      remoteTableData("system-audit", "/admin/api/system/audit")
    ]);
    if (!isCurrentRouteLoad(loadContext)) return;
    state.cache.settings = result[0].items;
    const activeSection = loadContext.section;
    const settingCards = systemSettingCards(result[0].items);
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
    content.innerHTML = sectionBody("system", activeSection, {
      settings: panel("系统设置", "按业务含义填写并保存，无需编写 JSON；密钥只显示是否已配置", settingCards),
      audit: panel("审计日志", "后台重要操作不可静默执行", auditTable)
    });
  }

  function remotePermissionSummary(value) {
    const labels = {
      notification: "通知", media_projection: "录屏", accessibility: "无障碍", battery: "电池白名单"
    };
    let source = value;
    if (typeof source === "string") {
      try { source = JSON.parse(source); } catch (_) { source = {}; }
    }
    source = source && typeof source === "object" ? source : {};
    return Object.entries(labels).map(([key, label]) =>
      '<span class="tag ' + (source[key] ? "ok" : "warn") + '">' +
      esc(label + (source[key] ? " ✓" : " ×")) + "</span>").join(" ");
  }

  async function remoteView(loadContext) {
    const data = await remoteTableData("remote-devices", "/admin/api/remote/devices", state.remoteFilters);
    if (!isCurrentRouteLoad(loadContext)) return;
    state.cache.remoteDevices = data.items || [];
    const deviceTable = table([
      { label: "用户", render: (row) => "<strong>" + esc(row.username || row.user_id) +
        "</strong><br><small>ID " + esc(row.user_id) + "</small>" },
      { label: "设备", render: (row) => esc(row.device_name || row.model || "未命名设备") +
        "<br><small>" + esc([row.manufacturer, row.model, "Android " + row.android_version].filter(Boolean).join(" · ")) + "</small>" },
      { label: "设备代码", render: (row) => row.device_code ? "<strong>" + esc(row.device_code) + "</strong>" : "—" },
      { label: "状态", render: (row) => '<span class="tag ' + (row.online ? "ok" : "warn") + '">' +
        (row.online ? "在线" : "离线") + "</span> " + esc(row.service_status || "unknown") +
        (Number(row.status) === 2 ? ' <span class="tag warn">撤销中</span>' : "") +
        (row.current_session ? ' <span class="tag ok">会话中</span>' : "") },
      { label: "权限", render: (row) => remotePermissionSummary(row.permission_status), className: "wrap" },
      { label: "版本 / 最后在线", render: (row) => esc((row.app_version || "—") + " / 插件 " + (row.plugin_version || "—")) +
        "<br><small>" + esc(formatTime(row.last_seen_at)) + "</small>" },
      { label: "操作", render: (row) => {
        const controls = [];
        if (has("remote.control")) {
          controls.push(Number(row.status) === 1 && row.online && row.service_status === "running" && row.device_code ?
            button("发起协助", "remote-credential", row.id, "layui-btn-normal") :
            disabledButton("发起协助", "设备需在线并运行远程服务"));
        }
        if (has("remote.revoke")) controls.push(Number(row.status) === 1 ?
          button("停用", "remote-revoke", row.id, "layui-btn-danger") : disabledButton("停用中", "等待设备确认停止"));
        return controls.join(" ") || "—";
      }}
    ], data.items, {
      key: "remote-devices", page: data.page, pageSize: data.page_size,
      total: data.total, hasMore: data.has_more,
      remote: { path: "/admin/api/remote/devices", cacheName: "remoteDevices", params: state.remoteFilters }
    });
    const filters = '<div class="toolbar">' +
      '<select id="remote-filter-online" class="layui-select"><option value="">全部在线状态</option>' +
      '<option value="1"' + (state.remoteFilters.online === "1" ? " selected" : "") + '>在线</option>' +
      '<option value="0"' + (state.remoteFilters.online === "0" ? " selected" : "") + '>离线</option></select>' +
      '<select id="remote-filter-permission" class="layui-select"><option value="">全部权限状态</option>' +
      Object.entries({ notification: "前台通知", media_projection: "屏幕共享", accessibility: "无障碍", battery: "电池白名单" })
        .map(([value, label]) => '<option value="' + esc(value) + '"' + (state.remoteFilters.permission === value ? " selected" : "") + '>' + esc(label) + '</option>').join("") +
      '</select><select id="remote-filter-permission-state" class="layui-select">' +
      '<option value="1"' + (state.remoteFilters.permission_granted === "1" ? " selected" : "") + '>已开启</option>' +
      '<option value="0"' + (state.remoteFilters.permission_granted === "0" ? " selected" : "") + '>未开启</option></select>' +
      '<button class="layui-btn" data-action="remote-filter">应用筛选</button></div>';
    content.innerHTML = panel("远程设备", "在线判定为 20 秒内收到心跳；打开控制台前需要复核管理员密码", filters + deviceTable);
  }

  const loaders = {
    "agent-teams": agentTeams,
    dashboard, users, agents: agentsView, wallet: walletView, payments: paymentsView, games, live: liveView,
    lottery: lotteryView, sports: sportsView, bets: betsView,
    im: imView, app: appView, remote: remoteView, rbac: rbacView, system: systemView
  };

  function showRemoteConsole(authorization, device) {
    const modalID = "remote-console-" + Date.now();
    const token = String(authorization.control_token || "");
    const deviceID = String(device.id || "");
    const html = '<div id="' + modalID + '" class="remote-console">' +
      '<div class="remote-console-main"><div class="remote-screen-stage">' +
      '<div class="remote-screen-placeholder">正在等待手机画面…</div>' +
      '<img class="remote-screen" alt="远程设备实时画面" draggable="false"></div>' +
      '<div class="remote-console-status">已授权，等待第一帧</div></div>' +
      '<div class="remote-console-tools">' +
      '<div class="remote-system-actions"><button type="button" data-system-action="back">返回</button>' +
      '<button type="button" data-system-action="home">主页</button>' +
      '<button type="button" data-system-action="recents">最近任务</button></div>' +
      '<label>文字输入<textarea class="remote-text" maxlength="2048" placeholder="输入后发送到手机当前输入框"></textarea></label>' +
      '<button type="button" class="layui-btn remote-send-text">发送文字</button>' +
      '<button type="button" class="layui-btn layui-btn-primary remote-set-clipboard">写入手机剪贴板</button>' +
      '<p>在画面上点击或拖动即可操作。手机必须已开启无障碍服务。</p></div></div>';
    let active = true;
    let frameTimer = 0;
    let objectURL = "";
    let lastSequence = "";
    let pointerStart = null;
    const updateStatus = (message, failed) => {
      const node = document.querySelector("#" + modalID + " .remote-console-status");
      if (node) { node.textContent = message; node.classList.toggle("error", Boolean(failed)); }
    };
    const remoteRequest = async (path, options) => {
      const config = Object.assign({ credentials: "same-origin" }, options || {});
      config.headers = Object.assign({ "X-Remote-Session": token }, config.headers || {});
      if (config.body && typeof config.body !== "string") {
        config.headers["Content-Type"] = "application/json";
        config.body = JSON.stringify(config.body);
      }
      if (config.method && config.method !== "GET") config.headers["X-CSRF-Token"] = csrfToken();
      const response = await fetch(path, config);
      if (!response.ok) {
        let message = response.status === 401 ? "控制授权已过期，请关闭后重新发起" : "远程控制请求失败";
        try { message = (await response.json()).message || message; } catch (_) { }
        throw new Error(message);
      }
      return response;
    };
    const sendControl = async (type, payload) => {
      try {
        await remoteRequest("/admin/api/remote/devices/" + encodeURIComponent(deviceID) + "/control", {
          method: "POST", body: { type: type, payload: payload || {} }
        });
        updateStatus("控制命令已发送", false);
      } catch (error) {
        updateStatus(error.message || "控制命令发送失败", true);
      }
    };
    const pollFrame = async () => {
      if (!active) return;
      try {
        const response = await remoteRequest("/admin/api/remote/devices/" + encodeURIComponent(deviceID) + "/frame");
        if (response.status === 204) {
          updateStatus("手机在线，正在等待画面", false);
        } else {
          const sequence = response.headers.get("X-Frame-Sequence") || "";
          if (sequence !== lastSequence) {
            const blob = await response.blob();
            const nextURL = URL.createObjectURL(blob);
            const image = document.querySelector("#" + modalID + " .remote-screen");
            if (image) { image.src = nextURL; image.classList.add("ready"); }
            const placeholder = document.querySelector("#" + modalID + " .remote-screen-placeholder");
            if (placeholder) placeholder.hidden = true;
            if (objectURL) URL.revokeObjectURL(objectURL);
            objectURL = nextURL;
            lastSequence = sequence;
          }
          updateStatus("画面已连接 · 帧 " + (sequence || "—"), false);
        }
      } catch (error) {
        updateStatus(error.message || "画面连接中断", true);
      } finally {
        if (active) frameTimer = window.setTimeout(pollFrame, 500);
      }
    };
    layer.open({
      type: 1, title: "远程控制 · " + esc(device.device_name || device.device_code || deviceID),
      area: ["960px", "88vh"], content: html,
      success: function () {
        const root = document.getElementById(modalID);
        const image = root?.querySelector(".remote-screen");
        image?.addEventListener("pointerdown", function (event) {
          event.preventDefault();
          image.setPointerCapture?.(event.pointerId);
          const bounds = image.getBoundingClientRect();
          pointerStart = { x: (event.clientX - bounds.left) / bounds.width, y: (event.clientY - bounds.top) / bounds.height, at: Date.now() };
        });
        image?.addEventListener("pointerup", function (event) {
          if (!pointerStart) return;
          event.preventDefault();
          const bounds = image.getBoundingClientRect();
          const end = { x: Math.max(0, Math.min(1, (event.clientX - bounds.left) / bounds.width)), y: Math.max(0, Math.min(1, (event.clientY - bounds.top) / bounds.height)) };
          const start = pointerStart;
          pointerStart = null;
          const distance = Math.hypot(end.x - start.x, end.y - start.y);
          if (distance < 0.015) void sendControl("tap", { x: start.x, y: start.y });
          else void sendControl("swipe", { x1: start.x, y1: start.y, x2: end.x, y2: end.y, duration_ms: Math.max(80, Math.min(1500, Date.now() - start.at)) });
        });
        root?.querySelectorAll("[data-system-action]").forEach((button) => button.addEventListener("click", function () {
          void sendControl("system_action", { action: button.dataset.systemAction });
        }));
        root?.querySelector(".remote-send-text")?.addEventListener("click", function () {
          const value = root.querySelector(".remote-text")?.value || "";
          if (value) void sendControl("text", { text: value });
        });
        root?.querySelector(".remote-set-clipboard")?.addEventListener("click", function () {
          const value = root.querySelector(".remote-text")?.value || "";
          if (value) void sendControl("clipboard_set", { text: value });
        });
        void pollFrame();
      },
      end: function () {
        active = false;
        if (frameTimer) window.clearTimeout(frameTimer);
        if (objectURL) URL.revokeObjectURL(objectURL);
        void remoteRequest("/admin/api/remote/devices/" + encodeURIComponent(deviceID) + "/sessions/end", { method: "POST", body: {} }).catch(() => {});
      }
    });
  }

  function resetTableRegistry() {
    Object.values(state.tables).forEach((model) => {
      if (model.searchTimer) window.clearTimeout(model.searchTimer);
      model.requestSerial += 1;
    });
    state.tables = {};
    state.tableSequence = 0;
  }

  async function loadRoute() {
    const requestedPath = (window.location.hash || "#" + consoleConfig.defaultRoute).slice(1).split("/");
    const requestedRoute = requestedPath[0];
    const route = pages[requestedRoute] && loaders[requestedRoute] ? requestedRoute : consoleConfig.defaultRoute;
    const section = normalizedSection(route, requestedPath[1]);
    const loadContext = { route, section, serial: ++state.routeLoadSerial };
    state.route = route;
    state.section = section;
    state.routeKey = route + (section ? "/" + section : "");
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
        const attributes = (field.required ? " required" : "") +
          (field.min !== undefined ? ' min="' + esc(field.min) + '"' : "") +
          (field.max !== undefined ? ' max="' + esc(field.max) + '"' : "") +
          (field.step !== undefined ? ' step="' + esc(field.step) + '"' : "");
        const help = field.help ? '<small class="form-help">' + esc(field.help) + "</small>" : "";
        const options = field.type === "checkboxes" ?
          '<div class="form-check-grid">' + (field.options || []).map((item) =>
            '<label class="form-check-option"><input type="checkbox" name="' +
            esc(field.name) + '" value="' + esc(item[0]) + '"' +
            (selectedValues.includes(String(item[0])) ? " checked" : "") +
            '><span>' + esc(item[1]) + "</span></label>").join("") + "</div>" :
          field.options ? '<select name="' + esc(field.name) + '"' + attributes + ">" +
          field.options.map((item) => '<option value="' + esc(item[0]) + '"' +
            (selectedValues.includes(String(item[0])) ? " selected" : "") + ">" + esc(item[1]) + "</option>").join("") +
          "</select>" : field.type === "textarea" ?
          '<textarea name="' + esc(field.name) + '" placeholder="' + esc(field.placeholder || "") + '"' +
          attributes + ">" + esc(field.value ?? "") + "</textarea>" :
          field.type === "file" ?
          '<input name="' + esc(field.name) + '" type="file" accept="' + esc(field.accept || "") + '">' :
          '<input name="' + esc(field.name) + '" type="' + esc(field.type || "text") +
          '" value="' + esc(field.value ?? "") + '" placeholder="' + esc(field.placeholder || "") + '"' + attributes +
          (field.inputmode ? ' inputmode="' + esc(field.inputmode) + '"' : "") +
          (field.readonly ? " readonly" : "") + ">";
        if (field.type === "checkboxes") {
          return '<div class="form-check-field ' + (field.wide ? "wide" : "") +
            '"><span class="form-check-label">' + esc(field.label) + "</span>" + options + help + "</div>";
        }
        return '<label class="' + (field.wide ? "wide" : "") + '">' +
          esc(field.label) + options + help + "</label>";
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

  function paymentBankAccountForm(account) {
    const editing = Boolean(account);
    return openForm(editing ? "编辑收款银行卡" : "新增收款银行卡", [
      { name: "display_name", label: "显示名称", value: account?.display_name || "", wide: true },
      { name: "bank_name", label: "银行名称", value: account?.bank_name || "" },
      { name: "branch_name", label: "开户支行（选填）", value: account?.branch_name || "", wide: true },
      { name: "holder_name", label: "收款人姓名", value: account?.holder_name || "" },
      {
        name: "card_number",
        label: editing ? "银行卡号（留空保留原卡号）" : "银行卡号",
        value: "", inputmode: "numeric", wide: true,
        placeholder: editing ? account.card_number_masked : "支持输入空格或短横线"
      },
      { name: "instructions", label: "付款说明（选填）", type: "textarea", value: account?.instructions || "", wide: true },
      { name: "sort_order", label: "排序", type: "number", value: account?.sort_order ?? 0 }
    ], (values) => {
      const displayName = String(values.display_name || "").trim();
      const bankName = String(values.bank_name || "").trim();
      const branchName = String(values.branch_name || "").trim();
      const holderName = String(values.holder_name || "").trim();
      const cardNumber = String(values.card_number || "").trim();
      const instructions = String(values.instructions || "").trim();
      const sortOrder = Number(values.sort_order);
      const normalizedCard = cardNumber.replace(/[\s_-]/g, "");
      if (!displayName || [...displayName].length > 100 || !bankName || [...bankName].length > 190 ||
          [...branchName].length > 190 || !holderName || [...holderName].length > 100 ||
          [...instructions].length > 500 || !Number.isSafeInteger(sortOrder) || Math.abs(sortOrder) > 1000000) {
        throw new Error("请检查显示名称、银行、支行、收款人、付款说明和排序");
      }
      if ((!editing || cardNumber) && !/^\d{12,30}$/.test(normalizedCard)) {
        throw new Error("银行卡号必须为12到30位数字");
      }
      const path = editing ?
        "/admin/api/payments/bank-accounts/" + encodeURIComponent(account.id) :
        "/admin/api/payments/bank-accounts";
      return api(path, {
        method: "POST",
        body: {
          display_name: displayName, bank_name: bankName, branch_name: branchName,
          holder_name: holderName, card_number: cardNumber,
          instructions, sort_order: sortOrder
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
    if (action === "agent-team-generate") {
      return mutateAndRefresh("/admin/api/team-prefixes", { method: "POST" }, "团队前缀已生成");
    }
    if (action === "agent-team-members") {
      const code = String(id || "").trim();
      if (!/^[0-9a-z]{3}$/.test(code) || code === "sys") throw new Error("团队前缀无效");
      const members = await fetchAllRemoteItems("/admin/api/team-prefixes/" +
        encodeURIComponent(code) + "/members");
      const memberRows = members.map((member) => "<tr><td><strong>" + esc(member.id) +
        "</strong></td><td>" + esc(member.nickname) + "</td><td>" + statusTag(member.status, {
          1: ["正常", "ok"], 2: ["冻结", "warn"], 3: ["已关闭", "bad"]
        }) + "</td><td>" + formatTime(member.joined_at) + "</td><td>" +
        (member.is_owner ? '<span class="tag ok">负责人</span>' : "成员") + "</td></tr>").join("");
      layer.open({
        type: 1, title: "团队成员 · " + esc(code), area: ["860px", "min(720px, calc(100vh - 48px))"],
        content: '<div class="modal-content"><p class="read-only-reason">仅显示当前在队成员，不包含联系方式、资金或邀请关系。</p>' +
          '<div class="table-wrap"><table class="admin-table"><thead><tr><th>用户 ID</th><th>昵称</th>' +
          '<th>账号状态</th><th>入队时间</th><th>身份</th></tr></thead><tbody>' +
          (memberRows || '<tr><td colspan="5" class="table-empty-cell">当前没有成员</td></tr>') +
          "</tbody></table></div></div>"
      });
      return;
    }
    if (action === "agent-create") {
      const options = agentPermissionOptions(state.cache.agentAllowedPermissions || []);
      return openForm("新建代理", [
        { name: "username", label: "登录账号" },
        { name: "display_name", label: "显示名称" },
        { name: "password", label: "初始密码（至少 12 位）", type: "password" },
        { name: "email", label: "邮箱", type: "email", wide: true },
        { name: "permission_keys", label: "代理业务权限", type: "checkboxes", options, value: [], wide: true,
          help: "管理权限会自动包含对应查看权限；团队前缀功能始终可用。" }
      ], (values) => api("/admin/api/agents", {
        method: "POST", body: {
          username: values.username, display_name: values.display_name,
          password: values.password, email: values.email,
          permission_keys: Array.isArray(values.permission_keys) ? values.permission_keys :
            (values.permission_keys ? [values.permission_keys] : [])
        }
      }));
    }
    if (action === "agent-edit") {
      const row = cached("agents", id);
      if (!row) throw new Error("代理数据已刷新，请重试");
      const options = agentPermissionOptions(state.cache.agentAllowedPermissions || []);
      return openForm("编辑代理 · " + row.username, [
        { name: "display_name", label: "显示名称", value: row.display_name },
        { name: "email", label: "邮箱", type: "email", value: row.email || "" },
        { name: "status", label: "状态", options: [[1, "启用"], [0, "停用"]], value: row.status },
        { name: "permission_keys", label: "代理业务权限", type: "checkboxes", options,
          value: row.permissions || [], wide: true, help: "停用后撤销代理端会话，但保留团队前缀归属。" }
      ], (values) => api("/admin/api/agents/" + encodeURIComponent(id), {
        method: "POST", body: {
          display_name: values.display_name, email: values.email, status: Number(values.status),
          permission_keys: Array.isArray(values.permission_keys) ? values.permission_keys :
            (values.permission_keys ? [values.permission_keys] : [])
        }
      }));
    }
    if (action === "agent-password") {
      const row = cached("agents", id);
      if (!row) throw new Error("代理数据已刷新，请重试");
      return openForm("重置代理密码 · " + row.username, [
        { name: "password", label: "新密码（12 至 128 个字符）", type: "password", wide: true },
        { name: "reason", label: "重置原因", type: "textarea", wide: true }
      ], (values) => api("/admin/api/agents/" + encodeURIComponent(id) + "/password", {
        method: "POST", body: { password: values.password, reason: values.reason }
      }));
    }
    if (action === "agent-prefixes") {
      const row = cached("agents", id);
      if (!row) throw new Error("代理数据已刷新，请重试");
      const data = await api("/admin/api/agents/" + encodeURIComponent(id) + "/team-prefixes");
      const rows = (data.items || []).map((item) => '<tr><td><strong>' + esc(item.code) +
        '</strong><br><small>' + esc(item.name) + '</small></td><td>' + formatNumber(item.member_count) +
        '</td><td>' + statusTag(item.status, { 1: ["启用", "ok"], 0: ["停用", "bad"] }) +
        '</td><td>' + (has("agents.write") ? '<button class="layui-btn layui-btn-sm layui-btn-danger" ' +
        'data-action="agent-prefix-unassign" data-id="' + esc(item.team_id) + '" data-agent-id="' +
        esc(id) + '">解除归属</button>' : "—") + '</td></tr>').join("");
      layer.open({
        type: 1, title: "团队前缀 · " + esc(row.display_name || row.username), area: ["720px", "auto"],
        content: '<div class="modal-form"><table class="data-table"><thead><tr><th>前缀</th><th>当前人数</th>' +
          '<th>团队状态</th><th>操作</th></tr></thead><tbody>' +
          (rows || '<tr><td colspan="4">暂未分配团队前缀</td></tr>') + '</tbody></table></div>'
      });
      return;
    }
    if (action === "agent-prefix-assign") {
      const row = cached("agents", id);
      if (!row) throw new Error("代理数据已刷新，请重试");
      const teams = await fetchAllRemoteItems("/admin/api/teams");
      const options = teams.filter((team) => team.code !== "sys")
        .map((team) => [team.id, team.code + " · " + team.name]);
      if (!options.length) throw new Error("当前没有可分配的非系统团队");
      return openForm("分配或转交团队前缀 · " + row.display_name, [
        { name: "team_id", label: "团队前缀", options, wide: true },
        { name: "reason", label: "分配或转交原因", type: "textarea", wide: true }
      ], (values) => api("/admin/api/agents/" + encodeURIComponent(id) + "/team-prefixes/" +
        encodeURIComponent(requireDecimalEntityID(values.team_id, "团队编号")) + "/assign", {
        method: "POST", body: { reason: values.reason }
      }));
    }
    if (action === "agent-prefix-unassign") {
      const agentID = target?.dataset.agentId || "";
      if (!agentID) throw new Error("代理编号无效");
      if (layer) layer.closeAll();
      return openForm("解除团队前缀归属", [
        { name: "reason", label: "解除原因", type: "textarea", wide: true }
      ], (values) => api("/admin/api/agents/" + encodeURIComponent(agentID) + "/team-prefixes/" +
        encodeURIComponent(id) + "/unassign", { method: "POST", body: { reason: values.reason } }));
    }
    if (action === "remote-filter") {
      const online = document.getElementById("remote-filter-online")?.value || "";
      const permission = document.getElementById("remote-filter-permission")?.value || "";
      const permissionGranted = document.getElementById("remote-filter-permission-state")?.value || "1";
      state.remoteFilters = {
        online,
        permission,
        permission_granted: permission ? permissionGranted : ""
      };
      const model = Object.values(state.tables).find((item) => item.key === "remote-devices");
      if (!model || !model.remote) return;
      model.remote.params = state.remoteFilters;
      model.page = 1;
      return loadRemoteTable(model, false);
    }
    if (action === "remote-credential") {
      const credential = await api("/admin/api/remote/devices/" + encodeURIComponent(id) + "/credential-requests", { method: "POST" });
      notify("正在等待手机确认控制授权");
      let status = credential;
      const deadline = Date.now() + 16000;
      while (status.status === "pending" && Date.now() < deadline) {
        await new Promise((resolve) => window.setTimeout(resolve, 1000));
        status = await api("/admin/api/remote/credential-requests/" + encodeURIComponent(credential.id));
      }
      if (status.status !== "ready") throw new Error("手机未在 15 秒内确认控制授权，请检查设备状态后重试");
      return openForm("验证管理员身份", [{
        name: "password", label: "请输入当前管理员密码", type: "password",
        placeholder: "密码仅用于本次身份复核", wide: true
      }], async (values) => {
        const revealed = await api("/admin/api/remote/credential-requests/" + encodeURIComponent(credential.id) + "/reveal", {
          method: "POST", body: { password: String(values.password || "") }
        });
        const device = (state.cache.remoteDevices || []).find((item) => String(item.id) === String(id)) || { id: id };
        showRemoteConsole(revealed, device);
        return { __skipRefresh: true, __formMessage: "身份验证成功" };
      });
    }
    if (action === "remote-revoke") {
      if (!window.confirm("确定停用这台设备的远程协助并撤销凭据吗？")) return;
      return mutateAndRefresh("/admin/api/remote/devices/" + encodeURIComponent(id) + "/revoke", { method: "POST" }, "已发送停用命令");
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
        { name: "name", label: "团队名称" },
        { name: "owner_user_id", label: "负责人用户 ID", value: "0",
          help: "建议保持为 0；首个转入非系统团队的用户会自动成为负责人。" }
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
          value: row.owner_user_id || "0", inputmode: "numeric",
          help: "负责人必须是当前团队的正常在队成员，并使用普通用户账号登录团队后台。" },
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
    if (action === "payment-bank-create") {
      return paymentBankAccountForm(null);
    }
    if (action === "payment-bank-edit") {
      const row = cached("paymentBankAccounts", id);
      if (!row) {
        notify("银行卡数据已刷新，请重试", true);
        return;
      }
      return paymentBankAccountForm(row);
    }
    if (action === "payment-bank-status") {
      const row = cached("paymentBankAccounts", id);
      if (!row) {
        notify("银行卡数据已刷新，请重试", true);
        return;
      }
      const status = Number(row.status) === 1 ? 0 : 1;
      if (!window.confirm(status === 1 ?
        "确认启用该收款银行卡？启用后可分配给新订单。" :
        "确认停用该收款银行卡？已分配订单不受影响。")) return;
      return mutateAndRefresh(
        "/admin/api/payments/bank-accounts/" + encodeURIComponent(id) + "/status",
        { method: "POST", body: { status } },
        status === 1 ? "收款银行卡已启用" : "收款银行卡已停用"
      );
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
    if (action === "payment-bank-assign") {
      const row = cached("paymentRecharges", id);
      if (!row) {
        notify("充值订单数据已刷新，请重试", true);
        return;
      }
      const options = (state.cache.paymentBankAccounts || [])
        .filter((account) => Number(account.status) === 1)
        .map((account) => [account.id,
          account.display_name + " · " + account.bank_name + " · " + account.card_number_masked]);
      if (!options.length) {
        notify("没有启用的收款银行卡，请先新增并启用", true);
        return;
      }
      return openForm("分配收款银行卡", [
        { name: "order_no", label: "平台订单", value: row.order_no, readonly: true, wide: true },
        { name: "bank_account_id", label: "收款银行卡", options }
      ], (values) => {
        if (!window.confirm("银行卡一经分配不能更换，确认分配给该订单？")) {
          throw new Error("操作已取消，本次未分配银行卡");
        }
        return api("/admin/api/payments/recharges/" + encodeURIComponent(id) + "/assign-bank", {
          method: "POST", body: { bank_account_id: String(values.bank_account_id || "") }
        });
      });
    }
    if (action === "payment-bank-close") {
      const row = cached("paymentRecharges", id);
      if (!row) {
        notify("充值订单数据已刷新，请重试", true);
        return;
      }
      return openForm("关闭银行卡充值订单", [
        { name: "order_no", label: "平台订单", value: row.order_no, readonly: true, wide: true },
        { name: "reason", label: "关闭原因（必填）", type: "textarea", wide: true }
      ], (values) => {
        const reason = String(values.reason || "").trim();
        if (!reason || [...reason].length > 500) throw new Error("关闭原因必填且不能超过500字");
        if (!window.confirm("订单关闭后用户必须重新下单，确认关闭？")) {
          throw new Error("操作已取消，本次未关闭订单");
        }
        return api("/admin/api/payments/recharges/" + encodeURIComponent(id) + "/close-bank", {
          method: "POST", body: { reason }
        });
      });
    }
    if (action === "payment-bank-proof-view") {
      const proof = await api(
        "/admin/api/payments/recharges/" + encodeURIComponent(id) + "/bank-proof"
      );
      const viewURL = safeHTTPURL(proof.view_url);
      if (!viewURL) {
        notify("付款凭证查看地址无效", true);
        return;
      }
      window.open(viewURL, "_blank", "noopener,noreferrer");
      return;
    }
    if (action === "payment-bank-proof-approve" || action === "payment-bank-proof-reject") {
      const row = cached("paymentRecharges", id);
      if (!row) {
        notify("充值订单数据已刷新，请重试", true);
        return;
      }
      const approve = action === "payment-bank-proof-approve";
      return openForm(approve ? "确认银行卡到账" : "驳回凭证并关闭订单", [
        { name: "order_no", label: "平台订单", value: row.order_no, readonly: true, wide: true },
        { name: "user_id", label: "用户 ID", value: row.user_id, readonly: true },
        {
          name: "reason", label: approve ? "到账核对说明（必填）" : "驳回原因（必填）",
          type: "textarea", wide: true
        }
      ], (values) => {
        const reason = String(values.reason || "").trim();
        if (!reason || [...reason].length > 500) throw new Error("审核说明必填且不能超过500字");
        const credited = Number(row.coin_amount || 0) + Number(row.bonus_coin || 0);
        const confirmation = approve ?
          "二次确认：已核实银行卡到账，将立即向用户 ID " + row.user_id +
            " 入账 " + formatNumber(credited) + " 星币。确认继续？" :
          "驳回后订单会立即关闭，用户不能重新上传凭证。确认继续？";
        if (!window.confirm(confirmation)) throw new Error("操作已取消，本次未审核");
        return api(
          "/admin/api/payments/recharges/" + encodeURIComponent(id) +
            "/bank-proof/" + (approve ? "approve" : "reject"),
          { method: "POST", body: { reason } }
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
        { name: "bet_levels", label: "下注档位（逗号分隔）", value: (row.bet_levels || []).join(",") },
        { name: "target_rtp_ppm", label: "目标 RTP（百万分比）", type: "number", value: row.target_rtp_ppm },
        { name: "status", label: "状态", options: [[1, "启用"], [0, "停用"]], value: row.status },
        { name: "sort_order", label: "排序", type: "number", value: row.sort_order }
      ], (values) => api("/admin/api/games/venues/" + id, {
        method: "POST", body: {
          name: values.name, min_balance: Number(values.min_balance),
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
      if (!row) {
        notify("设置数据已刷新，请重试", true);
        return;
      }
      const definition = systemSettingCatalog[row.key];
      return openForm((definition ? "编辑 · " + definition.title : "高级编辑 · " + row.key),
        systemSettingFormFields(row), (values) => saveSystemSettingForm(row, values));
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
    await fetch(consoleConfig.apiBase + "/logout", {
      method: "POST", credentials: "same-origin", headers: { "X-CSRF-Token": csrfToken() }
    });
    sessionStorage.removeItem(consoleConfig.csrfKey);
    window.location.replace(consoleConfig.base + "/login");
  });

  window.addEventListener("hashchange", loadRoute);
  api(consoleConfig.apiBase + "/me").then(function (me) {
    state.me = me;
    document.querySelectorAll("[data-permission]").forEach((item) => {
      item.hidden = !has(item.dataset.permission);
    });
    return loadRoute();
  }).catch(errorPanel);
})();
