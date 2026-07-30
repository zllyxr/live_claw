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
          <text class="status-tag" :class="statusClass(row)">{{ statusOf(row) }}</text>
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
            <text>{{ textOf(row, ["coin", "coin_amount"], "0") }}</text>
          </view>
          <view>
            <text>支付金额</text>
            <text>
              {{ textOf(row, ["fiat_currency", "currency"], "CNY") }}
              {{ textOf(row, ["money", "amount"], "0") }}
            </text>
          </view>
          <view v-if="Number(textOf(row, ['give', 'bonus_coin'], '0')) > 0">
            <text>赠送</text>
            <text>{{ textOf(row, ["give", "bonus_coin"], "0") }}</text>
          </view>
        </view>

        <view v-if="type === 'charge' && hasPaymentMeta(row)" class="payment-meta">
          <view v-if="providerTradeOf(row)">
            <text>渠道交易号</text>
            <text class="meta-value">{{ providerTradeOf(row) }}</text>
          </view>
          <view v-if="actualAmountOf(row)">
            <text>实际支付</text>
            <text class="meta-value">
              {{ textOf(row, ["crypto_currency"], "USDT") }} {{ actualAmountOf(row) }}
            </text>
          </view>
          <view v-if="networkOf(row)">
            <text>支付链</text>
            <text class="network-tag">{{ networkOf(row) }}</text>
          </view>
          <view v-if="expiresText(row)">
            <text>订单有效期</text>
            <text class="meta-value">{{ expiresText(row) }}</text>
          </view>
          <view v-if="textOf(row, ['block_transaction_id'])" class="wide-meta">
            <text>链上交易</text>
            <text class="meta-value">{{ textOf(row, ["block_transaction_id"]) }}</text>
          </view>
        </view>

        <view v-else-if="type === 'cash'" class="order-main">
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

        <view v-if="type === 'charge'" class="record-actions">
          <button
            class="order-action secondary"
            :disabled="!orderNoOf(row) || refreshingOrderNo === orderNoOf(row)"
            @tap="refreshRechargeOrder(row)"
          >
            {{ refreshingOrderNo === orderNoOf(row) ? "查询中" : "刷新状态" }}
          </button>
          <button
            class="order-action primary"
            :disabled="!canContinueRow(row)"
            @tap="continueRecharge(row)"
          >
            {{ continueLabel(row) }}
          </button>
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
import { onLoad, onPullDownRefresh, onReachBottom, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import {
  getRechargeOrderStatus,
  getRechargeOrders,
  getWalletBalance,
  getWalletLedger,
  getWithdrawalOrders
} from "@/api/services";
import type { RechargeOrder } from "@/types/api";
import { getSession, requireLogin } from "@/utils/session";
import {
  canContinueRecharge,
  clearForeignPendingPayments,
  clearPendingPayment,
  expirationTimestamp,
  openPaymentCashier,
  readPendingPayment,
  rechargeIsExpired,
  rechargeIsPaid,
  rechargeIsTerminal,
  rechargePaymentUrl,
  rechargeProviderTradeId,
  rechargeStatusCode,
  rechargeStatusText,
  savePendingPayment
} from "@/utils/payment";

type AnyRecord = Record<string, unknown>;
type DetailType = "detail" | "charge" | "cash";

const type = ref<DetailType>("detail");
const rows = ref<AnyRecord[]>([]);
const page = ref(1);
const loading = ref(false);
const finished = ref(false);
const refreshingOrderNo = ref("");
const loadedUID = ref("");
let loadRequestSequence = 0;
let orderStatusRequestSequence = 0;
let pendingReturnRequestSequence = 0;

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
  if (type.value === "charge") {
    return rechargeStatusText(row as RechargeOrder);
  }
  return textOf(row, ["status_text"], "处理中");
}

function statusClass(row: AnyRecord) {
  if (type.value !== "charge") {
    return "";
  }
  return ({
    0: "waiting",
    1: "waiting",
    2: "paid",
    3: "failed",
    4: "closed",
    5: "refunded"
  } as Record<number, string>)[rechargeStatusCode(row as RechargeOrder)] || "closed";
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

function orderNoOf(row: AnyRecord) {
  return textOf(row, ["order_no", "orderid", "id"]);
}

function providerTradeOf(row: AnyRecord) {
  return textOf(row, ["provider_trade_id", "provider_order_no"]);
}

function actualAmountOf(row: AnyRecord) {
  return textOf(row, ["actual_amount"]);
}

function networkOf(row: AnyRecord) {
  const network = textOf(row, ["network"]).toLowerCase();
  const tradeType = textOf(row, ["trade_type"]).toLowerCase();
  if (tradeType.includes("trc20")) {
    return "TRON · TRC20";
  }
  if (tradeType.includes("erc20")) {
    return "Ethereum · ERC20";
  }
  if (tradeType.includes("bep20")) {
    return "BSC · BEP20";
  }
  if (network === "tron") {
    return "TRON · TRC20";
  }
  if (network === "ethereum") {
    return "Ethereum · ERC20";
  }
  if (network === "bsc") {
    return "BSC · BEP20";
  }
  return network ? network.toUpperCase() : "";
}

function expiresText(row: AnyRecord) {
  const expiresAt = textOf(row, ["expires_at"]);
  const timestamp = expirationTimestamp(expiresAt);
  if (timestamp > 0) {
    return formatTimestamp(String(timestamp));
  }
  const expiration = Number(textOf(row, ["expiration_time"], "0"));
  if (Number.isFinite(expiration) && expiration > 0) {
    if (expiration > 1_000_000_000) {
      return formatTimestamp(String(Math.floor(expiration)));
    }
    return `剩余约 ${Math.floor(expiration)} 秒`;
  }
  return "";
}

function hasPaymentMeta(row: AnyRecord) {
  return Boolean(
    providerTradeOf(row) ||
      actualAmountOf(row) ||
      networkOf(row) ||
      expiresText(row) ||
      textOf(row, ["block_transaction_id"])
  );
}

function canContinueRow(row: AnyRecord) {
  return canContinueRecharge(row as RechargeOrder);
}

function continueLabel(row: AnyRecord) {
  const order = row as RechargeOrder;
  const status = rechargeStatusCode(order);
  if (status === 2) {
    return "已到账";
  }
  if (status === 3) {
    return "支付失败";
  }
  if (status === 4) {
    return "已关闭";
  }
  if (rechargeIsExpired(order)) {
    return "订单已超时";
  }
  if (status === 5) {
    return "已退款";
  }
  return rechargePaymentUrl(order) ? "继续支付" : "收银台不可用";
}

async function refreshRechargeOrder(row: AnyRecord) {
  const orderNo = orderNoOf(row);
  if (!orderNo) {
    uni.showToast({ title: "充值订单号无效", icon: "none" });
    return;
  }
  if (refreshingOrderNo.value) {
    return;
  }
  const requestSequence = ++orderStatusRequestSequence;
  const uid = String(getSession().uid || "");
  if (!requireLogin() || !uid || uid !== loadedUID.value) {
    uni.showToast({ title: "账号已切换，正在刷新充值记录", icon: "none" });
    void load(true);
    return;
  }
  refreshingOrderNo.value = orderNo;
  try {
    const result = await getRechargeOrderStatus(orderNo);
    if (
      String(getSession().uid || "") !== uid ||
      loadedUID.value !== uid ||
      requestSequence !== orderStatusRequestSequence
    ) {
      return;
    }
    if (!result) {
      throw new Error("充值订单不存在");
    }
    rows.value = rows.value.map((item) =>
      orderNoOf(item) === orderNo ? { ...item, ...result } : item
    );
    const latest = { ...row, ...result } as RechargeOrder;
    const stored = readPendingPayment(uid);
    if (
      stored?.orderNo === orderNo &&
      (rechargeIsTerminal(latest) || rechargeIsExpired(latest))
    ) {
      clearPendingPayment(uid);
    }
    uni.showToast({
      title: rechargeIsPaid(latest) ? "充值已到账" : rechargeStatusText(latest),
      icon: rechargeIsPaid(latest) ? "success" : "none"
    });
  } catch (error: any) {
    if (
      String(getSession().uid || "") === uid &&
      loadedUID.value === uid &&
      requestSequence === orderStatusRequestSequence
    ) {
      uni.showToast({ title: error?.message || "支付状态查询失败", icon: "none" });
    }
  } finally {
    if (requestSequence === orderStatusRequestSequence) {
      refreshingOrderNo.value = "";
    }
  }
}

function continueRecharge(row: AnyRecord) {
  if (!requireLogin()) {
    return;
  }
  const uid = String(getSession().uid || "");
  if (!uid || uid !== loadedUID.value) {
    uni.showToast({ title: "账号已切换，正在刷新充值记录", icon: "none" });
    rows.value = [];
    finished.value = false;
    void load(true);
    return;
  }
  const order = row as RechargeOrder;
  if (!canContinueRecharge(order)) {
    uni.showToast({ title: continueLabel(row), icon: "none" });
    return;
  }
  clearForeignPendingPayments(uid);
  savePendingPayment(uid, order);
  if (!openPaymentCashier(rechargePaymentUrl(order), titleOf(row))) {
    uni.showToast({ title: "支付链接无效", icon: "none" });
  }
}

async function checkPendingOnReturn(uid: string) {
  const requestSequence = ++pendingReturnRequestSequence;
  if (type.value !== "charge") {
    return;
  }
  const stored = readPendingPayment(uid);
  if (!stored) {
    return;
  }
  try {
    const result = await getRechargeOrderStatus(stored.orderNo);
    if (
      String(getSession().uid || "") !== uid ||
      loadedUID.value !== uid ||
      requestSequence !== pendingReturnRequestSequence
    ) {
      return;
    }
    if (!result) {
      return;
    }
    const latest: RechargeOrder = {
      ...result,
      order_no: String(result.order_no || result.orderid || stored.orderNo),
      payment_url: String(rechargePaymentUrl(result) || stored.paymentUrl),
      provider_trade_id: String(
        rechargeProviderTradeId(result) || stored.providerTradeId
      ),
      status: String(result.status ?? stored.status ?? "0"),
      expires_at: String(result.expires_at || stored.expiresAt || "")
    };
    rows.value = rows.value.map((item) =>
      orderNoOf(item) === stored.orderNo ? { ...item, ...latest } : item
    );
    if (rechargeIsPaid(latest)) {
      clearPendingPayment(uid);
      try {
        await getWalletBalance();
      } catch {
        // The order status is authoritative; balance can refresh on the next page show.
      }
      if (
        String(getSession().uid || "") === uid &&
        loadedUID.value === uid &&
        requestSequence === pendingReturnRequestSequence
      ) {
        uni.showToast({ title: "充值已到账", icon: "success" });
      }
      return;
    }
    if (rechargeIsTerminal(latest) || rechargeIsExpired(latest)) {
      clearPendingPayment(uid);
      return;
    }
    savePendingPayment(uid, latest);
  } catch {
    // Returning from the cashier must not block the server-backed order list.
  }
}

async function fetchPage(requestedPage: number) {
  if (type.value === "charge") {
    return getRechargeOrders(requestedPage);
  }
  if (type.value === "cash") {
    return getWithdrawalOrders(requestedPage);
  }
  return getWalletLedger(requestedPage);
}

async function load(reset = false) {
  if (!requireLogin() || (loading.value && !reset) || (finished.value && !reset)) {
    uni.stopPullDownRefresh();
    return;
  }
  const requestSequence = ++loadRequestSequence;
  const uid = String(getSession().uid || "");
  if (reset) {
    page.value = 1;
    finished.value = false;
  }
  const requestedPage = page.value;
  loading.value = true;
  try {
    const result = await fetchPage(requestedPage);
    if (
      String(getSession().uid || "") !== uid ||
      requestSequence !== loadRequestSequence
    ) {
      return;
    }
    const nextRows = listOf(result);
    rows.value = reset ? nextRows : rows.value.concat(nextRows);
    loadedUID.value = uid;
    const total = isRecord(result) ? Number(result.total || 0) : 0;
    finished.value = nextRows.length < 20 || (total > 0 && rows.value.length >= total);
    if (!finished.value) {
      page.value += 1;
    }
  } catch (error: any) {
    if (
      String(getSession().uid || "") === uid &&
      requestSequence === loadRequestSequence
    ) {
      uni.showToast({ title: error?.message || `${pageTitle.value}加载失败`, icon: "none" });
    }
  } finally {
    if (requestSequence === loadRequestSequence) {
      loading.value = false;
      uni.stopPullDownRefresh();
    }
  }
}

function refresh() {
  void load(true);
}

onLoad((query) => {
  const value = String(query?.type || "detail");
  type.value = value === "charge" || value === "cash" ? value : "detail";
  uni.setNavigationBarTitle({ title: pageTitle.value });
});

onShow(() => {
  if (!requireLogin()) {
    return;
  }
  const uid = String(getSession().uid || "");
  orderStatusRequestSequence += 1;
  pendingReturnRequestSequence += 1;
  refreshingOrderNo.value = "";
  clearForeignPendingPayments(uid);
  if (loadedUID.value && loadedUID.value !== uid) {
    rows.value = [];
    finished.value = false;
  }
  void (async () => {
    await load(true);
    if (String(getSession().uid || "") === uid) {
      await checkPendingOnReturn(uid);
    }
  })();
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
  display: inline-flex !important;
  height: 52rpx;
  padding: 0 20rpx;
  align-items: center !important;
  justify-content: center !important;
  border-radius: 26rpx;
  color: #fff;
  font-size: 22rpx;
  line-height: 1 !important;
  text-align: center !important;
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

.status-tag {
  display: inline-flex !important;
  max-width: 240rpx;
  min-height: 42rpx;
  padding: 8rpx 14rpx;
  align-items: center !important;
  justify-content: center !important;
  overflow: hidden;
  border-radius: 18rpx;
  color: var(--brand);
  font-size: 20rpx;
  line-height: 1 !important;
  text-align: center !important;
  text-overflow: ellipsis;
  white-space: nowrap;
  background: #fff1f4;
}

.status-tag.waiting {
  color: #986100;
  background: #fff6dc;
}

.status-tag.paid {
  color: #087d50;
  background: #e8f8f1;
}

.status-tag.failed {
  color: #b4233a;
  background: #ffecef;
}

.status-tag.closed {
  color: #667085;
  background: #edf1f7;
}

.status-tag.refunded {
  color: #6840aa;
  background: #f2ebff;
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

.payment-meta {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12rpx;
  margin-top: 16rpx;
  padding: 18rpx 20rpx;
  border: 1rpx solid #e9edf3;
  border-radius: 16rpx;
  background: #fff;
}

.payment-meta > view {
  min-width: 0;
}

.payment-meta > view > text:first-child {
  display: block;
  color: var(--ink-3);
  font-size: 20rpx;
}

.payment-meta .meta-value {
  display: block;
  margin-top: 8rpx;
  overflow: hidden;
  color: var(--ink);
  font-size: 22rpx;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.payment-meta .wide-meta {
  grid-column: 1 / -1;
}

.network-tag {
  display: inline-flex !important;
  min-height: 38rpx;
  padding: 6rpx 12rpx;
  margin-top: 7rpx;
  align-items: center !important;
  justify-content: center !important;
  border-radius: 19rpx;
  color: #087d50;
  font-size: 19rpx;
  font-weight: 800;
  line-height: 1 !important;
  text-align: center !important;
  background: #e8f8f1;
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

.record-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14rpx;
  margin-top: 20rpx;
}

.order-action {
  display: flex !important;
  width: 100%;
  height: 64rpx;
  align-items: center !important;
  justify-content: center !important;
  border-radius: 32rpx;
  font-size: 23rpx;
  font-weight: 900;
  line-height: 1 !important;
  text-align: center !important;
}

.order-action.secondary {
  border: 1rpx solid #e1e5ec;
  color: var(--ink-2);
  background: #fff;
}

.order-action.primary {
  color: #fff;
  background: var(--grad-brand);
}

.order-action[disabled] {
  color: #a2a8b5;
  background: #eceff4;
  opacity: 1;
}

.list-footer {
  display: block;
  padding: 22rpx 0;
  color: var(--ink-3);
  font-size: 22rpx;
  text-align: center;
}
</style>
