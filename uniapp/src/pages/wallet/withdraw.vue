<template>
  <view class="safe-page withdraw-page">
    <view class="balance-card">
      <text>{{ t("commerce.withdraw.availableCoin") }}</text>
      <text>{{ profit?.total || profit?.votes || "0" }}</text>
      <text>{{ profit?.tips || t("commerce.withdraw.rulesHint") }}</text>
    </view>

    <view class="section-head">
      <text>{{ t("commerce.withdraw.accounts") }}</text>
      <button @tap="showAccountForm = !showAccountForm">{{ showAccountForm ? t("commerce.common.collapse") : t("commerce.common.add") }}</button>
    </view>

    <view v-if="accounts.length" class="account-list">
      <view
        v-for="account in accounts"
        :key="String(account.id)"
        class="account-card"
        :class="{ active: String(selectedAccountId) === String(account.id) }"
        @tap="selectedAccountId = account.id || ''"
      >
        <view class="account-main">
          <text>{{ accountTypeName(account.type) }} {{ account.account_bank || "" }}</text>
          <text>{{ account.account }}</text>
          <text v-if="account.name">{{ account.name }}</text>
        </view>
        <button @tap.stop="removeAccount(account)">{{ t("commerce.common.delete") }}</button>
      </view>
    </view>
    <EmptyState
      v-else
      :title="loading ? t('commerce.withdraw.loadingAccounts') : t('commerce.withdraw.noAccounts')"
      :description="t('commerce.withdraw.noAccountsDescription')"
    />

    <view v-if="showAccountForm" class="form-card card">
      <picker :range="accountTypeNames" @change="onTypeChange">
        <view class="picker-row">
          <text>{{ t("commerce.withdraw.accountType") }}</text>
          <text>{{ accountTypeNames[accountTypeIndex] }}</text>
        </view>
      </picker>
      <input v-if="accountType === 3" v-model.trim="accountBank" class="input" :placeholder="t('commerce.withdraw.bankName')" />
      <input v-if="accountType === 4" v-model.trim="accountBank" class="input" disabled placeholder="USDT.TRC20" />
      <input v-model.trim="accountNo" class="input" :placeholder="t('commerce.withdraw.accountOrAddress')" />
      <input v-if="accountType !== 4" v-model.trim="accountName" class="input" :placeholder="t('commerce.withdraw.name')" />
      <button class="primary-button compact" :disabled="savingAccount || !accountNo" @tap="saveAccount">
        {{ savingAccount ? t("commerce.common.saving") : t("commerce.withdraw.saveAccount") }}
      </button>
    </view>

    <view class="cash-card card">
      <text class="cash-title">{{ t("commerce.withdraw.amount") }}</text>
      <input v-model.trim="amount" class="amount-input" type="number" :placeholder="t('commerce.withdraw.amountPlaceholder')" />
      <button class="primary-button submit" :disabled="submitting || !selectedAccountId || !amount" @tap="submit">
        {{ submitting ? t("commerce.common.submitting") : t("commerce.withdraw.submitRequest") }}
      </button>
      <button class="ghost-button detail-link" @tap="openCashRecord">{{ t("commerce.withdraw.records") }}</button>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import { addCashAccount, deleteCashAccount, getCashAccounts, getProfit, submitCash } from "@/api/services";
import type { CashAccount } from "@/types/api";
import { useI18n } from "@/i18n";

const { t } = useI18n();

const profit = ref<Record<string, unknown>>();
const accounts = ref<CashAccount[]>([]);
const selectedAccountId = ref<string | number>("");
const loading = ref(false);
const showAccountForm = ref(false);
const savingAccount = ref(false);
const submitting = ref(false);
const amount = ref("");
const accountTypeIndex = ref(0);
const accountNo = ref("");
const accountName = ref("");
const accountBank = ref("");

const accountTypes = [1, 2, 3, 4];
const accountTypeNames = computed(() => [
  t("commerce.withdraw.alipay"),
  t("commerce.withdraw.wechat"),
  t("commerce.withdraw.bankCard"),
  "USDT.TRC20"
]);
const accountType = computed(() => accountTypes[accountTypeIndex.value] || 1);

function accountTypeName(type?: string | number) {
  return accountTypeNames.value[accountTypes.indexOf(Number(type))] || t("commerce.withdraw.account");
}

function onTypeChange(event: any) {
  accountTypeIndex.value = Number(event?.detail?.value || 0);
  accountBank.value = accountType.value === 4 ? "USDT.TRC20" : "";
}

async function load() {
  loading.value = true;
  try {
    const [profitData, accountList] = await Promise.all([getProfit(), getCashAccounts()]);
    profit.value = profitData;
    accounts.value = accountList;
    if (!selectedAccountId.value && accountList[0]?.id) {
      selectedAccountId.value = accountList[0].id;
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("commerce.withdraw.loadFailed"), icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

async function saveAccount() {
  if (!accountNo.value || savingAccount.value) {
    return;
  }
  savingAccount.value = true;
  try {
    const saved = await addCashAccount({
      type: accountType.value,
      account: accountNo.value,
      name: accountName.value,
      accountBank: accountType.value === 4 ? "USDT.TRC20" : accountBank.value
    });
    if (saved) {
      accounts.value.unshift(saved);
      selectedAccountId.value = saved.id || "";
    }
    accountNo.value = "";
    accountName.value = "";
    accountBank.value = "";
    showAccountForm.value = false;
    uni.showToast({ title: t("commerce.withdraw.accountSaved"), icon: "none" });
    void load();
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("commerce.common.saveFailed"), icon: "none" });
  } finally {
    savingAccount.value = false;
  }
}

function removeAccount(account: CashAccount) {
  if (!account.id) {
    return;
  }
  uni.showModal({
    title: t("commerce.withdraw.deleteAccount"),
    content: t("commerce.withdraw.deleteAccountConfirm"),
    confirmColor: "#ff5878",
    success: ({ confirm }) => {
      if (!confirm) {
        return;
      }
      deleteCashAccount(account.id!)
        .then(() => {
          accounts.value = accounts.value.filter((item) => String(item.id) !== String(account.id));
          selectedAccountId.value = accounts.value[0]?.id || "";
          uni.showToast({ title: t("commerce.common.deleted"), icon: "none" });
        })
        .catch((error: any) => uni.showToast({ title: error?.message || t("commerce.common.deleteFailed"), icon: "none" }));
    }
  });
}

async function submit() {
  if (!selectedAccountId.value || !amount.value || submitting.value) {
    return;
  }
  submitting.value = true;
  try {
    const res = await submitCash(selectedAccountId.value, amount.value);
    amount.value = "";
    uni.showToast({ title: res.msg || t("commerce.withdraw.requestSubmitted"), icon: "none" });
    void load();
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("commerce.withdraw.failed"), icon: "none" });
  } finally {
    submitting.value = false;
  }
}

function openCashRecord() {
  uni.navigateTo({ url: "/pages/wallet/detail?type=cash" });
}

onShow(() => {
  uni.setNavigationBarTitle({ title: t("commerce.withdraw.navigationTitle") });
  void load();
});

onPullDownRefresh(() => {
  void load();
});
</script>

<style scoped>
.withdraw-page {
  background: var(--bg);
}

.balance-card {
  min-height: 210rpx;
  padding: 32rpx;
  border-radius: 24rpx;
  color: #fff;
  background: linear-gradient(135deg, #202238, var(--brand) 62%, #ff8a4d);
}

.balance-card text {
  display: block;
}

.balance-card text:first-child {
  font-size: 25rpx;
  opacity: 0.86;
}

.balance-card text:nth-child(2) {
  margin-top: 20rpx;
  font-size: 58rpx;
  font-weight: 900;
  line-height: 1;
}

.balance-card text:last-child {
  margin-top: 20rpx;
  color: rgba(255, 255, 255, 0.82);
  font-size: 23rpx;
  line-height: 1.45;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 34rpx 0 18rpx;
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

.account-card {
  display: flex;
  align-items: center;
  gap: 18rpx;
  min-height: 128rpx;
  padding: 22rpx;
  margin-bottom: 16rpx;
  border: 2rpx solid #edf0f5;
  border-radius: 18rpx;
  background: #fff;
}

.account-card.active {
  border-color: var(--brand);
  background: #fff6f8;
}

.account-main {
  flex: 1;
  min-width: 0;
}

.account-main text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-main text:first-child {
  color: var(--ink);
  font-size: 28rpx;
  font-weight: 900;
}

.account-main text:not(:first-child) {
  margin-top: 8rpx;
  color: #7b8494;
  font-size: 23rpx;
}

.account-card button {
  width: 86rpx;
  height: 52rpx;
  border-radius: 26rpx;
  color: #ff4f62;
  font-size: 22rpx;
  font-weight: 900;
  background: #fff0f2;
}

.form-card,
.cash-card {
  padding: 24rpx;
  margin-top: 20rpx;
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

.input {
  margin-top: 16rpx;
}

.compact {
  height: 82rpx;
  margin-top: 18rpx;
  font-size: 28rpx;
}

.cash-title {
  display: block;
  color: var(--ink);
  font-size: 31rpx;
  font-weight: 900;
}

.amount-input {
  width: 100%;
  height: 92rpx;
  margin-top: 20rpx;
  padding: 0 26rpx;
  border-radius: 18rpx;
  color: var(--ink);
  font-size: 31rpx;
  background: #f8fafc;
}

.submit {
  margin-top: 22rpx;
}

.detail-link {
  width: 100%;
  margin-top: 18rpx;
  color: var(--brand);
  font-weight: 900;
}
</style>
