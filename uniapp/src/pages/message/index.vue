<template>
  <view class="safe-page inbox-page">
    <view class="inbox-overview">
      <view class="overview-copy">
        <text class="overview-desc">
          {{ totalUnread ? `${totalUnread} 条消息等你查看` : "所有消息都已读" }}
        </text>
      </view>
      <view class="connection-pill" :class="connectionState">
        <view class="connection-dot" />
        <text>{{ connectionLabel }}</text>
      </view>
    </view>

    <view class="primary-actions">
      <view class="primary-action" @tap="startChat">
        <text>新私信</text>
      </view>
      <view class="primary-action" @tap="createGroup">
        <text>创建群聊</text>
      </view>
      <view class="primary-action" @tap="joinGroup">
        <text>加入群聊</text>
      </view>
    </view>

    <view class="segment-control">
      <view v-for="tab in tabs" :key="tab.key" class="segment-button" :class="{ active: activeTab === tab.key }" @tap="switchTab(tab.key)">
        <text>{{ tab.name }}</text>
        <view v-if="tab.key === 'chat' && totalUnread" class="segment-count">
          {{ totalUnread > 99 ? "99+" : totalUnread }}
        </view>
      </view>
    </view>

    <template v-if="activeTab === 'chat'">
      <button v-if="pendingApplications.length" class="application-banner" @tap="openApplications">
        <view class="application-icon">✓</view>
        <view class="application-copy">
          <text>待处理入群申请</text>
          <text>{{ pendingApplications.length }} 位用户正在等待审核</text>
        </view>
        <view class="banner-count">{{ pendingApplications.length }}</view>
      </button>

      <view v-if="items.length" class="search-box">
        <view class="search-mark" />
        <input
          v-model.trim="keyword"
          class="search-input"
          placeholder="搜索联系人、群聊或消息"
          confirm-type="search"
        />
        <button v-if="keyword" class="clear-search" @tap="keyword = ''">×</button>
      </view>
    </template>

    <view v-if="visibleItems.length" class="conversation-list">
      <button
        v-for="item in visibleItems"
        :key="rowKey(item)"
        class="conversation-row"
        @tap="openItem(item)"
        @longpress="removeItem(item)"
      >
        <view class="avatar-wrap">
          <view v-if="activeTab === 'chat' && conversationKind(item) === 'group'" class="group-avatar">
            <view class="group-head group-head-a" />
            <view class="group-head group-head-b" />
            <view class="group-body" />
          </view>
          <image v-else class="avatar" :src="avatarOf(item)" mode="aspectFill" />
          <view
            v-if="activeTab === 'chat' && conversationKind(item) === 'single'"
            class="online-dot"
          />
        </view>

        <view class="row-main">
          <view class="row-top">
            <view class="title-wrap">
              <text class="row-title">{{ titleOf(item) }}</text>
              <view
                v-if="activeTab === 'chat' && conversationKind(item) === 'group'"
                class="type-tag"
              >
                群聊
              </view>
              <view v-else-if="activeTab === 'system'" class="type-tag notice-tag">
                {{ noticeLabelOf(item) }}
              </view>
            </view>
            <text class="row-time">{{ timeOf(item) }}</text>
          </view>
          <view class="row-bottom">
            <text class="row-desc">{{ descOf(item) }}</text>
            <view v-if="unreadOf(item)" class="unread-badge">{{ unreadOf(item) }}</view>
          </view>
        </view>
      </button>
    </view>

    <EmptyState
      v-else
      kind="message"
      :title="emptyTitle"
      :description="emptyDescription"
    />
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onPullDownRefresh, onReachBottom, onShow, onUnload } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import {
  getChatGroupApplications,
  getChatGroups,
  getConversations,
  getUnifiedNotifications,
  joinChatGroup,
  removeConversation
} from "@/api/services";
import type { ChatGroupApplication, Conversation } from "@/types/api";
import {
  onIMConnectionState,
  onOpenIMMessage,
  type ChatKind,
  type IMConnectionState
} from "@/utils/openim";
import { absolutizeUrl, firstText } from "@/utils/url";
import { getSession, requireLogin } from "@/utils/session";

type MessageTab = "chat" | "system";

const tabs: Array<{ key: MessageTab; name: string }> = [
  { key: "chat", name: "聊天" },
  { key: "system", name: "通知" }
];

const activeTab = ref<MessageTab>("chat");
const items = ref<Record<string, unknown>[]>([]);
const pendingApplications = ref<ChatGroupApplication[]>([]);
const keyword = ref("");
const page = ref(1);
const loading = ref(false);
const finished = ref(false);
const connectionState = ref<IMConnectionState>("idle");
let loadedOnce = false;
let reloadTimer: ReturnType<typeof setTimeout> | undefined;

const greeting = computed(() => {
  const hour = new Date().getHours();
  if (hour < 11) return "早上好";
  if (hour < 18) return "下午好";
  return "晚上好";
});

const connectionLabel = computed(() => {
  const labels: Record<IMConnectionState, string> = {
    idle: "未连接",
    connecting: "连接中",
    ready: "实时在线",
    offline: "正在重连"
  };
  return labels[connectionState.value];
});

const totalUnread = computed(() =>
  activeTab.value === "chat"
    ? items.value.reduce(
        (total, item) =>
          total + Math.max(0, Number(item.unread || item.unread_count || item.unReadCount || 0)),
        0
      )
    : 0
);

const visibleItems = computed(() => {
  const query = keyword.value.trim().toLocaleLowerCase();
  if (!query || activeTab.value !== "chat") {
    return items.value;
  }
  return items.value.filter((item) =>
    [titleOf(item), descOf(item), conversationTarget(item)]
      .join(" ")
      .toLocaleLowerCase()
      .includes(query)
  );
});

const emptyTitle = computed(() => {
  if (loading.value) return activeTab.value === "chat" ? "正在同步会话" : "正在加载通知";
  if (keyword.value && activeTab.value === "chat") return "没有匹配的会话";
  return activeTab.value === "chat" ? "还没有聊天" : "暂无通知";
});

const emptyDescription = computed(() => {
  if (keyword.value && activeTab.value === "chat") return "换一个昵称、群名或消息关键词试试。";
  return activeTab.value === "chat"
    ? "发起一段新对话，消息会实时出现在这里。"
    : "平台通知与互动提醒会统一展示在这里。";
});

function rowKey(item: Record<string, unknown>) {
  if (activeTab.value === "chat") {
    return firstText(item.conversationID, item.id, item.userID, item.uid, item.touid);
  }
  return [
    firstText(item._notice_type, "system"),
    firstText(item.id, item.messageid, item.dynamicid, item.object_id),
    firstText(item.addtime, item.datetime, item.time, item.created_at),
    firstText(item.content, item.message, item.msg, item.title)
  ].join("-");
}

function avatarOf(item: Record<string, unknown>) {
  return (
    absolutizeUrl(
      firstText(
        item.avatar_thumb,
        item.avatar,
        item.user_avatar,
        item.peer_avatar_thumb,
        item.peer_avatar
      )
    ) || "/static/brand/icon-round.webp"
  );
}

function conversationKind(item: Record<string, unknown>): ChatKind {
  return firstText(item.conversation_type) === "group" || firstText(item.groupID, item.group_id)
    ? "group"
    : "single";
}

function conversationTarget(item: Record<string, unknown>) {
  return conversationKind(item) === "group"
    ? firstText(item.groupID, item.group_id, item.conversationID, item.id)
    : conversationUid(item);
}

function conversationUid(item: Record<string, unknown>) {
  const self = getSession().uid;
  const direct = firstText(item.touid, item.userID, item.peer_uid, item.peer_id, item.to_uid);
  if (direct && direct !== self) return direct;
  const from = firstText(item.from_uid, item.sender_id, item.uid);
  if (from && from !== self) return from;
  return direct || from;
}

function titleOf(item: Record<string, unknown>) {
  if (activeTab.value === "system") {
    return firstText(
      item.title,
      item.message_title,
      item.user_nicename,
      item.user_nickname,
      item.from_user_nicename,
      "系统通知"
    );
  }
  const uid = conversationUid(item);
  return firstText(
    item.user_nicename,
    item.user_nickname,
    item.peer_nicename,
    item.peer_nickname,
    item.group_name,
    item.title,
    uid ? `用户 ${uid}` : "",
    "新会话"
  );
}

function descOf(item: Record<string, unknown>) {
  return firstText(
    item.last_msg,
    item.lastMsgString,
    item.content,
    item.message,
    item.msg,
    item.lastMsgTimeString,
    "还没有消息"
  );
}

function noticeLabelOf(item: Record<string, unknown>) {
  return firstText(item._notice_label, "平台");
}

function timeOf(item: Record<string, unknown>) {
  const raw = firstText(item.datetime, item.addtime, item.updated_at, item.created_at, item.time);
  if (!raw) return "";
  const numeric = Number(raw);
  const timestamp =
    Number.isFinite(numeric) && numeric > 0
      ? numeric < 1_000_000_000_000
        ? numeric * 1000
        : numeric
      : Date.parse(raw);
  if (!Number.isFinite(timestamp)) return raw;
  const date = new Date(timestamp);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  if (diff >= 0 && diff < 60_000) return "刚刚";
  if (diff >= 60_000 && diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`;
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  if (date.toDateString() === now.toDateString()) return `${hours}:${minutes}`;
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return date.getFullYear() === now.getFullYear()
    ? `${month}-${day}`
    : `${date.getFullYear()}-${month}-${day}`;
}

function unreadOf(item: Record<string, unknown>) {
  const unread = Number(item.unread || item.unread_count || item.unReadCount || 0);
  return unread > 0 ? String(unread > 99 ? "99+" : unread) : "";
}

function uniqueItems(list: Record<string, unknown>[]) {
  const seen = new Set<string>();
  return list.filter((item) => {
    const key = rowKey(item);
    if (!key || seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function conversationTimestamp(item: Record<string, unknown>) {
  const numeric = Number(
    firstText(item.updated_at, item.latestMsgSendTime, item.addtime, item.createTime, "0")
  );
  return Number.isFinite(numeric) ? numeric : 0;
}

function mergeConversations(
  conversations: Record<string, unknown>[],
  groups: Record<string, unknown>[]
) {
  const existingGroups = new Set(
    conversations
      .filter((item) => conversationKind(item) === "group")
      .map((item) => conversationTarget(item))
  );
  const groupRows = groups
    .filter((group) => !existingGroups.has(firstText(group.groupID)))
    .map((group) => ({
      id: firstText(group.groupID),
      conversationID: firstText(group.groupID),
      conversation_type: "group",
      groupID: group.groupID,
      group_id: group.groupID,
      group_name: group.groupName,
      peer_nickname: group.groupName,
      last_msg: `${Number(group.memberCount || 0)} 位成员 · 还没有消息`,
      addtime: String(group.createTime || 0),
      updated_at: Number(group.createTime || 0)
    }));
  return conversations
    .concat(groupRows)
    .sort((left, right) => conversationTimestamp(right) - conversationTimestamp(left));
}

async function load(reset = false) {
  if (!requireLogin()) {
    uni.stopPullDownRefresh();
    return;
  }
  if (loading.value || (finished.value && !reset)) return;
  loading.value = true;
  if (reset) {
    page.value = 1;
    finished.value = false;
  }
  try {
    let list: Record<string, unknown>[] = [];
    if (activeTab.value === "chat") {
      const [conversations, groups, applications] = await Promise.all([
        getConversations(),
        getChatGroups(0, 200),
        getChatGroupApplications(0, 200).catch(() => [])
      ]);
      list = mergeConversations(
        conversations as unknown as Record<string, unknown>[],
        groups as unknown as Record<string, unknown>[]
      );
      pendingApplications.value = applications.filter(
        (application) => Number(application.handleResult || 0) === 0
      );
      finished.value = true;
    } else {
      list = await getUnifiedNotifications(page.value);
    }
    items.value = uniqueItems(reset ? list : items.value.concat(list));
    if (!list.length || activeTab.value === "chat") {
      finished.value = true;
    } else {
      page.value += 1;
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || "消息加载失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

function queueRealtimeRefresh() {
  if (activeTab.value !== "chat") return;
  if (reloadTimer) clearTimeout(reloadTimer);
  reloadTimer = setTimeout(() => void load(true), 240);
}

function switchTab(tab: MessageTab) {
  if (activeTab.value === tab) return;
  activeTab.value = tab;
  keyword.value = "";
  items.value = [];
  void load(true);
}

function startChat() {
  uni.navigateTo({ url: "/pages/message/new-chat?mode=single" });
}

function createGroup() {
  uni.navigateTo({ url: "/pages/message/new-chat?mode=group" });
}

function joinGroup() {
  (uni.showModal as any)({
    title: "加入群聊",
    content: "",
    editable: true,
    placeholderText: "请输入群聊 ID",
    confirmText: "继续",
    success: async ({ confirm, content }: { confirm: boolean; content: string }) => {
      const groupID = String(content || "").trim();
      if (!confirm || !groupID) return;
      try {
        const result = await joinChatGroup(groupID);
        uni.showToast({
          title: result.joined ? "已加入群聊" : "入群申请已提交",
          icon: "none"
        });
        if (result.joined) void load(true);
      } catch (error: any) {
        uni.showToast({ title: error?.message || "申请失败", icon: "none" });
      }
    }
  });
}

function openApplications() {
  const first = pendingApplications.value[0];
  if (!first) return;
  uni.navigateTo({
    url: `/pages/message/group-info?groupid=${encodeURIComponent(first.groupID)}&section=applications`
  });
}

function removeItem(item: Record<string, unknown>) {
  if (activeTab.value !== "chat") return;
  const targetID = conversationTarget(item);
  const kind = conversationKind(item);
  if (!targetID) return;
  uni.showModal({
    title: "隐藏会话",
    content: `确认从消息列表隐藏“${titleOf(item)}”？收到新消息后会再次显示。`,
    confirmColor: "#ff4d6e",
    success: async ({ confirm }) => {
      if (!confirm) return;
      try {
        await removeConversation(targetID, kind);
        items.value = items.value.filter((row) => rowKey(row) !== rowKey(item));
        uni.showToast({ title: "会话已隐藏", icon: "none" });
      } catch (error: any) {
        uni.showToast({ title: error?.message || "操作失败", icon: "none" });
      }
    }
  });
}

function openItem(item: Record<string, unknown>) {
  if (activeTab.value === "system") {
    uni.navigateTo({
      url:
        `/pages/message/notice?type=${encodeURIComponent(firstText(item._notice_type, "system"))}` +
        `&payload=${encodeURIComponent(JSON.stringify(item))}`
    });
    return;
  }
  const kind = conversationKind(item);
  const targetID = conversationTarget(item as Conversation);
  if (!targetID) {
    uni.showToast({ title: "暂时无法打开该会话", icon: "none" });
    return;
  }
  uni.navigateTo({
    url:
      `/pages/message/chat?kind=${kind}&target=${encodeURIComponent(targetID)}` +
      `&name=${encodeURIComponent(titleOf(item))}` +
      `&avatar=${encodeURIComponent(avatarOf(item))}`
  });
}

const stopConnection = onIMConnectionState((state) => {
  connectionState.value = state;
});
const stopMessages = onOpenIMMessage(() => queueRealtimeRefresh());

onShow(() => {
  if (!loadedOnce) loadedOnce = true;
  void load(true);
});

onPullDownRefresh(() => void load(true));
onReachBottom(() => void load(false));

onUnload(() => {
  stopMessages();
  stopConnection();
  if (reloadTimer) clearTimeout(reloadTimer);
});
</script>

<style scoped>
.inbox-page {
  padding-top: 20rpx;
  background:
    radial-gradient(circle at 92% 0%, rgba(122, 92, 255, 0.12), transparent 32%),
    var(--bg);
}

.inbox-overview {
  display: flex;
  min-height: 42rpx;
  align-items: center;
  justify-content: space-between;
  gap: 22rpx;
  padding: 24rpx 26rpx;
  border: 1rpx solid rgba(122, 92, 255, 0.08);
  border-radius: 30rpx;
  background: linear-gradient(135deg, #ffffff 0%, #faf8ff 100%);
  box-shadow: 0 14rpx 36rpx rgba(59, 45, 118, 0.08);
}

.overview-copy,
.application-copy,
.row-main {
  flex: 1;
  min-width: 0;
}

.overview-kicker,
.overview-title,
.overview-desc {
  display: block;
}

.overview-kicker {
  color: var(--violet);
  font-size: 18rpx;
  font-weight: 900;
  letter-spacing: 3rpx;
}

.overview-title {
  margin-top: 7rpx;
  color: var(--ink);
  font-size: 38rpx;
  font-weight: 900;
}

.overview-desc {
  margin-top: 7rpx;
  color: var(--ink-3);
  font-size: 22rpx;
}

.connection-pill,
.type-tag,
.segment-count,
.unread-badge,
.banner-count,
.application-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  text-align: center;
}

.connection-pill {
  flex: 0 0 auto;
  height: 48rpx;
  gap: 10rpx;
  padding: 0 17rpx;
  border: 1rpx solid rgba(17, 185, 129, 0.14);
  border-radius: 24rpx;
  color: var(--green-deep);
  font-size: 20rpx;
  font-weight: 800;
  background: rgba(17, 185, 129, 0.08);
}

.connection-pill.connecting,
.connection-pill.offline,
.connection-pill.idle {
  color: #8b719e;
  border-color: rgba(122, 92, 255, 0.12);
  background: rgba(122, 92, 255, 0.08);
}

.connection-dot {
  width: 12rpx;
  height: 12rpx;
  border-radius: 50%;
  background: currentColor;
  box-shadow: 0 0 0 5rpx rgba(17, 185, 129, 0.1);
}

.primary-actions {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14rpx;
  margin: 22rpx 0;
}

.primary-action,
.segment-button,
.application-banner,
.conversation-row,
.clear-search {
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
}

.primary-action {
  height: 58rpx;
  flex-direction: column;
  gap: 10rpx;
  border: 1rpx solid var(--line);
  border-radius: 24rpx;
  color: var(--ink-2);
  font-size: 21rpx;
  font-weight: 800;
  background: #fff;
  box-shadow: 0 8rpx 22rpx rgba(25, 27, 38, 0.04);
}

.action-icon {
  position: relative;
  display: flex;
  width: 48rpx;
  height: 48rpx;
  align-items: center;
  justify-content: center;
  border-radius: 16rpx;
  background: var(--violet-soft);
}

.icon-message::before {
  width: 25rpx;
  height: 19rpx;
  border: 4rpx solid var(--violet);
  border-radius: 9rpx;
  content: "";
}

.icon-message-tail {
  position: absolute;
  bottom: 10rpx;
  left: 12rpx;
  width: 9rpx;
  height: 9rpx;
  border-bottom: 4rpx solid var(--violet);
  transform: rotate(-32deg);
}

.person {
  position: absolute;
  width: 14rpx;
  height: 14rpx;
  border: 4rpx solid var(--violet);
  border-radius: 50%;
}

.person::after {
  position: absolute;
  top: 15rpx;
  left: -7rpx;
  width: 20rpx;
  height: 11rpx;
  border: 4rpx solid var(--violet);
  border-bottom: 0;
  border-radius: 12rpx 12rpx 0 0;
  content: "";
}

.person-left {
  top: 7rpx;
  left: 8rpx;
}

.person-right {
  right: 8rpx;
  bottom: 12rpx;
}

.icon-join text {
  color: var(--violet);
  font-size: 38rpx;
  font-weight: 500;
  line-height: 1;
}

.segment-control {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8rpx;
  padding: 7rpx;
  margin-bottom: 20rpx;
  border-radius: 25rpx;
  background: #e9ebf1;
}

.segment-button {
  height: 64rpx;
  gap: 9rpx;
  border-radius: 19rpx;
  color: var(--ink-3);
  font-size: 25rpx;
  font-weight: 900;
}

.segment-button.active {
  color: var(--ink);
  background: #fff;
  box-shadow: 0 5rpx 16rpx rgba(25, 27, 38, 0.08);
}

.segment-count {
  min-width: 31rpx;
  height: 31rpx;
  padding: 0 8rpx;
  border-radius: 16rpx;
  color: #fff;
  font-size: 17rpx;
  background: var(--brand);
}

.application-banner {
  width: 100%;
  min-height: 94rpx;
  justify-content: flex-start;
  gap: 16rpx;
  padding: 16rpx 18rpx;
  margin-bottom: 18rpx;
  border: 1rpx solid rgba(122, 92, 255, 0.1);
  border-radius: 22rpx;
  background: linear-gradient(135deg, #f4f0ff 0%, #fff3f7 100%);
}

.application-icon {
  flex: 0 0 54rpx;
  width: 54rpx;
  height: 54rpx;
  border-radius: 18rpx;
  color: #fff;
  font-size: 24rpx;
  font-weight: 900;
  background: var(--grad-cosmic);
}

.application-copy {
  text-align: left;
}

.application-copy text {
  display: block;
}

.application-copy text:first-child {
  color: var(--ink);
  font-size: 24rpx;
  font-weight: 900;
}

.application-copy text:last-child {
  margin-top: 7rpx;
  color: var(--ink-3);
  font-size: 20rpx;
}

.banner-count {
  min-width: 42rpx;
  height: 42rpx;
  padding: 0 10rpx;
  border-radius: 21rpx;
  color: var(--violet);
  font-size: 20rpx;
  font-weight: 900;
  background: #fff;
}

.search-box {
  display: flex;
  height: 76rpx;
  align-items: center;
  gap: 14rpx;
  padding: 0 18rpx;
  margin-bottom: 16rpx;
  border: 1rpx solid var(--line);
  border-radius: 22rpx;
  background: rgba(255, 255, 255, 0.86);
}

.search-mark {
  position: relative;
  width: 25rpx;
  height: 25rpx;
  border: 4rpx solid #a8aebc;
  border-radius: 50%;
}

.search-mark::after {
  position: absolute;
  right: -9rpx;
  bottom: -6rpx;
  width: 11rpx;
  height: 4rpx;
  border-radius: 2rpx;
  background: #a8aebc;
  content: "";
  transform: rotate(45deg);
}

.search-input {
  flex: 1;
  height: 72rpx;
  color: var(--ink);
  font-size: 25rpx;
}

.clear-search {
  flex: 0 0 42rpx;
  width: 42rpx;
  height: 42rpx;
  border-radius: 50%;
  color: #9ca3b1;
  font-size: 31rpx;
  background: #eef0f4;
}

.conversation-row {
  width: 100%;
  height: 98rpx;
  justify-content: flex-start;
  gap: 17rpx;
  padding: 17rpx 18rpx;
  margin-bottom: 12rpx;
  border: 1rpx solid rgba(237, 239, 245, 0.9);
  border-radius: 24rpx;
  background: #fff;
  box-shadow: 0 7rpx 22rpx rgba(25, 27, 38, 0.035);
}

.avatar-wrap {
  position: relative;
  flex: 0 0 76rpx;
  width: 76rpx;
  height: 76rpx;
}

.avatar,
.group-avatar {
  display: flex;
  width: 76rpx;
  height: 76rpx;
  align-items: center;
  justify-content: center;
  border-radius: 25rpx;
  background: #eef0f5;
}

.group-avatar {
  position: relative;
  overflow: hidden;
  background: linear-gradient(145deg, #7a5cff, #c44dff 55%, #ff7da0);
  box-shadow: inset 0 1rpx 1rpx rgba(255, 255, 255, 0.28);
}

.group-head {
  position: absolute;
  top: 18rpx;
  width: 18rpx;
  height: 18rpx;
  border: 3rpx solid rgba(255, 255, 255, 0.96);
  border-radius: 50%;
}

.group-head-a {
  left: 18rpx;
}

.group-head-b {
  right: 18rpx;
}

.group-body {
  position: absolute;
  bottom: 16rpx;
  width: 48rpx;
  height: 20rpx;
  border: 3rpx solid rgba(255, 255, 255, 0.96);
  border-bottom: 0;
  border-radius: 26rpx 26rpx 0 0;
}

.online-dot {
  position: absolute;
  right: -2rpx;
  bottom: -2rpx;
  width: 20rpx;
  height: 20rpx;
  border: 4rpx solid #fff;
  border-radius: 50%;
  background: var(--green);
}

.row-top,
.row-bottom,
.title-wrap {
  display: flex;
  min-width: 0;
  align-items: center;
}

.row-top,
.row-bottom {
  justify-content: space-between;
  gap: 14rpx;
  height: 18px;
}

.title-wrap {
  flex: 1;
  gap: 9rpx;
}

.row-title,
.row-desc {
  display: block;
  overflow: hidden;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row-title {
  color: var(--ink);
  font-size: 27rpx;
  font-weight: 900;
}

.type-tag {
  flex: 0 0 auto;
  height: 32rpx;
  padding: 0 10rpx;
  border-radius: 10rpx;
  color: var(--violet);
  font-size: 17rpx;
  font-weight: 900;
  background: var(--violet-soft);
}

.notice-tag {
  color: var(--brand-deep);
  background: var(--brand-soft);
}

.row-time {
  flex: 0 0 auto;
  color: #adb2bd;
  font-size: 19rpx;
}

.row-bottom {
  margin-top: 10rpx;
}

.row-desc {
  flex: 1;
  color: var(--ink-3);
  font-size: 22rpx;
}

.unread-badge {
  flex: 0 0 auto;
  min-width: 35rpx;
  height: 35rpx;
  padding: 0 9rpx;
  border-radius: 18rpx;
  color: #fff;
  font-size: 18rpx;
  font-weight: 900;
  background: var(--brand);
  box-shadow: 0 6rpx 12rpx rgba(255, 77, 110, 0.22);
}
</style>
