<template>
  <view class="safe-page group-info-page">
    <view class="group-card card">
      <view class="group-head">
        <image class="group-avatar" :src="groupAvatar" mode="aspectFill" />
        <view class="group-main">
          <text class="group-title">{{ group?.groupName || "群聊" }}</text>
          <text class="group-id" @tap="copyGroupID">群ID：{{ groupID }} · {{ members.length }}人</text>
        </view>
      </view>

      <view class="form-field">
        <text>群聊名称</text>
        <input v-model.trim="groupName" maxlength="30" :disabled="!canManage" placeholder="输入群聊名称" />
      </view>
      <view class="form-field">
        <text>群简介</text>
        <textarea v-model.trim="introduction" maxlength="120" :disabled="!canManage" placeholder="介绍一下这个群聊" />
      </view>
      <view class="form-field">
        <text>群公告</text>
        <textarea v-model.trim="notification" maxlength="300" :disabled="!canManage" placeholder="暂无群公告" />
      </view>
      <button v-if="canManage" class="save-button" :disabled="saving" @tap="saveGroup">保存群资料</button>
    </view>

    <view class="management card">
      <view class="setting-row" @tap="inviteMembers">
        <text>邀请群成员</text>
        <text>›</text>
      </view>
      <view v-if="canManage" class="setting-row">
        <view>
          <text>全员禁言</text>
          <text class="setting-desc">群主和管理员仍可发言</text>
        </view>
        <switch color="#7c5cff" :checked="groupMuted" @change="changeGroupMute" />
      </view>
    </view>

    <view v-if="canManage && applications.length" class="section">
      <view class="section-head">
        <text>入群申请</text>
        <text>{{ applications.length }}条待处理</text>
      </view>
      <view class="application-list card">
        <view v-for="application in applications" :key="`${application.groupID}-${application.userID}`" class="application-row">
          <image :src="applicationAvatar(application)" mode="aspectFill" />
          <view class="application-main">
            <text>{{ application.nickname || `用户${application.userID}` }}</text>
            <text>{{ application.reqMsg || "申请加入群聊" }}</text>
          </view>
          <button class="reject-button" @tap="handleApplication(application, false)">拒绝</button>
          <button class="accept-button" @tap="handleApplication(application, true)">同意</button>
        </view>
      </view>
    </view>

    <view class="section">
      <view class="section-head">
        <text>群成员</text>
        <text>{{ members.length }}人</text>
      </view>
      <view class="member-list card">
        <view v-for="member in members" :key="member.userID" class="member-row" @tap="manageMember(member)">
          <image :src="memberAvatar(member)" mode="aspectFill" />
          <view class="member-main">
            <view class="member-name-line">
              <text class="member-name">{{ member.nickname || `用户${member.userID}` }}</text>
              <text v-if="roleName(member)" class="role-badge">{{ roleName(member) }}</text>
              <text v-if="memberMuted(member)" class="mute-badge">禁言中</text>
            </view>
            <text class="member-id">ID：{{ member.userID }}</text>
          </view>
          <text v-if="canManageMember(member)" class="member-more">•••</text>
        </view>
      </view>
    </view>

    <button class="leave-button" :class="{ danger: isOwner }" @tap="leaveGroup">
      {{ isOwner ? "解散群聊" : "退出群聊" }}
    </button>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad, onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import {
  dismissChatGroup,
  getChatGroup,
  getChatGroupApplications,
  getChatGroupMembers,
  handleChatGroupApplication,
  kickChatGroupMember,
  muteChatGroup,
  muteChatGroupMember,
  quitChatGroup,
  setChatGroupMemberRole,
  transferChatGroupOwner,
  updateChatGroup
} from "@/api/services";
import type { ChatGroup, ChatGroupApplication, ChatGroupMember } from "@/types/api";
import { getSession, requireLogin } from "@/utils/session";
import { absolutizeUrl } from "@/utils/url";

const groupID = ref("");
const group = ref<ChatGroup>();
const members = ref<ChatGroupMember[]>([]);
const applications = ref<ChatGroupApplication[]>([]);
const groupName = ref("");
const introduction = ref("");
const notification = ref("");
const groupMuted = ref(false);
const loading = ref(false);
const saving = ref(false);
let loadedOnce = false;

const selfMember = computed(() => members.value.find((member) => member.userID === String(getSession().uid)));
const selfRole = computed(() => Number(selfMember.value?.roleLevel || 0));
const isOwner = computed(() => selfRole.value === 100 || group.value?.ownerUserID === String(getSession().uid));
const canManage = computed(() => isOwner.value || selfRole.value === 60);
const groupAvatar = computed(
  () => absolutizeUrl(String(group.value?.faceURL || "")) || "/static/brand/icon-round.webp"
);

function memberAvatar(member: ChatGroupMember) {
  return absolutizeUrl(String(member.faceURL || "")) || "/static/brand/icon-round.webp";
}

function applicationAvatar(application: ChatGroupApplication) {
  return absolutizeUrl(String(application.userFaceURL || "")) || "/static/brand/icon-round.webp";
}

function roleName(member: ChatGroupMember) {
  const role = Number(member.roleLevel || 0);
  return role === 100 ? "群主" : role === 60 ? "管理员" : "";
}

function memberMuted(member: ChatGroupMember) {
  const end = Number(member.muteEndTime || 0);
  const normalized = end < 1_000_000_000_000 ? end * 1000 : end;
  return normalized > Date.now();
}

function canManageMember(member: ChatGroupMember) {
  if (!canManage.value || member.userID === String(getSession().uid)) {
    return false;
  }
  if (isOwner.value) {
    return Number(member.roleLevel || 0) < 100;
  }
  return Number(member.roleLevel || 0) < 60;
}

async function load() {
  if (!requireLogin() || !groupID.value || loading.value) {
    uni.stopPullDownRefresh();
    return;
  }
  loading.value = true;
  try {
    const [groupInfo, memberList] = await Promise.all([
      getChatGroup(groupID.value),
      getChatGroupMembers(groupID.value, 0, 500)
    ]);
    group.value = groupInfo;
    members.value = memberList;
    groupName.value = String(groupInfo.groupName || "");
    introduction.value = String(groupInfo.introduction || "");
    notification.value = String(groupInfo.notification || "");
    groupMuted.value = Number(groupInfo.status || 0) === 3;
    uni.setNavigationBarTitle({ title: groupName.value || "群聊资料" });
    if (canManage.value) {
      const pending = await getChatGroupApplications(0, 100);
      applications.value = pending.filter(
        (application) => application.groupID === groupID.value && Number(application.handleResult || 0) === 0
      );
    } else {
      applications.value = [];
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || "群资料加载失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

async function saveGroup() {
  if (!canManage.value || saving.value || !groupName.value) {
    if (!groupName.value) {
      uni.showToast({ title: "群聊名称不能为空", icon: "none" });
    }
    return;
  }
  saving.value = true;
  try {
    await updateChatGroup(groupID.value, {
      groupName: groupName.value,
      introduction: introduction.value,
      notification: notification.value
    });
    uni.showToast({ title: "群资料已保存", icon: "none" });
    await load();
  } catch (error: any) {
    uni.showToast({ title: error?.message || "保存失败", icon: "none" });
  } finally {
    saving.value = false;
  }
}

function inviteMembers() {
  uni.navigateTo({
    url: `/pages/message/new-chat?mode=invite&groupid=${encodeURIComponent(groupID.value)}`
  });
}

async function changeGroupMute(event: any) {
  const next = Boolean(event?.detail?.value);
  try {
    await muteChatGroup(groupID.value, next);
    groupMuted.value = next;
    uni.showToast({ title: next ? "已开启全员禁言" : "已关闭全员禁言", icon: "none" });
  } catch (error: any) {
    groupMuted.value = !next;
    uni.showToast({ title: error?.message || "设置失败", icon: "none" });
  }
}

async function handleApplication(application: ChatGroupApplication, accept: boolean) {
  try {
    await handleChatGroupApplication(
      application.groupID,
      application.userID,
      accept,
      accept ? "欢迎加入群聊" : "暂不同意加入"
    );
    applications.value = applications.value.filter((item) => item.userID !== application.userID);
    uni.showToast({ title: accept ? "已同意" : "已拒绝", icon: "none" });
    if (accept) {
      await load();
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || "处理失败", icon: "none" });
  }
}

function manageMember(member: ChatGroupMember) {
  if (!canManageMember(member)) {
    return;
  }
  const labels: string[] = [];
  const actions: Array<() => Promise<unknown>> = [];
  const role = Number(member.roleLevel || 0);
  if (isOwner.value) {
    labels.push(role === 60 ? "取消管理员" : "设为管理员");
    actions.push(() => setChatGroupMemberRole(groupID.value, member.userID, role === 60 ? 20 : 60));
  }
  labels.push(memberMuted(member) ? "解除禁言" : "禁言10分钟");
  actions.push(() => muteChatGroupMember(groupID.value, member.userID, memberMuted(member) ? 0 : 600));
  labels.push("移出群聊");
  actions.push(() => kickChatGroupMember(groupID.value, member.userID));
  if (isOwner.value) {
    labels.push("转让群主");
    actions.push(() => transferChatGroupOwner(groupID.value, member.userID));
  }
  uni.showActionSheet({
    itemList: labels,
    success: async ({ tapIndex }) => {
      try {
        await actions[tapIndex]?.();
        uni.showToast({ title: "操作成功", icon: "none" });
        await load();
      } catch (error: any) {
        uni.showToast({ title: error?.message || "操作失败", icon: "none" });
      }
    }
  });
}

function copyGroupID() {
  uni.setClipboardData({ data: groupID.value });
}

function leaveGroup() {
  uni.showModal({
    title: isOwner.value ? "解散群聊" : "退出群聊",
    content: isOwner.value
      ? "解散后所有成员都将退出，聊天记录无法继续同步。确认解散？"
      : "确认退出这个群聊？",
    confirmColor: "#ff4f62",
    success: async ({ confirm }) => {
      if (!confirm) {
        return;
      }
      try {
        if (isOwner.value) {
          await dismissChatGroup(groupID.value);
        } else {
          await quitChatGroup(groupID.value);
        }
        uni.reLaunch({ url: "/pages/message/index" });
      } catch (error: any) {
        uni.showToast({ title: error?.message || "操作失败", icon: "none" });
      }
    }
  });
}

onLoad((query) => {
  groupID.value = String(query?.groupid || "");
});

onShow(() => {
  if (!groupID.value) {
    return;
  }
  if (!loadedOnce) {
    loadedOnce = true;
  }
  void load();
});

onPullDownRefresh(() => {
  void load();
});
</script>

<style scoped>
.group-info-page {
  background: var(--bg);
}

.group-card,
.management,
.section,
.leave-button {
  margin-bottom: 22rpx;
}

.group-card {
  padding: 24rpx;
}

.group-head {
  display: flex;
  align-items: center;
  gap: 18rpx;
  padding-bottom: 24rpx;
  border-bottom: 1rpx solid #edf0f5;
}

.group-avatar {
  width: 92rpx;
  height: 92rpx;
  border-radius: 24rpx;
  background: #eef1f5;
}

.group-main {
  flex: 1;
  min-width: 0;
}

.group-title,
.group-id {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.group-title {
  color: var(--ink);
  font-size: 32rpx;
  font-weight: 900;
}

.group-id {
  margin-top: 9rpx;
  color: var(--ink-3);
  font-size: 22rpx;
}

.form-field {
  margin-top: 22rpx;
}

.form-field > text {
  display: block;
  margin-bottom: 10rpx;
  color: var(--ink);
  font-size: 24rpx;
  font-weight: 900;
}

.form-field input,
.form-field textarea {
  width: 100%;
  padding: 18rpx 20rpx;
  border-radius: 16rpx;
  box-sizing: border-box;
  color: var(--ink);
  font-size: 26rpx;
  background: #f4f6f9;
}

.form-field input {
  height: 72rpx;
}

.form-field textarea {
  height: 126rpx;
}

.form-field input[disabled],
.form-field textarea[disabled] {
  color: var(--ink-2);
  opacity: 1;
}

.save-button {
  height: 74rpx;
  margin-top: 24rpx;
  border-radius: 37rpx;
  color: #fff;
  font-size: 26rpx;
  font-weight: 900;
  background: var(--brand);
}

.management {
  overflow: hidden;
}

.setting-row {
  display: flex;
  min-height: 94rpx;
  align-items: center;
  justify-content: space-between;
  gap: 16rpx;
  padding: 0 24rpx;
  color: var(--ink);
  font-size: 27rpx;
  font-weight: 800;
  border-bottom: 1rpx solid #edf0f5;
}

.setting-row:last-child {
  border-bottom: 0;
}

.setting-row > text:last-child {
  color: #b2bac7;
  font-size: 38rpx;
}

.setting-desc {
  display: block;
  margin-top: 6rpx;
  color: var(--ink-3);
  font-size: 21rpx;
  font-weight: 500;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 0 4rpx 14rpx;
}

.section-head text:first-child {
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 900;
}

.section-head text:last-child {
  color: var(--ink-3);
  font-size: 21rpx;
}

.application-list,
.member-list {
  overflow: hidden;
}

.application-row,
.member-row {
  display: flex;
  min-height: 104rpx;
  align-items: center;
  gap: 14rpx;
  padding: 16rpx 20rpx;
  border-bottom: 1rpx solid #edf0f5;
}

.application-row:last-child,
.member-row:last-child {
  border-bottom: 0;
}

.application-row image,
.member-row image {
  width: 68rpx;
  height: 68rpx;
  border-radius: 34rpx;
  background: #eef1f5;
}

.application-main,
.member-main {
  flex: 1;
  min-width: 0;
}

.application-main text,
.member-name,
.member-id {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.application-main text:first-child,
.member-name {
  color: var(--ink);
  font-size: 26rpx;
  font-weight: 900;
}

.application-main text:last-child,
.member-id {
  margin-top: 7rpx;
  color: var(--ink-3);
  font-size: 21rpx;
}

.accept-button,
.reject-button {
  flex: 0 0 86rpx;
  height: 54rpx;
  border-radius: 27rpx;
  font-size: 21rpx;
  font-weight: 800;
}

.accept-button {
  color: #fff;
  background: var(--brand);
}

.reject-button {
  color: var(--ink-2);
  background: #eef1f5;
}

.member-name-line {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 8rpx;
}

.role-badge,
.mute-badge {
  flex: 0 0 auto;
  padding: 3rpx 9rpx;
  border-radius: 10rpx;
  color: var(--brand);
  font-size: 18rpx;
  font-weight: 800;
  background: rgba(124, 92, 255, 0.09);
}

.mute-badge {
  color: #ff5f6d;
  background: rgba(255, 95, 109, 0.09);
}

.member-more {
  color: #a7afbd;
  font-size: 26rpx;
  letter-spacing: 2rpx;
}

.leave-button {
  height: 80rpx;
  border-radius: 20rpx;
  color: #ff4f62;
  font-size: 27rpx;
  font-weight: 900;
  background: #fff;
}

.leave-button.danger {
  color: #fff;
  background: #ff4f62;
}
</style>
