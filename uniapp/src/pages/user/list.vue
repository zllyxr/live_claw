<template>
  <view class="safe-page user-list-page">
    <view v-if="users.length" class="list">
      <view v-for="item in users" :key="String(item.id || item.uid)" class="row card" @tap="openHome(item)">
        <image class="avatar" :src="avatarOf(item)" mode="aspectFill" />
        <view class="main">
          <text class="name">{{ nameOf(item) }}</text>
          <text class="desc">{{ item.signature || `ID：${item.id || item.uid || ''}` }}</text>
        </view>
        <button v-if="!isSelf(item)" class="ghost-button follow" @tap.stop="follow(item)">
          {{ Number(item.isattention || item.isattent || 0) ? "已关注" : "关注" }}
        </button>
      </view>
    </view>
    <EmptyState v-else :title="loading ? '正在加载用户' : '暂无用户'" description="下拉页面可刷新列表。" />
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onLoad, onPullDownRefresh, onReachBottom } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getFansList, getFollowList, setAttention } from "@/api/services";
import type { UserProfile } from "@/types/api";
import { absolutizeUrl } from "@/utils/url";
import { getSession, requireLogin } from "@/utils/session";

type ListType = "follow" | "fans";

const type = ref<ListType>("follow");
const uid = ref("");
const users = ref<UserProfile[]>([]);
const page = ref(1);
const loading = ref(false);
const finished = ref(false);

function uidOf(item: UserProfile) {
  return String(item.id || item.uid || "");
}

function nameOf(item: UserProfile) {
  return item.user_nicename || item.user_nickname || "星域用户";
}

function avatarOf(item: UserProfile) {
  return absolutizeUrl(String(item.avatar_thumb || item.avatar || "")) || "/static/brand/icon-round.webp";
}

function isSelf(item: UserProfile) {
  return uidOf(item) === String(getSession().uid);
}

async function load(reset = false) {
  if (!requireLogin() || loading.value || (finished.value && !reset)) {
    uni.stopPullDownRefresh();
    return;
  }
  loading.value = true;
  if (reset) {
    page.value = 1;
    finished.value = false;
  }
  try {
    const list = type.value === "follow" ? await getFollowList(uid.value, page.value) : await getFansList(uid.value, page.value);
    users.value = reset ? list : users.value.concat(list);
    if (!list.length) {
      finished.value = true;
    } else {
      page.value += 1;
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || "列表加载失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

async function follow(item: UserProfile) {
  if (!requireLogin()) {
    return;
  }
  try {
    const res = await setAttention(uidOf(item));
    item.isattention = res?.isattent ?? (Number(item.isattention || item.isattent || 0) ? 0 : 1);
    item.isattent = item.isattention;
    uni.showToast({ title: Number(item.isattention) ? "已关注" : "已取消关注", icon: "none" });
  } catch (error: any) {
    uni.showToast({ title: error?.message || "操作失败", icon: "none" });
  }
}

function openHome(item: UserProfile) {
  const toUid = uidOf(item);
  if (toUid) {
    uni.navigateTo({ url: `/pages/user/home?uid=${encodeURIComponent(toUid)}` });
  }
}

onLoad((query) => {
  type.value = String(query?.type || "follow") === "fans" ? "fans" : "follow";
  uid.value = String(query?.uid || getSession().uid || "");
  const title = type.value === "fans" ? "粉丝" : "关注";
  uni.setNavigationBarTitle({ title });
  void load(true);
});

onPullDownRefresh(() => {
  void load(true);
});

onReachBottom(() => {
  void load(false);
});
</script>

<style scoped>
.row {
  display: flex;
  align-items: center;
  gap: 18rpx;
  min-height: 112rpx;
  padding: 18rpx;
  margin-bottom: 14rpx;
}

.avatar {
  width: 78rpx;
  height: 78rpx;
  border-radius: 39rpx;
  background: #f1f2f6;
}

.main {
  flex: 1;
  min-width: 0;
}

.name {
  display: block;
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.desc {
  display: block;
  margin-top: 8rpx;
  color: var(--ink-3);
  font-size: 24rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.follow {
  min-width: 120rpx;
}
</style>
