<template>
  <view
    class="lottery-page"
    :class="[`phase-${stagePhase}`, { 'bet-flash': betFx, 'dock-shake': dockShake }]"
  >
    <view class="velvet-noise" />
    <view class="ambient ambient-a" />
    <view class="ambient ambient-b" />

    <view class="top-nav">
      <view class="round-icon back-icon" :aria-label="t('commerce.gameBet.back')" @tap="goBack">
        <view class="arrow-left" />
      </view>

      <view class="game-identity">
        <view class="game-emblem">
          <image v-if="gameIcon && !gameIconFailed" :src="gameIcon" mode="aspectFit" @error="gameIconFailed = true" />
          <view v-else class="emblem-gem" />
        </view>
        <view class="game-title-wrap">
          <text class="game-title">{{ gameTitle }}</text>
          <view class="live-line">
            <view class="live-dot" />
            <text>{{ stagePhaseText }}</text>
          </view>
        </view>
      </view>

      <view class="nav-actions">
        <view class="balance-pill">
          <view class="coin-mini" />
          <text>{{ compactMoney(coin) }}</text>
        </view>
        <button class="round-icon history-icon" :aria-label="t('commerce.gameBet.drawHistory')" @tap="showHistory = true">
          <view class="history-bars"><i /><i /><i /></view>
        </button>
      </view>
    </view>

    <view v-if="loading" class="loading-scene">
      <view class="loading-orbit"><i /><i /><i /></view>
      <text>{{ t("commerce.gameBet.preparingTable") }}</text>
    </view>

    <view v-else-if="loadError" class="error-scene">
      <view class="broken-ball">!</view>
      <text>{{ loadError }}</text>
      <button @tap="loadDetail(true)">{{ t("commerce.gameBet.reenterTable") }}</button>
    </view>

    <template v-else>
      <view class="content-wrap">
        <view class="draw-stage">
          <canvas id="lotteryCanvas" canvas-id="lotteryCanvas" class="lottery-canvas" :disable-scroll="true"/>
          <view class="stage-grid" />
          <view class="stage-sheen" />
          <view class="issue-badge">
            <view class="signal-waves"><i /><i /><i /></view>
            <text>{{ shortIssue(currentIssue.issue_num) }}</text>
          </view>
          <button class="rule-button" :aria-label="t('commerce.gameBet.rules')" @tap="showRules = true">?</button>
          <view class="countdown-dial" :class="{ urgent: sealCountdown <= 10 && !isSealed }">
            <view class="dial-ticks" />
            <view class="dial-core">
              <text class="countdown-value">{{ countdownText }}</text>
              <text class="countdown-label">{{ isSealed ? t("commerce.gameBet.sealed") : t("commerce.gameBet.seal") }}</text>
            </view>
          </view>

          <view class="result-bay">
            <view v-if="drawing" class="drawing-capsules">
              <view v-for="n in resultSlots" :key="n" class="shuffle-ball" :style="shuffleStyle(n)">
                <i />
              </view>
            </view>
            <view v-else-if="latestNumbers.length" class="result-track">
              <view
                v-for="(value, index) in visibleLatestNumbers"
                :key="`${revealSerial}-${index}-${value}`"
                class="result-piece"
                :class="resultPieceClass(value, index)"
                :style="{ animationDelay: `${index * 80}ms` }"
              >
                <view v-if="gameMode === 'dice'" class="result-dice">
                  <i
                    v-for="dot in 9"
                    :key="dot"
                    :class="{ on: hasDicePip(numberFrom(value), dot) }"
                  />
                </view>
                <template v-else-if="gameMode === 'card'">
                  <text class="card-rank">{{ cardRank(numberFrom(value)) }}</text>
                  <text class="card-suit">{{ cardSuit(index) }}</text>
                </template>
                <text v-else>{{ formatBall(value) }}</text>
              </view>
            </view>
            <view v-else class="waiting-result">
              <view v-for="n in resultSlots" :key="n" class="empty-slot" />
            </view>
          </view>

          <view class="stage-caption">
            <view class="caption-status"><i />{{ latestIssueLabel }}</view>
            <view class="open-clock">{{ openCountdownText }}</view>
          </view>

          <view class="prop-card prop-card-left">
            <text>A</text><i>♠</i>
          </view>
          <view class="prop-card prop-card-right">
            <text>K</text><i>♥</i>
          </view>
          <view class="prop-tile"><text>{{ t("commerce.gameBet.fortuneTile") }}</text></view>
          <view class="prop-zodiac"><text>{{ t("commerce.gameBet.dragonTile") }}</text></view>
        </view>

        <view v-if="historyPreview.length" class="recent-ribbon" @tap="showHistory = true">
          <view class="ribbon-handle"><i /><i /><i /></view>
          <view v-for="row in historyPreview" :key="String(row.id || row.issue_num)" class="recent-round">
            <text class="recent-issue">{{ shortIssue(row.issue_num) }}</text>
            <view class="recent-balls">
              <view
                v-for="(value, index) in splitCode(row.open_code).slice(0, 5)"
                :key="`${row.issue_num}-${index}`"
                :class="['recent-ball', `tone-${ballTone(value)}`]"
              >{{ compactBall(value) }}</view>
            </view>
          </view>
          <view class="ribbon-arrow" />
        </view>

        <view class="bet-table">
          <view class="table-rim" />
          <view class="table-corner corner-a" />
          <view class="table-corner corner-b" />

          <scroll-view scroll-x class="play-tabs" :show-scrollbar="false">
            <view class="play-tabs-inner">
              <view v-for="play in plays" :key="String(play.id)" class="play-tab" :class="{ active: String(play.id) === activePlayId }" @tap="selectPlay(play)">
                <view class="play-glyph">{{ playGlyph(play) }}</view>
                <text>{{ playName(play) }}</text>
              </view>
            </view>
          </scroll-view>

          <view class="table-heading">
            <view>
              <text class="table-kicker">{{ activePlayCode }}</text>
              <text class="table-title">{{ activePlayName }}</text>
            </view>
            <view class="selection-counter" :class="{ active: selectedOptions.length }">
              <view class="mini-chip-stack"><i /><i /><i /></view>
              <text>{{ selectedOptions.length }}</text>
            </view>
          </view>

          <view v-if="activeOptions.length" class="option-grid" :class="optionGridClass">
            <button
              v-for="option in activeOptions"
              :key="String(option.id)"
              class="bet-option"
              :class="[
                `visual-${optionKind(option, activePlay)}`,
                `tone-${optionTone(option)}`,
                { selected: isOptionSelected(option), burst: burstOptionId === String(option.id) }
              ]"
              :disabled="isSealed || submitting"
              @tap="toggleOption(option, activePlay)"
            >
              <view class="option-halo" />
              <view class="option-visual">
                <view v-if="optionKind(option, activePlay) === 'dice'" class="dice-face">
                  <i
                    v-for="dot in 9"
                    :key="dot"
                    :class="{ on: hasDicePip(optionNumber(option), dot) }"
                  />
                </view>
                <view v-else-if="optionKind(option, activePlay) === 'card'" class="card-face">
                  <text>{{ cardRank(optionNumber(option)) }}</text>
                  <i>{{ cardSuit(optionNumber(option)) }}</i>
                </view>
                <view v-else-if="optionKind(option, activePlay) === 'mahjong'" class="mahjong-face">
                  <text>{{ optionVisualLabel(option) }}</text>
                  <i>{{ mahjongMark(option) }}</i>
                </view>
                <view v-else class="token-face">
                  <text>{{ optionVisualLabel(option) }}</text>
                  <i v-if="optionKind(option, activePlay) === 'zodiac'">{{ zodiacMark(option) }}</i>
                </view>
              </view>

              <view class="option-meta">
                <text class="option-name">{{ optionName(option) }}</text>
                <text class="option-odds">×{{ option.odds || "--" }}</text>
              </view>
            </button>
          </view>

          <view v-else class="no-options">
            <view class="empty-fan"><i /><i /><i /></view>
            <text>{{ t("commerce.gameBet.tableNotOpen") }}</text>
          </view>
        </view>
      </view>

      <view class="chip-dock">
        <view class="dock-topline" />
        <view class="quick-chip-row">
          <button
            v-for="(chip, index) in quickAmounts"
            :key="chip"
            class="quick-chip"
            :class="[`chip-${index}`, { active: amount === chip }]"
            @tap="setAmount(chip)"
          >
            <i /><text>{{ compactMoney(chip) }}</text><i />
          </button>
        </view>

        <view class="dock-main">
          <view class="amount-stepper">
            <button :aria-label="t('commerce.gameBet.decreaseChips')" @tap="changeAmount(-1)">−</button>
            <view class="amount-display">
              <text class="currency-mark">◇</text>
              <input v-model.number="amount" type="number" :min="minBet" :max="maxBet" @blur="normalizeAmount"/>
            </view>
            <button :aria-label="t('commerce.gameBet.increaseChips')" @tap="changeAmount(1)">＋</button>
          </view>
          <view class="place-bet" :class="{ ready: canSubmit, locking: submitting }" :disabled="!canSubmit" @tap="placeBet">
            <view class="bet-copy">
              <text class="bet-label">{{ submitting ? t("commerce.gameBet.locking") : isSealed ? t("commerce.gameBet.sealed") : t("commerce.gameBet.placeChips") }}</text>
              <text class="bet-total">{{ selectedOptions.length ? compactMoney(totalBet) : t("commerce.gameBet.chooseCards") }}</text>
            </view>
          </view>
        </view>
      </view>
    </template>

    <view v-if="betFx" class="chip-flight-layer" pointer-events="none">
      <i v-for="n in 14" :key="n" :style="flightChipStyle(n)" />
    </view>

    <view v-if="successTicket" class="success-layer" @tap="successTicket = false">
      <view class="success-ticket">
        <view class="success-halo"><i /></view>
        <view class="success-check"><i /></view>
        <text class="success-title">{{ t("commerce.gameBet.chipsAccepted") }}</text>
        <text class="success-amount">◇ {{ compactMoney(lastBetTotal) }}</text>
        <view class="ticket-cut"><i v-for="n in 9" :key="n" /></view>
      </view>
    </view>

    <view v-if="showHistory" class="sheet-mask" @tap="showHistory = false">
      <view class="history-sheet" @tap.stop>
        <view class="sheet-handle" />
        <view class="sheet-title-row">
          <view>
            <text class="sheet-kicker">{{ t("commerce.gameBet.liveArchive") }}</text>
            <text class="sheet-title">{{ t("commerce.gameBet.drawTrack") }}</text>
          </view>
          <button @tap="showHistory = false"><i /></button>
        </view>
        <scroll-view scroll-y class="history-list">
          <view
            v-for="(row, rowIndex) in history"
            :key="String(row.id || row.issue_num)"
            class="history-row"
          >
            <view class="history-index">{{ String(rowIndex + 1).padStart(2, "0") }}</view>
            <view class="history-info">
              <text>{{ shortIssue(row.issue_num) }}</text>
              <text>{{ drawTime(row) }}</text>
            </view>
            <view class="history-results">
              <view
                v-for="(value, index) in splitCode(row.open_code).slice(0, 10)"
                :key="`${row.issue_num}-${index}`"
                :class="['history-ball', `tone-${ballTone(value)}`]"
              >{{ compactBall(value) }}</view>
            </view>
          </view>
        </scroll-view>
      </view>
    </view>

    <view v-if="showRules" class="sheet-mask" @tap="showRules = false">
      <view class="rule-sheet" @tap.stop>
        <view class="sheet-handle" />
        <view class="rule-seal"><i>{{ playGlyph(activePlay) }}</i></view>
        <text class="rule-title">{{ activePlayName }}</text>
        <text class="rule-copy">{{ ruleDescription }}</text>
        <button class="rule-close" @tap="showRules = false">{{ t("commerce.gameBet.gotIt") }}</button>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, getCurrentInstance, nextTick, ref } from "vue";
import { onHide, onLoad, onReady, onShow, onUnload } from "@dcloudio/uni-app";
import {
  getLotteryCurrentIssue,
  getLotteryGameDetail,
  getLotteryIssueHistory,
  submitLotteryBet
} from "@/api/services";
import { displayUrl } from "@/utils/url";
import { requireLogin } from "@/utils/session";
import { useI18n } from "@/i18n";

const { locale, t } = useI18n();

type AnyRecord = Record<string, any>;

const instance = getCurrentInstance();
const loading = ref(true);
const loadError = ref("");
const detail = ref<AnyRecord>({});
const currentIssue = ref<AnyRecord>({});
const history = ref<AnyRecord[]>([]);
const activePlayId = ref("");
const selectedOptionIds = ref<string[]>([]);
const amount = ref(10);
const nowSeconds = ref(Math.floor(Date.now() / 1000));
const serverOffset = ref(0);
const drawing = ref(false);
const revealSerial = ref(0);
const showHistory = ref(false);
const showRules = ref(false);
const loadingHistory = ref(false);
const submitting = ref(false);
const successTicket = ref(false);
const lastBetTotal = ref(0);
const betFx = ref(false);
const dockShake = ref(false);
const burstOptionId = ref("");
const gameIconFailed = ref(false);

let gameId = "";
let gameCode = "";
let routeTitle = "";
let visible = false;
let syncing = false;
let syncCycle = 0;
let latestHistoryKey = "";
let waitingIssueNum = "";
let clockTimer: ReturnType<typeof setInterval> | undefined;
let syncTimer: ReturnType<typeof setTimeout> | undefined;
let canvasTimer: ReturnType<typeof setInterval> | undefined;
let successTimer: ReturnType<typeof setTimeout> | undefined;
let betFxTimer: ReturnType<typeof setTimeout> | undefined;
let burstTimer: ReturnType<typeof setTimeout> | undefined;
let shakeTimer: ReturnType<typeof setTimeout> | undefined;
let revealTimer: ReturnType<typeof setTimeout> | undefined;
let canvasContext: any;
let canvasWidth = 360;
let canvasHeight = 220;
let canvasFrame = 0;
let canvasBurstUntil = 0;

const game = computed<AnyRecord>(() => recordOf(detail.value.game));
const plays = computed<AnyRecord[]>(() => arrayOf(detail.value.plays));
const activePlay = computed<AnyRecord>(() => {
  return plays.value.find((play) => String(play.id) === activePlayId.value) || plays.value[0] || {};
});
const activeOptions = computed<AnyRecord[]>(() => arrayOf(activePlay.value.options));
const gameTitle = computed(() => localizedRecordText(game.value, "game_name") || routeTitle || t("commerce.gameBet.defaultTitle"));
const gameIcon = computed(() => {
  const code = textOf(game.value.game_code, gameCode).toUpperCase();
  return displayUrl(
    textOf(game.value.icon_url, game.value.icon),
    `/static/lotter/${code}.png`
  );
});
const coin = computed(() => numeric(detail.value.coin, 0));
const minBet = computed(() => Math.max(1, numeric(game.value.min_bet, 10)));
const maxBet = computed(() => Math.max(minBet.value, numeric(game.value.max_bet, 100000)));
const quickAmounts = computed(() => {
  const base = minBet.value;
  return [...new Set([base, base * 5, base * 10, base * 50])];
});
const selectedOptions = computed<AnyRecord[]>(() => {
  const selected = new Set(selectedOptionIds.value);
  return plays.value.flatMap((play) =>
    arrayOf(play.options)
      .filter((option) => selected.has(String(option.id)))
      .map((option) => ({ ...option, play }) as AnyRecord)
  );
});
const totalBet = computed(() => Math.max(0, numeric(amount.value, 0)) * selectedOptions.value.length);
const clockNow = computed(() => nowSeconds.value + serverOffset.value);
const sealCountdown = computed(() => {
  const sealTime = numeric(currentIssue.value.seal_time, 0);
  return sealTime ? Math.max(0, Math.ceil(sealTime - clockNow.value)) : 0;
});
const openCountdown = computed(() => {
  const openTime = numeric(currentIssue.value.open_time, 0);
  return openTime ? Math.max(0, Math.ceil(openTime - clockNow.value)) : 0;
});
const isSealed = computed(() => {
  if (!currentIssue.value.id || sealCountdown.value <= 0) return true;
  const canBet = String(currentIssue.value.can_bet ?? "").trim().toLowerCase();
  if (canBet) return !["1", "true"].includes(canBet);
  return String(currentIssue.value.status ?? "") !== "1";
});
const stagePhase = computed(() => (drawing.value ? "drawing" : isSealed.value ? "sealed" : "betting"));
const stagePhaseText = computed(() => {
  if (drawing.value) return t("commerce.gameBet.drawing");
  if (isSealed.value) return t("commerce.gameBet.seal");
  return t("commerce.common.liveSync");
});
const countdownText = computed(() => formatDuration(sealCountdown.value));
const openCountdownText = computed(() => {
  if (drawing.value) return t("commerce.gameBet.liveDraw");
  if (openCountdown.value <= 0) return t("commerce.gameBet.ready");
  return `${t("commerce.gameBet.drawIn")} ${formatDuration(openCountdown.value)}`;
});
const latestRow = computed<AnyRecord>(() => history.value[0] || {});
const latestNumbers = computed(() => splitCode(latestRow.value.open_code));
const visibleLatestNumbers = computed(() => latestNumbers.value.slice(0, gameMode.value === "card" ? 5 : 10));
const latestIssueLabel = computed(() => {
  const issue = latestRow.value.issue_num;
  return issue
    ? t("commerce.gameBet.verifiedIssue").replace("{issue}", shortIssue(issue))
    : t("commerce.gameBet.awaitingFirstDraw");
});
const historyPreview = computed(() => history.value.slice(0, 2));
const resultSlots = computed(() => {
  const currentCount = visibleLatestNumbers.value.length;
  if (currentCount) return Math.min(currentCount, 10);
  return gameMode.value === "dice" ? 3 : gameMode.value === "card" ? 5 : 6;
});
const activePlayName = computed(() => playName(activePlay.value));
const activePlayCode = computed(() => textOf(activePlay.value.play_code, t("commerce.gameBet.selectTable")));
const gameMode = computed<"number" | "dice" | "card" | "zodiac" | "mahjong">(() => {
  const source = [game.value.game_code, game.value.game_name, game.value.game_name_en]
    .join(" ")
    .toUpperCase();
  if (/K3|KS|DICE|骰|快三/.test(source)) return "dice";
  if (/CARD|POKER|扑克|纸牌/.test(source)) return "card";
  if (/MAHJONG|麻将/.test(source)) return "mahjong";
  if (/LHC|六合彩|生肖/.test(source)) return "zodiac";
  return "number";
});
const optionGridClass = computed(() => {
  const count = activeOptions.value.length;
  if (count <= 4) return "grid-roomy";
  if (count >= 10) return "grid-dense";
  return "grid-standard";
});
const canSubmit = computed(() => {
  return !submitting.value && !isSealed.value && selectedOptions.value.length > 0 && totalBet.value > 0;
});
const ruleDescription = computed(() => {
  return textOf(
    localizedRecordText(activePlay.value, "rule_desc"),
    localizedRecordText(game.value, "rule_desc"),
    localizedRecordText(game.value, "description"),
    t("commerce.gameBet.defaultRule")
  );
});

function recordOf(value: unknown): AnyRecord {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as AnyRecord) : {};
}

function arrayOf(value: unknown): AnyRecord[] {
  return Array.isArray(value) ? (value as AnyRecord[]) : [];
}

function textOf(...values: unknown[]) {
  for (const value of values) {
    const text = String(value ?? "").trim();
    if (text) return text;
  }
  return "";
}

function localizedRecordText(source: AnyRecord, base: string) {
  const suffixes = locale.value === "zh-CN"
    ? ["", "zh", "cn"]
    : locale.value === "ja"
      ? ["ja", "jp", "en", ""]
      : locale.value === "ko"
        ? ["ko", "kr", "en", ""]
        : ["en", ""];
  for (const suffix of suffixes) {
    const value = source[suffix ? `${base}_${suffix}` : base];
    if (value !== undefined && value !== null && String(value).trim()) return String(value);
  }
  return "";
}

function numeric(value: unknown, fallback = 0) {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
}

function splitCode(value: unknown) {
  return String(value || "")
    .split(/,|，|\s+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function numberFrom(value: unknown) {
  const match = String(value ?? "").match(/-?\d+/);
  return match ? Number(match[0]) : 0;
}

function optionNumber(option: AnyRecord) {
  return numberFrom(textOf(option.option_code, option.option_name));
}

function formatDuration(seconds: number) {
  const safe = Math.max(0, Math.floor(seconds));
  const minutes = Math.floor(safe / 60);
  const remain = safe % 60;
  return `${String(minutes).padStart(2, "0")}:${String(remain).padStart(2, "0")}`;
}

function compactMoney(value: unknown) {
  const number = numeric(value, 0);
  if (Math.abs(number) >= 1000000) return `${trimZero(number / 1000000)}m`;
  if (Math.abs(number) >= 10000) return `${trimZero(number / 10000)}w`;
  return trimZero(number);
}

function trimZero(value: number) {
  return value.toFixed(value % 1 === 0 ? 0 : 1);
}

function shortIssue(value: unknown) {
  const text = String(value || "--");
  if (text.length <= 9) return `NO.${text}`;
  return `NO.${text.slice(-8)}`;
}

function compactBall(value: unknown) {
  const text = String(value || "-");
  return text.length > 2 ? text.slice(-2) : text.padStart(2, "0");
}

function formatBall(value: unknown) {
  const text = String(value || "-");
  const number = numberFrom(text);
  if (/^\d+$/.test(text) && number >= 0 && number < 100) return String(number).padStart(2, "0");
  return text.slice(0, 2);
}

function ballTone(value: unknown) {
  const number = Math.abs(numberFrom(value));
  if (!number) return 0;
  return (number % 5) + 1;
}

function resultPieceClass(value: unknown, index: number) {
  return [`mode-${gameMode.value}`, `tone-${ballTone(value)}`, { red: cardSuit(index) === "♥" || cardSuit(index) === "♦" }];
}

function cardRank(value: number) {
  if (value === 1) return "A";
  if (value === 11) return "J";
  if (value === 12) return "Q";
  if (value === 13) return "K";
  return String(value || "?");
}

function cardSuit(value: number) {
  return ["♠", "♥", "♣", "♦"][Math.abs(value) % 4];
}

function playName(play: AnyRecord) {
  return textOf(localizedRecordText(play, "play_name"), play.play_code, t("commerce.gameBet.pickNumbers"));
}

function optionName(option: AnyRecord) {
  return textOf(localizedRecordText(option, "option_name"), option.option_code, t("commerce.common.option"));
}

function playGlyph(play: AnyRecord) {
  const source = `${textOf(play.play_code)} ${textOf(play.result_rule)} ${playName(play)}`.toUpperCase();
  if (/ZODIAC|生肖/.test(source)) return t("commerce.gameBet.glyph.zodiac");
  if (/COLOR|波色/.test(source)) return t("commerce.gameBet.glyph.color");
  if (/DRAGON|TIGER|龙虎/.test(source)) return t("commerce.gameBet.glyph.dragon");
  if (/ODD|EVEN|单双/.test(source)) return t("commerce.gameBet.glyph.oddEven");
  if (/BIG|SMALL|大小/.test(source)) return t("commerce.gameBet.glyph.bigSmall");
  if (/SUM|和值/.test(source)) return "Σ";
  if (/CARD|POKER|牌/.test(source)) return "A";
  if (/DICE|K3|骰/.test(source)) return t("commerce.gameBet.glyph.dice");
  return playName(play).slice(0, 1) || t("commerce.gameBet.glyph.select");
}

function selectPlay(play: AnyRecord) {
  activePlayId.value = String(play.id || "");
  try {
    uni.vibrateShort({ type: "light" } as any);
  } catch {
    // Vibration is optional across targets.
  }
}

function optionKind(option: AnyRecord, play: AnyRecord) {
  const source = `${textOf(play.result_rule)} ${textOf(play.play_code)} ${optionName(option)} ${textOf(option.option_code)}`.toUpperCase();
  const number = optionNumber(option);
  if (/ZODIAC|生肖|鼠|牛|虎|兔|龙|蛇|马|羊|猴|鸡|狗|猪/.test(source)) return "zodiac";
  if (/MAHJONG|麻将|万|筒|条|東|南|西|北|發|白板/.test(source)) return "mahjong";
  if (gameMode.value === "dice" && number >= 1 && number <= 6) return "dice";
  if (gameMode.value === "card" && number >= 0 && number <= 13) return "card";
  if (/COLOR|RED|BLUE|GREEN|红波|蓝波|绿波/.test(source)) return "color";
  if (number || /^0$/.test(textOf(option.option_code))) return "number";
  return "emblem";
}

function optionVisualLabel(option: AnyRecord) {
  const name = optionName(option);
  const number = optionNumber(option);
  if (optionKind(option, activePlay.value) === "number" && /^\d+$/.test(textOf(option.option_code, name))) {
    return String(number).padStart(gameMode.value === "zodiac" ? 2 : 1, "0");
  }
  const zodiac = name.match(/[鼠牛虎兔龙蛇马羊猴鸡狗猪]/)?.[0];
  if (zodiac) return zodiac;
  return name.slice(0, 2);
}

function zodiacMark(option: AnyRecord) {
  const zodiac = optionName(option).match(/[鼠牛虎兔龙蛇马羊猴鸡狗猪]/)?.[0] || optionVisualLabel(option);
  const index = "鼠牛虎兔龙蛇马羊猴鸡狗猪".indexOf(zodiac);
  return index >= 0 ? String(index + 1).padStart(2, "0") : t("commerce.gameBet.glyph.zodiac");
}

function mahjongMark(option: AnyRecord) {
  const number = optionNumber(option);
  return number ? ["●", t("commerce.gameBet.bamboo"), t("commerce.gameBet.characters")][number % 3] : t("commerce.gameBet.tile");
}

function optionTone(option: AnyRecord) {
  const source = `${textOf(option.option_code)} ${optionName(option)}`.toUpperCase();
  if (/RED|红/.test(source)) return 2;
  if (/BLUE|蓝/.test(source)) return 4;
  if (/GREEN|绿/.test(source)) return 3;
  return ballTone(optionNumber(option) || option.id);
}

function hasDicePip(value: number, dot: number) {
  const map: Record<number, number[]> = {
    1: [5],
    2: [1, 9],
    3: [1, 5, 9],
    4: [1, 3, 7, 9],
    5: [1, 3, 5, 7, 9],
    6: [1, 3, 4, 6, 7, 9]
  };
  return (map[Math.max(1, Math.min(6, value))] || []).includes(dot);
}

function isOptionSelected(option: AnyRecord) {
  return selectedOptionIds.value.includes(String(option.id));
}

function toggleOption(option: AnyRecord, play: AnyRecord) {
  if (isSealed.value || submitting.value) return;
  const id = String(option.id || "");
  if (!id) return;
  if (selectedOptionIds.value.includes(id)) {
    selectedOptionIds.value = selectedOptionIds.value.filter((item) => item !== id);
  } else {
    if (selectedOptionIds.value.length >= 8) {
      uni.showToast({ title: t("commerce.gameBet.maxSelections").replace("{count}", "8"), icon: "none" });
      return;
    }
    selectedOptionIds.value = [...selectedOptionIds.value, id];
    burstOptionId.value = id;
    if (burstTimer) clearTimeout(burstTimer);
    burstTimer = setTimeout(() => (burstOptionId.value = ""), 520);
  }
  activePlayId.value = String(play.id || activePlayId.value);
  try {
    uni.vibrateShort({ type: "light" } as any);
  } catch {
    // Optional feedback.
  }
}

function setAmount(value: number) {
  amount.value = Math.min(maxBet.value, Math.max(minBet.value, value));
}

function changeAmount(direction: number) {
  setAmount(numeric(amount.value, minBet.value) + minBet.value * direction);
}

function normalizeAmount() {
  const value = Math.round(numeric(amount.value, minBet.value));
  setAmount(value);
}

async function placeBet() {
  if (!canSubmit.value || !requireLogin()) return;
  normalizeAmount();
  if (totalBet.value < minBet.value) {
    uni.showToast({ title: `${t("commerce.gameBet.minimum")} ${minBet.value}`, icon: "none" });
    return;
  }
  if (totalBet.value > maxBet.value) {
    uni.showToast({ title: `${t("commerce.gameBet.maximumPerBet")} ${maxBet.value}`, icon: "none" });
    triggerDockShake();
    return;
  }

  const issueId = String(currentIssue.value.id || "");
  if (!issueId) return;
  submitting.value = true;
  lastBetTotal.value = totalBet.value;
  triggerBetFx();
  const itemSnapshot = selectedOptions.value.map((option) => ({
    optionId: String(option.id),
    amount: amount.value
  }));

  try {
    await submitLotteryBet({
      gameId,
      gameCode,
      issueId,
      items: itemSnapshot,
      clientTraceId: `UNI_TABLE_${Date.now()}_${itemSnapshot.length}`
    });
    selectedOptionIds.value = [];
    successTicket.value = true;
    if (successTimer) clearTimeout(successTimer);
    successTimer = setTimeout(() => (successTicket.value = false), 1800);
    try {
      uni.vibrateShort({ type: "heavy" } as any);
    } catch {
      // Optional feedback.
    }
    await loadDetail(false);
  } catch (error: any) {
    triggerDockShake();
    uni.showToast({ title: error?.message || t("commerce.gameBet.betFailed"), icon: "none" });
  } finally {
    submitting.value = false;
  }
}

function triggerBetFx() {
  betFx.value = true;
  canvasBurstUntil = Date.now() + 1050;
  if (betFxTimer) clearTimeout(betFxTimer);
  betFxTimer = setTimeout(() => (betFx.value = false), 960);
}

function triggerDockShake() {
  dockShake.value = true;
  if (shakeTimer) clearTimeout(shakeTimer);
  shakeTimer = setTimeout(() => (dockShake.value = false), 460);
}

function flightChipStyle(index: number) {
  const angle = -64 + index * 9.4;
  const distance = 250 + (index % 4) * 26;
  const delay = (index % 5) * 34;
  return {
    "--flight-angle": `${angle}deg`,
    "--flight-distance": `${distance}rpx`,
    "--flight-delay": `${delay}ms`,
    "--flight-color": ["#ffd667", "#ff5c8a", "#7f6cff", "#31ddc1"][index % 4]
  } as Record<string, string>;
}

function shuffleStyle(index: number) {
  return {
    animationDelay: `${(index % 5) * -110}ms`,
    "--shuffle-x": `${(index - resultSlots.value / 2) * 42}rpx`
  } as Record<string, string>;
}

function drawTime(row: AnyRecord) {
  const text = textOf(row.open_time_text);
  if (text) return text.slice(5, 16);
  const timestamp = numeric(row.open_time, 0);
  if (!timestamp) return t("commerce.common.liveSync");
  const date = new Date(timestamp * 1000);
  return `${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")} ${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

function syncServerClock(issue: AnyRecord) {
  const sealTime = numeric(issue.seal_time, 0);
  const remaining = numeric(issue.seal_countdown, -1);
  if (sealTime && remaining >= 0) {
    serverOffset.value = sealTime - remaining - Date.now() / 1000;
  }
}

function applyDetail(payload: AnyRecord, initial = false) {
  detail.value = payload;
  gameIconFailed.value = false;
  const nextIssue = recordOf(payload.current_issue);
  currentIssue.value = nextIssue;
  syncServerClock(nextIssue);
  history.value = arrayOf(payload.history);
  latestHistoryKey = historyKey(history.value[0]);
  if (initial || !activePlayId.value || !plays.value.some((play) => String(play.id) === activePlayId.value)) {
    activePlayId.value = String(plays.value[0]?.id || "");
  }
  if (initial) amount.value = minBet.value;
}

async function loadDetail(showLoader = true) {
  if (!gameId && !gameCode) {
    loadError.value = t("commerce.gameBet.missingTableId");
    loading.value = false;
    return;
  }
  if (showLoader) loading.value = true;
  loadError.value = "";
  try {
    const payload = await getLotteryGameDetail(gameId, gameCode);
    if (!payload?.game) throw new Error(t("commerce.gameBet.tableUnavailable"));
    applyDetail(payload, showLoader || !detail.value.game);
    gameId = String(recordOf(payload.game).id || gameId);
    gameCode = textOf(recordOf(payload.game).game_code, gameCode);
    if (visible) scheduleSync(1200);
    await nextTick();
    if (visible) initCanvas();
  } catch (error: any) {
    if (showLoader || !detail.value.game) loadError.value = error?.message || t("commerce.gameBet.connectionFailed");
  } finally {
    loading.value = false;
  }
}

function historyKey(row: AnyRecord | undefined) {
  return row ? `${row.id || row.issue_num || ""}:${row.open_code || ""}` : "";
}

async function refreshHistory(reveal = false) {
  if (loadingHistory.value || (!gameId && !gameCode)) return false;
  loadingHistory.value = true;
  try {
    const payload = await getLotteryIssueHistory(gameId, gameCode, 1);
    const rows = arrayOf(payload?.list);
    if (!rows.length) return false;
    const nextKey = historyKey(rows[0]);
    const changed = Boolean(latestHistoryKey && nextKey && nextKey !== latestHistoryKey);
    history.value = rows;
    if (!latestHistoryKey) latestHistoryKey = nextKey;
    if (changed) {
      latestHistoryKey = nextKey;
      if (reveal || waitingIssueNum) triggerReveal();
      waitingIssueNum = "";
    }
    return changed;
  } catch {
    return false;
  } finally {
    loadingHistory.value = false;
  }
}

function triggerReveal() {
  revealSerial.value += 1;
  drawing.value = false;
  canvasBurstUntil = Date.now() + 900;
  if (revealTimer) clearTimeout(revealTimer);
  revealTimer = setTimeout(() => {
    // Retain the stable revealed state after the entrance animation.
  }, 1400);
}

async function syncRound() {
  if (syncing || (!gameId && !gameCode) || !visible) return;
  syncing = true;
  syncCycle += 1;
  try {
    const previousId = String(currentIssue.value.id || "");
    const previousIssue = String(currentIssue.value.issue_num || "");
    const nextIssue = await getLotteryCurrentIssue(gameId, gameCode);
    if (nextIssue?.id) {
      const nextId = String(nextIssue.id);
      if (previousId && nextId !== previousId) {
        waitingIssueNum = previousIssue;
        drawing.value = true;
        selectedOptionIds.value = [];
      }
      currentIssue.value = nextIssue;
      syncServerClock(nextIssue);
    }
    if (drawing.value || syncCycle % 3 === 0) {
      await refreshHistory(true);
    }
  } catch {
    if (sealCountdown.value <= 0) {
      waitingIssueNum = String(currentIssue.value.issue_num || waitingIssueNum);
      drawing.value = true;
      await refreshHistory(true);
    }
  } finally {
    syncing = false;
    if (visible) scheduleSync(sealCountdown.value <= 8 || drawing.value ? 1000 : 3000);
  }
}

function scheduleSync(delay = 0) {
  if (syncTimer) clearTimeout(syncTimer);
  if (!visible || (!gameId && !gameCode)) return;
  syncTimer = setTimeout(() => void syncRound(), Math.max(200, delay));
}

function startRuntime() {
  stopRuntime(false);
  visible = true;
  nowSeconds.value = Math.floor(Date.now() / 1000);
  clockTimer = setInterval(() => {
    const wasOpen = sealCountdown.value > 0;
    nowSeconds.value = Math.floor(Date.now() / 1000);
    if (wasOpen && sealCountdown.value <= 0) {
      waitingIssueNum = String(currentIssue.value.issue_num || "");
      drawing.value = true;
      scheduleSync(120);
    }
  }, 1000);
  scheduleSync(600);
  startCanvas();
}

function stopRuntime(markHidden = true) {
  if (markHidden) visible = false;
  if (clockTimer) clearInterval(clockTimer);
  if (syncTimer) clearTimeout(syncTimer);
  if (canvasTimer) clearInterval(canvasTimer);
  clockTimer = undefined;
  syncTimer = undefined;
  canvasTimer = undefined;
}

function initCanvas() {
  if (!visible) return;
  const query = uni.createSelectorQuery().in(instance?.proxy as any);
  query
    .select("#lotteryCanvas")
    .boundingClientRect((rect: any) => {
      if (rect?.width && rect?.height) {
        canvasWidth = rect.width;
        canvasHeight = rect.height;
      }
      canvasContext = uni.createCanvasContext("lotteryCanvas", instance?.proxy as any);
      startCanvas();
      drawCanvasFrame();
    })
    .exec();
}

function startCanvas() {
  if (!visible || !canvasContext || canvasTimer) return;
  canvasTimer = setInterval(drawCanvasFrame, 50);
}

function drawOval(ctx: any, cx: number, cy: number, rx: number, ry: number) {
  const k = 0.5522848;
  ctx.beginPath();
  ctx.moveTo(cx - rx, cy);
  ctx.bezierCurveTo(cx - rx, cy - ry * k, cx - rx * k, cy - ry, cx, cy - ry);
  ctx.bezierCurveTo(cx + rx * k, cy - ry, cx + rx, cy - ry * k, cx + rx, cy);
  ctx.bezierCurveTo(cx + rx, cy + ry * k, cx + rx * k, cy + ry, cx, cy + ry);
  ctx.bezierCurveTo(cx - rx * k, cy + ry, cx - rx, cy + ry * k, cx - rx, cy);
  ctx.closePath();
}

function drawCanvasFrame() {
  const ctx = canvasContext;
  if (!ctx) return;
  canvasFrame += 1;
  const width = canvasWidth;
  const height = canvasHeight;
  const cx = width / 2;
  const cy = height * 0.46;
  const fast = drawing.value ? 3.5 : 1;
  const t = canvasFrame * 0.025 * fast;

  ctx.clearRect(0, 0, width, height);
  const wash = ctx.createLinearGradient(0, 0, width, height);
  wash.addColorStop(0, "rgba(62, 45, 139, 0.08)");
  wash.addColorStop(0.5, "rgba(18, 194, 158, 0.05)");
  wash.addColorStop(1, "rgba(255, 69, 120, 0.08)");
  ctx.setFillStyle(wash);
  ctx.fillRect(0, 0, width, height);

  for (let ring = 0; ring < 4; ring += 1) {
    drawOval(ctx, cx, cy, width * (0.31 + ring * 0.045), height * (0.19 + ring * 0.027));
    ctx.setStrokeStyle(`rgba(${ring % 2 ? "255, 215, 115" : "114, 255, 224"}, ${0.12 - ring * 0.018})`);
    ctx.setLineWidth(ring === 0 ? 1.4 : 0.7);
    ctx.stroke();
  }

  const sparkCount = 28;
  for (let index = 0; index < sparkCount; index += 1) {
    const angle = index * 2.399 + t * (index % 3 ? 0.12 : -0.08);
    const radius = width * (0.14 + ((index * 17) % 100) / 360);
    const x = cx + Math.cos(angle) * radius;
    const y = cy + Math.sin(angle) * radius * 0.45;
    const pulse = 0.25 + (Math.sin(t * 2 + index) + 1) * 0.18;
    ctx.beginPath();
    ctx.arc(x, y, index % 5 === 0 ? 1.7 : 0.8, 0, Math.PI * 2);
    ctx.setFillStyle(`rgba(210, 255, 244, ${pulse})`);
    ctx.fill();
  }

  const orbitNumbers = latestNumbers.value.length ? latestNumbers.value : ["8", "19", "26", "31", "6", "12", "23", "41"];
  const ballCount = Math.min(8, Math.max(6, orbitNumbers.length));
  for (let index = 0; index < ballCount; index += 1) {
    const angle = t * (drawing.value ? 0.72 : 0.16) + (Math.PI * 2 * index) / ballCount;
    const wobble = drawing.value ? Math.sin(t * 2.4 + index * 1.7) * width * 0.045 : 0;
    const x = cx + Math.cos(angle) * (width * 0.24 + wobble);
    const y = cy + Math.sin(angle) * height * 0.115 + Math.cos(t * 1.6 + index) * (drawing.value ? 12 : 3);
    const size = Math.max(7, width * 0.025);
    const value = drawing.value
      ? String(((index * 11 + Math.floor(t * 9)) % 49) + 1)
      : orbitNumbers[index % orbitNumbers.length];
    const tone = ballTone(value);
    const colors = ["#6c5ce7", "#20c9a7", "#ff4f7b", "#f2b84b", "#3a8dff", "#a957e8"];

    ctx.beginPath();
    ctx.arc(x + 1.5, y + 3, size + 1, 0, Math.PI * 2);
    ctx.setFillStyle("rgba(0,0,0,.25)");
    ctx.fill();
    const gradient = ctx.createCircularGradient(x - size * 0.3, y - size * 0.35, size * 1.6);
    gradient.addColorStop(0, "#ffffff");
    gradient.addColorStop(0.16, colors[tone]);
    gradient.addColorStop(1, "#15122d");
    ctx.beginPath();
    ctx.arc(x, y, size, 0, Math.PI * 2);
    ctx.setFillStyle(gradient);
    ctx.fill();
    ctx.beginPath();
    ctx.arc(x - size * 0.28, y - size * 0.32, size * 0.18, 0, Math.PI * 2);
    ctx.setFillStyle("rgba(255,255,255,.78)");
    ctx.fill();
  }

  if (Date.now() < canvasBurstUntil) {
    const progress = 1 - (canvasBurstUntil - Date.now()) / 1050;
    for (let index = 0; index < 30; index += 1) {
      const angle = (Math.PI * 2 * index) / 30 + index * 0.17;
      const distance = progress * (45 + (index % 7) * 9);
      const x = cx + Math.cos(angle) * distance;
      const y = cy + Math.sin(angle) * distance * 0.7;
      const alpha = Math.max(0, 1 - progress);
      ctx.beginPath();
      ctx.arc(x, y, 1.5 + (index % 3), 0, Math.PI * 2);
      ctx.setFillStyle([`rgba(255,214,103,${alpha})`, `rgba(255,79,123,${alpha})`, `rgba(92,235,210,${alpha})`][index % 3]);
      ctx.fill();
    }
  }

  ctx.draw(false);
}

function goBack() {
  uni.navigateBack({
    fail: () => uni.switchTab({ url: "/pages/tabbar/game/index" })
  });
}

onLoad((options) => {
  gameId = String(options?.game_id || "");
  gameCode = String(options?.game_code || "");
  routeTitle = decodeURIComponent(String(options?.title || ""));
  void loadDetail(true);
});

onReady(() => {
  void nextTick(initCanvas);
});

onShow(() => {
  startRuntime();
  void nextTick(initCanvas);
});

onHide(() => stopRuntime(true));

onUnload(() => {
  stopRuntime(true);
  [successTimer, betFxTimer, burstTimer, shakeTimer, revealTimer].forEach((timer) => timer && clearTimeout(timer));
});
</script>

<style scoped>
.lottery-page {
  position: relative;
  min-height: 100vh;
  overflow-x: hidden;
  color: #f8f7ff;
  background:
    radial-gradient(circle at 50% -10%, rgba(103, 83, 221, 0.52), transparent 38%),
    linear-gradient(162deg, #090a1c 0%, #11132d 42%, #080a19 100%);
}

.velvet-noise,
.ambient,
.stage-grid,
.stage-sheen {
  position: absolute;
  pointer-events: none;
}

.velvet-noise {
  z-index: 0;
  inset: 0;
  opacity: 0.32;
  background-image:
    radial-gradient(circle at 20% 20%, rgba(255, 255, 255, 0.05) 0 1rpx, transparent 1.5rpx),
    radial-gradient(circle at 75% 70%, rgba(255, 255, 255, 0.035) 0 1rpx, transparent 1.5rpx);
  background-size: 18rpx 18rpx, 26rpx 26rpx;
}

.ambient {
  z-index: 0;
  width: 420rpx;
  height: 420rpx;
  border-radius: 50%;
  filter: blur(18rpx);
}

.ambient-a {
  top: 260rpx;
  right: -280rpx;
  background: rgba(255, 65, 135, 0.17);
}

.ambient-b {
  top: 940rpx;
  left: -300rpx;
  background: rgba(37, 214, 180, 0.12);
}

.top-nav {
  position: relative;
  z-index: 20;
  display: flex;
  min-height: 120rpx;
  align-items: center;
  gap: 18rpx;
  padding: calc(18rpx + var(--status-bar-height)) 24rpx 16rpx;
}

.round-icon {
  position: relative;
  display: flex;
  width: 68rpx;
  height: 68rpx;
  flex: 0 0 68rpx;
  align-items: center;
  justify-content: center;
  border: 1rpx solid rgba(255, 255, 255, 0.13);
  border-radius: 24rpx;
  background: rgba(255, 255, 255, 0.07);
  box-shadow: inset 0 1rpx 0 rgba(255, 255, 255, 0.14), 0 10rpx 30rpx rgba(0, 0, 0, 0.2);
  backdrop-filter: blur(16px);
}

.round-icon:active {
  transform: scale(0.94);
}

.arrow-left {
  width: 20rpx;
  height: 20rpx;
  border-bottom: 4rpx solid #fff;
  border-left: 4rpx solid #fff;
  transform: rotate(45deg) translate(3rpx, -3rpx);
}

.game-identity {
  display: flex;
  min-width: 0;
  flex: 1;
  align-items: center;
  gap: 14rpx;
}

.game-emblem {
  position: relative;
  display: flex;
  width: 74rpx;
  height: 74rpx;
  flex: 0 0 74rpx;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 2rpx solid rgba(255, 223, 128, 0.35);
  border-radius: 26rpx;
  background: linear-gradient(145deg, rgba(255, 229, 153, 0.22), rgba(125, 92, 255, 0.2));
  box-shadow: inset 0 0 24rpx rgba(255, 255, 255, 0.08), 0 8rpx 24rpx rgba(0, 0, 0, 0.22);
}

.game-emblem image {
  width: 62rpx;
  height: 62rpx;
}

.emblem-gem {
  width: 34rpx;
  height: 34rpx;
  border-radius: 9rpx;
  background: linear-gradient(135deg, #fff3a4, #ff697e 45%, #765eff 80%);
  box-shadow: 0 0 20rpx rgba(255, 213, 105, 0.48);
  transform: rotate(45deg);
}

.game-title-wrap {
  min-width: 0;
}

.game-title {
  display: block;
  max-width: 280rpx;
  overflow: hidden;
  color: #fff;
  font-size: 30rpx;
  font-weight: 800;
  letter-spacing: 1rpx;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.live-line {
  display: flex;
  align-items: center;
  gap: 8rpx;
  margin-top: 8rpx;
  color: rgba(236, 239, 255, 0.55);
  font-size: 18rpx;
  letter-spacing: 2rpx;
}

.live-dot {
  width: 10rpx;
  height: 10rpx;
  border-radius: 50%;
  background: #45e5bd;
  box-shadow: 0 0 14rpx #45e5bd;
  animation: livePulse 1.7s ease-in-out infinite;
}

.phase-sealed .live-dot {
  background: #ff6b82;
  box-shadow: 0 0 14rpx #ff6b82;
}

.phase-drawing .live-dot {
  background: #ffd467;
  box-shadow: 0 0 14rpx #ffd467;
}

.nav-actions {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 12rpx;
}

.balance-pill {
  display: flex;
  height: 60rpx;
  align-items: center;
  gap: 9rpx;
  padding: 0 18rpx 0 11rpx;
  border: 1rpx solid rgba(255, 221, 118, 0.19);
  border-radius: 999rpx;
  color: #ffe7a0;
  font-size: 22rpx;
  font-weight: 800;
  background: rgba(255, 204, 83, 0.08);
}

.coin-mini {
  width: 30rpx;
  height: 30rpx;
  border: 4rpx double #ffe197;
  border-radius: 50%;
  background: linear-gradient(135deg, #ffeaa3, #d99020);
  box-shadow: 0 0 12rpx rgba(255, 213, 103, 0.34);
}

.history-bars {
  display: flex;
  width: 29rpx;
  height: 30rpx;
  align-items: flex-end;
  gap: 4rpx;
}

.history-bars i {
  width: 6rpx;
  border-radius: 4rpx;
  background: linear-gradient(180deg, #fff2b0, #b66dff);
}

.history-bars i:nth-child(1) { height: 15rpx; }
.history-bars i:nth-child(2) { height: 28rpx; }
.history-bars i:nth-child(3) { height: 21rpx; }

.content-wrap {
  position: relative;
  z-index: 2;
  padding: 0 22rpx 286rpx;
}

.draw-stage {
  position: relative;
  height: 450rpx;
  overflow: hidden;
  border: 1rpx solid rgba(255, 255, 255, 0.13);
  border-radius: 42rpx;
  background:
    radial-gradient(ellipse at 50% 55%, rgba(30, 193, 157, 0.16), transparent 42%),
    linear-gradient(145deg, rgba(30, 27, 72, 0.98), rgba(10, 14, 37, 0.98));
  box-shadow:
    inset 0 1rpx 0 rgba(255, 255, 255, 0.16),
    inset 0 -40rpx 70rpx rgba(0, 0, 0, 0.24),
    0 30rpx 80rpx rgba(0, 0, 0, 0.36);
}

.lottery-canvas {
  position: absolute;
  z-index: 1;
  inset: 0;
  width: 100%;
  height: 100%;
}

.stage-grid {
  z-index: 0;
  inset: 0;
  opacity: 0.18;
  background-image:
    linear-gradient(rgba(130, 255, 231, 0.12) 1rpx, transparent 1rpx),
    linear-gradient(90deg, rgba(130, 255, 231, 0.1) 1rpx, transparent 1rpx);
  background-size: 44rpx 44rpx;
  mask-image: linear-gradient(to bottom, transparent, #000 44%, transparent 96%);
}

.stage-sheen {
  z-index: 2;
  top: -220rpx;
  left: -20%;
  width: 140%;
  height: 320rpx;
  background: linear-gradient(110deg, transparent 25%, rgba(255, 255, 255, 0.06) 50%, transparent 70%);
  transform: rotate(-8deg);
  animation: stageSheen 6s ease-in-out infinite;
}

.issue-badge {
  position: absolute;
  z-index: 4;
  top: 24rpx;
  left: 24rpx;
  display: flex;
  height: 48rpx;
  align-items: center;
  gap: 10rpx;
  padding: 0 16rpx;
  border: 1rpx solid rgba(93, 237, 208, 0.2);
  border-radius: 16rpx;
  color: rgba(218, 255, 247, 0.74);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 18rpx;
  letter-spacing: 1rpx;
  background: rgba(23, 154, 127, 0.09);
}

.signal-waves {
  display: flex;
  height: 20rpx;
  align-items: flex-end;
  gap: 3rpx;
}

.signal-waves i {
  width: 3rpx;
  border-radius: 4rpx;
  background: #5cebd2;
  animation: signal 0.9s ease-in-out infinite alternate;
}

.signal-waves i:nth-child(1) { height: 7rpx; }
.signal-waves i:nth-child(2) { height: 14rpx; animation-delay: -0.3s; }
.signal-waves i:nth-child(3) { height: 20rpx; animation-delay: -0.6s; }

.rule-button {
  position: absolute;
  z-index: 5;
  top: 24rpx;
  right: 24rpx;
  display: flex;
  width: 46rpx;
  height: 46rpx;
  align-items: center;
  justify-content: center;
  border: 1rpx solid rgba(255, 255, 255, 0.15);
  border-radius: 50%;
  color: rgba(255, 255, 255, 0.7);
  font-size: 22rpx;
  font-weight: 700;
  background: rgba(255, 255, 255, 0.07);
}

.countdown-dial {
  position: absolute;
  z-index: 4;
  top: 76rpx;
  left: 50%;
  display: flex;
  width: 168rpx;
  height: 168rpx;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: conic-gradient(from 40deg, rgba(94, 239, 207, 0.1), #66f0d2, rgba(116, 91, 255, 0.16), #ffd56b, rgba(94, 239, 207, 0.1));
  box-shadow: 0 0 46rpx rgba(72, 235, 202, 0.2);
  transform: translateX(-50%);
  animation: dialFloat 3.2s ease-in-out infinite;
}

.countdown-dial::before {
  position: absolute;
  inset: 5rpx;
  border-radius: 50%;
  background: #111532;
  box-shadow: inset 0 0 34rpx rgba(92, 235, 210, 0.1);
  content: "";
}

.countdown-dial.urgent {
  background: conic-gradient(from 40deg, #ff6a82, rgba(255, 95, 130, 0.12), #ffd56b, #ff6a82);
  box-shadow: 0 0 52rpx rgba(255, 77, 110, 0.32);
  animation: urgentPulse 0.8s ease-in-out infinite;
}

.dial-ticks {
  position: absolute;
  inset: 14rpx;
  border: 2rpx dotted rgba(255, 255, 255, 0.18);
  border-radius: 50%;
  animation: dialSpin 18s linear infinite;
}

.dial-core {
  position: relative;
  z-index: 2;
  text-align: center;
}

.countdown-value {
  display: block;
  color: #fff8dd;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 38rpx;
  font-weight: 800;
  letter-spacing: 1rpx;
  text-shadow: 0 0 18rpx rgba(255, 219, 116, 0.35);
}

.countdown-label {
  display: block;
  margin-top: 5rpx;
  color: rgba(212, 255, 246, 0.54);
  font-size: 16rpx;
  letter-spacing: 5rpx;
}

.result-bay {
  position: absolute;
  z-index: 5;
  right: 40rpx;
  bottom: 68rpx;
  left: 40rpx;
  display: flex;
  min-height: 90rpx;
  align-items: center;
  justify-content: center;
  padding: 12rpx 20rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.1);
  border-radius: 999rpx;
  background: linear-gradient(180deg, rgba(2, 7, 19, 0.52), rgba(8, 12, 29, 0.86));
  box-shadow: inset 0 12rpx 26rpx rgba(0, 0, 0, 0.32), 0 8rpx 28rpx rgba(0, 0, 0, 0.22);
  backdrop-filter: blur(12px);
}

.result-track,
.waiting-result,
.drawing-capsules {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10rpx;
}

.result-piece {
  position: relative;
  display: flex;
  width: 62rpx;
  height: 62rpx;
  flex: 0 0 62rpx;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 3rpx solid rgba(255, 255, 255, 0.78);
  border-radius: 50%;
  color: #fff;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 23rpx;
  font-weight: 900;
  background: radial-gradient(circle at 30% 24%, #fff, var(--ball-color, #6858db) 18%, #241b59 76%);
  box-shadow: inset -7rpx -10rpx 16rpx rgba(0, 0, 0, 0.24), 0 8rpx 16rpx rgba(0, 0, 0, 0.28), 0 0 16rpx var(--ball-glow, rgba(104, 88, 219, 0.3));
  animation: revealBall 0.55s cubic-bezier(0.18, 0.89, 0.32, 1.28) both;
}

.result-piece::after {
  position: absolute;
  top: 8rpx;
  left: 12rpx;
  width: 14rpx;
  height: 8rpx;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.68);
  content: "";
  transform: rotate(-28deg);
}

.result-piece.mode-dice,
.result-piece.mode-card {
  width: 68rpx;
  height: 76rpx;
  flex-basis: 68rpx;
  border: 0;
  border-radius: 12rpx;
  background: #fff;
}

.result-piece.mode-card {
  height: 88rpx;
  align-items: flex-start;
  justify-content: flex-start;
  padding: 8rpx;
  color: #171829;
}

.result-piece.mode-card.red { color: #e63f58; }
.result-piece.mode-card::after,
.result-piece.mode-dice::after { display: none; }

.result-dice {
  display: grid;
  width: 100%;
  height: 100%;
  grid-template-columns: repeat(3, 1fr);
  grid-template-rows: repeat(3, 1fr);
  gap: 3rpx;
  padding: 11rpx;
  background: linear-gradient(145deg, #fff, #e9e9f3);
}

.result-dice i {
  display: block;
  width: 9rpx;
  height: 9rpx;
  align-self: center;
  justify-self: center;
  border-radius: 50%;
  background: transparent;
}

.result-dice i.on {
  background: var(--ball-color);
  box-shadow: inset -2rpx -2rpx 3rpx rgba(0, 0, 0, 0.22);
}

.card-rank { font-size: 22rpx; font-weight: 900; }
.card-suit { margin: 20rpx 0 0 -13rpx; font-size: 24rpx; }

.tone-1 { --ball-color: #6b5bea; --ball-glow: rgba(107, 91, 234, 0.48); }
.tone-2 { --ball-color: #ff4f7b; --ball-glow: rgba(255, 79, 123, 0.48); }
.tone-3 { --ball-color: #20c9a7; --ball-glow: rgba(32, 201, 167, 0.46); }
.tone-4 { --ball-color: #3a8dff; --ball-glow: rgba(58, 141, 255, 0.48); }
.tone-5 { --ball-color: #efb540; --ball-glow: rgba(239, 181, 64, 0.48); }

.empty-slot {
  width: 52rpx;
  height: 52rpx;
  border: 2rpx dashed rgba(255, 255, 255, 0.17);
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.025);
}

.drawing-capsules {
  position: relative;
  width: 100%;
  height: 68rpx;
}

.shuffle-ball {
  position: absolute;
  left: 50%;
  width: 48rpx;
  height: 48rpx;
  border-radius: 50%;
  background: radial-gradient(circle at 30% 25%, #fff, #ffcf68 18%, #9a3dff 70%);
  box-shadow: 0 0 18rpx rgba(255, 208, 104, 0.35);
  animation: shuffleBall 0.72s ease-in-out infinite alternate;
  transform: translateX(var(--shuffle-x));
}

.shuffle-ball i {
  position: absolute;
  top: 8rpx;
  left: 11rpx;
  width: 12rpx;
  height: 7rpx;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.72);
}

.stage-caption {
  position: absolute;
  z-index: 5;
  right: 46rpx;
  bottom: 25rpx;
  left: 46rpx;
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: rgba(221, 229, 255, 0.42);
  font-size: 16rpx;
  letter-spacing: 1rpx;
}

.caption-status {
  display: flex;
  align-items: center;
  gap: 8rpx;
}

.caption-status i {
  width: 7rpx;
  height: 7rpx;
  border-radius: 50%;
  background: #54e8c8;
  box-shadow: 0 0 10rpx #54e8c8;
}

.open-clock {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  color: rgba(255, 229, 151, 0.5);
}

.prop-card,
.prop-tile,
.prop-zodiac {
  position: absolute;
  z-index: 3;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  pointer-events: none;
  opacity: 0.22;
}

.prop-card {
  width: 54rpx;
  height: 76rpx;
  padding: 7rpx;
  border-radius: 8rpx;
  color: #17162b;
  font-size: 18rpx;
  font-weight: 900;
  background: #fff;
}

.prop-card i { align-self: flex-end; font-size: 22rpx; font-style: normal; }
.prop-card-left { top: 108rpx; left: 34rpx; transform: rotate(-16deg); }
.prop-card-right { top: 98rpx; right: 40rpx; color: #e63e55; transform: rotate(14deg); }

.prop-tile,
.prop-zodiac {
  width: 58rpx;
  height: 68rpx;
  align-items: center;
  justify-content: center;
  border: 5rpx solid #d7d9bf;
  border-radius: 8rpx;
  font-family: serif;
  font-size: 27rpx;
  font-weight: 900;
  background: #f5f0d8;
  box-shadow: inset 0 -8rpx 0 #bec8a4;
}

.prop-tile { top: 200rpx; left: 32rpx; color: #2c9b70; transform: rotate(9deg); }
.prop-zodiac { top: 202rpx; right: 34rpx; color: #bd4151; transform: rotate(-8deg); }

.phase-drawing .draw-stage {
  box-shadow: inset 0 1rpx 0 rgba(255, 255, 255, 0.18), 0 0 80rpx rgba(255, 206, 99, 0.12), 0 30rpx 80rpx rgba(0, 0, 0, 0.38);
}

.recent-ribbon {
  position: relative;
  z-index: 4;
  display: flex;
  min-height: 102rpx;
  align-items: center;
  gap: 16rpx;
  margin: -16rpx 18rpx 24rpx;
  padding: 20rpx 42rpx 18rpx 24rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.12);
  border-radius: 20rpx 20rpx 28rpx 28rpx;
  background: linear-gradient(100deg, rgba(31, 31, 73, 0.98), rgba(15, 19, 44, 0.98));
  box-shadow: 0 16rpx 36rpx rgba(0, 0, 0, 0.25), inset 0 1rpx 0 rgba(255, 255, 255, 0.1);
}

.ribbon-handle {
  display: flex;
  width: 22rpx;
  flex: 0 0 22rpx;
  flex-direction: column;
  gap: 5rpx;
}

.ribbon-handle i {
  height: 3rpx;
  border-radius: 3rpx;
  background: rgba(255, 255, 255, 0.23);
}

.recent-round {
  min-width: 0;
  flex: 1;
}

.recent-issue {
  display: block;
  color: rgba(219, 225, 255, 0.42);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 14rpx;
}

.recent-balls {
  display: flex;
  gap: 5rpx;
  margin-top: 8rpx;
}

.recent-ball,
.history-ball {
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #fff;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-weight: 800;
  background: var(--ball-color);
  box-shadow: inset -3rpx -4rpx 6rpx rgba(0, 0, 0, 0.2);
}

.recent-ball {
  width: 28rpx;
  height: 28rpx;
  font-size: 12rpx;
}

.ribbon-arrow {
  position: absolute;
  right: 22rpx;
  width: 14rpx;
  height: 14rpx;
  border-top: 3rpx solid rgba(255, 255, 255, 0.33);
  border-right: 3rpx solid rgba(255, 255, 255, 0.33);
  transform: rotate(45deg);
}

.bet-table {
  position: relative;
  min-height: 620rpx;
  overflow: hidden;
  padding: 22rpx 20rpx 36rpx;
  border: 7rpx solid #201c43;
  border-radius: 44rpx;
  background:
    radial-gradient(ellipse at 50% 35%, rgba(26, 166, 136, 0.24), transparent 55%),
    linear-gradient(145deg, #123d42, #102b36 52%, #121c33);
  box-shadow:
    inset 0 0 0 2rpx rgba(103, 255, 225, 0.13),
    inset 0 -80rpx 120rpx rgba(0, 0, 0, 0.2),
    0 24rpx 70rpx rgba(0, 0, 0, 0.38);
}

.table-rim {
  position: absolute;
  inset: 12rpx;
  border: 1rpx solid rgba(108, 245, 218, 0.1);
  border-radius: 32rpx;
  pointer-events: none;
}

.table-corner {
  position: absolute;
  width: 150rpx;
  height: 150rpx;
  border: 1rpx solid rgba(255, 213, 103, 0.11);
  border-radius: 50%;
  pointer-events: none;
}

.corner-a { right: -80rpx; bottom: -80rpx; }
.corner-b { top: 170rpx; left: -105rpx; }

.play-tabs {
  position: relative;
  z-index: 3;
  width: 100%;
  white-space: nowrap;
}

.play-tabs-inner {
  display: inline-flex;
  gap: 12rpx;
  padding: 2rpx 4rpx 16rpx;
}

.play-tab {
  display: inline-flex;
  height: 68rpx;
  align-items: center;
  gap: 10rpx;
  padding: 0 9rpx 0 9rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.08);
  border-radius: 22rpx;
  color: rgba(225, 235, 255, 0.54);
  font-size: 18rpx;
  font-weight: 700;
  background: rgba(5, 12, 25, 0.24);
}

.play-tab.active {
  border-color: rgba(255, 218, 112, 0.32);
  color: #fff7db;
  background: linear-gradient(120deg, rgba(255, 206, 86, 0.17), rgba(121, 91, 255, 0.16));
  box-shadow: 0 8rpx 22rpx rgba(0, 0, 0, 0.15), inset 0 1rpx 0 rgba(255, 255, 255, 0.12);
}

.play-glyph {
  display: flex;
  width: 38rpx;
  height: 38rpx;
  align-items: center;
  justify-content: center;
  border-radius: 16rpx;
  color: rgba(255, 255, 255, 0.64);
  font-size: 17rpx;
  font-weight: 900;
  background: rgba(255, 255, 255, 0.08);
}

.play-tab.active .play-glyph {
  color: #271d4b;
  background: linear-gradient(135deg, #ffe28a, #ff8d89);
  box-shadow: 0 0 18rpx rgba(255, 211, 105, 0.24);
}

.table-heading {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  padding: 20rpx 10rpx 26rpx;
}

.table-kicker {
  display: block;
  color: rgba(100, 235, 209, 0.42);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 14rpx;
  letter-spacing: 3rpx;
}

.table-title {
  display: block;
  margin-top: 6rpx;
  color: #f5fff9;
  font-size: 29rpx;
  font-weight: 800;
  letter-spacing: 2rpx;
}

.selection-counter {
  display: flex;
  height: 50rpx;
  align-items: center;
  gap: 10rpx;
  padding: 0 14rpx;
  border-radius: 999rpx;
  color: rgba(255, 255, 255, 0.4);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 20rpx;
  font-weight: 800;
  background: rgba(0, 0, 0, 0.16);
}

.selection-counter.active { color: #ffe495; }

.mini-chip-stack {
  position: relative;
  width: 38rpx;
  height: 26rpx;
}

.mini-chip-stack i {
  position: absolute;
  width: 28rpx;
  height: 12rpx;
  border: 2rpx solid rgba(255, 255, 255, 0.42);
  border-radius: 50%;
  background: #7759ef;
}

.mini-chip-stack i:nth-child(1) { top: 0; left: 0; }
.mini-chip-stack i:nth-child(2) { top: 7rpx; left: 5rpx; background: #ff5b82; }
.mini-chip-stack i:nth-child(3) { top: 14rpx; left: 10rpx; background: #e9b946; }

.option-grid {
  position: relative;
  z-index: 2;
  display: grid;
  gap: 14rpx;
  padding: 0 4rpx;
}

.grid-roomy { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.grid-standard { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.grid-dense { grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10rpx; }

.bet-option {
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 174rpx;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 14rpx 7rpx 12rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.09);
  border-radius: 26rpx;
  color: #fff;
  background: linear-gradient(145deg, rgba(255, 255, 255, 0.075), rgba(2, 10, 23, 0.16));
  box-shadow: inset 0 1rpx 0 rgba(255, 255, 255, 0.08), 0 10rpx 22rpx rgba(0, 0, 0, 0.13);
  transition: transform 0.18s ease, border-color 0.2s ease, background 0.2s ease;
}

.grid-dense .bet-option { min-height: 155rpx; border-radius: 22rpx; }

.bet-option:active { transform: scale(0.94); }
.bet-option[disabled] { opacity: 0.38; }

.bet-option.selected {
  z-index: 2;
  border-color: rgba(255, 220, 117, 0.7);
  background: linear-gradient(145deg, rgba(255, 222, 126, 0.18), rgba(114, 86, 234, 0.2));
  box-shadow: inset 0 1rpx 0 rgba(255, 255, 255, 0.25), 0 0 0 2rpx rgba(255, 207, 95, 0.08), 0 14rpx 30rpx rgba(0, 0, 0, 0.22);
  transform: translateY(-7rpx);
}

.option-halo {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 100rpx;
  height: 100rpx;
  border-radius: 50%;
  background: radial-gradient(circle, var(--ball-glow), transparent 65%);
  transform: translate(-50%, -58%);
}

.selected .option-halo {
  width: 145rpx;
  height: 145rpx;
  animation: optionGlow 1.4s ease-in-out infinite;
}

.option-visual {
  position: relative;
  z-index: 2;
  display: flex;
  width: 76rpx;
  height: 76rpx;
  align-items: center;
  justify-content: center;
}

.grid-dense .option-visual { width: 66rpx; height: 66rpx; }

.token-face {
  position: relative;
  display: flex;
  width: 72rpx;
  height: 72rpx;
  align-items: center;
  justify-content: center;
  border: 4rpx solid rgba(255, 255, 255, 0.74);
  border-radius: 50%;
  color: #fff;
  font-size: 25rpx;
  font-weight: 900;
  background: radial-gradient(circle at 29% 24%, #fff, var(--ball-color) 19%, #1f2146 80%);
  box-shadow: inset -8rpx -10rpx 14rpx rgba(0, 0, 0, 0.2), 0 8rpx 18rpx rgba(0, 0, 0, 0.24), 0 0 18rpx var(--ball-glow);
}

.grid-dense .token-face { width: 62rpx; height: 62rpx; font-size: 22rpx; }

.visual-zodiac .token-face {
  border: 2rpx solid rgba(255, 228, 160, 0.62);
  border-radius: 24rpx 24rpx 50% 50%;
  font-family: serif;
  background: linear-gradient(150deg, #fff0b4 0%, var(--ball-color) 48%, #34234e 100%);
}

.visual-zodiac .token-face i {
  position: absolute;
  right: -4rpx;
  bottom: -4rpx;
  display: flex;
  width: 27rpx;
  height: 27rpx;
  align-items: center;
  justify-content: center;
  border: 2rpx solid rgba(255, 255, 255, 0.6);
  border-radius: 50%;
  color: #332041;
  font-family: ui-monospace, monospace;
  font-size: 10rpx;
  font-style: normal;
  background: #ffe59c;
}

.visual-color .token-face {
  border-width: 8rpx;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: inset 0 0 0 8rpx var(--ball-color), 0 0 22rpx var(--ball-glow);
}

.visual-color .token-face text { color: var(--ball-color); font-size: 19rpx; }

.dice-face {
  display: grid;
  width: 70rpx;
  height: 70rpx;
  grid-template-columns: repeat(3, 1fr);
  grid-template-rows: repeat(3, 1fr);
  gap: 4rpx;
  padding: 10rpx;
  border-radius: 16rpx;
  background: linear-gradient(145deg, #fff, #e9e9f3);
  box-shadow: inset -7rpx -8rpx 13rpx rgba(36, 31, 65, 0.16), 0 9rpx 18rpx rgba(0, 0, 0, 0.25);
  transform: rotate(-3deg);
}

.dice-face i {
  display: block;
  width: 11rpx;
  height: 11rpx;
  align-self: center;
  justify-self: center;
  border-radius: 50%;
  background: transparent;
}

.dice-face i.on {
  background: var(--ball-color);
  box-shadow: inset -2rpx -2rpx 3rpx rgba(0, 0, 0, 0.2);
}

.card-face {
  position: relative;
  width: 60rpx;
  height: 80rpx;
  padding: 7rpx;
  border: 2rpx solid rgba(255, 255, 255, 0.8);
  border-radius: 9rpx;
  color: #17182d;
  text-align: left;
  background: linear-gradient(145deg, #fff, #eeeefa);
  box-shadow: 0 9rpx 18rpx rgba(0, 0, 0, 0.26);
  transform: rotate(-4deg);
}

.tone-2 .card-face,
.tone-4 .card-face { color: #e63c59; }
.card-face text { display: block; font-size: 19rpx; font-weight: 900; }
.card-face i { position: absolute; right: 8rpx; bottom: 7rpx; font-size: 28rpx; font-style: normal; }

.mahjong-face {
  position: relative;
  display: flex;
  width: 68rpx;
  height: 78rpx;
  align-items: center;
  justify-content: center;
  border: 6rpx solid #d8ddc7;
  border-radius: 10rpx;
  color: #b93450;
  font-family: serif;
  font-size: 23rpx;
  font-weight: 900;
  background: #fffbe6;
  box-shadow: inset 0 -10rpx 0 #bbc8a5, 0 10rpx 17rpx rgba(0, 0, 0, 0.25);
}

.mahjong-face i {
  position: absolute;
  right: 4rpx;
  bottom: 9rpx;
  color: #309468;
  font-size: 12rpx;
  font-style: normal;
}

.option-meta {
  position: relative;
  z-index: 2;
  display: flex;
  width: 100%;
  min-width: 0;
  align-items: center;
  justify-content: center;
  gap: 6rpx;
  margin-top: 10rpx;
}

.option-name {
  max-width: 66%;
  overflow: hidden;
  color: rgba(245, 251, 255, 0.72);
  font-size: 19rpx;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.option-odds {
  color: #ffe18b;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 17rpx;
  font-weight: 700;
}

.grid-dense .option-name { display: none; }
.grid-dense .option-odds { font-size: 15rpx; }
.bet-option.burst::after {
  position: absolute;
  z-index: 0;
  width: 40rpx;
  height: 40rpx;
  border: 4rpx solid rgba(255, 221, 116, 0.65);
  border-radius: 50%;
  content: "";
  animation: selectBurst 0.52s ease-out both;
}

.no-options {
  display: flex;
  min-height: 360rpx;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: rgba(255, 255, 255, 0.38);
  font-size: 21rpx;
}

.empty-fan {
  position: relative;
  width: 140rpx;
  height: 90rpx;
  margin-bottom: 28rpx;
}

.empty-fan i {
  position: absolute;
  bottom: 0;
  left: 50%;
  width: 54rpx;
  height: 78rpx;
  border: 2rpx solid rgba(255, 255, 255, 0.14);
  border-radius: 9rpx;
  background: rgba(255, 255, 255, 0.04);
  transform-origin: 50% 100%;
}

.empty-fan i:nth-child(1) { transform: translateX(-50%) rotate(-20deg); }
.empty-fan i:nth-child(2) { transform: translateX(-50%); }
.empty-fan i:nth-child(3) { transform: translateX(-50%) rotate(20deg); }

.chip-dock {
  position: fixed;
  z-index: 40;
  right: 0;
  bottom: 0;
  left: 0;
  padding: 16rpx 22rpx calc(18rpx + env(safe-area-inset-bottom));
  border-top: 1rpx solid rgba(255, 255, 255, 0.13);
  background: linear-gradient(180deg, rgba(20, 20, 48, 0.9), rgba(8, 9, 24, 0.98));
  box-shadow: 0 -24rpx 60rpx rgba(0, 0, 0, 0.42), inset 0 1rpx 0 rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(24px);
}

.dock-topline {
  position: absolute;
  top: -1rpx;
  left: 50%;
  width: 180rpx;
  height: 2rpx;
  background: linear-gradient(90deg, transparent, rgba(255, 215, 106, 0.8), transparent);
  transform: translateX(-50%);
}

.quick-chip-row {
  display: flex;
  align-items: center;
  gap: 14rpx;
  margin-bottom: 14rpx;
  padding: 0 8rpx;
}

.quick-chip {
  position: relative;
  display: flex;
  width: 74rpx;
  height: 74rpx;
  flex: 0 0 74rpx;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 5rpx dashed rgba(255, 255, 255, 0.6);
  border-radius: 50%;
  color: #fff;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 19rpx;
  font-weight: 900;
  background: var(--chip-color);
  box-shadow: inset 0 0 0 5rpx rgba(0, 0, 0, 0.19), 0 7rpx 12rpx rgba(0, 0, 0, 0.25);
  transition: transform 0.18s ease;
}

.quick-chip::after {
  position: absolute;
  inset: 9rpx 13rpx;
  border: 2rpx solid rgba(255, 255, 255, 0.42);
  border-radius: 50%;
  content: "";
}

.quick-chip text { position: relative; z-index: 2; }
.quick-chip i { display: none; }
.quick-chip.active { transform: translateY(-9rpx) scale(1.08); box-shadow: inset 0 0 0 5rpx rgba(0,0,0,.16), 0 0 20rpx var(--chip-color); }
.chip-0 { --chip-color: #6f58dc; }
.chip-1 { --chip-color: #1aa987; }
.chip-2 { --chip-color: #e24c70; }
.chip-3 { --chip-color: #d99a2d; }

.dock-main {
  display: grid;
  grid-template-columns: minmax(0, 0.82fr) minmax(0, 1.18fr);
  gap: 16rpx;
}

.amount-stepper {
  display: grid;
  height: 90rpx;
  grid-template-columns: 58rpx minmax(0, 1fr) 58rpx;
  align-items: center;
  border: 1rpx solid rgba(255, 255, 255, 0.1);
  border-radius: 28rpx;
  background: rgba(255, 255, 255, 0.05);
  box-shadow: inset 0 1rpx 0 rgba(255, 255, 255, 0.07);
}

.amount-stepper button,
.amount-stepper > uni-button {
  display: flex;
  height: 58rpx;
  align-items: center;
  justify-content: center;
  color: rgba(255, 255, 255, 0.62);
  font-size: 31rpx;
  background: transparent;
}

.amount-stepper > uni-button::after {
  border: 0;
}

.amount-display {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: center;
  gap: 5rpx;
}

.currency-mark {
  color: #ffd76a;
  font-size: 18rpx;
}

.amount-display input {
  width: 100%;
  height: 60rpx;
  color: #fff6d5;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 28rpx;
  font-weight: 900;
  text-align: center;
}

.place-bet {
  position: relative;
  display: flex;
  height: 90rpx;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1rpx solid rgba(255, 255, 255, 0.12);
  border-radius: 28rpx;
  color: rgba(255, 255, 255, 0.42);
  background: linear-gradient(135deg, #34334d, #25263b);
  box-shadow: inset 0 1rpx 0 rgba(255, 255, 255, 0.1);
}

.place-bet.ready {
  color: #281939;
  background: linear-gradient(120deg, #ffe078 0%, #ffab6d 42%, #ff6e91 100%);
  box-shadow: 0 12rpx 28rpx rgba(255, 99, 127, 0.28), inset 0 2rpx 0 rgba(255, 255, 255, 0.5);
}

.place-bet:active { transform: scale(0.97); }

.bet-button-shine {
  position: absolute;
  top: -80%;
  left: -35%;
  width: 70rpx;
  height: 260%;
  background: rgba(255, 255, 255, 0.26);
  transform: rotate(28deg);
}

.ready .bet-button-shine { animation: buttonShine 2.6s ease-in-out infinite; }

.bet-lever {
  position: relative;
  z-index: 2;
  width: 34rpx;
  height: 48rpx;
  margin-right: 16rpx;
  border-radius: 16rpx;
  background: rgba(0, 0, 0, 0.16);
}

.bet-lever::before {
  position: absolute;
  top: 5rpx;
  left: 14rpx;
  width: 6rpx;
  height: 31rpx;
  border-radius: 4rpx;
  background: currentColor;
  content: "";
  transform: rotate(18deg);
  transform-origin: bottom;
}

.bet-lever i {
  position: absolute;
  z-index: 2;
  top: 1rpx;
  left: 18rpx;
  width: 15rpx;
  height: 15rpx;
  border-radius: 50%;
  background: currentColor;
}

.locking .bet-lever::before { animation: leverPull 0.45s ease-in-out infinite alternate; }

.bet-copy {
  position: relative;
  z-index: 2;
  min-width: 100rpx;
  text-align: left;
}

.bet-label {
  display: block;
  font-size: 25rpx;
  font-weight: 900;
  letter-spacing: 3rpx;
}

.bet-total {
  display: block;
  margin-top: 5rpx;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 17rpx;
  font-weight: 700;
  opacity: 0.7;
}

.dock-shake .chip-dock { animation: dockShake 0.42s ease; }

.chip-flight-layer {
  position: fixed;
  z-index: 70;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
}

.chip-flight-layer i {
  position: absolute;
  bottom: 76rpx;
  left: 68%;
  width: 34rpx;
  height: 24rpx;
  border: 4rpx dashed rgba(255, 255, 255, 0.72);
  border-radius: 50%;
  background: var(--flight-color);
  box-shadow: inset 0 0 0 4rpx rgba(0, 0, 0, 0.16), 0 5rpx 12rpx rgba(0, 0, 0, 0.28);
  animation: chipFlight 0.82s cubic-bezier(0.25, 0.76, 0.37, 0.98) var(--flight-delay) both;
}

.success-layer,
.sheet-mask {
  position: fixed;
  z-index: 100;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 30rpx;
  background: rgba(4, 5, 17, 0.68);
  backdrop-filter: blur(16px);
  animation: maskIn 0.22s ease both;
}

.success-ticket {
  position: relative;
  display: flex;
  width: 410rpx;
  min-height: 400rpx;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1rpx solid rgba(255, 236, 176, 0.35);
  border-radius: 42rpx;
  background:
    radial-gradient(circle at 50% 20%, rgba(255, 218, 106, 0.21), transparent 34%),
    linear-gradient(145deg, #292257, #151630);
  box-shadow: 0 30rpx 100rpx rgba(0, 0, 0, 0.54), inset 0 1rpx 0 rgba(255, 255, 255, 0.18);
  animation: ticketIn 0.48s cubic-bezier(0.18, 0.89, 0.32, 1.18) both;
}

.success-halo {
  position: absolute;
  top: 45rpx;
  width: 180rpx;
  height: 180rpx;
  border: 2rpx dotted rgba(255, 223, 130, 0.38);
  border-radius: 50%;
  animation: dialSpin 8s linear infinite;
}

.success-halo i {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 100rpx;
  height: 100rpx;
  border-radius: 50%;
  background: rgba(255, 213, 99, 0.12);
  box-shadow: 0 0 60rpx rgba(255, 213, 99, 0.35);
  transform: translate(-50%, -50%);
}

.success-check {
  position: relative;
  z-index: 2;
  display: flex;
  width: 92rpx;
  height: 92rpx;
  align-items: center;
  justify-content: center;
  border: 6rpx double #fff2bd;
  border-radius: 50%;
  background: linear-gradient(135deg, #ffd96b, #ff826f);
  box-shadow: 0 0 40rpx rgba(255, 200, 88, 0.44);
}

.success-check i {
  width: 36rpx;
  height: 20rpx;
  border-bottom: 7rpx solid #392144;
  border-left: 7rpx solid #392144;
  transform: rotate(-45deg) translate(3rpx, -4rpx);
}

.success-title {
  position: relative;
  z-index: 2;
  margin-top: 36rpx;
  color: #fff7db;
  font-size: 30rpx;
  font-weight: 900;
  letter-spacing: 4rpx;
}

.success-amount {
  position: relative;
  z-index: 2;
  margin-top: 14rpx;
  color: #ffd96d;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 25rpx;
  font-weight: 800;
}

.ticket-cut {
  position: absolute;
  right: 0;
  bottom: -10rpx;
  left: 0;
  display: flex;
  justify-content: space-around;
}

.ticket-cut i {
  width: 24rpx;
  height: 24rpx;
  border-radius: 50%;
  background: #080a19;
}

.sheet-mask {
  z-index: 90;
  align-items: flex-end;
  padding: 0;
}

.history-sheet,
.rule-sheet {
  width: 100%;
  border-top: 1rpx solid rgba(255, 255, 255, 0.14);
  border-radius: 42rpx 42rpx 0 0;
  background:
    radial-gradient(circle at 70% 0%, rgba(105, 83, 227, 0.2), transparent 40%),
    #101227;
  box-shadow: 0 -30rpx 90rpx rgba(0, 0, 0, 0.48), inset 0 1rpx 0 rgba(255, 255, 255, 0.1);
  animation: sheetUp 0.35s cubic-bezier(0.22, 0.8, 0.31, 1) both;
}

.history-sheet {
  height: min(78vh, 980rpx);
  padding: 16rpx 24rpx calc(24rpx + env(safe-area-inset-bottom));
}

.sheet-handle {
  width: 90rpx;
  height: 8rpx;
  margin: 0 auto 24rpx;
  border-radius: 999rpx;
  background: rgba(255, 255, 255, 0.14);
}

.sheet-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4rpx 8rpx 24rpx;
}

.sheet-kicker {
  display: block;
  color: rgba(88, 232, 204, 0.46);
  font-size: 14rpx;
  letter-spacing: 4rpx;
}

.sheet-title {
  display: block;
  margin-top: 7rpx;
  font-size: 32rpx;
  font-weight: 900;
  letter-spacing: 2rpx;
}

.sheet-title-row button {
  position: relative;
  width: 58rpx;
  height: 58rpx;
  border-radius: 20rpx;
  background: rgba(255, 255, 255, 0.06);
}

.sheet-title-row button i::before,
.sheet-title-row button i::after {
  position: absolute;
  top: 27rpx;
  left: 17rpx;
  width: 25rpx;
  height: 3rpx;
  border-radius: 3rpx;
  background: rgba(255, 255, 255, 0.58);
  content: "";
}

.sheet-title-row button i::before { transform: rotate(45deg); }
.sheet-title-row button i::after { transform: rotate(-45deg); }

.history-list { height: calc(100% - 128rpx); }

.history-row {
  display: grid;
  min-height: 104rpx;
  grid-template-columns: 54rpx 180rpx minmax(0, 1fr);
  align-items: center;
  gap: 14rpx;
  margin-bottom: 12rpx;
  padding: 14rpx 16rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.07);
  border-radius: 24rpx;
  background: rgba(255, 255, 255, 0.035);
}

.history-index {
  display: flex;
  width: 45rpx;
  height: 45rpx;
  align-items: center;
  justify-content: center;
  border-radius: 15rpx;
  color: rgba(255, 225, 140, 0.74);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 17rpx;
  background: rgba(255, 213, 99, 0.09);
}

.history-info text:first-child {
  display: block;
  overflow: hidden;
  color: rgba(245, 248, 255, 0.77);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 17rpx;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.history-info text:last-child {
  display: block;
  margin-top: 7rpx;
  color: rgba(214, 222, 250, 0.35);
  font-size: 14rpx;
}

.history-results {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 7rpx;
}

.history-ball {
  width: 34rpx;
  height: 34rpx;
  font-size: 13rpx;
}

.rule-sheet {
  min-height: 500rpx;
  padding: 16rpx 52rpx calc(42rpx + env(safe-area-inset-bottom));
  text-align: center;
}

.rule-seal {
  display: flex;
  width: 110rpx;
  height: 110rpx;
  align-items: center;
  justify-content: center;
  margin: 10rpx auto 24rpx;
  border: 3rpx double rgba(255, 224, 133, 0.58);
  border-radius: 36rpx;
  background: linear-gradient(145deg, rgba(255, 214, 101, 0.17), rgba(111, 84, 224, 0.22));
  box-shadow: 0 0 34rpx rgba(255, 211, 102, 0.12);
  transform: rotate(45deg);
}

.rule-seal i {
  color: #ffe398;
  font-size: 32rpx;
  font-style: normal;
  font-weight: 900;
  transform: rotate(-45deg);
}

.rule-title {
  display: block;
  color: #fff7dc;
  font-size: 32rpx;
  font-weight: 900;
}

.rule-copy {
  display: block;
  margin-top: 22rpx;
  color: rgba(225, 229, 250, 0.56);
  font-size: 23rpx;
  line-height: 1.8;
}

.rule-close {
  display: flex;
  width: 100%;
  height: 86rpx;
  align-items: center;
  justify-content: center;
  margin-top: 34rpx;
  border-radius: 28rpx;
  color: #291e3d;
  font-size: 25rpx;
  font-weight: 900;
  background: linear-gradient(120deg, #ffe17e, #ff8c81);
}

.loading-scene,
.error-scene {
  position: relative;
  z-index: 2;
  display: flex;
  min-height: 72vh;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: rgba(230, 235, 255, 0.48);
  font-size: 22rpx;
  letter-spacing: 3rpx;
}

.loading-orbit {
  position: relative;
  width: 140rpx;
  height: 140rpx;
  margin-bottom: 30rpx;
  border: 2rpx solid rgba(92, 235, 210, 0.12);
  border-radius: 50%;
  animation: dialSpin 2.4s linear infinite;
}

.loading-orbit i {
  position: absolute;
  width: 25rpx;
  height: 25rpx;
  border-radius: 50%;
  background: #725de8;
  box-shadow: 0 0 18rpx rgba(114, 93, 232, 0.62);
}

.loading-orbit i:nth-child(1) { top: -10rpx; left: 56rpx; }
.loading-orbit i:nth-child(2) { right: 0; bottom: 8rpx; background: #ff5c82; }
.loading-orbit i:nth-child(3) { bottom: 4rpx; left: 4rpx; background: #3ee3c2; }

.broken-ball {
  display: flex;
  width: 104rpx;
  height: 104rpx;
  align-items: center;
  justify-content: center;
  margin-bottom: 26rpx;
  border: 5rpx double rgba(255, 120, 143, 0.6);
  border-radius: 50%;
  color: #ff788f;
  font-size: 45rpx;
  font-weight: 900;
  background: rgba(255, 91, 126, 0.08);
}

.error-scene button {
  display: flex;
  height: 70rpx;
  align-items: center;
  justify-content: center;
  margin-top: 28rpx;
  padding: 0 34rpx;
  border: 1rpx solid rgba(255, 222, 127, 0.36);
  border-radius: 24rpx;
  color: #ffe290;
  font-size: 22rpx;
  letter-spacing: 1rpx;
  background: rgba(255, 217, 105, 0.08);
}

@keyframes livePulse { 50% { opacity: 0.35; transform: scale(0.75); } }
@keyframes signal { from { opacity: 0.3; transform: scaleY(0.55); } to { opacity: 1; transform: scaleY(1); } }
@keyframes stageSheen { 0%, 60%, 100% { transform: translateX(-30%) rotate(-8deg); } 78% { transform: translateX(70%) rotate(-8deg); } }
@keyframes dialFloat { 50% { transform: translateX(-50%) translateY(-6rpx); } }
@keyframes urgentPulse { 50% { transform: translateX(-50%) scale(1.04); } }
@keyframes dialSpin { to { transform: rotate(360deg); } }
@keyframes revealBall { from { opacity: 0; transform: translateY(-45rpx) rotate(-30deg) scale(0.45); } 70% { opacity: 1; transform: translateY(7rpx) rotate(5deg) scale(1.06); } to { transform: none; } }
@keyframes shuffleBall { from { margin-top: -16rpx; transform: translateX(var(--shuffle-x)) rotate(-35deg) scale(0.82); } to { margin-top: 17rpx; transform: translateX(calc(var(--shuffle-x) * -0.75)) rotate(55deg) scale(1.08); } }
@keyframes optionGlow { 50% { opacity: 0.42; transform: translate(-50%, -58%) scale(1.18); } }
@keyframes chipDrop { from { opacity: 0; transform: translateY(-50rpx) rotate(-140deg) scale(1.45); } to { opacity: 1; transform: none; } }
@keyframes selectBurst { from { opacity: 0.8; transform: scale(0.6); } to { opacity: 0; transform: scale(4.5); } }
@keyframes buttonShine { 0%, 55% { transform: translateX(-120rpx) rotate(28deg); } 78%, 100% { transform: translateX(440rpx) rotate(28deg); } }
@keyframes leverPull { to { transform: translateY(8rpx); } }
@keyframes dockShake { 0%, 100% { transform: translateX(0); } 20% { transform: translateX(-11rpx); } 40% { transform: translateX(9rpx); } 60% { transform: translateX(-6rpx); } 80% { transform: translateX(4rpx); } }
@keyframes chipFlight { 0% { opacity: 0; transform: translate(0, 0) rotate(0) scale(0.6); } 15% { opacity: 1; } 100% { opacity: 0; transform: rotate(var(--flight-angle)) translateY(calc(var(--flight-distance) * -1)) rotate(540deg) scale(0.35); } }
@keyframes maskIn { from { opacity: 0; } }
@keyframes ticketIn { from { opacity: 0; transform: translateY(60rpx) scale(0.72) rotate(-4deg); } }
@keyframes sheetUp { from { opacity: 0; transform: translateY(100%); } }

@media (max-width: 370px) {
  .balance-pill { display: none; }
  .game-title { max-width: 250rpx; }
  .draw-stage { height: 430rpx; }
  .result-piece { width: 54rpx; height: 54rpx; flex-basis: 54rpx; }
  .result-track { gap: 7rpx; }
  .history-row { grid-template-columns: 48rpx 140rpx minmax(0, 1fr); }
}

@media (prefers-reduced-motion: reduce) {
  .lottery-page *,
  .lottery-page *::before,
  .lottery-page *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    scroll-behavior: auto !important;
  }
}
</style>
