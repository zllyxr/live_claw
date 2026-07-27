<template>
  <view class="safe-page message-page">
    <view class="tabs">
      <view
        v-for="tab in tabs"
        :key="tab.key"
        class="tab"
        :class="{ active: activeTab === tab.key }"
        @tap="switchTab(tab.key)"
      >
        {{ tab.name }}
      </view>
    </view>

    <view v-if="activeTab === 'chat'" class="chat-actions">
      <button class="chat-action" @tap="startChat">
        <text class="action-icon">＋</text>
        <text>发起聊天</text>
      </button>
      <button class="chat-action" @tap="createGroup">
        <text class="action-icon">群</text>
        <text>创建群聊</text>
      </button>
      <button class="chat-action" @tap="joinGroup">
        <text class="action-icon">码</text>
        <text>加入群聊</text>
      </button>
    </view>

    <view v-if="items.length" class="list">
      <view
        v-for="item in items"
        :key="rowKey(item)"
        class="row card"
        @tap="openItem(item)"
        @longpress="removeItem(item)"
      >
        <image class="avatar" :src="avatarOf(item)" mode="aspectFill" />
        <view class="row-main">
          <view class="title-line">
            <text class="row-title">{{ titleOf(item) }}</text>
            <text v-if="activeTab === 'chat' && conversationKind(item) === 'group'" class="group-tag">群聊</text>
          </view>
          <text class="row-desc">{{ descOf(item) }}</text>
          <view v-if="activeTab === 'system'" class="notice-meta">
            <text class="notice-source">{{ noticeLabelOf(item) }}</text>
            <text v-if="timeOf(item)" class="notice-time">{{ timeOf(item) }}</text>
          </view>
        </view>
        <text v-if="unreadOf(item)" class="badge">{{ unreadOf(item) }}</text>
      </view>
    </view>
    <EmptyState
      v-else
      :title="loading ? '正在加载消息' : activeTab === 'chat' ? '暂无聊天' : '暂无系统通知'"
      :description="activeTab === 'chat' ? '新的私信和群聊会显示在这里。' : '平台与互动通知会统一显示在这里。'"
    />
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onPullDownRefresh, onReachBottom, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import {
  getChatGroups,
  getConversations,
  getUnifiedNotifications,
  joinChatGroup,
  removeConversation
} from "@/api/services";
import type { Conversation } from "@/types/api";
import { absolutizeUrl, firstText } from "@/utils/url";
import { getSession, requireLogin } from "@/utils/session";

type MessageTab = "chat" | "system";

const tabs: Array<{ key: MessageTab; name: string }> = [
  { key: "chat", name: "聊天" },
  { key: "system", name: "系统通知" }
];

const activeTab = ref<MessageTab>("chat");
const items = ref<Record<string, unknown>[]>([]);
const page = ref(1);
const loading = ref(false);
const finished = ref(false);
let loadedOnce = false;

function rowKey(item: Record<string, unknown>) {
  if (activeTab.value === "chat") {
    return firstText(item.conversationID, item.id, item.userID, item.uid, item.touid, item.to_uid);
  }
  return [
    firstText(item._notice_type, "system"),
    firstText(item.id, item.messageid, item.dynamicid, item.dynamic_id, item.object_id),
    firstText(item.addtime, item.datetime, item.time, item.created_at),
    firstText(item.uid, item.from_uid, item.touid),
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

function conversationKind(item: Record<string, unknown>) {
  return firstText(item.conversation_type) === "group" || firstText(item.groupID, item.group_id)
    ? "group"
    : "single";
}

function conversationTarget(item: Record<string, unknown>) {
  return conversationKind(item) === "group"
    ? firstText(item.groupID, item.group_id)
    : conversationUid(item);
}

function titleOf(item: Record<string, unknown>) {
  if (activeTab.value === "system") {
    return firstText(
      item.title,
      item.message_title,
      item.user_nicename,
      item.user_nickname,
      item.from_user_nicename,
      noticeLabelOf(item)
    );
  }
  const uid = conversationUid(item);
  return firstText(
    item.user_nicename ||
      item.user_nickname ||
      item.peer_nicename ||
      item.peer_nickname ||
      item.group_name ||
      item.from_user_nicename ||
      item.from_user_nickname ||
      item.title ||
      item.name,
    activeTab.value === "chat" && uid ? `用户 ${uid}` : "",
    activeTab.value === "chat" ? "私信" : "系统消息"
  );
}

function descOf(item: Record<string, unknown>) {
  return firstText(item.last_msg, item.lastMsgString, item.content, item.message, item.msg, item.lastMsgTimeString, item.addtime);
}

function noticeLabelOf(item: Record<string, unknown>) {
  return firstText(item._notice_label, "系统通知");
}

function timeOf(item: Record<string, unknown>) {
  const raw = firstText(item.datetime, item.addtime, item.created_at, item.time);
  if (!raw) {
    return "";
  }
  const numeric = Number(raw);
  const timestamp =
    Number.isFinite(numeric) && numeric > 0
      ? numeric < 1_000_000_000_000
        ? numeric * 1000
        : numeric
      : Date.parse(raw);
  if (!Number.isFinite(timestamp)) {
    return raw;
  }
  const date = new Date(timestamp);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  if (diff >= 0 && diff < 60_000) {
    return "刚刚";
  }
  if (diff >= 60_000 && diff < 3_600_000) {
    return `${Math.floor(diff / 60_000)}分钟前`;
  }
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  if (date.toDateString() === now.toDateString()) {
    return `${hours}:${minutes}`;
  }
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  if (date.getFullYear() === now.getFullYear()) {
    return `${month}-${day} ${hours}:${minutes}`;
  }
  return `${date.getFullYear()}-${month}-${day}`;
}

function unreadOf(item: Record<string, unknown>) {
  const unread = Number(item.unread || item.unread_count || item.unReadCount || 0);
  return unread > 0 ? String(unread > 99 ? "99+" : unread) : "";
}

function uniqueItems(list: Record<string, unknown>[]) {
  const seen = new Set<string>();
  return list.filter((item) => {
    const key = rowKey(item);
    if (!key || seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}

function conversationTimestamp(item: Record<string, unknown>) {
  const numeric = Number(firstText(item.latestMsgSendTime, item.addtime, item.createTime, "0"));
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
      id: `group_${firstText(group.groupID)}`,
      conversationID: `group_${firstText(group.groupID)}`,
      conversation_type: "group",
      groupID: group.groupID,
      group_id: group.groupID,
      group_name: group.groupName,
      peer_nickname: group.groupName,
      peer_avatar: group.faceURL,
      last_msg: `${Number(group.memberCount || 0)}位成员`,
      addtime: String(group.createTime || 0)
    }));
  return conversations.concat(groupRows).sort((left, right) => conversationTimestamp(right) - conversationTimestamp(left));
}

function conversationUid(item: Record<string, unknown>) {
  const sessionUid = getSession().uid;
  const direct = firstText(item.touid, item.userID, item.peer_uid, item.peer_id, item.to_uid);
  if (direct && direct !== sessionUid) {
    return direct;
  }
  const from = firstText(item.from_uid, item.sender_id, item.uid);
  if (from && from !== sessionUid) {
    return from;
  }
  return direct || from;
}

async function load(reset = false) {
  if (!requireLogin()) {
    uni.stopPullDownRefresh();
    return;
  }
  if (loading.value || (finished.value && !reset)) {
    return;
  }
  loading.value = true;
  if (reset) {
    page.value = 1;
    finished.value = false;
  }
  try {
    let list: Record<string, unknown>[] = [];
    if (activeTab.value === "chat") {
      const [conversations, groups] = await Promise.all([getConversations(), getChatGroups(0, 100)]);
      list = mergeConversations(
        conversations as unknown as Record<string, unknown>[],
        groups as unknown as Record<string, unknown>[]
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

function switchTab(tab: MessageTab) {
  if (activeTab.value === tab) {
    return;
  }
  activeTab.value = tab;
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
    placeholderText: "请输入群ID",
    confirmText: "申请加入",
    success: async ({ confirm, content }: { confirm: boolean; content: string }) => {
      const groupID = String(content || "").trim();
      if (!confirm || !groupID) {
        return;
      }
      try {
        await joinChatGroup(groupID);
        uni.showToast({ title: "入群申请已提交", icon: "none" });
      } catch (error: any) {
        uni.showToast({ title: error?.message || "申请失败", icon: "none" });
      }
    }
  });
}

function removeItem(item: Record<string, unknown>) {
  if (activeTab.value !== "chat") {
    return;
  }
  const targetID = conversationTarget(item);
  const kind = conversationKind(item);
  if (!targetID) {
    return;
  }
  uni.showModal({
    title: "删除会话",
    content: `删除与“${titleOf(item)}”的会话和本地聊天记录？`,
    confirmColor: "#ff4f62",
    success: async ({ confirm }) => {
      if (!confirm) {
        return;
      }
      try {
        await removeConversation(targetID, kind);
        items.value = items.value.filter((row) => rowKey(row) !== rowKey(item));
        uni.showToast({ title: "会话已删除", icon: "none" });
      } catch (error: any) {
        uni.showToast({ title: error?.message || "删除失败", icon: "none" });
      }
    }
  });
}

function openItem(item: Record<string, unknown>) {
  if (activeTab.value !== "chat") {
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
      (kind === "single" ? `&touid=${encodeURIComponent(targetID)}` : `&groupid=${encodeURIComponent(targetID)}`) +
      `&name=${encodeURIComponent(titleOf(item))}&avatar=${encodeURIComponent(avatarOf(item))}`
  });
}

onShow(() => {
  if (!loadedOnce) {
    loadedOnce = true;
    void load(true);
  }
});

onPullDownRefresh(() => {
  void load(true);
});

onReachBottom(() => {
  void load(false);
});
</script>

<style scoped>
.tabs {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10rpx;
  padding: 8rpx;
  margin-bottom: 22rpx;
  border-radius: 36rpx;
  background: #eceff5;
}

.chat-actions {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14rpx;
  margin-bottom: 22rpx;
}

.chat-action {
  display: flex;
  min-width: 0;
  height: 112rpx;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8rpx;
  border-radius: 20rpx;
  color: var(--ink-2);
  font-size: 22rpx;
  font-weight: 800;
  background: #fff;
  border: 1rpx solid #edf0f5;
}

.action-icon {
  color: var(--brand);
  font-size: 31rpx;
  font-weight: 900;
}

.tab {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 62rpx;
  padding: 0 20rpx;
  border-radius: 31rpx;
  color: var(--ink-2);
  font-size: 25rpx;
  font-weight: 800;
  background: transparent;
}

.tab.active {
  color: var(--ink);
  background: #fff;
  box-shadow: 0 4rpx 16rpx rgba(22, 29, 45, 0.08);
}

.row {
  display: flex;
  align-items: center;
  gap: 18rpx;
  width: 100%;
  min-height: 112rpx;
  padding: 18rpx;
  margin-bottom: 14rpx;
  text-align: left;
}

.avatar {
  width: 76rpx;
  height: 76rpx;
  border-radius: 38rpx;
  background: #f1f2f6;
}

.row-main {
  flex: 1;
  min-width: 0;
}

.title-line {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10rpx;
}

.row-title {
  display: inline-block;
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.group-tag {
  flex: 0 0 auto;
  padding: 3rpx 10rpx;
  border-radius: 12rpx;
  color: var(--brand);
  font-size: 19rpx;
  font-weight: 800;
  background: rgba(124, 92, 255, 0.09);
}

.row-desc {
  display: block;
  margin-top: 8rpx;
  color: var(--ink-3);
  font-size: 24rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.notice-meta {
  display: flex;
  align-items: center;
  gap: 12rpx;
  margin-top: 10rpx;
}

.notice-source {
  padding: 4rpx 12rpx;
  border-radius: 14rpx;
  color: var(--brand);
  font-size: 20rpx;
  font-weight: 800;
  background: rgba(124, 92, 255, 0.09);
}

.notice-time {
  color: #a0a8b5;
  font-size: 21rpx;
}
</style>
