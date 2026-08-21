<template>
  <view id="appLiveVideoHost" class="app-live-video-host" :prop="state" :change:prop="liveVideoRenderer.update" />
</template>

<script>
// @ts-nocheck -- vue-tsc does not model uni-app's separate renderjs module.
export default {
  name: "AppLiveVideo",
  props: {
    state: {
      type: Object,
      required: true
    }
  },
  emits: ["canplay", "error"],
  methods: {
    handleCanPlay() {
      this.$emit("canplay");
    },
    handlePlaybackError(detail) {
      this.$emit("error", detail);
    }
  }
};
</script>

<script module="liveVideoRenderer" lang="renderjs">
import Hls from "hls.js";

export default {
  methods: {
    notify(method, detail) {
      if (this.ownerInstance) {
        this.ownerInstance.callMethod(method, detail);
      }
    },
    ensureVideo(ownerInstance, viewInstance) {
      this.ownerInstance = ownerInstance || this.ownerInstance;
      this.viewInstance = viewInstance || this.viewInstance;
      const instanceElement = this.viewInstance && this.viewInstance.$el;
      const host = instanceElement && instanceElement.nodeType === 1
        ? instanceElement
        : document.getElementById("appLiveVideoHost");
      if (!host || host.nodeType !== 1) {
        this.notify("handlePlaybackError", { reason: "video-host" });
        return;
      }
      if (this.video && this.video.parentNode === host) {
        return this.video;
      }
      const video = document.createElement("video");
      video.autoplay = true;
      video.controls = false;
      video.playsInline = true;
      video.preload = "auto";
      video.setAttribute("playsinline", "true");
      video.setAttribute("webkit-playsinline", "true");
      video.setAttribute("x5-playsinline", "true");
      video.style.position = "absolute";
      video.style.inset = "0";
      video.style.width = "100%";
      video.style.height = "100%";
      video.style.objectFit = "cover";
      video.style.background = "#070709";
      video.addEventListener("canplay", () => this.notify("handleCanPlay"));
      video.addEventListener("error", () => {
        if (!this.hls) {
          this.notify("handlePlaybackError", { reason: "media" });
        }
      });
      while (host.firstChild) {
        host.removeChild(host.firstChild);
      }
      host.appendChild(video);
      this.video = video;
      return video;
    },
    destroyHls() {
      if (this.hls) {
        this.hls.destroy();
        this.hls = undefined;
      }
      this.hlsRecoveryAttempts = 0;
    },
    play(video) {
      const result = video.play();
      if (result && result.catch) {
        result.catch(() => undefined);
      }
    },
    loadSource(video, src) {
      this.destroyHls();
      this.currentSource = src;
      if (/\.m3u8(?:\?|#|$)/i.test(src) && Hls.isSupported()) {
        const hls = new Hls({
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
        this.hls = hls;
        hls.attachMedia(video);
        hls.on(Hls.Events.MEDIA_ATTACHED, () => hls.loadSource(src));
        hls.on(Hls.Events.MANIFEST_PARSED, () => {
          this.hlsRecoveryAttempts = 0;
          this.notify("handleCanPlay");
          this.play(video);
        });
        hls.on(Hls.Events.ERROR, (_event, data) => {
          if (!data || !data.fatal || this.hls !== hls) {
            return;
          }
          if (this.hlsRecoveryAttempts < 2 && data.type === Hls.ErrorTypes.NETWORK_ERROR) {
            this.hlsRecoveryAttempts += 1;
            hls.startLoad();
            return;
          }
          if (this.hlsRecoveryAttempts < 2 && data.type === Hls.ErrorTypes.MEDIA_ERROR) {
            this.hlsRecoveryAttempts += 1;
            hls.recoverMediaError();
            return;
          }
          this.notify("handlePlaybackError", {
            reason: "hls",
            type: String(data.type || ""),
            details: String(data.details || "")
          });
        });
        return;
      }
      video.src = src;
      video.load();
      this.play(video);
    },
    update(state, _oldState, ownerInstance, viewInstance) {
      this.ownerInstance = ownerInstance || this.ownerInstance;
      this.viewInstance = viewInstance || this.viewInstance;
      if (!state || !state.src) {
        this.destroyHls();
        this.currentSource = "";
        if (this.video) {
          this.video.removeAttribute("src");
          this.video.load();
        }
        return;
      }
      const video = this.ensureVideo(ownerInstance, viewInstance);
      if (!video) {
        return;
      }
      video.muted = Boolean(state.muted);
      video.volume = state.muted ? 0 : 1;
      if (state.poster) {
        video.poster = state.poster;
      } else {
        video.removeAttribute("poster");
      }
      if (this.currentSource !== state.src) {
        this.loadSource(video, state.src);
      } else if (video.paused) {
        this.play(video);
      }
    },
    cleanup() {
      this.destroyHls();
      if (this.video) {
        this.video.pause();
        this.video.removeAttribute("src");
        this.video.load();
      }
      this.video = undefined;
      this.currentSource = "";
      this.ownerInstance = undefined;
      this.viewInstance = undefined;
    }
  },
  beforeDestroy() {
    this.cleanup();
  },
  beforeUnmount() {
    this.cleanup();
  }
};
</script>

<style scoped>
.app-live-video-host {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: #070709;
}
</style>
