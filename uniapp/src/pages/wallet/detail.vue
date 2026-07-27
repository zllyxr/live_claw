<template>
  <web-view v-if="url" :src="url" />
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onLoad } from "@dcloudio/uni-app";
import { buildCashRecordUrl, buildChargeDetailUrl, buildDetailUrl } from "@/utils/navigation";

const url = ref("");

const pageMap: Record<string, { title: string; url: () => string }> = {
  detail: { title: "我的明细", url: buildDetailUrl },
  charge: { title: "充值明细", url: buildChargeDetailUrl },
  cash: { title: "提现记录", url: buildCashRecordUrl }
};

onLoad((query) => {
  const type = String(query?.type || "detail");
  const page = pageMap[type] ?? pageMap.detail;
  if (!page) {
    return;
  }
  url.value = page.url();
  uni.setNavigationBarTitle({ title: page.title });
});
</script>
