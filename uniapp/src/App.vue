<script setup lang="ts">
import { onLaunch, onShow, onHide } from "@dcloudio/uni-app";
import { preloadSession } from "@/utils/session";
import { checkHotUpdate } from "@/utils/hotupdate";

onLaunch(() => {
  preloadSession();
  // App 端静默检查资源热更新，失败不影响启动
  void checkHotUpdate();
});

onShow(() => {});
onHide(() => {});
</script>

<style>
page {
  background: #f7f8fb;
}
</style>
