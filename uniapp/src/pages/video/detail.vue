<template>
  <view class="video-detail-page">
    <video
      v-if="videoSrc"
      class="player"
      :src="videoSrc"
      :poster="cover"
      controls
      autoplay
      object-fit="contain"
    />
    <view v-else class="poster-wrap">
      <image :src="cover || '/static/brand/icon.webp'" mode="aspectFill" />
      <text>{{ loading ? "正在加载视频" : "暂无可播放视频" }}</text>
    </view>

    <view class="detail-card">
      <text class="title">{{ title }}</text>
      <text class="meta">{{ detail?.datetime || "" }} · {{ detail?.likes || 0 }} 赞 · {{ detail?.comments || 0 }} 评论</text>
      <view v-if="authorName" class="author" @tap="openAuthor">
        <image :src="authorAvatar" mode="aspectFill" />
        <text>{{ authorName }}</text>
        <text>›</text>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad } from "@dcloudio/uni-app";
import { getVideoDetail } from "@/api/services";
import type { VideoItem } from "@/types/api";
import { absolutizeUrl, firstText } from "@/utils/url";

const id = ref("");
const detail = ref<VideoItem>();
const loading = ref(false);

const title = computed(() => firstText(detail.value?.title, detail.value?.des, detail.value?.content, "星域视频"));
const videoSrc = computed(() => absolutizeUrl(firstText(detail.value?.href, detail.value?.video_url, detail.value?.video, detail.value?.url)));
const cover = computed(() => absolutizeUrl(firstText(detail.value?.video_thumb, detail.value?.thumb, detail.value?.thumb_s)));
const author = computed(() => detail.value?.userinfo || {});
const authorName = computed(() => firstText(author.value.user_nicename, author.value.user_nickname));
const authorAvatar = computed(() => absolutizeUrl(firstText(author.value.avatar_thumb, author.value.avatar)) || "/static/brand/icon-round.webp");
const authorUid = computed(() => firstText(author.value.id, author.value.uid, detail.value?.uid));

async function load() {
  if (!id.value) {
    return;
  }
  loading.value = true;
  try {
    detail.value = await getVideoDetail(id.value);
    uni.setNavigationBarTitle({ title: title.value });
  } catch (error: any) {
    uni.showToast({ title: error?.message || "视频加载失败", icon: "none" });
  } finally {
    loading.value = false;
  }
}

function openAuthor() {
  if (authorUid.value) {
    uni.navigateTo({ url: `/pages/user/home?uid=${encodeURIComponent(authorUid.value)}` });
  }
}

onLoad((query) => {
  id.value = String(query?.id || "");
  void load();
});
</script>

<style scoped>
.video-detail-page {
  min-height: 100vh;
  background: #0c0d12;
}

.player,
.poster-wrap {
  width: 100%;
  height: 58vh;
  background: #000;
}

.poster-wrap {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(255, 255, 255, 0.75);
  font-size: 26rpx;
}

.poster-wrap image {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0.45;
}

.poster-wrap text {
  position: relative;
  z-index: 1;
}

.detail-card {
  margin: -28rpx 24rpx 0;
  padding: 28rpx;
  border-radius: 24rpx 24rpx 0 0;
  background: #fff;
}

.title {
  display: block;
  color: var(--ink);
  font-size: 34rpx;
  font-weight: 900;
  line-height: 1.45;
}

.meta {
  display: block;
  margin-top: 16rpx;
  color: var(--ink-3);
  font-size: 24rpx;
}

.author {
  display: flex;
  align-items: center;
  gap: 16rpx;
  min-height: 92rpx;
  margin-top: 28rpx;
  padding-top: 20rpx;
  border-top: 1rpx solid #f0f2f6;
}

.author image {
  width: 64rpx;
  height: 64rpx;
  border-radius: 32rpx;
  background: var(--line);
}

.author text:nth-child(2) {
  flex: 1;
  min-width: 0;
  color: var(--ink);
  font-size: 27rpx;
  font-weight: 900;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.author text:last-child {
  color: #b8bfcc;
  font-size: 42rpx;
}
</style>
