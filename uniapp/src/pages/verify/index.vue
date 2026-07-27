<template>
  <view class="safe-page verify-page">
    <view class="notice-card">
      <text class="notice-title">实名认证</text>
      <text class="notice-desc">请提交真实资料，审核通过后可使用更多直播与收益功能。</text>
    </view>

    <view class="form-card card">
      <view class="field">
        <text class="label">真实姓名</text>
        <input v-model.trim="realName" class="input-lite" placeholder="请输入真实姓名" maxlength="20" />
      </view>
      <view class="field">
        <text class="label">手机号</text>
        <input v-model.trim="mobile" class="input-lite" placeholder="请输入手机号" type="number" maxlength="20" />
      </view>
      <view class="field">
        <text class="label">证件号码</text>
        <input v-model.trim="cardNo" class="input-lite" placeholder="请输入身份证号" maxlength="30" />
      </view>
    </view>

    <view class="upload-section">
      <text class="section-title">证件照片</text>
      <view class="upload-grid">
        <view v-for="item in uploadItems" :key="item.key" class="upload-card" @tap="choose(item.key)">
          <image v-if="images[item.key]" class="upload-image" :src="imageSrc(images[item.key])" mode="aspectFill" />
          <view v-else class="upload-empty">
            <text class="plus">+</text>
            <text>{{ item.label }}</text>
          </view>
          <view v-if="uploadingKey === item.key" class="upload-mask">上传中</view>
        </view>
      </view>
    </view>

    <button class="primary-button submit" :disabled="submitting || Boolean(uploadingKey)" @tap="submit">
      {{ submitting ? "正在提交" : "提交认证" }}
    </button>
    <button class="ghost-link" @tap="openLegacyAuth">打开旧版认证页</button>
  </view>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";
import { submitAuth, uploadOne } from "@/api/services";
import type { UploadResult } from "@/types/api";
import { absolutizeUrl } from "@/utils/url";
import { requireLogin } from "@/utils/session";
import { buildAuthUrl, openWebView } from "@/utils/navigation";

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

const uploadItems: Array<{ key: ImageKey; label: string }> = [
  { key: "frontView", label: "身份证正面" },
  { key: "backView", label: "身份证反面" },
  { key: "handsetView", label: "手持身份证" }
];

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
          throw new Error("上传返回为空");
        }
        images[key] = file;
      } catch (error: any) {
        uni.showToast({ title: error?.message || "上传失败", icon: "none" });
      } finally {
        uploadingKey.value = "";
      }
    }
  });
}

function validate() {
  if (!realName.value) {
    return "请输入真实姓名";
  }
  if (!mobile.value) {
    return "请输入手机号";
  }
  if (!cardNo.value) {
    return "请输入证件号码";
  }
  if (!images.frontView || !images.backView || !images.handsetView) {
    return "请上传完整证件照片";
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
      title: "提交成功",
      content: "认证资料已提交，请等待平台审核。",
      showCancel: false,
      confirmColor: "#ff5878",
      success: () => uni.navigateBack()
    });
  } catch (error: any) {
    uni.showToast({ title: error?.message || "提交失败", icon: "none" });
  } finally {
    submitting.value = false;
  }
}

function openLegacyAuth() {
  openWebView(buildAuthUrl(), "认证");
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
