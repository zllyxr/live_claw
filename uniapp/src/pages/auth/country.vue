<template>
  <view class="country-page">
    <view class="nav-row">
      <image class="back" src="/static/native/back.png" mode="aspectFit" @tap="goBack" />
      <text class="nav-title">{{ t("misc.country.title") }}</text>
      <view class="nav-spacer" />
    </view>

    <view class="search-box">
      <text class="search-icon">⌕</text>
      <input
        v-model.trim="keyword"
        class="search-input"
        confirm-type="search"
        :placeholder="t('misc.country.searchPlaceholder')"
        @input="onSearchInput"
        @confirm="loadCountries(true)"
      />
      <text v-if="keyword" class="clear" @tap="clearSearch">×</text>
    </view>

    <scroll-view scroll-y class="country-scroll" :scroll-into-view="activeAnchor" :show-scrollbar="false">
      <view v-if="loading" class="state-text">{{ t("misc.common.loading") }}</view>
      <view v-else-if="!groups.length" class="state-text">{{ t("misc.common.noResults") }}</view>
      <view v-else>
        <view v-for="group in groups" :id="anchorOf(group.title)" :key="group.title" class="group">
          <view v-if="group.title" class="group-title">{{ group.title }}</view>
          <view v-for="item in group.lists" :key="`${item.tel}-${item.name}`" class="country-row" @tap="selectCountry(item)">
            <text class="country-name">{{ countryName(item) }}</text>
            <text class="country-tel">+{{ item.tel }}</text>
          </view>
        </view>
      </view>
    </scroll-view>

    <view v-if="!keyword" class="index-bar">
      <text v-for="letter in indexLetters" :key="letter" @tap="jumpTo(letter)">{{ letter }}</text>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref } from "vue";
import { onLoad } from "@dcloudio/uni-app";
import { getCountryCodes } from "@/api/services";
import type { CountryCodeGroup, CountryCodeItem } from "@/types/api";
import { saveSelectedCountry } from "@/utils/country";
import { t } from "@/i18n";

interface CountryGroup {
  title: string;
  lists: CountryCodeItem[];
}

const from = ref("login");
const keyword = ref("");
const loading = ref(false);
const groups = ref<CountryGroup[]>([]);
const activeAnchor = ref("");
let searchTimer: number | undefined;

const indexLetters = computed(() => groups.value.map((group) => group.title).filter(Boolean));

onLoad((query) => {
  from.value = String(query?.from || "login");
  void loadCountries();
});

function anchorOf(title: string) {
  return `country-${title}`;
}

function normalizeGroups(rows: Array<CountryCodeGroup | CountryCodeItem>) {
  if (!rows.length) {
    return [];
  }
  const first = rows[0] as CountryCodeGroup;
  if (Array.isArray(first.lists)) {
    return (rows as CountryCodeGroup[])
      .map((group) => ({
        title: String(group.title || ""),
        lists: (group.lists || []).filter((item) => item.tel)
      }))
      .filter((group) => group.lists.length);
  }
  return [
    {
      title: t("misc.country.searchResults"),
      lists: (rows as CountryCodeItem[]).filter((item) => item.tel)
    }
  ];
}

async function loadCountries(isSearch = false) {
  loading.value = true;
  try {
    const list = await getCountryCodes(isSearch ? keyword.value : "");
    groups.value = normalizeGroups(list);
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.country.loadFailed"), icon: "none" });
  } finally {
    loading.value = false;
  }
}

function onSearchInput() {
  if (searchTimer) {
    clearTimeout(searchTimer);
  }
  searchTimer = setTimeout(() => {
    void loadCountries(Boolean(keyword.value));
  }, 300) as unknown as number;
}

function clearSearch() {
  keyword.value = "";
  void loadCountries();
}

function countryName(item: CountryCodeItem) {
  return String(item.name || item.name_en || t("misc.country.countryRegion"));
}

function selectCountry(item: CountryCodeItem) {
  const tel = String(item.tel || "");
  if (!tel) {
    return;
  }
  saveSelectedCountry({
    from: from.value,
    tel,
    name: countryName(item)
  });
  uni.navigateBack();
}

function jumpTo(letter: string) {
  activeAnchor.value = "";
  setTimeout(() => {
    activeAnchor.value = anchorOf(letter);
  }, 20);
}

function goBack() {
  uni.navigateBack();
}

onUnmounted(() => {
  if (searchTimer) {
    clearTimeout(searchTimer);
  }
});
</script>

<style scoped>
.country-page {
  position: relative;
  min-height: 100vh;
  padding: calc(24rpx + var(--status-bar-height)) 28rpx 0;
  color: #23252d;
  background: var(--bg);
}

.nav-row {
  display: flex;
  height: 72rpx;
  align-items: center;
  justify-content: space-between;
}

.back,
.nav-spacer {
  width: 46rpx;
  height: 46rpx;
}

.nav-title {
  font-size: 31rpx;
  font-weight: 900;
}

.search-box {
  display: flex;
  height: 76rpx;
  align-items: center;
  margin: 22rpx 0;
  padding: 0 22rpx;
  border-radius: 18rpx;
  background: #fff;
  box-shadow: 0 8rpx 24rpx rgba(37, 45, 66, 0.06);
}

.search-icon,
.clear {
  width: 42rpx;
  color: #9aa1ae;
  font-size: 30rpx;
  font-weight: 800;
  text-align: center;
}

.search-input {
  flex: 1;
  min-width: 0;
  height: 76rpx;
  color: #242631;
  font-size: 27rpx;
}

.country-scroll {
  height: calc(100vh - 194rpx - var(--status-bar-height));
  padding-bottom: env(safe-area-inset-bottom);
}

.group-title {
  height: 52rpx;
  padding-left: 6rpx;
  color: #8d96a5;
  font-size: 23rpx;
  font-weight: 900;
  line-height: 52rpx;
}

.country-row {
  display: flex;
  min-height: 88rpx;
  align-items: center;
  justify-content: space-between;
  padding: 0 56rpx 0 24rpx;
  border-bottom: 1rpx solid #eef0f5;
  background: #fff;
}

.country-name {
  min-width: 0;
  color: #252732;
  font-size: 28rpx;
  font-weight: 800;
}

.country-tel {
  flex-shrink: 0;
  margin-left: 24rpx;
  color: var(--brand);
  font-size: 26rpx;
  font-weight: 900;
}

.index-bar {
  position: fixed;
  top: 206rpx;
  right: 8rpx;
  z-index: 4;
  display: flex;
  flex-direction: column;
  align-items: center;
  color: var(--brand);
  font-size: 18rpx;
  font-weight: 900;
  line-height: 1.35;
}

.index-bar text {
  width: 36rpx;
  text-align: center;
}

.state-text {
  padding-top: 160rpx;
  color: #8d96a5;
  font-size: 27rpx;
  font-weight: 800;
  text-align: center;
}
</style>
