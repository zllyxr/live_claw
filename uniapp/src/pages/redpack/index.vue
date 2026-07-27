<template>
  <view class="safe-page redpack-page">
    <view class="hero">
      <text>直播间红包</text>
      <text>{{ stream ? "抢红包、查看领取记录，也可以给直播间发红包。" : "缺少直播流信息，无法读取红包。" }}</text>
    </view>

    <view class="tabs">
      <view :class="{ active: tab === 'list' }" @tap="tab = 'list'">红包列表</view>
      <view :class="{ active: tab === 'send' }" @tap="tab = 'send'">发红包</view>
    </view>

    <template v-if="tab === 'list'">
      <view class="section-head">
        <text>当前红包</text>
        <button @tap="load">刷新</button>
      </view>
      <view v-if="packs.length" class="pack-list">
        <view v-for="pack in packs" :key="String(pack.id)" class="pack-card card">
          <image :src="avatarOf(pack)" mode="aspectFill" />
          <view class="pack-main">
            <text>{{ pack.des || "恭喜发财，大吉大利" }}</text>
            <text>{{ pack.user_nickname || pack.user_nicename || "星域用户" }} · {{ typeText(pack) }}</text>
            <text v-if="Number(pack.second || 0) > 0">倒计时 {{ pack.second }} 秒</text>
          </view>
          <button :disabled="Number(pack.isrob || 0) !== 1 || robbingId === String(pack.id)" @tap="rob(pack)">
            {{ Number(pack.isrob || 0) === 1 ? (robbingId === String(pack.id) ? "抢中" : "抢") : "已抢" }}
          </button>
        </view>
      </view>
      <EmptyState v-else :title="loading ? '正在加载红包' : '暂无红包'" description="直播间有红包时会显示在这里。" />
    </template>

    <template v-else>
      <view class="form-card card">
        <picker :range="typeNames" @change="onTypeChange">
          <view class="picker-row">
            <text>红包类型</text>
            <text>{{ typeNames[typeIndex] }}</text>
          </view>
        </picker>
        <picker :range="grantNames" @change="onGrantChange">
          <view class="picker-row">
            <text>发放方式</text>
            <text>{{ grantNames[grantIndex] }}</text>
          </view>
        </picker>
        <input v-model.trim="coin" class="input" type="number" placeholder="星币金额" />
        <input v-model.trim="nums" class="input" type="number" placeholder="红包个数" />
        <input v-model.trim="desc" class="input" maxlength="50" placeholder="恭喜发财，大吉大利" />
        <button class="primary-button submit" :disabled="sending || !stream || !coin || !nums" @tap="send">
          {{ sending ? "发送中" : "发送红包" }}
        </button>
      </view>
    </template>
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onLoad, onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { getRedPacks, robRedPack, sendRedPack } from "@/api/services";
import type { RedPackItem } from "@/types/api";
import { absolutizeUrl } from "@/utils/url";
import { requireLogin } from "@/utils/session";

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
const desc = ref("恭喜发财，大吉大利");

const typeNames = ["普通红包", "手气红包"];
const grantNames = ["立即发放", "延迟发放"];

function avatarOf(pack: RedPackItem) {
  return absolutizeUrl(String(pack.avatar_thumb || pack.avatar || "")) || "/static/brand/icon-round.webp";
}

function typeText(pack: RedPackItem) {
  const grant = Number(pack.type_grant || 0) === 1 ? "延迟" : "立即";
  const type = Number(pack.type || 0) === 1 ? "手气" : "普通";
  return `${grant}${type}`;
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
    uni.showToast({ title: error?.message || "红包加载失败", icon: "none" });
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
    uni.showToast({ title: win > 0 ? `抢到 ${win} 星币` : res?.msg || "手慢了", icon: "none" });
    await load();
  } catch (error: any) {
    uni.showToast({ title: error?.message || "抢红包失败", icon: "none" });
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
    desc.value = "恭喜发财，大吉大利";
    tab.value = "list";
    uni.showToast({ title: "红包已发送", icon: "none" });
    await load();
  } catch (error: any) {
    uni.showToast({ title: error?.message || "发送失败", icon: "none" });
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
