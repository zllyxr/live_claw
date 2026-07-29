(function () {
  const layer = window.layui && window.layui.layer;
  const content = document.getElementById("page-content");
  const heading = document.getElementById("page-heading");
  const description = document.getElementById("page-description");
  const topTitle = document.getElementById("top-title");
  const actions = document.getElementById("page-actions");
  const state = { me: null, route: "", cache: {} };

  const pages = {
    dashboard: ["数据统计", "平台概览", "关键业务数据、资金与待处理事项"],
    users: ["用户管理", "用户与团队", "账号状态、团队归属和邀请码体系"],
    wallet: ["资金管理", "资金审核与流水", "充值、提现、调账及逐场游戏输赢"],
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

  function formatNumber(value) {
    return new Intl.NumberFormat("zh-CN").format(Number(value || 0));
  }

  function formatTime(value) {
    const numeric = Number(value || 0);
    if (!numeric) return "—";
    return new Date(numeric * 1000).toLocaleString("zh-CN", { hour12: false });
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
    if (layer) {
      layer.msg(message, { icon: error ? 2 : 1 });
    } else {
      window.alert(message);
    }
  }

  function errorPanel(error) {
    content.innerHTML = '<div class="panel-error">' + esc(error.message || "加载失败") + "</div>";
  }

  function panel(title, subtitle, body) {
    return '<section class="panel data-panel"><div class="data-panel-head"><div><h2>' +
      esc(title) + "</h2><p>" + esc(subtitle || "") +
      '</p></div></div>' + body + "</section>";
  }

  function table(columns, rows) {
    if (!rows || !rows.length) {
      return '<div class="empty-state">暂无数据</div>';
    }
    const head = columns.map((column) => "<th>" + esc(column.label) + "</th>").join("");
    const body = rows.map((row) => "<tr>" + columns.map((column) => {
      const value = typeof column.render === "function" ? column.render(row) : esc(row[column.key]);
      return '<td class="' + (column.className || "") + '">' + value + "</td>";
    }).join("") + "</tr>").join("");
    return '<div class="table-wrap"><table class="admin-table"><thead><tr>' +
      head + "</tr></thead><tbody>" + body + "</tbody></table></div>";
  }

  function button(label, action, id, style) {
    return '<button class="layui-btn layui-btn-sm ' + (style || "layui-btn-primary") +
      '" data-action="' + esc(action) + '" data-id="' + esc(id) + '">' + esc(label) + "</button>";
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

  async function dashboard() {
    const data = await api("/admin/api/dashboard");
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

  async function users() {
    const result = await Promise.all([api("/admin/api/users?page_size=100"), api("/admin/api/teams")]);
    state.cache.users = result[0].items;
    state.cache.teams = result[1].items;
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
      { label: "操作", render: (row) => has("users.write") ?
        '<div class="row-actions">' + button("状态", "user-status", row.id) +
        button("团队", "user-team", row.id) + "</div>" : "—" }
    ], result[0].items);
    const teamTable = table([
      { label: "代码", render: (row) => "<strong>" + esc(row.code) + "</strong>" },
      { label: "名称", key: "name" }, { label: "成员", render: (row) => formatNumber(row.member_count) },
      { label: "负责人", render: (row) => row.owner_user_id || "—" },
      { label: "状态", render: (row) => statusTag(row.status, { 1: ["启用", "ok"], 0: ["停用", "bad"] }) }
    ], result[1].items);
    content.innerHTML = panel("用户列表", "余额、团队和账号状态", userTable) +
      panel("团队列表", "邀请码前三位即团队代码", teamTable);
  }

  async function walletView() {
    const result = await Promise.all([
      api("/admin/api/wallet/ledger?page_size=30"),
      api("/admin/api/wallet/recharges?page_size=20"),
      api("/admin/api/wallet/withdrawals?page_size=20"),
      api("/admin/api/wallet/adjustments?page_size=20")
    ]);
    state.cache.adjustments = result[3].items;
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
    ], result[0].items);
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
    ], result[3].items);
    const recharge = table([
      { label: "订单", key: "order_no" }, { label: "用户", key: "user_id" },
      { label: "金额(分)", render: (row) => formatNumber(row.amount_minor) },
      { label: "星币", render: (row) => formatNumber(row.coin_amount) },
      { label: "状态", render: (row) => statusTag(row.status, {
        0: ["已创建", "warn"], 1: ["支付中", "warn"], 2: ["已支付", "ok"], 3: ["失败", "bad"]
      }) }
    ], result[1].items);
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
    ], result[2].items);
    content.innerHTML = panel("资金流水", "账本不可修改；游戏流水精确到场、桌和局", ledger) +
      panel("后台调账", "申请人与审核人必须为不同管理员", adjustments) +
      '<div class="subgrid">' + panel("充值订单", "", recharge) + panel("提现订单", "", withdrawal) + "</div>";
  }

  async function games() {
    const data = await api("/admin/api/games");
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

  async function liveView() {
    const data = await api("/admin/api/live/rooms?page_size=100");
    state.cache.live = data.items;
    if (has("live.write")) {
      actions.insertAdjacentHTML("afterbegin",
        '<button class="layui-btn" data-action="live-create">新增抖音房间</button>');
    }
    const roomTable = table([
      { label: "直播间", render: (row) => "<strong>" + esc(row.title) + "</strong><br><small>" + esc(row.room_no) + "</small>", className: "wrap" },
      { label: "主播", render: (row) => esc(row.nickname || row.host_name) + "<br><small>ID " + esc(row.host_user_id) + "</small>" },
      { label: "抖音房间", render: (row) => '<a href="' + esc(row.provider_page) + '" target="_blank" rel="noreferrer">' + esc(row.provider_room_id) + "</a>" },
      { label: "解析", render: (row) => statusTag(row.last_resolve_status, {
        0: ["未解析", ""], 1: ["正常", "ok"], 2: ["失败", "bad"]
      }) },
      { label: "状态", render: (row) => statusTag(row.status, {
        0: ["离线", "warn"], 1: ["在线", "ok"], 2: ["停用", "bad"]
      }) },
      { label: "操作", render: (row) => has("live.write") ? button("编辑", "live-edit", row.id) : "—" }
    ], data.items);
    content.innerHTML = panel("直播间列表", "数据库和接口双重限制 provider=douyin", roomTable);
  }

  async function lotteryView() {
    const data = await api("/admin/api/lottery/catalog");
    state.cache.lottery = data;
    state.cache.lotteryGames = data.games || [];
    state.cache.lotteryCategories = data.categories || [];
    state.cache.lotteryPlays = data.plays || [];
    state.cache.lotteryOptions = data.options || [];
    state.cache.lotteryIssues = data.issues || [];
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
      { label: "最新期号", render: (row) => row.latest_issue ?
        esc(row.latest_issue.issue_no) + "<br><small>" + formatTime(row.latest_issue.draw_at) + "</small>" : "—" },
      { label: "玩法", render: (row) => formatNumber((data.plays || []).filter((play) =>
        String(play.game_id) === String(row.id)).length) + " 个" },
      { label: "操作", render: (row) => has("lottery.write") ?
        '<div class="row-actions">' + button("玩法配置", "lottery-config", row.id) +
        button("编辑", "lottery-game-edit", row.id, "layui-btn-normal") + "</div>" :
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
    ], data.issues);
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

  async function sportsView() {
    const results = await Promise.all([
      api("/admin/api/sports/matches?page_size=100"),
      api("/admin/api/sports/sync")
    ]);
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
    ], data.items);
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

  function betOrderTable(rows, statusLabels) {
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
    ], rows);
  }

  async function betsView() {
    const data = await api("/admin/api/bets/dashboard?page_size=50");
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
      panel("彩票投注订单", "按期号记录投注和派彩", betOrderTable(data.lottery_orders, betLabels)) +
      panel("体育投注订单", "按赛事记录投注和派彩", betOrderTable(data.sports_orders, betLabels)) +
      panel("游戏逐场结算", "精确到游戏、场次、桌号与会话", betOrderTable(data.game_orders, gameLabels));
  }

  async function imView() {
    const data = await api("/admin/api/im/conversations?page_size=100");
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
    ], data.items);
    content.innerHTML = panel("会话列表", "消息记录可追溯，管理员操作全部审计", conversationTable);
  }

  async function appView() {
    const data = await api("/admin/api/app/releases?page_size=100");
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
      { label: "操作", render: (row) => has("app.write") && Number(row.status) === 0 ?
        button("发布", "app-publish", row.id) : "—" }
    ], data.items);
    content.innerHTML = panel("版本记录", "安装包保存在 MinIO，发布前校验 SHA-256", releaseTable);
  }

  async function rbacView() {
    const data = await api("/admin/api/rbac");
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
      { label: "最后登录", render: (row) => formatTime(row.last_login_at) }
    ], data.admins);
    const roleTable = table([
      { label: "角色", render: (row) => "<strong>" + esc(row.name) + "</strong><br><small>" + esc(row.role_key) + "</small>" },
      { label: "数据范围", render: (row) => ({ 1: "全部", 2: "团队", 3: "本人" }[row.data_scope] || row.data_scope) },
      { label: "权限", render: (row) => esc((row.permissions || []).join("、")), className: "wrap" },
      { label: "状态", render: (row) => statusTag(row.status, { 1: ["启用", "ok"], 0: ["停用", "bad"] }) }
    ], data.roles);
    content.innerHTML = panel("管理员", "管理员会话 12 小时过期，变更权限立即生效", adminTable) +
      panel("角色", "超级管理员角色不可降权", roleTable);
  }

  async function systemView() {
    const result = await Promise.all([
      api("/admin/api/system/settings"), api("/admin/api/system/audit?page_size=50")
    ]);
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
    ], result[1].items);
    content.innerHTML = panel("系统设置", "密钥只显示是否已配置，不回传明文", settingTable) +
      panel("审计日志", "后台重要操作不可静默执行", auditTable);
  }

  const loaders = {
    dashboard, users, wallet: walletView, games, live: liveView,
    lottery: lotteryView, sports: sportsView, bets: betsView,
    im: imView, app: appView, rbac: rbacView, system: systemView
  };

  async function loadRoute() {
    const route = (window.location.hash || "#dashboard").slice(1);
    state.route = pages[route] ? route : "dashboard";
    setHeader(state.route);
    content.innerHTML = '<div class="empty-state">正在加载…</div>';
    try {
      await loaders[state.route]();
    } catch (error) {
      errorPanel(error);
    }
  }

  function openForm(title, fields, submit) {
    if (!layer) return;
    const formID = "modal-form-" + Date.now();
    const html = '<form id="' + formID + '" class="modal-form"><div class="form-grid">' +
      fields.map((field) => {
        const options = field.options ? '<select name="' + esc(field.name) + '">' +
          field.options.map((item) => '<option value="' + esc(item[0]) + '"' +
            (String(item[0]) === String(field.value) ? " selected" : "") + ">" + esc(item[1]) + "</option>").join("") +
          "</select>" : field.type === "textarea" ?
          '<textarea name="' + esc(field.name) + '">' + esc(field.value || "") + "</textarea>" :
          field.type === "file" ?
          '<input name="' + esc(field.name) + '" type="file" accept="' + esc(field.accept || "") + '">' :
          '<input name="' + esc(field.name) + '" type="' + esc(field.type || "text") +
          '" value="' + esc(field.value || "") + '" placeholder="' + esc(field.placeholder || "") + '">';
        return '<label class="' + (field.wide ? "wide" : "") + '">' + esc(field.label) + options + "</label>";
      }).join("") + "</div></form>";
    layer.open({
      type: 1, title, area: ["620px", "auto"], content: html, btn: ["保存", "取消"],
      yes: async function (index) {
        const form = document.getElementById(formID);
        const values = {};
        new FormData(form).forEach((value, key) => {
          values[key] = value instanceof File ? value : String(value);
        });
        try {
          await submit(values);
          layer.close(index);
          notify("操作成功");
          await loadRoute();
        } catch (error) {
          notify(error.message || "操作失败", true);
        }
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
          void loadRoute().catch(errorPanel);
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

  async function handleAction(action, id) {
    if (action === "refresh") return loadRoute();
    if (action === "lottery-tab") {
      state.cache.lotteryTab = id || "games";
      return loadRoute();
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
      const options = (state.cache.teams || []).filter((team) => Number(team.status) === 1)
        .map((team) => [team.id, team.code + " · " + team.name]);
      return openForm("调整用户团队", [
        { name: "team_id", label: "目标团队", options },
        { name: "reason", label: "原因", wide: true }
      ], (values) => api("/admin/api/users/" + id + "/team", {
        method: "POST", body: { team_id: Number(values.team_id), reason: values.reason }
      }));
    }
    if (action === "team-create") {
      return openForm("新建团队", [
        { name: "code", label: "三位团队代码", placeholder: "0-9 / a-z" },
        { name: "name", label: "团队名称" }, { name: "owner_user_id", label: "负责人用户 ID", value: "0" }
      ], (values) => api("/admin/api/teams", {
        method: "POST", body: {
          code: values.code, name: values.name, owner_user_id: Number(values.owner_user_id || 0)
        }
      }));
    }
    if (action === "adjustment-create") {
      return openForm("发起调账", [
        { name: "user_id", label: "用户 ID", type: "number" },
        { name: "amount", label: "星币变动（可为负）", type: "number" },
        { name: "reason", label: "调账原因", type: "textarea", wide: true }
      ], (values) => api("/admin/api/wallet/adjustments", {
        method: "POST", body: {
          user_id: Number(values.user_id), amount: Number(values.amount),
          reason: values.reason, evidence_asset_id: 0
        }
      }));
    }
    if (action === "adjustment-approve") {
      return api("/admin/api/wallet/adjustments/" + id + "/approve", { method: "POST", body: {} })
        .then(() => { notify("调账已入账"); return loadRoute(); }).catch((error) => notify(error.message, true));
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
          category_id: Number(values.category_id), game_code: values.game_code, name: values.name,
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
          category_id: Number(values.category_id), game_code: values.game_code,
          name: values.name, issue_interval_seconds: Number(values.issue_interval_seconds),
          sale_close_seconds: Number(values.sale_close_seconds),
          min_bet: Number(values.min_bet), max_bet: Number(values.max_bet),
          sort_order: Number(values.sort_order)
        }
      }));
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
      const options = (state.cache.lottery.games || []).map((item) => [item.id, item.name]);
      const now = Math.floor(Date.now() / 1000);
      return openForm("新建彩票期号", [
        { name: "game_id", label: "彩票游戏", options },
        { name: "issue_no", label: "期号" },
        { name: "sale_open_at", label: "开售 Unix 秒", type: "number", value: now },
        { name: "sale_close_at", label: "封盘 Unix 秒", type: "number", value: now + 300 },
        { name: "draw_at", label: "开奖 Unix 秒", type: "number", value: now + 310 }
      ], (values) => api("/admin/api/lottery/issues", {
        method: "POST", body: {
          game_id: Number(values.game_id), issue_no: values.issue_no,
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
          game_id: Number(id), play_code: values.play_code,
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
          play_id: Number(id), option_code: values.option_code,
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
      return api("/admin/api/lottery/issues/" + id + "/close", { method: "POST", body: {} })
        .then(() => { notify("期号已封盘"); return loadRoute(); }).catch((error) => notify(error.message, true));
    }
    if (action === "lottery-draw") {
      return openForm("录入开奖结果", [
        { name: "winner_option_ids", label: "中奖选项 ID（逗号分隔）" },
        { name: "source", label: "结果来源", value: "manual_reviewed" }
      ], (values) => api("/admin/api/lottery/issues/" + id + "/draw", {
        method: "POST", body: {
          result: { winner_option_ids: values.winner_option_ids.split(",").map(Number).filter(Boolean) },
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
      const rows = [];
      (data.items || []).forEach((market) => {
        (market.options || []).forEach((marketOption) => rows.push({
          ...marketOption, market_name: market.name, market_code: market.market_code
        }));
      });
      state.cache.sportsOptions = rows;
      const marketTable = table([
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
      ], rows);
      const addButton = has("sports.write") ?
        '<div class="modal-toolbar"><button class="layui-btn" data-action="sports-market-create" data-id="' +
        esc(id) + '">新增盘口</button></div>' : "";
      layer.open({
        type: 1, title: "赛事盘口", area: ["900px", "620px"],
        content: '<div class="modal-content">' + addButton + marketTable + "</div>"
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
          match_id: Number(id), market_code: values.market_code, name: values.name,
          settlement_rule: values.settlement_rule, status: 1, sort_order: 0,
          options: JSON.parse(values.options)
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
      return api("/admin/api/sports/matches/" + id + "/settle", { method: "POST", body: {} })
        .then(() => { notify("赛事已提交结算队列"); return loadRoute(); })
        .catch((error) => notify(error.message, true));
    }
    if (action === "im-members") {
      const data = await api("/admin/api/im/conversations/" + encodeURIComponent(id) + "/members");
      const memberTable = table([
        { label: "用户", render: (row) => esc(row.nickname + " (" + row.user_id + ")") },
        { label: "角色", key: "role" }, { label: "状态", key: "member_status" },
        { label: "禁言至", render: (row) => formatTime(row.mute_until) }
      ], data.items);
      layer.open({ type: 1, title: "群组成员", area: ["760px", "560px"], content: memberTable });
      return;
    }
    if (action === "im-all-mute") {
      const row = cached("im", id);
      return api("/admin/api/im/conversations/" + encodeURIComponent(id), {
        method: "POST", body: { action: "all_mute", value: !row.all_muted, reason: "后台管理操作" }
      }).then(() => { notify("群组状态已更新"); return loadRoute(); }).catch((error) => notify(error.message, true));
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
              asset_id: Number(finalized.asset_id),
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
    if (action === "app-publish") {
      return api("/admin/api/app/releases/" + id + "/publish", { method: "POST", body: {} })
        .then(() => { notify("版本已发布"); return loadRoute(); }).catch((error) => notify(error.message, true));
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
    if (action === "role-create") {
      const permissions = (state.cache.rbac.permissions || []).map((item) => [item.id, item.permission_key]);
      return openForm("新建角色", [
        { name: "role_key", label: "角色标识" }, { name: "name", label: "名称" },
        { name: "description", label: "说明", wide: true },
        { name: "data_scope", label: "数据范围", options: [[1, "全部"], [2, "团队"], [3, "本人"]] },
        { name: "permission_ids", label: "权限 ID（逗号分隔）", value: permissions.map((item) => item[0]).join(","), wide: true }
      ], (values) => api("/admin/api/rbac/roles", {
        method: "POST", body: {
          role_key: values.role_key, name: values.name, description: values.description,
          data_scope: Number(values.data_scope),
          permission_ids: values.permission_ids.split(",").map(Number).filter(Boolean)
        }
      }));
    }
    if (action === "admin-create") {
      const roles = (state.cache.rbac.roles || []).filter((item) => Number(item.status) === 1);
      return openForm("新建管理员", [
        { name: "username", label: "登录账号" }, { name: "display_name", label: "显示名称" },
        { name: "password", label: "初始密码（至少 12 位）", type: "password" },
        { name: "email", label: "邮箱" },
        { name: "role_ids", label: "角色 ID（逗号分隔）", value: roles.map((item) => item.id).join(","), wide: true }
      ], (values) => api("/admin/api/rbac/admins", {
        method: "POST", body: {
          username: values.username, display_name: values.display_name, password: values.password,
          email: values.email, role_ids: values.role_ids.split(",").map(Number).filter(Boolean)
        }
      }));
    }
  }

  document.addEventListener("click", function (event) {
    const target = event.target.closest("[data-action]");
    if (!target) return;
    event.preventDefault();
    void handleAction(target.dataset.action, target.dataset.id || "");
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
