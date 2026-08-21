<script setup lang="ts">
import { onLaunch, onShow, onHide } from "@dcloudio/uni-app";
import { onBeforeSessionClear, preloadSession } from "@/utils/session";
import { checkHotUpdate } from "@/utils/hotupdate";
import { closeIMDatabase, initIMDatabase } from "@/utils/imDatabase";
import { cleanupRemoteOnSessionEnd } from "@/utils/remote-assistance";
import { applyTabBarLocale, useI18n } from "@/i18n";

const { locale, setLocale } = useI18n();

onBeforeSessionClear((session) => cleanupRemoteOnSessionEnd(session));

onLaunch(() => {
  setLocale(locale.value);
  applyTabBarLocale();
  preloadSession();
  void initIMDatabase();
  // App 端静默检查资源热更新，失败不影响启动
  void checkHotUpdate();
});

onShow(() => {
  applyTabBarLocale();
  void initIMDatabase();
  // 从后台、安装器或系统设置返回时继续检查强制更新；内部带并发与间隔保护。
  void checkHotUpdate();
});
onHide(() => {
  void closeIMDatabase();
});
</script>

<style>
page {
  background: #f7f8fb;
}
</style>
