<template>
  <view class="safe-page notice-page">
    <view class="notice-card card">
      <view class="notice-head">
        <image class="avatar" :src="avatar" mode="aspectFill" />
        <view class="head-main">
          <view class="notice-heading">
            <text class="notice-type">系统通知</text>
            <text class="notice-source">{{ sourceName }}</text>
          </view>
          <text class="notice-time">{{ timeText || "刚刚" }}</text>
        </view>
      </view>
      <text class="notice-title">{{ title }}</text>
      <text class="notice-content">{{ content || "暂无详细内容" }}</text>
    </view>

    <view class="action-list card">
      <view v-if="targetUid" class="action-row" @tap="openUser">
        <text>查看用户主页</text>
        <text>›</text>
      </view>
      <view v-if="dynamicId" class="action-row" @tap="openDynamic">
        <text>查看相关动态</text>
        <text>›</text>
      </view>
      <view v-if="groupId" class="action-row" @tap="openGroup">
        <text>查看群聊与入群申请</text>
        <text>›</text>
      </view>
      <view class="action-row" @tap="backMessages">
        <text>返回消息中心</text>
        <text>›</text>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad } from "@dcloudio/uni-app";
import { absolutizeUrl, firstText } from "@/utils/url";

type NoticeTab = "system" | "group" | "at" | "like" | "comment" | "fans";

const tab = ref<NoticeTab>("system");
const item = ref<Record<string, unknown>>({});

const sourceName = computed(() => {
  const map: Record<NoticeTab, string> = {
    system: "平台",
    group: "群聊申请",
    at: "提及",
    like: "点赞",
    comment: "评论",
    fans: "关注"
  };
  return firstText(item.value._notice_label, map[tab.value], "平台");
});

const title = computed(() =>
  firstText(
    item.value.title,
    item.value.user_nicename,
    item.value.user_nickname,
    item.value.from_user_nicename,
    "系统通知"
  )
);
const content = computed(() =>
  firstText(
    item.value.content,
    item.value.message,
    item.value.msg,
    item.value.comment_content,
    item.value.dynamic_title,
    item.value.addtime
  )
);
const avatar = computed(
  () =>
    absolutizeUrl(
      firstText(item.value.avatar, item.value.avatar_thumb, item.value.user_avatar, item.value.from_avatar)
    ) || "/static/brand/icon-round.webp"
);
const timeText = computed(() => {
  const raw = firstText(item.value.datetime, item.value.addtime, item.value.created_at, item.value.time);
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
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  return `${year}-${month}-${day} ${hours}:${minutes}`;
});
const targetUid = computed(() => firstText(item.value.uid, item.value.from_uid, item.value.touid, item.value.userid));
const dynamicId = computed(() => firstText(item.value.dynamicid, item.value.dynamic_id, item.value.object_id));
const groupId = computed(() => firstText(item.value.groupID, item.value.group_id));

function openUser() {
  if (targetUid.value) {
    uni.navigateTo({ url: `/pages/user/home?uid=${encodeURIComponent(targetUid.value)}` });
  }
}

function openDynamic() {
  if (dynamicId.value) {
    uni.navigateTo({ url: `/pages/dynamic/detail?id=${encodeURIComponent(dynamicId.value)}` });
  }
}

function openGroup() {
  if (groupId.value) {
    uni.navigateTo({ url: `/pages/message/group-info?groupid=${encodeURIComponent(groupId.value)}` });
  }
}

function backMessages() {
  uni.navigateBack();
}

onLoad((query) => {
  tab.value = String(query?.type || "system") as NoticeTab;
  const payload = String(query?.payload || "");
  if (payload) {
    try {
      item.value = JSON.parse(decodeURIComponent(payload));
    } catch {
      item.value = {};
    }
  }
  uni.setNavigationBarTitle({ title: "系统通知" });
});
</script>

<style scoped>
.notice-page {
  background: var(--bg);
}

.notice-card {
  padding: 28rpx;
}

.notice-head {
  display: flex;
  align-items: center;
  gap: 18rpx;
}

.avatar {
  width: 82rpx;
  height: 82rpx;
  border-radius: 41rpx;
  background: var(--line);
}

.head-main {
  flex: 1;
  min-width: 0;
}

.notice-heading {
  display: flex;
  align-items: center;
  gap: 12rpx;
}

.notice-type {
  color: var(--ink);
  font-size: 30rpx;
  font-weight: 900;
}

.notice-source {
  padding: 5rpx 12rpx;
  border-radius: 14rpx;
  color: var(--brand);
  font-size: 20rpx;
  font-weight: 800;
  background: rgba(124, 92, 255, 0.09);
}

.notice-time {
  display: block;
  margin-top: 8rpx;
  color: #98a2b3;
  font-size: 23rpx;
}

.notice-title {
  display: block;
  margin-top: 30rpx;
  color: var(--ink);
  font-size: 34rpx;
  font-weight: 900;
  line-height: 1.35;
}

.notice-content {
  display: block;
  margin-top: 18rpx;
  color: #4b5565;
  font-size: 28rpx;
  line-height: 1.7;
}

.action-list {
  overflow: hidden;
  margin-top: 22rpx;
}

.action-row {
  display: flex;
  height: 98rpx;
  align-items: center;
  justify-content: space-between;
  padding: 0 26rpx;
  border-bottom: 1rpx solid #f0f2f6;
  color: var(--ink);
  font-size: 28rpx;
  font-weight: 800;
}

.action-row:last-child {
  border-bottom: 0;
}

.action-row text:last-child {
  color: #b8bfcc;
  font-size: 42rpx;
}
</style>
