<template>
  <view class="edit-page">
    <!-- 顶部宇宙渐变 hero -->
    <view class="hero">
      <view class="hero-stars" />
      <view class="hero-orb orb-a" />
      <view class="hero-orb orb-b" />
      <image v-if="hasBg" class="hero-bg" :src="bgUrl" mode="aspectFill" />
      <view class="hero-shade" />

      <view class="nav-row">
        <view class="nav-btn" @tap="goBack">
          <text class="nav-arrow">‹</text>
        </view>
        <text class="nav-title">{{ t("misc.profile.title") }}</text>
        <view class="nav-btn ghost" :class="{ busy: uploading }" @tap="chooseBg">
          <text class="nav-chip-text">{{ uploading ? t("misc.common.uploading") : t("misc.profile.changeBackground") }}</text>
        </view>
      </view>
    </view>

    <!-- 头像浮层 -->
    <view class="avatar-zone">
      <view class="avatar-ring" @tap="chooseAvatar">
        <image class="avatar" :src="avatarDisplay" mode="aspectFill" @error="avatarFailed = true" />
        <view v-if="uploading" class="avatar-loading">
          <view class="spinner" />
        </view>
        <view class="camera-badge">
          <text class="camera-glyph">✎</text>
        </view>
      </view>
      <text class="preview-name">{{ nickname || t("misc.common.defaultUser") }}</text>
      <view class="id-chip">ID · {{ user?.liang_name || user?.id || "—" }}</view>
    </view>

    <!-- 表单 -->
    <view class="form-area">
      <view class="form-card c1">
        <view class="label-row">
          <view class="label-band" />
          <text class="label">{{ t("misc.profile.nickname") }}</text>
          <text class="count">{{ nickname.length }}/20</text>
        </view>
        <input v-model.trim="nickname" class="input-box" :placeholder="t('misc.profile.nicknamePlaceholder')" placeholder-class="ph" maxlength="20" />
      </view>

      <view class="form-card c2">
        <view class="label-row">
          <view class="label-band" />
          <text class="label">{{ t("misc.profile.gender") }}</text>
        </view>
        <view class="segmented">
          <view
            v-for="item in sexes"
            :key="item.value"
            class="seg"
            :class="{ active: sex === item.value }"
            @tap="sex = item.value"
          >
            <text class="seg-icon">{{ item.icon }}</text>
            <text class="seg-text">{{ item.label }}</text>
          </view>
        </view>
      </view>

      <view class="form-card c3">
        <view class="label-row">
          <view class="label-band" />
          <text class="label">{{ t("misc.profile.signature") }}</text>
          <text class="count">{{ signature.length }}/80</text>
        </view>
        <textarea
          v-model.trim="signature"
          class="textarea-box"
          :placeholder="t('misc.profile.signaturePlaceholder')"
          placeholder-class="ph"
          maxlength="80"
          auto-height
        />
      </view>
    </view>

    <!-- 保存 -->
    <view class="save-bar">
      <view class="save-btn" :class="{ disabled: saving || uploading }" @tap="save">
        <view class="shine" />
        <text>{{ saving ? t("misc.common.saving") : t("misc.profile.saveProfile") }}</text>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { getBaseInfo, updateAvatar, updateUserBg, updateUserFields, uploadOne } from "@/api/services";
import type { UploadResult, UserProfile } from "@/types/api";
import { absolutizeUrl } from "@/utils/url";
import { requireLogin, saveUser } from "@/utils/session";
import { t } from "@/i18n";

const user = ref<UserProfile>();
const nickname = ref("");
const sex = ref("0");
const signature = ref("");
const saving = ref(false);
const uploading = ref(false);

const sexes = computed(() => [
  { label: t("misc.profile.secret"), value: "0", icon: "✦" },
  { label: t("misc.profile.male"), value: "1", icon: "♂" },
  { label: t("misc.profile.female"), value: "2", icon: "♀" }
]);

const avatarFailed = ref(false);
const avatarUrl = computed(() => absolutizeUrl(String(user.value?.avatar_thumb || user.value?.avatar || "")) || "/static/icons/avatar-default.svg");
const avatarDisplay = computed(() => (avatarFailed.value ? "/static/icons/avatar-default.svg" : avatarUrl.value));
const hasBg = computed(() => Boolean(String(user.value?.bg_img || "").trim()));
const bgUrl = computed(() => absolutizeUrl(String(user.value?.bg_img || "")));

function uploadKey(result: UploadResult) {
  return result.file || result.file_name || result.filepath || result.url || "";
}

function goBack() {
  if (getCurrentPages().length > 1) {
    uni.navigateBack();
    return;
  }
  uni.switchTab({ url: "/pages/tabbar/me/index" });
}

async function load() {
  if (!requireLogin()) {
    return;
  }
  try {
    const info = await getBaseInfo();
    user.value = info;
    avatarFailed.value = false;
    nickname.value = String(info?.user_nicename || info?.user_nickname || "");
    sex.value = String(info?.sex ?? "0");
    signature.value = String(info?.signature || "");
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.profile.loadFailed"), icon: "none" });
  }
}

function chooseAvatar() {
  chooseImage(async (path) => {
    uploading.value = true;
    try {
      const uploaded = await uploadOne(path);
      const key = uploadKey(uploaded);
      await updateAvatar(key);
      uni.showToast({ title: t("misc.profile.avatarUpdated"), icon: "none" });
      await load();
    } catch (error: any) {
      uni.showToast({ title: error?.message || t("misc.profile.avatarUploadFailed"), icon: "none" });
    } finally {
      uploading.value = false;
    }
  });
}

function chooseBg() {
  chooseImage(async (path) => {
    uploading.value = true;
    try {
      const uploaded = await uploadOne(path);
      const key = uploadKey(uploaded);
      await updateUserBg(key);
      uni.showToast({ title: t("misc.profile.backgroundUpdated"), icon: "none" });
      await load();
    } catch (error: any) {
      uni.showToast({ title: error?.message || t("misc.profile.backgroundUploadFailed"), icon: "none" });
    } finally {
      uploading.value = false;
    }
  });
}

function chooseImage(onPick: (path: string) => void) {
  if (!requireLogin() || uploading.value) {
    return;
  }
  uni.chooseImage({
    count: 1,
    sizeType: ["compressed"],
    sourceType: ["album", "camera"],
    success: (res) => {
      const path = res.tempFilePaths[0];
      if (path) {
        onPick(path);
      }
    }
  });
}

async function save() {
  if (!requireLogin() || saving.value || uploading.value) {
    return;
  }
  if (!nickname.value) {
    uni.showToast({ title: t("misc.profile.enterNickname"), icon: "none" });
    return;
  }
  saving.value = true;
  try {
    await updateUserFields({
      user_nickname: nickname.value,
      sex: sex.value,
      signature: signature.value
    });
    const info = await getBaseInfo();
    if (info) {
      saveUser(info);
    }
    uni.showToast({ title: t("misc.profile.saved"), icon: "none" });
    setTimeout(() => uni.navigateBack(), 350);
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.common.saveFailed"), icon: "none" });
  } finally {
    saving.value = false;
  }
}

onShow(() => {
  void load();
});
</script>

<style scoped>
.edit-page {
  min-height: 100vh;
  padding-bottom: calc(150rpx + env(safe-area-inset-bottom));
  background: var(--bg);
  overflow-x: hidden;
}

/* ---------- hero ---------- */
.hero {
  position: relative;
  height: calc(340rpx + var(--status-bar-height));
  overflow: hidden;
  background: linear-gradient(150deg, #3b2a86 0%, #6a3fb5 48%, #b04a96 100%);
}

.hero-stars {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image:
    radial-gradient(3rpx 3rpx at 16% 28%, rgba(255, 255, 255, 0.85), transparent 100%),
    radial-gradient(2rpx 2rpx at 36% 62%, rgba(255, 255, 255, 0.5), transparent 100%),
    radial-gradient(3rpx 3rpx at 56% 20%, rgba(255, 255, 255, 0.7), transparent 100%),
    radial-gradient(2rpx 2rpx at 74% 52%, rgba(255, 255, 255, 0.45), transparent 100%),
    radial-gradient(3rpx 3rpx at 88% 26%, rgba(255, 255, 255, 0.6), transparent 100%);
}

.hero-orb {
  position: absolute;
  border-radius: 50%;
  pointer-events: none;
}

.orb-a {
  top: -120rpx;
  right: -70rpx;
  width: 320rpx;
  height: 320rpx;
  background: radial-gradient(circle at 36% 36%, rgba(255, 173, 205, 0.5), rgba(255, 173, 205, 0) 70%);
}

.orb-b {
  bottom: -140rpx;
  left: -90rpx;
  width: 320rpx;
  height: 320rpx;
  background: radial-gradient(circle at 60% 40%, rgba(122, 92, 255, 0.5), rgba(122, 92, 255, 0) 70%);
}

.hero-bg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0.5;
}

.hero-shade {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 140rpx;
  background: linear-gradient(180deg, rgba(20, 12, 56, 0) 0%, rgba(20, 12, 56, 0.32) 100%);
}

.nav-row {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: calc(20rpx + var(--status-bar-height)) 24rpx 0;
}

.nav-btn {
  display: flex;
  min-width: 64rpx;
  height: 64rpx;
  align-items: center;
  justify-content: center;
  border-radius: 999rpx;
  background: rgba(255, 255, 255, 0.18);
  backdrop-filter: blur(10px);
  transition: transform 0.15s ease;
}

.nav-btn:active {
  transform: scale(0.92);
}

.nav-btn.ghost {
  padding: 0 22rpx;
}

.nav-btn.busy {
  opacity: 0.6;
}

.nav-arrow {
  color: #fff;
  font-size: 44rpx;
  font-weight: 300;
  line-height: 1;
  margin-top: -4rpx;
}

.nav-chip-text {
  color: #fff;
  font-size: 23rpx;
  font-weight: 700;
}

.nav-title {
  color: #fff;
  font-size: 32rpx;
  font-weight: 800;
  letter-spacing: 2rpx;
}

/* ---------- avatar ---------- */
.avatar-zone {
  position: relative;
  z-index: 3;
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-top: -110rpx;
  animation: floatIn 0.5s ease both;
}

@keyframes floatIn {
  from {
    opacity: 0;
    transform: translateY(20rpx);
  }
  to {
    opacity: 1;
    transform: none;
  }
}

.avatar-ring {
  position: relative;
  width: 176rpx;
  height: 176rpx;
  padding: 8rpx;
  border-radius: 50%;
  background: linear-gradient(135deg, #7a5cff, #ff4d88);
  box-shadow: 0 16rpx 40rpx rgba(84, 56, 160, 0.35);
}

.avatar {
  width: 100%;
  height: 100%;
  border: 6rpx solid #fff;
  border-radius: 50%;
  background: #eef1f7;
}

.avatar-loading {
  position: absolute;
  inset: 8rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(15, 12, 40, 0.45);
}

.spinner {
  width: 44rpx;
  height: 44rpx;
  border: 6rpx solid rgba(255, 255, 255, 0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: rotate 0.8s linear infinite;
}

@keyframes rotate {
  to {
    transform: rotate(360deg);
  }
}

.camera-badge {
  position: absolute;
  right: 2rpx;
  bottom: 2rpx;
  display: flex;
  width: 52rpx;
  height: 52rpx;
  align-items: center;
  justify-content: center;
  border: 4rpx solid #fff;
  border-radius: 50%;
  background: var(--grad-brand);
  box-shadow: 0 6rpx 14rpx rgba(255, 77, 110, 0.4);
}

.camera-glyph {
  color: #fff;
  font-size: 24rpx;
  line-height: 1;
}

.preview-name {
  margin-top: 20rpx;
  color: var(--ink);
  font-size: 34rpx;
  font-weight: 800;
}

.id-chip {
  margin-top: 12rpx;
  padding: 6rpx 20rpx;
  border-radius: 999rpx;
  color: var(--ink-3);
  font-size: 21rpx;
  background: var(--surface);
  box-shadow: var(--shadow-soft);
}

/* ---------- form ---------- */
.form-area {
  padding: 30rpx 28rpx 0;
}

.form-card {
  margin-bottom: 20rpx;
  padding: 26rpx;
  border-radius: var(--radius);
  background: var(--surface);
  box-shadow: var(--shadow-soft);
  animation: floatIn 0.5s ease both;
}

.form-card.c1 {
  animation-delay: 0.06s;
}

.form-card.c2 {
  animation-delay: 0.12s;
}

.form-card.c3 {
  animation-delay: 0.18s;
}

.label-row {
  display: flex;
  align-items: center;
  gap: 12rpx;
  margin-bottom: 18rpx;
}

.label-band {
  width: 8rpx;
  height: 26rpx;
  border-radius: 5rpx;
  background: var(--grad-brand);
}

.label {
  flex: 1;
  color: var(--ink);
  font-size: 27rpx;
  font-weight: 800;
}

.count {
  color: var(--ink-3);
  font-size: 21rpx;
  font-variant-numeric: tabular-nums;
}

.input-box {
  width: 100%;
  height: 84rpx;
  padding: 0 24rpx;
  border-radius: 18rpx;
  color: var(--ink);
  font-size: 29rpx;
  background: var(--bg);
}

.textarea-box {
  width: 100%;
  min-height: 130rpx;
  padding: 20rpx 24rpx;
  border-radius: 18rpx;
  color: var(--ink);
  font-size: 27rpx;
  line-height: 1.6;
  background: var(--bg);
}

.ph {
  color: var(--ink-3);
}

/* segmented */
.segmented {
  display: flex;
  gap: 14rpx;
}

.seg {
  display: flex;
  flex: 1;
  height: 96rpx;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6rpx;
  border-radius: 20rpx;
  background: var(--bg);
  transition: all 0.18s ease;
}

.seg:active {
  transform: scale(0.94);
}

.seg-icon {
  color: var(--ink-3);
  font-size: 30rpx;
  line-height: 1;
}

.seg-text {
  color: var(--ink-2);
  font-size: 23rpx;
  font-weight: 600;
}

.seg.active {
  background: var(--grad-brand);
  box-shadow: 0 10rpx 22rpx rgba(255, 77, 110, 0.28);
}

.seg.active .seg-icon,
.seg.active .seg-text {
  color: #fff;
}

/* ---------- save ---------- */
.save-bar {
  position: fixed;
  right: 0;
  bottom: 0;
  left: 0;
  z-index: 20;
  padding: 16rpx 28rpx calc(16rpx + env(safe-area-inset-bottom));
  background: rgba(245, 246, 250, 0.9);
  backdrop-filter: blur(14px);
}

.save-btn {
  position: relative;
  display: flex;
  height: 96rpx;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 48rpx;
  background: var(--grad-brand);
  box-shadow: var(--shadow-brand);
  transition: transform 0.15s ease;
}

.save-btn:active {
  transform: scale(0.97);
}

.save-btn.disabled {
  opacity: 0.55;
}

.save-btn text {
  color: #fff;
  font-size: 31rpx;
  font-weight: 800;
  letter-spacing: 4rpx;
}

.shine {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 60rpx;
  background: linear-gradient(100deg, transparent, rgba(255, 255, 255, 0.6), transparent);
  transform: skewX(-24deg);
  animation: shine 2.6s ease-in-out infinite;
}

@keyframes shine {
  0% {
    left: -80rpx;
  }
  55%,
  100% {
    left: 110%;
  }
}
</style>
