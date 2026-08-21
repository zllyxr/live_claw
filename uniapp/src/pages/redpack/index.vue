<template>
  <view class="safe-page redpack-page">
    <view class="hero">
      <text>{{ t("misc.redpack.title") }}</text>
      <text>{{ stream ? t("misc.redpack.description") : t("misc.redpack.missingStream") }}</text>
    </view>

    <view class="tabs">
      <view :class="{ active: tab === 'list' }" @tap="tab = 'list'">{{ t("misc.redpack.list") }}</view>
      <view :class="{ active: tab === 'send' }" @tap="tab = 'send'">{{ t("misc.redpack.send") }}</view>
    </view>

    <template v-if="tab === 'list'">
      <view class="section-head">
        <text>{{ t("misc.redpack.current") }}</text>
        <button @tap="load">{{ t("misc.common.refresh") }}</button>
      </view>
      <view v-if="packs.length" class="pack-list">
        <view v-for="pack in packs" :key="String(pack.id)" class="pack-card card">
          <image :src="avatarOf(pack)" mode="aspectFill" />
          <view class="pack-main">
            <text>{{ pack.des || t("misc.redpack.defaultGreeting") }}</text>
            <text>{{ pack.user_nickname || pack.user_nicename || t("misc.common.defaultUser") }} · {{ typeText(pack) }}</text>
            <text v-if="Number(pack.second || 0) > 0">{{ t("misc.redpack.countdown") }} {{ pack.second }} {{ t("misc.redpack.seconds") }}</text>
          </view>
          <button :disabled="Number(pack.isrob || 0) !== 1 || robbingId === String(pack.id)" @tap="rob(pack)">
            {{ Number(pack.isrob || 0) === 1 ? (robbingId === String(pack.id) ? t("misc.redpack.grabbing") : t("misc.redpack.grab")) : t("misc.redpack.grabbed") }}
          </button>
        </view>
      </view>
      <EmptyState v-else :title="loading ? t('misc.redpack.loading') : t('misc.redpack.empty')" :description="t('misc.redpack.emptyDescription')" />
    </template>

    <template v-else>
      <view class="form-card card">
        <picker :range="typeNames" @change="onTypeChange">
          <view class="picker-row">
            <text>{{ t("misc.redpack.type") }}</text>
            <text>{{ typeNames[typeIndex] }}</text>
          </view>
        </picker>
        <picker :range="grantNames" @change="onGrantChange">
          <view class="picker-row">
            <text>{{ t("misc.redpack.grantMethod") }}</text>
            <text>{{ grantNames[grantIndex] }}</text>
          </view>
        </picker>
        <input v-model.trim="coin" class="input" type="number" :placeholder="t('misc.redpack.coinAmount')" />
        <input v-model.trim="nums" class="input" type="number" :placeholder="t('misc.redpack.quantity')" />
        <input v-model.trim="desc" class="input" maxlength="50" :placeholder="t('misc.redpack.defaultGreeting')" />
        <button class="primary-button submit" :disabled="sending || !stream || !coin || !nums" @tap="send">
          {{ sending ? t("misc.common.sending") : t("misc.redpack.sendRedpack") }}
        </button>
      </view>
    </template>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad, onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getRedPacks, robRedPack, sendRedPack } from "@/api/services";
import type { RedPackItem } from "@/types/api";
import { absolutizeUrl } from "@/utils/url";
import { requireLogin } from "@/utils/session";
import { t } from "@/i18n";

const stream = ref("");
const tab = ref<"list" | "send">("list");
const packs = ref<RedPackItem[]>([]);
const loading = ref(false);
const robbingId = ref("");
const sending = ref(false);
const typeIndex = ref(0);
const grantIndex = ref(0);
const coin = ref("");
const nums = ref("");
const desc = ref(t("misc.redpack.defaultGreeting"));

const typeNames = computed(() => [t("misc.redpack.normal"), t("misc.redpack.lucky")]);
const grantNames = computed(() => [t("misc.redpack.immediateGrant"), t("misc.redpack.delayedGrant")]);

function avatarOf(pack: RedPackItem) {
  return absolutizeUrl(String(pack.avatar_thumb || pack.avatar || "")) || "/static/brand/icon-round.webp";
}

function typeText(pack: RedPackItem) {
  const grant = Number(pack.type_grant || 0) === 1 ? t("misc.redpack.delayed") : t("misc.redpack.immediate");
  const type = Number(pack.type || 0) === 1 ? t("misc.redpack.lucky") : t("misc.redpack.normal");
  return `${grant} ${type}`;
}

function onTypeChange(event: any) {
  typeIndex.value = Number(event?.detail?.value || 0);
}

function onGrantChange(event: any) {
  grantIndex.value = Number(event?.detail?.value || 0);
}

async function load() {
  if (!requireLogin() || !stream.value) {
    uni.stopPullDownRefresh();
    return;
  }
  loading.value = true;
  try {
    packs.value = await getRedPacks(stream.value);
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.redpack.loadFailed"), icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

async function rob(pack: RedPackItem) {
  if (!pack.id || robbingId.value || !stream.value || Number(pack.isrob || 0) !== 1) {
    return;
  }
  robbingId.value = String(pack.id);
  try {
    const res = await robRedPack(stream.value, pack.id);
    const win = Number(res?.win || 0);
    uni.showToast({ title: win > 0 ? `${t("misc.redpack.won")} ${win} ${t("misc.redpack.coins")}` : res?.msg || t("misc.redpack.tooSlow"), icon: "none" });
    await load();
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.redpack.grabFailed"), icon: "none" });
  } finally {
    robbingId.value = "";
  }
}

async function send() {
  if (!stream.value || !coin.value || !nums.value || sending.value) {
    return;
  }
  sending.value = true;
  try {
    await sendRedPack({
      stream: stream.value,
      type: typeIndex.value,
      typeGrant: grantIndex.value,
      coin: coin.value,
      nums: nums.value,
      des: desc.value
    });
    coin.value = "";
    nums.value = "";
    desc.value = t("misc.redpack.defaultGreeting");
    tab.value = "list";
    uni.showToast({ title: t("misc.redpack.sent"), icon: "none" });
    await load();
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.common.sendFailed"), icon: "none" });
  } finally {
    sending.value = false;
  }
}

onLoad((query) => {
  stream.value = decodeURIComponent(String(query?.stream || ""));
});

onShow(() => {
  void load();
});

onPullDownRefresh(() => {
  void load();
});
</script>

<style scoped>
.redpack-page {
  background: var(--bg);
}

.hero {
  min-height: 180rpx;
  padding: 34rpx 30rpx;
  border-radius: 24rpx;
  color: #fff;
  background: linear-gradient(135deg, #e4374f, #ff8a4d);
}

.hero text {
  display: block;
}

.hero text:first-child {
  font-size: 40rpx;
  font-weight: 900;
}

.hero text:last-child {
  margin-top: 16rpx;
  color: rgba(255, 255, 255, 0.84);
  font-size: 24rpx;
}

.tabs {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14rpx;
  margin-top: 22rpx;
}

.tabs view {
  display: flex;
  height: 70rpx;
  align-items: center;
  justify-content: center;
  border-radius: 35rpx;
  color: #7b8494;
  font-size: 26rpx;
  font-weight: 900;
  background: #fff;
}

.tabs .active {
  color: #fff;
  background: var(--brand);
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 30rpx 0 18rpx;
}

.section-head text {
  color: var(--ink);
  font-size: 31rpx;
  font-weight: 900;
}

.section-head button {
  display: flex;
  min-width: 104rpx;
  height: 56rpx;
  align-items: center;
  justify-content: center;
  border-radius: 28rpx;
  color: var(--brand);
  font-size: 24rpx;
  font-weight: 900;
  background: #fff1f4;
}

.pack-card {
  display: flex;
  align-items: center;
  gap: 18rpx;
  min-height: 138rpx;
  padding: 22rpx;
  margin-bottom: 16rpx;
}

.pack-card image {
  width: 76rpx;
  height: 76rpx;
  flex: 0 0 auto;
  border-radius: 38rpx;
}

.pack-main {
  flex: 1;
  min-width: 0;
}

.pack-main text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pack-main text:first-child {
  color: var(--ink);
  font-size: 28rpx;
  font-weight: 900;
}

.pack-main text:not(:first-child) {
  margin-top: 8rpx;
  color: var(--ink-3);
  font-size: 22rpx;
}

.pack-card button {
  display: flex;
  width: 94rpx;
  height: 58rpx;
  align-items: center;
  justify-content: center;
  border-radius: 29rpx;
  color: #fff;
  font-size: 24rpx;
  font-weight: 900;
  background: var(--brand);
}

.pack-card button[disabled] {
  color: #98a2b3;
  background: #f2f4f7;
}

.form-card {
  padding: 24rpx;
  margin-top: 26rpx;
}

.picker-row {
  display: flex;
  height: 88rpx;
  align-items: center;
  justify-content: space-between;
  padding: 0 24rpx;
  border-radius: 16rpx;
  color: var(--ink);
  font-size: 27rpx;
  font-weight: 800;
  background: #f8fafc;
}

.picker-row + .picker-row,
.input {
  margin-top: 16rpx;
}

.input {
  height: 88rpx;
}

.submit {
  height: 84rpx;
  margin-top: 20rpx;
  font-size: 28rpx;
}
</style>
