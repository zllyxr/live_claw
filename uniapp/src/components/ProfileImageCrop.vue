<template>
  <view class="crop-overlay" role="dialog" aria-modal="true" :aria-label="title" @keydown.esc="emit('cancel')">
    <view class="crop-card">
      <text class="crop-title">{{ title }}</text>
      <view class="crop-frame" :class="{ round: kind === 'avatar' }" :style="{ aspectRatio: size.width + ' / ' + size.height }"
        @pointerdown="startDrag" @pointermove="moveDrag" @pointerup="endDrag" @pointercancel="endDrag" @lostpointercapture="endDrag">
        <image v-if="bounds" class="preview" :src="src" :style="previewStyle" mode="scaleToFill" />
      </view>
      <text class="hint">{{ t('misc.profile.cropHint') }}</text>
      <slider :value="zoom * 100" :min="100" :max="400" :step="1" :block-size="20" activeColor="#7254ca"
        :aria-label="t('misc.profile.zoom')" @changing="setZoom" @change="setZoom" />
      <view class="crop-actions">
        <button class="cancel" @tap="emit('cancel')">{{ t('misc.common.cancel') }}</button>
        <button class="apply" :disabled="!bounds" @tap="confirm">{{ t('misc.profile.useCrop') }}</button>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, shallowRef } from "vue";
import { t } from "@/i18n";
import { cropBounds, profileImageSizes, type ProfileImageKind } from "@/utils/profile-crop";

const props = defineProps<{ src: string; kind: ProfileImageKind }>();
const emit = defineEmits<{ cancel: []; confirm: [path: string]; error: [] }>();
const size = computed(() => profileImageSizes[props.kind]);
const title = computed(() => t(props.kind === "avatar" ? "misc.profile.cropAvatar" : "misc.profile.cropBackground"));
const source = shallowRef<HTMLImageElement>();
const zoom = ref(1);
const center = ref({ x: 0, y: 0 });
const bounds = computed(() => source.value && cropBounds(source.value.naturalWidth, source.value.naturalHeight,
  props.kind, zoom.value, center.value.x, center.value.y));
const previewStyle = computed(() => {
  const rect = bounds.value;
  if (!source.value || !rect) return {};
  return {
    width: source.value.naturalWidth / rect.width * 100 + "%",
    height: source.value.naturalHeight / rect.height * 100 + "%",
    left: -rect.x / rect.width * 100 + "%", top: -rect.y / rect.height * 100 + "%"
  };
});

let image: HTMLImageElement;
onMounted(() => {
  image = new Image();
  image.onload = () => {
    center.value = { x: image.naturalWidth / 2, y: image.naturalHeight / 2 };
    source.value = image;
  };
  image.onerror = () => emit("error");
  image.src = props.src;
});
onBeforeUnmount(() => { image.onload = image.onerror = null; });

let drag: { id: number; x: number; y: number } | undefined;
function startDrag(event: PointerEvent) {
  if (!bounds.value || !event.isPrimary || event.button !== 0) return;
  (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
  drag = { id: event.pointerId, x: event.clientX, y: event.clientY };
}
function moveDrag(event: PointerEvent) {
  const rect = bounds.value;
  if (!rect || drag?.id !== event.pointerId) return;
  const frame = (event.currentTarget as HTMLElement).getBoundingClientRect();
  center.value = {
    x: rect.x + rect.width / 2 - (event.clientX - drag.x) * rect.width / frame.width,
    y: rect.y + rect.height / 2 - (event.clientY - drag.y) * rect.height / frame.height
  };
  drag = { id: event.pointerId, x: event.clientX, y: event.clientY };
}
function endDrag(event: PointerEvent) {
  if (drag?.id === event.pointerId) drag = undefined;
}
function setZoom(event: { detail: { value: number } }) {
  const rect = bounds.value;
  if (!rect) return;
  center.value = { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 };
  zoom.value = event.detail.value / 100;
}
function confirm() {
  const rect = bounds.value;
  if (!rect || !source.value) return;
  try {
    const canvas = document.createElement("canvas");
    canvas.width = size.value.width;
    canvas.height = size.value.height;
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error();
    ctx.fillStyle = "#fff";
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.drawImage(source.value, rect.x, rect.y, rect.width, rect.height, 0, 0, canvas.width, canvas.height);
    emit("confirm", canvas.toDataURL("image/jpeg", 0.9));
  } catch {
    emit("error");
  }
}
</script>

<style scoped>
.crop-overlay { position: fixed; inset: 0; z-index: 100; display: flex; align-items: center; justify-content: center; padding: 16px; background: rgba(17, 13, 32, .78); }
.crop-card { box-sizing: border-box; width: min(100%, 420px); max-height: 94vh; overflow: auto; padding: 20px; border-radius: 18px; background: var(--surface, white); }
.crop-title, .hint { display: block; }
.crop-title { margin-bottom: 16px; color: var(--ink); font-size: 19px; font-weight: 700; }
.crop-frame { position: relative; width: 100%; overflow: hidden; margin: auto; background: #eceaf2; cursor: grab; touch-action: none; user-select: none; }
.crop-frame.round { width: min(100%, 46vh); border-radius: 50%; }
.crop-frame:active { cursor: grabbing; }
.preview { position: absolute; max-width: none; pointer-events: none; }
.hint { margin-top: 12px; font-size: 14px; color: var(--ink-2); }
.crop-actions { display: flex; gap: 12px; margin-top: 12px; }
.crop-actions button { flex: 1; border-radius: 24px; font-size: 15px; }
.cancel { background: var(--bg); color: var(--ink); }
.apply { background: var(--grad-brand); color: white; }
</style>
