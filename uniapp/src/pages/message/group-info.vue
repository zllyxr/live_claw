<template>
  <view class="safe-page group-page">
    <view class="group-hero">
      <view class="hero-avatar">
        <view class="hero-person hero-person-a" />
        <view class="hero-person hero-person-b" />
        <view class="hero-people-body" />
      </view>
      <view class="hero-copy">
        <view class="hero-title-line">
          <text class="hero-title">{{ group?.groupName || t("social.common.groupChat") }}</text>
          <view class="role-tag">{{ selfRoleName }}</view>
        </view>
        <text class="hero-meta">
          {{ members.length }} / {{ group?.maxMemberCount || 500 }} {{ t("social.common.members") }}
        </text>
        <button class="group-id-button" @tap="copyGroupID">
          <text>{{ t("social.groupInfo.groupId") }}: {{ groupID }}</text>
          <view class="copy-mark" />
        </button>
      </view>
    </view>

    <view v-if="notification" class="announcement-card">
      <view class="announcement-icon">!</view>
      <view class="announcement-copy">
        <text>{{ t("social.groupInfo.announcement") }}</text>
        <text>{{ notification }}</text>
      </view>
    </view>

    <view v-if="canManage && applications.length" class="section applications-section">
      <view class="section-head">
        <view>
          <text class="section-title">{{ t("social.groupInfo.applications") }}</text>
          <text class="section-desc">{{ applications.length }} {{ t("social.groupInfo.awaitingReview") }}</text>
        </view>
        <view class="section-count">{{ applications.length }}</view>
      </view>
      <view class="application-list panel">
        <view
          v-for="application in applications"
          :key="application.applicationID || `${application.groupID}-${application.userID}`"
          class="application-row"
        >
          <image :src="applicationAvatar(application)" mode="aspectFill" />
          <view class="application-main">
            <text>{{ application.nickname || `${t("social.common.user")} ${application.userID}` }}</text>
            <text>{{ application.reqMsg || t("social.groupInfo.requestToJoin") }}</text>
            <text>{{ formatTime(application.reqTime) }}</text>
          </view>
          <view class="application-actions">
            <button
              class="mini-button reject"
              :disabled="handlingApplication === application.applicationID"
              @tap="handleApplication(application, false)"
            >
              {{ t("social.groupInfo.reject") }}
            </button>
            <button
              class="mini-button accept"
              :disabled="handlingApplication === application.applicationID"
              @tap="handleApplication(application, true)"
            >
              {{ t("social.groupInfo.accept") }}
            </button>
          </view>
        </view>
      </view>
    </view>

    <view class="section">
      <view class="section-head">
        <view>
          <text class="section-title">{{ t("social.groupInfo.profile") }}</text>
          <text class="section-desc">{{ canManage ? t("social.groupInfo.rememberSave") : t("social.groupInfo.adminOnlyEdit") }}</text>
        </view>
        <view v-if="saving" class="saving-tag">{{ t("social.groupInfo.saving") }}</view>
      </view>
      <view class="profile-panel panel">
        <view class="form-field">
          <text class="field-label">{{ t("social.newChat.groupName") }}</text>
          <input
            v-model="groupName"
            maxlength="200"
            :disabled="!canManage"
            :placeholder="t('social.groupInfo.groupNamePlaceholder')"
          />
          <text class="field-count">{{ groupName.length }}/200</text>
        </view>
        <view class="form-field">
          <text class="field-label">{{ t("social.groupInfo.introduction") }}</text>
          <textarea
            v-model="introduction"
            maxlength="1000"
            :disabled="!canManage"
            :placeholder="t('social.groupInfo.introductionPlaceholder')"
          />
          <text class="field-count">{{ introduction.length }}/1000</text>
        </view>
        <view class="form-field">
          <text class="field-label">{{ t("social.groupInfo.announcement") }}</text>
          <textarea
            v-model="notification"
            maxlength="2000"
            :disabled="!canManage"
            :placeholder="t('social.groupInfo.noAnnouncement')"
          />
          <text class="field-count">{{ notification.length }}/2000</text>
        </view>
        <view v-if="canManage" class="policy-row" @tap="chooseJoinPolicy">
          <view>
            <text>{{ t("social.groupInfo.joinMethod") }}</text>
            <text>{{ joinPolicyDescription }}</text>
          </view>
          <view class="policy-value">
            <text>{{ joinPolicyLabel }}</text>
            <view class="chevron" />
          </view>
        </view>
        <button
          v-if="canManage"
          class="save-button"
          :disabled="saving || !groupName.trim()"
          @tap="saveGroup"
        >
          {{ t("social.groupInfo.saveProfile") }}
        </button>
      </view>
    </view>

    <view class="section">
      <view class="section-head member-section-head">
        <view>
          <text class="section-title">{{ t("social.groupInfo.membersTitle") }}</text>
          <text class="section-desc">{{ t("social.groupInfo.total") }} {{ members.length }} {{ t("social.common.people") }}</text>
        </view>
        <view class="invite-button" @tap="inviteMembers">
          <text>＋</text>
          <text>{{ t("social.newChat.inviteMembers") }}</text>
        </view>
      </view>

      <view v-if="members.length > 8" class="member-search">
        <view class="search-mark" />
        <input v-model.trim="memberKeyword" :placeholder="t('social.groupInfo.searchMembers')" />
        <button v-if="memberKeyword" class="clear-search" @tap="memberKeyword = ''">×</button>
      </view>

      <view class="member-list panel">
        <button
          v-for="member in visibleMembers"
          :key="member.userID"
          class="member-row"
          @tap="openMember(member)"
        >
          <view class="member-avatar-wrap">
            <image :src="memberAvatar(member)" mode="aspectFill" />
            <view v-if="member.userID === String(getSession().uid)" class="self-dot" />
          </view>
          <view class="member-main">
            <view class="member-name-line">
              <text class="member-name">{{ member.nickname || `${t("social.common.user")} ${member.userID}` }}</text>
              <view v-if="roleName(member)" class="member-tag role">
                {{ roleName(member) }}
              </view>
              <view v-if="memberMuted(member)" class="member-tag muted">{{ t("social.groupInfo.muted") }}</view>
            </view>
            <text class="member-id">ID {{ member.userID }} · {{ joinedText(member) }}</text>
          </view>
          <view v-if="canManageMember(member)" class="member-more">
            <view />
            <view />
            <view />
          </view>
          <view v-else class="member-chevron" />
        </button>
        <view v-if="!visibleMembers.length" class="member-empty">{{ t("social.groupInfo.noMatchingMembers") }}</view>
      </view>
    </view>

    <view v-if="canManage" class="section">
      <view class="section-head">
        <view>
          <text class="section-title">{{ t("social.groupInfo.management") }}</text>
          <text class="section-desc">{{ t("social.groupInfo.ownerAdminOnly") }}</text>
        </view>
      </view>
      <view class="management panel">
        <view class="setting-row">
          <view>
            <text>{{ t("social.groupInfo.muteAll") }}</text>
            <text>{{ t("social.groupInfo.muteAllDescription") }}</text>
          </view>
          <switch color="#7a5cff" :checked="groupMuted" @change="changeGroupMute" />
        </view>
      </view>
    </view>

    <button class="leave-button" :class="{ danger: isOwner }" @tap="leaveGroup">
      {{ isOwner ? t("social.groupInfo.dismissGroup") : t("social.groupInfo.leaveGroup") }}
    </button>
  </view>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from "vue";
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
import { useI18n } from "@/i18n";

const { locale, t } = useI18n();

const groupID = ref("");
const initialSection = ref("");
const group = ref<ChatGroup>();
const members = ref<ChatGroupMember[]>([]);
const applications = ref<ChatGroupApplication[]>([]);
const groupName = ref("");
const introduction = ref("");
const notification = ref("");
const joinPolicy = ref(1);
const groupMuted = ref(false);
const memberKeyword = ref("");
const loading = ref(false);
const saving = ref(false);
const handlingApplication = ref("");

const selfMember = computed(() =>
  members.value.find((member) => member.userID === String(getSession().uid))
);
const selfRole = computed(() =>
  Math.max(Number(selfMember.value?.roleLevel || 0), Number(group.value?.roleLevel || 0))
);
const isOwner = computed(
  () => selfRole.value === 100 || group.value?.ownerUserID === String(getSession().uid)
);
const canManage = computed(() => isOwner.value || selfRole.value >= 60);
const selfRoleName = computed(() =>
  isOwner.value
    ? t("social.groupInfo.owner")
    : selfRole.value >= 60
      ? t("social.groupInfo.admin")
      : t("social.groupInfo.member")
);
const joinPolicyLabel = computed(
  () => ({
    1: t("social.groupInfo.reviewRequired"),
    2: t("social.groupInfo.joinAllowed"),
    3: t("social.groupInfo.joinForbidden")
  } as Record<number, string>)[joinPolicy.value] || t("social.groupInfo.reviewRequired")
);
const joinPolicyDescription = computed(
  () =>
    ({
      1: t("social.groupInfo.reviewDescription"),
      2: t("social.groupInfo.directJoinDescription"),
      3: t("social.groupInfo.closedDescription")
    } as Record<number, string>)[joinPolicy.value] || ""
);
const visibleMembers = computed(() => {
  const query = memberKeyword.value.toLocaleLowerCase();
  if (!query) return members.value;
  return members.value.filter((member) =>
    `${member.nickname || ""} ${member.userID}`.toLocaleLowerCase().includes(query)
  );
});

function memberAvatar(member: ChatGroupMember) {
  return absolutizeUrl(String(member.faceURL || "")) || "/static/brand/icon-round.webp";
}

function applicationAvatar(application: ChatGroupApplication) {
  return absolutizeUrl(String(application.userFaceURL || "")) || "/static/brand/icon-round.webp";
}

function roleName(member: ChatGroupMember) {
  const role = Number(member.roleLevel || 0);
  return role === 100
    ? t("social.groupInfo.owner")
    : role === 60
      ? t("social.groupInfo.admin")
      : "";
}

function memberMuted(member: ChatGroupMember) {
  const end = Number(member.muteEndTime || 0);
  const normalized = end > 0 && end < 1_000_000_000_000 ? end * 1000 : end;
  return normalized > Date.now();
}

function canManageMember(member: ChatGroupMember) {
  if (!canManage.value || member.userID === String(getSession().uid)) return false;
  const role = Number(member.roleLevel || 0);
  return isOwner.value ? role < 100 : role < 60;
}

function joinedText(member: ChatGroupMember) {
  const raw = Number(member.joinTime || 0);
  if (!raw) return t("social.groupInfo.member");
  const timestamp = raw < 1_000_000_000_000 ? raw * 1000 : raw;
  const date = new Date(timestamp);
  return `${new Intl.DateTimeFormat(locale.value, { year: "numeric", month: "numeric", day: "numeric" }).format(date)} ${t("social.groupInfo.joined")}`;
}

function formatTime(raw?: number) {
  if (!raw) return "";
  const timestamp = raw < 1_000_000_000_000 ? raw * 1000 : raw;
  const date = new Date(timestamp);
  return `${new Intl.DateTimeFormat(locale.value, { month: "numeric", day: "numeric" }).format(date)} ${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
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
    joinPolicy.value = Number(groupInfo.needVerification || 1);
    groupMuted.value = Boolean(groupInfo.allMuted) || Number(groupInfo.status || 0) === 3;
    uni.setNavigationBarTitle({ title: groupName.value || t("social.groupInfo.title") });

    if (canManage.value) {
      const pending = await getChatGroupApplications(0, 500);
      applications.value = pending.filter(
        (application) =>
          application.groupID === groupID.value && Number(application.handleResult || 0) === 0
      );
    } else {
      applications.value = [];
    }

    if (initialSection.value === "applications" && applications.value.length) {
      await nextTick();
      initialSection.value = "";
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("social.groupInfo.loadFailed"), icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

async function saveGroup() {
  const title = groupName.value.trim();
  if (!canManage.value || saving.value || !title) {
    if (!title) uni.showToast({ title: t("social.groupInfo.nameRequired"), icon: "none" });
    return;
  }
  saving.value = true;
  try {
    await updateChatGroup(groupID.value, {
      groupName: title,
      introduction: introduction.value.trim(),
      notification: notification.value.trim(),
      needVerification: joinPolicy.value
    });
    uni.showToast({ title: t("social.groupInfo.saved"), icon: "none" });
    await load();
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("social.groupInfo.saveFailed"), icon: "none" });
  } finally {
    saving.value = false;
  }
}

function chooseJoinPolicy() {
  if (!canManage.value) return;
  const options = [
    t("social.groupInfo.adminReviewOption"),
    t("social.groupInfo.directJoinOption"),
    t("social.groupInfo.noExternalJoinOption")
  ];
  uni.showActionSheet({
    itemList: options,
    success: ({ tapIndex }) => {
      joinPolicy.value = tapIndex + 1;
    }
  });
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
    if (group.value) {
      group.value.allMuted = next;
      group.value.status = next ? 3 : 0;
    }
    uni.showToast({ title: next ? t("social.groupInfo.muteAllEnabled") : t("social.groupInfo.muteAllDisabled"), icon: "none" });
  } catch (error: any) {
    groupMuted.value = !next;
    uni.showToast({ title: error?.message || t("social.groupInfo.settingFailed"), icon: "none" });
  }
}

async function handleApplication(application: ChatGroupApplication, accept: boolean) {
  handlingApplication.value = String(application.applicationID || application.userID);
  try {
    await handleChatGroupApplication(
      application.groupID,
      application.userID,
      accept,
      accept ? t("social.groupInfo.welcomeMessage") : t("social.groupInfo.rejectionMessage")
    );
    applications.value = applications.value.filter(
      (item) => item.applicationID !== application.applicationID
    );
    uni.showToast({ title: accept ? t("social.groupInfo.applicationAccepted") : t("social.groupInfo.applicationRejected"), icon: "none" });
    if (accept) await load();
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("social.groupInfo.handleFailed"), icon: "none" });
  } finally {
    handlingApplication.value = "";
  }
}

function openMember(member: ChatGroupMember) {
  if (canManageMember(member)) {
    manageMember(member);
    return;
  }
  if (member.userID !== String(getSession().uid)) {
    uni.navigateTo({ url: `/pages/user/home?uid=${encodeURIComponent(member.userID)}` });
  }
}

function manageMember(member: ChatGroupMember) {
  const labels: string[] = [t("social.groupInfo.viewProfile")];
  const actions: Array<() => Promise<unknown> | void> = [
    () => uni.navigateTo({ url: `/pages/user/home?uid=${encodeURIComponent(member.userID)}` })
  ];
  const role = Number(member.roleLevel || 0);

  if (isOwner.value) {
    labels.push(role === 60 ? t("social.groupInfo.removeAdmin") : t("social.groupInfo.makeAdmin"));
    actions.push(() =>
      setChatGroupMemberRole(groupID.value, member.userID, role === 60 ? 10 : 60)
    );
  }

  labels.push(memberMuted(member) ? t("social.groupInfo.unmute") : t("social.groupInfo.setMute"));
  actions.push(() => {
    if (memberMuted(member)) {
      return muteChatGroupMember(groupID.value, member.userID, 0);
    }
    return chooseMuteDuration(member);
  });

  labels.push(t("social.groupInfo.removeMember"));
  actions.push(() => confirmKick(member));

  if (isOwner.value) {
    labels.push(t("social.groupInfo.transferOwner"));
    actions.push(() => confirmTransfer(member));
  }

  uni.showActionSheet({
    itemList: labels,
    success: async ({ tapIndex }) => {
      try {
        await actions[tapIndex]?.();
        if (tapIndex > 0) {
          uni.showToast({ title: t("social.common.operationSucceeded"), icon: "none" });
          await load();
        }
      } catch (error: any) {
        uni.showToast({ title: error?.message || t("social.common.operationFailed"), icon: "none" });
      }
    }
  });
}

function chooseMuteDuration(member: ChatGroupMember) {
  return new Promise<unknown>((resolve, reject) => {
    const labels = [
      t("social.groupInfo.tenMinutes"),
      t("social.groupInfo.oneHour"),
      t("social.groupInfo.oneDay"),
      t("social.groupInfo.sevenDays")
    ];
    const durations = [600, 3600, 86_400, 604_800];
    uni.showActionSheet({
      itemList: labels,
      success: ({ tapIndex }) => {
        muteChatGroupMember(groupID.value, member.userID, durations[tapIndex] || 600)
          .then(resolve)
          .catch(reject);
      },
      fail: () => resolve(undefined)
    });
  });
}

function confirmKick(member: ChatGroupMember) {
  return new Promise<unknown>((resolve, reject) => {
    uni.showModal({
      title: t("social.groupInfo.removeMember"),
      content: `${t("social.groupInfo.removePrefix")}${member.nickname || `${t("social.common.user")} ${member.userID}`}${t("social.groupInfo.removeSuffix")}`,
      confirmColor: "#ff4d6e",
      success: ({ confirm }) => {
        if (!confirm) {
          resolve(undefined);
          return;
        }
        kickChatGroupMember(groupID.value, member.userID).then(resolve).catch(reject);
      }
    });
  });
}

function confirmTransfer(member: ChatGroupMember) {
  return new Promise<unknown>((resolve, reject) => {
    uni.showModal({
      title: t("social.groupInfo.transferOwner"),
      content: `${t("social.groupInfo.transferPrefix")}${member.nickname || `${t("social.common.user")} ${member.userID}`}${t("social.groupInfo.transferSuffix")}`,
      confirmColor: "#ff4d6e",
      success: ({ confirm }) => {
        if (!confirm) {
          resolve(undefined);
          return;
        }
        transferChatGroupOwner(groupID.value, member.userID).then(resolve).catch(reject);
      }
    });
  });
}

function copyGroupID() {
  uni.setClipboardData({
    data: groupID.value,
    success: () => uni.showToast({ title: t("social.groupInfo.idCopied"), icon: "none" })
  });
}

function leaveGroup() {
  uni.showModal({
    title: isOwner.value ? t("social.groupInfo.dismissGroup") : t("social.groupInfo.leaveGroup"),
    content: isOwner.value
      ? t("social.groupInfo.dismissConfirm")
      : t("social.groupInfo.leaveConfirm"),
    confirmColor: "#ff4d6e",
    success: async ({ confirm }) => {
      if (!confirm) return;
      try {
        if (isOwner.value) {
          await dismissChatGroup(groupID.value);
        } else {
          await quitChatGroup(groupID.value);
        }
        uni.reLaunch({ url: "/pages/message/index" });
      } catch (error: any) {
        uni.showToast({ title: error?.message || t("social.common.operationFailed"), icon: "none" });
      }
    }
  });
}

onLoad((query) => {
  groupID.value = String(query?.groupid || "");
  initialSection.value = String(query?.section || "");
});

onShow(() => {
  if (groupID.value) void load();
});

onPullDownRefresh(() => void load());
</script>

<style scoped>
.group-page {
  background:
    radial-gradient(circle at 96% 0%, rgba(122, 92, 255, 0.12), transparent 28%),
    var(--bg);
}

.group-hero {
  display: flex;
  min-height: 164rpx;
  align-items: center;
  gap: 22rpx;
  padding: 24rpx;
  margin-bottom: 18rpx;
  border: 1rpx solid rgba(122, 92, 255, 0.1);
  border-radius: 30rpx;
  background: linear-gradient(135deg, #fff 0%, #f9f6ff 100%);
  box-shadow: 0 14rpx 34rpx rgba(55, 43, 106, 0.075);
}

.hero-avatar {
  position: relative;
  display: flex;
  flex: 0 0 104rpx;
  width: 104rpx;
  height: 104rpx;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 34rpx;
  background: var(--grad-cosmic);
  box-shadow: 0 12rpx 26rpx rgba(122, 92, 255, 0.24);
}

.hero-person {
  position: absolute;
  top: 24rpx;
  width: 25rpx;
  height: 25rpx;
  border: 4rpx solid rgba(255, 255, 255, 0.96);
  border-radius: 50%;
}

.hero-person-a {
  left: 23rpx;
}

.hero-person-b {
  right: 23rpx;
}

.hero-people-body {
  position: absolute;
  bottom: 21rpx;
  width: 67rpx;
  height: 28rpx;
  border: 4rpx solid rgba(255, 255, 255, 0.96);
  border-bottom: 0;
  border-radius: 35rpx 35rpx 0 0;
}

.hero-copy,
.application-main,
.member-main {
  flex: 1;
  min-width: 0;
}

.hero-title-line,
.member-name-line {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10rpx;
}

.hero-title {
  display: block;
  min-width: 0;
  overflow: hidden;
  color: var(--ink);
  font-size: 31rpx;
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.role-tag,
.member-tag,
.section-count,
.saving-tag,
.announcement-icon,
.mini-button,
.invite-button,
.group-id-button,
.policy-value,
.save-button,
.leave-button,
.clear-search {
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
}

.role-tag {
  flex: 0 0 auto;
  height: 34rpx;
  padding: 0 11rpx;
  border-radius: 11rpx;
  color: var(--violet);
  font-size: 17rpx;
  font-weight: 900;
  background: var(--violet-soft);
}

.hero-meta {
  display: block;
  margin-top: 8rpx;
  color: var(--ink-3);
  font-size: 21rpx;
}

.group-id-button {
  width: fit-content;
  height: 40rpx;
  justify-content: center;
  gap: 8rpx;
  margin-top: 8rpx;
  color: #7f8796;
  font-size: 19rpx;
}

.copy-mark {
  position: relative;
  width: 17rpx;
  height: 17rpx;
  border: 2rpx solid currentColor;
  border-radius: 4rpx;
}

.copy-mark::after {
  position: absolute;
  top: -6rpx;
  left: 4rpx;
  width: 13rpx;
  height: 13rpx;
  border-top: 2rpx solid currentColor;
  border-right: 2rpx solid currentColor;
  border-radius: 3rpx;
  content: "";
}

.announcement-card {
  display: flex;
  align-items: flex-start;
  gap: 15rpx;
  padding: 18rpx 20rpx;
  margin-bottom: 22rpx;
  border: 1rpx solid rgba(255, 77, 110, 0.08);
  border-radius: 22rpx;
  background: linear-gradient(135deg, #fff6f8, #fff9f2);
}

.announcement-icon {
  flex: 0 0 42rpx;
  width: 42rpx;
  height: 42rpx;
  border-radius: 14rpx;
  color: #fff;
  font-size: 23rpx;
  font-weight: 900;
  background: var(--grad-brand);
}

.announcement-copy {
  flex: 1;
  min-width: 0;
}

.announcement-copy text {
  display: block;
}

.announcement-copy text:first-child {
  color: var(--ink);
  font-size: 22rpx;
  font-weight: 900;
}

.announcement-copy text:last-child {
  margin-top: 7rpx;
  color: var(--ink-2);
  font-size: 21rpx;
  line-height: 1.55;
}

.section {
  margin-bottom: 24rpx;
}

.section-head {
  display: flex;
  min-height: 60rpx;
  align-items: center;
  justify-content: space-between;
  gap: 18rpx;
  padding: 0 4rpx;
  margin-bottom: 12rpx;
}

.section-title,
.section-desc {
  display: block;
}

.section-title {
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 900;
}

.section-desc {
  margin-top: 5rpx;
  color: var(--ink-3);
  font-size: 19rpx;
}

.section-count,
.saving-tag {
  min-width: 42rpx;
  height: 42rpx;
  padding: 0 11rpx;
  border-radius: 21rpx;
  color: #fff;
  font-size: 19rpx;
  font-weight: 900;
  background: var(--brand);
}

.saving-tag {
  min-width: 78rpx;
  color: var(--violet);
  background: var(--violet-soft);
}

.panel {
  overflow: hidden;
  border: 1rpx solid rgba(231, 233, 240, 0.88);
  border-radius: 25rpx;
  background: #fff;
  box-shadow: 0 8rpx 24rpx rgba(25, 27, 38, 0.04);
}

.application-row {
  display: flex;
  min-height: 122rpx;
  align-items: center;
  gap: 14rpx;
  padding: 17rpx;
  border-bottom: 1rpx solid var(--line);
}

.application-row:last-child {
  border-bottom: 0;
}

.application-row image {
  flex: 0 0 64rpx;
  width: 64rpx;
  height: 64rpx;
  border-radius: 21rpx;
  background: #eef0f5;
}

.application-main text {
  display: block;
  overflow: hidden;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.application-main text:first-child {
  color: var(--ink);
  font-size: 23rpx;
  font-weight: 900;
}

.application-main text:nth-child(2) {
  margin-top: 6rpx;
  color: var(--ink-2);
  font-size: 20rpx;
}

.application-main text:last-child {
  margin-top: 5rpx;
  color: var(--ink-3);
  font-size: 17rpx;
}

.application-actions {
  display: flex;
  flex: 0 0 auto;
  gap: 8rpx;
}

.mini-button {
  width: 72rpx;
  height: 52rpx;
  border-radius: 17rpx;
  font-size: 19rpx;
  font-weight: 900;
}

.mini-button.reject {
  color: var(--ink-2);
  background: #eef0f4;
}

.mini-button.accept {
  color: #fff;
  background: var(--violet);
}

.profile-panel {
  padding: 22rpx;
}

.form-field {
  position: relative;
  margin-bottom: 20rpx;
}

.field-label {
  display: block;
  margin-bottom: 10rpx;
  color: var(--ink);
  font-size: 22rpx;
  font-weight: 900;
}

.form-field input,
.form-field textarea {
  width: 100%;
  padding: 17rpx 20rpx;
  border: 2rpx solid transparent;
  border-radius: 18rpx;
  color: var(--ink);
  font-size: 24rpx;
  background: #f4f5f8;
}

.form-field input {
  height: 72rpx;
  padding-right: 88rpx;
}

.form-field textarea {
  height: 132rpx;
  padding-bottom: 38rpx;
}

.form-field input:focus,
.form-field textarea:focus {
  border-color: rgba(122, 92, 255, 0.24);
  background: #fff;
}

.form-field input[disabled],
.form-field textarea[disabled] {
  color: var(--ink-2);
  opacity: 1;
}

.field-count {
  position: absolute;
  right: 14rpx;
  bottom: 11rpx;
  color: #b0b5bf;
  font-size: 16rpx;
}

.policy-row {
  display: flex;
  width: 100%;
  min-height: 88rpx;
  align-items: center;
  justify-content: space-between;
  gap: 18rpx;
  padding: 13rpx 2rpx;
  border-top: 1rpx solid var(--line);
  text-align: left;
}

.policy-row > view:first-child {
  flex: 1;
  min-width: 0;
}

.policy-row > view:first-child text {
  display: block;
}

.policy-row > view:first-child text:first-child {
  color: var(--ink);
  font-size: 23rpx;
  font-weight: 900;
}

.policy-row > view:first-child text:last-child {
  margin-top: 6rpx;
  color: var(--ink-3);
  font-size: 19rpx;
}

.policy-value {
  flex: 0 0 auto;
  gap: 10rpx;
  color: var(--violet);
  font-size: 21rpx;
  font-weight: 800;
}

.chevron,
.member-chevron {
  width: 13rpx;
  height: 13rpx;
  border-top: 3rpx solid currentColor;
  border-right: 3rpx solid currentColor;
  transform: rotate(45deg);
}

.save-button {
  width: 100%;
  height: 76rpx;
  margin-top: 12rpx;
  border-radius: 24rpx;
  color: #fff;
  font-size: 25rpx;
  font-weight: 900;
  background: var(--grad-cosmic);
  box-shadow: 0 10rpx 24rpx rgba(122, 92, 255, 0.2);
}

.save-button[disabled] {
  opacity: 0.44;
}

.invite-button {
  flex: 0 0 auto;
  height: 54rpx;
  gap: 7rpx;
  padding: 0 16rpx;
  border-radius: 18rpx;
  color: var(--violet);
  font-size: 20rpx;
  font-weight: 900;
  background: var(--violet-soft);
}

.invite-button text:first-child {
  font-size: 28rpx;
  font-weight: 500;
}

.member-search {
  display: flex;
  height: 70rpx;
  align-items: center;
  gap: 13rpx;
  padding: 0 17rpx;
  margin-bottom: 12rpx;
  border: 1rpx solid var(--line);
  border-radius: 21rpx;
  background: #fff;
}

.search-mark {
  position: relative;
  width: 22rpx;
  height: 22rpx;
  border: 3rpx solid #a7adba;
  border-radius: 50%;
}

.search-mark::after {
  position: absolute;
  right: -8rpx;
  bottom: -5rpx;
  width: 10rpx;
  height: 3rpx;
  background: #a7adba;
  content: "";
  transform: rotate(45deg);
}

.member-search input {
  flex: 1;
  height: 68rpx;
  color: var(--ink);
  font-size: 23rpx;
}

.clear-search {
  width: 40rpx;
  height: 40rpx;
  border-radius: 50%;
  color: #9ba2af;
  font-size: 29rpx;
  background: #eef0f4;
}

.member-row {
  display: flex;
  width: 100%;
  min-height: 102rpx;
  align-items: center;
  justify-content: flex-start;
  gap: 14rpx;
  padding: 14rpx 17rpx;
  border-bottom: 1rpx solid var(--line);
  text-align: left;
}

.member-row:last-child {
  border-bottom: 0;
}

.member-avatar-wrap {
  position: relative;
  flex: 0 0 66rpx;
  width: 66rpx;
  height: 66rpx;
}

.member-avatar-wrap image {
  width: 66rpx;
  height: 66rpx;
  border-radius: 22rpx;
  background: #eef0f5;
}

.self-dot {
  position: absolute;
  right: -2rpx;
  bottom: -2rpx;
  width: 18rpx;
  height: 18rpx;
  border: 4rpx solid #fff;
  border-radius: 50%;
  background: var(--green);
}

.member-name {
  display: block;
  min-width: 0;
  overflow: hidden;
  color: var(--ink);
  font-size: 24rpx;
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.member-tag {
  flex: 0 0 auto;
  height: 31rpx;
  padding: 0 9rpx;
  border-radius: 10rpx;
  font-size: 16rpx;
  font-weight: 900;
}

.member-tag.role {
  color: var(--violet);
  background: var(--violet-soft);
}

.member-tag.muted {
  color: var(--brand-deep);
  background: var(--brand-soft);
}

.member-id {
  display: block;
  margin-top: 7rpx;
  overflow: hidden;
  color: var(--ink-3);
  font-size: 18rpx;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.member-more {
  display: flex;
  flex: 0 0 48rpx;
  width: 48rpx;
  height: 48rpx;
  align-items: center;
  justify-content: center;
  gap: 4rpx;
  border-radius: 16rpx;
  background: #f0f1f5;
}

.member-more view {
  width: 5rpx;
  height: 5rpx;
  border-radius: 50%;
  background: #858d9b;
}

.member-chevron {
  flex: 0 0 13rpx;
  color: #bdc2cc;
}

.member-empty {
  padding: 54rpx 20rpx;
  color: var(--ink-3);
  font-size: 22rpx;
  text-align: center;
}

.setting-row {
  display: flex;
  min-height: 94rpx;
  align-items: center;
  justify-content: space-between;
  gap: 18rpx;
  padding: 16rpx 20rpx;
}

.setting-row > view {
  flex: 1;
  min-width: 0;
}

.setting-row text {
  display: block;
}

.setting-row text:first-child {
  color: var(--ink);
  font-size: 24rpx;
  font-weight: 900;
}

.setting-row text:last-child {
  margin-top: 7rpx;
  color: var(--ink-3);
  font-size: 19rpx;
}

.leave-button {
  width: 100%;
  height: 80rpx;
  margin-top: 8rpx;
  border: 1rpx solid rgba(255, 77, 110, 0.1);
  border-radius: 24rpx;
  color: var(--brand-deep);
  font-size: 25rpx;
  font-weight: 900;
  background: #fff;
}

.leave-button.danger {
  color: #fff;
  border-color: transparent;
  background: var(--grad-brand);
  box-shadow: 0 10rpx 24rpx rgba(255, 77, 110, 0.2);
}
</style>
