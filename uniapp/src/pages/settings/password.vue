<template>
  <view class="safe-page password-page">
    <view class="form card">
      <input v-model.trim="oldpass" class="input" password :placeholder="t('misc.password.current')" />
      <input v-model.trim="pass" class="input" password :placeholder="t('misc.auth.newPassword')" />
      <input v-model.trim="pass2" class="input" password :placeholder="t('misc.auth.confirmNewPassword')" />
    </view>
    <button class="primary-button submit" :disabled="submitting" @tap="submit">{{ t("misc.password.confirmChange") }}</button>
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { updatePassword } from "@/api/services";
import { clearSession, requireLogin } from "@/utils/session";
import { t } from "@/i18n";

const oldpass = ref("");
const pass = ref("");
const pass2 = ref("");
const submitting = ref(false);

async function submit() {
  if (!requireLogin() || submitting.value) {
    return;
  }
  if (!oldpass.value || !pass.value || !pass2.value) {
    uni.showToast({ title: t("misc.password.fillAll"), icon: "none" });
    return;
  }
  if (pass.value !== pass2.value) {
    uni.showToast({ title: t("misc.password.newMismatch"), icon: "none" });
    return;
  }
  submitting.value = true;
  try {
    await updatePassword(oldpass.value, pass.value, pass2.value);
    clearSession();
    uni.showToast({ title: t("misc.password.changed"), icon: "none" });
    setTimeout(() => {
      uni.redirectTo({ url: "/pages/auth/login" });
    }, 500);
  } catch (error: any) {
    uni.showToast({ title: error?.message || t("misc.password.changeFailed"), icon: "none" });
  } finally {
    submitting.value = false;
  }
}
</script>

<style scoped>
.form {
  padding: 24rpx;
}

.input {
  margin-bottom: 18rpx;
}

.input:last-child {
  margin-bottom: 0;
}

.submit {
  margin-top: 28rpx;
}
</style>
