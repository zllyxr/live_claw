<template>
  <view class="publish-page">
    <view class="publish-shell">
      <view class="top-panel">
        <view class="author-row">
          <image class="avatar" :src="avatarUrl" mode="aspectFill" />
          <view class="author-main">
            <text class="author-name">{{ authorName }}</text>
            <text class="author-sub">{{ modeSub }}</text>
          </view>
          <button class="draft-button" @tap="resetDraft">清空</button>
        </view>

        <textarea
          v-model="text"
          class="editor"
          maxlength="300"
          auto-height
          placeholder="分享这一刻..."
          :adjust-position="true"
        />
        <view class="editor-foot">
          <text>{{ modeLabel }}</text>
          <text>{{ text.length }}/300</text>
        </view>
      </view>

      <view class="mode-strip" :class="{ two: modes.length === 2 }">
        <button
          v-for="item in modes"
          :key="item.key"
          class="mode-button"
          :class="{ active: mode === item.key }"
          @tap="switchMode(item.key)"
        >
          <text class="mode-icon">{{ item.icon }}</text>
          <text>{{ item.name }}</text>
        </button>
      </view>

      <view class="media-panel">
        <template v-if="mode === 'image'">
          <view class="image-grid">
            <view v-for="(image, index) in images" :key="image" class="image-cell">
              <image class="preview-image" :src="image" mode="aspectFill" @tap="previewImage(image)" />
              <button class="remove-button" @tap.stop="removeImage(index)">×</button>
            </view>
            <button v-if="images.length < 9" class="add-cell" @tap="chooseImages">
              <text>+</text>
              <text>{{ images.length ? "继续添加" : "添加图片" }}</text>
            </button>
          </view>
          <text class="media-tip">{{ imageTip }}</text>
        </template>

        <template v-else-if="mode === 'video'">
          <view v-if="videoPath" class="video-card">
            <video class="video-preview" :src="videoPath" :poster="videoThumb" controls />
            <view class="media-actions">
              <button @tap="chooseVideo">重选视频</button>
              <button class="danger" @tap="clearVideo">删除视频</button>
            </view>
          </view>
          <button v-else class="large-picker" @tap="chooseVideo">
            <text class="large-plus">+</text>
            <text>选择视频</text>
          </button>
          <text class="media-tip">支持单个视频，发布时会先上传视频文件。</text>
        </template>

        <template v-else>
          <view class="voice-card">
            <view class="voice-state" :class="{ recording }">
              <view class="voice-dot" />
              <view class="voice-main">
                <text>{{ voiceTitle }}</text>
                <text>{{ voiceSubtitle }}</text>
              </view>
            </view>
            <view class="media-actions">
              <button class="record-button" :class="{ recording }" @tap="toggleRecord">
                {{ recording ? "停止录音" : voicePath ? "重新录音" : "开始录音" }}
              </button>
              <button v-if="voicePath" class="danger" @tap="clearVoice">删除语音</button>
            </view>
          </view>
          <text class="media-tip">最长录制 60 秒，发布时会上传音频文件。</text>
        </template>
      </view>

      <view v-if="statusText" class="status-row">
        <view class="status-bar">
          <view class="status-fill" :style="{ width: `${progress}%` }" />
        </view>
        <text>{{ statusText }}</text>
      </view>

      <button class="primary-button publish-submit" :disabled="!canPublish || publishing || recording" @tap="publish">
        {{ submitText }}
      </button>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import { ACTIVE_TYPES } from "@/constants/config";
import { publishDynamic, uploadOne } from "@/api/services";
import type { UploadResult } from "@/types/api";
import { getSession, requireLogin } from "@/utils/session";
import { absolutizeUrl, firstText } from "@/utils/url";

type PublishMode = "image" | "video" | "voice";

const DYNAMIC_DIRTY_KEY = "claw_dynamic_dirty";

const supportsVoice = ref(false);

// #ifdef APP-PLUS
supportsVoice.value = true;
// #endif

const modes = computed<Array<{ key: PublishMode; name: string; icon: string }>>(() => {
  const base: Array<{ key: PublishMode; name: string; icon: string }> = [
  { key: "image", name: "图片", icon: "图" },
  { key: "video", name: "视频", icon: "视" }
  ];
  if (supportsVoice.value) {
    base.push({ key: "voice", name: "语音", icon: "声" });
  }
  return base;
});

const text = ref("");
const mode = ref<PublishMode>("image");
const images = ref<string[]>([]);
const videoPath = ref("");
const videoThumb = ref("");
const voicePath = ref("");
const voiceSeconds = ref(0);
const recording = ref(false);
const publishing = ref(false);
const statusText = ref("");
const progress = ref(0);
const recordElapsed = ref(0);

let recordStart = 0;
let recordTimer: ReturnType<typeof setInterval> | undefined;
let recorder: UniApp.RecorderManager | undefined;

const session = computed(() => getSession());
const user = computed(() => session.value.user);
const avatarUrl = computed(() => absolutizeUrl(firstText(user.value?.avatar_thumb, user.value?.avatar)) || "/static/brand/icon-round.webp");
const authorName = computed(() => firstText(user.value?.user_nicename, user.value?.user_nickname, "星域用户"));
const modeLabel = computed(() => modes.value.find((item) => item.key === mode.value)?.name || "图片");
const imageTip = computed(() => `最多 9 张图片，图片、视频${supportsVoice.value ? "、语音" : ""}只能选择一种。`);
const modeSub = computed(() => {
  if (mode.value === "image") {
    return images.value.length ? `已选择 ${images.value.length} 张图片` : "发布图片动态";
  }
  if (mode.value === "video") {
    return videoPath.value ? "已选择 1 个视频" : "发布视频动态";
  }
  if (recording.value) {
    return `正在录音 ${recordElapsed.value}s`;
  }
  return voicePath.value ? `已录制 ${voiceSeconds.value}s` : "发布语音动态";
});

const voiceTitle = computed(() => {
  if (recording.value) {
    return "正在录音";
  }
  return voicePath.value ? "语音已准备好" : "还没有录音";
});

const voiceSubtitle = computed(() => {
  if (recording.value) {
    return `${recordElapsed.value}s / 60s`;
  }
  return voicePath.value ? `${voiceSeconds.value}s` : "点击下方按钮开始录制";
});

const canPublish = computed(() => {
  const hasText = Boolean(text.value.trim());
  if (mode.value === "image") {
    return hasText || images.value.length > 0;
  }
  if (mode.value === "video") {
    return hasText || Boolean(videoPath.value);
  }
  return hasText || Boolean(voicePath.value);
});

const submitText = computed(() => {
  if (publishing.value) {
    return "发布中";
  }
  if (recording.value) {
    return "请先停止录音";
  }
  return "发布动态";
});

function ensureRecorder() {
  if (!supportsVoice.value) {
    uni.showToast({ title: "H5暂不支持语音动态，请在App内发布", icon: "none" });
    return undefined;
  }
  if (recorder) {
    return recorder;
  }
  // #ifdef APP-PLUS
  try {
    recorder = uni.getRecorderManager();
    recorder.onStop((res) => {
      stopRecordTimer();
      recording.value = false;
      voicePath.value = res.tempFilePath;
      voiceSeconds.value = Math.max(1, Math.round((Date.now() - recordStart) / 1000));
      recordElapsed.value = voiceSeconds.value;
    });
    recorder.onError(() => {
      stopRecordTimer();
      recording.value = false;
      uni.showToast({ title: "录音失败", icon: "none" });
    });
    return recorder;
  } catch {
    uni.showToast({ title: "当前环境不支持录音", icon: "none" });
    return undefined;
  }
  // #endif
  return undefined;
}

function stopRecordTimer() {
  if (recordTimer) {
    clearInterval(recordTimer);
    recordTimer = undefined;
  }
}

function startRecordTimer() {
  stopRecordTimer();
  recordElapsed.value = 0;
  recordTimer = setInterval(() => {
    recordElapsed.value = Math.min(60, Math.max(0, Math.round((Date.now() - recordStart) / 1000)));
    if (recordElapsed.value >= 60 && recorder && recording.value) {
      recorder.stop();
    }
  }, 250);
}

function switchMode(next: PublishMode) {
  if (next === "voice" && !supportsVoice.value) {
    uni.showToast({ title: "H5暂不支持语音动态，请在App内发布", icon: "none" });
    return;
  }
  if (mode.value === next) {
    return;
  }
  if (hasMedia()) {
    uni.showModal({
      title: "切换类型",
      content: "切换后会清空当前已选媒体，确认继续？",
      confirmColor: "#ff5878",
      success: ({ confirm }) => {
        if (confirm) {
          clearMedia();
          mode.value = next;
        }
      }
    });
    return;
  }
  mode.value = next;
}

function hasMedia() {
  return Boolean(images.value.length || videoPath.value || voicePath.value || recording.value);
}

function clearMedia() {
  images.value = [];
  videoPath.value = "";
  videoThumb.value = "";
  voicePath.value = "";
  voiceSeconds.value = 0;
  recordElapsed.value = 0;
  if (recording.value && recorder) {
    recorder.stop();
  }
  recording.value = false;
  stopRecordTimer();
}

function resetDraft() {
  if (!text.value && !hasMedia()) {
    return;
  }
  uni.showModal({
    title: "清空内容",
    content: "确认清空当前编辑内容？",
    confirmColor: "#ff5878",
    success: ({ confirm }) => {
      if (!confirm) {
        return;
      }
      text.value = "";
      clearMedia();
      statusText.value = "";
      progress.value = 0;
    }
  });
}

function chooseImages() {
  if (!requireLogin() || publishing.value || recording.value) {
    return;
  }
  mode.value = "image";
  uni.chooseImage({
    count: Math.max(1, 9 - images.value.length),
    sizeType: ["compressed"],
    sourceType: ["album", "camera"],
    success: (res) => {
      videoPath.value = "";
      videoThumb.value = "";
      voicePath.value = "";
      voiceSeconds.value = 0;
      images.value = images.value.concat(res.tempFilePaths).slice(0, 9);
    }
  });
}

function chooseVideo() {
  if (!requireLogin() || publishing.value || recording.value) {
    return;
  }
  mode.value = "video";
  uni.chooseVideo({
    compressed: true,
    sourceType: ["album", "camera"],
    success: (res) => {
      images.value = [];
      voicePath.value = "";
      voiceSeconds.value = 0;
      videoPath.value = res.tempFilePath;
      videoThumb.value = String((res as any).thumbTempFilePath || "");
    }
  });
}

function toggleRecord() {
  if (!requireLogin() || publishing.value) {
    return;
  }
  if (!supportsVoice.value) {
    uni.showToast({ title: "H5暂不支持语音动态，请在App内发布", icon: "none" });
    return;
  }
  mode.value = "voice";
  const manager = ensureRecorder();
  if (!manager) {
    return;
  }
  if (recording.value) {
    manager.stop();
    return;
  }
  images.value = [];
  videoPath.value = "";
  videoThumb.value = "";
  voicePath.value = "";
  voiceSeconds.value = 0;
  recordStart = Date.now();
  recording.value = true;
  startRecordTimer();
  manager.start({
    duration: 60000,
    format: "mp3"
  });
}

function removeImage(index: number) {
  images.value.splice(index, 1);
}

function clearVideo() {
  videoPath.value = "";
  videoThumb.value = "";
}

function clearVoice() {
  voicePath.value = "";
  voiceSeconds.value = 0;
  recordElapsed.value = 0;
}

function previewImage(current: string) {
  uni.previewImage({ current, urls: images.value });
}

function uploadKey(result: UploadResult) {
  return result.file || result.file_name || result.filepath || result.url || "";
}

async function uploadFiles(paths: string[], label: string) {
  const uploaded: string[] = [];
  for (let index = 0; index < paths.length; index += 1) {
    const path = paths[index];
    if (!path) {
      continue;
    }
    statusText.value = `${label} ${index + 1}/${paths.length}`;
    progress.value = Math.round((index / Math.max(1, paths.length)) * 80);
    const key = uploadKey(await uploadOne(path));
    if (key) {
      uploaded.push(key);
    }
  }
  progress.value = 86;
  return uploaded;
}

function markDynamicDirty() {
  try {
    uni.setStorageSync(DYNAMIC_DIRTY_KEY, "1");
  } catch {
    // Ignore storage failures; publish success is still valid.
  }
}

async function publish() {
  if (!requireLogin() || !canPublish.value || publishing.value || recording.value) {
    return;
  }
  if (mode.value === "voice" && !supportsVoice.value) {
    uni.showToast({ title: "H5暂不支持语音动态，请在App内发布", icon: "none" });
    return;
  }
  publishing.value = true;
  progress.value = 4;
  statusText.value = "准备发布";
  try {
    if (mode.value === "image" && images.value.length) {
      const uploaded = await uploadFiles(images.value, "上传图片");
      await publishDynamic({
        type: ACTIVE_TYPES.image,
        text: text.value.trim(),
        images: uploaded.join(";")
      });
    } else if (mode.value === "video" && videoPath.value) {
      const uploaded = await uploadFiles([videoPath.value], "上传视频");
      const cover = videoThumb.value ? (await uploadFiles([videoThumb.value], "上传封面"))[0] || "" : "";
      await publishDynamic({
        type: ACTIVE_TYPES.video,
        text: text.value.trim(),
        videoUrl: uploaded[0] || "",
        videoImage: cover
      });
    } else if (mode.value === "voice" && voicePath.value) {
      const uploaded = await uploadFiles([voicePath.value], "上传语音");
      await publishDynamic({
        type: ACTIVE_TYPES.voice,
        text: text.value.trim(),
        voiceUrl: uploaded[0] || "",
        voiceDuration: voiceSeconds.value
      });
    } else {
      await publishDynamic({ type: ACTIVE_TYPES.text, text: text.value.trim() });
    }
    progress.value = 100;
    statusText.value = "发布成功";
    markDynamicDirty();
    uni.showToast({ title: "发布成功", icon: "success" });
    setTimeout(() => uni.navigateBack(), 450);
  } catch (error: any) {
    progress.value = 0;
    statusText.value = "";
    uni.showToast({ title: error?.message || "发布失败", icon: "none" });
  } finally {
    publishing.value = false;
  }
}

onShow(() => {
  if (!requireLogin()) {
    return;
  }
  if (mode.value === "voice" && !supportsVoice.value) {
    clearMedia();
    mode.value = "image";
  }
});

onUnmounted(() => {
  if (recording.value && recorder) {
    recorder.stop();
  }
  stopRecordTimer();
});
</script>

<style scoped>
.publish-page {
  min-height: 100vh;
  color: var(--ink);
  background: var(--bg);
}

.publish-shell {
  min-height: 100vh;
  padding: 24rpx 28rpx calc(36rpx + env(safe-area-inset-bottom));
}

.top-panel {
  padding: 26rpx;
  border: 1rpx solid #e9edf4;
  border-radius: 20rpx;
  background: #fff;
  box-shadow: 0 8rpx 22rpx rgba(35, 45, 70, 0.04);
}

.author-row {
  display: flex;
  align-items: center;
  gap: 18rpx;
}

.avatar {
  width: 74rpx;
  height: 74rpx;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--line);
}

.author-main {
  flex: 1;
  min-width: 0;
}

.author-name {
  display: block;
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.author-sub {
  display: block;
  margin-top: 10rpx;
  color: var(--ink-3);
  font-size: 23rpx;
}

.draft-button {
  display: flex;
  width: 94rpx;
  height: 52rpx;
  align-items: center;
  justify-content: center;
  border-radius: 26rpx;
  color: var(--brand);
  font-size: 24rpx;
  font-weight: 900;
  background: #fff2f5;
}

.editor {
  width: 100%;
  min-height: 254rpx;
  margin-top: 24rpx;
  color: var(--ink);
  font-size: 31rpx;
  line-height: 1.55;
}

.editor-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 14rpx;
  color: #9aa3b3;
  font-size: 23rpx;
}

.mode-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16rpx;
  margin: 22rpx 0;
}

.mode-strip.two {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.mode-button {
  display: flex;
  height: 88rpx;
  align-items: center;
  justify-content: center;
  gap: 12rpx;
  border: 2rpx solid #ebedf3;
  border-radius: 18rpx;
  color: var(--ink-2);
  font-size: 26rpx;
  font-weight: 900;
  background: #fff;
}

.mode-button.active {
  border-color: var(--brand);
  color: var(--brand);
  background: #fff5f7;
}

.mode-icon {
  display: flex;
  width: 42rpx;
  height: 42rpx;
  align-items: center;
  justify-content: center;
  border-radius: 14rpx;
  color: #fff;
  font-size: 20rpx;
  background: var(--brand);
}

.media-panel {
  padding: 24rpx;
  border: 1rpx solid #e9edf4;
  border-radius: 20rpx;
  background: #fff;
}

.image-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12rpx;
}

.image-cell,
.add-cell {
  position: relative;
  overflow: hidden;
  aspect-ratio: 1 / 1;
  border-radius: 14rpx;
  background: #f1f3f7;
}

.preview-image {
  width: 100%;
  height: 100%;
}

.remove-button {
  position: absolute;
  top: 8rpx;
  right: 8rpx;
  display: flex;
  width: 42rpx;
  height: 42rpx;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #fff;
  font-size: 30rpx;
  line-height: 1;
  background: rgba(0, 0, 0, 0.55);
}

.add-cell,
.large-picker {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  border: 2rpx dashed #d9deea;
  color: var(--ink-3);
  font-size: 24rpx;
  font-weight: 800;
}

.add-cell text:first-child,
.large-plus {
  margin-bottom: 12rpx;
  color: var(--brand);
  font-size: 48rpx;
  line-height: 1;
}

.large-picker {
  width: 100%;
  height: 320rpx;
  border-radius: 18rpx;
  background: #f8fafc;
}

.video-card {
  overflow: hidden;
  border-radius: 18rpx;
  background: #f8fafc;
}

.video-preview {
  display: block;
  width: 100%;
  height: 390rpx;
  background: #111;
}

.voice-card {
  padding: 24rpx;
  border-radius: 18rpx;
  background: #f8fafc;
}

.voice-state {
  display: flex;
  align-items: center;
  gap: 18rpx;
  min-height: 112rpx;
}

.voice-dot {
  width: 52rpx;
  height: 52rpx;
  border-radius: 50%;
  background: #98a2b3;
}

.voice-state.recording .voice-dot {
  background: var(--brand);
  box-shadow: 0 0 0 14rpx rgba(255, 88, 120, 0.12);
}

.voice-main {
  flex: 1;
  min-width: 0;
}

.voice-main text:first-child {
  display: block;
  color: var(--ink);
  font-size: 30rpx;
  font-weight: 900;
}

.voice-main text:last-child {
  display: block;
  margin-top: 12rpx;
  color: var(--ink-3);
  font-size: 24rpx;
}

.media-actions {
  display: flex;
  gap: 16rpx;
  margin-top: 18rpx;
}

.media-actions button {
  display: flex;
  flex: 1;
  min-width: 0;
  height: 72rpx;
  align-items: center;
  justify-content: center;
  border-radius: 36rpx;
  color: var(--brand);
  font-size: 25rpx;
  font-weight: 900;
  background: #fff2f5;
}

.media-actions .danger {
  color: #d92d20;
  background: #fff1f0;
}

.media-actions .record-button.recording {
  color: #fff;
  background: var(--brand);
}

.media-tip {
  display: block;
  margin-top: 18rpx;
  color: #98a2b3;
  font-size: 23rpx;
  line-height: 1.45;
}

.status-row {
  margin-top: 22rpx;
  padding: 20rpx 22rpx;
  border-radius: 18rpx;
  color: var(--ink-2);
  font-size: 24rpx;
  background: #fff;
}

.status-bar {
  overflow: hidden;
  height: 10rpx;
  margin-bottom: 16rpx;
  border-radius: 10rpx;
  background: var(--line);
}

.status-fill {
  height: 100%;
  border-radius: 10rpx;
  background: linear-gradient(90deg, var(--brand), #ff8a4d);
  transition: width 0.18s ease;
}

.publish-submit {
  margin-top: 28rpx;
}
</style>
