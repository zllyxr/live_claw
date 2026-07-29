<template>
  <view class="safe-page contact-page">
    <view class="page-intro">
      <view class="intro-icon" :class="mode">
        <view v-if="mode === 'single'" class="message-glyph"><view /></view>
        <template v-else>
          <view class="person-glyph person-glyph-a" />
          <view class="person-glyph person-glyph-b" />
        </template>
      </view>
      <view class="intro-copy">
        <text>{{ pageTitle }}</text>
        <text>{{ pageDescription }}</text>
      </view>
    </view>

    <view v-if="mode === 'group'" class="group-name-panel panel">
      <view class="field-head">
        <text>群聊名称</text>
        <text>{{ groupName.length }}/200</text>
      </view>
      <input
        v-model="groupName"
        maxlength="200"
        placeholder="给新群聊起个名字"
        confirm-type="next"
      />
    </view>

    <view class="search-box panel">
      <view class="search-mark" />
      <input
        v-model.trim="keyword"
        placeholder="搜索昵称或用户 ID"
        confirm-type="search"
        @confirm="search"
      />
      <button v-if="keyword" class="clear-search" @tap="clearKeyword">×</button>
      <button class="search-button" :disabled="loading" @tap="search">
        {{ loading ? "搜索中" : "搜索" }}
      </button>
    </view>

    <view v-if="mode !== 'single' && selected.length" class="selection-panel panel">
      <view class="selection-head">
        <text>已选择 {{ selected.length }} 人</text>
        <button @tap="selected = []">清空</button>
      </view>
      <scroll-view class="selected-scroll" scroll-x>
        <view class="selected-list">
          <button
            v-for="user in selected"
            :key="uidOf(user)"
            class="selected-user"
            @tap="selectUser(user)"
          >
            <view class="selected-avatar">
              <image :src="avatarOf(user)" mode="aspectFill" />
              <view class="remove-mark">×</view>
            </view>
            <text>{{ nameOf(user) }}</text>
          </button>
        </view>
      </scroll-view>
    </view>

    <view class="section-head">
      <view>
        <text>{{ keyword ? "搜索结果" : "推荐联系人" }}</text>
        <text>{{ mode === "single" ? "选择联系人开始聊天" : `还可选择 ${100 - selected.length} 人` }}</text>
      </view>
      <view v-if="users.length" class="result-count">{{ users.length }}</view>
    </view>

    <view v-if="users.length" class="user-list panel">
      <view
        v-for="user in users"
        :key="uidOf(user)"
        class="user-row"
        :class="{ disabled: unavailable(user) }"
        @tap="selectUser(user)"
      >
        <image class="avatar" :src="avatarOf(user)" mode="aspectFill" />
        <view class="user-main">
          <text class="user-name">{{ nameOf(user) }}</text>
          <text class="user-id">用户 ID {{ uidOf(user) }}</text>
        </view>
        <view v-if="unavailable(user)" class="state-tag">
          {{ existingIDs.has(uidOf(user)) ? "已在群内" : "本人" }}
        </view>
        <view
          v-else-if="mode !== 'single'"
          class="check-button"
          :class="{ active: selectedOf(user) }"
        >
          <view v-if="selectedOf(user)" class="check-mark" />
        </view>
        <view v-else class="row-chevron" />
      </view>
    </view>

    <EmptyState
      v-else
      kind="search"
      :title="loading ? '正在搜索用户' : '没有找到用户'"
      description="换一个昵称或用户 ID 试试。"
    />

    <view v-if="mode !== 'single'" class="submit-space" />
    <view v-if="mode !== 'single'" class="submit-bar">
      <button
        class="submit-button"
        :disabled="submitting || !selected.length || (mode === 'group' && !groupName.trim())"
        @tap="submit"
      >
        <view v-if="submitting" class="loading-spinner" />
        <text v-else>
          {{ mode === "group" ? `创建群聊（${selected.length + 1}）` : `邀请 ${selected.length} 人` }}
        </text>
      </button>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
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

const pageTitle = computed(() =>
  mode.value === "group" ? "创建群聊" : mode.value === "invite" ? "邀请群成员" : "发起新私信"
);
const pageDescription = computed(() =>
  mode.value === "group"
    ? "选择联系人，一起开启新的群聊"
    : mode.value === "invite"
      ? "选择还未加入本群的联系人"
      : "从关注列表或搜索结果中选择联系人"
);

function uidOf(user: UserProfile) {
  return String(user.id || user.uid || "");
}

function nameOf(user: UserProfile) {
  return String(user.user_nicename || user.user_nickname || `用户 ${uidOf(user)}`);
}

function avatarOf(user: UserProfile) {
  return (
    absolutizeUrl(String(user.avatar_thumb || user.avatar || "")) ||
    "/static/brand/icon-round.webp"
  );
}

function unavailable(user: UserProfile) {
  const uid = uidOf(user);
  return !uid || uid === String(getSession().uid) || existingIDs.value.has(uid);
}

function selectedOf(user: UserProfile) {
  return selected.value.some((item) => uidOf(item) === uidOf(user));
}

function uniqueUsers(list: UserProfile[]) {
  const seen = new Set<string>();
  return list.filter((user) => {
    const uid = uidOf(user);
    if (!uid || seen.has(uid)) return false;
    seen.add(uid);
    return true;
  });
}

async function loadSuggestions() {
  if (!requireLogin()) return;
  loading.value = true;
  try {
    users.value = uniqueUsers(await getFollowList(getSession().uid, 1));
  } catch {
    users.value = [];
  } finally {
    loading.value = false;
  }
}

async function search() {
  if (!requireLogin() || loading.value) return;
  if (!keyword.value) {
    await loadSuggestions();
    return;
  }
  loading.value = true;
  try {
    users.value = uniqueUsers(await searchUsers(keyword.value, 1));
  } catch (error: any) {
    uni.showToast({ title: error?.message || "搜索失败", icon: "none" });
  } finally {
    loading.value = false;
  }
}

function clearKeyword() {
  keyword.value = "";
  void loadSuggestions();
}

function selectUser(user: UserProfile) {
  if (unavailable(user)) return;
  if (mode.value === "single") {
    uni.redirectTo({
      url:
        `/pages/message/chat?kind=single&target=${encodeURIComponent(uidOf(user))}` +
        `&name=${encodeURIComponent(nameOf(user))}` +
        `&avatar=${encodeURIComponent(avatarOf(user))}`
    });
    return;
  }
  if (selectedOf(user)) {
    selected.value = selected.value.filter((item) => uidOf(item) !== uidOf(user));
    return;
  }
  if (selected.value.length >= 100) {
    uni.showToast({ title: "单次最多选择 100 人", icon: "none" });
    return;
  }
  selected.value.push(user);
}

async function submit() {
  if (!selected.value.length || submitting.value) return;
  if (mode.value === "group" && !groupName.value.trim()) {
    uni.showToast({ title: "请输入群聊名称", icon: "none" });
    return;
  }
  submitting.value = true;
  try {
    const userIDs = selected.value.map(uidOf);
    if (mode.value === "group") {
      const group = await createChatGroup(groupName.value.trim(), userIDs);
      uni.redirectTo({
        url:
          `/pages/message/chat?kind=group&target=${encodeURIComponent(group.groupID)}` +
          `&name=${encodeURIComponent(group.groupName)}`
      });
      return;
    }
    const result = await inviteChatGroupMembers(groupID.value, userIDs);
    uni.showToast({ title: `已邀请 ${result.invited || selected.value.length} 人`, icon: "none" });
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
  uni.setNavigationBarTitle({ title: pageTitle.value });

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
.contact-page {
  background:
    radial-gradient(circle at 94% 0%, rgba(122, 92, 255, 0.12), transparent 28%),
    var(--bg);
}

.page-intro {
  display: flex;
  align-items: center;
  gap: 18rpx;
  padding: 20rpx 4rpx 24rpx;
}

.intro-icon {
  position: relative;
  display: flex;
  flex: 0 0 72rpx;
  width: 72rpx;
  height: 72rpx;
  align-items: center;
  justify-content: center;
  border-radius: 24rpx;
  color: var(--violet);
  background: var(--violet-soft);
}

.intro-icon.group,
.intro-icon.invite {
  color: #fff;
  background: var(--grad-cosmic);
  box-shadow: 0 9rpx 20rpx rgba(122, 92, 255, 0.19);
}

.message-glyph {
  position: relative;
  width: 35rpx;
  height: 27rpx;
  border: 4rpx solid currentColor;
  border-radius: 12rpx;
}

.message-glyph::after {
  position: absolute;
  bottom: -8rpx;
  left: 4rpx;
  width: 11rpx;
  height: 9rpx;
  border-bottom: 4rpx solid currentColor;
  transform: rotate(-31deg);
  content: "";
}

.person-glyph {
  position: absolute;
  width: 17rpx;
  height: 17rpx;
  border: 3rpx solid currentColor;
  border-radius: 50%;
}

.person-glyph::after {
  position: absolute;
  top: 17rpx;
  left: -7rpx;
  width: 25rpx;
  height: 12rpx;
  border: 3rpx solid currentColor;
  border-bottom: 0;
  border-radius: 14rpx 14rpx 0 0;
  content: "";
}

.person-glyph-a {
  top: 12rpx;
  left: 11rpx;
}

.person-glyph-b {
  right: 11rpx;
  bottom: 25rpx;
}

.intro-copy {
  flex: 1;
  min-width: 0;
}

.intro-copy text {
  display: block;
}

.intro-copy text:first-child {
  color: var(--ink);
  font-size: 32rpx;
  font-weight: 900;
}

.intro-copy text:last-child {
  margin-top: 7rpx;
  color: var(--ink-3);
  font-size: 21rpx;
}

.panel {
  border-radius: 24rpx;
  background: #fff;
}

.group-name-panel,
.search-box,
.selection-panel {
  margin-bottom: 17rpx;
}

.group-name-panel {
  padding: 19rpx;
}

.field-head,
.selection-head,
.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18rpx;
}

.field-head {
  margin-bottom: 11rpx;
}

.field-head text:first-child {
  color: var(--ink);
  font-size: 22rpx;
  font-weight: 900;
}

.field-head text:last-child {
  color: var(--ink-3);
  font-size: 18rpx;
}

.group-name-panel input {
  height: 72rpx;
  padding: 0 19rpx;
  border: 2rpx solid transparent;
  border-radius: 18rpx;
  color: var(--ink);
  font-size: 25rpx;
  background: #f4f5f8;
}

.group-name-panel input:focus {
  border-color: rgba(122, 92, 255, 0.24);
  background: #fff;
}

.search-box {
  display: flex;
  height: 80rpx;
  align-items: center;
  gap: 13rpx;
  padding: 0 10rpx 0 18rpx;
}

.search-mark {
  position: relative;
  width: 23rpx;
  height: 23rpx;
  border: 3rpx solid #a7adba;
  border-radius: 50%;
}

.search-mark::after {
  position: absolute;
  right: -8rpx;
  bottom: -5rpx;
  width: 10rpx;
  height: 3rpx;
  background: #a7adba;
  content: "";
  transform: rotate(45deg);
}

.search-box input {
  flex: 1;
  height: 76rpx;
  color: var(--ink);
  font-size: 24rpx;
}

.clear-search,
.search-button,
.selection-head button,
.selected-user,
.user-row,
.state-tag,
.check-button,
.result-count,
.submit-button {
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
}

.clear-search {
  flex: 0 0 40rpx;
  width: 40rpx;
  height: 40rpx;
  border-radius: 50%;
  color: #969eac;
  font-size: 28rpx;
  background: #eef0f4;
}

.search-button {
  flex: 0 0 94rpx;
  height: 60rpx;
  border-radius: 18rpx;
  color: #fff;
  font-size: 22rpx;
  font-weight: 900;
  background: var(--violet);
}

.search-button[disabled] {
  opacity: 0.48;
}

.selection-panel {
  padding: 17rpx;
}

.selection-head {
  margin-bottom: 14rpx;
}

.selection-head > text {
  color: var(--ink);
  font-size: 22rpx;
  font-weight: 900;
}

.selection-head button {
  width: 76rpx;
  height: 44rpx;
  border-radius: 15rpx;
  color: var(--brand-deep);
  font-size: 19rpx;
  font-weight: 800;
  background: var(--brand-soft);
}

.selected-scroll {
  width: 100%;
  white-space: nowrap;
}

.selected-list {
  display: inline-flex;
  gap: 15rpx;
}

.selected-user {
  width: 84rpx;
  flex-direction: column;
  gap: 7rpx;
  overflow: visible;
}

.selected-avatar {
  position: relative;
  width: 62rpx;
  height: 62rpx;
}

.selected-avatar image {
  width: 62rpx;
  height: 62rpx;
  border-radius: 21rpx;
  background: #eef0f5;
}

.remove-mark {
  position: absolute;
  top: -7rpx;
  right: -7rpx;
  display: flex;
  width: 25rpx;
  height: 25rpx;
  align-items: center;
  justify-content: center;
  border: 3rpx solid #fff;
  border-radius: 50%;
  color: #fff;
  font-size: 18rpx;
  background: var(--brand);
}

.selected-user > text {
  display: block;
  width: 84rpx;
  overflow: hidden;
  color: var(--ink-2);
  font-size: 18rpx;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.section-head {
  min-height: 66rpx;
  padding: 0 4rpx;
}

.section-head > view:first-child {
  flex: 1;
  min-width: 0;
}

.section-head text {
  display: block;
}

.section-head text:first-child {
  color: var(--ink);
  font-size: 28rpx;
  font-weight: 900;
}

.section-head text:last-child {
  margin-top: 5rpx;
  color: var(--ink-3);
  font-size: 18rpx;
}

.result-count {
  min-width: 40rpx;
  height: 40rpx;
  padding: 0 10rpx;
  border-radius: 20rpx;
  color: var(--violet);
  font-size: 19rpx;
  font-weight: 900;
  background: var(--violet-soft);
}

.user-list {
  overflow: hidden;
}

.user-row {
  width: 100%;
  height: 98rpx;
  justify-content: flex-start;
  gap: 15rpx;
  padding: 15rpx 17rpx;
  border-bottom: 1rpx solid var(--line);
  text-align: left;
  margin-top: 10px;
}

.user-row:last-child {
  border-bottom: 0;
}

.user-row.disabled {
  opacity: 0.5;
}

.avatar {
  flex: 0 0 68rpx;
  width: 68rpx;
  height: 68rpx;
  border-radius: 23rpx;
  background: #eef0f5;
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
  height: 36rpx;
}

.user-name {
  color: var(--ink);
  font-size: 20rpx;
  font-weight: 900;
}

.user-id {
  margin-top: 7rpx;
  color: var(--ink-3);
  font-size: 18rpx;
}

.state-tag {
  flex: 0 0 auto;
  height: 38rpx;
  padding: 0 11rpx;
  border-radius: 13rpx;
  color: var(--ink-3);
  font-size: 18rpx;
  background: #f0f1f4;
}

.check-button {
  flex: 0 0 43rpx;
  width: 43rpx;
  height: 43rpx;
  border: 3rpx solid #d4d8e1;
  border-radius: 15rpx;
  color: #fff;
}

.check-button.active {
  border-color: transparent;
  background: var(--violet);
  box-shadow: 0 6rpx 14rpx rgba(122, 92, 255, 0.2);
}

.check-mark {
  width: 17rpx;
  height: 10rpx;
  border-bottom: 3rpx solid #fff;
  border-left: 3rpx solid #fff;
  transform: rotate(-45deg) translateY(-2rpx);
}

.row-chevron {
  flex: 0 0 14rpx;
  width: 14rpx;
  height: 14rpx;
  margin-right: 7rpx;
  border-top: 3rpx solid #bcc1cb;
  border-right: 3rpx solid #bcc1cb;
  transform: rotate(45deg);
}

.submit-space {
  height: 126rpx;
}

.submit-bar {
  position: fixed;
  right: 0;
  bottom: 0;
  left: 0;
  z-index: 20;
  padding: 15rpx 24rpx calc(15rpx + env(safe-area-inset-bottom));
  border-top: 1rpx solid var(--line);
  background: rgba(255, 255, 255, 0.97);
  box-shadow: 0 -8rpx 28rpx rgba(25, 27, 38, 0.05);
}

.submit-button {
  width: 100%;
  height: 80rpx;
  gap: 12rpx;
  border-radius: 25rpx;
  color: #fff;
  font-size: 25rpx;
  font-weight: 900;
  background: var(--grad-cosmic);
  box-shadow: 0 10rpx 24rpx rgba(122, 92, 255, 0.2);
}

.submit-button[disabled] {
  opacity: 0.43;
}

.loading-spinner {
  width: 23rpx;
  height: 23rpx;
  border: 3rpx solid rgba(255, 255, 255, 0.35);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
