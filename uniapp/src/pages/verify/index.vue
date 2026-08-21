<template>
  <view class="safe-page verify-page">
    <view class="notice-card">
      <text class="notice-title">{{ t("misc.verify.title") }}</text>
      <text class="notice-desc">{{ t("misc.verify.description") }}</text>
    </view>

    <view class="form-card card">
      <view class="field">
        <text class="label">{{ t("misc.verify.realName") }}</text>
        <input v-model.trim="realName" class="input-lite" :placeholder="t('misc.verify.realNamePlaceholder')" maxlength="20" />
      </view>
      <view class="field">
        <text class="label">{{ t("misc.auth.phone") }}</text>
        <input v-model.trim="mobile" class="input-lite" :placeholder="t('misc.auth.phonePlaceholder')" type="number" maxlength="20" />
      </view>
      <view class="field">
        <text class="label">{{ t("misc.verify.documentNumber") }}</text>
        <input v-model.trim="cardNo" class="input-lite" :placeholder="t('misc.verify.idNumberPlaceholder')" maxlength="30" />
      </view>
    </view>

    <view class="upload-section">
      <text class="section-title">{{ t("misc.verify.documentPhotos") }}</text>
      <view class="upload-grid">
        <view v-for="item in uploadItems" :key="item.key" class="upload-card" @tap="choose(item.key)">
          <image v-if="images[item.key]" class="upload-image" :src="imageSrc(images[item.key])" mode="aspectFill" />
          <view v-else class="upload-empty">
            <text class="plus">+</text>
            <text>{{ item.label }}</text>
          </view>
          <view v-if="uploadingKey === item.key" class="upload-mask">{{ t("misc.common.uploading") }}</view>
        </view>
      </view>
    </view>

    <button class="primary-button submit" :disabled="submitting || Boolean(uploadingKey)" @tap="submit">
      {{ submitting ? t("misc.common.submitting") : t("misc.detail.submitVerification") }}
    </button>
    <button class="ghost-link" @tap="openAuthStatus">{{ t("misc.verify.viewStatus") }}</button>
  </view>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import { submitAuth, uploadOne } from "@/api/services";
import type { UploadResult } from "@/types/api";
import { absolutizeUrl } from "@/utils/url";
import { requireLogin } from "@/utils/session";
import { t } from "@/i18n";

type ImageKey = "frontView" | "backView" | "handsetView";

const realName = ref("");
const mobile = ref("");
const cardNo = ref("");
const uploadingKey = ref<ImageKey | "">("");
const submitting = ref(false);
const images = reactive<Record<ImageKey, string>>({
  frontView: "",
  backView: "",
  handsetView: ""
});

const uploadItems = computed<Array<{ key: ImageKey; label: string }>>(() => [
  { key: "frontView", label: t("misc.verify.idFront") },
  { key: "backView", label: t("misc.verify.idBack") },
  { key: "handsetView", label: t("misc.verify.holdingId") }
]);

function uploadKey(result: UploadResult) {
  return result.file || result.file_name || result.filepath || result.url || "";
}

function imageSrc(value: string) {
  return absolutizeUrl(value) || value;
}

function choose(key: ImageKey) {
  if (!requireLogin() || uploadingKey.value) {
    return;
  }
  uni.chooseImage({
    count: 1,
    sizeType: ["compressed"],
    sourceType: ["album", "camera"],
    success: async (res) => {
      const path = res.tempFilePaths[0];
      if (!path) {
        return;
      }
      uploadingKey.value = key;
      try {
        const uploaded = await uploadOne(path);
        const file = uploadKey(uploaded);
        if (!file) {
          throw new Error(t("misc.verify.emptyUpload"));
        }
        images[key] = file;
      } catch (error: any) {
        uni.showToast({ title: error?.message || t("misc.common.uploadFailed"), icon: "none" });
      } finally {
        uploadingKey.value = "";
      }
    }
  });
}

function validate() {
  if (!realName.value) {
    return t("misc.verify.realNamePlaceholder");
  }
  if (!mobile.value) {
    return t("misc.auth.phonePlaceholder");
  }
  if (!cardNo.value) {
    return t("misc.verify.documentNumberPlaceholder");
  }
  if (!images.frontView || !images.backView || !images.handsetView) {
    return t("misc.verify.uploadAllPhotos");
  }
  return "";
}

async function submit() {
  if (!requireLogin() || submitting.value) {
    return;
  }
  const message = validate();
  if (message) {
    uni.showToast({ title: message, icon: "none" });
    return;
  }
  submitting.value = true;
  try {
    await submitAuth({
      realName: realName.value,
      mobile: mobile.value,
      cardNo: cardNo.value,
      frontView: images.frontView,
      backView: images.backView,
      handsetView: images.handsetView
    });
    uni.showModal({
      title: t("misc.common.submitSuccess"),
      content: t("misc.verify.submittedDescription"),
      showCancel: false,
      confirmColor: "#ff5878",
      success: () => uni.navigateBack()
    });
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.common.submitFailed"), icon: "none" });
  } finally {
    submitting.value = false;
  }
}

function openAuthStatus() {
  uni.navigateTo({ url: "/pages/detail/index?type=auth" });
}
</script>

<style scoped>
.verify-page {
  background: var(--bg);
}

.notice-card {
  padding: 34rpx 30rpx;
  border-radius: 22rpx;
  color: #fff;
  background: linear-gradient(135deg, #31364f, var(--brand));
}

.notice-title {
  display: block;
  font-size: 38rpx;
  font-weight: 900;
}

.notice-desc {
  display: block;
  margin-top: 18rpx;
  color: rgba(255, 255, 255, 0.88);
  font-size: 25rpx;
  line-height: 1.5;
}

.form-card {
  margin-top: 22rpx;
  padding: 0 24rpx;
}

.field {
  padding: 26rpx 0;
  border-bottom: 1rpx solid #f0f2f6;
}

.field:last-child {
  border-bottom: 0;
}

.label {
  display: block;
  margin-bottom: 16rpx;
  color: var(--ink);
  font-size: 25rpx;
  font-weight: 900;
}

.input-lite {
  width: 100%;
  height: 58rpx;
  color: var(--ink);
  font-size: 29rpx;
}

.upload-section {
  margin-top: 34rpx;
}

.section-title {
  display: block;
  margin-bottom: 20rpx;
  color: var(--ink);
  font-size: 31rpx;
  font-weight: 900;
}

.upload-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16rpx;
}

.upload-card {
  position: relative;
  overflow: hidden;
  height: 178rpx;
  border: 2rpx dashed #d8deea;
  border-radius: 18rpx;
  background: #fff;
}

.upload-image {
  width: 100%;
  height: 100%;
}

.upload-empty {
  display: flex;
  height: 100%;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--ink-3);
  font-size: 23rpx;
  font-weight: 800;
}

.plus {
  margin-bottom: 12rpx;
  color: var(--brand);
  font-size: 48rpx;
  line-height: 1;
}

.upload-mask {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 25rpx;
  font-weight: 900;
  background: rgba(0, 0, 0, 0.45);
}

.submit {
  margin-top: 46rpx;
}

.ghost-link {
  display: flex;
  height: 82rpx;
  align-items: center;
  justify-content: center;
  margin-top: 20rpx;
  color: var(--brand);
  font-size: 27rpx;
  font-weight: 900;
}
</style>
