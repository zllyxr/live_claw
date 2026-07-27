<template>
  <view class="safe-page password-page">
    <view class="form card">
      <input v-model.trim="oldpass" class="input" password placeholder="当前密码" />
      <input v-model.trim="pass" class="input" password placeholder="新密码" />
      <input v-model.trim="pass2" class="input" password placeholder="确认新密码" />
    </view>
    <button class="primary-button submit" :disabled="submitting" @tap="submit">确认修改</button>
  </view>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { updatePassword } from "@/api/services";
import { clearSession, requireLogin } from "@/utils/session";

const oldpass = ref("");
const pass = ref("");
const pass2 = ref("");
const submitting = ref(false);

async function submit() {
  if (!requireLogin() || submitting.value) {
    return;
  }
  if (!oldpass.value || !pass.value || !pass2.value) {
    uni.showToast({ title: "请完整填写密码", icon: "none" });
    return;
  }
  if (pass.value !== pass2.value) {
    uni.showToast({ title: "两次新密码不一致", icon: "none" });
    return;
  }
  submitting.value = true;
  try {
    await updatePassword(oldpass.value, pass.value, pass2.value);
    clearSession();
    uni.showToast({ title: "密码已修改，请重新登录", icon: "none" });
    setTimeout(() => {
      uni.redirectTo({ url: "/pages/auth/login" });
    }, 500);
  } catch (error: any) {
    uni.showToast({ title: error?.message || "修改失败", icon: "none" });
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
