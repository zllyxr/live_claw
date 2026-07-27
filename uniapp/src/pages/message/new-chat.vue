<template>
  <view class="safe-page new-chat-page">
    <view v-if="mode === 'group'" class="group-form card">
      <text class="field-label">群聊名称</text>
      <input v-model.trim="groupName" class="group-name-input" maxlength="30" placeholder="输入群聊名称" />
    </view>

    <view class="search-bar card">
      <input
        v-model.trim="keyword"
        class="search-input"
        placeholder="搜索昵称或用户ID"
        confirm-type="search"
        @confirm="search"
      />
      <button class="search-button" :disabled="loading" @tap="search">搜索</button>
    </view>

    <view v-if="mode !== 'single' && selected.length" class="selected-bar card">
      <view class="selected-main">
        <text>已选择 {{ selected.length }} 人</text>
        <text>{{ selected.map(nameOf).join("、") }}</text>
      </view>
      <button class="clear-button" @tap="selected = []">清空</button>
    </view>

    <view class="section-head">
      <text>{{ keyword ? "搜索结果" : "我的关注" }}</text>
      <text v-if="mode !== 'single'">最多选择100人</text>
    </view>

    <view v-if="users.length" class="user-list">
      <view
        v-for="user in users"
        :key="uidOf(user)"
        class="user-row card"
        :class="{ disabled: unavailable(user) }"
        @tap="selectUser(user)"
      >
        <image class="avatar" :src="avatarOf(user)" mode="aspectFill" />
        <view class="user-main">
          <text class="user-name">{{ nameOf(user) }}</text>
          <text class="user-id">ID：{{ uidOf(user) }}</text>
        </view>
        <text v-if="unavailable(user)" class="member-state">已在群内</text>
        <view v-else-if="mode !== 'single'" class="check" :class="{ active: selectedOf(user) }">
          {{ selectedOf(user) ? "✓" : "" }}
        </view>
        <text v-else class="open-arrow">›</text>
      </view>
    </view>
    <EmptyState
      v-else
      :title="loading ? '正在搜索用户' : '没有找到用户'"
      description="可以输入昵称或用户ID继续搜索。"
    />

    <view v-if="mode !== 'single'" class="submit-space" />
    <view v-if="mode !== 'single'" class="submit-bar">
      <button class="submit-button" :disabled="submitting || !selected.length" @tap="submit">
        {{ mode === "group" ? "创建群聊" : "邀请加入群聊" }}
      </button>
    </view>
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onLoad } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import {
  createChatGroup,
  getChatGroupMembers,
  getFollowList,
  inviteChatGroupMembers,
  searchUsers
} from "@/api/services";
import type { UserProfile } from "@/types/api";
import { getSession, requireLogin } from "@/utils/session";
import { absolutizeUrl } from "@/utils/url";

type PageMode = "single" | "group" | "invite";

const mode = ref<PageMode>("single");
const groupID = ref("");
const groupName = ref("");
const keyword = ref("");
const users = ref<UserProfile[]>([]);
const selected = ref<UserProfile[]>([]);
const existingIDs = ref(new Set<string>());
const loading = ref(false);
const submitting = ref(false);

function uidOf(user: UserProfile) {
  return String(user.id || user.uid || "");
}

function nameOf(user: UserProfile) {
  return String(user.user_nicename || user.user_nickname || `用户${uidOf(user)}`);
}

function avatarOf(user: UserProfile) {
  return absolutizeUrl(String(user.avatar_thumb || user.avatar || "")) || "/static/brand/icon-round.webp";
}

function unavailable(user: UserProfile) {
  const uid = uidOf(user);
  return !uid || uid === String(getSession().uid) || existingIDs.value.has(uid);
}

function selectedOf(user: UserProfile) {
  return selected.value.some((item) => uidOf(item) === uidOf(user));
}

async function loadSuggestions() {
  if (!requireLogin()) {
    return;
  }
  loading.value = true;
  try {
    users.value = await getFollowList(getSession().uid, 1);
  } catch {
    users.value = [];
  } finally {
    loading.value = false;
  }
}

async function search() {
  if (!requireLogin()) {
    return;
  }
  if (!keyword.value) {
    await loadSuggestions();
    return;
  }
  loading.value = true;
  try {
    users.value = await searchUsers(keyword.value, 1);
  } catch (error: any) {
    uni.showToast({ title: error?.message || "搜索失败", icon: "none" });
  } finally {
    loading.value = false;
  }
}

function selectUser(user: UserProfile) {
  if (unavailable(user)) {
    return;
  }
  if (mode.value === "single") {
    uni.redirectTo({
      url:
        `/pages/message/chat?kind=single&target=${encodeURIComponent(uidOf(user))}` +
        `&touid=${encodeURIComponent(uidOf(user))}&name=${encodeURIComponent(nameOf(user))}` +
        `&avatar=${encodeURIComponent(avatarOf(user))}`
    });
    return;
  }
  if (selectedOf(user)) {
    selected.value = selected.value.filter((item) => uidOf(item) !== uidOf(user));
    return;
  }
  if (selected.value.length >= 100) {
    uni.showToast({ title: "单次最多选择100人", icon: "none" });
    return;
  }
  selected.value.push(user);
}

async function submit() {
  if (!selected.value.length || submitting.value) {
    return;
  }
  submitting.value = true;
  try {
    const userIDs = selected.value.map(uidOf);
    if (mode.value === "group") {
      if (!groupName.value) {
        uni.showToast({ title: "请输入群聊名称", icon: "none" });
        return;
      }
      const group = await createChatGroup(groupName.value, userIDs);
      uni.redirectTo({
        url:
          `/pages/message/chat?kind=group&target=${encodeURIComponent(group.groupID)}` +
          `&groupid=${encodeURIComponent(group.groupID)}&name=${encodeURIComponent(group.groupName)}` +
          `&avatar=${encodeURIComponent(String(group.faceURL || ""))}`
      });
      return;
    }
    await inviteChatGroupMembers(groupID.value, userIDs);
    uni.showToast({ title: "邀请已发送", icon: "none" });
    setTimeout(() => uni.navigateBack(), 350);
  } catch (error: any) {
    uni.showToast({ title: error?.message || "操作失败", icon: "none" });
  } finally {
    submitting.value = false;
  }
}

onLoad(async (query) => {
  const nextMode = String(query?.mode || "single");
  mode.value = nextMode === "group" || nextMode === "invite" ? nextMode : "single";
  groupID.value = String(query?.groupid || "");
  uni.setNavigationBarTitle({
    title: mode.value === "group" ? "创建群聊" : mode.value === "invite" ? "邀请群成员" : "发起聊天"
  });
  if (mode.value === "invite" && groupID.value) {
    try {
      const members = await getChatGroupMembers(groupID.value, 0, 500);
      existingIDs.value = new Set(members.map((member) => member.userID));
    } catch {
      existingIDs.value = new Set();
    }
  }
  await loadSuggestions();
});
</script>

<style scoped>
.new-chat-page {
  padding-bottom: calc(24rpx + env(safe-area-inset-bottom));
  background: var(--bg);
}

.group-form,
.search-bar,
.selected-bar {
  margin-bottom: 18rpx;
}

.group-form {
  padding: 22rpx;
}

.field-label {
  display: block;
  margin-bottom: 14rpx;
  color: var(--ink);
  font-size: 25rpx;
  font-weight: 900;
}

.group-name-input,
.search-input {
  height: 76rpx;
  padding: 0 22rpx;
  border-radius: 16rpx;
  color: var(--ink);
  font-size: 27rpx;
  background: #f4f6f9;
}

.search-bar {
  display: flex;
  gap: 12rpx;
  padding: 14rpx;
}

.search-input {
  flex: 1;
  min-width: 0;
}

.search-button {
  flex: 0 0 116rpx;
  height: 76rpx;
  border-radius: 16rpx;
  color: #fff;
  font-size: 25rpx;
  font-weight: 900;
  background: var(--brand);
}

.selected-bar {
  display: flex;
  align-items: center;
  gap: 18rpx;
  padding: 20rpx;
}

.selected-main {
  flex: 1;
  min-width: 0;
}

.selected-main text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.selected-main text:first-child {
  color: var(--ink);
  font-size: 26rpx;
  font-weight: 900;
}

.selected-main text:last-child {
  margin-top: 7rpx;
  color: var(--ink-3);
  font-size: 22rpx;
}

.clear-button {
  flex: 0 0 92rpx;
  height: 56rpx;
  border-radius: 28rpx;
  color: var(--brand);
  font-size: 22rpx;
  font-weight: 800;
  background: rgba(124, 92, 255, 0.09);
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 24rpx 4rpx 14rpx;
}

.section-head text:first-child {
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 900;
}

.section-head text:last-child {
  color: var(--ink-3);
  font-size: 21rpx;
}

.user-row {
  display: flex;
  align-items: center;
  gap: 16rpx;
  min-height: 108rpx;
  padding: 16rpx 20rpx;
  margin-bottom: 12rpx;
}

.user-row.disabled {
  opacity: 0.5;
}

.avatar {
  width: 72rpx;
  height: 72rpx;
  border-radius: 36rpx;
  background: #eef1f5;
}

.user-main {
  flex: 1;
  min-width: 0;
}

.user-name,
.user-id {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-name {
  color: var(--ink);
  font-size: 28rpx;
  font-weight: 900;
}

.user-id {
  margin-top: 7rpx;
  color: var(--ink-3);
  font-size: 22rpx;
}

.check {
  display: flex;
  width: 42rpx;
  height: 42rpx;
  align-items: center;
  justify-content: center;
  border-radius: 21rpx;
  color: #fff;
  font-size: 24rpx;
  font-weight: 900;
  border: 2rpx solid #d6dbe4;
}

.check.active {
  background: var(--brand);
  border-color: var(--brand);
}

.member-state,
.open-arrow {
  color: var(--ink-3);
  font-size: 22rpx;
}

.open-arrow {
  font-size: 40rpx;
}

.submit-space {
  height: 124rpx;
}

.submit-bar {
  position: fixed;
  right: 0;
  bottom: 0;
  left: 0;
  z-index: 10;
  padding: 16rpx 24rpx calc(16rpx + env(safe-area-inset-bottom));
  background: #fff;
  border-top: 1rpx solid #edf0f5;
}

.submit-button {
  height: 78rpx;
  border-radius: 39rpx;
  color: #fff;
  font-size: 28rpx;
  font-weight: 900;
  background: var(--brand);
}

.submit-button[disabled] {
  opacity: 0.45;
}
</style>
