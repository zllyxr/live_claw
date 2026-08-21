<template>
  <view class="user-home page">
    <view class="hero">
      <image class="hero-bg" :src="bgUrl" mode="aspectFill" />
      <view class="hero-shade" />
      <view class="hero-content">
        <image class="avatar" :src="avatarUrl" mode="aspectFill" />
        <view class="name-row">
          <text class="name">{{ name }}</text>
          <text class="uid">ID：{{ user?.liang_name || user?.id || uid }}</text>
        </view>
        <text class="signature">{{ user?.signature || t("misc.common.noSignature") }}</text>
        <view class="stats">
          <view class="stat-item" @tap="openList('follow')">
            <text>{{ user?.follows || 0 }}</text>
            <text>{{ t("misc.common.following") }}</text>
          </view>
          <view class="stat-item" @tap="openList('fans')">
            <text>{{ user?.fans || 0 }}</text>
            <text>{{ t("misc.common.fans") }}</text>
          </view>
          <view class="stat-item">
            <text>{{ user?.coin || user?.votes || 0 }}</text>
            <text>{{ t("misc.common.balance") }}</text>
          </view>
        </view>
      </view>
    </view>

    <view class="action-bar">
      <button v-if="own" class="primary-button action-primary" @tap="editProfile">{{ t("misc.profile.editProfile") }}</button>
      <template v-else>
        <button class="primary-button action-primary" @tap="toggleFollow">{{ followed ? t("misc.common.followed") : t("misc.common.follow") }}</button>
        <button class="ghost-button action-ghost" @tap="openChat">{{ t("misc.common.message") }}</button>
        <button class="ghost-button action-ghost danger" @tap="toggleBlack">{{ blacked ? t("misc.user.unblock") : t("misc.user.block") }}</button>
      </template>
    </view>

    <view class="content safe-page">
      <view class="section-head">
        <text class="title-lg">{{ t("misc.common.posts") }}</text>
        <view class="ghost-button" @tap="openDynamics">{{ t("misc.common.all") }}</view>
      </view>

      <view v-if="dynamics.length" class="feed">
        <view v-for="item in dynamics" :key="String(item.id || item.dynamicid)" class="dynamic-card card" @tap="openDynamic(item)">
          <text v-if="item.title" class="dynamic-text">{{ item.title }}</text>
          <view v-if="imageList(item).length" class="thumb-row">
            <image v-for="image in imageList(item).slice(0, 3)" :key="image" class="thumb" :src="image" mode="aspectFill" />
          </view>
          <text class="dynamic-meta">{{ item.datetime || "" }} · {{ item.likes || 0 }} {{ t("misc.common.likes") }} · {{ item.comments || 0 }} {{ t("misc.common.comments") }}</text>
        </view>
      </view>
      <EmptyState v-else :title="loading ? t('misc.user.loadingHome') : t('misc.user.noPosts')" :description="t('misc.user.noPostsDescription')" />
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad, onPullDownRefresh } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getUserDynamics, getUserHome, setAttention, setBlack } from "@/api/services";
import type { DynamicItem, UserProfile } from "@/types/api";
import { absolutizeUrl } from "@/utils/url";
import { getSession, requireLogin } from "@/utils/session";
import { t } from "@/i18n";

const uid = ref("");
const user = ref<UserProfile>();
const dynamics = ref<DynamicItem[]>([]);
const loading = ref(false);
const followed = ref(false);
const blacked = ref(false);

const own = computed(() => String(uid.value) === String(getSession().uid));
const name = computed(() => user.value?.user_nicename || user.value?.user_nickname || t("misc.common.defaultUser"));
const avatarUrl = computed(() => absolutizeUrl(String(user.value?.avatar_thumb || user.value?.avatar || "")) || "/static/brand/icon-round.webp");
const bgUrl = computed(() => absolutizeUrl(String(user.value?.bg_img || user.value?.avatar || "")) || "/static/brand/icon.webp");

function imageList(item: DynamicItem) {
  return String(item.thumb || "")
    .split(";")
    .map((image) => absolutizeUrl(image))
    .filter(Boolean);
}

function dynamicId(item: DynamicItem) {
  return String(item.id || item.dynamicid || "");
}

async function load() {
  if (!requireLogin() || !uid.value) {
    uni.stopPullDownRefresh();
    return;
  }
  loading.value = true;
  try {
    const [profile, list] = await Promise.all([getUserHome(uid.value), getUserDynamics(uid.value, 1)]);
    user.value = profile;
    dynamics.value = list.slice(0, 6);
    followed.value = Number(profile?.isattention || profile?.isattent || 0) === 1;
    blacked.value = Number(profile?.isblack || 0) === 1;
    uni.setNavigationBarTitle({ title: name.value });
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.user.homeLoadFailed"), icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

async function toggleFollow() {
  if (!requireLogin()) {
    return;
  }
  try {
    const res = await setAttention(uid.value);
    followed.value = Number(res?.isattent ?? (followed.value ? 0 : 1)) === 1;
    uni.showToast({ title: followed.value ? t("misc.common.followed") : t("misc.common.unfollowed"), icon: "none" });
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.common.operationFailed"), icon: "none" });
  }
}

function toggleBlack() {
  if (!requireLogin()) {
    return;
  }
  uni.showModal({
    title: blacked.value ? t("misc.user.unblock") : t("misc.user.blockUser"),
    content: blacked.value ? t("misc.user.unblockConfirm") : t("misc.user.blockConfirm"),
    confirmColor: "#ff5878",
    success: ({ confirm }) => {
      if (!confirm) {
        return;
      }
      setBlack(uid.value)
        .then((res) => {
          blacked.value = Number(res?.isblack ?? (blacked.value ? 0 : 1)) === 1;
          uni.showToast({ title: blacked.value ? t("misc.user.blocked") : t("misc.user.unblocked"), icon: "none" });
        })
        .catch((error: any) => {
          uni.showToast({ title: error?.message || t("misc.common.operationFailed"), icon: "none" });
        });
    }
  });
}

function openChat() {
  if (!requireLogin()) {
    return;
  }
  uni.navigateTo({
    url:
      `/pages/message/chat?touid=${encodeURIComponent(uid.value)}` +
      `&name=${encodeURIComponent(name.value)}&avatar=${encodeURIComponent(avatarUrl.value)}`
  });
}

function openList(type: "follow" | "fans") {
  uni.navigateTo({ url: `/pages/user/list?type=${type}&uid=${encodeURIComponent(uid.value)}&name=${encodeURIComponent(name.value)}` });
}

function openDynamic(item: DynamicItem) {
  const id = dynamicId(item);
  if (id) {
    uni.navigateTo({ url: `/pages/dynamic/detail?id=${encodeURIComponent(id)}` });
  }
}

function openDynamics() {
  if (own.value) {
    uni.navigateTo({ url: "/pages/dynamic/my" });
  } else {
    uni.showToast({ title: t("misc.user.recentPostsShown"), icon: "none" });
  }
}

function editProfile() {
  uni.navigateTo({ url: "/pages/profile/edit" });
}

onLoad((query) => {
  uid.value = String(query?.uid || getSession().uid || "");
  void load();
});

onPullDownRefresh(() => {
  void load();
});
</script>

<style scoped>
.user-home {
  background: var(--bg);
}

.hero {
  position: relative;
  min-height: 460rpx;
  overflow: hidden;
  color: #fff;
}

.hero-bg,
.hero-shade {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.hero-shade {
  background: linear-gradient(180deg, rgba(16, 18, 28, 0.18), rgba(16, 18, 28, 0.78));
}

.hero-content {
  position: relative;
  z-index: 1;
  padding: 160rpx 28rpx 34rpx;
}

.avatar {
  width: 132rpx;
  height: 132rpx;
  border: 4rpx solid rgba(255, 255, 255, 0.86);
  border-radius: 66rpx;
  background: #f1f2f6;
}

.name-row {
  margin-top: 18rpx;
}

.name {
  display: block;
  font-size: 42rpx;
  font-weight: 900;
}

.uid,
.signature {
  display: block;
  margin-top: 8rpx;
  color: rgba(255, 255, 255, 0.84);
  font-size: 24rpx;
}

.signature {
  max-width: 660rpx;
  line-height: 1.45;
}

.stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12rpx;
  margin-top: 28rpx;
}

.stat-item {
  min-height: 92rpx;
  padding: 16rpx;
  border-radius: 14rpx;
  color: #fff;
  text-align: left;
  background: rgba(0, 0, 0, 0.18);
  border: 1rpx solid rgba(255, 255, 255, 0.14);
}

.stat-item text:first-child {
  display: block;
  font-size: 34rpx;
  font-weight: 900;
}

.stat-item text:last-child {
  display: block;
  margin-top: 6rpx;
  color: rgba(255, 255, 255, 0.74);
  font-size: 23rpx;
}

.action-bar {
  display: flex;
  gap: 16rpx;
  padding: 20rpx 24rpx;
  background: #fff;
  border-bottom: 1rpx solid #eef1f5;
}

.action-primary {
  flex: 1;
  height: 76rpx;
  border-radius: 38rpx;
  font-size: 28rpx;
}

.action-ghost {
  min-width: 132rpx;
  height: 76rpx;
  border-radius: 38rpx;
}

.danger {
  color: #ff4f62;
}

.content {
  padding-top: 24rpx;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18rpx;
}

.dynamic-card {
  display: block;
  width: 100%;
  padding: 22rpx;
  margin-bottom: 16rpx;
  text-align: left;
}

.dynamic-text {
  display: block;
  color: #323845;
  font-size: 28rpx;
  line-height: 1.5;
}

.thumb-row {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8rpx;
  margin-top: 16rpx;
}

.thumb {
  width: 100%;
  height: 150rpx;
  border-radius: 10rpx;
  background: #f1f2f6;
}

.dynamic-meta {
  display: block;
  margin-top: 16rpx;
  color: var(--ink-3);
  font-size: 23rpx;
}
</style>
