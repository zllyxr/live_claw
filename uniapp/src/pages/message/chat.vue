<template>
  <view class="chat-page">
    <view class="chat-nav">
      <button class="nav-button nav-back" @tap="goBack">‹</button>
      <view class="nav-peer">
        <image class="nav-avatar" :src="avatar" mode="aspectFill" />
        <view class="nav-title">
          <text>{{ name }}</text>
          <text v-if="kind === 'group'">群聊</text>
        </view>
      </view>
      <button class="nav-button nav-more" @tap="openMore">•••</button>
    </view>

    <scroll-view
      class="message-scroll"
      scroll-y
      :scroll-top="scrollTop"
      :scroll-into-view="scrollIntoView"
      upper-threshold="80"
      @scrolltoupper="loadOlder"
    >
      <view class="message-list">
        <view v-if="loadingHistory" class="history-state">正在加载聊天记录...</view>
        <view v-else-if="historyEnd && messages.length" class="history-state">没有更早的消息了</view>

        <view v-for="message in messages" :id="messageDomID(message)" :key="messageKey(message)">
          <view v-if="message.system" class="system-message">
            <text>{{ message.content || "群聊信息已更新" }}</text>
          </view>
          <view
            v-else
            class="bubble-row"
            :class="{ self: isSelf(message) }"
            @longpress="messageActions(message)"
          >
            <image class="avatar" :src="messageAvatar(message)" mode="aspectFill" />
            <view class="message-main">
              <text v-if="kind === 'group' && !isSelf(message)" class="sender-name">
                {{ message.sender_name || `用户${message.uid || ""}` }}
              </text>
              <view class="bubble">
                <image
                  v-if="message.image"
                  class="chat-image"
                  :src="absolutizeUrl(message.image)"
                  mode="aspectFill"
                  @tap="previewImage(message.image)"
                />
                <view v-else-if="message.voice" class="voice-message" @tap="playVoice(message)">
                  <text>▶</text>
                  <text>{{ message.voice_duration || 0 }}″ 语音</text>
                </view>
                <video
                  v-else-if="message.video"
                  class="chat-video"
                  :src="absolutizeUrl(message.video)"
                  :poster="absolutizeUrl(message.video_cover || '')"
                  controls
                />
                <view v-else-if="message.file" class="file-message" @tap="openFile(message)">
                  <text class="file-icon">文</text>
                  <view>
                    <text class="file-name">{{ message.file_name || "聊天文件" }}</text>
                    <text class="file-size">{{ fileSize(message.file_size) }}</text>
                  </view>
                </view>
                <text v-else>{{ message.content || "" }}</text>
              </view>
              <text class="message-time">{{ messageTime(message) }}</text>
            </view>
          </view>
        </view>

        <view v-if="!loadingHistory && !messages.length" class="empty-chat">
          <text>{{ kind === "group" ? "群聊已创建" : "开始聊天吧" }}</text>
          <text>请文明交流，谨防陌生人诈骗。</text>
        </view>
      </view>
    </scroll-view>

    <view class="composer">
      <button class="attach-button" :disabled="sending" @tap="openAttachmentMenu">＋</button>
      <button class="voice-button" :class="{ recording }" :disabled="sending" @tap="toggleRecording">
        {{ recording ? "停" : "录" }}
      </button>
      <input
        v-model.trim="draft"
        class="composer-input"
        placeholder="输入消息"
        confirm-type="send"
        @confirm="sendText"
      />
      <button class="send-button" :disabled="!draft || sending" @tap="sendText">发送</button>
    </view>
    <view v-if="recording" class="recording-tip">正在录音，再点一次发送</view>
  </view>
</template>

<script setup lang="ts">
import { nextTick, ref } from "vue";
import { onLoad, onUnload } from "@dcloudio/uni-app";
import {
  deleteLocalChatMessage,
  getChatHistory,
  getUserHome,
  markConversationRead,
  removeConversation,
  reportLiveUser,
  revokeChatMessage,
  sendFileMessage,
  sendImageMessage,
  sendTextMessage,
  sendVideoMessage,
  sendVoiceMessage,
  setBlack,
  uploadOne
} from "@/api/services";
import type { ChatMessage } from "@/types/api";
import { getSession, requireLogin } from "@/utils/session";
import { absolutizeUrl } from "@/utils/url";
import { onOpenIMMessage, pickOpenIMLocalFile, type ChatKind } from "@/utils/openim";

const targetID = ref("");
const kind = ref<ChatKind>("single");
const name = ref("聊天");
const avatar = ref("/static/brand/icon-round.webp");
const draft = ref("");
const messages = ref<ChatMessage[]>([]);
const sending = ref(false);
const recording = ref(false);
const loadingHistory = ref(false);
const historyEnd = ref(false);
const scrollTop = ref(0);
const scrollIntoView = ref("");
const blacked = ref(false);
let stopMessages: (() => void) | undefined;
let audio: UniApp.InnerAudioContext | undefined;
let recorder: UniApp.RecorderManager | undefined;
let recordStartedAt = 0;
let discardRecording = false;

onLoad((query) => {
  kind.value = String(query?.kind || "") === "group" || query?.groupid ? "group" : "single";
  targetID.value = String(query?.target || query?.groupid || query?.touid || "");
  name.value = decodeURIComponent(String(query?.name || (kind.value === "group" ? "群聊" : "私信")));
  avatar.value =
    decodeURIComponent(String(query?.avatar || "")) ||
    "/static/brand/icon-round.webp";
  stopMessages = onOpenIMMessage((message) => {
    appendMessage(message);
    scrollToBottom();
    void markConversationRead(targetID.value, kind.value).catch(() => undefined);
  }, targetID.value, kind.value);
  void loadInitial();
  if (kind.value === "single") {
    void getUserHome(targetID.value)
      .then((profile) => {
        blacked.value = Number(profile?.isblack || 0) === 1;
      })
      .catch(() => undefined);
  }
  setupRecorder();
});

onUnload(() => {
  stopMessages?.();
  stopMessages = undefined;
  audio?.destroy();
  audio = undefined;
  if (recording.value) {
    discardRecording = true;
    recorder?.stop();
  }
  recorder = undefined;
});

function messageKey(message: ChatMessage) {
  return String(message.client_msg_id || message.id || `${message.addtime}-${message.uid}-${message.content}`);
}

function messageDomID(message: ChatMessage) {
  return `msg-${messageKey(message).replace(/[^A-Za-z0-9_-]/g, "")}`;
}

function appendMessage(message: ChatMessage) {
  const key = messageKey(message);
  if (messages.value.some((item) => messageKey(item) === key)) {
    return;
  }
  messages.value.push(message);
}

function isSelf(message: ChatMessage) {
  const uid = getSession().uid;
  return String(message.uid || message.from_uid || "") === uid || Boolean(message.is_self);
}

function messageAvatar(message: ChatMessage) {
  if (isSelf(message)) {
    const user = getSession().user;
    return absolutizeUrl(String(user?.avatar_thumb || user?.avatar || "")) || "/static/brand/icon-round.webp";
  }
  return (
    absolutizeUrl(String(message.sender_avatar || message.avatar_thumb || message.avatar || avatar.value)) ||
    "/static/brand/icon-round.webp"
  );
}

function messageTime(message: ChatMessage) {
  const numeric = Number(message.addtime || 0);
  if (!Number.isFinite(numeric) || numeric <= 0) {
    return "";
  }
  const date = new Date(numeric < 1_000_000_000_000 ? numeric * 1000 : numeric);
  return `${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

async function loadInitial() {
  if (!requireLogin() || !targetID.value) {
    return;
  }
  loadingHistory.value = true;
  try {
    const result = await getChatHistory(targetID.value, kind.value, "", 30);
    messages.value = result.messages.slice().reverse();
    historyEnd.value = result.isEnd;
    void markConversationRead(targetID.value, kind.value).catch(() => undefined);
    scrollToBottom();
  } catch (error: any) {
    uni.showToast({ title: error?.message || "聊天加载失败", icon: "none" });
  } finally {
    loadingHistory.value = false;
  }
}

async function loadOlder() {
  if (loadingHistory.value || historyEnd.value || !messages.value.length) {
    return;
  }
  const first = messages.value[0];
  if (!first) {
    historyEnd.value = true;
    return;
  }
  const cursor = String(first.client_msg_id || first.id || "");
  if (!cursor) {
    historyEnd.value = true;
    return;
  }
  loadingHistory.value = true;
  const anchorID = messageDomID(first);
  try {
    const result = await getChatHistory(targetID.value, kind.value, cursor, 30);
    const existing = new Set(messages.value.map(messageKey));
    const older = result.messages
      .slice()
      .reverse()
      .filter((message) => !existing.has(messageKey(message)));
    messages.value = older.concat(messages.value);
    historyEnd.value = result.isEnd || !older.length;
    await nextTick();
    scrollIntoView.value = anchorID;
  } catch (error: any) {
    uni.showToast({ title: error?.message || "记录加载失败", icon: "none" });
  } finally {
    loadingHistory.value = false;
  }
}

function scrollToBottom() {
  scrollIntoView.value = "";
  setTimeout(() => {
    scrollTop.value += 99999;
  }, 80);
}

async function sendText() {
  if (!draft.value || sending.value || !targetID.value) {
    return;
  }
  const content = draft.value;
  draft.value = "";
  sending.value = true;
  try {
    appendMessage(await sendTextMessage(targetID.value, content, kind.value));
    scrollToBottom();
  } catch (error: any) {
    draft.value = content;
    uni.showToast({ title: error?.message || "发送失败", icon: "none" });
  } finally {
    sending.value = false;
  }
}

function uploadedSource(uploaded: any, fallback = "") {
  return String(uploaded.file || uploaded.file_name || uploaded.filepath || uploaded.url || fallback);
}

function openAttachmentMenu() {
  uni.showActionSheet({
    itemList: ["发送图片", "发送视频", "发送文件"],
    success: ({ tapIndex }) => {
      if (tapIndex === 0) {
        chooseImage();
      } else if (tapIndex === 1) {
        chooseVideo();
      } else if (tapIndex === 2) {
        chooseFile();
      }
    }
  });
}

function setupRecorder() {
  try {
    recorder = uni.getRecorderManager();
    recorder.onStop(async (result) => {
      recording.value = false;
      if (discardRecording) {
        discardRecording = false;
        return;
      }
      const filePath = String(result.tempFilePath || "");
      if (!filePath) {
        return;
      }
      sending.value = true;
      try {
        const uploaded = await uploadOne(filePath);
        const duration = Math.max(1, Math.round((Date.now() - recordStartedAt) / 1000));
        appendMessage(
          await sendVoiceMessage(
            targetID.value,
            uploadedSource(uploaded, filePath),
            duration,
            Number((result as any).fileSize || 0),
            kind.value
          )
        );
        scrollToBottom();
      } catch (error: any) {
        uni.showToast({ title: error?.message || "语音发送失败", icon: "none" });
      } finally {
        sending.value = false;
      }
    });
    recorder.onError((error: any) => {
      recording.value = false;
      uni.showToast({ title: error?.errMsg || "录音失败，请检查麦克风权限", icon: "none" });
    });
  } catch {
    recorder = undefined;
  }
}

function toggleRecording() {
  if (!recorder) {
    uni.showToast({ title: "当前设备暂不支持录音", icon: "none" });
    return;
  }
  if (recording.value) {
    recorder.stop();
    return;
  }
  recordStartedAt = Date.now();
  discardRecording = false;
  recording.value = true;
  recorder.start({ duration: 60_000, format: "mp3" });
}

function chooseImage() {
  uni.chooseImage({
    count: 1,
    success: async (res) => {
      const filePath = res.tempFilePaths[0];
      if (!filePath) {
        return;
      }
      sending.value = true;
      try {
        const uploaded = await uploadOne(filePath);
        appendMessage(await sendImageMessage(targetID.value, uploadedSource(uploaded, filePath), kind.value));
        scrollToBottom();
      } catch (error: any) {
        uni.showToast({ title: error?.message || "图片发送失败", icon: "none" });
      } finally {
        sending.value = false;
      }
    }
  });
}

function chooseVideo() {
  uni.chooseVideo({
    sourceType: ["album", "camera"],
    compressed: true,
    success: async (res: any) => {
      const filePath = String(res.tempFilePath || "");
      if (!filePath) {
        return;
      }
      sending.value = true;
      try {
        const uploaded = await uploadOne(filePath);
        appendMessage(
          await sendVideoMessage(
            targetID.value,
            uploadedSource(uploaded, filePath),
            "",
            Number(res.duration || 0),
            Number(res.size || 0),
            kind.value
          )
        );
        scrollToBottom();
      } catch (error: any) {
        uni.showToast({ title: error?.message || "视频发送失败", icon: "none" });
      } finally {
        sending.value = false;
      }
    }
  });
}

function chooseFile() {
  const choose = (uni as any).chooseFile || (uni as any).chooseMessageFile;
  if (typeof choose !== "function") {
    void pickOpenIMLocalFile()
      .then((filePath) => {
        if (filePath) {
          return sendSelectedFile(filePath, filePath.split("/").pop() || "聊天文件", 0);
        }
      })
      .catch(() => {
        uni.showToast({ title: "当前设备暂不支持选择文件", icon: "none" });
      });
    return;
  }
  choose({
    count: 1,
    success: async (res: any) => {
      const file = res.tempFiles?.[0];
      const filePath = String(file?.path || res.tempFilePaths?.[0] || "");
      if (!filePath) {
        return;
      }
      await sendSelectedFile(filePath, String(file?.name || "聊天文件"), Number(file?.size || 0));
    }
  });
}

async function sendSelectedFile(filePath: string, fileName: string, size: number) {
  sending.value = true;
  try {
    const uploaded = await uploadOne(filePath);
    appendMessage(
      await sendFileMessage(
        targetID.value,
        uploadedSource(uploaded, filePath),
        fileName,
        size,
        kind.value
      )
    );
    scrollToBottom();
  } catch (error: any) {
    uni.showToast({ title: error?.message || "文件发送失败", icon: "none" });
  } finally {
    sending.value = false;
  }
}

function previewImage(source: string) {
  const url = absolutizeUrl(source);
  uni.previewImage({ current: url, urls: [url] });
}

function playVoice(message: ChatMessage) {
  if (!message.voice) {
    return;
  }
  audio?.destroy();
  audio = uni.createInnerAudioContext();
  audio.src = absolutizeUrl(message.voice);
  audio.play();
}

function fileSize(size?: number) {
  const value = Number(size || 0);
  if (!value) {
    return "聊天文件";
  }
  return value >= 1024 * 1024
    ? `${(value / 1024 / 1024).toFixed(1)} MB`
    : `${Math.max(1, Math.round(value / 1024))} KB`;
}

function openFile(message: ChatMessage) {
  const url = absolutizeUrl(String(message.file || ""));
  const browserWindow = (globalThis as unknown as { window?: Window }).window;
  if (browserWindow && /^https?:/i.test(url)) {
    browserWindow.open(url, "_blank");
    return;
  }
  uni.downloadFile({
    url,
    success: ({ tempFilePath }) => {
      uni.openDocument({
        filePath: tempFilePath,
        showMenu: true,
        fail: () => uni.showToast({ title: "暂时无法打开该文件", icon: "none" })
      });
    },
    fail: () => uni.showToast({ title: "文件下载失败", icon: "none" })
  });
}

function messageActions(message: ChatMessage) {
  if (message.system || !message.client_msg_id) {
    return;
  }
  const labels = isSelf(message) ? ["撤回消息", "从本机删除"] : ["从本机删除", "举报消息"];
  uni.showActionSheet({
    itemList: labels,
    success: async ({ tapIndex }) => {
      try {
        if (isSelf(message) && tapIndex === 0) {
          await revokeChatMessage(targetID.value, String(message.client_msg_id), kind.value);
        } else if ((isSelf(message) && tapIndex === 1) || (!isSelf(message) && tapIndex === 0)) {
          await deleteLocalChatMessage(targetID.value, String(message.client_msg_id), kind.value);
        } else {
          await reportLiveUser(String(message.uid || targetID.value), `聊天消息举报：${message.content || message.file_name || "媒体消息"}`);
        }
        messages.value = messages.value.filter((item) => messageKey(item) !== messageKey(message));
        uni.showToast({ title: "操作成功", icon: "none" });
      } catch (error: any) {
        uni.showToast({ title: error?.message || "操作失败", icon: "none" });
      }
    }
  });
}

function openMore() {
  if (kind.value === "group") {
    uni.navigateTo({
      url: `/pages/message/group-info?groupid=${encodeURIComponent(targetID.value)}`
    });
    return;
  }
  const labels = ["查看用户主页", blacked.value ? "解除拉黑" : "拉黑用户", "举报用户", "清空聊天记录"];
  uni.showActionSheet({
    itemList: labels,
    success: async ({ tapIndex }) => {
      if (tapIndex === 0) {
        uni.navigateTo({ url: `/pages/user/home?uid=${encodeURIComponent(targetID.value)}` });
        return;
      }
      if (tapIndex === 1) {
        try {
          const result = await setBlack(targetID.value);
          blacked.value = Number(result?.isblack || 0) === 1;
          uni.showToast({ title: blacked.value ? "已拉黑" : "已解除拉黑", icon: "none" });
        } catch (error: any) {
          uni.showToast({ title: error?.message || "操作失败", icon: "none" });
        }
        return;
      }
      if (tapIndex === 2) {
        const reasons = ["骚扰辱骂", "广告引流", "疑似诈骗", "其他原因"];
        uni.showActionSheet({
          itemList: reasons,
          success: ({ tapIndex: reasonIndex }) => {
            void reportLiveUser(targetID.value, reasons[reasonIndex] || "其他原因")
              .then(() => uni.showToast({ title: "已举报", icon: "none" }))
              .catch((error: any) => uni.showToast({ title: error?.message || "举报失败", icon: "none" }));
          }
        });
        return;
      }
      clearConversation();
    }
  });
}

function clearConversation() {
  uni.showModal({
    title: "清空聊天记录",
    content: "确认删除这个会话及本地聊天记录？",
    confirmColor: "#ff4f62",
    success: async ({ confirm }) => {
      if (!confirm) {
        return;
      }
      try {
        await removeConversation(targetID.value, kind.value);
        messages.value = [];
        uni.showToast({ title: "已清空", icon: "none" });
      } catch (error: any) {
        uni.showToast({ title: error?.message || "清空失败", icon: "none" });
      }
    }
  });
}

function goBack() {
  const pages = getCurrentPages();
  if (pages.length > 1) {
    uni.navigateBack();
    return;
  }
  uni.redirectTo({ url: "/pages/message/index" });
}
</script>

<style scoped>
.chat-page {
  position: relative;
  height: 100vh;
  overflow: hidden;
  background: var(--bg);
}

.chat-nav {
  position: fixed;
  top: 0;
  right: 0;
  left: 0;
  z-index: 10;
  display: flex;
  height: calc(var(--status-bar-height) + 96rpx);
  align-items: center;
  padding: var(--status-bar-height) 18rpx 0;
  box-sizing: border-box;
  background: #fff;
  border-bottom: 1rpx solid #edf0f5;
}

.nav-button {
  display: flex;
  flex: 0 0 76rpx;
  width: 76rpx;
  height: 72rpx;
  align-items: center;
  justify-content: center;
  color: var(--ink);
  background: transparent;
}

.nav-back {
  font-size: 50rpx;
  font-weight: 400;
}

.nav-more {
  font-size: 25rpx;
  font-weight: 900;
  letter-spacing: 3rpx;
}

.nav-peer {
  display: flex;
  flex: 1;
  min-width: 0;
  align-items: center;
  justify-content: center;
  gap: 14rpx;
}

.nav-avatar {
  width: 54rpx;
  height: 54rpx;
  border-radius: 16rpx;
  background: #eef0f5;
}

.nav-title {
  min-width: 0;
}

.nav-title text {
  display: block;
  max-width: 390rpx;
  overflow: hidden;
  text-align: center;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.nav-title text:first-child {
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 900;
}

.nav-title text:last-child {
  margin-top: 2rpx;
  color: var(--ink-3);
  font-size: 18rpx;
}

.message-scroll {
  position: fixed;
  top: calc(var(--status-bar-height) + 96rpx);
  right: 0;
  bottom: calc(104rpx + env(safe-area-inset-bottom));
  left: 0;
}

.message-list {
  padding: 20rpx 24rpx 36rpx;
}

.history-state {
  padding: 12rpx 0 24rpx;
  color: #9aa3b1;
  font-size: 21rpx;
  text-align: center;
}

.system-message {
  display: flex;
  justify-content: center;
  margin: 10rpx 0 24rpx;
}

.system-message text {
  max-width: 560rpx;
  padding: 8rpx 16rpx;
  border-radius: 16rpx;
  color: #8992a1;
  font-size: 21rpx;
  line-height: 1.4;
  text-align: center;
  background: rgba(255, 255, 255, 0.75);
}

.bubble-row {
  display: flex;
  align-items: flex-start;
  gap: 14rpx;
  margin-bottom: 22rpx;
}

.bubble-row.self {
  flex-direction: row-reverse;
}

.avatar {
  width: 68rpx;
  height: 68rpx;
  border-radius: 22rpx;
  background: #eef1f5;
}

.message-main {
  display: flex;
  max-width: 540rpx;
  min-width: 0;
  flex-direction: column;
  align-items: flex-start;
}

.self .message-main {
  align-items: flex-end;
}

.sender-name {
  margin: 0 6rpx 7rpx;
  color: #8b94a3;
  font-size: 20rpx;
}

.bubble {
  min-height: 62rpx;
  padding: 18rpx 22rpx;
  border-radius: 6rpx 20rpx 20rpx;
  box-sizing: border-box;
  color: var(--ink);
  font-size: 28rpx;
  line-height: 1.45;
  background: #fff;
}

.self .bubble {
  color: #fff;
  border-radius: 20rpx 6rpx 20rpx 20rpx;
  background: var(--brand);
}

.chat-image {
  display: block;
  width: 280rpx;
  height: 280rpx;
  border-radius: 12rpx;
}

.chat-video {
  display: block;
  width: 360rpx;
  height: 260rpx;
  border-radius: 12rpx;
}

.voice-message {
  display: flex;
  min-width: 190rpx;
  align-items: center;
  gap: 16rpx;
}

.file-message {
  display: flex;
  min-width: 330rpx;
  align-items: center;
  gap: 16rpx;
}

.file-icon {
  display: flex;
  width: 58rpx;
  height: 66rpx;
  align-items: center;
  justify-content: center;
  border-radius: 10rpx;
  color: var(--brand);
  font-size: 22rpx;
  font-weight: 900;
  background: rgba(124, 92, 255, 0.1);
}

.self .file-icon {
  color: #fff;
  background: rgba(255, 255, 255, 0.18);
}

.file-name,
.file-size {
  display: block;
  max-width: 240rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-name {
  font-size: 24rpx;
  font-weight: 900;
}

.file-size {
  margin-top: 5rpx;
  opacity: 0.7;
  font-size: 19rpx;
}

.message-time {
  margin: 6rpx 6rpx 0;
  color: #a3aab6;
  font-size: 18rpx;
}

.empty-chat {
  padding: 160rpx 20rpx 60rpx;
  color: #9aa3b1;
  text-align: center;
}

.empty-chat text {
  display: block;
}

.empty-chat text:first-child {
  color: var(--ink-2);
  font-size: 27rpx;
  font-weight: 900;
}

.empty-chat text:last-child {
  margin-top: 12rpx;
  font-size: 21rpx;
}

.composer {
  position: fixed;
  right: 0;
  bottom: 0;
  left: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 12rpx;
  padding: 16rpx 20rpx calc(16rpx + env(safe-area-inset-bottom));
  background: #fff;
  border-top: 1rpx solid #edf0f5;
}

.attach-button,
.voice-button,
.send-button {
  flex: 0 0 70rpx;
  height: 70rpx;
  border-radius: 35rpx;
  color: var(--brand);
  font-size: 38rpx;
  font-weight: 600;
  background: rgba(124, 92, 255, 0.09);
}

.voice-button {
  flex-basis: 70rpx;
  color: var(--ink-2);
  font-size: 22rpx;
  font-weight: 900;
  background: #eef1f5;
}

.voice-button.recording {
  color: #fff;
  background: #ff5f6d;
}

.send-button {
  flex-basis: 104rpx;
  color: #fff;
  font-size: 25rpx;
  font-weight: 900;
  background: var(--brand);
}

.send-button[disabled],
.attach-button[disabled] {
  opacity: 0.45;
}

.composer-input {
  flex: 1;
  height: 72rpx;
  padding: 0 24rpx;
  border-radius: 36rpx;
  color: var(--ink);
  font-size: 28rpx;
  background: #f3f5f8;
}

.recording-tip {
  position: fixed;
  bottom: calc(122rpx + env(safe-area-inset-bottom));
  left: 50%;
  z-index: 20;
  padding: 12rpx 22rpx;
  border-radius: 28rpx;
  color: #fff;
  font-size: 22rpx;
  font-weight: 800;
  background: rgba(25, 29, 40, 0.84);
  transform: translateX(-50%);
}
</style>
