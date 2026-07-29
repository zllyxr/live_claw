<template>
  <view class="safe-page invite-page">
    <view class="invite-hero">
      <image class="app-icon" src="/static/brand/icon-round.webp" mode="aspectFill" />
      <view class="hero-main">
        <text class="title">邀请好友加入星域</text>
        <text class="desc">{{ heroDesc }}</text>
      </view>
    </view>

    <view class="code-card card">
      <view class="profile-row">
        <image class="avatar" :src="avatarUrl" mode="aspectFill" />
        <view class="profile-main">
          <text>{{ name }}</text>
          <text>ID: {{ session.uid }}</text>
        </view>
      </view>

      <view class="state-strip" :class="{ ok: hasAgent, warn: canBind }">
        <text>{{ stateTitle }}</text>
        <text>{{ stateDesc }}</text>
      </view>

      <view class="qr-box">
        <image v-if="qrUrl" class="qr" :src="qrUrl" mode="aspectFit" />
        <view v-else class="qr-empty">{{ loading ? "加载中" : "暂无二维码" }}</view>
      </view>

      <view class="invite-code">
        <text>邀请码</text>
        <text>{{ ownInviteCode || "-" }}</text>
      </view>

      <view class="invite-link">
        <text>邀请链接</text>
        <text>{{ inviteLink || "暂无链接" }}</text>
      </view>

      <view class="actions">
        <button @tap="copyCode">复制邀请码</button>
        <button @tap="copyLink">复制邀请链接</button>
      </view>
    </view>

    <view v-if="canBind" class="bind-card card">
      <text class="bind-title">{{ mustBind ? "填写邀请码（必填）" : "填写邀请人邀请码" }}</text>
      <text class="bind-desc">{{ mustBind ? "当前账号需要绑定邀请关系后继续使用。" : "如果你是被好友邀请来的，可以在这里补填邀请码。" }}</text>
      <view class="bind-row">
        <input v-model.trim="bindCode" placeholder="请输入邀请人邀请码" maxlength="20" />
        <button :disabled="binding" @tap="bindCodeNow">{{ binding ? "提交中" : "绑定" }}</button>
      </view>
    </view>

    <view v-else class="bind-card card done-card">
      <text class="bind-title">{{ hasAgent ? "已绑定邀请关系" : "无需填写邀请码" }}</text>
      <text class="bind-desc">{{ hasAgent ? "当前账号已经设置过邀请人，不能再次修改。" : "当前未开启邀请码绑定，继续分享你的邀请码即可。" }}</text>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad, onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import { bindInviteAttribution, bindInviteCode, checkInviteAgent, getBaseInfo, getInviteCode } from "@/api/services";
import type { InviteAgentState, InviteBindResult, InviteCode, UserProfile } from "@/types/api";
import { PUBLIC_WEB_URL } from "@/constants/config";
import { absolutizeUrl, firstText } from "@/utils/url";
import { clearPendingInvite, normalizeInviteCode, pickInviteParams, savePendingInvite, truthyFlag, type PendingInviteParams } from "@/utils/invite";
import { getSession, isLoggedIn, requireLogin } from "@/utils/session";

const user = ref<UserProfile>();
const invite = ref<InviteCode>();
const agentState = ref<InviteAgentState>();
const routeInvite = ref<PendingInviteParams>();
const bindCode = ref("");
const loading = ref(false);
const binding = ref(false);
let attemptedRouteBind = "";

const session = computed(() => getSession());
const name = computed(() => firstText(user.value?.user_nicename, user.value?.user_nickname, "星域用户"));
const avatarUrl = computed(() => absolutizeUrl(firstText(user.value?.avatar_thumb, user.value?.avatar)) || "/static/brand/icon-round.webp");
const qrUrl = computed(() => absolutizeUrl(firstText(invite.value?.qr, invite.value?.qrcode)));
const ownInviteCode = computed(() => normalizeInviteCode(invite.value?.code));
const hasAgent = computed(() => truthyFlag(agentState.value?.has_agent));
const inviteEnabled = computed(() => truthyFlag(agentState.value?.agent_switch));
const mustBind = computed(() => truthyFlag(agentState.value?.agent_must));
const canBind = computed(() => inviteEnabled.value && !hasAgent.value);
const heroDesc = computed(() => {
  if (hasAgent.value) {
    return "分享自己的邀请码给好友，当前账号的邀请关系已完成绑定。";
  }
  if (canBind.value) {
    return "上方是你的推广信息；下方可填写邀请人的邀请码。";
  }
  return "复制邀请码或分享链接，好友注册后可建立邀请关系。";
});
const stateTitle = computed(() => {
  if (loading.value) {
    return "正在加载邀请信息";
  }
  if (hasAgent.value) {
    return "已绑定邀请人";
  }
  if (canBind.value) {
    return mustBind.value ? "邀请码待填写" : "可填写邀请人";
  }
  return "邀请绑定未开启";
});
const stateDesc = computed(() => {
  if (hasAgent.value) {
    return "已设置的邀请关系不可修改。";
  }
  if (canBind.value) {
    return mustBind.value ? "请填写邀请人邀请码完成绑定。" : "被好友邀请时可在下方补填邀请码。";
  }
  return "你仍然可以分享自己的邀请码和链接。";
});
const inviteLink = computed(() => {
  if (ownInviteCode.value) {
    return `${PUBLIC_WEB_URL}?ref=${encodeURIComponent(ownInviteCode.value)}`;
  }
  const direct = firstText(invite.value?.href, invite.value?.url, invite.value?.link);
  return direct ? absolutizeUrl(direct) : "";
});

function isSelfInvite(code: string) {
  return Boolean(code && ownInviteCode.value && code === ownInviteCode.value);
}

function isBoundResult(result?: InviteBindResult) {
  return truthyFlag(result?.bound) || truthyFlag(result?.already_bound);
}

async function attemptAutoBind() {
  const params = routeInvite.value;
  const code = normalizeInviteCode(firstText(params?.code, params?.ref));
  const key = `${code}:${params?.clickId || ""}`;
  if (!params || (!code && !params.clickId) || attemptedRouteBind === key || hasAgent.value || !inviteEnabled.value) {
    return false;
  }
  attemptedRouteBind = key;
  if (isSelfInvite(code)) {
    clearPendingInvite();
    return false;
  }
  try {
    const result = await bindInviteAttribution(params);
    clearPendingInvite();
    if (isBoundResult(result)) {
      uni.showToast({ title: result?.msg || "邀请关系已绑定", icon: "none" });
      return true;
    }
  } catch {
    clearPendingInvite();
  }
  return false;
}

async function load() {
  if (!isLoggedIn()) {
    savePendingInvite(routeInvite.value);
    requireLogin();
    uni.stopPullDownRefresh();
    return;
  }
  loading.value = true;
  try {
    const [profile, code, state] = await Promise.all([getBaseInfo(), getInviteCode(), checkInviteAgent()]);
    user.value = profile;
    invite.value = code;
    agentState.value = state;
    const bound = await attemptAutoBind();
    if (bound) {
      agentState.value = await checkInviteAgent();
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || "邀请信息加载失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

function copy(value: string, title: string) {
  if (!value) {
    uni.showToast({ title: "暂无可复制内容", icon: "none" });
    return;
  }
  uni.setClipboardData({
    data: value,
    success: () => uni.showToast({ title, icon: "none" })
  });
}

function copyCode() {
  copy(ownInviteCode.value, "邀请码已复制");
}

function copyLink() {
  copy(inviteLink.value, "邀请链接已复制");
}

async function bindCodeNow() {
  if (!requireLogin() || binding.value) {
    return;
  }
  if (!bindCode.value) {
    uni.showToast({ title: "请输入邀请码", icon: "none" });
    return;
  }
  const code = normalizeInviteCode(bindCode.value);
  if (isSelfInvite(code)) {
    uni.showToast({ title: "不能绑定自己的邀请码", icon: "none" });
    return;
  }
  if (!canBind.value) {
    uni.showToast({ title: hasAgent.value ? "已设置，不能更改" : "当前无需填写邀请码", icon: "none" });
    return;
  }
  binding.value = true;
  try {
    const result = await bindInviteCode(code);
    uni.showToast({ title: result?.msg || "绑定成功", icon: "none" });
    bindCode.value = "";
    agentState.value = await checkInviteAgent();
  } catch (error: any) {
    uni.showToast({ title: error?.message || "绑定失败", icon: "none" });
  } finally {
    binding.value = false;
  }
}

onLoad((query) => {
  routeInvite.value = pickInviteParams(query as Record<string, unknown> | undefined);
  if (routeInvite.value) {
    bindCode.value = normalizeInviteCode(firstText(routeInvite.value.code, routeInvite.value.ref));
    savePendingInvite(routeInvite.value);
  }
});

onShow(() => {
  void load();
});

onPullDownRefresh(() => {
  void load();
});
</script>

<style scoped>
.invite-page {
  background: var(--bg);
}

.invite-hero {
  display: flex;
  align-items: center;
  gap: 22rpx;
  min-height: 190rpx;
  padding: 30rpx;
  border-radius: 24rpx;
  color: #fff;
  background: linear-gradient(135deg, #23283d, var(--brand) 58%, #ff8a4d);
}

.app-icon {
  width: 96rpx;
  height: 96rpx;
  border: 4rpx solid rgba(255, 255, 255, 0.6);
  border-radius: 28rpx;
}

.hero-main {
  flex: 1;
  min-width: 0;
}

.title {
  display: block;
  font-size: 34rpx;
  font-weight: 900;
}

.desc {
  display: block;
  margin-top: 16rpx;
  color: rgba(255, 255, 255, 0.86);
  font-size: 24rpx;
  line-height: 1.45;
}

.code-card {
  margin-top: 22rpx;
  padding: 28rpx;
}

.profile-row {
  display: flex;
  align-items: center;
  gap: 18rpx;
}

.avatar {
  width: 78rpx;
  height: 78rpx;
  border-radius: 50%;
  background: var(--line);
}

.profile-main {
  flex: 1;
  min-width: 0;
}

.profile-main text:first-child {
  display: block;
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 900;
}

.profile-main text:last-child {
  display: block;
  margin-top: 10rpx;
  color: var(--ink-3);
  font-size: 23rpx;
}

.state-strip {
  margin-top: 26rpx;
  padding: 18rpx 22rpx;
  border: 1rpx solid #edf0f5;
  border-radius: 18rpx;
  background: #f8fafc;
}

.state-strip.ok {
  border-color: #d8f4e7;
  background: #f0fff8;
}

.state-strip.warn {
  border-color: #ffe3ab;
  background: #fff9eb;
}

.state-strip text:first-child {
  display: block;
  color: var(--ink);
  font-size: 26rpx;
  font-weight: 900;
}

.state-strip text:last-child {
  display: block;
  margin-top: 8rpx;
  color: #7b8494;
  font-size: 23rpx;
  line-height: 1.45;
}

.qr-box {
  display: flex;
  width: 360rpx;
  height: 360rpx;
  align-items: center;
  justify-content: center;
  margin: 42rpx auto 30rpx;
  border: 1rpx solid #edf0f5;
  border-radius: 24rpx;
  background: #fff;
}

.qr {
  width: 310rpx;
  height: 310rpx;
}

.qr-empty {
  color: var(--ink-3);
  font-size: 25rpx;
}

.invite-code {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 92rpx;
  padding: 0 24rpx;
  border-radius: 16rpx;
  background: #fff5f7;
}

.invite-code text:first-child {
  color: #7b8494;
  font-size: 25rpx;
}

.invite-code text:last-child {
  color: var(--brand);
  font-size: 36rpx;
  font-weight: 900;
  letter-spacing: 0;
}

.invite-link {
  margin-top: 16rpx;
  padding: 18rpx 24rpx;
  border-radius: 16rpx;
  background: #f8fafc;
}

.invite-link text:first-child {
  display: block;
  color: #7b8494;
  font-size: 24rpx;
}

.invite-link text:last-child {
  display: block;
  overflow-wrap: anywhere;
  margin-top: 10rpx;
  color: var(--ink);
  font-size: 23rpx;
  line-height: 1.45;
}

.actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18rpx;
  margin-top: 24rpx;
}

.actions button,
.bind-row button {
  display: flex;
  height: 78rpx;
  align-items: center;
  justify-content: center;
  border-radius: 39rpx;
  color: #fff;
  font-size: 26rpx;
  font-weight: 900;
  background: var(--brand);
}

.actions button:first-child {
  color: var(--brand);
  background: #fff1f4;
}

.bind-card {
  margin-top: 22rpx;
  padding: 28rpx;
}

.bind-title {
  display: block;
  color: var(--ink);
  font-size: 30rpx;
  font-weight: 900;
}

.bind-desc {
  display: block;
  margin-top: 12rpx;
  margin-bottom: 22rpx;
  color: #7b8494;
  font-size: 24rpx;
  line-height: 1.45;
}

.bind-row {
  display: flex;
  gap: 16rpx;
}

.bind-row input {
  flex: 1;
  min-width: 0;
  height: 78rpx;
  padding: 0 22rpx;
  border: 1rpx solid #e8ecf4;
  border-radius: 39rpx;
  color: var(--ink);
  font-size: 26rpx;
  background: #f8fafc;
}

.bind-row button {
  width: 132rpx;
  flex: 0 0 auto;
}

.done-card {
  border: 1rpx solid #edf0f5;
  background: #fff;
}

.done-card .bind-desc {
  margin-bottom: 0;
}
</style>
