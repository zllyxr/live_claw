<template>
  <view class="game-page">
    <view class="cosmic-head">
      <view class="brand-copy">
        <image v-if="locale === 'zh-CN'" class="brand-title-art" src="/static/art/game-park/titles/game-center-title-v1.webp" mode="widthFix" />
        <text v-else class="brand-title-text">{{ t("commerce.gameHome.title") }}</text>
      </view>
      <view class="wallet-panel">
        <view class="balance-row">
          <image class="coin-icon" src="/static/brand/icon-round.webp" mode="aspectFit" />
          <text class="balance-value">{{ home?.coin || "0" }}</text>
          <text class="balance-unit">{{ t("commerce.common.coin") }}</text>
        </view>
        <view class="wallet-links">
          <view class="wallet-link" @tap="openRecharge">
            <image src="/static/icons/wallet-recharge.svg" mode="aspectFit" />
            <text>{{ t("commerce.gameHome.recharge") }}</text>
          </view>
          <view class="wallet-rule" />
          <view class="wallet-link" @tap="openWithdraw">
            <image src="/static/icons/wallet-withdraw.svg" mode="aspectFit" />
            <text>{{ t("commerce.gameHome.withdraw") }}</text>
          </view>
        </view>
      </view>
    </view>

    <view class="park-heading">
      <view class="zone-shortcuts">
        <view class="zone-shortcut" @tap="openSports">
          <SafeImage src="/static/art/category/sports.webp" mode="aspectFill" />
          <text>{{ t("commerce.gameHome.sportsEvents") }}</text>
        </view>
        <view class="zone-shortcut" @tap="scrollToLottery">
          <SafeImage src="/static/art/category/lottery.webp" mode="aspectFill" />
          <text>{{ t("commerce.gameHome.lotteryGames") }}</text>
        </view>
      </view>
    </view>

    <view id="game-park" class="park-map">
      <image class="park-art" src="/static/art/game-park/orbital-park.webp" mode="aspectFill"/>
      <view
        v-for="attraction in parkAttractions"
        :key="attraction.code"
        class="map-marker"
        :class="[`marker-${attraction.order}`, { active: selectedCode === attraction.code }]"
        @tap="selectAttraction(attraction.code)"
      >
        <text class="marker-order">{{ attraction.order }}</text>
        <view class="marker-copy">
          <text class="marker-name">{{ attraction.name }}</text>
          <text class="marker-meta">{{ attraction.meta }}</text>
        </view>
      </view>
      <view
        v-if="selectedCode === 'deepsea_hunter'"
        class="map-enter"
        @tap.stop="launchSelected"
      >
        {{ t("commerce.gameHome.enterNow") }}
      </view>
    </view>

    <view class="selected-dock">
      <view class="selected-cover" @tap="launchSelected">
        <SafeImage
          class="selected-cover-img"
          :src="coverOf(selectedGame)"
          fallback="/static/art/game-park/orbital-park.webp"
          mode="aspectFill"
        />
        <text v-if="selectedGame?.is_hot === '1'" class="selected-flag">HOT</text>
      </view>
      <view class="selected-main">
        <view class="selected-title-row">
          <text class="selected-name">{{ selectedGameName }}</text>
          <text class="selected-players">{{ selectedPlayersText }}</text>
        </view>
        <text class="selected-remark">{{ selectedGameRemark }}</text>
        <view class="selected-foot">
          <text class="selected-mode">{{ modeText(selectedGame) }}</text>
          <view class="launch-button" :class="{ disabled: launching }" @tap="launchSelected">
            {{ launching ? t("commerce.gameHome.entering") : t("commerce.gameHome.enterNow") }}
          </view>
        </view>
      </view>
    </view>
    <view id="lottery-panel" class="lottery-panel">
      <view class="lottery-head">
        <view>
          <text class="lottery-title">{{ t("commerce.gameHome.lotteryTitle") }}</text>
          <text class="lottery-subtitle">{{ t("commerce.gameHome.lotterySubtitle") }}</text>
        </view>
      </view>

      <scroll-view
        v-if="normalizedCategories.length"
        scroll-x
        class="lottery-categories"
        :show-scrollbar="false"
      >
        <view
          v-for="category in normalizedCategories"
          :key="String(category.id)"
          class="lottery-category"
          :class="{ active: String(category.id) === selectedCategory }"
          @tap="selectedCategory = String(category.id)"
        >
          {{ category.name || t("commerce.common.all") }}
        </view>
      </scroll-view>

      <view v-if="filteredGames.length" class="lottery-grid">
        <view
          v-for="game in filteredGames"
          :key="String(game.id)"
          class="lottery-game"
          @tap="openLotteryGame(game)"
        >
          <SafeImage
            class="lottery-icon"
            :src="gameIcon(game)"
            fallback="/static/art/category/lottery.webp"
            mode="aspectFill"
          />
          <text class="lottery-name">{{ lotteryGameName(game) }}</text>
          <text class="lottery-code">{{ game.game_code || "GAME" }}</text>
        </view>
      </view>
      <EmptyState
        v-else
        kind="bet"
        :title="loading ? t('commerce.gameHome.loading') : t('commerce.gameHome.emptyCategory')"
        :description="t('commerce.gameHome.emptyDescription')"
      />
    </view>

    <FishingVenuePicker
      :visible="venuePickerVisible"
      :venues="fishingVenues"
      :balance="home?.coin"
      @close="venuePickerVisible = false"
      @select="launchFishingVenue"
    />
  </view>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from "vue";
import { onPullDownRefresh, onShow } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import FishingVenuePicker from "@/components/FishingVenuePicker.vue";
import SafeImage from "@/components/SafeImage.vue";
import { enterMiniGame, getLotteryHome, getMiniGames } from "@/api/services";
import type {
  LotteryCategory,
  LotteryGame,
  LotteryHome,
  FishingVenue,
  MiniGameBundle,
  MiniGameItem
} from "@/types/api";
import { absolutizeUrl, displayUrl, localAssetUrl } from "@/utils/url";
import { consumeGameZoneIntent, openGameView } from "@/utils/navigation";
import { requireLogin } from "@/utils/session";
import { useI18n } from "@/i18n";

const { locale, t } = useI18n();

const home = ref<LotteryHome>();
const miniGames = ref<MiniGameBundle>();
const selectedCategory = ref("");
const selectedCode = ref("deepsea_hunter");
const loading = ref(false);
const launching = ref(false);
const venuePickerVisible = ref(false);
let loadedOnce = false;

const normalizedCategories = computed<LotteryCategory[]>(() => home.value?.categories || []);

const filteredGames = computed(() => {
  const games = home.value?.games || [];
  return selectedCategory.value
    ? games.filter((game) => String(game.category_id || "") === selectedCategory.value)
    : games;
});

const miniGameCount = computed(() => miniGames.value?.total || String(miniGames.value?.games?.length || 0));

const selectedGame = computed<MiniGameItem | undefined>(() => {
  const games = miniGames.value?.games || [];
  return games.find((game) => String(game.code || "") === selectedCode.value) || games[0];
});

function localizedGameField(game: MiniGameItem | undefined, field: "name" | "remark") {
  if (!game) return "";
  const source = game as unknown as Record<string, unknown>;
  const suffixes = locale.value === "zh-CN"
    ? ["", "zh", "cn", "en"]
    : locale.value === "ja"
      ? ["ja", "jp", "en", ""]
      : locale.value === "ko"
        ? ["ko", "kr", "en", ""]
        : ["en", ""];
  for (const suffix of suffixes) {
    const value = source[suffix ? `${field}_${suffix}` : field];
    if (value !== undefined && value !== null && String(value).trim()) return String(value);
  }
  return "";
}

const selectedGameName = computed(() =>
  localizedGameField(selectedGame.value, "name") || t("commerce.gameHome.attractions.deepseaHunter")
);
const selectedGameRemark = computed(() =>
  localizedGameField(selectedGame.value, "remark") || t("commerce.gameHome.deepseaRemark")
);
const selectedPlayersText = computed(() => {
  const value = String(selectedGame.value?.players_text || "");
  const matched = value.match(/^(\d+)\s*[-–]\s*(\d+)\s*人$/);
  if (matched) {
    return t("commerce.gameHome.playerRange")
      .replace("{min}", matched[1] || "")
      .replace("{max}", matched[2] || "");
  }
  return value || t("commerce.gameHome.playerRange").replace("{min}", "1").replace("{max}", "4");
});

const fishingVenues = computed<FishingVenue[]>(() => miniGames.value?.fishing_venues || []);

const attractionConfig = computed(() => [
  { order: 1, code: "deepsea_hunter", name: t("commerce.gameHome.attractions.deepseaHunter"), meta: t("commerce.gameHome.multiplayerLive") },
  { order: 2, code: "ddz", name: t("commerce.gameHome.attractions.landlord"), meta: t("commerce.gameHome.threePlayer") },
  { order: 3, code: "mahjong", name: t("commerce.gameHome.attractions.mahjong"), meta: t("commerce.gameHome.fourPlayer") },
  { order: 4, code: "zhajinhua", name: t("commerce.gameHome.attractions.goldenFlower"), meta: t("commerce.gameHome.threePlayer") },
  { order: 5, code: "paodekuai", name: t("commerce.gameHome.attractions.runFast"), meta: t("commerce.gameHome.threePlayer") },
  { order: 6, code: "mahjong_red", name: t("commerce.gameHome.attractions.redMahjong"), meta: t("commerce.gameHome.fourPlayer") }
]);

const parkAttractions = computed(() => {
  const games = miniGames.value?.games || [];
  return attractionConfig.value.map((item) => {
    const game = games.find((entry) => String(entry.code || "") === item.code);
    return {
      ...item,
      name: localizedGameField(game, "name") || item.name,
      meta: game?.players_text ? `${playersText(game.players_text)} · ${modeText(game)}` : item.meta
    };
  });
});

function coverOf(game?: MiniGameItem) {
  const raw = String(game?.cover || "").trim();
  if (!raw) {
    return "/static/art/game-park/orbital-park.webp";
  }
  if (raw.startsWith("/static")) {
    return localAssetUrl(raw);
  }
  return absolutizeUrl(raw);
}

function modeText(game?: MiniGameItem) {
  const mode = String(game?.play_mode || "");
  return ["realtime", "single", "local-keyboard", "local-turn-based", "webrtc"].includes(mode)
    ? t(`commerce.gameHome.modes.${mode}`)
    : t("commerce.gameHome.modes.casual");
}

function playersText(value: unknown) {
  const text = String(value || "");
  const matched = text.match(/^(\d+)\s*[-–]\s*(\d+)\s*人$/);
  return matched
    ? t("commerce.gameHome.playerRange")
      .replace("{min}", matched[1] || "")
      .replace("{max}", matched[2] || "")
    : text;
}

function selectAttraction(code: string) {
  selectedCode.value = code;
}

async function load() {
  loading.value = true;
  try {
    const [lotteryResult, miniGameResult] = await Promise.all([
      getLotteryHome(),
      getMiniGames()
    ]);
    home.value = lotteryResult;
    miniGames.value = miniGameResult;
    const firstCategory = lotteryResult?.categories?.[0];
    if (!selectedCategory.value && firstCategory) {
      selectedCategory.value = String(firstCategory.id || "");
    }
    if (!miniGameResult?.games?.some((game) => game.code === selectedCode.value)) {
      selectedCode.value = String(miniGameResult?.games?.[0]?.code || "deepsea_hunter");
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("commerce.gameHome.loadFailed"), icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

async function launchSelected() {
  const game = selectedGame.value;
  if (!game || launching.value) {
    return;
  }
  if (game.need_login === "1" && !requireLogin()) {
    return;
  }
  if (String(game.code || "") === "deepsea_hunter") {
    venuePickerVisible.value = true;
    return;
  }
  await launchGame(game);
}

async function launchFishingVenue(venue: FishingVenue) {
  const game = selectedGame.value;
  if (!game || launching.value) return;
  venuePickerVisible.value = false;
  await launchGame(game, String(venue.venue_code || "novice"));
}

async function launchGame(game: MiniGameItem, room = "") {
  launching.value = true;
  uni.showLoading({ title: t("commerce.gameHome.enterGame"), mask: true });
  try {
    const info = await enterMiniGame(String(game.code || ""), room);
    const url = String(info?.launch_url || "");
    if (!url) {
      throw new Error(t("commerce.gameHome.invalidGameUrl"));
    }
    openGameView(absolutizeUrl(url) || url);
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("commerce.gameHome.enterFailed"), icon: "none" });
  } finally {
    uni.hideLoading();
    launching.value = false;
  }
}

function openMiniGameCatalog() {
  uni.navigateTo({ url: "/pages/minigame/index" });
}

function openSports() {
  uni.switchTab({ url: "/pages/tabbar/sports/index" });
}

function scrollToLottery() {
  uni.pageScrollTo({ selector: "#lottery-panel", duration: 280 });
}

function consumeHomeZoneIntent() {
  const zone = consumeGameZoneIntent();
  if (!zone) {
    return;
  }
  if (zone === "fishing") {
    selectedCode.value = "deepsea_hunter";
  }
  void nextTick(() => {
    setTimeout(() => {
      uni.pageScrollTo({
        selector: zone === "lottery" ? "#lottery-panel" : "#game-park",
        duration: 320
      });
    }, 80);
  });
}

function gameIcon(game: LotteryGame) {
  const originalIcon = `/static/lotter/${String(game.game_code || "").toUpperCase()}.png`;
  return displayUrl(game.icon_url || game.icon || "", originalIcon);
}

function lotteryGameName(game: LotteryGame) {
  const source = game as unknown as Record<string, unknown>;
  const keys = locale.value === "zh-CN"
    ? ["game_name", "game_name_zh", "game_name_en"]
    : locale.value === "ja"
      ? ["game_name_ja", "game_name_jp", "game_name_en", "game_name"]
      : locale.value === "ko"
        ? ["game_name_ko", "game_name_kr", "game_name_en", "game_name"]
        : ["game_name_en", "game_name"];
  for (const key of keys) {
    const value = source[key];
    if (value !== undefined && value !== null && String(value).trim()) return String(value);
  }
  return t("commerce.common.game");
}

function openLotteryGame(game: LotteryGame) {
  if (!requireLogin()) {
    return;
  }
  const query = [
    `game_id=${encodeURIComponent(String(game.id || ""))}`,
    `game_code=${encodeURIComponent(String(game.game_code || ""))}`,
    `title=${encodeURIComponent(lotteryGameName(game))}`
  ].join("&");
  uni.navigateTo({ url: `/pages/game/bet?${query}` });
}

function openRecharge() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/wallet/recharge" });
  }
}

function openWithdraw() {
  if (requireLogin()) {
    uni.navigateTo({ url: "/pages/wallet/withdraw" });
  }
}

onShow(() => {
  consumeHomeZoneIntent();
  if (!loadedOnce) {
    loadedOnce = true;
    void load();
  }
});

onPullDownRefresh(() => {
  void load();
});
</script>

<style scoped>
.game-page {
  min-height: 100vh;
  overflow-x: hidden;
  padding: calc(22rpx + var(--status-bar-height)) 22rpx calc(142rpx + env(safe-area-inset-bottom));
  color: #fff;
  background: #061735;
}

.cosmic-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14rpx;
}

.brand-copy {
  min-width: 0;
  padding-top: 8rpx;
}

.brand-title-art {
  display: block;
  width: 370rpx;
  max-width: 100%;
  height: auto;
}

.brand-title-text {
  display: block;
  max-width: 370rpx;
  padding: 10rpx 4rpx;
  color: #ffe7a2;
  font-size: 42rpx;
  font-weight: 900;
  line-height: 1.05;
  text-shadow: 0 6rpx 18rpx rgba(0, 0, 0, 0.34);
}

.wallet-panel {
  width: 260rpx;
  flex: 0 0 260rpx;
  overflow: hidden;
  border: 2rpx solid rgba(255, 222, 147, 0.38);
  border-radius: 25rpx;
  background: #0b244d;
  box-shadow: 0 12rpx 30rpx rgba(0, 4, 17, 0.28);
}

.balance-row {
  display: flex;
  height: 76rpx;
  align-items: center;
  padding: 0 17rpx;
}

.coin-icon {
  width: 34rpx;
  height: 34rpx;
  flex: 0 0 34rpx;
  margin-right: 8rpx;
  border-radius: 50%;
}

.balance-value {
  min-width: 0;
  color: #ffe7a2;
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 41rpx;
  font-weight: 900;
  letter-spacing: -1rpx;
  line-height: 1;
  overflow: hidden;
  text-overflow: ellipsis;
}

.balance-unit {
  margin-left: 5rpx;
  color: rgba(255, 243, 208, 0.7);
  font-size: 17rpx;
}

.wallet-links {
  display: flex;
  height: 54rpx;
  align-items: center;
  justify-content: center;
  border-top: 2rpx solid rgba(255, 255, 255, 0.09);
}

.wallet-link {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: center;
  gap: 7rpx;
  color: #ffe7a2;
  font-size: 19rpx;
  font-weight: 800;
}

.wallet-link image {
  width: 25rpx;
  height: 25rpx;
}

.wallet-rule {
  width: 2rpx;
  height: 27rpx;
  background: rgba(255, 255, 255, 0.12);
}

.park-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 18rpx;
  margin-top: 29rpx;
  margin-bottom: 9px;
}

.park-title-art {
  display: block;
  width: 345rpx;
  max-width: 100%;
  height: auto;
}

.library-kicker,
.lottery-kicker {
  display: block;
  margin-top: 5rpx;
  color: rgba(221, 230, 250, 0.46);
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 15rpx;
  font-weight: 700;
  letter-spacing: 4rpx;
}

.zone-shortcuts {
  display: flex;
  flex: 0 0 auto;
  gap: 10rpx;
}

.zone-shortcut {
  display: flex;
  height: 58rpx;
  align-items: center;
  gap: 7rpx;
  padding: 0 13rpx 0 7rpx;
  border: 2rpx solid rgba(255, 221, 145, 0.28);
  border-radius: 20rpx;
  color: #fff4d2;
  font-size: 18rpx;
  font-weight: 800;
  background: #0b244d;
}

.zone-shortcut :deep(.safe-image) {
  width: 42rpx;
  height: 42rpx;
  border-radius: 13rpx;
}

.catalog-line {
  display: flex;
  height: 55rpx;
  align-items: center;
  margin: 18rpx 0 13rpx;
  padding: 0 17rpx;
  border: 2rpx solid rgba(255, 255, 255, 0.11);
  border-radius: 18rpx;
  background: #0a2046;
}

.catalog-count {
  color: #ffe29c;
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 23rpx;
  font-weight: 900;
}

.catalog-copy {
  flex: 1;
  margin-left: 7rpx;
  color: rgba(235, 241, 255, 0.66);
  font-size: 19rpx;
}

.catalog-action {
  color: #ff8069;
  font-size: 18rpx;
  font-weight: 800;
}

.park-map {
  position: relative;
  height: 930rpx;
  overflow: hidden;
  border: 2rpx solid rgba(255, 224, 158, 0.28);
  border-radius: 30rpx;
  background: #081d41;
  box-shadow: 0 24rpx 56rpx rgba(0, 4, 18, 0.42);
}

.park-art {
  width: 100%;
  height: 100%;
}

.map-marker {
  position: absolute;
  z-index: 2;
  display: flex;
  min-width: 184rpx;
  min-height: 68rpx;
  align-items: center;
  gap: 10rpx;
  padding: 8rpx 13rpx 8rpx 8rpx;
  border: 2rpx solid rgba(236, 242, 255, 0.48);
  border-radius: 23rpx;
  background: rgba(5, 27, 58, 0.9);
  box-shadow: 0 9rpx 20rpx rgba(0, 6, 24, 0.34);
  transition: transform 0.16s ease, border-color 0.16s ease;
}

.map-marker:active {
  transform: scale(0.96);
}

.map-marker.active {
  border-color: #ffd77c;
  box-shadow: 0 0 0 3rpx rgba(255, 215, 124, 0.12), 0 10rpx 26rpx rgba(0, 6, 24, 0.42);
}

.marker-order {
  display: flex;
  width: 43rpx;
  height: 43rpx;
  flex: 0 0 43rpx;
  align-items: center;
  justify-content: center;
  border: 2rpx solid rgba(255, 255, 255, 0.48);
  border-radius: 50%;
  color: #fff;
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 22rpx;
  font-weight: 900;
  line-height: 43rpx;
  text-align: center;
  background: #285faa;
}

.map-marker.active .marker-order {
  color: #193256;
  border-color: #fff1c3;
  background: #f2bd59;
}

.marker-name {
  display: block;
  max-width: 130rpx;
  color: #fff;
  font-size: 20rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.marker-meta {
  display: block;
  max-width: 135rpx;
  margin-top: 3rpx;
  color: rgba(224, 233, 249, 0.68);
  font-size: 14rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.marker-1 {
  top: 13%;
  left: 4%;
}

.marker-2 {
  top: 15%;
  right: 3%;
}

.marker-3 {
  top: 42%;
  right: 2%;
}

.marker-4 {
  top: 44%;
  left: 2%;
}

.marker-5 {
  bottom: 13%;
  left: 3%;
}

.marker-6 {
  right: 2%;
  bottom: 14%;
}

.map-enter {
  position: absolute;
  top: 30%;
  left: 13%;
  z-index: 3;
  display: flex;
  height: 58rpx;
  align-items: center;
  justify-content: center;
  padding: 0 25rpx;
  border: 2rpx solid #ffb07f;
  border-radius: 29rpx;
  color: #fff;
  font-size: 19rpx;
  font-weight: 900;
  background: #f06450;
  box-shadow: 0 9rpx 22rpx rgba(240, 100, 80, 0.3);
}

.selected-dock {
  position: relative;
  z-index: 4;
  display: flex;
  min-height: 236rpx;
  gap: 18rpx;
  margin: -30rpx 8rpx 0;
  padding: 18rpx;
  border: 2rpx solid #e7bb6c;
  border-radius: 30rpx;
  background: #0a2853;
  box-shadow: 0 18rpx 42rpx rgba(0, 4, 18, 0.42);
}

.selected-cover {
  position: relative;
  width: 202rpx;
  height: 202rpx;
  flex: 0 0 202rpx;
  overflow: hidden;
  border-radius: 22rpx;
}

.selected-cover-img {
  width: 100%;
  height: 100%;
}

.selected-flag {
  position: absolute;
  top: 10rpx;
  left: 10rpx;
  padding: 5rpx 13rpx;
  border-radius: 14rpx;
  color: #fff;
  font-size: 16rpx;
  font-weight: 900;
  letter-spacing: 1rpx;
  background: #f46957;
}

.selected-main {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
}

.selected-title-row {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 9rpx;
}

.selected-name {
  min-width: 0;
  color: #fff;
  font-size: 31rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.selected-players {
  flex: 0 0 auto;
  padding: 5rpx 10rpx;
  border-radius: 10rpx;
  color: rgba(235, 242, 255, 0.72);
  font-size: 16rpx;
  background: rgba(255, 255, 255, 0.08);
}

.selected-remark {
  display: -webkit-box;
  margin-top: 13rpx;
  color: rgba(224, 233, 249, 0.65);
  font-size: 18rpx;
  line-height: 1.55;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.selected-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10rpx;
  margin-top: auto;
}

.selected-mode {
  color: rgba(224, 233, 249, 0.7);
  font-size: 17rpx;
  font-weight: 700;
}

.launch-button {
  display: flex;
  height: 64rpx;
  align-items: center;
  justify-content: center;
  padding: 0 24rpx;
  border: 2rpx solid #ffad8e;
  border-radius: 32rpx;
  color: #fff;
  font-size: 20rpx;
  font-weight: 900;
  background: #f06450;
  box-shadow: 0 9rpx 22rpx rgba(240, 100, 80, 0.25);
}

.launch-button.disabled {
  opacity: 0.55;
}

.library-entry {
  display: flex;
  min-height: 118rpx;
  align-items: center;
  justify-content: space-between;
  gap: 20rpx;
  margin-top: 20rpx;
  padding: 18rpx 22rpx;
  border: 2rpx solid rgba(255, 255, 255, 0.11);
  border-radius: 25rpx;
  background: #0a2046;
}

.library-title {
  display: block;
  margin-top: 7rpx;
  color: #fff;
  font-size: 23rpx;
  font-weight: 900;
}

.library-button {
  display: flex;
  height: 54rpx;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  padding: 0 19rpx;
  border: 2rpx solid rgba(255, 219, 138, 0.42);
  border-radius: 18rpx;
  color: #ffe29c;
  font-size: 18rpx;
  font-weight: 900;
  background: #102e5a;
}

.lottery-panel {
  margin-top: 20rpx;
  padding: 22rpx;
  border: 2rpx solid rgba(255, 255, 255, 0.11);
  border-radius: 28rpx;
  background: #0a2046;
}

.lottery-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.lottery-title {
  display: block;
  color: #fff;
  font-size: 28rpx;
  font-weight: 900;
}

.lottery-subtitle {
  display: block;
  margin-top: 7rpx;
  color: rgba(224, 233, 249, 0.58);
  font-size: 18rpx;
  line-height: 1.4;
}

.lottery-categories {
  width: 100%;
  margin-top: 20rpx;
  white-space: nowrap;
}

.lottery-category {
  display: inline-flex;
  height: 54rpx;
  align-items: center;
  margin-right: 11rpx;
  padding: 0 19rpx;
  border: 2rpx solid rgba(255, 255, 255, 0.11);
  border-radius: 17rpx;
  color: rgba(230, 237, 251, 0.62);
  font-size: 18rpx;
  font-weight: 800;
  background: #0d2a56;
}

.lottery-category.active {
  color: #193256;
  border-color: #ffe5a4;
  background: #f0c86f;
}

.lottery-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12rpx;
  margin-top: 18rpx;
}

.lottery-game {
  min-width: 0;
  padding: 12rpx 8rpx;
  border: 2rpx solid rgba(255, 255, 255, 0.08);
  border-radius: 18rpx;
  text-align: center;
  background: #0d2a56;
}

.lottery-icon {
  width: 70rpx;
  height: 70rpx;
  margin: 0 auto;
  border-radius: 17rpx;
}

.lottery-name {
  display: block;
  width: 100%;
  margin-top: 9rpx;
  color: #fff;
  font-size: 17rpx;
  font-weight: 800;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lottery-code {
  display: block;
  margin-top: 4rpx;
  color: rgba(224, 233, 249, 0.42);
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 13rpx;
}

.game-page :deep(.empty-state) {
  color: rgba(235, 242, 255, 0.62);
}

@media (max-width: 360px) {
  .brand-title-art,
  .brand-title-text {
    width: 340rpx;
  }

  .wallet-panel {
    width: 230rpx;
    flex-basis: 230rpx;
  }

  .zone-shortcut {
    padding-right: 9rpx;
  }

  .selected-cover {
    width: 184rpx;
    height: 184rpx;
    flex-basis: 184rpx;
  }
}
</style>
