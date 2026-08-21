<template>
  <view class="chat-page">
    <view class="chat-nav">
      <view class="back-mark" @tap="goBack"></view>
      <view class="nav-peer">
        <view v-if="kind === 'group'" class="nav-group-avatar">
          <view class="nav-person nav-person-a" />
          <view class="nav-person nav-person-b" />
        </view>
        <image v-else class="nav-avatar" :src="avatar" mode="aspectFill" />
        <view class="nav-title">
          <text>{{ name }}</text>
          <view class="peer-status">
            <view class="status-dot" :class="connectionState" />
            <text>{{ peerSubtitle }}</text>
          </view>
        </view>
      </view>
      <view @tap="openMore">···</view>
    </view>
    <scroll-view
      class="message-scroll"
      scroll-y
      :scroll-top="scrollTop"
      :scroll-into-view="scrollIntoView"
      :scroll-with-animation="scrollAnimated"
      upper-threshold="90"
      @scrolltoupper="loadOlder"
    >
      <view class="message-list">
        <button
          v-if="!historyEnd && messages.length"
          class="history-button"
          :disabled="loadingHistory"
          @tap="loadOlder"
        >
          <view v-if="loadingHistory" class="loading-spinner" />
          <text>{{ loadingHistory ? t("social.common.loading") : t("social.chat.earlierMessages") }}</text>
        </button>
        <view v-else-if="historyEnd && messages.length" class="history-end">{{ t("social.chat.historyEnd") }}</view>

        <template v-for="(message, index) in messages" :key="messageKey(message)">
          <view v-if="shouldShowTime(index)" class="time-divider">
            <text>{{ messageDateTime(message) }}</text>
          </view>

          <view :id="messageDomID(message)">
            <view v-if="message.system" class="system-message">
              <text>{{ systemText(message) }}</text>
            </view>

            <view
              v-else
              class="bubble-row"
              :class="{ self: isSelf(message) }"
              @longpress="messageActions(message)"
            >
              <image class="message-avatar" :src="messageAvatar(message)" mode="aspectFill" />
              <view class="message-main">
                <text v-if="kind === 'group' && !isSelf(message)" class="sender-name">
                  {{ message.sender_name || `${t("social.common.user")} ${message.uid || ""}` }}
                </text>
                <view
                  class="bubble"
                  :class="{
                    'media-bubble': message.image || message.video,
                    'file-bubble': message.file
                  }"
                >
                  <image
                    v-if="message.image"
                    class="chat-image"
                    :src="absolutizeUrl(message.image)"
                    mode="aspectFill"
                    @tap.stop="previewImage(message.image)"
                  />
                  <button
                    v-else-if="message.voice"
                    class="voice-message"
                    :class="{ playing: playingMessageID === messageKey(message) }"
                    @tap.stop="playVoice(message)"
                  >
                    <view class="voice-bars">
                      <view />
                      <view />
                      <view />
                    </view>
                    <text>{{ Math.max(1, Number(message.voice_duration || 0)) }}″</text>
                  </button>
                  <video
                    v-else-if="message.video"
                    class="chat-video"
                    :src="absolutizeUrl(message.video)"
                    :poster="absolutizeUrl(message.video_cover || '')"
                    controls
                  />
                  <button
                    v-else-if="message.file"
                    class="file-message"
                    @tap.stop="openFile(message)"
                  >
                    <view class="file-copy">
                      <text class="file-name">{{ message.file_name || t("social.common.chatFile") }}</text>
                      <text class="file-size">{{ fileSize(message.file_size) }}</text>
                    </view>
                    <view class="file-icon">
                      <view class="file-fold" />
                      <text>FILE</text>
                    </view>
                  </button>
                  <text v-else class="message-text">{{ message.content || "" }}</text>
                </view>
                <text class="message-meta">
                  {{ compactTime(message) }}{{ isSelf(message) ? ` · ${t("social.common.sent")}` : "" }}
                </text>
              </view>
            </view>
          </view>
        </template>

        <view v-if="!loadingHistory && !messages.length" class="empty-chat">
          <image src="/static/art/empty/message.webp" mode="aspectFit" />
          <text>{{ kind === "group" ? t("social.chat.groupReady") : t("social.chat.startConversation") }}</text>
          <text>{{ t("social.chat.safetyTip") }}</text>
        </view>
        <view id="chat-tail" class="chat-tail" />
      </view>
    </scroll-view>

    <view v-if="composerNotice" class="composer-notice">
      <view class="notice-lock" />
      <text>{{ composerNotice }}</text>
    </view>

    <view v-if="attachmentMenu && !composerNotice" class="attachment-panel">
      <button class="attachment-item" :disabled="sending" @tap="chooseImage">
        <view class="attachment-icon image-icon"><view /></view>
        <text>{{ t("social.common.image") }}</text>
      </button>
      <button class="attachment-item" :disabled="sending" @tap="chooseVideo">
        <view class="attachment-icon video-icon"><view /></view>
        <text>{{ t("social.common.video") }}</text>
      </button>
      <button class="attachment-item" :disabled="sending" @tap="chooseFile">
        <view class="attachment-icon document-icon"><text>FILE</text></view>
        <text>{{ t("social.common.file") }}</text>
      </button>
    </view>

    <view v-if="!composerNotice" class="composer">
      <button class="composer-tool voice-toggle" :class="{ active: voiceMode }" :disabled="sending" @tap="toggleVoiceMode">
        <view class="mic-mark" />
      </button>
      <button v-if="voiceMode" class="record-button" :class="{ recording }" :disabled="sending" @tap="toggleRecording">
        <view v-if="recording" class="record-pulse" />
        <text>{{ recording ? t("social.chat.finishRecording") : t("social.chat.startRecording") }}</text>
      </button>
      <textarea v-else v-model="draft" class="composer-input" :maxlength="5000" auto-height cursor-spacing="18" :placeholder="t('social.chat.inputPlaceholder')" confirm-type="send" @confirm="sendText"/>

      <button v-if="draft.trim() && !voiceMode" class="send-button" :disabled="sending" @tap="sendText">
        <view v-if="sending" class="loading-spinner white" />
        <text v-else>{{ t("social.common.send") }}</text>
      </button
>
      <button
        v-else
        class="composer-tool add-toggle"
        :class="{ active: attachmentMenu }"
        :disabled="sending || voiceMode"
        @tap="attachmentMenu = !attachmentMenu"
      >
        <text>＋</text>
      </button>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from "vue";
import { onLoad, onUnload } from "@dcloudio/uni-app";
import {
  deleteLocalChatMessage,
  getChatBlackList,
  getChatGroup,
  getChatHistory,
  markConversationRead,
  removeConversation,
  reportLiveUser,
  revokeChatMessage,
  sendFileMessage,
  sendImageMessage,
  sendTextMessage,
  sendVideoMessage,
  sendVoiceMessage,
  setChatBlack,
  uploadOne
} from "@/api/services";
import type { ChatGroup, ChatMessage } from "@/types/api";
import { getSession, requireLogin } from "@/utils/session";
import { absolutizeUrl } from "@/utils/url";
import {
  onIMConnectionState,
  onOpenIMMessage,
  pickOpenIMLocalFile,
  type ChatKind,
  type IMConnectionState
} from "@/utils/openim";
import { useI18n } from "@/i18n";

const { locale, t } = useI18n();

const targetID = ref("");
const kind = ref<ChatKind>("single");
const name = ref(t("social.chat.defaultTitle"));
const avatar = ref("/static/brand/icon-round.webp");
const draft = ref("");
const messages = ref<ChatMessage[]>([]);
const group = ref<ChatGroup>();
const sending = ref(false);
const recording = ref(false);
const voiceMode = ref(false);
const attachmentMenu = ref(false);
const loadingHistory = ref(false);
const historyEnd = ref(false);
const scrollTop = ref(0);
const scrollIntoView = ref("");
const scrollAnimated = ref(false);
const blacked = ref(false);
const connectionState = ref<IMConnectionState>("idle");
const playingMessageID = ref("");
let stopMessages: (() => void) | undefined;
let stopConnection: (() => void) | undefined;
let audio: UniApp.InnerAudioContext | undefined;
let recorder: UniApp.RecorderManager | undefined;
let recordStartedAt = 0;
let discardRecording = false;

const peerSubtitle = computed(() => {
  if (kind.value === "group") {
    const count = Number(group.value?.memberCount || 0);
    return count ? `${count} ${t("social.common.members")}` : t("social.common.groupChat");
  }
  if (connectionState.value === "ready") return t("social.common.realtimeOnline");
  if (connectionState.value === "connecting") return t("social.common.connecting");
  return t("social.chat.autoSync");
});

const composerNotice = computed(() => {
  if (kind.value === "single" && blacked.value) return t("social.chat.blockedNotice");
  if (
    kind.value === "group" &&
    Boolean(group.value?.allMuted) &&
    Number(group.value?.roleLevel || 0) < 60
  ) {
    return t("social.chat.allMutedNotice");
  }
  return "";
});

onLoad((query) => {
  kind.value = String(query?.kind || "") === "group" || query?.groupid ? "group" : "single";
  targetID.value = String(query?.target || query?.groupid || query?.touid || "");
  name.value = decodeURIComponent(
    String(query?.name || (kind.value === "group" ? t("social.common.groupChat") : t("social.chat.privateMessage")))
  );
  avatar.value =
    decodeURIComponent(String(query?.avatar || "")) || "/static/brand/icon-round.webp";

  stopConnection = onIMConnectionState((state) => {
    connectionState.value = state;
  });
  stopMessages = onOpenIMMessage(
    (message) => {
      appendMessage(message);
      scrollToBottom(true);
      void markConversationRead(targetID.value, kind.value).catch(() => undefined);
    },
    targetID.value,
    kind.value
  );

  void loadInitial();
  void loadConversationState();
  setupRecorder();
});

onUnload(() => {
  stopMessages?.();
  stopMessages = undefined;
  stopConnection?.();
  stopConnection = undefined;
  audio?.destroy();
  audio = undefined;
  if (recording.value) {
    discardRecording = true;
    recorder?.stop();
  }
  recorder = undefined;
});

async function loadConversationState() {
  if (kind.value === "group") {
    try {
      group.value = await getChatGroup(targetID.value);
      if (group.value.groupName) name.value = group.value.groupName;
    } catch {
      group.value = undefined;
    }
    return;
  }
  try {
    const blocked = await getChatBlackList(0, 200);
    blacked.value = blocked.some((item) => String(item.userID) === targetID.value);
  } catch {
    blacked.value = false;
  }
}

function messageKey(message: ChatMessage) {
  return String(
    message.server_msg_id ||
      message.client_msg_id ||
      message.id ||
      `${message.addtime}-${message.uid}-${message.content}`
  );
}

function messageDomID(message: ChatMessage) {
  return `msg-${messageKey(message).replace(/[^A-Za-z0-9_-]/g, "")}`;
}

function messageTimestamp(message: ChatMessage) {
  const numeric = Number(message.addtime || 0);
  if (!Number.isFinite(numeric) || numeric <= 0) return 0;
  return numeric < 1_000_000_000_000 ? numeric * 1000 : numeric;
}

function sortMessages() {
  messages.value.sort((left, right) => {
    const leftSeq = Number(left.sequence || 0);
    const rightSeq = Number(right.sequence || 0);
    if (leftSeq && rightSeq) return leftSeq - rightSeq;
    return messageTimestamp(left) - messageTimestamp(right);
  });
}

function appendMessage(message: ChatMessage) {
  const key = messageKey(message);
  if (messages.value.some((item) => messageKey(item) === key)) return;
  messages.value.push(message);
  sortMessages();
}

function isSelf(message: ChatMessage) {
  const uid = getSession().uid;
  return String(message.uid || message.from_uid || "") === uid || Boolean(message.is_self);
}

function messageAvatar(message: ChatMessage) {
  if (isSelf(message)) {
    const user = getSession().user;
    return (
      absolutizeUrl(String(user?.avatar_thumb || user?.avatar || "")) ||
      "/static/brand/icon-round.webp"
    );
  }
  return (
    absolutizeUrl(
      String(message.sender_avatar || message.avatar_thumb || message.avatar || avatar.value)
    ) || "/static/brand/icon-round.webp"
  );
}

function compactTime(message: ChatMessage) {
  const timestamp = messageTimestamp(message);
  if (!timestamp) return "";
  const date = new Date(timestamp);
  return `${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

const englishShortMonths = [
  "Jan", "Feb", "Mar", "Apr", "May", "Jun",
  "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"
];

function localizedMonthDay(date: Date) {
  const month = date.getMonth() + 1;
  const day = date.getDate();
  if (locale.value === "en") return `${englishShortMonths[date.getMonth()]} ${day}`;
  if (locale.value === "ko") return `${month}월 ${day}일`;
  return `${month}月${day}日`;
}

function messageDateTime(message: ChatMessage) {
  const timestamp = messageTimestamp(message);
  if (!timestamp) return "";
  const date = new Date(timestamp);
  const now = new Date();
  const time = compactTime(message);
  if (date.toDateString() === now.toDateString()) return time;
  const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1);
  if (date.toDateString() === yesterday.toDateString()) return `${t("social.chat.yesterday")} ${time}`;
  return `${localizedMonthDay(date)} ${time}`;
}

function shouldShowTime(index: number) {
  if (index === 0) return true;
  const currentMessage = messages.value[index];
  const previousMessage = messages.value[index - 1];
  if (!currentMessage || !previousMessage) return true;
  const current = messageTimestamp(currentMessage);
  const previous = messageTimestamp(previousMessage);
  return !current || !previous || current - previous >= 5 * 60_000;
}

function systemText(message: ChatMessage) {
  const content = String(message.content || "").trim();
  if (!content) return t("social.chat.groupUpdated");
  try {
    const parsed = JSON.parse(content) as Record<string, unknown>;
    return String(parsed.text || parsed.message || parsed.title || t("social.chat.groupUpdated"));
  } catch {
    return content;
  }
}

async function loadInitial() {
  if (!requireLogin() || !targetID.value) return;
  loadingHistory.value = true;
  try {
    const result = await getChatHistory(targetID.value, kind.value, "", 30);
    messages.value = result.messages.slice().reverse();
    sortMessages();
    historyEnd.value = result.isEnd;
    void markConversationRead(targetID.value, kind.value).catch(() => undefined);
    await nextTick();
    scrollToBottom(false);
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("social.chat.loadFailed"), icon: "none" });
  } finally {
    loadingHistory.value = false;
  }
}

async function loadOlder() {
  if (loadingHistory.value || historyEnd.value || !messages.value.length) return;
  const first = messages.value[0];
  const cursor = String(first?.server_msg_id || first?.client_msg_id || first?.id || "");
  if (!cursor) {
    historyEnd.value = true;
    return;
  }
  if (!first) return;
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
    sortMessages();
    historyEnd.value = result.isEnd || !older.length;
    await nextTick();
    scrollAnimated.value = false;
    scrollIntoView.value = anchorID;
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("social.chat.historyLoadFailed"), icon: "none" });
  } finally {
    loadingHistory.value = false;
  }
}

function scrollToBottom(animated = true) {
  scrollIntoView.value = "";
  scrollAnimated.value = animated;
  nextTick(() => {
    scrollIntoView.value = "chat-tail";
    scrollTop.value += 99999;
  });
}

async function sendText() {
  const content = draft.value.trim();
  if (!content || sending.value || !targetID.value || composerNotice.value) return;
  draft.value = "";
  attachmentMenu.value = false;
  sending.value = true;
  try {
    appendMessage(await sendTextMessage(targetID.value, content, kind.value));
    scrollToBottom(true);
  } catch (error: any) {
    draft.value = content;
    uni.showToast({ title: error?.message || t("social.common.sendFailed"), icon: "none" });
  } finally {
    sending.value = false;
  }
}

function uploadedSource(uploaded: any, fallback = "") {
  return String(uploaded.file || uploaded.file_name || uploaded.filepath || uploaded.url || fallback);
}

function toggleVoiceMode() {
  if (recording.value) return;
  voiceMode.value = !voiceMode.value;
  attachmentMenu.value = false;
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
      if (!filePath) return;
      const duration = Math.max(1, Math.round((Date.now() - recordStartedAt) / 1000));
      if (duration < 1) {
        uni.showToast({ title: t("social.chat.recordingTooShort"), icon: "none" });
        return;
      }
      sending.value = true;
      try {
        const uploaded = await uploadOne(filePath);
        appendMessage(
          await sendVoiceMessage(
            targetID.value,
            uploadedSource(uploaded, filePath),
            duration,
            Number((result as any).fileSize || 0),
            kind.value
          )
        );
        scrollToBottom(true);
      } catch (error: any) {
        uni.showToast({ title: error?.message || t("social.chat.voiceSendFailed"), icon: "none" });
      } finally {
        sending.value = false;
      }
    });
    recorder.onError((error: any) => {
      recording.value = false;
      uni.showToast({ title: error?.errMsg || t("social.chat.recordFailedPermission"), icon: "none" });
    });
  } catch {
    recorder = undefined;
  }
}

function toggleRecording() {
  if (!recorder) {
    uni.showToast({ title: t("social.chat.recordUnsupported"), icon: "none" });
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
  attachmentMenu.value = false;
  uni.chooseImage({
    count: 1,
    sourceType: ["album", "camera"],
    success: async (result) => {
      const filePath = result.tempFilePaths[0];
      if (!filePath) return;
      sending.value = true;
      try {
        const uploaded = await uploadOne(filePath);
        appendMessage(
          await sendImageMessage(
            targetID.value,
            uploadedSource(uploaded, filePath),
            kind.value
          )
        );
        scrollToBottom(true);
      } catch (error: any) {
        uni.showToast({ title: error?.message || t("social.chat.imageSendFailed"), icon: "none" });
      } finally {
        sending.value = false;
      }
    }
  });
}

function chooseVideo() {
  attachmentMenu.value = false;
  uni.chooseVideo({
    sourceType: ["album", "camera"],
    compressed: true,
    success: async (result: any) => {
      const filePath = String(result.tempFilePath || "");
      if (!filePath) return;
      sending.value = true;
      try {
        const uploaded = await uploadOne(filePath);
        appendMessage(
          await sendVideoMessage(
            targetID.value,
            uploadedSource(uploaded, filePath),
            "",
            Number(result.duration || 0),
            Number(result.size || 0),
            kind.value
          )
        );
        scrollToBottom(true);
      } catch (error: any) {
        uni.showToast({ title: error?.message || t("social.chat.videoSendFailed"), icon: "none" });
      } finally {
        sending.value = false;
      }
    }
  });
}

function chooseFile() {
  attachmentMenu.value = false;
  const choose = (uni as any).chooseFile || (uni as any).chooseMessageFile;
  if (typeof choose !== "function") {
    void pickOpenIMLocalFile()
      .then((filePath) => {
        if (filePath) {
          return sendSelectedFile(filePath, filePath.split("/").pop() || t("social.common.chatFile"), 0);
        }
        uni.showToast({ title: t("social.chat.fileSelectionUnsupported"), icon: "none" });
      })
      .catch(() => uni.showToast({ title: t("social.chat.fileSelectionUnsupported"), icon: "none" }));
    return;
  }
  choose({
    count: 1,
    success: async (result: any) => {
      const file = result.tempFiles?.[0];
      const filePath = String(file?.path || result.tempFilePaths?.[0] || "");
      if (!filePath) return;
      await sendSelectedFile(
        filePath,
        String(file?.name || filePath.split("/").pop() || t("social.common.chatFile")),
        Number(file?.size || 0)
      );
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
    scrollToBottom(true);
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("social.chat.fileSendFailed"), icon: "none" });
  } finally {
    sending.value = false;
  }
}

function previewImage(source: string) {
  const url = absolutizeUrl(source);
  const urls = messages.value
    .map((message) => absolutizeUrl(String(message.image || "")))
    .filter(Boolean);
  uni.previewImage({ current: url, urls: urls.length ? urls : [url] });
}

function playVoice(message: ChatMessage) {
  if (!message.voice) return;
  const key = messageKey(message);
  if (playingMessageID.value === key) {
    audio?.stop();
    playingMessageID.value = "";
    return;
  }
  audio?.destroy();
  audio = uni.createInnerAudioContext();
  audio.src = absolutizeUrl(message.voice);
  audio.onEnded(() => {
    playingMessageID.value = "";
  });
  audio.onError(() => {
    playingMessageID.value = "";
    uni.showToast({ title: t("social.chat.voicePlayFailed"), icon: "none" });
  });
  playingMessageID.value = key;
  audio.play();
}

function fileSize(size?: number) {
  const value = Number(size || 0);
  if (!value) return t("social.chat.unknownSize");
  if (value >= 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`;
  return `${Math.max(1, Math.round(value / 1024))} KB`;
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
        fail: () => uni.showToast({ title: t("social.chat.cannotOpenFile"), icon: "none" })
      });
    },
    fail: () => uni.showToast({ title: t("social.chat.fileDownloadFailed"), icon: "none" })
  });
}

function messageActions(message: ChatMessage) {
  const messageID = String(message.server_msg_id || message.id || "");
  if (message.system || !messageID) return;
  const own = isSelf(message);
  const labels = own
    ? [t("social.chat.revokeMessage"), t("social.chat.deleteLocal")]
    : [t("social.chat.deleteLocal"), t("social.chat.reportMessage")];
  uni.showActionSheet({
    itemList: labels,
    success: async ({ tapIndex }) => {
      try {
        if (own && tapIndex === 0) {
          await revokeChatMessage(targetID.value, messageID, kind.value);
        } else if ((own && tapIndex === 1) || (!own && tapIndex === 0)) {
          await deleteLocalChatMessage(targetID.value, messageID, kind.value);
        } else {
          await reportLiveUser(
            String(message.uid || targetID.value),
            `${t("social.chat.messageReportPrefix")}${message.content || message.file_name || t("social.chat.mediaMessage")}`
          );
        }
        messages.value = messages.value.filter((item) => messageKey(item) !== messageKey(message));
        uni.showToast({ title: own && tapIndex === 0 ? t("social.chat.messageRevoked") : t("social.common.operationSucceeded"), icon: "none" });
      } catch (error: any) {
        uni.showToast({ title: error?.message || t("social.common.operationFailed"), icon: "none" });
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
  const labels = [
    t("social.chat.viewUser"),
    blacked.value ? t("social.chat.unblockUser") : t("social.chat.blockUser"),
    t("social.chat.reportUser"),
    t("social.common.hideConversation")
  ];
  uni.showActionSheet({
    itemList: labels,
    success: async ({ tapIndex }) => {
      if (tapIndex === 0) {
        uni.navigateTo({ url: `/pages/user/home?uid=${encodeURIComponent(targetID.value)}` });
        return;
      }
      if (tapIndex === 1) {
        try {
          await setChatBlack(targetID.value, !blacked.value);
          blacked.value = !blacked.value;
          uni.showToast({ title: blacked.value ? t("social.chat.blocked") : t("social.chat.unblocked"), icon: "none" });
        } catch (error: any) {
          uni.showToast({ title: error?.message || t("social.common.operationFailed"), icon: "none" });
        }
        return;
      }
      if (tapIndex === 2) {
        const reasonLabels = [
          t("social.report.abuse"),
          t("social.report.advertising"),
          t("social.report.suspectedFraud"),
          t("social.report.other")
        ];
        // The service stores these established Chinese reason values; only the displayed labels are localized.
        const reasonValues = ["骚扰辱骂", "广告引流", "疑似诈骗", "其他原因"];
        uni.showActionSheet({
          itemList: reasonLabels,
          success: ({ tapIndex: reasonIndex }) => {
            void reportLiveUser(targetID.value, reasonValues[reasonIndex] || "其他原因")
              .then(() => uni.showToast({ title: t("social.common.reported"), icon: "none" }))
              .catch((error: any) =>
                uni.showToast({ title: error?.message || t("social.common.reportFailed"), icon: "none" })
              );
          }
        });
        return;
      }
      hideConversation();
    }
  });
}

function hideConversation() {
  uni.showModal({
    title: t("social.common.hideConversation"),
    content: t("social.chat.hideConfirm"),
    confirmColor: "#ff4d6e",
    success: async ({ confirm }) => {
      if (!confirm) return;
      try {
        await removeConversation(targetID.value, kind.value);
        uni.showToast({ title: t("social.common.conversationHidden"), icon: "none" });
        setTimeout(goBack, 300);
      } catch (error: any) {
        uni.showToast({ title: error?.message || t("social.common.operationFailed"), icon: "none" });
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
  background:
    radial-gradient(circle at 100% 0%, rgba(122, 92, 255, 0.08), transparent 32%),
    #f4f5f9;
}

.chat-nav {
  position: fixed;
  top: 0;
  right: 0;
  left: 0;
  z-index: 20;
  display: flex;
  height: calc(var(--status-bar-height) + 98rpx);
  align-items: center;
  justify-content: space-between;
  padding: var(--status-bar-height) 18rpx 0;
  border-bottom: 1rpx solid rgba(226, 229, 237, 0.88);
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 6rpx 22rpx rgba(25, 27, 38, 0.035);
}

.nav-button,
.history-button,
.voice-message,
.file-message,
.composer-tool,
.record-button,
.send-button,
.attachment-item {
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
}

.nav-button {
  flex: 0 0 70rpx;
  width: 70rpx;
  height: 70rpx;
  margin: 0px;
  border-radius: 23rpx;
  background: #f0f1f5;
}

.back-mark {
  width: 20rpx;
  height: 20rpx;
  border-bottom: 4rpx solid var(--ink);
  border-left: 4rpx solid var(--ink);
  transform: rotate(45deg);
}

.nav-more {
  gap: 5rpx;
}

.more-dot {
  width: 7rpx;
  height: 7rpx;
  border-radius: 50%;
  background: var(--ink);
}

.nav-peer {
  position: absolute;
  right: 104rpx;
  left: 104rpx;
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: center;
  gap: 12rpx;
}

.nav-avatar,
.nav-group-avatar {
  flex: 0 0 54rpx;
  width: 54rpx;
  height: 54rpx;
  border-radius: 18rpx;
  background: #eef0f5;
}

.nav-group-avatar {
  position: relative;
  background: var(--grad-cosmic);
}

.nav-person {
  position: absolute;
  width: 14rpx;
  height: 14rpx;
  border: 3rpx solid #fff;
  border-radius: 50%;
}

.nav-person::after {
  position: absolute;
  top: 13rpx;
  left: -5rpx;
  width: 18rpx;
  height: 9rpx;
  border: 3rpx solid #fff;
  border-bottom: 0;
  border-radius: 10rpx 10rpx 0 0;
  content: "";
}

.nav-person-a {
  top: 10rpx;
  left: 8rpx;
}

.nav-person-b {
  right: 8rpx;
  bottom: 17rpx;
}

.nav-title {
  min-width: 0;
  text-align: left;
}

.nav-title > text {
  display: block;
  max-width: 350rpx;
  overflow: hidden;
  color: var(--ink);
  font-size: 27rpx;
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.peer-status {
  display: flex;
  align-items: center;
  gap: 7rpx;
  margin-top: 4rpx;
  color: var(--ink-3);
  font-size: 18rpx;
}

.status-dot {
  width: 10rpx;
  height: 10rpx;
  border-radius: 50%;
  background: #b4bac5;
}

.status-dot.ready {
  background: var(--green);
  box-shadow: 0 0 0 4rpx rgba(17, 185, 129, 0.1);
}

.status-dot.connecting,
.status-dot.offline {
  background: #a98af5;
}

.message-scroll {
  position: fixed;
  top: calc(var(--status-bar-height) + 98rpx);
  right: 0;
  bottom: calc(106rpx + env(safe-area-inset-bottom));
  left: 0;
}

.message-list {
  min-height: 100%;
  padding: 22rpx 24rpx 34rpx;
}

.history-button {
  width: 180rpx;
  height: 50rpx;
  gap: 10rpx;
  margin: 0 auto 22rpx;
  border-radius: 25rpx;
  color: #858d9c;
  font-size: 20rpx;
  background: rgba(255, 255, 255, 0.88);
}

.history-end {
  margin-bottom: 20rpx;
  color: #aab0bc;
  font-size: 19rpx;
  text-align: center;
}

.loading-spinner {
  width: 22rpx;
  height: 22rpx;
  border: 3rpx solid rgba(122, 92, 255, 0.2);
  border-top-color: var(--violet);
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

.loading-spinner.white {
  border-color: rgba(255, 255, 255, 0.35);
  border-top-color: #fff;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.time-divider {
  display: flex;
  justify-content: center;
  margin: 8rpx 0 22rpx;
}

.time-divider text,
.system-message text {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 38rpx;
  padding: 5rpx 14rpx;
  border-radius: 13rpx;
  color: #9ca3af;
  font-size: 18rpx;
  line-height: 1.35;
  text-align: center;
  background: rgba(224, 226, 233, 0.72);
}

.system-message {
  display: flex;
  justify-content: center;
  margin: 0 0 22rpx;
}

.system-message text {
  max-width: 560rpx;
}

.bubble-row {
  display: flex;
  align-items: flex-start;
  gap: 13rpx;
  margin-bottom: 22rpx;
}

.bubble-row.self {
  flex-direction: row-reverse;
}

.message-avatar {
  flex: 0 0 66rpx;
  width: 66rpx;
  height: 66rpx;
  border: 3rpx solid rgba(255, 255, 255, 0.92);
  border-radius: 23rpx;
  background: #eef1f5;
  box-shadow: 0 5rpx 14rpx rgba(25, 27, 38, 0.08);
}

.message-main {
  display: flex;
  max-width: 548rpx;
  min-width: 0;
  flex-direction: column;
  align-items: flex-start;
}

.self .message-main {
  align-items: flex-end;
}

.sender-name {
  margin: 0 7rpx 7rpx;
  color: #8d95a3;
  font-size: 19rpx;
}

.bubble {
  min-height: 64rpx;
  padding: 16rpx 21rpx;
  border: 1rpx solid rgba(228, 230, 237, 0.9);
  border-radius: 8rpx 22rpx 22rpx;
  color: var(--ink);
  font-size: 27rpx;
  line-height: 1.55;
  background: #fff;
  box-shadow: 0 6rpx 18rpx rgba(25, 27, 38, 0.045);
}

.self .bubble {
  color: #fff;
  border-color: transparent;
  border-radius: 22rpx 8rpx 22rpx 22rpx;
  background: linear-gradient(135deg, #825fff, #a557ed);
  box-shadow: 0 8rpx 20rpx rgba(122, 92, 255, 0.18);
}

.bubble.media-bubble {
  overflow: hidden;
  padding: 0;
  border: 0;
  background: transparent;
}

.bubble.file-bubble {
  padding: 0;
}

.message-text {
  white-space: pre-wrap;
  word-break: break-word;
}

.chat-image {
  display: block;
  width: 294rpx;
  height: 294rpx;
  border: 4rpx solid #fff;
  border-radius: 20rpx;
  background: #e9ebf1;
  box-shadow: 0 7rpx 22rpx rgba(25, 27, 38, 0.09);
}

.chat-video {
  display: block;
  width: 380rpx;
  height: 270rpx;
  border: 4rpx solid #fff;
  border-radius: 20rpx;
  background: #171821;
}

.voice-message {
  min-width: 176rpx;
  height: 52rpx;
  justify-content: space-between;
  gap: 24rpx;
  color: inherit;
}

.voice-bars {
  display: flex;
  height: 27rpx;
  align-items: center;
  gap: 4rpx;
}

.voice-bars view {
  width: 4rpx;
  border-radius: 4rpx;
  background: currentColor;
}

.voice-bars view:nth-child(1) {
  height: 11rpx;
}

.voice-bars view:nth-child(2) {
  height: 23rpx;
}

.voice-bars view:nth-child(3) {
  height: 16rpx;
}

.voice-message.playing .voice-bars view {
  animation: voiceWave 0.7s ease-in-out infinite alternate;
}

.voice-message.playing .voice-bars view:nth-child(2) {
  animation-delay: 0.2s;
}

@keyframes voiceWave {
  to {
    transform: scaleY(0.45);
  }
}

.file-message {
  width: 366rpx;
  min-height: 122rpx;
  justify-content: space-between;
  gap: 18rpx;
  padding: 18rpx;
  color: inherit;
}

.file-copy {
  flex: 1;
  min-width: 0;
  text-align: left;
}

.file-name,
.file-size {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-name {
  font-size: 24rpx;
  font-weight: 900;
}

.file-size {
  margin-top: 9rpx;
  opacity: 0.66;
  font-size: 19rpx;
}

.file-icon {
  position: relative;
  display: flex;
  flex: 0 0 68rpx;
  width: 68rpx;
  height: 78rpx;
  align-items: center;
  justify-content: center;
  border: 2rpx solid currentColor;
  border-radius: 12rpx;
}

.file-icon text {
  margin-top: 15rpx;
  font-size: 15rpx;
  font-weight: 900;
}

.file-fold {
  position: absolute;
  top: -2rpx;
  right: -2rpx;
  width: 20rpx;
  height: 20rpx;
  border-bottom: 2rpx solid currentColor;
  border-left: 2rpx solid currentColor;
  border-radius: 0 10rpx 0 5rpx;
}

.message-meta {
  margin: 6rpx 7rpx 0;
  color: #a3a9b4;
  font-size: 17rpx;
}

.empty-chat {
  display: flex;
  min-height: 620rpx;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80rpx 20rpx;
  text-align: center;
}

.empty-chat image {
  width: 310rpx;
  height: 260rpx;
}

.empty-chat text {
  display: block;
}

.empty-chat text:nth-child(2) {
  color: var(--ink-2);
  font-size: 28rpx;
  font-weight: 900;
}

.empty-chat text:last-child {
  max-width: 480rpx;
  margin-top: 12rpx;
  color: var(--ink-3);
  font-size: 21rpx;
  line-height: 1.55;
}

.chat-tail {
  height: 2rpx;
}

.composer-notice {
  position: fixed;
  right: 0;
  bottom: 0;
  left: 0;
  z-index: 20;
  display: flex;
  min-height: calc(92rpx + env(safe-area-inset-bottom));
  align-items: center;
  justify-content: center;
  gap: 12rpx;
  padding: 16rpx 24rpx calc(16rpx + env(safe-area-inset-bottom));
  color: #8c93a0;
  font-size: 22rpx;
  text-align: center;
  background: rgba(255, 255, 255, 0.96);
  border-top: 1rpx solid var(--line);
}

.notice-lock {
  position: relative;
  width: 22rpx;
  height: 18rpx;
  border: 3rpx solid #9ca3af;
  border-radius: 5rpx;
}

.notice-lock::before {
  position: absolute;
  top: -14rpx;
  left: 3rpx;
  width: 10rpx;
  height: 12rpx;
  border: 3rpx solid #9ca3af;
  border-bottom: 0;
  border-radius: 8rpx 8rpx 0 0;
  content: "";
}

.attachment-panel {
  position: fixed;
  right: 0;
  bottom: calc(104rpx + env(safe-area-inset-bottom));
  left: 0;
  z-index: 18;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16rpx;
  padding: 22rpx 38rpx;
  border-top: 1rpx solid var(--line);
  background: rgba(255, 255, 255, 0.97);
  box-shadow: 0 -10rpx 28rpx rgba(25, 27, 38, 0.05);
}

.attachment-item {
  height: 116rpx;
  flex-direction: column;
  gap: 9rpx;
  border-radius: 20rpx;
  color: var(--ink-2);
  font-size: 20rpx;
  font-weight: 800;
  background: none;
  border: none;
}

.attachment-icon {
  position: relative;
  display: flex;
  width: 50rpx;
  height: 50rpx;
  align-items: center;
  justify-content: center;
  border-radius: 15rpx;
  color: var(--violet);
  background: var(--violet-soft);
}

.image-icon::before {
  width: 25rpx;
  height: 22rpx;
  border: 3rpx solid currentColor;
  border-radius: 5rpx;
  content: "";
}

.image-icon view {
  position: absolute;
  right: 11rpx;
  bottom: 12rpx;
  width: 16rpx;
  height: 12rpx;
  border-top: 3rpx solid currentColor;
  transform: rotate(-38deg);
}

.video-icon::before {
  width: 25rpx;
  height: 22rpx;
  border: 3rpx solid currentColor;
  border-radius: 6rpx;
  content: "";
}

.video-icon view {
  position: absolute;
  right: 7rpx;
  width: 9rpx;
  height: 14rpx;
  border-top: 3rpx solid currentColor;
  border-right: 3rpx solid currentColor;
  transform: rotate(45deg);
}

.document-icon {
  border: 3rpx solid currentColor;
  background: transparent;
}

.document-icon text {
  font-size: 13rpx;
  font-weight: 900;
}

.composer {
  position: fixed;
  right: 0;
  bottom: 0;
  left: 0;
  z-index: 20;
  display: flex;
  min-height: calc(106rpx + env(safe-area-inset-bottom));
  align-items: flex-end;
  gap: 12rpx;
  padding: 16rpx 18rpx calc(16rpx + env(safe-area-inset-bottom));
  background: rgba(255, 255, 255, 0.97);
  border-top: 1rpx solid var(--line);
}

.composer-tool,
.send-button {
  flex: 0 0 68rpx;
  width: 68rpx;
  height: 68rpx;
  border-radius: 22rpx;
  color: var(--ink-2);
  background: #eff1f5;
}

.composer-tool.active {
  color: #fff;
  background: var(--violet);
}

.mic-mark {
  position: relative;
  width: 16rpx;
  height: 25rpx;
  border: 3rpx solid currentColor;
  border-radius: 10rpx;
}

.mic-mark::after {
  position: absolute;
  top: 15rpx;
  left: -9rpx;
  width: 28rpx;
  height: 16rpx;
  border-bottom: 3rpx solid currentColor;
  border-radius: 0 0 16rpx 16rpx;
  content: "";
}

.composer-input {
  flex: 1;
  min-height: 68rpx;
  max-height: 190rpx;
  padding: 16rpx 22rpx;
  border: 2rpx solid transparent;
  border-radius: 23rpx;
  color: var(--ink);
  font-size: 26rpx;
  line-height: 1.4;
  background: #f2f3f7;
}

.composer-input:focus {
  border-color: rgba(122, 92, 255, 0.26);
  background: #fff;
}

.send-button {
  flex-basis: 120rpx;
  width: 92rpx;
  color: #fff;
  font-size: 23rpx;
  font-weight: 900;
  background: var(--grad-cosmic);
  box-shadow: 0 8rpx 20rpx rgba(122, 92, 255, 0.2);
}

.add-toggle text {
  font-size: 38rpx;
  font-weight: 400;
  line-height: 1;
}

.record-button {
  position: relative;
  flex: 1;
  height: 68rpx;
  overflow: hidden;
  border-radius: 23rpx;
  color: var(--ink-2);
  font-size: 24rpx;
  font-weight: 900;
  background: #f2f3f7;
}

.record-button.recording {
  color: #fff;
  background: linear-gradient(135deg, #ff5a78, #ff8464);
}

.record-pulse {
  width: 15rpx;
  height: 15rpx;
  margin-right: 11rpx;
  border-radius: 50%;
  background: #fff;
  animation: pulse 0.8s ease-in-out infinite alternate;
}

@keyframes pulse {
  to {
    opacity: 0.35;
    transform: scale(0.7);
  }
}

.composer-tool[disabled],
.send-button[disabled],
.attachment-item[disabled] {
  opacity: 0.45;
}
.attachment-item:after{
  border: none;
}
</style>
