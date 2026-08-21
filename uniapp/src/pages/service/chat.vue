<template>
  <view class="support-chat">
    <scroll-view class="messages" scroll-y :scroll-top="scrollTop">
      <view v-if="loading" class="state">{{ t("misc.service.connecting") }}</view>
      <view v-else-if="!messages.length" class="welcome card">
        <text>{{ t("misc.service.platformSupport") }}</text>
        <text>{{ t("misc.service.chatWelcome") }}</text>
      </view>
      <view
        v-for="message in messages"
        :key="message.id"
        class="message-row"
        :class="{ self: message.sender_type === 1 }"
      >
        <view class="bubble">
          <text>{{ message.text_content || t("misc.service.attachment") }}</text>
          <text class="time">{{ formatTime(message.created_at) }}</text>
        </view>
      </view>
    </scroll-view>
    <view class="composer">
      <input
        v-model.trim="draft"
        class="input"
        confirm-type="send"
        :placeholder="t('misc.service.problemPlaceholder')"
        @confirm="send"
      />
      <button :disabled="!draft || sending" @tap="send">{{ sending ? t("misc.common.sending") : t("misc.common.send") }}</button>
    </view>
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onLoad, onUnload } from "@dcloudio/uni-app";
import {
  getSupportConversation,
  getSupportMessages,
  sendSupportMessage,
  type SupportConversation,
  type SupportMessage
} from "@/api/support";
import { requireLogin } from "@/utils/session";
import { t } from "@/i18n";

const conversation = ref<SupportConversation>();
const messages = ref<SupportMessage[]>([]);
const draft = ref("");
const loading = ref(true);
const sending = ref(false);
const scrollTop = ref(0);
let pollTimer: number | undefined;

function formatTime(timestamp: number) {
  if (!timestamp) return "";
  const date = new Date(timestamp * 1000);
  return `${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

function scrollBottom() {
  setTimeout(() => {
    scrollTop.value += 100000;
  }, 30);
}

async function refresh(silent = false) {
  try {
    if (!conversation.value) {
      conversation.value = await getSupportConversation();
    }
    messages.value = await getSupportMessages(conversation.value.id);
    scrollBottom();
  } catch (error: any) {
    if (!silent) {
      uni.showToast({ title: error?.message || t("misc.service.connectionFailed"), icon: "none" });
    }
  } finally {
    loading.value = false;
  }
}

async function send() {
  const text = draft.value.trim();
  if (!text || sending.value || !conversation.value) return;
  sending.value = true;
  try {
    const message = await sendSupportMessage(conversation.value.id, text);
    messages.value.push(message);
    draft.value = "";
    scrollBottom();
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.service.messageFailed"), icon: "none" });
  } finally {
    sending.value = false;
  }
}

onLoad(() => {
  if (!requireLogin()) return;
  void refresh();
  pollTimer = setInterval(() => void refresh(true), 5000) as unknown as number;
});

onUnload(() => {
  if (pollTimer) clearInterval(pollTimer);
});
</script>

<style scoped>
.support-chat {
  display: flex;
  height: 100vh;
  flex-direction: column;
  background: var(--bg);
}

.messages {
  min-height: 0;
  flex: 1;
  padding: 24rpx;
  box-sizing: border-box;
}

.state,
.welcome {
  padding: 28rpx;
  color: var(--ink-3);
  font-size: 25rpx;
  text-align: center;
}

.welcome text {
  display: block;
}

.welcome text:first-child {
  color: var(--ink);
  font-size: 32rpx;
  font-weight: 900;
}

.welcome text:last-child {
  margin-top: 14rpx;
}

.message-row {
  display: flex;
  margin-bottom: 18rpx;
}

.message-row.self {
  justify-content: flex-end;
}

.bubble {
  max-width: 74%;
  padding: 18rpx 20rpx 12rpx;
  border-radius: 18rpx 18rpx 18rpx 4rpx;
  color: var(--ink);
  font-size: 27rpx;
  line-height: 1.5;
  background: #fff;
  box-shadow: 0 8rpx 24rpx rgba(33, 38, 54, 0.06);
}

.self .bubble {
  border-radius: 18rpx 18rpx 4rpx 18rpx;
  color: #fff;
  background: var(--brand);
}

.time {
  display: block;
  margin-top: 6rpx;
  font-size: 20rpx;
  text-align: right;
  opacity: 0.65;
}

.composer {
  display: flex;
  gap: 14rpx;
  padding: 18rpx 22rpx calc(18rpx + env(safe-area-inset-bottom));
  border-top: 1rpx solid #edf0f5;
  background: #fff;
}

.input {
  height: 76rpx;
  min-width: 0;
  flex: 1;
  padding: 0 24rpx;
  border-radius: 38rpx;
  color: var(--ink);
  background: #f4f6f9;
}

.composer button {
  width: 128rpx;
  height: 76rpx;
  border-radius: 38rpx;
  color: #fff;
  font-size: 25rpx;
  font-weight: 900;
  background: var(--brand);
}
</style>
