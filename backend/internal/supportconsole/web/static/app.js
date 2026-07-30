(function () {
  "use strict";

  const API_BASE = "/support-console/api";
  const CSRF_STORAGE_KEY = "claw_support_csrf";
  const state = {
    me: null,
    dashboard: {},
    scope: "queue",
    query: "",
    conversations: [],
    conversationPage: 1,
    conversationPageSize: 20,
    conversationTotal: 0,
    conversationHasMore: false,
    conversationsLoading: false,
    conversationsRequestSerial: 0,
    selectedID: "",
    selectionToken: 0,
    detailRequestSerial: 0,
    messagesRequestSerial: 0,
    olderMessagesRequestSerial: 0,
    detail: null,
    messages: [],
    messageTotal: 0,
    messagesHasMore: false,
    messagesCursor: "",
    olderMessagesLoading: false,
    agents: [],
    quickReplies: [],
    eventSource: null,
    pollTimer: null,
    refreshTimer: null,
    identityTimer: null,
    csrfMarker: "",
    initialMessagesLoaded: false
  };

  const elements = {};

  function byID(id) {
    return document.getElementById(id);
  }

  function cacheElements() {
    [
      "console-layout", "connection-pill", "connection-text", "agent-avatar", "agent-name",
      "agent-username", "logout-button", "summary-queue", "summary-mine",
      "summary-resolved", "scope-count-queue", "scope-count-mine", "conversation-search",
      "search-clear", "queue-title", "queue-subtitle", "refresh-button", "conversation-list",
      "conversation-pagination", "conversation-total", "conversation-page",
      "conversation-prev", "conversation-next",
      "chat-empty", "chat-workspace", "chat-avatar", "chat-customer-name", "chat-status-tag",
      "chat-priority-tag", "chat-conversation-meta", "claim-button", "transfer-button",
      "priority-select", "resolve-button", "assignment-banner", "message-list",
      "quick-reply-panel", "quick-reply-button", "close-quick-replies", "quick-reply-list",
      "composer-hint", "composer-form", "message-input", "send-button", "profile-empty",
      "profile-content", "profile-avatar", "profile-name", "profile-account", "profile-tags",
      "profile-user-id", "profile-created-at", "profile-conversation-id", "profile-category",
      "profile-assignee", "notes-count", "notes-list", "note-form", "note-input",
      "note-submit", "transfer-dialog", "transfer-form", "transfer-agent",
      "confirm-transfer", "resolve-dialog", "resolve-form", "confirm-resolve", "toast-region"
    ].forEach(function (id) {
      elements[id] = byID(id);
    });
  }

  function readCookie(name) {
    const prefix = name + "=";
    const item = document.cookie.split(";").map(function (part) {
      return part.trim();
    }).find(function (part) {
      return part.indexOf(prefix) === 0;
    });
    return item ? decodeURIComponent(item.slice(prefix.length)) : "";
  }

  function csrfToken() {
    return readCookie("claw_support_csrf") ||
      sessionStorage.getItem(CSRF_STORAGE_KEY) || "";
  }

  async function api(path, options) {
    const config = Object.assign({ credentials: "same-origin" }, options || {});
    config.headers = Object.assign({}, config.headers || {});
    if (config.body && typeof config.body !== "string") {
      config.headers["Content-Type"] = "application/json";
      config.body = JSON.stringify(config.body);
    }
    if (config.method && config.method !== "GET" && config.method !== "HEAD") {
      const csrf = csrfToken();
      if (csrf) {
        config.headers["X-CSRF-Token"] = csrf;
      }
    }

    const response = await fetch(API_BASE + path, config);
    if (response.status === 401) {
      sessionStorage.removeItem(CSRF_STORAGE_KEY);
      window.location.replace("/support-console/login");
      throw new Error("登录已失效");
    }
    const responseText = await response.text();
    const envelope = parseResponseJSON(responseText);
    if (!response.ok || Number(envelope.code) !== 0) {
      const error = new Error(envelope.message || "操作失败");
      error.status = response.status;
      throw error;
    }
    return envelope.data || {};
  }

  function parseResponseJSON(source) {
    if (!source) return {};
    try {
      /*
       * Go may encode uint64 identifiers as JSON numbers. Preserve identifiers
       * longer than JavaScript's safe-integer range before parsing them.
       */
      const safeSource = source.replace(
        /("(?:id|[a-z_]+_id)"\s*:\s*)(-?\d{16,})(?=\s*[,}])/gi,
        "$1\"$2\""
      );
      return JSON.parse(safeSource);
    } catch (_) {
      return {};
    }
  }

  function itemList(data) {
    if (Array.isArray(data)) return data;
    if (data && Array.isArray(data.items)) return data.items;
    if (data && Array.isArray(data.conversations)) return data.conversations;
    if (data && Array.isArray(data.messages)) return data.messages;
    if (data && Array.isArray(data.agents)) return data.agents;
    if (data && Array.isArray(data.quick_replies)) return data.quick_replies;
    return [];
  }

  function firstValue(source, keys, fallback) {
    source = source || {};
    for (let index = 0; index < keys.length; index += 1) {
      const value = source[keys[index]];
      if (value !== undefined && value !== null && value !== "") return value;
    }
    return fallback;
  }

  function numberValue(source, keys) {
    const value = Number(firstValue(source, keys, 0));
    return Number.isFinite(value) ? value : 0;
  }

  function entityID(entity) {
    return String(firstValue(entity, ["id", "conversation_id"], ""));
  }

  function currentAgentID() {
    return String(firstValue(state.me, ["id", "agent_id", "admin_user_id"], ""));
  }

  function assigneeID(conversation) {
    return String(firstValue(conversation, ["assigned_agent_id", "assigned_admin_id", "assignee_id"], "0"));
  }

  function userFromDetail() {
    const detail = state.detail || {};
    return detail.user || detail.customer || {
      id: detail.user_id,
      nickname: detail.nickname || detail.username,
      username: detail.username,
      avatar_url: detail.avatar_url
    };
  }

  function conversationFromDetail() {
    return state.detail && (state.detail.conversation || state.detail) || {};
  }

  function initials(value) {
    const text = String(value || "用户").trim();
    return text ? Array.from(text)[0].toUpperCase() : "用";
  }

  function assetURL(value) {
    const raw = String(value || "").trim();
    if (!raw) return "";
    try {
      const parsed = new URL(raw, window.location.origin);
      if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return "";
      if (parsed.pathname.indexOf("/claw-public/") === 0) {
        return parsed.pathname + parsed.search;
      }
      return parsed.href;
    } catch (_) {
      return "";
    }
  }

  function formatTime(value, includeDate) {
    let timestamp = Number(value || 0);
    if (!timestamp) return "—";
    if (timestamp < 100000000000) timestamp *= 1000;
    const date = new Date(timestamp);
    if (Number.isNaN(date.getTime())) return "—";
    const today = new Date();
    const sameDay = date.getFullYear() === today.getFullYear() &&
      date.getMonth() === today.getMonth() && date.getDate() === today.getDate();
    const options = includeDate || !sameDay
      ? { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false }
      : { hour: "2-digit", minute: "2-digit", hour12: false };
    return new Intl.DateTimeFormat("zh-CN", options).format(date).replace(/\//g, "-");
  }

  function relativeTime(value) {
    let timestamp = Number(value || 0);
    if (!timestamp) return "";
    if (timestamp < 100000000000) timestamp *= 1000;
    const seconds = Math.max(0, Math.floor((Date.now() - timestamp) / 1000));
    if (seconds < 60) return "刚刚";
    if (seconds < 3600) return Math.floor(seconds / 60) + " 分钟前";
    if (seconds < 86400) return Math.floor(seconds / 3600) + " 小时前";
    return formatTime(timestamp, true);
  }

  function statusInfo(status) {
    return {
      0: { label: "待接入", className: "is-waiting" },
      1: { label: "处理中", className: "is-handling" },
      2: { label: "已解决", className: "is-resolved" },
      3: { label: "已关闭", className: "is-closed" }
    }[Number(status)] || { label: "未知", className: "is-closed" };
  }

  function priorityInfo(priority) {
    return {
      1: { label: "普通", className: "is-normal" },
      2: { label: "高", className: "is-high" },
      3: { label: "紧急", className: "is-urgent" }
    }[Number(priority)] || { label: "普通", className: "is-normal" };
  }

  function createElement(tag, className, text) {
    const element = document.createElement(tag);
    if (className) element.className = className;
    if (text !== undefined && text !== null) element.textContent = String(text);
    return element;
  }

  function avatarElement(className, name, url) {
    const avatar = createElement("div", className);
    const source = assetURL(url);
    if (source) {
      const image = document.createElement("img");
      image.src = source;
      image.alt = "";
      image.referrerPolicy = "no-referrer";
      image.addEventListener("error", function () {
        image.remove();
        avatar.textContent = initials(name);
      }, { once: true });
      avatar.appendChild(image);
    } else {
      avatar.textContent = initials(name);
    }
    return avatar;
  }

  function replaceAvatar(element, name, url) {
    element.textContent = "";
    const source = assetURL(url);
    if (source) {
      const image = document.createElement("img");
      image.src = source;
      image.alt = "";
      image.referrerPolicy = "no-referrer";
      image.addEventListener("error", function () {
        image.remove();
        element.textContent = initials(name);
      }, { once: true });
      element.appendChild(image);
    } else {
      element.textContent = initials(name);
    }
  }

  function toast(message, type) {
    const item = createElement("div", "toast " + (type === "error" ? "is-error" : "is-success"));
    const icon = createElement("span", "toast-icon", type === "error" ? "!" : "✓");
    const copy = createElement("div", "toast-copy", message);
    item.append(icon, copy);
    elements["toast-region"].appendChild(item);
    requestAnimationFrame(function () {
      item.classList.add("is-visible");
    });
    window.setTimeout(function () {
      item.classList.remove("is-visible");
      window.setTimeout(function () {
        item.remove();
      }, 220);
    }, 3000);
  }

  function setButtonBusy(button, busy, busyText) {
    if (!button) return;
    if (busy) {
      button.dataset.originalText = button.textContent;
      button.textContent = busyText || "处理中…";
      button.disabled = true;
    } else {
      button.textContent = button.dataset.originalText || button.textContent;
      button.disabled = false;
      delete button.dataset.originalText;
    }
  }

  function restoreButtonLabel(button) {
    if (!button) return;
    button.textContent = button.dataset.originalText || button.textContent;
    delete button.dataset.originalText;
  }

  function sameConversationContext(conversationID, selectionToken) {
    return state.selectedID === conversationID &&
      state.selectionToken === selectionToken;
  }

  async function refreshAfterSuccessfulMutation(tasks, failureMessage) {
    const results = await Promise.allSettled(tasks);
    const failed = results.some(function (result) {
      return result.status === "rejected";
    });
    if (failed) {
      toast(failureMessage || "操作已成功，但页面刷新失败，请手动刷新", "error");
    }
    return !failed;
  }

  function setConnection(stateName, label) {
    elements["connection-pill"].dataset.state = stateName;
    elements["connection-text"].textContent = label;
  }

  function setMobilePanel(panel) {
    elements["console-layout"].dataset.mobilePanel = panel;
    document.querySelectorAll("[data-mobile-panel]").forEach(function (button) {
      if (button.classList.contains("mobile-panel-button")) {
        button.classList.toggle("is-active", button.dataset.mobilePanel === panel);
      }
    });
  }

  async function loadMe() {
    const data = await api("/me");
    state.me = data.agent || data.admin || data;
    const name = firstValue(state.me, ["display_name", "nickname", "name", "username"], "客服座席");
    const username = firstValue(state.me, ["username", "account"], "");
    const isSupervisor = Boolean(state.me.is_supervisor);
    const allScopeButton = document.querySelector('.scope-button[data-scope="all"]');
    if (allScopeButton) {
      allScopeButton.classList.toggle("is-hidden", !isSupervisor);
    }
    const scopeTabs = document.querySelector(".scope-tabs");
    if (scopeTabs) scopeTabs.classList.toggle("is-supervisor", isSupervisor);
    elements["agent-name"].textContent = name;
    elements["agent-username"].textContent = username ? "@" + username : "已登录";
    elements["agent-avatar"].textContent = initials(name);
  }

  async function loadDashboard() {
    try {
      state.dashboard = await api("/dashboard");
      const queue = numberValue(state.dashboard, ["queue", "waiting", "waiting_count", "queue_count"]);
      const mine = numberValue(state.dashboard, ["mine", "handling", "mine_count", "handling_count"]);
      const resolved = numberValue(state.dashboard, ["resolved_today", "today_resolved", "resolved_count"]);
      elements["summary-queue"].textContent = String(queue);
      elements["summary-mine"].textContent = String(mine);
      elements["summary-resolved"].textContent = String(resolved);
      elements["scope-count-queue"].textContent = String(queue);
      elements["scope-count-mine"].textContent = String(mine);
    } catch (error) {
      if (error.status !== 403) throw error;
    }
  }

  function scopeCopy(scope) {
    return {
      queue: ["待接入会话", "优先展示等待时间较长的用户"],
      mine: ["我的会话", "仅显示当前由我处理的会话"],
      history: ["历史会话", "查看已经解决或关闭的服务记录"],
      all: ["全部会话", "主管可查看全部座席会话"]
    }[scope] || ["客服会话", ""];
  }

  function renderConversationLoading() {
    elements["conversation-list"].replaceChildren(
      createElement("div", "list-state", "正在加载会话…")
    );
    renderConversationPagination();
  }

  function renderConversationPagination() {
    const total = Math.max(0, Number(state.conversationTotal) || 0);
    const pageCount = Math.max(1, Math.ceil(total / state.conversationPageSize));
    const currentPage = Math.min(Math.max(1, state.conversationPage), pageCount);
    elements["conversation-total"].textContent = "共 " + total + " 条";
    elements["conversation-page"].textContent = currentPage + " / " + pageCount;
    elements["conversation-prev"].disabled =
      state.conversationsLoading || currentPage <= 1;
    elements["conversation-next"].disabled =
      state.conversationsLoading || !state.conversationHasMore || currentPage >= pageCount;
    elements["conversation-pagination"].setAttribute(
      "aria-busy", String(state.conversationsLoading)
    );
  }

  function renderConversations() {
    const list = elements["conversation-list"];
    list.replaceChildren();
    renderConversationPagination();
    if (!state.conversations.length) {
      const empty = createElement("div", "queue-empty");
      const icon = createElement("span", "queue-empty-icon", state.query ? "⌕" : "✓");
      const title = createElement("strong", "", state.query ? "没有找到相关会话" : "当前没有待处理会话");
      const text = createElement("p", "", state.query ? "请尝试其他用户名称或会话编号" : "新咨询到达后会自动出现在这里");
      empty.append(icon, title, text);
      list.appendChild(empty);
      return;
    }

    state.conversations.forEach(function (conversation) {
      const id = entityID(conversation);
      const name = firstValue(conversation, ["nickname", "display_name", "username", "user_name"], "用户");
      const preview = firstValue(conversation,
        ["last_message_preview", "last_message", "text_content", "subject"], "暂无消息内容");
      const avatarURL = firstValue(conversation, ["avatar_url", "avatar"], "");
      const unread = numberValue(conversation, ["unread_count", "unread"]);
      const status = statusInfo(firstValue(conversation, ["status"], 0));
      const priority = priorityInfo(firstValue(conversation, ["priority"], 1));

      const button = createElement("button", "conversation-card");
      button.type = "button";
      button.dataset.conversationId = id;
      button.setAttribute("aria-pressed", String(id === state.selectedID));
      if (id === state.selectedID) button.classList.add("is-selected");

      const inner = createElement("div", "conversation-card-inner");
      const avatar = avatarElement("conversation-avatar", name, avatarURL);
      const body = createElement("div", "conversation-card-body");
      const top = createElement("div", "conversation-card-top");
      const nameElement = createElement("strong", "conversation-name", name);
      const time = createElement("time", "conversation-time",
        relativeTime(firstValue(conversation, ["last_message_at", "updated_at", "created_at"], 0)));
      top.append(nameElement, time);

      const tags = createElement("div", "conversation-tags");
      tags.append(
        createElement("span", "status-tag " + status.className, status.label),
        createElement("span", "priority-tag " + priority.className, priority.label)
      );
      if (unread > 0) {
        tags.appendChild(createElement("span", "unread-badge", unread > 99 ? "99+" : String(unread)));
      }
      const previewElement = createElement("p", "conversation-preview", preview);
      body.append(top, tags, previewElement);
      inner.append(avatar, body);
      button.appendChild(inner);
      list.appendChild(button);
    });
  }

  async function loadConversations(options) {
    options = options || {};
    const requestSerial = state.conversationsRequestSerial + 1;
    state.conversationsRequestSerial = requestSerial;
    state.conversationsLoading = true;
    const requestedScope = state.scope;
    const requestedQuery = state.query;
    const requestedPage = state.conversationPage;
    if (!options.silent) renderConversationLoading();
    else renderConversationPagination();
    const params = new URLSearchParams({
      scope: state.scope,
      page: String(state.conversationPage),
      page_size: String(state.conversationPageSize)
    });
    if (state.query) params.set("q", state.query);
    try {
      const data = await api("/conversations?" + params.toString());
      if (requestSerial !== state.conversationsRequestSerial ||
          requestedScope !== state.scope || requestedQuery !== state.query ||
          requestedPage !== state.conversationPage) {
        return false;
      }
      const total = Math.max(0, Number(data.total) || 0);
      const lastPage = Math.max(1, Math.ceil(total / state.conversationPageSize));
      if (requestedPage > lastPage) {
        state.conversationTotal = total;
        state.conversationPage = lastPage;
        state.conversationsLoading = false;
        return loadConversations(options);
      }
      state.conversations = itemList(data);
      state.conversationTotal = total;
      state.conversationHasMore = Boolean(data.has_more);
      state.conversationsLoading = false;
      renderConversations();
      return true;
    } catch (error) {
      if (requestSerial !== state.conversationsRequestSerial) return false;
      state.conversationsLoading = false;
      renderConversationPagination();
      elements["conversation-list"].replaceChildren(
        createElement("div", "list-state is-error", error.message || "会话加载失败")
      );
      if (!options.silent) toast(error.message || "会话加载失败", "error");
      if (options.throwOnError) throw error;
      return false;
    }
  }

  async function changeConversationPage(nextPage) {
    const totalPages = Math.max(
      1, Math.ceil(state.conversationTotal / state.conversationPageSize)
    );
    const target = Math.min(Math.max(1, Number(nextPage) || 1), totalPages);
    if (target === state.conversationPage || state.conversationsLoading) return;
    const previousPage = state.conversationPage;
    state.conversationPage = target;
    const loaded = await loadConversations();
    if (!loaded) {
      state.conversationPage = previousPage;
      renderConversationPagination();
    }
  }

  async function selectConversation(id) {
    if (!id) return;
    state.selectedID = String(id);
    state.selectionToken += 1;
    const selected = state.selectedID;
    const selectionToken = state.selectionToken;
    state.detail = null;
    state.initialMessagesLoaded = false;
    state.messageTotal = 0;
    state.messagesHasMore = false;
    state.messagesCursor = "";
    state.olderMessagesLoading = false;
    state.messages = [];
    renderConversations();
    elements["chat-empty"].classList.add("is-hidden");
    elements["chat-workspace"].classList.remove("is-hidden");
    elements["profile-empty"].classList.add("is-hidden");
    elements["profile-content"].classList.remove("is-hidden");
    elements["message-list"].replaceChildren(createElement("div", "list-state", "正在加载消息…"));
    setMobilePanel("chat");
    try {
      await Promise.all([loadConversationDetail(), loadMessages()]);
    } catch (error) {
      if (sameConversationContext(selected, selectionToken)) {
        toast(error.message || "会话加载失败", "error");
      }
    }
  }

  function clearSelection() {
    state.selectedID = "";
    state.selectionToken += 1;
    state.detail = null;
    state.messages = [];
    state.messageTotal = 0;
    state.messagesHasMore = false;
    state.messagesCursor = "";
    state.olderMessagesLoading = false;
    elements["chat-empty"].classList.remove("is-hidden");
    elements["chat-workspace"].classList.add("is-hidden");
    elements["profile-empty"].classList.remove("is-hidden");
    elements["profile-content"].classList.add("is-hidden");
    renderConversations();
  }

  async function loadConversationDetail() {
    if (!state.selectedID) return;
    const selected = state.selectedID;
    const selectionToken = state.selectionToken;
    const requestSerial = state.detailRequestSerial + 1;
    state.detailRequestSerial = requestSerial;
    let data;
    try {
      data = await api("/conversations/" + encodeURIComponent(selected));
    } catch (error) {
      if (!sameConversationContext(selected, selectionToken) ||
          requestSerial !== state.detailRequestSerial) {
        return false;
      }
      throw error;
    }
    if (!sameConversationContext(selected, selectionToken) ||
        requestSerial !== state.detailRequestSerial) {
      return false;
    }
    state.detail = data;
    renderConversationDetail();
    return true;
  }

  function renderConversationDetail() {
    const conversation = conversationFromDetail();
    const user = userFromDetail();
    const name = firstValue(user, ["nickname", "display_name", "username", "name"], "用户");
    const avatarURL = firstValue(user, ["avatar_url", "avatar"], "");
    const status = statusInfo(firstValue(conversation, ["status"], 0));
    const priority = priorityInfo(firstValue(conversation, ["priority"], 1));
    const assigned = assigneeID(conversation);
    const me = currentAgentID();
    const statusNumber = Number(firstValue(conversation, ["status"], 0));
    const isOpen = statusNumber < 2;
    const unassigned = !assigned || assigned === "0";
    const mine = assigned === me && me !== "";
    const supervisor = Boolean(state.me && state.me.is_supervisor);

    replaceAvatar(elements["chat-avatar"], name, avatarURL);
    elements["chat-customer-name"].textContent = name;
    elements["chat-status-tag"].textContent = status.label;
    elements["chat-status-tag"].className = "status-tag " + status.className;
    elements["chat-priority-tag"].textContent = priority.label;
    elements["chat-priority-tag"].className = "priority-tag " + priority.className;
    elements["chat-conversation-meta"].textContent =
      "会话 " + (entityID(conversation) || state.selectedID) + " · " +
      firstValue(conversation, ["subject"], "在线客服");

    elements["claim-button"].classList.toggle("is-hidden", !isOpen || !unassigned);
    elements["claim-button"].disabled = !isOpen || !unassigned;
    elements["transfer-button"].disabled = !isOpen || (!mine && !supervisor);
    elements["resolve-button"].disabled = !isOpen || (!mine && !supervisor);
    elements["priority-select"].value = String(firstValue(conversation, ["priority"], 1));
    elements["priority-select"].disabled = !isOpen || (!mine && !unassigned);

    let assignmentText = "";
    let assignmentClass = "";
    if (!isOpen) {
      assignmentText = "该会话已经" + status.label + "，仅可查看历史消息。";
      assignmentClass = "is-neutral";
    } else if (unassigned) {
      assignmentText = "该会话尚未分配，请先接入后再回复用户。";
      assignmentClass = "is-waiting";
    } else if (mine) {
      assignmentText = "当前会话由你负责，请及时回复用户。";
      assignmentClass = "is-mine";
    } else {
      assignmentText = "当前会话由 " +
        firstValue(conversation, ["assignee_name", "assigned_agent_name"], "其他座席") +
        " 负责，仅可查看。";
      assignmentClass = "is-neutral";
    }
    elements["assignment-banner"].textContent = assignmentText;
    elements["assignment-banner"].className = "assignment-banner " + assignmentClass;

    const canReply = isOpen && mine;
    elements["message-input"].disabled = !canReply;
    elements["send-button"].disabled = !canReply;
    elements["quick-reply-button"].disabled = !canReply;
    elements["message-input"].placeholder = canReply
      ? "请输入回复内容"
      : (unassigned ? "请先接入会话" : "当前会话不可回复");
    elements["composer-hint"].textContent = canReply
      ? "Enter 发送，Shift + Enter 换行"
      : "接入会话后可向用户回复";

    renderProfile(conversation, user);
  }

  async function loadMessages(options) {
    options = options || {};
    if (!state.selectedID) return;
    const selected = state.selectedID;
    const selectionToken = state.selectionToken;
    const requestSerial = state.messagesRequestSerial + 1;
    state.messagesRequestSerial = requestSerial;
    const params = new URLSearchParams({ limit: "60" });
    let data;
    try {
      data = await api(
        "/conversations/" + encodeURIComponent(selected) + "/messages?" + params.toString()
      );
    } catch (error) {
      if (!sameConversationContext(selected, selectionToken) ||
          requestSerial !== state.messagesRequestSerial) {
        return false;
      }
      throw error;
    }
    if (!sameConversationContext(selected, selectionToken) ||
        requestSerial !== state.messagesRequestSerial) {
      return false;
    }
    const nextMessages = itemList(data);
    const previousLastID = state.messages.length ? entityID(state.messages[state.messages.length - 1]) : "";
    const nextLastID = nextMessages.length ? entityID(nextMessages[nextMessages.length - 1]) : "";
    state.messages = mergeMessages(state.messages, nextMessages);
    state.messageTotal = Math.max(
      state.messages.length, Number(data.total) || state.messages.length
    );
    state.messagesCursor = state.messages.length ? entityID(state.messages[0]) : "";
    state.messagesHasMore = state.messageTotal > state.messages.length ||
      (state.messages.length === nextMessages.length && Boolean(data.has_more));
    renderMessages();
    if (!state.initialMessagesLoaded || (nextLastID && nextLastID !== previousLastID && !options.silent)) {
      scrollMessagesToBottom();
    }
    state.initialMessagesLoaded = true;
    return true;
  }

  function compareMessageIDs(left, right) {
    const leftID = entityID(left).replace(/^0+/, "") || "0";
    const rightID = entityID(right).replace(/^0+/, "") || "0";
    if (/^\d+$/.test(leftID) && /^\d+$/.test(rightID)) {
      if (leftID.length !== rightID.length) return leftID.length - rightID.length;
      if (leftID !== rightID) return leftID < rightID ? -1 : 1;
    }
    const leftTime = Number(firstValue(left, ["created_at", "sent_at"], 0));
    const rightTime = Number(firstValue(right, ["created_at", "sent_at"], 0));
    if (leftTime !== rightTime) return leftTime - rightTime;
    return entityID(left).localeCompare(entityID(right));
  }

  function mergeMessages(existing, incoming) {
    const byID = new Map();
    existing.concat(incoming).forEach(function (message, index) {
      const id = entityID(message);
      const key = id || "missing-" + index;
      byID.set(key, message);
    });
    return Array.from(byID.values()).sort(compareMessageIDs);
  }

  async function loadOlderMessages() {
    if (!state.selectedID || !state.messagesHasMore ||
        !state.messagesCursor || state.olderMessagesLoading) return;
    const selected = state.selectedID;
    const selectionToken = state.selectionToken;
    const requestSerial = state.olderMessagesRequestSerial + 1;
    state.olderMessagesRequestSerial = requestSerial;
    const beforeID = state.messagesCursor;
    const list = elements["message-list"];
    state.olderMessagesLoading = true;
    renderMessages();
    const previousHeight = list.scrollHeight;
    const previousTop = list.scrollTop;
    try {
      const params = new URLSearchParams({
        before_id: beforeID,
        limit: "60"
      });
      const data = await api(
        "/conversations/" + encodeURIComponent(selected) + "/messages?" + params.toString()
      );
      if (!sameConversationContext(selected, selectionToken) ||
          requestSerial !== state.olderMessagesRequestSerial) {
        return;
      }
      const olderMessages = itemList(data);
      state.messages = mergeMessages(state.messages, olderMessages);
      state.messageTotal = Math.max(
        state.messages.length, Number(data.total) || state.messageTotal
      );
      state.messagesCursor = state.messages.length ? entityID(state.messages[0]) : "";
      const madeProgress =
        state.messagesCursor !== beforeID && olderMessages.length > 0;
      state.messagesHasMore = madeProgress &&
        (Boolean(data.has_more) || state.messageTotal > state.messages.length);
      state.olderMessagesLoading = false;
      renderMessages();
      requestAnimationFrame(function () {
        if (!sameConversationContext(selected, selectionToken) ||
            requestSerial !== state.olderMessagesRequestSerial) {
          return;
        }
        const behavior = list.style.scrollBehavior;
        list.style.scrollBehavior = "auto";
        list.scrollTop = Math.max(0, list.scrollHeight - previousHeight + previousTop);
        list.style.scrollBehavior = behavior;
      });
    } catch (error) {
      if (!sameConversationContext(selected, selectionToken) ||
          requestSerial !== state.olderMessagesRequestSerial) {
        return;
      }
      state.olderMessagesLoading = false;
      renderMessages();
      toast(error.message || "更早消息加载失败", "error");
    }
  }

  function renderMessages() {
    const list = elements["message-list"];
    list.replaceChildren();
    if (!state.messages.length) {
      const empty = createElement("div", "message-empty");
      empty.append(
        createElement("span", "message-empty-icon", "对话"),
        createElement("strong", "", "还没有消息"),
        createElement("p", "", "接入会话后向用户发送第一条回复")
      );
      list.appendChild(empty);
      return;
    }

    if (state.messagesHasMore) {
      const historyControl = createElement("div", "message-history-control");
      const loadedCount = createElement(
        "span", "message-history-count",
        "已加载 " + state.messages.length + " / " + state.messageTotal + " 条"
      );
      const loadOlderButton = createElement(
        "button", "secondary-button compact-button message-history-button",
        state.olderMessagesLoading ? "加载中…" : "加载更早消息"
      );
      loadOlderButton.type = "button";
      loadOlderButton.dataset.loadOlderMessages = "true";
      loadOlderButton.disabled = state.olderMessagesLoading;
      historyControl.append(loadedCount, loadOlderButton);
      list.appendChild(historyControl);
    }

    let previousDate = "";
    state.messages.forEach(function (message) {
      let createdAt = Number(firstValue(message, ["created_at", "sent_at"], 0));
      if (createdAt && createdAt < 100000000000) createdAt *= 1000;
      const dateKey = createdAt ? new Date(createdAt).toDateString() : "";
      if (dateKey && dateKey !== previousDate) {
        const separator = createElement("div", "date-separator");
        separator.appendChild(createElement("span", "", formatTime(createdAt, true)));
        list.appendChild(separator);
        previousDate = dateKey;
      }

      const senderType = Number(firstValue(message, ["sender_type"], 1));
      const isAgent = senderType === 2;
      const isSystem = senderType === 3;
      if (isSystem) {
        const system = createElement("div", "system-message");
        system.appendChild(createElement("span", "",
          firstValue(message, ["text_content", "content"], "系统消息")));
        list.appendChild(system);
        return;
      }

      const row = createElement("div", "message-row " + (isAgent ? "is-agent" : "is-user"));
      const senderName = firstValue(message, ["sender_name"], isAgent ? "客服" : "用户");
      const avatar = avatarElement("message-avatar", senderName,
        firstValue(message, ["sender_avatar_url", "avatar_url"], ""));
      const body = createElement("div", "message-body");
      const meta = createElement("div", "message-meta");
      meta.append(
        createElement("strong", "", senderName),
        createElement("time", "", formatTime(createdAt, false))
      );
      const bubble = createElement("div", "message-bubble");
      const type = Number(firstValue(message, ["message_type"], 1));
      const content = firstValue(message, ["text_content", "content"], "");
      const attachmentURL = assetURL(firstValue(message, ["asset_url", "url"], ""));
      if (type === 2 && attachmentURL) {
        const image = document.createElement("img");
        image.className = "message-image";
        image.src = attachmentURL;
        image.alt = content || "用户发送的图片";
        image.loading = "lazy";
        image.referrerPolicy = "no-referrer";
        bubble.appendChild(image);
        if (content) bubble.appendChild(createElement("p", "", content));
      } else if (type === 3 && attachmentURL) {
        const link = createElement("a", "attachment-link", content || "查看附件");
        link.href = attachmentURL;
        link.target = "_blank";
        link.rel = "noopener noreferrer";
        bubble.appendChild(link);
      } else {
        bubble.appendChild(createElement("p", "", content || "附件消息"));
      }
      body.append(meta, bubble);
      if (isAgent) {
        row.append(body, avatar);
      } else {
        row.append(avatar, body);
      }
      list.appendChild(row);
    });
  }

  function scrollMessagesToBottom() {
    requestAnimationFrame(function () {
      elements["message-list"].scrollTop = elements["message-list"].scrollHeight;
    });
  }

  function renderProfile(conversation, user) {
    const name = firstValue(user, ["nickname", "display_name", "username", "name"], "用户");
    const username = firstValue(user, ["username", "account"], "");
    const userID = String(firstValue(user, ["id", "user_id"], firstValue(conversation, ["user_id"], "—")));
    const avatarURL = firstValue(user, ["avatar_url", "avatar"], "");
    replaceAvatar(elements["profile-avatar"], name, avatarURL);
    elements["profile-name"].textContent = name;
    elements["profile-account"].textContent = username ? "@" + username : "用户 ID " + userID;
    elements["profile-user-id"].textContent = userID;
    elements["profile-created-at"].textContent = formatTime(
      firstValue(user, ["created_at", "registered_at"], 0), true);
    elements["profile-conversation-id"].textContent = entityID(conversation) || state.selectedID;
    elements["profile-category"].textContent =
      firstValue(conversation, ["category", "subject"], "一般咨询");
    elements["profile-assignee"].textContent =
      firstValue(conversation, ["assignee_name", "assigned_agent_name"], "未分配");

    elements["profile-tags"].replaceChildren();
    const status = statusInfo(firstValue(conversation, ["status"], 0));
    const priority = priorityInfo(firstValue(conversation, ["priority"], 1));
    elements["profile-tags"].append(
      createElement("span", "status-tag " + status.className, status.label),
      createElement("span", "priority-tag " + priority.className, priority.label)
    );
    if (Number(firstValue(user, ["is_virtual"], 0)) === 1) {
      elements["profile-tags"].appendChild(createElement("span", "profile-tag", "虚拟用户"));
    }

    const detail = state.detail || {};
    const notes = Array.isArray(detail.notes) ? detail.notes :
      (detail.notes && Array.isArray(detail.notes.items) ? detail.notes.items : []);
    renderNotes(notes);
  }

  function renderNotes(notes) {
    elements["notes-count"].textContent = String(notes.length);
    const list = elements["notes-list"];
    list.replaceChildren();
    if (!notes.length) {
      list.appendChild(createElement("div", "list-state compact-state", "暂无服务备注"));
      return;
    }
    notes.forEach(function (note) {
      const item = createElement("article", "note-item");
      const content = createElement("p", "", firstValue(note, ["content", "text"], ""));
      const meta = createElement("div", "note-meta");
      meta.append(
        createElement("strong", "", firstValue(note, ["agent_name", "created_by_name"], "客服")),
        createElement("time", "", formatTime(firstValue(note, ["created_at"], 0), true))
      );
      item.append(content, meta);
      list.appendChild(item);
    });
  }

  async function loadAgents() {
    try {
      state.agents = await loadAllPagedItems("/agents");
      renderAgents();
    } catch (error) {
      if (error.status !== 403) throw error;
    }
  }

  function renderAgents() {
    const select = elements["transfer-agent"];
    select.replaceChildren();
    const placeholder = createElement("option", "", "请选择在线座席");
    placeholder.value = "";
    select.appendChild(placeholder);
    const me = currentAgentID();
    state.agents.forEach(function (agent) {
      const id = String(firstValue(agent, ["id", "agent_id", "admin_user_id"], ""));
      if (!id || id === me || Number(firstValue(agent, ["status"], 1)) === 0) return;
      const name = firstValue(agent, ["display_name", "nickname", "name", "username"], "客服");
      const active = numberValue(agent, ["active_conversations", "handling_count"]);
      const option = createElement("option", "", name + (active ? " · 处理中 " + active : ""));
      option.value = id;
      select.appendChild(option);
    });
  }

  async function loadQuickReplies() {
    try {
      state.quickReplies = await loadAllPagedItems("/quick-replies");
      renderQuickReplies();
    } catch (error) {
      if (error.status !== 403) throw error;
    }
  }

  async function loadAllPagedItems(path) {
    const allItems = [];
    const seen = new Set();
    let page = 1;
    while (page <= 1000) {
      const params = new URLSearchParams({
        page: String(page),
        page_size: "100"
      });
      const data = await api(path + "?" + params.toString());
      const pageItems = itemList(data);
      pageItems.forEach(function (item, index) {
        const id = entityID(item);
        const key = id || page + ":" + index;
        if (seen.has(key)) return;
        seen.add(key);
        allItems.push(item);
      });
      const total = Math.max(0, Number(data.total) || 0);
      if (!Boolean(data.has_more) || !pageItems.length ||
          (total > 0 && allItems.length >= total)) {
        break;
      }
      page += 1;
    }
    return allItems;
  }

  function renderQuickReplies() {
    const list = elements["quick-reply-list"];
    list.replaceChildren();
    if (!state.quickReplies.length) {
      list.appendChild(createElement("div", "list-state compact-state", "暂无快捷回复"));
      return;
    }
    state.quickReplies.forEach(function (reply) {
      const content = firstValue(reply, ["content", "text_content", "text"], "");
      if (!content) return;
      const button = createElement("button", "quick-reply-item");
      button.type = "button";
      button.dataset.quickReply = content;
      const inner = createElement("div", "quick-reply-item-inner");
      inner.append(
        createElement("strong", "", firstValue(reply, ["title", "name"], "快捷回复")),
        createElement("p", "", content)
      );
      button.appendChild(inner);
      list.appendChild(button);
    });
  }

  async function claimConversation() {
    if (!state.selectedID) return;
    const conversationID = state.selectedID;
    const selectionToken = state.selectionToken;
    setButtonBusy(elements["claim-button"], true, "接入中…");
    try {
      await api("/conversations/" + encodeURIComponent(conversationID) + "/claim", {
        method: "POST",
        body: {}
      });
    } catch (error) {
      toast(error.message || "接入会话失败", "error");
      if (sameConversationContext(conversationID, selectionToken)) {
        setButtonBusy(elements["claim-button"], false);
      } else {
        restoreButtonLabel(elements["claim-button"]);
      }
      return;
    }

    toast("会话已接入");
    if (sameConversationContext(conversationID, selectionToken)) {
      state.scope = "mine";
      state.conversationPage = 1;
      updateScopeUI();
    }
    const refreshTasks = [
      loadDashboard(),
      loadConversations({ silent: true, throwOnError: true })
    ];
    if (sameConversationContext(conversationID, selectionToken)) {
      refreshTasks.push(loadConversationDetail());
    }
    await refreshAfterSuccessfulMutation(
      refreshTasks,
      "会话已接入，但页面刷新失败，请手动刷新"
    );
    restoreButtonLabel(elements["claim-button"]);
    if (sameConversationContext(conversationID, selectionToken)) {
      elements["message-input"].focus();
    }
  }

  async function sendMessage() {
    const submittedDraft = elements["message-input"].value;
    const text = submittedDraft.trim();
    if (!text || !state.selectedID || elements["send-button"].disabled) return;
    const conversationID = state.selectedID;
    const selectionToken = state.selectionToken;
    setButtonBusy(elements["send-button"], true, "发送中…");
    try {
      const clientID = "support_console_" + Date.now() + "_" +
        (window.crypto && window.crypto.randomUUID
          ? window.crypto.randomUUID()
          : Math.random().toString(36).slice(2));
      await api("/conversations/" + encodeURIComponent(conversationID) + "/messages", {
        method: "POST",
        body: {
          client_message_id: clientID,
          message_type: 1,
          text_content: text,
          asset_id: 0
        }
      });
    } catch (error) {
      toast(error.message || "消息发送失败", "error");
      if (sameConversationContext(conversationID, selectionToken)) {
        setButtonBusy(elements["send-button"], false);
      } else {
        restoreButtonLabel(elements["send-button"]);
      }
      return;
    }

    const sameContext = sameConversationContext(conversationID, selectionToken);
    if (sameContext) {
      if (elements["message-input"].value === submittedDraft) {
        elements["message-input"].value = "";
      }
      elements["quick-reply-panel"].classList.add("is-hidden");
    }
    const refreshTasks = [
      loadConversations({ silent: true, throwOnError: true })
    ];
    if (sameContext) refreshTasks.push(loadMessages());
    await refreshAfterSuccessfulMutation(
      refreshTasks,
      "消息已发送，但页面刷新失败，请手动刷新"
    );
    if (sameConversationContext(conversationID, selectionToken)) {
      restoreButtonLabel(elements["send-button"]);
      if (state.detail) {
        renderConversationDetail();
      } else {
        elements["send-button"].disabled = false;
      }
      if (!elements["message-input"].disabled) {
        elements["message-input"].focus();
      }
    } else {
      restoreButtonLabel(elements["send-button"]);
    }
  }

  async function changePriority() {
    if (!state.selectedID) return;
    const conversationID = state.selectedID;
    const selectionToken = state.selectionToken;
    const priority = Number(elements["priority-select"].value);
    elements["priority-select"].disabled = true;
    try {
      await api("/conversations/" + encodeURIComponent(conversationID) + "/priority", {
        method: "POST",
        body: { priority: priority }
      });
    } catch (error) {
      toast(error.message || "优先级更新失败", "error");
      if (sameConversationContext(conversationID, selectionToken) && state.detail) {
        renderConversationDetail();
      }
      return;
    }

    toast("会话优先级已更新");
    const refreshTasks = [
      loadConversations({ silent: true, throwOnError: true })
    ];
    if (sameConversationContext(conversationID, selectionToken)) {
      refreshTasks.push(loadConversationDetail());
    }
    await refreshAfterSuccessfulMutation(
      refreshTasks,
      "优先级已更新，但页面刷新失败，请手动刷新"
    );
  }

  async function transferConversation(targetAgentID) {
    if (!state.selectedID || !targetAgentID) return;
    const conversationID = state.selectedID;
    const selectionToken = state.selectionToken;
    const exactTargetAgentID = String(targetAgentID);
    setButtonBusy(elements["confirm-transfer"], true, "转接中…");
    try {
      await api("/conversations/" + encodeURIComponent(conversationID) + "/transfer", {
        method: "POST",
        body: { target_agent_id: exactTargetAgentID }
      });
    } catch (error) {
      toast(error.message || "会话转接失败", "error");
      setButtonBusy(elements["confirm-transfer"], false);
      return;
    }

    if (elements["transfer-dialog"].open) elements["transfer-dialog"].close();
    toast("会话已转接");
    const transferredCurrentConversation =
      sameConversationContext(conversationID, selectionToken);
    if (transferredCurrentConversation) {
      clearSelection();
      setMobilePanel("queue");
    }
    await refreshAfterSuccessfulMutation(
      [
        loadDashboard(),
        loadConversations({ silent: true, throwOnError: true })
      ],
      "会话已转接，但页面刷新失败，请手动刷新"
    );
    setButtonBusy(elements["confirm-transfer"], false);
  }

  async function resolveConversation() {
    if (!state.selectedID) return;
    const conversationID = state.selectedID;
    const selectionToken = state.selectionToken;
    setButtonBusy(elements["confirm-resolve"], true, "处理中…");
    try {
      await api("/conversations/" + encodeURIComponent(conversationID) + "/resolve", {
        method: "POST",
        body: {}
      });
    } catch (error) {
      toast(error.message || "解决会话失败", "error");
      setButtonBusy(elements["confirm-resolve"], false);
      return;
    }

    if (elements["resolve-dialog"].open) elements["resolve-dialog"].close();
    toast("会话已解决");
    const resolvedCurrentConversation =
      sameConversationContext(conversationID, selectionToken);
    if (resolvedCurrentConversation) {
      clearSelection();
      setMobilePanel("queue");
    }
    await refreshAfterSuccessfulMutation(
      [
        loadDashboard(),
        loadConversations({ silent: true, throwOnError: true })
      ],
      "会话已解决，但页面刷新失败，请手动刷新"
    );
    setButtonBusy(elements["confirm-resolve"], false);
  }

  async function addNote() {
    const content = elements["note-input"].value.trim();
    const user = userFromDetail();
    const userID = String(firstValue(user, ["id", "user_id"],
      firstValue(conversationFromDetail(), ["user_id"], "")));
    if (!content || !userID) return;
    setButtonBusy(elements["note-submit"], true, "添加中…");
    try {
      await api("/users/" + encodeURIComponent(userID) + "/notes", {
        method: "POST",
        body: { content: content }
      });
      elements["note-input"].value = "";
      toast("服务备注已添加");
      await loadConversationDetail();
    } catch (error) {
      toast(error.message || "备注添加失败", "error");
    } finally {
      setButtonBusy(elements["note-submit"], false);
    }
  }

  function updateScopeUI() {
    document.querySelectorAll(".scope-button").forEach(function (button) {
      const active = button.dataset.scope === state.scope;
      button.classList.toggle("is-active", active);
      button.setAttribute("aria-pressed", String(active));
    });
    const copy = scopeCopy(state.scope);
    elements["queue-title"].textContent = copy[0];
    elements["queue-subtitle"].textContent = copy[1];
  }

  function scheduleLiveRefresh(event) {
    window.clearTimeout(state.refreshTimer);
    state.refreshTimer = window.setTimeout(async function () {
      const payload = event && event.data ? safeJSON(event.data) : {};
      const eventConversationID = String(firstValue(payload, ["conversation_id", "id"], ""));
      const work = [loadDashboard(), loadConversations({ silent: true })];
      if (state.selectedID && (!eventConversationID || eventConversationID === state.selectedID)) {
        work.push(loadMessages({ silent: true }), loadConversationDetail());
      }
      await Promise.allSettled(work);
    }, 180);
  }

  function safeJSON(value) {
    try {
      return JSON.parse(value);
    } catch (_) {
      return {};
    }
  }

  function connectEvents() {
    if (!window.EventSource) {
      setConnection("polling", "定时刷新");
      startPolling();
      return;
    }
    if (state.eventSource) state.eventSource.close();
    setConnection("connecting", "正在连接");
    const source = new EventSource(API_BASE + "/events", { withCredentials: true });
    state.eventSource = source;
    source.onopen = function () {
      setConnection("online", "实时在线");
    };
    source.onmessage = scheduleLiveRefresh;
    [
      "support", "support.message", "support.conversation", "message.created",
      "conversation.created", "conversation.updated", "dashboard.updated"
    ].forEach(function (eventName) {
      source.addEventListener(eventName, scheduleLiveRefresh);
    });
    source.onerror = function () {
      setConnection("connecting", "正在重连");
    };
    startPolling();
  }

  function startPolling() {
    window.clearInterval(state.pollTimer);
    state.pollTimer = window.setInterval(function () {
      const work = [loadDashboard(), loadConversations({ silent: true })];
      if (state.selectedID) work.push(loadMessages({ silent: true }));
      Promise.allSettled(work);
    }, 15000);
  }

  function watchAccountChanges() {
    state.csrfMarker = readCookie("claw_support_csrf");
    window.clearInterval(state.identityTimer);
    state.identityTimer = window.setInterval(function () {
      const currentMarker = readCookie("claw_support_csrf");
      if (currentMarker !== state.csrfMarker) {
        window.location.reload();
      }
    }, 1000);
  }

  async function logout() {
    setButtonBusy(elements["logout-button"], true, "退出中…");
    try {
      await api("/logout", { method: "POST", body: {} });
    } catch (_) {
      // Clear local state even when the server-side session already expired.
    } finally {
      sessionStorage.removeItem(CSRF_STORAGE_KEY);
      if (state.eventSource) state.eventSource.close();
      window.location.replace("/support-console/login");
    }
  }

  function bindEvents() {
    document.addEventListener("click", function (event) {
      const panelButton = event.target.closest("[data-mobile-panel]");
      if (panelButton) setMobilePanel(panelButton.dataset.mobilePanel);

      const scopeButton = event.target.closest(".scope-button");
      if (scopeButton) {
        state.scope = scopeButton.dataset.scope;
        state.conversationPage = 1;
        updateScopeUI();
        loadConversations();
      }

      const conversationButton = event.target.closest("[data-conversation-id]");
      if (conversationButton) selectConversation(conversationButton.dataset.conversationId);

      const olderMessagesButton = event.target.closest("[data-load-older-messages]");
      if (olderMessagesButton) loadOlderMessages();

      const replyButton = event.target.closest("[data-quick-reply]");
      if (replyButton) {
        elements["message-input"].value = replyButton.dataset.quickReply;
        elements["quick-reply-panel"].classList.add("is-hidden");
        elements["message-input"].focus();
      }
    });

    let searchTimer;
    elements["conversation-search"].addEventListener("input", function () {
      state.query = elements["conversation-search"].value.trim();
      state.conversationPage = 1;
      elements["search-clear"].classList.toggle("is-visible", Boolean(state.query));
      window.clearTimeout(searchTimer);
      searchTimer = window.setTimeout(function () {
        loadConversations();
      }, 320);
    });
    elements["search-clear"].addEventListener("click", function () {
      elements["conversation-search"].value = "";
      state.query = "";
      state.conversationPage = 1;
      elements["search-clear"].classList.remove("is-visible");
      loadConversations();
      elements["conversation-search"].focus();
    });
    elements["conversation-prev"].addEventListener("click", function () {
      changeConversationPage(state.conversationPage - 1);
    });
    elements["conversation-next"].addEventListener("click", function () {
      changeConversationPage(state.conversationPage + 1);
    });
    elements["refresh-button"].addEventListener("click", async function () {
      setButtonBusy(elements["refresh-button"], true, "刷新中…");
      await Promise.allSettled([
        loadDashboard(), loadConversations({ silent: true }),
        state.selectedID ? loadMessages({ silent: true }) : Promise.resolve()
      ]);
      setButtonBusy(elements["refresh-button"], false);
    });
    elements["claim-button"].addEventListener("click", claimConversation);
    elements["priority-select"].addEventListener("change", changePriority);
    elements["transfer-button"].addEventListener("click", function () {
      elements["transfer-agent"].value = "";
      elements["transfer-dialog"].showModal();
    });
    elements["resolve-button"].addEventListener("click", function () {
      elements["resolve-dialog"].showModal();
    });
    elements["composer-form"].addEventListener("submit", function (event) {
      event.preventDefault();
      sendMessage();
    });
    elements["message-input"].addEventListener("keydown", function (event) {
      if (event.key === "Enter" && !event.shiftKey && !event.isComposing) {
        event.preventDefault();
        sendMessage();
      }
    });
    elements["quick-reply-button"].addEventListener("click", function () {
      elements["quick-reply-panel"].classList.toggle("is-hidden");
    });
    elements["close-quick-replies"].addEventListener("click", function () {
      elements["quick-reply-panel"].classList.add("is-hidden");
    });
    elements["note-form"].addEventListener("submit", function (event) {
      event.preventDefault();
      addNote();
    });
    elements["transfer-form"].addEventListener("submit", function (event) {
      if (event.submitter && event.submitter.value === "default") {
        event.preventDefault();
        const target = elements["transfer-agent"].value;
        if (!target) {
          toast("请选择目标座席", "error");
          elements["transfer-agent"].focus();
          return;
        }
        transferConversation(target);
      }
    });
    elements["resolve-form"].addEventListener("submit", function (event) {
      if (event.submitter && event.submitter.value === "default") {
        event.preventDefault();
        resolveConversation();
      }
    });
    elements["logout-button"].addEventListener("click", logout);
    window.addEventListener("beforeunload", function () {
      if (state.eventSource) state.eventSource.close();
      window.clearInterval(state.pollTimer);
      window.clearInterval(state.identityTimer);
    });
  }

  async function initialize() {
    cacheElements();
    bindEvents();
    try {
      await loadMe();
      await Promise.all([loadDashboard(), loadAgents(), loadQuickReplies()]);
      updateScopeUI();
      await loadConversations();
      watchAccountChanges();
      connectEvents();
    } catch (error) {
      toast(error.message || "工作台初始化失败", "error");
      setConnection("offline", "连接失败");
    }
  }

  initialize();
})();
