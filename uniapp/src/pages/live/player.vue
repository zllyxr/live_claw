<template>
  <view class="live-room">
    <video v-if="playSrc" id="liveVideo" class="live-video" :src="playSrc" :poster="cover" autoplay :muted="isMuted" object-fit="cover" :controls="false" :show-center-play-btn="false" @canplay="onLiveVideoCanPlay" @error="onLiveVideoError"/>
    <view v-else class="live-fallback">
      <image class="fallback-cover" src="/static/art/live/stream-away-v1.webp" mode="aspectFill" />
      <view class="fallback-mask" />
      <view class="fallback-content">
        <text class="fallback-status">{{ resolvingStream ? "正在连接" : "直播暂停" }}</text>
        <text class="fallback-title">{{ resolvingStream ? "正在连接直播" : "主播暂时离开" }}</text>
        <text class="fallback-description">
          {{ resolvingStream ? "正在为你接入直播信号" : "暂时没有收到直播信号，稍后再来看看" }}
        </text>
        <button v-if="src && !resolvingStream" @tap="() => resolveRoomStream()">重试拉流</button>
      </view>
    </view>

    <view class="top-shade" />
    <view class="bottom-shade" />

    <view class="top-layer">
      <view class="anchor-pill" @tap="openAnchorHome">
        <view class="avatar-wrap">
          <image class="anchor-avatar" :src="anchorAvatar" mode="aspectFill" />
          <view class="anchor-level">播</view>
        </view>
        <view class="anchor-copy">
          <text class="anchor-name">{{ anchorName }}</text>
          <text class="anchor-id">ID: {{ liveUid || "0" }}</text>
        </view>
        <button class="follow-chip" :class="{ followed }" @tap.stop="followAnchor">
          {{ followed ? "已关" : "关注" }}
        </button>
      </view>

      <scroll-view scroll-x class="audience-strip" :show-scrollbar="false">
        <view class="audience-inner">
          <image
            v-for="item in audienceUsers"
            :key="userId(item)"
            class="audience-avatar"
            :src="userAvatar(item)"
            mode="aspectFill"
            @tap="openUser(item)"
          />
        </view>
      </scroll-view>

      <view class="room-tools">
        <view class="user-count" @tap="openPanel('users')">{{ onlineCount }}</view>
        <button class="sound-chip" :class="{ muted: isMuted }" @tap="toggleSound">{{ isMuted ? "静音" : "声音" }}</button>
        <image class="close-icon" src="/static/live/icon_live_close.png" mode="aspectFit" @tap="leaveRoom" />
      </view>
    </view>

    <view class="income-row">
      <view class="income-chip" @tap="openPanel('rank')">
        <text>星币</text>
        <text>{{ roomVotes }}</text>
        <image src="/static/live/icon_arrow_right.png" mode="aspectFit" />
      </view>
      <view class="income-chip" @tap="openPanel('guard')">
        <text>守护</text>
        <text>{{ guardCount }}</text>
        <image src="/static/live/icon_arrow_right.png" mode="aspectFit" />
      </view>
    </view>

    <view class="title-tip">
      <image src="/static/live/icon_live_title_laba.png" mode="aspectFit" />
      <text>{{ titleTip }}</text>
    </view>

    <view v-if="trialCountdown > 0" class="trial-chip">
      <text>试看 {{ trialCountdown }}s</text>
    </view>

    <view class="enter-room-tip">
      <image src="/static/live/icon_live_jin_guang.png" mode="scaleToFill" />
      <text>{{ enterTip }}</text>
    </view>

    <view v-if="giftToast" class="gift-toast">
      <image class="gift-toast-avatar" :src="avatarForGift" mode="aspectFill" />
      <view class="gift-toast-main">
        <text>{{ giftToast.name }}</text>
        <text>送出 {{ giftToast.gift }} x{{ giftToast.count }}</text>
      </view>
      <image class="gift-toast-icon" :src="giftToast.icon" mode="aspectFit" />
    </view>

    <view v-if="giftBurst" :key="giftBurst.id" class="gift-burst">
      <image
        v-for="item in giftBurstItems"
        :key="item.key"
        class="gift-float"
        :src="giftBurst.icon"
        mode="aspectFit"
        :style="{ left: item.left, '--gift-x': item.x, '--gift-y': item.y, animationDelay: item.delay }"
      />
      <view class="gift-burst-core">
        <image :src="giftBurst.icon" mode="aspectFit" />
        <text>{{ giftBurst.gift }}</text>
        <text>x{{ giftBurst.count }}</text>
      </view>
    </view>

    <scroll-view class="chat-board" scroll-y :scroll-top="chatScrollTop" :show-scrollbar="false">
      <view class="chat-list">
        <view v-for="message in messages" :key="message.id" class="chat-item" :class="message.type">
          <text v-if="message.type === 'system'" class="chat-system-text">{{ message.content }}</text>
          <view v-else class="chat-inline">
            <text v-if="message.level" class="chat-level">LV{{ message.level }}</text>
            <text v-if="message.vipType" class="chat-vip">VIP</text>
            <text v-if="message.guardType" class="chat-guard">守</text>
            <text v-if="message.badge" class="chat-badge">{{ message.badge }}</text>
            <text class="chat-name">{{ message.name }}</text>
            <text class="chat-colon">{{ message.type === "enter" ? "" : "：" }}</text>
            <text class="chat-text">{{ message.content }}</text>
          </view>
        </view>
      </view>
    </scroll-view>

    <view class="bottom-layer">
      <view v-if="chatActive" class="chat-composer">
        <input
          v-model.trim="draft"
          class="chat-input"
          placeholder="说点什么..."
          confirm-type="send"
          :focus="inputFocus"
          @confirm="sendChat"
          @blur="onChatBlur"
        />
        <button class="chat-send" :disabled="!draft" @tap="sendChat">发送</button>
      </view>
      <template v-else>
        <view class="chat-entry" @tap="focusChat">
          <text>说点什么...</text>
          <image src="/static/live/icon_live_chat_face.png" mode="aspectFit" />
        </view>
        <view class="bottom-actions">
          <image class="bottom-icon" src="/static/live/icon_live_game.png" mode="aspectFit" @tap="toggleGameChooser" />
          <image class="bottom-icon" src="/static/live/icon_live_first_charge.png" mode="aspectFit" @tap="openRecharge" />
          <image class="bottom-icon" src="/static/live/icon_live_gift.png" mode="aspectFit" @tap="openPanel('gift')" />
          <image class="bottom-icon optional" src="/static/live/icon_live_red_pack.png" mode="aspectFit" @tap="openRedPack" />
        </view>
        <view v-if="gameChooserOpen" class="live-game-popover">
          <view class="live-game-option" @tap="openLiveGame('sports')">
            <image src="/static/live/icon_live_game_sports.png" mode="aspectFit" />
            <text>体育</text>
          </view>
          <view class="live-game-option" @tap="openLiveGame('lottery')">
            <image src="/static/live/icon_live_game_lottery.png" mode="aspectFit" />
            <text>彩票</text>
          </view>
          <view class="game-popover-arrow" />
        </view>
      </template>
    </view>

    <view v-if="panel" class="sheet-mask" @tap="closePanel">
      <view class="bottom-sheet" :class="panel" @tap.stop>
        <view class="sheet-head">
          <text>{{ panelTitle }}</text>
          <p class="sheet-close" @tap="closePanel">关闭</p>
        </view>
        <view v-if="panel === 'gift'" class="gift-sheet">
          <view class="gift-tabs">
            <text class="active">热门</text>
            <text>礼物</text>
            <text>背包</text>
            <view class="gift-tip">幸运礼物说明 ›</view>
          </view>
          <scroll-view scroll-y class="gift-body" :show-scrollbar="false">
            <view class="gift-grid">
              <button v-for="gift in gifts" :key="String(gift.id)" class="gift-card" :class="{ active: String(selectedGift?.id || '') === String(gift.id || '') }" @tap="selectedGift = gift">
                <image class="gift-icon" :src="giftIcon(gift)" mode="aspectFit" />
                <text class="gift-name">{{ gift.giftname || gift.name || "礼物" }}</text>
                <text class="gift-price">{{ gift.needcoin || 0 }}星币</text>
              </button>
            </view>
            <view v-if="!gifts.length" class="sheet-empty">暂无礼物数据</view>
          </scroll-view>
          <view class="gift-footer">
            <view class="coin-charge" @tap="openRecharge">
              <image src="/static/live/icon_live_gift.png" mode="aspectFit" />
              <text>{{ giftBundle?.coin || "0" }}</text>
              <text class="charge-text">充值</text>
              <text class="charge-arrow">›</text>
            </view>
            <picker :range="giftCountOptions" @change="changeGiftCount">
              <view class="count-picker">{{ giftCount }}</view>
            </picker>
            <button class="send-gift" :disabled="sendingGift || !selectedGift" @tap="sendGift">赠送</button>
          </view>
        </view>
        <scroll-view v-else-if="panel === 'users'" scroll-y class="user-list sheet-body" :show-scrollbar="false">
          <view v-for="item in onlineSheetUsers" :key="userId(item)" class="user-row" @tap="openUser(item)">
            <image class="user-avatar" :src="userAvatar(item)" mode="aspectFill" />
            <view class="user-main">
              <text class="user-name">{{ userName(item) }}</text>
              <text class="user-id">ID: {{ userId(item) || "-" }} · 贡献 {{ userContribution(item) }}</text>
            </view>
            <button class="row-action" @tap.stop="chooseManageUser(item)">管理</button>
          </view>
          <view v-if="!onlineSheetUsers.length" class="sheet-empty">暂无在线用户</view>
        </scroll-view>
        <scroll-view v-else-if="panel === 'manage'" scroll-y class="manage-panel sheet-body" :show-scrollbar="false">
          <view v-if="selectedUser" class="selected-user" @tap="openUser(selectedUser)">
            <image class="user-avatar" :src="userAvatar(selectedUser)" mode="aspectFill" />
            <view class="user-main">
              <text class="user-name">{{ userName(selectedUser) }}</text>
              <text class="user-id">ID: {{ userId(selectedUser) || "-" }}</text>
            </view>
          </view>
          <view v-if="selectedUser" class="manage-actions">
            <button @tap="setAdmin(selectedUser)">房管</button>
            <button @tap="shutUp(selectedUser)">禁言</button>
            <button @tap="kick(selectedUser)">踢出</button>
            <button @tap="reportUser(selectedUser)">举报</button>
          </view>
          <text class="sheet-subtitle">房管列表</text>
          <view v-for="item in admins" :key="userId(item)" class="user-row" @tap="openUser(item)">
            <image class="user-avatar" :src="userAvatar(item)" mode="aspectFill" />
            <view class="user-main">
              <text class="user-name">{{ userName(item) }}</text>
              <text class="user-id">ID: {{ userId(item) || "-" }}</text>
            </view>
            <button class="row-action" @tap.stop="chooseManageUser(item)">管理</button>
          </view>
          <view v-if="!selectedUser && !admins.length" class="sheet-empty">请选择在线用户进行管理</view>
        </scroll-view>
        <scroll-view v-else-if="panel === 'guard'" scroll-y class="rank-list sheet-body" :show-scrollbar="false">
          <view v-for="item in guards" :key="rowKey(item)" class="rank-row">
            <image class="rank-avatar" :src="rowAvatar(item)" mode="aspectFill" />
            <view class="rank-main">
              <text class="rank-name">{{ rowName(item) }}</text>
              <text class="rank-desc">{{ guardDesc(item) }}</text>
            </view>
          </view>
          <view v-if="!guards.length" class="sheet-empty">暂无守护用户</view>
        </scroll-view>
        <scroll-view v-else-if="panel === 'rank'" scroll-y class="rank-list sheet-body" :show-scrollbar="false">
          <view v-for="item in ranks" :key="rowKey(item)" class="rank-row">
            <image class="rank-avatar" :src="rowAvatar(item)" mode="aspectFill" />
            <view class="rank-main">
              <text class="rank-name">{{ rowName(item) }}</text>
              <text class="rank-desc">{{ rowDesc(item) }}</text>
            </view>
          </view>
          <view v-if="!ranks.length" class="sheet-empty">暂无贡献记录</view>
        </scroll-view>
      </view>
    </view>

    <view v-if="liveGameKind" class="live-native-game-mask" @tap="closeLiveGamePanel">
      <view class="live-native-game-sheet" @tap.stop>
        <view class="live-native-game-grabber" />
        <view class="live-native-game-header">
          <button class="game-header-btn" :class="{ hidden: liveGameView === 'home' }" @tap="backLiveGameHome">返回</button>
          <view class="game-header-title">
            <text>{{ liveGameTitle }}</text>
            <text>直播不中断</text>
          </view>
          <button class="game-header-btn" @tap="closeLiveGamePanel">X</button>
        </view>

        <view v-if="liveGameLoading" class="live-native-game-loading">加载中...</view>
        <scroll-view v-else scroll-y class="live-native-game-body" :show-scrollbar="false">
          <view v-if="liveGameKind === 'sports' && liveGameView === 'home'" class="live-sports-panel">
            <scroll-view scroll-x class="live-game-tabs" :show-scrollbar="false">
              <view
                v-for="tab in liveSportsTabs"
                :key="tab.key"
                class="live-game-tab"
                :class="{ active: liveSportsTab === tab.key }"
                @tap="selectLiveSportsTab(tab.key)"
              >
                {{ tab.name }}
              </view>
            </scroll-view>
            <view class="live-panel-actions">
              <button @tap="openLiveSportsAllRecords">投注记录</button>
              <button @tap="loadLiveGamePanel('sports')">刷新赛事</button>
            </view>
            <view v-for="match in liveSportsMatches" :key="liveSportsMatchId(match)" class="live-sports-card">
              <view class="live-sports-league">
                <text>{{ match.competition_type || match.league_name || "足球赛事" }}</text>
                <text>{{ liveSportsStatus(match) }}</text>
              </view>
              <view class="live-sports-score">
                <text>{{ liveSportsTeamName(match, "home") }}</text>
                <text>{{ liveSportsScore(match) }}</text>
                <text>{{ liveSportsTeamName(match, "away") }}</text>
              </view>
              <view class="live-card-actions">
                <button class="live-game-primary" @tap="openLiveSportsBet(match)">下注</button>
                <button class="live-game-secondary" @tap="openLiveSportsRecords(match, 'home')">记录</button>
              </view>
            </view>
            <view v-if="!liveSportsMatches.length" class="live-native-game-empty">暂无赛事</view>
          </view>

          <view v-else-if="liveGameKind === 'lottery' && liveGameView === 'home'" class="live-lottery-panel">
            <view class="live-lottery-balance">
              <text>余额</text>
              <text>{{ liveLotteryHome?.coin || "0" }}星币</text>
            </view>
            <view class="live-panel-actions">
              <button @tap="openLiveLotteryAllRecords">投注记录</button>
              <button @tap="loadLiveGamePanel('lottery')">刷新余额</button>
            </view>
            <scroll-view scroll-x class="live-game-tabs" :show-scrollbar="false">
              <view
                v-for="category in liveLotteryCategories"
                :key="String(category.id)"
                class="live-game-tab"
                :class="{ active: liveLotteryCategory === String(category.id || '') }"
                @tap="selectLiveLotteryCategory(String(category.id || ''))"
              >
                {{ liveLotteryCategoryName(category) }}
              </view>
            </scroll-view>
            <view class="live-lottery-grid">
              <view v-for="game in liveLotteryGames" :key="String(game.id)" class="live-lottery-card" @tap="openLiveLotteryBet(game)">
                <image :src="liveLotteryGameIcon(game)" mode="aspectFill" />
                <view>
                  <text>{{ liveLotteryGameName(game) }}</text>
                  <text>{{ game.game_code || "LOTTERY" }}</text>
                </view>
                <button>下注</button>
              </view>
            </view>
            <view v-if="!liveLotteryGames.length" class="live-native-game-empty">暂无彩票</view>
          </view>

          <view v-else-if="liveGameKind === 'lottery' && liveGameView === 'lotteryBet'" class="live-bet-panel">
            <view class="live-bet-card">
              <text class="live-bet-title">{{ liveLotteryGameName(liveLotteryGame) }}</text>
              <text class="live-bet-sub">{{ liveLotteryIssueText() }}</text>
            </view>
            <view class="live-panel-actions">
              <button @tap="openLiveLotteryRecords(liveLotteryGame, 'lotteryBet')">投注记录</button>
            </view>
            <view v-for="play in liveLotteryPlays" :key="playName(play)" class="live-bet-section">
              <text class="live-bet-section-title">{{ playName(play) }}</text>
              <view class="live-bet-options">
                <button
                  v-for="option in playOptions(play)"
                  :key="optionId(option)"
                  class="live-bet-option"
                  :class="{ active: optionId(liveLotteryOption) === optionId(option) }"
                  @tap="liveLotteryOption = option"
                >
                  {{ optionName(option) }} @{{ optionOdds(option) }}
                </button>
              </view>
            </view>
            <view v-if="!liveLotteryPlays.length" class="live-native-game-empty">暂无玩法</view>
            <view class="live-bet-submit">
              <input v-model="liveLotteryAmount" type="number" placeholder="金额" />
              <button @tap="confirmLiveLotteryBet">确认投注</button>
            </view>
          </view>

          <view v-else-if="liveGameKind === 'sports' && liveGameView === 'sportsBet'" class="live-bet-panel">
            <view class="live-bet-card">
              <text class="live-bet-title">{{ liveSportsMatch ? `${liveSportsTeamName(liveSportsMatch, "home")} vs ${liveSportsTeamName(liveSportsMatch, "away")}` : "体育投注" }}</text>
              <text class="live-bet-sub">{{ liveSportsMatch ? `${liveSportsScore(liveSportsMatch)}  ${liveSportsStatus(liveSportsMatch)}` : "" }}</text>
            </view>
            <view class="live-panel-actions">
              <button @tap="openLiveSportsRecords(liveSportsMatch, 'sportsBet')">投注记录</button>
            </view>
            <view v-for="market in liveSportsMarketList" :key="marketName(market)" class="live-bet-section">
              <text class="live-bet-section-title">{{ marketName(market) }}</text>
              <view class="live-bet-options">
                <button
                  v-for="option in marketOptions(market)"
                  :key="optionId(option)"
                  class="live-bet-option sports"
                  :class="{ active: optionId(liveSportsOption) === optionId(option) }"
                  @tap="liveSportsOption = option"
                >
                  {{ optionName(option) }} @{{ optionOdds(option) }}
                </button>
              </view>
            </view>
            <view v-if="!liveSportsMarketList.length" class="live-native-game-empty">盘口未开放</view>
            <view class="live-bet-submit sports">
              <input v-model="liveSportsAmount" type="number" placeholder="金额" />
              <button @tap="confirmLiveSportsBet">确认投注</button>
            </view>
          </view>

          <view v-else-if="liveGameKind === 'lottery' && liveGameView === 'lotteryRecords'" class="live-record-panel">
            <view class="live-panel-actions">
              <button @tap="openLiveLotteryRecords(liveLotteryGame, liveRecordBackView)">刷新记录</button>
            </view>
            <view class="live-record-summary">
              <view>
                <text>下注</text>
                <text>{{ amountValue(liveLotteryRecords, ["total_bet"]) }}</text>
              </view>
              <view>
                <text>派彩</text>
                <text>{{ amountValue(liveLotteryRecords, ["total_payout"]) }}</text>
              </view>
              <view>
                <text>盈亏</text>
                <text>{{ amountValue(liveLotteryRecords, ["profit_loss", "net_amount"]) }}</text>
              </view>
            </view>
            <view v-for="order in liveLotteryOrderList" :key="recordKey(order)" class="live-record-card">
              <view class="live-record-head">
                <text>{{ lotteryRecordTitle(order) }}</text>
                <text>{{ recordStatus(order) }}</text>
              </view>
              <text class="live-record-meta">{{ lotteryOrderMeta(order) }}</text>
              <text v-if="recordTime(order)" class="live-record-meta">时间：{{ recordTime(order) }}</text>
              <view class="live-record-result">
                <view>
                  <text>开奖号码</text>
                  <text>{{ lotteryRecordOpenCode(order) }}</text>
                </view>
                <view>
                  <text>开奖状态</text>
                  <text>{{ lotteryRecordIssueStatus(order) }}</text>
                </view>
              </view>
              <view class="live-record-amounts">
                <view><text>下注</text><text>{{ amountValue(order, ["total_bet", "bet_money", "money"]) }}</text></view>
                <view><text>派彩</text><text>{{ amountValue(order, ["total_payout", "win_money"]) }}</text></view>
                <view><text>盈亏</text><text>{{ amountValue(order, ["profit_loss", "net_amount"]) }}</text></view>
              </view>
              <view v-if="recordItems(order).length" class="live-record-lines">
                <view v-for="item in recordItems(order)" :key="recordKey(item)" class="live-record-line">
                  <text>{{ lotteryRecordItemTitle(item) }}</text>
                  <text>{{ lotteryRecordItemMeta(item) }}</text>
                </view>
              </view>
            </view>
            <view v-if="!liveLotteryOrderList.length" class="live-native-game-empty">暂无投注记录</view>
          </view>

          <view v-else-if="liveGameKind === 'sports' && liveGameView === 'sportsRecords'" class="live-record-panel">
            <view class="live-panel-actions">
              <button @tap="openLiveSportsRecords(liveSportsMatch, liveRecordBackView)">刷新记录</button>
            </view>
            <view v-for="order in liveSportsOrderList" :key="recordKey(order)" class="live-record-card sports">
              <view class="live-record-head">
                <text>{{ sportsRecordTitle(order) }}</text>
                <text>{{ recordStatus(order) }}</text>
              </view>
              <text class="live-record-meta">{{ sportsOrderMeta(order) }}</text>
              <text v-if="recordTime(order)" class="live-record-meta">时间：{{ recordTime(order) }}</text>
              <view class="live-record-result sports">
                <view>
                  <text>赛果</text>
                  <text>{{ sportsRecordScore(order) }}</text>
                </view>
                <view>
                  <text>状态</text>
                  <text>{{ sportsRecordResultStatus(order) }}</text>
                </view>
              </view>
              <view class="live-record-amounts">
                <view><text>下注</text><text>{{ amountValue(order, ["total_bet", "bet_money", "money"]) }}</text></view>
                <view><text>派彩</text><text>{{ amountValue(order, ["total_payout", "win_money"]) }}</text></view>
                <view><text>盈亏</text><text>{{ amountValue(order, ["net_amount", "profit_loss"]) }}</text></view>
              </view>
              <view v-if="recordItems(order).length" class="live-record-lines">
                <view v-for="item in recordItems(order)" :key="recordKey(item)" class="live-record-line">
                  <text>{{ sportsRecordItemTitle(item) }}</text>
                  <text>{{ sportsRecordItemMeta(item) }}</text>
                </view>
              </view>
            </view>
            <view v-if="!liveSportsOrderList.length" class="live-native-game-empty">暂无体育投注记录</view>
          </view>
        </scroll-view>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref } from "vue";
import { onLoad } from "@dcloudio/uni-app";
import {
  enterLiveRoom,
  getGuardList,
  getLiveAdminList,
  getLiveGiftList,
  getLiveUserRank,
  getLiveUserListInfo,
  getLotteryBetRecords,
  getLotteryHome,
  getSportsBetMarkets,
  getSportsBetRecords,
  getSportsHome,
  kickLiveUser,
  reportLiveUser,
  resolveLiveSource,
  sendLiveGift,
  signOutWatchLive,
  setAttention,
  setLiveAdmin,
  shutUpLiveUser,
  submitLotteryBet,
  submitSportsBet
} from "@/api/services";
import type { LiveGift, LiveGiftBundle, LotteryGame, LotteryHome, SportsHome, SportsMatch, UserProfile } from "@/types/api";
import { getSession, isLoggedIn, requireLogin } from "@/utils/session";
import { displayUrl, staticAsset } from "@/utils/url";
import { deepDecode, isPlayableLiveUrl, resolveLiveStream } from "@/utils/liveStream";
import { LiveSocketClient, type LiveSocketChat, type LiveSocketGift } from "@/utils/liveSocket";

type Panel = "" | "gift" | "users" | "manage" | "guard" | "rank" | "function";
type FunctionKey = "users" | "manage" | "guard" | "rank" | "report" | "share" | "recharge";
type LiveGameKind = "sports" | "lottery";
type LiveGameView = "home" | "lotteryBet" | "sportsBet" | "lotteryRecords" | "sportsRecords";

const src = ref("");
const playSrc = ref("");
const resolvingStream = ref(false);
const streamError = ref("");
const cover = ref("");
const liveUid = ref("");
const stream = ref("");
const anchorName = ref("主播");
const BRAND_ICON = staticAsset("/static/brand/icon.webp");
const BRAND_ICON_ROUND = staticAsset("/static/brand/icon-round.webp");
const GAME_PLACEHOLDER = staticAsset("/static/brand/icon.webp");
const anchorAvatar = ref(BRAND_ICON_ROUND);
const roomVotes = ref("0");
const initialNums = ref("0");
let leavingRoom = false;
const followed = ref(false);
const isMuted = ref(true);
const panel = ref<Panel>("");
const users = ref<UserProfile[]>([]);
const admins = ref<UserProfile[]>([]);
const guards = ref<Record<string, unknown>[]>([]);
const ranks = ref<Record<string, unknown>[]>([]);
const giftBundle = ref<LiveGiftBundle>();
const selectedGift = ref<LiveGift>();
const selectedUser = ref<UserProfile>();
const giftCount = ref(1);
const sendingGift = ref(false);
const draft = ref("");
const chatActive = ref(false);
const inputFocus = ref(false);
const gameChooserOpen = ref(false);
const liveGameKind = ref<LiveGameKind | "">("");
const liveGameView = ref<LiveGameView>("home");
const liveRecordBackView = ref<LiveGameView>("home");
const liveGameLoading = ref(false);
const liveLotteryHome = ref<LotteryHome>();
const liveLotteryCategory = ref("");
const liveLotteryGame = ref<LotteryGame>();
const liveLotteryDetail = ref<Record<string, unknown>>();
const liveLotteryRecords = ref<Record<string, unknown>>();
const liveLotteryOption = ref<Record<string, unknown>>();
const liveLotteryAmount = ref("10");
const liveSportsHome = ref<SportsHome>();
const liveSportsTab = ref("today");
const liveSportsMatch = ref<SportsMatch>();
const liveSportsMarkets = ref<Record<string, unknown>>();
const liveSportsRecords = ref<Record<string, unknown>>();
const liveSportsOption = ref<Record<string, unknown>>();
const liveSportsAmount = ref("10");
const giftToast = ref<{ name: string; gift: string; count: number; icon: string }>();
const giftBurst = ref<{ id: number; gift: string; count: number; icon: string }>();
const messages = ref<LiveSocketChat[]>([]);
const chatScrollTop = ref(0);
const trialCountdown = ref(0);
const socketConnected = ref(false);
const socketUserType = ref(30);
const speakLimit = ref(0);
const myGuardType = ref(0);
let giftToastTimer: ReturnType<typeof setTimeout> | undefined;
let giftBurstTimer: ReturnType<typeof setTimeout> | undefined;
let trialTimer: ReturnType<typeof setInterval> | undefined;
let hlsPlayer: any;
let liveSocketClient: LiveSocketClient | undefined;
let directRefreshAttempts = 0;
let directHlsRecoveryAttempts = 0;
let directRefreshTimer: ReturnType<typeof setTimeout> | undefined;

const giftCountOptions = [1, 10, 66, 188, 520, 1314];
const gifts = computed(() => [...(giftBundle.value?.giftlist || []), ...(giftBundle.value?.proplist || [])]);
const onlineCount = computed(() => {
  const count = Math.max(users.value.length, Number(initialNums.value || 0));
  return count > 9999 ? "9999+" : String(count || 0);
});
const guardCount = computed(() => String(guards.value.length || 0));
const audienceUsers = computed<UserProfile[]>(() => {
  if (users.value.length) {
    return users.value.slice(0, 8);
  }
  return [
    {
      id: liveUid.value,
      user_nicename: anchorName.value,
      avatar: anchorAvatar.value || BRAND_ICON_ROUND
    }
  ];
});
const onlineSheetUsers = computed<UserProfile[]>(() => {
  return users.value;
});
const avatarForGift = computed(() => displayUrl(String(getSession().user?.avatar_thumb || getSession().user?.avatar || ""), "/static/brand/icon-round.webp"));
const giftBurstItems = computed(() =>
  Array.from({ length: 12 }, (_, index) => ({
    key: `${giftBurst.value?.id || 0}-${index}`,
    left: `${12 + ((index * 13) % 76)}%`,
    x: `${(index % 2 === 0 ? -1 : 1) * (48 + index * 9)}rpx`,
    y: `${-240 - ((index * 37) % 280)}rpx`,
    delay: `${index * 48}ms`
  }))
);
const enterTip = computed(() => `${anchorName.value} 的直播间`);
const titleTip = computed(() => "欢迎来到星域直播，关注主播不错过开播。");
const panelTitle = computed(() => {
  const titles: Record<Panel, string> = {
    "": "",
    gift: "礼物",
    users: "在线用户",
    manage: "房管与操作",
    guard: "守护",
    rank: "贡献榜",
    function: "更多功能"
  };
  return titles[panel.value];
});

const liveGameTitle = computed(() => {
  if (liveGameView.value === "sportsRecords") {
    return "体育投注记录";
  }
  if (liveGameView.value === "lotteryRecords") {
    return "彩票投注记录";
  }
  if (liveGameView.value === "sportsBet") {
    return "体育投注";
  }
  if (liveGameView.value === "lotteryBet") {
    return "彩票投注";
  }
  return liveGameKind.value === "sports" ? "体育" : "彩票";
});
const liveSportsTabs = computed(() => {
  const apiTabs = (liveSportsHome.value?.tabs || [])
    .map((item) => ({ key: String(item.key || ""), name: String(item.name || "") }))
    .filter((item) => item.key && item.name);
  return apiTabs.length
    ? apiTabs
    : [
        { key: "today", name: "今日" },
        { key: "tomorrow", name: "明日" },
        { key: "fixtures", name: "赛程" }
      ];
});
const liveSportsMatches = computed(() => liveSportsHome.value?.matches || liveSportsHome.value?.upcoming || []);
const liveLotteryCategories = computed(() => liveLotteryHome.value?.categories || []);
const liveLotteryGames = computed(() => {
  const games = liveLotteryHome.value?.games || [];
  if (!liveLotteryCategory.value) {
    return games;
  }
  return games.filter((game) => String(game.category_id || "") === liveLotteryCategory.value);
});
const liveLotteryPlays = computed(() => arrayValue(liveLotteryDetail.value, "plays"));
const liveSportsMarketList = computed(() => firstArray(liveSportsMarkets.value, ["markets", "items", "list"]));
const liveLotteryOrderList = computed(() => firstArray(liveLotteryRecords.value, ["items", "list", "orders"]));
const liveSportsOrderList = computed(() => firstArray(liveSportsRecords.value, ["items", "list", "orders"]));
function addSystem(content: string, badge = "系统") {
  messages.value.push({ id: `system-${Date.now()}-${messages.value.length}`, name: "系统", content, badge, type: "system" });
  scrollChat();
}

function pushChat(message: LiveSocketChat) {
  messages.value.push(message);
  if (messages.value.length > 120) {
    messages.value = messages.value.slice(-100);
  }
  scrollChat();
}

function asRecord(value: unknown) {
  return (value && typeof value === "object" ? value : {}) as Record<string, unknown>;
}

function arrayValue(source: unknown, key: string) {
  const value = asRecord(source)[key];
  return Array.isArray(value) ? (value as Record<string, unknown>[]) : [];
}

function firstArray(source: unknown, keys: string[]) {
  const record = asRecord(source);
  for (const key of keys) {
    const value = record[key];
    if (Array.isArray(value)) {
      return value as Record<string, unknown>[];
    }
  }
  return [];
}

function textValue(source: unknown, keys: string[], fallback = "") {
  const record = asRecord(source);
  for (const key of keys) {
    const value = record[key];
    if (value !== undefined && value !== null && value !== "") {
      return String(value);
    }
  }
  return fallback;
}

function numberValue(source: unknown, keys: string[], fallback = 0) {
  const value = Number(textValue(source, keys, ""));
  return Number.isFinite(value) ? value : fallback;
}

function applyLiveUserListInfo(info: unknown) {
  const record = asRecord(info);
  const list = firstArray(record, ["userlists", "userlist", "users", "list"]);
  if (list.length) {
    users.value = list as UserProfile[];
  }
  const nums = textValue(record, ["nums", "total", "count"], "");
  if (nums) {
    initialNums.value = nums;
  }
  const votes = textValue(record, ["votestotal", "votes"], "");
  if (votes) {
    roomVotes.value = votes;
  }
}

function mergeLiveUser(item: UserProfile) {
  const nextId = userId(item);
  if (!nextId) {
    return;
  }
  const index = users.value.findIndex((user) => userId(user) === nextId);
  if (index >= 0) {
    users.value.splice(index, 1, { ...users.value[index], ...item });
  } else {
    users.value.unshift(item);
    const nextCount = Math.max(Number(initialNums.value || 0), users.value.length);
    initialNums.value = String(nextCount);
  }
}

function mergeLiveUsers(list: UserProfile[]) {
  list.forEach(mergeLiveUser);
}

function removeLiveUser(uid: string) {
  if (!uid) {
    return;
  }
  users.value = users.value.filter((user) => userId(user) !== uid);
  const current = Number(initialNums.value || 0);
  if (current > 0) {
    initialNums.value = String(Math.max(0, current - 1));
  }
}

function handleSocketGift(event: LiveSocketGift) {
  if (event.votes) {
    roomVotes.value = event.votes;
  }
  pushChat(event.chat);
  showGiftToast(event.name, event.giftName, event.giftCount, event.giftIcon);
}

function disconnectLiveSocket() {
  liveSocketClient?.disconnect();
  liveSocketClient = undefined;
  socketConnected.value = false;
}

function connectLiveSocket(info: unknown) {
  socketUserType.value = numberValue(info, ["usertype"], 30);
  speakLimit.value = numberValue(info, ["speak_limit"], 0);
  const guardInfo = asRecord(asRecord(info).guard);
  myGuardType.value = Number(guardInfo.type || asRecord(info).guard_type || 0) || 0;
  if (!isLoggedIn() || !liveUid.value || !stream.value) {
    disconnectLiveSocket();
    return;
  }
  disconnectLiveSocket();
  liveSocketClient = new LiveSocketClient({
    liveUid: liveUid.value,
    stream: stream.value,
    userType: socketUserType.value,
    guardType: myGuardType.value,
    onConnect: (connected) => {
      socketConnected.value = connected;
    },
    onChat: pushChat,
    onGift: handleSocketGift,
    onEnter: ({ user, chat }) => {
      mergeLiveUser(user);
      pushChat(chat);
    },
    onLeave: removeLiveUser,
    onKick: (uid) => {
      if (uid && uid === getSession().uid) {
        uni.showToast({ title: "你已被踢出直播间", icon: "none" });
        leaveRoom();
        return;
      }
      removeLiveUser(uid);
    },
    onShutUp: (uid, content) => {
      if (uid && uid === getSession().uid) {
        uni.showToast({ title: content || "你已被禁言", icon: "none" });
      }
    },
    onSetAdmin: (uid, action) => {
      if (uid && uid === getSession().uid) {
        socketUserType.value = action === 1 ? 40 : 30;
      }
      if (panel.value === "manage") {
        void getLiveAdminList(liveUid.value).then((data) => {
          admins.value = data?.list || [];
        }).catch(() => undefined);
      }
    },
    onVotes: (votes) => {
      if (votes) {
        roomVotes.value = votes;
      }
    },
    onLiveEnd: (reason) => {
      addSystem(reason || "直播已结束");
    },
    onFakeFans: mergeLiveUsers,
    onError: (message) => {
      uni.showToast({ title: message, icon: "none" });
    }
  });
  liveSocketClient.connect();
}

async function refreshLiveUsers() {
  if (!liveUid.value || !stream.value) {
    return;
  }
  const info = await getLiveUserListInfo(liveUid.value, stream.value, 1);
  applyLiveUserListInfo(info);
}

function vipTypeOf(source: unknown) {
  const vip = asRecord(asRecord(source).vip);
  return textValue(vip, ["type"], "");
}

function shouldLimitTrial(enterInfo?: unknown) {
  if (!isLoggedIn()) {
    return true;
  }
  const type = vipTypeOf(enterInfo) || vipTypeOf(getSession().user);
  if (type === "") {
    return true;
  }
  return Number(type || 0) <= 0;
}

function clearTrialCountdown() {
  if (trialTimer) {
    clearInterval(trialTimer);
    trialTimer = undefined;
  }
  trialCountdown.value = 0;
}

function startTrialCountdown(seconds = 10) {
  clearTrialCountdown();
  trialCountdown.value = seconds;
  trialTimer = setInterval(() => {
    trialCountdown.value -= 1;
    if (trialCountdown.value <= 0) {
      clearTrialCountdown();
      destroyHlsPreview();
      playSrc.value = "";
      uni.showToast({ title: "试看已结束，请开通会员后继续观看", icon: "none" });
      setTimeout(() => leaveRoom(), 350);
    }
  }, 1000);
}

function configureTrial(enterInfo?: unknown) {
  if (shouldLimitTrial(enterInfo)) {
    startTrialCountdown(10);
    return;
  }
  clearTrialCountdown();
}

function liveLotteryCategoryName(category: Record<string, unknown>) {
  return textValue(category, ["name", "name_cn", "title"], "全部");
}

function liveLotteryGameName(game?: LotteryGame) {
  return String(game?.game_name || game?.game_name_en || game?.name || "彩票");
}

function liveLotteryGameIcon(game?: LotteryGame) {
  return displayUrl(String(game?.icon_url || game?.icon || ""), "/static/brand/icon.webp") || GAME_PLACEHOLDER;
}

function liveLotteryIssueText() {
  const issue = asRecord(liveLotteryDetail.value?.current_issue);
  if (!Object.keys(issue).length) {
    return "暂无可投注期号";
  }
  const issueNo = textValue(issue, ["issue_num", "issue", "id"], "--");
  const countdown = textValue(issue, ["bet_countdown", "countdown"], "0");
  const closed = textValue(issue, ["can_bet"], "1") === "1" ? "" : " 已封盘";
  return `期号：${issueNo}  倒计时：${countdown}${closed}`;
}

function liveLotteryIssueId() {
  const issue = asRecord(liveLotteryDetail.value?.current_issue);
  return textValue(issue, ["can_bet"], "1") === "1" ? textValue(issue, ["id"], "") : "";
}

function playName(play: Record<string, unknown>) {
  return textValue(play, ["play_name", "name", "title"], "玩法");
}

function playOptions(play: Record<string, unknown>) {
  return firstArray(play, ["options", "items", "list"]);
}

function optionName(option: Record<string, unknown>) {
  return textValue(option, ["option_name", "name", "title"], "选项");
}

function optionOdds(option: Record<string, unknown>) {
  return textValue(option, ["odds", "rate", "value"], "--");
}

function optionId(option?: Record<string, unknown>) {
  return textValue(option, ["id", "option_id"], "");
}

function liveSportsMatchId(match?: SportsMatch) {
  if (!match) {
    return "";
  }
  return String(match.match_id || match.public_match_id || match.id || match.source_match_id || "");
}

function liveSportsTeamName(match: SportsMatch, side: "home" | "away") {
  const direct = side === "home" ? match.home_name : match.away_name;
  const team = side === "home" ? match.home_team : match.away_team;
  if (direct) {
    return String(direct);
  }
  if (team && typeof team === "object") {
    return String(team.name || "球队");
  }
  return String(team || "球队");
}

function liveSportsScore(match: SportsMatch) {
  return `${match.home_score ?? "-"} : ${match.away_score ?? "-"}`;
}

function liveSportsStatus(match: SportsMatch) {
  return String(match.status_text || match.kickoff_text || match.kickoff_time_text || match.match_time || "赛程");
}

function marketName(market: Record<string, unknown>) {
  return textValue(market, ["market_name", "name", "market_code"], "盘口");
}

function marketOptions(market: Record<string, unknown>) {
  return firstArray(market, ["options", "items", "list"]);
}

function amountValue(source: unknown, keys: string[], fallback = "0") {
  return textValue(source, keys, fallback);
}

function lotteryRecordTitle(order: Record<string, unknown>) {
  return textValue(order, ["game_name", "game_name_en", "name"], "彩票订单");
}

function recordField(source: Record<string, unknown>, key: string) {
  const value = source[key];
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
}

function sportsRecordTitle(order: Record<string, unknown>) {
  const direct = textValue(order, ["match_title", "match_name"], "");
  if (direct) {
    return direct;
  }
  const match = recordField(order, "match");
  const home = textValue(match, ["home_name"], textValue(order, ["home_name"], "主队"));
  const away = textValue(match, ["away_name"], textValue(order, ["away_name"], "客队"));
  return `${home} VS ${away}`;
}

function recordStatus(order: Record<string, unknown>) {
  return textValue(order, ["status_text", "status_name", "status"], "--");
}

function lotteryOrderMeta(order: Record<string, unknown>) {
  const issue = textValue(order, ["issue_num", "issue"], "--");
  const no = textValue(order, ["order_no", "orderid", "id"], "--");
  return `期号：${issue}  订单：${no}`;
}

function sportsOrderMeta(order: Record<string, unknown>) {
  const matchId = textValue(order, ["display_match_id", "public_match_id", "match_id"], "--");
  const no = textValue(order, ["order_no", "orderid", "id"], "--");
  return `编号：${matchId}  订单：${no}`;
}

function recordTime(order: Record<string, unknown>) {
  return textValue(order, ["bet_time_text", "addtime", "datetime", "time"], "");
}

function recordItems(order: Record<string, unknown>) {
  const items = firstArray(order, ["items", "details"]).slice(0, 4);
  if (items.length) {
    return items;
  }
  const flat = textValue(order, ["option_name", "market_name", "bet_name", "play_name"], "");
  return flat ? [order] : [];
}

function recordKey(item: Record<string, unknown>) {
  return textValue(item, ["order_no", "orderid", "id", "option_id", "addtime"], JSON.stringify(item));
}

function lotteryRecordOpenCode(order: Record<string, unknown>) {
  return textValue(order, ["open_code", "award_code", "result_code"], "待开奖");
}

function lotteryRecordIssueStatus(order: Record<string, unknown>) {
  return textValue(order, ["issue_status_text", "issue_state_text"], recordStatus(order));
}

function lotteryRecordItemTitle(item: Record<string, unknown>) {
  const play = textValue(item, ["play_name", "play_code"], "");
  const option = textValue(item, ["option_name", "option_code"], "");
  return `${play || "玩法"} · ${option || "投注项"}`;
}

function lotteryRecordItemMeta(item: Record<string, unknown>) {
  const odds = optionOdds(item);
  const amount = amountValue(item, ["bet_amount", "amount", "money"]);
  const payout = amountValue(item, ["payout_amount", "win_money"], "-");
  const status = textValue(item, ["win_status_text", "status_text"], "待开奖");
  return `赔率 ${odds} · 投注 ${amount} · 派彩 ${payout} · ${status}`;
}

function sportsRecordScore(order: Record<string, unknown>) {
  const match = recordField(order, "match");
  const home = textValue(match, ["home_score"], textValue(order, ["home_score"], ""));
  const away = textValue(match, ["away_score"], textValue(order, ["away_score"], ""));
  if (home !== "" && away !== "") {
    return `${home} : ${away}`;
  }
  return "待同步";
}

function sportsRecordResultStatus(order: Record<string, unknown>) {
  const match = recordField(order, "match");
  const label = textValue(match, ["status_text"], textValue(order, ["match_status_text"], ""));
  if (label) {
    return label;
  }
  const settle = textValue(match, ["settle_status"], textValue(order, ["settle_status"], ""));
  if (settle === "1" || settle === "2") {
    return "已结算";
  }
  if (settle === "0") {
    return "待结算";
  }
  return "赛果";
}

function sportsRecordItemTitle(item: Record<string, unknown>) {
  const market = marketName(item);
  const option = optionName(item);
  return `${market} · ${option}`;
}

function sportsRecordItemMeta(item: Record<string, unknown>) {
  const odds = optionOdds(item);
  const amount = amountValue(item, ["bet_amount", "amount", "money"]);
  const payout = amountValue(item, ["payout_amount", "win_money"], "-");
  const status = textValue(item, ["win_status_text", "status_text"], "待结算");
  return `赔率 ${odds} · 投注 ${amount} · 派彩 ${payout} · ${status}`;
}

function destroyHlsPreview() {
  if (hlsPlayer?.destroy) {
    hlsPlayer.destroy();
  }
  hlsPlayer = undefined;
}

function h5VideoElement() {
  if (typeof document === "undefined") {
    return null;
  }
  const host = document.getElementById("liveVideo");
  return (
    host?.tagName.toLowerCase() === "video"
      ? host
      : host?.querySelector("video") || document.querySelector(".live-video video")
  ) as HTMLVideoElement | null;
}

function syncSoundState(video = h5VideoElement()) {
  if (video) {
    video.muted = isMuted.value;
    video.volume = isMuted.value ? 0 : 1;
  }
}

function playH5Video(video: HTMLVideoElement) {
  syncSoundState(video);
  void video.play().catch(() => {
    if (!isMuted.value) {
      uni.showToast({ title: "浏览器拦截自动播放声音，请点声音按钮", icon: "none" });
    }
  });
}

async function attachHlsPreview() {
  if (!/\.m3u8(\?|#|$)/i.test(playSrc.value)) {
    return;
  }
  if (typeof document === "undefined") {
    return;
  }
  await nextTick();
  const host = document.getElementById("liveVideo");
  const video = (
    host?.tagName.toLowerCase() === "video"
      ? host
      : host?.querySelector("video") || document.querySelector(".live-video video")
  ) as HTMLVideoElement | null;
  if (!video) {
    return;
  }
  syncSoundState(video);
  if (video.canPlayType("application/vnd.apple.mpegurl")) {
    video.src = playSrc.value;
    playH5Video(video);
    return;
  }
  // #ifdef H5
  try {
    const mod = await import("hls.js");
    const Hls = mod.default;
    if (!Hls.isSupported()) {
      return;
    }
    destroyHlsPreview();
    directHlsRecoveryAttempts = 0;
    hlsPlayer = new Hls({
      lowLatencyMode: true,
      liveSyncDurationCount: 3,
      liveMaxLatencyDurationCount: 10,
      backBufferLength: 30,
      maxBufferLength: 30,
      manifestLoadingMaxRetry: 4,
      manifestLoadingRetryDelay: 800,
      fragLoadingMaxRetry: 6,
      fragLoadingRetryDelay: 800
    });
    hlsPlayer.loadSource(playSrc.value);
    hlsPlayer.attachMedia(video);
    hlsPlayer.on(Hls.Events.MANIFEST_PARSED, () => {
      directRefreshAttempts = 0;
      directHlsRecoveryAttempts = 0;
      if (directRefreshTimer) {
        clearTimeout(directRefreshTimer);
        directRefreshTimer = undefined;
      }
      playH5Video(video);
    });
    hlsPlayer.on(Hls.Events.ERROR, (_event: unknown, data: { fatal?: boolean; type?: string }) => {
      if (!data?.fatal) {
        return;
      }
      if (data.type === Hls.ErrorTypes.NETWORK_ERROR && directHlsRecoveryAttempts < 2) {
        directHlsRecoveryAttempts += 1;
        hlsPlayer?.startLoad?.();
        return;
      }
      if (data.type === Hls.ErrorTypes.MEDIA_ERROR && directHlsRecoveryAttempts < 2) {
        directHlsRecoveryAttempts += 1;
        hlsPlayer?.recoverMediaError?.();
        return;
      }
      void retryDirectSource();
    });
  } catch {
    // App/native video does not need hls.js; H5 unsupported browsers will show fallback on error.
  }
  // #endif
}

async function resolveRoomStream(nextSource: unknown = src.value, forceRefresh = false) {
  const sourcePage = deepDecode(nextSource);
  src.value = sourcePage;
  playSrc.value = "";
  streamError.value = "";
  destroyHlsPreview();
  if (!sourcePage) {
    streamError.value = "直播地址暂不可用";
    return;
  }
  resolvingStream.value = true;
  try {
    let mediaSource = sourcePage;
    if (!isPlayableLiveUrl(mediaSource) && liveUid.value && stream.value) {
      const direct = await resolveLiveSource(liveUid.value, stream.value, forceRefresh);
      mediaSource = deepDecode(direct?.url || "");
      if (!mediaSource || direct?.delivery !== "direct") {
        throw new Error("没有取得可用的直播地址");
      }
    }
    const resolved = await resolveLiveStream(mediaSource);
    if (resolved.src) {
      playSrc.value = resolved.src;
      await attachHlsPreview();
      return;
    }
    streamError.value = resolved.reason || "未解析到可播放直播流";
  } catch (error: any) {
    streamError.value = error?.message || "直播流拉取失败";
  } finally {
    resolvingStream.value = false;
  }
}

async function retryDirectSource() {
  const isServerResolvedSource = Boolean(
    src.value &&
    !isPlayableLiveUrl(src.value) &&
    liveUid.value &&
    stream.value
  );
  if (!isServerResolvedSource || directRefreshAttempts >= 4) {
    streamError.value = "直播流播放失败，请重试";
    return;
  }
  if (resolvingStream.value || directRefreshTimer) {
    return;
  }
  directRefreshAttempts += 1;
  streamError.value = "直播地址正在更新…";
  const retryDelays = [500, 1500, 4000, 8000];
  directRefreshTimer = setTimeout(() => {
    directRefreshTimer = undefined;
    void resolveRoomStream(src.value, true);
  }, retryDelays[Math.min(directRefreshAttempts - 1, retryDelays.length - 1)]);
}

function onLiveVideoError() {
  void retryDirectSource();
}

function onLiveVideoCanPlay() {
  directRefreshAttempts = 0;
  directHlsRecoveryAttempts = 0;
  if (directRefreshTimer) {
    clearTimeout(directRefreshTimer);
    directRefreshTimer = undefined;
  }
}

function scrollChat() {
  setTimeout(() => {
    chatScrollTop.value += 9999;
  }, 60);
}

async function initRoom() {
  if (!liveUid.value || !stream.value) {
    return;
  }
  try {
    if (isLoggedIn()) {
      const enterInfo = await enterLiveRoom(liveUid.value, stream.value).catch(() => undefined);
      applyLiveUserListInfo(enterInfo);
      configureTrial(enterInfo);
      connectLiveSocket(enterInfo);
      const latestPull = textValue(enterInfo, ["source_page"], textValue(enterInfo, ["pull"], ""));
      if (latestPull && deepDecode(latestPull) !== src.value) {
        await resolveRoomStream(latestPull);
      }
    } else {
      configureTrial();
      disconnectLiveSocket();
    }
    await refreshLiveUsers().catch(() => undefined);
    void getGuardList(liveUid.value, 1).then((list) => {
      guards.value = list;
    }).catch(() => undefined);
  } catch (error: any) {
    uni.showToast({ title: error?.message || "直播间初始化失败", icon: "none" });
  }
}

async function openPanel(next: Panel) {
  gameChooserOpen.value = false;
  liveGameKind.value = "";
  panel.value = next;
  try {
    if (next === "gift" && !giftBundle.value) {
      giftBundle.value = await getLiveGiftList(0);
      selectedGift.value = gifts.value[0];
    }
    if (next === "users") {
      await refreshLiveUsers();
    }
    if (next === "manage") {
      if (!users.value.length) {
        await refreshLiveUsers().catch(() => undefined);
      }
      const data = await getLiveAdminList(liveUid.value);
      admins.value = data?.list || [];
    }
    if (next === "guard") {
      guards.value = await getGuardList(liveUid.value, 1);
    }
    if (next === "rank") {
      ranks.value = await getLiveUserRank(liveUid.value, stream.value);
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || "面板加载失败", icon: "none" });
  }
}

function closePanel() {
  panel.value = "";
}

function userId(item?: UserProfile) {
  return String(item?.id || item?.uid || "");
}

function userName(item?: UserProfile) {
  return item?.user_nicename || item?.user_nickname || "星域用户";
}

function userAvatar(item?: UserProfile) {
  return displayUrl(String(item?.avatar_thumb || item?.avatar || ""), "/static/brand/icon-round.webp");
}

function userContribution(item?: UserProfile) {
  return String(item?.contribution || item?.consumption || item?.votestotal || "0");
}

function giftIcon(item: LiveGift) {
  return displayUrl(String(item.gifticon || item.icon || ""), "/static/brand/icon-round.webp");
}

function rowKey(item: Record<string, unknown>) {
  return String(item.id || item.uid || item.touid || item.addtime || JSON.stringify(item));
}

function rowAvatar(item: Record<string, unknown>) {
  return displayUrl(String(item.avatar_thumb || item.avatar || item.user_avatar || ""), "/static/brand/icon-round.webp");
}

function rowName(item: Record<string, unknown>) {
  return String(item.user_nicename || item.user_nickname || item.name || item.uid || "星域用户");
}

function rowDesc(item: Record<string, unknown>) {
  return String(item.totalcoin || item.coin || item.contribute || item.endtime || item.addtime || "贡献记录");
}

function guardDesc(item: Record<string, unknown>) {
  const type = String(item.type || item.guard_type || "");
  const typeName = type === "2" ? "年守护" : type === "1" ? "月守护" : "守护用户";
  const end = String(item.endtime || item.end_time || item.addtime || "");
  return end ? `${typeName} · ${end}` : typeName;
}

function focusChat() {
  if (!requireLogin()) {
    return;
  }
  gameChooserOpen.value = false;
  liveGameKind.value = "";
  chatActive.value = true;
  setTimeout(() => {
    inputFocus.value = true;
  }, 80);
}

function onChatBlur() {
  inputFocus.value = false;
  setTimeout(() => {
    if (!draft.value) {
      chatActive.value = false;
    }
  }, 120);
}

function sendChat() {
  if (!requireLogin()) {
    return;
  }
  const content = draft.value.trim();
  if (!content) {
    return;
  }
  const level = Number(getSession().user?.level || 0);
  if (speakLimit.value > 0 && level < speakLimit.value) {
    uni.showToast({ title: `等级达到 ${speakLimit.value} 才能发言`, icon: "none" });
    return;
  }
  if (!liveSocketClient?.isConnected()) {
    uni.showToast({ title: "聊天服务器未连接，请稍后重试", icon: "none" });
    return;
  }
  liveSocketClient.sendChat(content);
  draft.value = "";
  chatActive.value = false;
}

async function followAnchor() {
  if (!requireLogin() || !liveUid.value) {
    return;
  }
  try {
    const res = await setAttention(liveUid.value);
    followed.value = Number(res?.isattent ?? (followed.value ? 0 : 1)) === 1;
    addSystem(followed.value ? "已关注主播" : "已取消关注主播");
  } catch (error: any) {
    uni.showToast({ title: error?.message || "关注失败", icon: "none" });
  }
}

function toggleSound() {
  isMuted.value = !isMuted.value;
  void nextTick(() => {
    const video = h5VideoElement();
    syncSoundState(video);
    if (video && !isMuted.value) {
      playH5Video(video);
    }
  });
  uni.showToast({ title: isMuted.value ? "已静音" : "已开启声音", icon: "none" });
}

function changeGiftCount(event: any) {
  giftCount.value = giftCountOptions[Number(event?.detail?.value || 0)] || 1;
}

async function sendGift() {
  if (!requireLogin() || !selectedGift.value || sendingGift.value) {
    return;
  }
  sendingGift.value = true;
  try {
    const result = await sendLiveGift({
      liveUid: liveUid.value,
      stream: stream.value,
      toUids: selectedUser.value ? userId(selectedUser.value) : liveUid.value,
      giftId: selectedGift.value.id || "",
      giftCount: giftCount.value || 1
    });
    if (result?.coin !== undefined && giftBundle.value) {
      giftBundle.value = { ...giftBundle.value, coin: result.coin as string | number };
    }
    const giftToken = textValue(result, ["gifttoken", "giftToken"], "");
    const name = getSession().user?.user_nicename || getSession().user?.user_nickname || "我";
    const giftName = String(selectedGift.value.giftname || selectedGift.value.name || "礼物");
    const icon = giftIcon(selectedGift.value);
    if (giftToken && liveSocketClient?.isConnected()) {
      liveSocketClient.sendGift(selectedGift.value, giftToken, anchorName.value);
    } else {
      addSystem(`${name} 送出了 ${giftName} x${giftCount.value || 1}`, "礼物");
      showGiftToast(name, giftName, giftCount.value || 1, icon);
      if (giftToken) {
        uni.showToast({ title: "礼物已送出，聊天广播未连接", icon: "none" });
      }
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || "送礼失败", icon: "none" });
  } finally {
    sendingGift.value = false;
  }
}

function showGiftToast(name: string, gift: string, count: number, icon: string) {
  giftToast.value = { name, gift, count, icon };
  giftBurst.value = { id: Date.now(), gift, count, icon };
  if (giftToastTimer) {
    clearTimeout(giftToastTimer);
  }
  if (giftBurstTimer) {
    clearTimeout(giftBurstTimer);
  }
  giftToastTimer = setTimeout(() => {
    giftToast.value = undefined;
  }, 2600);
  giftBurstTimer = setTimeout(() => {
    giftBurst.value = undefined;
  }, 2100);
}

function chooseManageUser(item: UserProfile) {
  selectedUser.value = item;
  void openPanel("manage");
}

function reportAnchor() {
  reportLiveTarget(liveUid.value);
}

function reportUser(item: UserProfile) {
  reportLiveTarget(userId(item));
}

function reportLiveTarget(toUid: string) {
  if (!requireLogin() || !toUid) {
    return;
  }
  const reasons = ["直播内容违规", "骚扰辱骂", "广告引流", "其他原因"];
  uni.showActionSheet({
    itemList: reasons,
    success: ({ tapIndex }) => {
      reportLiveUser(toUid, reasons[tapIndex] || "其他原因")
        .then(() => uni.showToast({ title: "已举报", icon: "none" }))
        .catch((error: any) => uni.showToast({ title: error?.message || "举报失败", icon: "none" }));
    }
  });
}

function setAdmin(item: UserProfile) {
  if (!requireLogin()) {
    return;
  }
  const targetId = userId(item);
  const action = admins.value.some((admin) => userId(admin) === targetId) ? 0 : 1;
  setLiveAdmin(liveUid.value, userId(item))
    .then(() => {
      if (liveSocketClient?.isConnected()) {
        liveSocketClient.sendSetAdmin(action, targetId, userName(item));
      } else {
        addSystem(`${userName(item)} 房管状态已更新`);
      }
      return openPanel("manage");
    })
    .catch((error: any) => uni.showToast({ title: error?.message || "设置失败", icon: "none" }));
}

function shutUp(item: UserProfile) {
  if (!requireLogin()) {
    return;
  }
  shutUpLiveUser(liveUid.value, stream.value, userId(item), 1)
    .then(() => {
      if (liveSocketClient?.isConnected()) {
        liveSocketClient.sendShutUp(userId(item), userName(item), 1);
      } else {
        addSystem(`${userName(item)} 已被禁言`);
      }
    })
    .catch((error: any) => uni.showToast({ title: error?.message || "禁言失败", icon: "none" }));
}

function kick(item: UserProfile) {
  if (!requireLogin()) {
    return;
  }
  uni.showModal({
    title: "踢出直播间",
    content: `确认踢出 ${userName(item)}？`,
    confirmColor: "#ff5878",
    success: ({ confirm }) => {
      if (!confirm) {
        return;
      }
      kickLiveUser(liveUid.value, userId(item))
        .then(() => {
          if (liveSocketClient?.isConnected()) {
            liveSocketClient.sendKick(userId(item), userName(item));
          } else {
            users.value = users.value.filter((user) => userId(user) !== userId(item));
            addSystem(`${userName(item)} 已被踢出直播间`);
          }
        })
        .catch((error: any) => uni.showToast({ title: error?.message || "踢人失败", icon: "none" }));
    }
  });
}

function runFunction(key: FunctionKey) {
  if (key === "report") {
    reportAnchor();
    return;
  }
  if (key === "share") {
    const shareUrl =
      `/pages/live/player?title=${encodeURIComponent(anchorName.value)}` +
      `&src=${encodeURIComponent(src.value)}&cover=${encodeURIComponent(cover.value)}` +
      `&liveuid=${encodeURIComponent(liveUid.value)}&stream=${encodeURIComponent(stream.value)}` +
      `&avatar=${encodeURIComponent(anchorAvatar.value)}&anchor=${encodeURIComponent(anchorName.value)}` +
      `&nums=${encodeURIComponent(initialNums.value)}&votes=${encodeURIComponent(roomVotes.value)}`;
    uni.setClipboardData({
      data: shareUrl,
      success: () => uni.showToast({ title: "直播间链接已复制", icon: "none" })
    });
    return;
  }
  if (key === "recharge") {
    openRecharge();
    return;
  }
  void openPanel(key);
}

function toggleGameChooser() {
  if (!requireLogin()) {
    return;
  }
  panel.value = "";
  liveGameKind.value = "";
  gameChooserOpen.value = !gameChooserOpen.value;
}

async function openLiveGame(kind: LiveGameKind) {
  if (!requireLogin()) {
    return;
  }
  gameChooserOpen.value = false;
  panel.value = "";
  liveGameKind.value = kind;
  liveGameView.value = "home";
  await loadLiveGamePanel(kind);
}

function closeLiveGamePanel() {
  liveGameKind.value = "";
  liveGameView.value = "home";
  liveRecordBackView.value = "home";
  liveLotteryGame.value = undefined;
  liveSportsMatch.value = undefined;
}

async function loadLiveGamePanel(kind = liveGameKind.value) {
  if (!kind) {
    return;
  }
  liveGameLoading.value = true;
  try {
    if (kind === "sports") {
      liveSportsHome.value = await getSportsHome(liveSportsTab.value);
      liveSportsTab.value = liveSportsHome.value?.selected_tab || liveSportsTab.value;
    } else {
      liveLotteryHome.value = await getLotteryHome();
      if (!liveLotteryCategory.value && liveLotteryHome.value?.categories?.[0]) {
        liveLotteryCategory.value = String(liveLotteryHome.value.categories[0].id || "");
      }
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || "加载失败", icon: "none" });
  } finally {
    liveGameLoading.value = false;
  }
}

async function selectLiveSportsTab(tab: string) {
  if (liveSportsTab.value === tab) {
    return;
  }
  liveSportsTab.value = tab;
  await loadLiveGamePanel("sports");
}

function selectLiveLotteryCategory(categoryId: string) {
  liveLotteryCategory.value = categoryId;
}

function openLiveLotteryBet(game: LotteryGame) {
  if (String(game.status || "1") !== "1") {
    uni.showToast({ title: "游戏维护中", icon: "none" });
    return;
  }
  const query = [
    `game_id=${encodeURIComponent(String(game.id || ""))}`,
    `game_code=${encodeURIComponent(String(game.game_code || ""))}`,
    `title=${encodeURIComponent(liveLotteryGameName(game))}`
  ].join("&");
  uni.navigateTo({ url: `/pages/game/bet?${query}` });
}

async function openLiveSportsBet(match: SportsMatch) {
  const matchId = liveSportsMatchId(match);
  if (!matchId) {
    uni.showToast({ title: "赛事ID缺失", icon: "none" });
    return;
  }
  liveGameLoading.value = true;
  liveSportsMatch.value = match;
  liveSportsOption.value = undefined;
  liveSportsAmount.value = "10";
  try {
    liveSportsMarkets.value = await getSportsBetMarkets(matchId);
    const firstOption = liveSportsMarketList.value.flatMap((market) => marketOptions(market))[0];
    liveSportsOption.value = firstOption;
    liveGameView.value = "sportsBet";
  } catch (error: any) {
    uni.showToast({ title: error?.message || "盘口加载失败", icon: "none" });
  } finally {
    liveGameLoading.value = false;
  }
}

function backLiveGameHome() {
  if (
    (liveGameView.value === "lotteryRecords" || liveGameView.value === "sportsRecords") &&
    liveRecordBackView.value !== "home"
  ) {
    liveGameView.value = liveRecordBackView.value;
    liveRecordBackView.value = "home";
    return;
  }
  liveGameView.value = "home";
  liveRecordBackView.value = "home";
  liveLotteryGame.value = undefined;
  liveLotteryDetail.value = undefined;
  liveLotteryRecords.value = undefined;
  liveLotteryOption.value = undefined;
  liveSportsMatch.value = undefined;
  liveSportsMarkets.value = undefined;
  liveSportsRecords.value = undefined;
  liveSportsOption.value = undefined;
}

async function openLiveLotteryRecords(game = liveLotteryGame.value, backView = liveGameView.value) {
  if (!requireLogin()) {
    return;
  }
  liveGameLoading.value = true;
  liveLotteryRecords.value = undefined;
  liveRecordBackView.value = backView === "lotteryBet" ? "lotteryBet" : "home";
  if (game) {
    liveLotteryGame.value = game;
  }
  try {
    liveLotteryRecords.value = await getLotteryBetRecords(game, 1);
    liveGameView.value = "lotteryRecords";
  } catch (error: any) {
    uni.showToast({ title: error?.message || "记录加载失败", icon: "none" });
  } finally {
    liveGameLoading.value = false;
  }
}

function openLiveLotteryAllRecords() {
  void openLiveLotteryRecords(undefined, "home");
}

async function openLiveSportsRecords(match = liveSportsMatch.value, backView = liveGameView.value) {
  if (!requireLogin()) {
    return;
  }
  liveGameLoading.value = true;
  liveSportsRecords.value = undefined;
  liveRecordBackView.value = backView === "sportsBet" ? "sportsBet" : "home";
  if (match) {
    liveSportsMatch.value = match;
  }
  try {
    liveSportsRecords.value = await getSportsBetRecords(match ? liveSportsMatchId(match) : "", 1);
    liveGameView.value = "sportsRecords";
  } catch (error: any) {
    uni.showToast({ title: error?.message || "记录加载失败", icon: "none" });
  } finally {
    liveGameLoading.value = false;
  }
}

function openLiveSportsAllRecords() {
  void openLiveSportsRecords(undefined, "home");
}

async function confirmLiveLotteryBet() {
  const game = liveLotteryGame.value;
  const option = liveLotteryOption.value;
  const amount = Number(liveLotteryAmount.value);
  const issueId = liveLotteryIssueId();
  if (!game || !option || !optionId(option)) {
    uni.showToast({ title: "请选择投注项", icon: "none" });
    return;
  }
  if (!issueId) {
    uni.showToast({ title: "暂无可投注期号", icon: "none" });
    return;
  }
  if (!amount || amount < 1) {
    uni.showToast({ title: "请输入正确金额", icon: "none" });
    return;
  }
  liveGameLoading.value = true;
  try {
    await submitLotteryBet({
      gameId: game.id || "",
      gameCode: String(game.game_code || ""),
      issueId,
      optionId: optionId(option),
      amount
    });
    uni.showToast({ title: "投注成功", icon: "none" });
    await openLiveLotteryRecords(game, "lotteryBet");
  } catch (error: any) {
    uni.showToast({ title: error?.message || "投注失败", icon: "none" });
  } finally {
    liveGameLoading.value = false;
  }
}

async function confirmLiveSportsBet() {
  const match = liveSportsMatch.value;
  const option = liveSportsOption.value;
  const amount = Number(liveSportsAmount.value);
  if (!match || !option || !optionId(option)) {
    uni.showToast({ title: "请选择投注项", icon: "none" });
    return;
  }
  if (!amount || amount < 1) {
    uni.showToast({ title: "请输入正确金额", icon: "none" });
    return;
  }
  liveGameLoading.value = true;
  try {
    await submitSportsBet({
      matchId: liveSportsMatchId(match),
      optionId: optionId(option),
      amount
    });
    uni.showToast({ title: "投注成功", icon: "none" });
    await openLiveSportsRecords(match, "sportsBet");
  } catch (error: any) {
    uni.showToast({ title: error?.message || "投注失败", icon: "none" });
  } finally {
    liveGameLoading.value = false;
  }
}

function openRecharge() {
  if (!requireLogin()) {
    return;
  }
  gameChooserOpen.value = false;
  liveGameKind.value = "";
  uni.navigateTo({ url: "/pages/wallet/recharge" });
}

function openRedPack() {
  if (!requireLogin()) {
    return;
  }
  gameChooserOpen.value = false;
  liveGameKind.value = "";
  panel.value = "";
  uni.navigateTo({ url: `/pages/redpack/index?stream=${encodeURIComponent(stream.value)}` });
}

function openUser(item: UserProfile) {
  const uid = userId(item);
  if (uid) {
    uni.navigateTo({ url: `/pages/user/home?uid=${encodeURIComponent(uid)}` });
  }
}

function openAnchorHome() {
  if (liveUid.value) {
    uni.navigateTo({ url: `/pages/user/home?uid=${encodeURIComponent(liveUid.value)}` });
  }
}

async function leaveRoom() {
  if (leavingRoom) {
    return;
  }
  leavingRoom = true;
  const client = liveSocketClient;
  liveSocketClient = undefined;
  socketConnected.value = false;
  const leaveTasks: Promise<unknown>[] = [];
  if (client) {
    leaveTasks.push(client.leave());
  }
  if (isLoggedIn() && stream.value) {
    leaveTasks.push(signOutWatchLive(liveUid.value, stream.value));
  }
  if (leaveTasks.length) {
    await Promise.race([
      Promise.allSettled(leaveTasks),
      new Promise((resolve) => setTimeout(resolve, 700))
    ]);
  }
  clearTrialCountdown();
  const pages = getCurrentPages();
  if (pages.length > 1) {
    uni.navigateBack();
    return;
  }
  uni.switchTab({ url: "/pages/tabbar/live/index" });
}

function h5HashQuery() {
  const locationLike = (globalThis as unknown as { location?: { hash?: string } }).location;
  const hash = locationLike?.hash || "";
  const queryText = hash.includes("?") ? hash.slice(hash.indexOf("?") + 1) : "";
  const params: Record<string, string> = {};
  queryText.split("&").forEach((pair) => {
    const [key, value = ""] = pair.split("=");
    if (key) {
      params[decodeURIComponent(key)] = value;
    }
  });
  return params;
}

onLoad((query) => {
  const fallback = h5HashQuery();
  const params = { ...fallback, ...(query || {}) } as Record<string, unknown>;
  src.value = deepDecode(params.src || "");
  cover.value = displayUrl(deepDecode(params.cover || ""));
  liveUid.value = deepDecode(params.liveuid || "");
  stream.value = deepDecode(params.stream || "");
  anchorName.value = deepDecode(params.anchor || params.title || "主播");
  anchorAvatar.value = displayUrl(deepDecode(params.avatar || ""), "/static/brand/icon-round.webp");
  roomVotes.value = deepDecode(params.votes || params.hotvotes || "0");
  initialNums.value = deepDecode(params.nums || "0");
  void resolveRoomStream();
  void initRoom();
});

onUnmounted(() => {
  if (liveSocketClient) {
    void liveSocketClient.leave();
    liveSocketClient = undefined;
  }
  if (!leavingRoom && isLoggedIn() && stream.value) {
    void signOutWatchLive(liveUid.value, stream.value);
  }
  destroyHlsPreview();
  clearTrialCountdown();
  if (directRefreshTimer) {
    clearTimeout(directRefreshTimer);
    directRefreshTimer = undefined;
  }
  if (giftToastTimer) {
    clearTimeout(giftToastTimer);
  }
  if (giftBurstTimer) {
    clearTimeout(giftBurstTimer);
  }
});
</script>

<style scoped>
.live-room {
  position: relative;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  color: #fff;
  background: #070709;
}

.live-video,
.live-fallback {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.live-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 28rpx;
}

.fallback-cover,
.fallback-mask {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.fallback-mask {
  background:
    linear-gradient(180deg, rgba(4, 7, 14, 0.2) 0%, rgba(4, 7, 14, 0.05) 38%, rgba(4, 7, 14, 0.74) 100%),
    linear-gradient(90deg, rgba(5, 8, 16, 0.2), transparent 58%);
}

.fallback-content {
  position: relative;
  z-index: 1;
  display: flex;
  width: 560rpx;
  max-width: calc(100vw - 96rpx);
  flex-direction: column;
  align-items: center;
  margin-top: -110rpx;
  text-align: center;
}

.fallback-status {
  display: inline-flex;
  height: 42rpx;
  align-items: center;
  padding: 0 18rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.28);
  border-radius: 22rpx;
  color: rgba(255, 255, 255, 0.78);
  font-size: 19rpx;
  font-weight: 600;
  letter-spacing: 2rpx;
  background: rgba(7, 10, 17, 0.38);
  backdrop-filter: blur(16rpx);
}

.fallback-title {
  margin-top: 24rpx;
  color: #fff;
  font-size: 42rpx;
  font-weight: 700;
  letter-spacing: 2rpx;
  text-shadow: 0 3rpx 18rpx rgba(0, 0, 0, 0.46);
}

.fallback-description {
  margin-top: 12rpx;
  color: rgba(255, 255, 255, 0.7);
  font-size: 23rpx;
  line-height: 1.6;
  text-shadow: 0 2rpx 12rpx rgba(0, 0, 0, 0.5);
}

.fallback-content button {
  display: flex;
  height: 64rpx;
  align-items: center;
  justify-content: center;
  margin-top: 28rpx;
  padding: 0 36rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.32);
  border-radius: 32rpx;
  color: #fff;
  font-size: 23rpx;
  font-weight: 700;
  background: rgba(8, 11, 18, 0.52);
  backdrop-filter: blur(18rpx);
}

.top-shade,
.bottom-shade {
  position: absolute;
  left: 0;
  right: 0;
  pointer-events: none;
}

.top-shade {
  top: 0;
  height: 260rpx;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.5), transparent);
}

.bottom-shade {
  bottom: 0;
  height: 520rpx;
  background: linear-gradient(0deg, rgba(0, 0, 0, 0.62), transparent);
}

.top-layer {
  position: absolute;
  top: calc(20rpx + var(--status-bar-height));
  left: 20rpx;
  right: 20rpx;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 10rpx;
  height: 72rpx;
}

.anchor-pill {
  display: flex;
  width: 278rpx;
  height: 72rpx;
  align-items: center;
  padding: 2rpx 6rpx 2rpx 2rpx;
  border-radius: 36rpx;
  background: rgba(0, 0, 0, 0.42);
}

.avatar-wrap {
  position: relative;
  width: 68rpx;
  height: 68rpx;
  flex: 0 0 auto;
}

.anchor-avatar {
  width: 68rpx;
  height: 68rpx;
  border-radius: 50%;
  background: #1c1f27;
}

.anchor-level {
  position: absolute;
  right: -2rpx;
  bottom: -1rpx;
  display: flex;
  width: 30rpx;
  height: 30rpx;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #fff;
  font-size: 16rpx;
  font-weight: 900;
  background: var(--brand);
}

.anchor-copy {
  flex: 1;
  min-width: 0;
  margin-left: 10rpx;
}

.anchor-name,
.anchor-id {
  display: block;
  max-width: 120rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.anchor-name {
  font-size: 27rpx;
  font-weight: 900;
  line-height: 1.05;
}

.anchor-id {
  margin-top: 8rpx;
  color: rgba(255, 255, 255, 0.82);
  font-size: 20rpx;
  line-height: 1;
}

.follow-chip {
  display: flex;
  width: 78rpx;
  height: 58rpx;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border-radius: 30rpx;
  color: #fff;
  font-size: 23rpx;
  font-weight: 900;
  background: var(--brand);
}

.follow-chip.followed {
  background: rgba(255, 255, 255, 0.18);
}

.audience-strip {
  height: 72rpx;
  white-space: nowrap;
}

.audience-inner {
  display: inline-flex;
  height: 72rpx;
  align-items: center;
  gap: 8rpx;
}

.audience-avatar {
  width: 54rpx;
  height: 54rpx;
  flex: 0 0 auto;
  border: 2rpx solid rgba(255, 255, 255, 0.55);
  border-radius: 50%;
  background: #222;
}

.room-tools {
  display: flex;
  align-items: center;
  gap: 10rpx;
  height: 72rpx;
}

.user-count {
  display: flex;
  min-width: 56rpx;
  height: 56rpx;
  align-items: center;
  justify-content: center;
  padding: 0 10rpx;
  border-radius: 28rpx;
  color: #fff;
  font-size: 22rpx;
  background: rgba(0, 0, 0, 0.42);
}

.sound-chip {
  display: flex;
  width: 72rpx;
  height: 56rpx;
  align-items: center;
  justify-content: center;
  border-radius: 28rpx;
  color: #fff;
  font-size: 21rpx;
  font-weight: 900;
  background: rgba(255, 88, 120, 0.82);
}

.sound-chip.muted {
  color: rgba(255, 255, 255, 0.76);
  background: rgba(0, 0, 0, 0.42);
}

.close-icon {
  width: 56rpx;
  height: 56rpx;
}

.income-row {
  position: absolute;
  top: calc(104rpx + var(--status-bar-height));
  left: 20rpx;
  display: flex;
  gap: 16rpx;
}

.income-chip {
  display: flex;
  height: 40rpx;
  align-items: center;
  padding: 0 12rpx 0 16rpx;
  border-radius: 20rpx;
  background: rgba(0, 0, 0, 0.4);
}

.income-chip text {
  font-size: 23rpx;
}

.income-chip text + text {
  margin-left: 10rpx;
  font-size: 21rpx;
}

.income-chip image {
  width: 18rpx;
  height: 18rpx;
  margin-left: 10rpx;
}

.title-tip {
  position: absolute;
  top: calc(154rpx + var(--status-bar-height));
  left: 20rpx;
  display: flex;
  max-width: 620rpx;
  height: 52rpx;
  align-items: center;
  padding: 0 18rpx;
  border-radius: 26rpx;
  background: linear-gradient(90deg, rgba(255, 88, 120, 0.78), rgba(34, 34, 44, 0.28));
}

.title-tip image {
  width: 34rpx;
  height: 34rpx;
  margin-right: 12rpx;
}

.title-tip text {
  min-width: 0;
  font-size: 24rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.trial-chip {
  position: absolute;
  top: calc(218rpx + var(--status-bar-height));
  right: 22rpx;
  display: flex;
  height: 52rpx;
  align-items: center;
  padding: 0 20rpx;
  border-radius: 26rpx;
  color: #fff;
  background: rgba(255, 88, 120, 0.86);
  box-shadow: 0 10rpx 26rpx rgba(0, 0, 0, 0.18);
}

.trial-chip text {
  font-size: 23rpx;
  font-weight: 900;
  white-space: nowrap;
}

.enter-room-tip {
  position: absolute;
  left: 20rpx;
  bottom: calc(478rpx + env(safe-area-inset-bottom));
  display: flex;
  width: 500rpx;
  max-width: calc(100vw - 40rpx);
  height: 65rpx;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.enter-room-tip image {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0.86;
}

.enter-room-tip text {
  position: relative;
  z-index: 1;
  width: 100%;
  box-sizing: border-box;
  padding: 0 10rpx;
  color: #fff4a5;
  font-size: 25rpx;
  font-weight: 900;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.gift-toast {
  position: absolute;
  left: 20rpx;
  bottom: calc(412rpx + env(safe-area-inset-bottom));
  display: flex;
  width: 520rpx;
  height: 108rpx;
  align-items: center;
  padding: 12rpx 16rpx;
  border-radius: 54rpx;
  background: linear-gradient(90deg, rgba(255, 88, 120, 0.82), rgba(20, 20, 28, 0.28));
  animation: giftToastIn 2600ms ease both;
  pointer-events: none;
}

.gift-toast-avatar {
  width: 72rpx;
  height: 72rpx;
  border: 2rpx solid #fff;
  border-radius: 50%;
}

.gift-toast-main {
  flex: 1;
  min-width: 0;
  margin-left: 14rpx;
}

.gift-toast-main text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.gift-toast-main text:first-child {
  font-size: 25rpx;
  font-weight: 900;
}

.gift-toast-main text:last-child {
  margin-top: 8rpx;
  color: #fff9b8;
  font-size: 23rpx;
}

.gift-toast-icon {
  width: 76rpx;
  height: 76rpx;
  animation: giftIconPulse 760ms ease-in-out infinite alternate;
}

.gift-burst {
  position: absolute;
  inset: 0;
  z-index: 8;
  pointer-events: none;
}

.gift-float {
  position: absolute;
  bottom: calc(310rpx + env(safe-area-inset-bottom));
  width: 58rpx;
  height: 58rpx;
  opacity: 0;
  filter: drop-shadow(0 8rpx 12rpx rgba(0, 0, 0, 0.32));
  animation: giftFloat 1550ms cubic-bezier(0.18, 0.82, 0.24, 1) both;
}

.gift-burst-core {
  position: absolute;
  left: 50%;
  top: 43%;
  display: flex;
  width: 240rpx;
  min-height: 260rpx;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  transform: translate(-50%, -50%);
  animation: giftCorePop 1900ms ease both;
}

.gift-burst-core image {
  width: 150rpx;
  height: 150rpx;
  filter: drop-shadow(0 12rpx 20rpx rgba(255, 88, 120, 0.38));
}

.gift-burst-core text {
  display: block;
  max-width: 240rpx;
  margin-top: 10rpx;
  color: #fff;
  font-size: 28rpx;
  font-weight: 900;
  text-align: center;
  text-shadow: 0 4rpx 12rpx rgba(0, 0, 0, 0.46);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.gift-burst-core text:last-child {
  margin-top: 0;
  color: #ffe66f;
  font-size: 52rpx;
  font-style: italic;
}

@keyframes giftToastIn {
  0% {
    opacity: 0;
    transform: translateX(-80rpx) scale(0.92);
  }
  12%,
  84% {
    opacity: 1;
    transform: translateX(0) scale(1);
  }
  100% {
    opacity: 0;
    transform: translateX(40rpx) scale(0.96);
  }
}

@keyframes giftIconPulse {
  from {
    transform: scale(1) rotate(-5deg);
  }
  to {
    transform: scale(1.12) rotate(6deg);
  }
}

@keyframes giftFloat {
  0% {
    opacity: 0;
    transform: translate3d(0, 0, 0) scale(0.5) rotate(-14deg);
  }
  16% {
    opacity: 1;
  }
  82% {
    opacity: 1;
  }
  100% {
    opacity: 0;
    transform: translate3d(var(--gift-x), var(--gift-y), 0) scale(1.2) rotate(18deg);
  }
}

@keyframes giftCorePop {
  0% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(0.35);
  }
  18% {
    opacity: 1;
    transform: translate(-50%, -50%) scale(1.12);
  }
  34% {
    transform: translate(-50%, -50%) scale(0.96);
  }
  76% {
    opacity: 1;
    transform: translate(-50%, -50%) scale(1);
  }
  100% {
    opacity: 0;
    transform: translate(-50%, -56%) scale(0.86);
  }
}

.chat-board {
  position: absolute;
  left: 20rpx;
  bottom: calc(116rpx + env(safe-area-inset-bottom));
  width: calc(100vw - 220rpx);
  height: 400rpx;
}

.chat-list {
  display: flex;
  min-height: 100%;
  flex-direction: column;
  justify-content: flex-end;
}

.chat-item {
  max-width: 100%;
  align-self: flex-start;
  margin-bottom: 10rpx;
  padding: 8rpx 12rpx;
  border-radius: 12rpx;
  background: rgba(0, 0, 0, 0.36);
}

.chat-item.system {
  background: rgba(255, 255, 255, 0.14);
}

.chat-inline {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  row-gap: 4rpx;
}

.chat-level,
.chat-vip,
.chat-guard,
.chat-badge {
  height: 24rpx;
  margin-right: 6rpx;
  padding: 0 7rpx;
  border-radius: 6rpx;
  color: #fff;
  font-size: 18rpx;
  font-weight: 800;
  line-height: 24rpx;
}

.chat-level {
  background: linear-gradient(90deg, #40d3ff, #7c5cff);
}

.chat-vip {
  background: linear-gradient(90deg, #ffb23d, var(--brand));
}

.chat-guard {
  background: linear-gradient(90deg, #6f7dff, #ff58ca);
}

.chat-name {
  margin-right: 2rpx;
  color: #fcd4df;
  font-size: 26rpx;
  font-weight: 900;
  line-height: 34rpx;
}

.chat-badge {
  background: rgba(255, 88, 120, 0.8);
}

.chat-colon {
  color: #fff;
  font-size: 25rpx;
  line-height: 34rpx;
}

.chat-text {
  color: #f1f1f1;
  font-size: 25rpx;
  line-height: 34rpx;
  word-break: break-word;
}

.chat-item.enter .chat-text {
  color: #fff6a8;
}

.chat-item.gift .chat-text {
  color: #ffe56f;
  font-weight: 900;
}

.chat-system-text {
  color: rgba(255, 255, 255, 0.9);
  font-size: 23rpx;
  line-height: 32rpx;
}

.bottom-layer {
  position: absolute;
  left: 10rpx;
  right: 10rpx;
  bottom: calc(10rpx + env(safe-area-inset-bottom));
  height: 90rpx;
}

.chat-entry,
.chat-composer {
  position: absolute;
  left: 0;
  top: 10rpx;
  display: flex;
  height: 60rpx;
  align-items: center;
  border-radius: 30rpx;
  background: rgba(0, 0, 0, 0.42);
}

.chat-entry {
  width: 232rpx;
  padding-left: 16rpx;
}

.chat-entry text {
  flex: 1;
  color: #fff;
  font-size: 25rpx;
}

.chat-entry image {
  width: 52rpx;
  height: 52rpx;
  margin-right: 4rpx;
}

.bottom-actions {
  position: absolute;
  right: 0;
  top: 5rpx;
  display: flex;
  align-items: center;
  gap: 4rpx;
}

.bottom-icon {
  width: 80rpx;
  height: 80rpx;
  padding: 10rpx;
}

.bottom-icon.optional {
  opacity: 0.92;
}

.live-game-popover {
  position: absolute;
  right: 272rpx;
  bottom: 96rpx;
  z-index: 20;
  display: flex;
  min-width: 226rpx;
  height: 130rpx;
  align-items: center;
  justify-content: center;
  padding: 0 18rpx;
  border-radius: 10rpx;
  background: rgba(0, 0, 0, 0.8);
}

.live-game-option {
  display: flex;
  width: 88rpx;
  height: 110rpx;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 22rpx;
}

.live-game-option + .live-game-option {
  margin-left: 20rpx;
}

.live-game-option image {
  width: 72rpx;
  height: 72rpx;
  margin-bottom: 8rpx;
}

.game-popover-arrow {
  position: absolute;
  left: 50%;
  bottom: -18rpx;
  width: 0;
  height: 0;
  margin-left: -18rpx;
  border-left: 18rpx solid transparent;
  border-right: 18rpx solid transparent;
  border-top: 20rpx solid rgba(0, 0, 0, 0.8);
}

.chat-composer {
  right: 0;
  padding: 0 8rpx 0 22rpx;
}

.chat-input {
  flex: 1;
  min-width: 0;
  height: 60rpx;
  color: #fff;
  font-size: 26rpx;
}

.chat-send {
  display: flex;
  width: 102rpx;
  height: 52rpx;
  align-items: center;
  justify-content: center;
  border-radius: 26rpx;
  color: #fff;
  font-size: 24rpx;
  font-weight: 900;
  background: var(--brand);
}

.sheet-mask {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: flex-end;
  background: rgba(0, 0, 0, 0.2);
}

.bottom-sheet {
  width: 100%;
  max-height: 72vh;
  overflow: hidden;
  padding-bottom: env(safe-area-inset-bottom);
  color: #fff;
  background: rgba(18, 16, 24, 0.96);
}

.bottom-sheet.gift {
  min-height: 520rpx;
  background: rgba(18, 16, 24, 0.98);
}

.bottom-sheet.users,
.bottom-sheet.manage,
.bottom-sheet.guard,
.bottom-sheet.rank {
  min-height: 520rpx;
}

.sheet-head {
  display: flex;
  height: 80rpx;
  align-items: center;
  justify-content: space-between;
  padding: 0 24rpx;
  border-bottom: 1rpx solid rgba(255, 255, 255, 0.08);
}

.sheet-head text {
  font-size: 30rpx;
  font-weight: 900;
}

.sheet-close {
  border-radius: 26rpx;
  color: rgba(255, 255, 255, 0.72);
  font-size: 20rpx;
  text-align: center;
  background: rgba(255, 255, 255, 0.08);
  margin: 5px 10px;
}

.sheet-scroll {
  max-height: calc(72vh - 80rpx - env(safe-area-inset-bottom));
  padding: 22rpx;
}

.sheet-body {
  height: calc(72vh - 80rpx - env(safe-area-inset-bottom));
  max-height: 560rpx;
  padding: 10rpx 24rpx 24rpx;
  box-sizing: border-box;
}

.gift-tabs {
  display: flex;
  height: 72rpx;
  align-items: center;
  padding: 0 20rpx;
}

.gift-tabs text {
  margin-right: 34rpx;
  color: rgba(255, 255, 255, 0.55);
  font-size: 26rpx;
  font-weight: 900;
}

.gift-tabs .active {
  color: #fff;
}

.gift-tip {
  margin-left: auto;
  color: rgba(255, 255, 255, 0.55);
  font-size: 22rpx;
}

.gift-body {
  height: 328rpx;
  margin: 0 14rpx;
  background-image: url("../../static/live/bg_gift_list.png");
  background-size: 100% 100%;
}

.gift-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 4rpx;
  padding: 10rpx;
}

.gift-card {
  min-height: 152rpx;
  padding: 10rpx 6rpx;
  border: 1rpx solid transparent;
  border-radius: 8rpx;
  color: #fff;
  background: none;
}

.gift-card.active {
  border-color: var(--brand);
  background: rgba(255, 88, 120, 0.14);
}

.gift-icon {
  display: block;
  width: 70rpx;
  height: 70rpx;
  margin: 0 auto 8rpx;
}

.gift-name,
.gift-price {
  display: block;
  color: #fff;
  font-size: 21rpx;
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.gift-price {
  margin-top: 6rpx;
  color: rgba(255, 255, 255, 0.58);
  font-size: 19rpx;
}

.gift-footer {
  display: flex;
  height: 92rpx;
  align-items: center;
  padding: 0 20rpx;
}

.coin-charge {
  display: flex;
  flex: 1;
  min-width: 0;
  align-items: center;
}

.coin-charge image {
  width: 38rpx;
  height: 38rpx;
  margin-right: 8rpx;
}

.coin-charge text {
  color: #fff;
  font-size: 26rpx;
}

.coin-charge .charge-text {
  margin-left: 8rpx;
  color: #32a0ff;
  font-size: 24rpx;
}

.charge-arrow {
  color: #32a0ff;
}

.count-picker,
.send-gift {
  display: flex;
  width: 116rpx;
  height: 60rpx;
  align-items: center;
  justify-content: center;
  margin-left: 12rpx;
  border-radius: 30rpx;
  color: var(--brand);
  font-size: 26rpx;
  background: rgba(255, 255, 255, 0.08);
}

.send-gift {
  color: #fff;
  background: var(--brand);
}

.function-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 22rpx 10rpx;
  padding: 16rpx 0 30rpx;
}

.function-item {
  display: flex;
  min-width: 0;
  flex-direction: column;
  align-items: center;
  color: #fff;background: none;
}

.function-item image {
  width: 70rpx;
  height: 70rpx;
  margin-bottom: 12rpx;
}

.function-item text {
  font-size: 24rpx;
}

.user-list,
.manage-panel,
.rank-list {
  padding-bottom: 24rpx;
}

.user-row,
.selected-user,
.rank-row {
  display: flex;
  align-items: center;
  min-height: 104rpx;
  padding: 14rpx 4rpx;
  border-bottom: 1rpx solid rgba(255, 255, 255, 0.08);
}

.user-avatar,
.rank-avatar {
  width: 68rpx;
  height: 68rpx;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.1);
}

.user-main,
.rank-main {
  flex: 1;
  min-width: 0;
  margin-left: 16rpx;
}

.user-name,
.rank-name {
  display: block;
  color: #fff;
  font-size: 27rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-id,
.rank-desc {
  display: block;
  margin-top: 8rpx;
  color: rgba(255, 255, 255, 0.5);
  font-size: 22rpx;
}

.row-action {
  display: flex;
  width: 92rpx;
  height: 52rpx;
  align-items: center;
  justify-content: center;
  border-radius: 26rpx;
  color: #fff;
  font-size: 23rpx;
  background: rgba(255, 88, 120, 0.85);
}

.manage-actions {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12rpx;
  padding: 22rpx 0;
}

.manage-actions button {
  display: flex;
  height: 66rpx;
  align-items: center;
  justify-content: center;
  border-radius: 33rpx;
  color: #fff;
  font-size: 23rpx;
  background: rgba(255, 255, 255, 0.1);
}

.manage-actions button[disabled] {
  opacity: 0.4;
}

.sheet-subtitle {
  margin-top: 8rpx;
  color: rgba(255, 255, 255, 0.72);
  font-size: 25rpx;
  font-weight: 900;
}

.sheet-empty {
  padding: 50rpx 20rpx;
  color: rgba(255, 255, 255, 0.5);
  font-size: 25rpx;
  text-align: center;
}

.live-native-game-mask {
  position: absolute;
  inset: 0;
  z-index: 40;
  display: flex;
  align-items: flex-end;
  background: transparent;
}

.live-native-game-sheet {
  display: flex;
  width: 100%;
  height: 50%;
  flex-direction: column;
  color: #fff;
  background: rgba(20, 24, 32, 0.5);
  border-radius: 28rpx 28rpx 0 0;
  box-shadow: 0 -18rpx 48rpx rgba(0, 0, 0, 0.28);
  backdrop-filter: blur(18rpx);
  overflow: hidden;
}

.live-native-game-grabber {
  width: 68rpx;
  height: 8rpx;
  flex: 0 0 auto;
  margin: 18rpx auto 4rpx;
  border-radius: 999rpx;
  background: rgba(255, 255, 255, 0.45);
}

.live-native-game-header {
  position: relative;
  display: flex;
  height: 78rpx;
  flex: 0 0 auto;
  align-items: center;
  justify-content: space-between;
  padding: 0 18rpx;
}

.game-header-title {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.game-header-title text:first-child {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 30rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.game-header-title text:last-child {
  margin-top: 4rpx;
  color: rgba(255, 255, 255, 0.62);
  font-size: 20rpx;
}

.game-header-btn {
  display: flex;
  width: 92rpx;
  height: 56rpx;
  align-items: center;
  justify-content: center;
  border-radius: 28rpx;
  color: #fff;
  font-size: 24rpx;
  font-weight: 900;
  background: rgba(255, 255, 255, 0.16);
}

.game-header-btn.hidden {
  opacity: 0;
  pointer-events: none;
}

.live-native-game-loading,
.live-native-game-empty {
  display: flex;
  min-height: 220rpx;
  align-items: center;
  justify-content: center;
  color: rgba(255, 255, 255, 0.72);
  font-size: 26rpx;
}

.live-native-game-body {
  flex: 0 0 auto;
  flex: 1;
  min-height: 0;
  padding: 8rpx 22rpx calc(22rpx + env(safe-area-inset-bottom));
}

.live-game-tabs {
  width: 100%;
  white-space: nowrap;
  margin-bottom: 16rpx;
}

.live-game-tab {
  display: inline-flex;
  min-width: 124rpx;
  height: 58rpx;
  align-items: center;
  justify-content: center;
  margin-right: 12rpx;
  padding: 0 22rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.24);
  border-radius: 29rpx;
  color: rgba(255, 255, 255, 0.82);
  font-size: 24rpx;
  font-weight: 900;
  background: rgba(255, 255, 255, 0.12);
}

.live-game-tab.active {
  border-color: rgba(255, 88, 120, 0.9);
  color: #fff;
  background: rgba(255, 88, 120, 0.72);
}

.live-panel-actions {
  display: flex;
  gap: 12rpx;
  margin-bottom: 16rpx;
}

.live-panel-actions button {
  display: flex;
  height: 58rpx;
  min-width: 150rpx;
  align-items: center;
  justify-content: center;
  padding: 0 22rpx;
  border-radius: 29rpx;
  color: #fff;
  font-size: 23rpx;
  font-weight: 900;
  background: rgba(255, 255, 255, 0.15);
}

.live-sports-card,
.live-lottery-card,
.live-bet-card,
.live-bet-section,
.live-record-card,
.live-record-summary {
  width: 100%;
  box-sizing: border-box;
  margin-bottom: 14rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.18);
  border-radius: 18rpx;
  background: rgba(0, 0, 0, 0.22);
}

.live-sports-card {
  padding: 18rpx;
}

.live-sports-league,
.live-sports-score {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.live-sports-league text {
  color: rgba(255, 255, 255, 0.68);
  font-size: 22rpx;
}

.live-sports-score {
  margin-top: 14rpx;
}

.live-sports-score text {
  width: 32%;
  color: #fff;
  font-size: 25rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.live-sports-score text:nth-child(2) {
  width: 26%;
  color: #36e193;
  font-size: 30rpx;
  text-align: center;
}

.live-sports-score text:last-child {
  text-align: right;
}

.live-card-actions {
  display: grid;
  grid-template-columns: 1fr 150rpx;
  gap: 12rpx;
  margin-top: 16rpx;
}

.live-game-primary,
.live-game-secondary {
  display: flex;
  height: 58rpx;
  align-items: center;
  justify-content: center;
  border-radius: 29rpx;
  color: #fff;
  font-size: 24rpx;
  font-weight: 900;
  background: linear-gradient(90deg, rgba(30, 199, 118, 0.9), rgba(56, 226, 150, 0.9));
}

.live-game-secondary {
  background: rgba(255, 255, 255, 0.15);
}

.live-lottery-balance {
  display: flex;
  height: 72rpx;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16rpx;
  padding: 0 20rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.2);
  border-radius: 18rpx;
  background: rgba(0, 0, 0, 0.22);
}

.live-lottery-balance text:last-child {
  color: #ffb2d3;
  font-weight: 900;
}

.live-lottery-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14rpx;
}

.live-lottery-card {
  display: flex;
  min-height: 126rpx;
  align-items: center;
  padding: 14rpx;
}

.live-lottery-card image {
  width: 72rpx;
  height: 72rpx;
  flex: 0 0 auto;
  border-radius: 18rpx;
}

.live-lottery-card view {
  flex: 1;
  min-width: 0;
  margin-left: 14rpx;
}

.live-lottery-card text {
  display: block;
  color: #fff;
  font-size: 24rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.live-lottery-card text:last-child {
  margin-top: 8rpx;
  color: rgba(255, 255, 255, 0.54);
  font-size: 20rpx;
}

.live-lottery-card button {
  display: flex;
  width: 72rpx;
  height: 46rpx;
  align-items: center;
  justify-content: center;
  border-radius: 23rpx;
  color: #fff;
  font-size: 21rpx;
  font-weight: 900;
  background: rgba(255, 88, 120, 0.82);
}

.live-bet-card,
.live-bet-section {
  padding: 18rpx;
}

.live-bet-title,
.live-bet-sub,
.live-bet-section-title {
  display: block;
}

.live-bet-title {
  color: #fff;
  font-size: 28rpx;
  font-weight: 900;
}

.live-bet-sub {
  margin-top: 8rpx;
  color: rgba(255, 255, 255, 0.66);
  font-size: 22rpx;
}

.live-bet-section-title {
  margin-bottom: 12rpx;
  color: rgba(255, 255, 255, 0.82);
  font-size: 24rpx;
  font-weight: 900;
}

.live-bet-options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12rpx;
}

.live-bet-option {
  display: flex;
  min-height: 62rpx;
  align-items: center;
  justify-content: center;
  border: 1rpx solid rgba(255, 255, 255, 0.2);
  border-radius: 12rpx;
  color: #fff;
  font-size: 22rpx;
  font-weight: 900;
  background: rgba(255, 255, 255, 0.12);
}

.live-bet-option.active {
  border-color: rgba(255, 88, 120, 0.95);
  background: rgba(255, 88, 120, 0.72);
}

.live-bet-option.sports.active {
  border-color: rgba(40, 209, 134, 0.95);
  background: rgba(40, 209, 134, 0.56);
}

.live-bet-submit {
  position: sticky;
  bottom: 0;
  display: flex;
  gap: 12rpx;
  padding: 12rpx 0 4rpx;
  background: rgba(20, 24, 32, 0.5);
}

.live-bet-submit input {
  flex: 1;
  height: 72rpx;
  min-width: 0;
  box-sizing: border-box;
  padding: 0 22rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.2);
  border-radius: 36rpx;
  color: #fff;
  font-size: 26rpx;
  background: rgba(0, 0, 0, 0.24);
}

.live-bet-submit button {
  display: flex;
  width: 190rpx;
  height: 72rpx;
  align-items: center;
  justify-content: center;
  border-radius: 36rpx;
  color: #fff;
  font-size: 25rpx;
  font-weight: 900;
  background: linear-gradient(90deg, rgba(255, 79, 163, 0.95), rgba(255, 142, 199, 0.95));
}

.live-bet-submit.sports button {
  background: linear-gradient(90deg, rgba(24, 189, 115, 0.95), rgba(50, 224, 146, 0.95));
}

.live-record-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  padding: 16rpx;
}

.live-record-summary view,
.live-record-amounts view {
  display: flex;
  min-width: 0;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.live-record-summary text:first-child,
.live-record-amounts text:first-child {
  color: rgba(255, 255, 255, 0.58);
  font-size: 20rpx;
}

.live-record-summary text:last-child,
.live-record-amounts text:last-child {
  margin-top: 6rpx;
  color: #ffb2d3;
  font-size: 25rpx;
  font-weight: 900;
}

.live-record-card {
  padding: 18rpx;
}

.live-record-card.sports .live-record-amounts text:last-child {
  color: #36e193;
}

.live-record-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16rpx;
  margin-bottom: 10rpx;
}

.live-record-head text:first-child {
  flex: 1;
  min-width: 0;
  color: #fff;
  font-size: 25rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.live-record-head text:last-child {
  flex: 0 0 auto;
  padding: 5rpx 12rpx;
  border-radius: 999rpx;
  color: #fff;
  font-size: 20rpx;
  background: rgba(40, 209, 134, 0.22);
}

.live-record-meta {
  display: block;
  color: rgba(255, 255, 255, 0.62);
  font-size: 21rpx;
  line-height: 34rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.live-record-result {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10rpx;
  margin-top: 12rpx;
  padding: 12rpx;
  border-radius: 14rpx;
  background: rgba(255, 88, 120, 0.13);
}

.live-record-result.sports {
  background: rgba(40, 209, 134, 0.13);
}

.live-record-result view {
  min-width: 0;
}

.live-record-result text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.live-record-result text:first-child {
  color: rgba(255, 255, 255, 0.5);
  font-size: 19rpx;
}

.live-record-result text:last-child {
  margin-top: 6rpx;
  color: #fff;
  font-size: 23rpx;
  font-weight: 900;
}

.live-record-amounts {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8rpx;
  margin: 14rpx 0 10rpx;
  padding: 12rpx 0;
  border-radius: 14rpx;
  background: rgba(0, 0, 0, 0.18);
}

.live-record-lines {
  display: grid;
  gap: 10rpx;
}

.live-record-line {
  min-width: 0;
  padding: 10rpx 12rpx;
  border-radius: 12rpx;
  background: rgba(255, 255, 255, 0.06);
}

.live-record-line text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.live-record-line text:first-child {
  color: rgba(255, 255, 255, 0.92);
  font-size: 22rpx;
  font-weight: 900;
}

.live-record-line text:last-child {
  margin-top: 6rpx;
  color: rgba(255, 255, 255, 0.6);
  font-size: 20rpx;
}
</style>
