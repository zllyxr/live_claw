<template>
  <view class="safe-page wallet-detail-page">
    <view class="detail-hero">
      <text>{{ pageTitle }}</text>
      <text>{{ pageDescription }}</text>
      <button @tap="refresh">刷新</button>
    </view>

    <view v-if="rows.length" class="record-list">
      <view v-for="row in rows" :key="rowKey(row)" class="record-card">
        <view class="record-head">
          <view>
            <text>{{ titleOf(row) }}</text>
            <text>{{ textOf(row, ["datetime"], formatTimestamp(textOf(row, ["addtime"]))) }}</text>
          </view>
          <text class="status">{{ statusOf(row) }}</text>
        </view>

        <view v-if="type === 'detail'" class="ledger-main">
          <view>
            <text>可用变动</text>
            <text :class="{ positive: numberOf(row, 'delta_available') > 0 }">
              {{ signedAmount(row, "delta_available") }}
            </text>
          </view>
          <view>
            <text>冻结变动</text>
            <text>{{ signedAmount(row, "delta_frozen") }}</text>
          </view>
          <view>
            <text>可用余额</text>
            <text>{{ textOf(row, ["balance_available", "balance"], "0") }}</text>
          </view>
        </view>

        <view v-else-if="type === 'charge'" class="order-main">
          <view>
            <text>充值星币</text>
            <text>{{ textOf(row, ["coin"], "0") }}</text>
          </view>
          <view>
            <text>支付金额</text>
            <text>{{ textOf(row, ["currency"], "CNY") }} {{ textOf(row, ["money"], "0") }}</text>
          </view>
          <view v-if="Number(textOf(row, ['give'], '0')) > 0">
            <text>赠送</text>
            <text>{{ textOf(row, ["give"], "0") }}</text>
          </view>
        </view>

        <view v-else class="order-main">
          <view>
            <text>提现星币</text>
            <text>{{ textOf(row, ["coin"], "0") }}</text>
          </view>
          <view>
            <text>到账金额</text>
            <text>{{ textOf(row, ["currency"], "CNY") }} {{ textOf(row, ["money"], "0") }}</text>
          </view>
          <view>
            <text>收款账户</text>
            <text>{{ textOf(row, ["account"], "-") }}</text>
          </view>
        </view>

        <view class="record-foot">
          <text>编号 {{ textOf(row, ["entry_no", "order_no", "orderid", "id"], "-") }}</text>
          <text v-if="textOf(row, ['game_code'])">
            {{ textOf(row, ["game_code"]) }} {{ textOf(row, ["round_no"]) }}
          </text>
          <text v-if="textOf(row, ['reject_reason'])">{{ textOf(row, ["reject_reason"]) }}</text>
        </view>
      </view>
      <text v-if="finished" class="list-footer">没有更多记录了</text>
    </view>
    <EmptyState
      v-else
      :title="loading ? `正在加载${pageTitle}` : `暂无${pageTitle}`"
      description="下拉刷新可重新读取服务端记录。"
    />
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad, onPullDownRefresh, onReachBottom } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import {
  getRechargeOrders,
  getWalletLedger,
  getWithdrawalOrders
} from "@/api/services";
import { requireLogin } from "@/utils/session";

type AnyRecord = Record<string, unknown>;
type DetailType = "detail" | "charge" | "cash";

const type = ref<DetailType>("detail");
const rows = ref<AnyRecord[]>([]);
const page = ref(1);
const loading = ref(false);
const finished = ref(false);

const pageTitle = computed(() => ({
  detail: "我的明细",
  charge: "充值明细",
  cash: "提现记录"
})[type.value]);
const pageDescription = computed(() => ({
  detail: "每笔游戏、直播和资金变动均可追溯",
  charge: "查看充值订单及到账状态",
  cash: "查看提现审核与打款状态"
})[type.value]);

function isRecord(value: unknown): value is AnyRecord {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function textOf(source: AnyRecord, keys: string[], fallback = "") {
  for (const key of keys) {
    const value = source[key];
    if (value !== undefined && value !== null && String(value).trim()) {
      return String(value);
    }
  }
  return fallback;
}

function listOf(source: unknown) {
  if (!isRecord(source)) {
    return [] as AnyRecord[];
  }
  for (const key of ["list", "items"]) {
    const value = source[key];
    if (Array.isArray(value)) {
      return value.filter(isRecord);
    }
  }
  return [] as AnyRecord[];
}

function rowKey(row: AnyRecord) {
  return textOf(row, ["entry_no", "order_no", "orderid", "id"], JSON.stringify(row));
}

function titleOf(row: AnyRecord) {
  if (type.value === "charge") {
    return textOf(row, ["channel_name"], "星币充值");
  }
  if (type.value === "cash") {
    return "星币提现";
  }
  return textOf(row, ["description", "title", "business_type"], "资金变动");
}

function statusOf(row: AnyRecord) {
  if (type.value === "detail") {
    return textOf(row, ["business_type"], "流水");
  }
  return textOf(row, ["status_text"], "处理中");
}

function numberOf(row: AnyRecord, key: string) {
  return Number(row[key] || 0);
}

function signedAmount(row: AnyRecord, key: string) {
  const amount = numberOf(row, key);
  return amount > 0 ? `+${amount}` : String(amount);
}

function formatTimestamp(value: string) {
  const timestamp = Number(value);
  if (!timestamp) {
    return "";
  }
  const date = new Date(timestamp * 1000);
  const part = (item: number) => String(item).padStart(2, "0");
  return `${date.getFullYear()}-${part(date.getMonth() + 1)}-${part(date.getDate())} ${part(date.getHours())}:${part(date.getMinutes())}`;
}

async function fetchPage() {
  if (type.value === "charge") {
    return getRechargeOrders(page.value);
  }
  if (type.value === "cash") {
    return getWithdrawalOrders(page.value);
  }
  return getWalletLedger(page.value);
}

async function load(reset = false) {
  if (!requireLogin() || loading.value || (finished.value && !reset)) {
    uni.stopPullDownRefresh();
    return;
  }
  if (reset) {
    page.value = 1;
    finished.value = false;
  }
  loading.value = true;
  try {
    const result = await fetchPage();
    const nextRows = listOf(result);
    rows.value = reset ? nextRows : rows.value.concat(nextRows);
    const total = isRecord(result) ? Number(result.total || 0) : 0;
    finished.value = nextRows.length < 20 || (total > 0 && rows.value.length >= total);
    if (!finished.value) {
      page.value += 1;
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || `${pageTitle.value}加载失败`, icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

function refresh() {
  void load(true);
}

onLoad((query) => {
  const value = String(query?.type || "detail");
  type.value = value === "charge" || value === "cash" ? value : "detail";
  uni.setNavigationBarTitle({ title: pageTitle.value });
  void load(true);
});

onPullDownRefresh(refresh);
onReachBottom(() => void load(false));
</script>

<style scoped>
.wallet-detail-page {
  min-height: 100vh;
  padding: 24rpx;
  background: var(--bg);
}

.detail-hero {
  position: relative;
  min-height: 166rpx;
  padding: 30rpx;
  border-radius: 24rpx;
  color: #fff;
  background: linear-gradient(135deg, #2b2f48, var(--brand));
}

.detail-hero > text {
  display: block;
}

.detail-hero > text:first-child {
  font-size: 36rpx;
  font-weight: 900;
}

.detail-hero > text:nth-child(2) {
  margin-top: 15rpx;
  color: rgba(255, 255, 255, 0.82);
  font-size: 23rpx;
}

.detail-hero button {
  position: absolute;
  top: 28rpx;
  right: 28rpx;
  height: 52rpx;
  padding: 0 20rpx;
  border-radius: 26rpx;
  color: #fff;
  font-size: 22rpx;
  background: rgba(255, 255, 255, 0.16);
}

.record-list {
  margin-top: 22rpx;
}

.record-card {
  padding: 24rpx;
  margin-bottom: 18rpx;
  border: 1rpx solid #e9edf3;
  border-radius: 20rpx;
  background: #fff;
}

.record-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20rpx;
}

.record-head > view text {
  display: block;
}

.record-head > view text:first-child {
  color: var(--ink);
  font-size: 27rpx;
  font-weight: 900;
}

.record-head > view text:last-child {
  margin-top: 8rpx;
  color: var(--ink-3);
  font-size: 21rpx;
}

.status {
  max-width: 240rpx;
  padding: 8rpx 14rpx;
  overflow: hidden;
  border-radius: 18rpx;
  color: var(--brand);
  font-size: 20rpx;
  text-overflow: ellipsis;
  white-space: nowrap;
  background: #fff1f4;
}

.ledger-main,
.order-main {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12rpx;
  margin-top: 22rpx;
  padding: 20rpx;
  border-radius: 16rpx;
  background: #f7f8fb;
}

.ledger-main view,
.order-main view {
  min-width: 0;
}

.ledger-main text,
.order-main text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ledger-main text:first-child,
.order-main text:first-child {
  color: var(--ink-3);
  font-size: 20rpx;
}

.ledger-main text:last-child,
.order-main text:last-child {
  margin-top: 9rpx;
  color: var(--ink);
  font-size: 25rpx;
  font-weight: 900;
}

.ledger-main text.positive {
  color: #1d9a62;
}

.record-foot {
  display: flex;
  flex-wrap: wrap;
  gap: 8rpx 20rpx;
  margin-top: 18rpx;
}

.record-foot text {
  color: var(--ink-3);
  font-size: 20rpx;
}

.list-footer {
  display: block;
  padding: 22rpx 0;
  color: var(--ink-3);
  font-size: 22rpx;
  text-align: center;
}
</style>
