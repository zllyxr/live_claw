<template>
  <view class="safe-page detail-page">
    <view v-if="detail" class="detail-card card">
      <view class="author-row">
        <image class="avatar" :src="avatarOf(detail)" mode="aspectFill" @tap="openUser(authorId(detail))" />
        <view class="author-main" @tap="openUser(authorId(detail))">
          <text class="name">{{ authorName(detail) }}</text>
          <text class="time">{{ detail.datetime || "" }}</text>
        </view>
        <button v-if="!isSelf(authorId(detail))" class="ghost-button mini" @tap="followAuthor">关注</button>
        <button class="more-button" @tap="openMore">•••</button>
      </view>

      <text v-if="detail.title" class="content">{{ detail.title }}</text>

      <view v-if="imageList(detail).length" class="image-grid">
        <image
          v-for="image in imageList(detail)"
          :key="image"
          class="feed-image"
          :src="image"
          mode="aspectFill"
          @tap="preview(image, imageList(detail))"
        />
      </view>

      <video
        v-if="detail.href"
        class="feed-video"
        :src="absolutizeUrl(String(detail.href))"
        :poster="absolutizeUrl(String(detail.video_thumb || ''))"
        controls
      />

      <view v-if="detail.voice" class="voice-row">
        <text>语音 {{ detail.length || 0 }}s</text>
      </view>

      <view class="meta-row">
        <button class="meta" @tap="like">{{ Number(detail.islike || 0) ? "已赞" : "点赞" }} {{ detail.likes || 0 }}</button>
        <text class="meta-text">评论 {{ commentTotal }}</text>
      </view>
    </view>

    <view class="comment-box card">
      <textarea
        v-model="draft"
        class="comment-input"
        :placeholder="replyTarget ? `回复 ${commentName(replyTarget)}` : '说点什么...'"
        maxlength="300"
        auto-height
      />
      <view class="comment-actions">
        <button v-if="replyTarget" class="ghost-button" @tap="clearReply">取消回复</button>
        <button class="pill-button" :disabled="submitting" @tap="submitComment">发送</button>
      </view>
    </view>

    <view class="section-title">评论</view>
    <view v-if="comments.length" class="comments">
      <view v-for="comment in comments" :key="String(comment.id || comment.commentid)" class="comment-card card">
        <image class="comment-avatar" :src="commentAvatar(comment)" mode="aspectFill" @tap="openUser(commentUid(comment))" />
        <view class="comment-body">
          <view class="comment-head">
            <text class="comment-name">{{ commentName(comment) }}</text>
            <text class="comment-time">{{ comment.datetime || "" }}</text>
          </view>
          <text class="comment-content">{{ comment.content || "" }}</text>
          <view class="comment-tools">
            <button @tap="replyTo(comment)">回复</button>
            <button v-if="canDeleteComment(comment)" @tap="confirmDeleteComment(comment)">删除</button>
          </view>
          <view v-if="comment.replylist?.length" class="reply-box">
            <view v-for="reply in comment.replylist" :key="String(reply.id || reply.commentid)" class="reply-row">
              <text class="reply-name">{{ commentName(reply) }}</text>
              <text class="reply-text">{{ reply.content || "" }}</text>
              <button v-if="canDeleteComment(reply)" class="reply-delete" @tap="confirmDeleteComment(reply)">删除</button>
            </view>
          </view>
          <button v-if="Number(comment.replys || 0) > 0" class="load-reply" @tap="loadReplies(comment)">
            查看全部 {{ comment.replys }} 条回复
          </button>
        </view>
      </view>
    </view>
    <EmptyState v-else :title="loading ? '正在加载评论' : '暂无评论'" description="成为第一个评论的人。" />
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { onLoad, onPullDownRefresh, onReachBottom } from "@dcloudio/uni-app";
import EmptyState from "@/components/EmptyState.vue";
import {
  commentDynamic,
  deleteDynamic,
  deleteDynamicComment,
  getDynamicComments,
  getDynamicDetail,
  getDynamicReplies,
  likeDynamic,
  reportDynamic,
  setAttention
} from "@/api/services";
import type { DynamicComment, DynamicItem } from "@/types/api";
import { displayCount } from "@/utils/format";
import { requireLogin, getSession } from "@/utils/session";
import { absolutizeUrl } from "@/utils/url";

const dynamicId = ref("");
const detail = ref<DynamicItem>();
const comments = ref<DynamicComment[]>([]);
const commentTotal = ref<string | number>(0);
const page = ref(1);
const loading = ref(false);
const finished = ref(false);
const submitting = ref(false);
const draft = ref("");
const replyTarget = ref<DynamicComment>();

function idOf(item?: DynamicItem) {
  return String(item?.id || item?.dynamicid || dynamicId.value || "");
}

function authorId(item?: DynamicItem) {
  return String(item?.uid || item?.userinfo?.id || item?.userinfo?.uid || "");
}

function authorName(item?: DynamicItem) {
  return item?.userinfo?.user_nicename || item?.userinfo?.user_nickname || "星域用户";
}

function isSelf(uid: string | number | undefined) {
  const session = getSession();
  return Boolean(uid && String(uid) === String(session.uid));
}

function avatarOf(item: DynamicItem) {
  return absolutizeUrl(String(item.userinfo?.avatar_thumb || item.userinfo?.avatar || "")) || "/static/brand/icon-round.webp";
}

function imageList(item: DynamicItem) {
  return String(item.thumb || "")
    .split(";")
    .map((image) => absolutizeUrl(image))
    .filter(Boolean);
}

function commentUid(comment: DynamicComment) {
  return String(comment.uid || comment.userinfo?.id || comment.userinfo?.uid || "");
}

function commentName(comment: DynamicComment) {
  return comment.userinfo?.user_nicename || comment.userinfo?.user_nickname || "星域用户";
}

function commentAvatar(comment: DynamicComment) {
  return absolutizeUrl(String(comment.userinfo?.avatar_thumb || comment.userinfo?.avatar || "")) || "/static/brand/icon-round.webp";
}

function commentId(comment: DynamicComment) {
  return String(comment.id || comment.commentid || "");
}

function canDeleteComment(comment: DynamicComment) {
  return isSelf(commentUid(comment)) || isSelf(authorId(detail.value));
}

async function loadDetail() {
  if (!dynamicId.value) {
    return;
  }
  detail.value = await getDynamicDetail(dynamicId.value);
}

async function loadComments(reset = false) {
  if (!dynamicId.value || loading.value || (finished.value && !reset)) {
    return;
  }
  loading.value = true;
  if (reset) {
    page.value = 1;
    finished.value = false;
  }
  try {
    const bundle = await getDynamicComments(dynamicId.value, page.value);
    const list = bundle?.commentlist || [];
    commentTotal.value = bundle?.comments ?? list.length;
    comments.value = reset ? list : comments.value.concat(list);
    if (!list.length) {
      finished.value = true;
    } else {
      page.value += 1;
    }
  } catch (error: any) {
    uni.showToast({ title: error?.message || "评论加载失败", icon: "none" });
  } finally {
    loading.value = false;
    uni.stopPullDownRefresh();
  }
}

async function reload() {
  try {
    await Promise.all([loadDetail(), loadComments(true)]);
  } catch (error: any) {
    uni.showToast({ title: error?.message || "动态加载失败", icon: "none" });
  } finally {
    uni.stopPullDownRefresh();
  }
}

async function like() {
  if (!detail.value || !requireLogin()) {
    return;
  }
  try {
    const res = await likeDynamic(idOf(detail.value));
    detail.value.islike = (res?.islike as string | number | undefined) ?? 1;
    detail.value.likes = (res?.likes as string | number | undefined) ?? Number(detail.value.likes || 0) + 1;
  } catch (error: any) {
    uni.showToast({ title: error?.message || "操作失败", icon: "none" });
  }
}

async function followAuthor() {
  if (!detail.value || !requireLogin()) {
    return;
  }
  try {
    const res = await setAttention(authorId(detail.value));
    detail.value.isattent = res?.isattent ?? 1;
    uni.showToast({ title: Number(res?.isattent || 1) ? "已关注" : "已取消关注", icon: "none" });
  } catch (error: any) {
    uni.showToast({ title: error?.message || "关注失败", icon: "none" });
  }
}

function submitComment() {
  if (!requireLogin()) {
    return;
  }
  const content = draft.value.trim();
  if (!content || submitting.value) {
    uni.showToast({ title: "请输入评论内容", icon: "none" });
    return;
  }
  submitting.value = true;
  const target = replyTarget.value;
  commentDynamic({
    dynamicId: dynamicId.value,
    content,
    toUid: target ? commentUid(target) : authorId(detail.value),
    commentId: target ? commentId(target) : "",
    parentId: target ? commentId(target) : ""
  })
    .then(() => {
      draft.value = "";
      replyTarget.value = undefined;
      uni.showToast({ title: "已发送", icon: "none" });
      return loadComments(true);
    })
    .catch((error: any) => {
      uni.showToast({ title: error?.message || "发送失败", icon: "none" });
    })
    .finally(() => {
      submitting.value = false;
    });
}

function replyTo(comment: DynamicComment) {
  if (requireLogin()) {
    replyTarget.value = comment;
  }
}

function clearReply() {
  replyTarget.value = undefined;
}

async function loadReplies(comment: DynamicComment) {
  if (!requireLogin()) {
    return;
  }
  try {
    comment.replylist = await getDynamicReplies(commentId(comment), 1);
  } catch (error: any) {
    uni.showToast({ title: error?.message || "回复加载失败", icon: "none" });
  }
}

function openMore() {
  const own = isSelf(authorId(detail.value));
  const itemList = own ? ["删除动态", "举报动态"] : ["举报动态"];
  uni.showActionSheet({
    itemList,
    success: ({ tapIndex }) => {
      const action = itemList[tapIndex];
      if (action === "删除动态") {
        confirmDeleteDynamic();
      } else {
        report();
      }
    }
  });
}

function confirmDeleteDynamic() {
  uni.showModal({
    title: "删除动态",
    content: "删除后不可恢复，确认删除这条动态？",
    confirmColor: "#ff5878",
    success: ({ confirm }) => {
      if (!confirm || !detail.value || !requireLogin()) {
        return;
      }
      deleteDynamic(idOf(detail.value))
        .then(() => {
          uni.showToast({ title: "已删除", icon: "none" });
          setTimeout(() => uni.navigateBack(), 300);
        })
        .catch((error: any) => {
          uni.showToast({ title: error?.message || "删除失败", icon: "none" });
        });
    }
  });
}

function report() {
  if (!requireLogin()) {
    return;
  }
  const reasons = ["内容违规", "垃圾广告", "骚扰或辱骂", "其他原因"];
  uni.showActionSheet({
    itemList: reasons,
    success: ({ tapIndex }) => {
      reportDynamic(dynamicId.value, reasons[tapIndex] || "其他原因")
        .then(() => {
          uni.showToast({ title: "已举报", icon: "none" });
        })
        .catch((error: any) => {
          uni.showToast({ title: error?.message || "举报失败", icon: "none" });
        });
    }
  });
}

function confirmDeleteComment(comment: DynamicComment) {
  uni.showModal({
    title: "删除评论",
    content: "确认删除这条评论？",
    confirmColor: "#ff5878",
    success: ({ confirm }) => {
      if (!confirm || !requireLogin()) {
        return;
      }
      deleteDynamicComment(dynamicId.value, commentId(comment), commentUid(comment))
        .then(() => {
          uni.showToast({ title: "已删除", icon: "none" });
          return loadComments(true);
        })
        .catch((error: any) => {
          uni.showToast({ title: error?.message || "删除失败", icon: "none" });
        });
    }
  });
}

function preview(current: string, urls: string[]) {
  uni.previewImage({ current, urls });
}

function openUser(uid: string) {
  if (uid) {
    uni.navigateTo({ url: `/pages/user/home?uid=${encodeURIComponent(uid)}` });
  }
}

onLoad((query) => {
  dynamicId.value = String(query?.id || query?.dynamicid || "");
  void reload();
});

onPullDownRefresh(() => {
  void reload();
});

onReachBottom(() => {
  void loadComments(false);
});

defineExpose({ displayCount });
</script>

<style scoped>
.detail-page {
  padding-top: 24rpx;
}

.detail-card,
.comment-box,
.comment-card {
  padding: 24rpx;
}

.author-row {
  display: flex;
  align-items: center;
  gap: 16rpx;
}

.avatar,
.comment-avatar {
  width: 76rpx;
  height: 76rpx;
  border-radius: 38rpx;
  background: var(--line);
}

.author-main {
  flex: 1;
  min-width: 0;
}

.name,
.comment-name {
  display: block;
  color: var(--ink);
  font-size: 29rpx;
  font-weight: 900;
}

.time,
.comment-time {
  display: block;
  margin-top: 6rpx;
  color: #9aa3b3;
  font-size: 22rpx;
}

.mini {
  height: 54rpx;
  padding: 0 20rpx;
  font-size: 23rpx;
}

.more-button {
  width: 58rpx;
  height: 54rpx;
  color: var(--ink-3);
  font-size: 28rpx;
  font-weight: 900;
}

.content {
  display: block;
  margin-top: 22rpx;
  color: #323845;
  font-size: 30rpx;
  line-height: 1.6;
}

.image-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8rpx;
  margin-top: 20rpx;
}

.feed-image {
  width: 100%;
  height: 214rpx;
  border-radius: 10rpx;
  background: #f1f2f6;
}

.feed-video {
  width: 100%;
  height: 380rpx;
  margin-top: 20rpx;
  border-radius: 12rpx;
  overflow: hidden;
  background: #111;
}

.voice-row {
  display: inline-flex;
  height: 64rpx;
  align-items: center;
  padding: 0 24rpx;
  margin-top: 20rpx;
  border-radius: 32rpx;
  color: var(--brand);
  font-size: 25rpx;
  background: #fff2f5;
}

.meta-row {
  display: flex;
  align-items: center;
  gap: 26rpx;
  margin-top: 24rpx;
  padding-top: 20rpx;
  border-top: 1rpx solid #f0f2f6;
}

.meta,
.meta-text {
  color: var(--ink-2);
  font-size: 25rpx;
  font-weight: 800;
}

.comment-box {
  margin-top: 20rpx;
}

.comment-input {
  width: 100%;
  min-height: 96rpx;
  color: var(--ink);
  font-size: 28rpx;
  line-height: 1.5;
}

.comment-actions {
  display: flex;
  justify-content: flex-end;
  gap: 16rpx;
  margin-top: 18rpx;
}

.section-title {
  margin: 30rpx 4rpx 18rpx;
  color: var(--ink);
  font-size: 31rpx;
  font-weight: 900;
}

.comment-card {
  display: flex;
  gap: 16rpx;
  margin-bottom: 16rpx;
}

.comment-body {
  flex: 1;
  min-width: 0;
}

.comment-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 14rpx;
}

.comment-content {
  display: block;
  margin-top: 12rpx;
  color: #313744;
  font-size: 27rpx;
  line-height: 1.5;
}

.comment-tools {
  display: flex;
  gap: 24rpx;
  margin-top: 14rpx;
}

.comment-tools button,
.load-reply,
.reply-delete {
  color: var(--ink-3);
  font-size: 23rpx;
  font-weight: 700;
}

.reply-box {
  margin-top: 14rpx;
  padding: 16rpx;
  border-radius: 12rpx;
  background: var(--bg);
}

.reply-row {
  display: flex;
  align-items: flex-start;
  gap: 10rpx;
  margin-bottom: 10rpx;
}

.reply-row:last-child {
  margin-bottom: 0;
}

.reply-name {
  color: var(--brand);
  font-size: 24rpx;
  font-weight: 800;
}

.reply-text {
  flex: 1;
  color: #4b5565;
  font-size: 24rpx;
  line-height: 1.4;
}

.load-reply {
  margin-top: 12rpx;
}
</style>
