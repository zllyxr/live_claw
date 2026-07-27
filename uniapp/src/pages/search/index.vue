<template>
  <view class="search-page">
    <view class="search-header">
      <view class="search-box">
        <image src="/static/native/home_hot_search_dark.png" mode="aspectFit" />
        <input
          v-model.trim="keyword"
          class="search-input"
          confirm-type="search"
          placeholder="请输入您要搜索的昵称或ID"
          maxlength="32"
          focus
          @confirm="submit"
        />
        <text v-if="keyword" class="clear" @tap="clear">×</text>
      </view>
      <text class="cancel" @tap="back">取消</text>
    </view>

    <view v-if="lastKeyword" class="result-head">
      <text>搜索结果</text>
      <text>{{ totalLabel }}</text>
    </view>

    <view v-if="items.length" class="result-list">
      <view v-for="item in items" :key="userKey(item)" class="user-row" @tap="openUser(item)">
        <image class="avatar" :src="avatarOf(item)" mode="aspectFill" />
        <view class="user-main">
          <view class="name-line">
            <text class="name">{{ nameOf(item) }}</text>
            <text class="id">ID: {{ idOf(item) }}</text>
          </view>
          <text class="sign">{{ item.signature || "这个人还没有留下签名" }}</text>
        </view>
        <button class="chat-button" @tap.stop="openChat(item)">私信</button>
      </view>
    </view>

    <view v-else class="empty-wrap">
      <image src="/static/brand/icon-round.webp" mode="aspectFit" />
      <text class="empty-title">{{ emptyTitle }}</text>
      <text class="empty-desc">{{ emptyDesc }}</text>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad, onPullDownRefresh, onReachBottom } from "@dcloudio/uni-app";
import { searchUsers } from "@/api/services";
import type { UserProfile } from "@/types/api";
import { absolutizeUrl, firstText } from "@/utils/url";

const keyword = ref("");
const lastKeyword = ref("");
const items = ref<UserProfile[]>([]);
const page = ref(1);
const loading = ref(false);
const finished = ref(false);

const totalLabel = computed(() => (finished.value ? `${items.value.length} 人` : "继续上滑加载"));
const emptyTitle = computed(() => {
  if (loading.value) {
    return "正在搜索";
  }
  return lastKeyword.value ? "没有找到相关用户" : "搜索星域用户";
});
const emptyDesc = computed(() => {
  if (lastKeyword.value) {
    return "换个昵称或ID再试一次。";
  }
  return "输入昵称或ID，快速找到主播和好友。";
});

function idOf(item: UserProfile) {
  return firstText(item.id, item.uid);
}

function userKey(item: UserProfile) {
  return idOf(item) || String(item.user_nicename || item.user_nickname || Math.random());
}

function nameOf(item: UserProfile) {
  return firstText(item.user_nicename, item.user_nickname, item.userNiceName, "星域用户");
}

function avatarOf(item: UserProfile) {
  return absolutizeUrl(firstText(item.avatar_thumb, item.avatar)) || "/static/brand/icon-round.webp";
}

async function load(reset = false) {
  const key = keyword.value.trim();
  if (!key) {
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
    items.value = [];
  }
  try {
    lastKeyword.value = key;
    const list = await searchUsers(key, page.value);
    items.value = reset ? list : items.value.concat(list);
    if (!list.length) {
      finished.value = true;
    } else {
      page.value += 1;
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || "搜索失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

function submit() {
  void load(true);
}

function clear() {
  keyword.value = "";
  lastKeyword.value = "";
  items.value = [];
  page.value = 1;
  finished.value = false;
}

function back() {
  uni.navigateBack();
}

function openUser(item: UserProfile) {
  const uid = idOf(item);
  if (!uid) {
    return;
  }
  uni.navigateTo({ url: `/pages/user/home?uid=${encodeURIComponent(uid)}` });
}

function openChat(item: UserProfile) {
  const uid = idOf(item);
  if (!uid) {
    return;
  }
  uni.navigateTo({
    url:
      `/pages/message/chat?touid=${encodeURIComponent(uid)}` +
      `&name=${encodeURIComponent(nameOf(item))}&avatar=${encodeURIComponent(avatarOf(item))}`
  });
}

onLoad((query) => {
  const key = String(query?.key || "");
  if (key) {
    keyword.value = key;
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
.search-page {
  min-height: 100vh;
  padding: calc(24rpx + var(--status-bar-height)) 28rpx 40rpx;
  background: var(--bg);
}

.search-header {
  display: flex;
  align-items: center;
  gap: 18rpx;
}

.search-box {
  display: flex;
  flex: 1;
  min-width: 0;
  height: 76rpx;
  align-items: center;
  padding: 0 22rpx;
  border-radius: 38rpx;
  background: #fff;
  box-shadow: 0 8rpx 22rpx rgba(35, 45, 70, 0.05);
}

.search-box image {
  width: 34rpx;
  height: 34rpx;
  margin-right: 14rpx;
}

.search-input {
  flex: 1;
  min-width: 0;
  color: var(--ink);
  font-size: 27rpx;
}

.clear {
  display: flex;
  width: 38rpx;
  height: 38rpx;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #fff;
  font-size: 28rpx;
  line-height: 1;
  background: #c7ccd6;
}

.cancel {
  flex: 0 0 auto;
  color: #4e5968;
  font-size: 27rpx;
  font-weight: 800;
}

.result-head {
  display: flex;
  justify-content: space-between;
  margin: 36rpx 4rpx 20rpx;
  color: var(--ink-3);
  font-size: 24rpx;
}

.result-head text:first-child {
  color: var(--ink);
  font-size: 30rpx;
  font-weight: 900;
}

.user-row {
  display: flex;
  align-items: center;
  gap: 18rpx;
  min-height: 126rpx;
  margin-bottom: 14rpx;
  padding: 18rpx;
  border: 1rpx solid #e9edf4;
  border-radius: 18rpx;
  background: #fff;
}

.avatar {
  width: 82rpx;
  height: 82rpx;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--line);
}

.user-main {
  flex: 1;
  min-width: 0;
}

.name-line {
  display: flex;
  align-items: baseline;
  gap: 14rpx;
}

.name {
  max-width: 260rpx;
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.id {
  color: #98a2b3;
  font-size: 22rpx;
}

.sign {
  display: block;
  margin-top: 14rpx;
  color: #7b8494;
  font-size: 24rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chat-button {
  display: flex;
  width: 92rpx;
  height: 54rpx;
  align-items: center;
  justify-content: center;
  border-radius: 27rpx;
  color: #fff;
  font-size: 23rpx;
  font-weight: 800;
  background: var(--brand);
}

.empty-wrap {
  display: flex;
  min-height: 70vh;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
}

.empty-wrap image {
  width: 128rpx;
  height: 128rpx;
  opacity: 0.9;
}

.empty-title {
  margin-top: 30rpx;
  color: var(--ink);
  font-size: 31rpx;
  font-weight: 900;
}

.empty-desc {
  margin-top: 14rpx;
  color: var(--ink-3);
  font-size: 25rpx;
}
</style>
