<template>
  <web-view v-if="url" :src="url" />
  <view v-else class="safe-page detail-hub">
    <view class="hub-head">
      <text>详情中心</text>
      <text>旧端 H5 详情与原生详情统一入口</text>
    </view>
    <view class="hub-list">
      <view v-for="item in hubItems" :key="item.type" class="hub-row" @tap="openType(item.type)">
        <view class="hub-icon">{{ item.icon }}</view>
        <view class="hub-main">
          <text>{{ item.title }}</text>
          <text>{{ item.desc }}</text>
        </view>
        <text class="arrow">›</text>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onLoad } from "@dcloudio/uni-app";
import {
  buildAuthUrl,
  buildCashRecordUrl,
  buildChargeDetailUrl,
  buildDetailUrl
} from "@/utils/navigation";

const url = ref("");

const pageMap: Record<string, { title: string; url: () => string }> = {
  detail: { title: "我的明细", url: buildDetailUrl },
  charge: { title: "充值明细", url: buildChargeDetailUrl },
  cash: { title: "提现记录", url: buildCashRecordUrl },
  auth: { title: "认证详情", url: buildAuthUrl }
};

const hubItems = [
  { type: "detail", icon: "明", title: "我的明细", desc: "查看账户收支与直播明细" },
  { type: "charge", icon: "充", title: "充值明细", desc: "查看星币充值记录" },
  { type: "cash", icon: "提", title: "提现记录", desc: "查看收益提现记录" },
  { type: "auth", icon: "认", title: "认证详情", desc: "打开旧版认证详情流程" }
];

function setPage(nextUrl: string, title: string) {
  url.value = nextUrl;
  uni.setNavigationBarTitle({ title });
}

function openType(type: string) {
  const page = pageMap[type];
  if (!page) {
    return;
  }
  setPage(page.url(), page.title);
}

onLoad((query) => {
  const directUrl = String(query?.url || "");
  const directTitle = decodeURIComponent(String(query?.title || "详情"));
  if (directUrl) {
    setPage(decodeURIComponent(directUrl), directTitle);
    return;
  }
  const type = String(query?.type || "");
  if (type && pageMap[type]) {
    openType(type);
    return;
  }
  uni.setNavigationBarTitle({ title: "详情中心" });
});
</script>

<style scoped>
.detail-hub {
  background: var(--bg);
}

.hub-head {
  min-height: 170rpx;
  padding: 32rpx 30rpx;
  border-radius: 24rpx;
  color: #fff;
  background: linear-gradient(135deg, #23283d, var(--brand));
}

.hub-head text:first-child {
  display: block;
  font-size: 38rpx;
  font-weight: 900;
}

.hub-head text:last-child {
  display: block;
  margin-top: 18rpx;
  color: rgba(255, 255, 255, 0.86);
  font-size: 25rpx;
}

.hub-list {
  overflow: hidden;
  margin-top: 22rpx;
  border: 1rpx solid #e9edf4;
  border-radius: 18rpx;
  background: #fff;
}

.hub-row {
  display: grid;
  grid-template-columns: 64rpx minmax(0, 1fr) 34rpx;
  gap: 18rpx;
  align-items: center;
  min-height: 118rpx;
  padding: 20rpx 24rpx;
  border-bottom: 1rpx solid #f0f2f6;
}

.hub-row:last-child {
  border-bottom: 0;
}

.hub-icon {
  display: flex;
  width: 58rpx;
  height: 58rpx;
  align-items: center;
  justify-content: center;
  border-radius: 18rpx;
  color: var(--brand);
  font-size: 22rpx;
  font-weight: 900;
  background: #fff1f4;
}

.hub-main {
  min-width: 0;
}

.hub-main text:first-child {
  display: block;
  color: var(--ink);
  font-size: 28rpx;
  font-weight: 900;
}

.hub-main text:last-child {
  display: block;
  margin-top: 10rpx;
  color: var(--ink-3);
  font-size: 23rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arrow {
  color: #a3aab7;
  font-size: 46rpx;
  line-height: 1;
}
</style>
