<template>
  <view class="mg-page">
    <!-- 头部 -->
    <view class="mg-hero">
      <view class="hero-stars" />
      <view class="hero-orb" />
      <view class="nav-row">
        <view class="nav-btn" @tap="goBack">
          <text class="nav-arrow">‹</text>
        </view>
        <text class="nav-title">{{ t("misc.minigame.title") }}</text>
        <view class="nav-spacer" />
      </view>
      <text class="hero-sub">{{ t("misc.minigame.subtitle") }}</text>
    </view>

    <!-- 分类筛选 -->
    <scroll-view v-if="categories.length > 1" scroll-x class="cat-strip" :show-scrollbar="false">
      <view class="cat-chip" :class="{ on: activeCat === '' }" @tap="activeCat = ''">{{ t("misc.common.all") }}</view>
      <view
        v-for="cat in categories"
        :key="cat.key"
        class="cat-chip"
        :class="{ on: activeCat === cat.key }"
        @tap="activeCat = String(cat.key || '')"
      >
        {{ categoryName(cat) }}
      </view>
    </scroll-view>

    <!-- 加载骨架 -->
    <view v-if="loading" class="grid">
      <view v-for="i in 4" :key="i" class="sk-card"><view class="sk-shimmer" /></view>
    </view>

    <!-- 游戏列表 -->
    <template v-else-if="shownGames.length">
      <view class="grid">
        <view
          v-for="game in shownGames"
          :key="String(game.code)"
          class="mg-card"
          @tap="launch(game)"
        >
          <view class="art-box">
            <SafeImage class="art" :src="coverOf(game)" :fallback="fallbackCover(game)" mode="aspectFill" />
            <view class="art-mask" />
            <view v-if="game.is_hot === '1'" class="flag hot">HOT</view>
            <view v-else-if="game.is_new === '1'" class="flag new">NEW</view>
            <view class="play-fab">
              <text class="play-tri">▶</text>
            </view>
          </view>
          <view class="mg-copy">
            <text class="mg-name">{{ gameName(game) }}</text>
            <view class="mg-meta">
              <text class="meta-pill">{{ game.players_text }}</text>
              <text class="meta-pill ghost">{{ modeText(game) }}</text>
            </view>
            <text v-if="game.remark" class="mg-remark">{{ game.remark }}</text>
          </view>
        </view>
      </view>
      <text class="mg-foot">{{ t("misc.minigame.totalPrefix") }} {{ shownGames.length }} {{ t("misc.minigame.totalSuffix") }}</text>
    </template>

    <EmptyState v-else kind="bet" :title="t('misc.minigame.empty')" :description="t('misc.minigame.emptyDescription')" />

    <FishingVenuePicker
      :visible="venuePickerVisible"
      :venues="bundle?.fishing_venues"
      @close="venuePickerVisible = false"
      @select="launchFishingVenue"
    />
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import FishingVenuePicker from "@/components/FishingVenuePicker.vue";
import SafeImage from "@/components/SafeImage.vue";
import { enterMiniGame, getMiniGames } from "@/api/services";
import type {
  FishingVenue,
  MiniGameBundle,
  MiniGameCategory,
  MiniGameItem
} from "@/types/api";
import { absolutizeUrl, localAssetUrl } from "@/utils/url";
import { openGameView } from "@/utils/navigation";
import { requireLogin } from "@/utils/session";
import { useI18n } from "@/i18n";

const { locale, t } = useI18n();

const bundle = ref<MiniGameBundle>();
const activeCat = ref("");
const loading = ref(false);
const launching = ref(false);
const venuePickerVisible = ref(false);
const pendingFishingGame = ref<MiniGameItem>();

const categories = computed<MiniGameCategory[]>(() => bundle.value?.categories || []);

const shownGames = computed<MiniGameItem[]>(() => {
  const all = bundle.value?.games || [];
  return activeCat.value ? all.filter((g) => g.category === activeCat.value) : all;
});

function categoryName(category: MiniGameCategory) {
  if (locale.value !== "zh-CN" && String(category.key || "") === "fishing") {
    return t("misc.minigame.fishing");
  }
  return String(category.name || "");
}

function gameName(game: MiniGameItem) {
  if (locale.value !== "zh-CN" && String(game.code || "") === "deepsea_hunter") {
    return t("misc.minigame.deepSeaHunter");
  }
  return String(game.name || "");
}

/** 分类兜底封面：新游戏没配图也不会开天窗 */
const CATEGORY_COVER: Record<string, string> = {
  arcade: "/static/art/category/casino.webp",
  casual: "/static/art/category/board.webp",
  battle: "/static/art/category/sports.webp"
};

/**
 * 封面地址解析。
 * 注册表里以 /static/ 开头的是**打进包里的本地资源**，
 * 必须保持原样（H5 下会相对应用基路径解析）；
 * 其余（上传的图、外链）才走 absolutizeUrl 拼服务端域名。
 */
function coverOf(game: MiniGameItem) {
  const raw = String(game.cover || "").trim();
  if (!raw) {
    return "";
  }
  // 注意：判断条件不能写成带尾斜杠的 "/static/" —— vite 插件会把该字面量
  // 重写成 "/h5/static/"，导致条件永远不成立（已踩过一次）。
  if (raw.startsWith("/static")) {
    // 运行时拿到的本地资源要按应用基路径解析（H5 挂在 /h5/ 下）
    return localAssetUrl(raw);
  }
  return absolutizeUrl(raw);
}

function fallbackCover(game: MiniGameItem) {
  return CATEGORY_COVER[String(game.category || "")] || "/static/art/category/board.webp";
}

function modeText(game: MiniGameItem) {
  const modeKey: Record<string, string> = {
    realtime: "realtime",
    single: "single",
    "local-keyboard": "localKeyboard",
    "local-turn-based": "turnBased",
    webrtc: "onlineBattle"
  };
  return t(`misc.minigame.${modeKey[String(game.play_mode || "")] || "casual"}`);
}

async function load() {
  loading.value = true;
  try {
    bundle.value = await getMiniGames();
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.minigame.loadFailed"), icon: "none" });
  } finally {
    loading.value = false;
  }
}

async function launch(game: MiniGameItem) {
  if (launching.value) {
    return;
  }
  if (game.need_login === "1" && !requireLogin()) {
    return;
  }
  if (String(game.code || "") === "deepsea_hunter") {
    pendingFishingGame.value = game;
    venuePickerVisible.value = true;
    return;
  }
  await launchGame(game);
}

async function launchFishingVenue(venue: FishingVenue) {
  const game = pendingFishingGame.value;
  if (!game || launching.value) return;
  venuePickerVisible.value = false;
  await launchGame(game, String(venue.venue_code || "novice"));
}

async function launchGame(game: MiniGameItem, room = "") {
  launching.value = true;
  uni.showLoading({ title: t("misc.minigame.entering"), mask: true });
  try {
    const info = await enterMiniGame(String(game.code || ""), room);
    const url = String(info?.launch_url || "");
    if (!url) {
      throw new Error(t("misc.minigame.invalidUrl"));
    }
    // 方向以 enter 的服务端结果为准；麻将、斗地主、捕鱼均会锁定横屏。
    openGameView(absolutizeUrl(url) || url);
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.minigame.enterFailed"), icon: "none" });
  } finally {
    uni.hideLoading();
    launching.value = false;
  }
}

function goBack() {
  if (getCurrentPages().length > 1) {
    uni.navigateBack();
    return;
  }
  uni.switchTab({ url: "/pages/tabbar/game/index" });
}

onShow(() => {
  void load();
});
</script>

<style scoped>
.mg-page {
  min-height: 100vh;
  padding-bottom: calc(50rpx + env(safe-area-inset-bottom));
  background: var(--bg);
  overflow-x: hidden;
}

/* ---------- 头部 ---------- */
.mg-hero {
  position: relative;
  overflow: hidden;
  padding: calc(24rpx + var(--status-bar-height)) 28rpx 34rpx;
  border-radius: 0 0 34rpx 34rpx;
  background: linear-gradient(140deg, #2a1b6e 0%, #5a2ea6 52%, #b04a96 100%);
  box-shadow: 0 14rpx 36rpx rgba(48, 26, 112, 0.26);
}

.hero-stars {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image:
    radial-gradient(3rpx 3rpx at 16% 26%, rgba(255, 255, 255, 0.85), transparent 100%),
    radial-gradient(2rpx 2rpx at 38% 62%, rgba(255, 255, 255, 0.5), transparent 100%),
    radial-gradient(3rpx 3rpx at 62% 20%, rgba(255, 255, 255, 0.7), transparent 100%),
    radial-gradient(2rpx 2rpx at 86% 48%, rgba(255, 255, 255, 0.45), transparent 100%);
}

.hero-orb {
  position: absolute;
  top: -120rpx;
  right: -80rpx;
  width: 300rpx;
  height: 300rpx;
  border-radius: 50%;
  pointer-events: none;
  background: radial-gradient(circle at 38% 38%, rgba(255, 173, 205, 0.45), rgba(255, 173, 205, 0) 70%);
}

.nav-row {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.nav-btn {
  display: flex;
  width: 64rpx;
  height: 64rpx;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.18);
  backdrop-filter: blur(8px);
}

.nav-arrow {
  margin-top: -4rpx;
  color: #fff;
  font-size: 44rpx;
  font-weight: 300;
  line-height: 1;
}

.nav-title {
  color: #fff;
  font-size: 34rpx;
  font-weight: 800;
  letter-spacing: 3rpx;
}

.nav-spacer {
  width: 64rpx;
}

.hero-sub {
  position: relative;
  z-index: 1;
  display: block;
  margin-top: 18rpx;
  color: rgba(255, 255, 255, 0.7);
  font-size: 23rpx;
  letter-spacing: 1rpx;
}

/* ---------- 分类 ---------- */
.cat-strip {
  width: 100%;
  margin: 24rpx 0 4rpx;
  padding: 0 28rpx;
  white-space: nowrap;
}

.cat-chip {
  display: inline-flex;
  align-items: center;
  height: 62rpx;
  margin-right: 14rpx;
  padding: 0 28rpx;
  border-radius: 999rpx;
  color: var(--ink-2);
  font-size: 25rpx;
  font-weight: 600;
  background: var(--surface);
  box-shadow: var(--shadow-soft);
}

.cat-chip.on {
  color: #fff;
  font-weight: 800;
  background: var(--grad-brand);
  box-shadow: 0 10rpx 20rpx rgba(255, 77, 110, 0.26);
}

/* ---------- 列表 ---------- */
.grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18rpx;
  padding: 24rpx 28rpx 0;
}

.mg-card {
  overflow: hidden;
  border-radius: var(--radius);
  background: var(--surface);
  box-shadow: var(--shadow-soft);
  transition: transform 0.15s ease;
}

.mg-card:active {
  transform: scale(0.96);
}

.art-box {
  position: relative;
  width: 100%;
  aspect-ratio: 4 / 3;
  overflow: hidden;
  background: #ece9f6;
}

.art {
  width: 100%;
  height: 100%;
}

.art-mask {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 46%;
  background: linear-gradient(180deg, rgba(12, 8, 32, 0), rgba(12, 8, 32, 0.5));
}

.flag {
  position: absolute;
  top: 12rpx;
  left: 12rpx;
  padding: 4rpx 14rpx;
  border-radius: 999rpx;
  color: #fff;
  font-size: 17rpx;
  font-weight: 900;
  letter-spacing: 1rpx;
}

.flag.hot {
  background: var(--grad-brand);
}

.flag.new {
  background: linear-gradient(135deg, #11b981, #5be0a8);
}

.play-fab {
  position: absolute;
  right: 14rpx;
  bottom: 14rpx;
  display: flex;
  width: 56rpx;
  height: 56rpx;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 6rpx 16rpx rgba(0, 0, 0, 0.28);
}

.play-tri {
  margin-left: 4rpx;
  color: var(--brand);
  font-size: 22rpx;
}

.mg-copy {
  padding: 16rpx 18rpx 20rpx;
}

.mg-name {
  display: block;
  color: var(--ink);
  font-size: 27rpx;
  font-weight: 800;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mg-meta {
  display: flex;
  gap: 8rpx;
  margin-top: 12rpx;
}

.meta-pill {
  padding: 4rpx 14rpx;
  border-radius: 999rpx;
  color: var(--brand);
  font-size: 18rpx;
  font-weight: 700;
  background: var(--brand-soft);
}

.meta-pill.ghost {
  color: var(--ink-2);
  background: var(--bg);
}

.mg-remark {
  display: block;
  margin-top: 12rpx;
  color: var(--ink-3);
  font-size: 19rpx;
  line-height: 1.45;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.mg-foot {
  display: block;
  margin-top: 28rpx;
  color: var(--ink-3);
  font-size: 21rpx;
  text-align: center;
}

/* ---------- 骨架 ---------- */
.sk-card {
  position: relative;
  overflow: hidden;
  aspect-ratio: 4 / 3.4;
  border-radius: var(--radius);
  background: #ecedf3;
}

.sk-shimmer {
  position: absolute;
  inset: 0;
  background: linear-gradient(100deg, transparent 30%, rgba(255, 255, 255, 0.7) 50%, transparent 70%);
  background-size: 220% 100%;
  animation: shimmer 1.2s ease-in-out infinite;
}

@keyframes shimmer {
  0% {
    background-position: 130% 0;
  }
  100% {
    background-position: -90% 0;
  }
}
</style>
