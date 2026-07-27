# 邀请归因链路

## 当前现状

- 用户注册时会生成 `cmf_agent_code.uid/code`，邀请码是真实业务数据。
- 上下级关系写入 `cmf_agent.uid/one_uid`。
- 原有绑定入口是 `User.setDistribut`，已经限制：
  - 用户只能设置一次上级。
  - 不能填写自己的邀请码。
  - 不能填写直接下级的邀请码。
- 移动端原来只支持 OpenInstall 或手填邀请码；本地配置中 `openinstall_switch=1`，但 `openinstall_appkey` 为空，所以 OpenInstall 免填链路不可用。

## 新增链路

### 1. 用户 A 生成邀请链接

后端继续使用 `Agent.getCode` 返回用户自己的邀请码。

可用链接：

```text
https://yourdomain.com/invite?ref=USER_A_CODE
https://yourdomain.com/appapi/agent/downapp?code=USER_A_CODE
```

`/invite` 已路由到 `appapi/agent/invite`，老二维码页 `/appapi/agent/downapp` 保留。

### 2. 下载页记录点击

下载页会校验 `ref/code` 是否存在于 `cmf_agent_code`，存在才记录 `cmf_invite_click`。

记录字段包括：

- `click_id`
- `ref_code`
- `inviter_uid`
- `platform`
- `ip` / `ip_hash`
- `user_agent` / `ua_hash`
- `referer`
- `landing_url`
- `download_url`
- `expires_at`

不会写入任何 demo 数据。Web 页面拿不到稳定设备 ID 时，`device_fingerprint` 保持空，等 App 首次打开再上报。

### 3. App 首次登录后归因

新增 PhalApi：

```text
Invite.resolve
Invite.bind
```

`Invite.resolve` 只解析候选来源，不绑定上下级。

`Invite.bind` 需要 `uid/token`，并在匹配置信度足够时复用 `Model_User::setDistribut` 写入真实 `cmf_agent`。

推荐参数：

```text
platform=android|ios
device_id=客户端稳定设备指纹
android_id=Android ID
idfv=iOS IDFV
code=OpenInstall 或短链透传的邀请码
ref=OpenInstall 或短链透传的邀请码
click_id=下载页点击ID
```

Android 已在登录后调用 `Invite.bind`，失败再进入原手填弹窗。

iOS 已在首页邀请码检查时调用 `Invite.bind`，失败再进入原手填弹窗。

## 匹配优先级

从高到低：

1. `direct_ref`：App 明确拿到 `code/ref`，置信度 100，可自动绑定。
2. `click_id`：App 明确拿到下载页 `click_id`，置信度 95，可自动绑定。
3. `device_fingerprint`：设备指纹命中点击记录，置信度 90，可自动绑定。
4. `ip_ua`：IP 和 User-Agent 同时命中点击记录，置信度 70，可自动绑定。
5. `ip_recent`：仅同 IP 且 2 小时内最近点击，置信度 40，只返回候选，不自动绑定，避免共享网络绑错人。

自动绑定阈值：`confidence >= 60`。

点击记录有效期：7 天。

## 数据表

DDL 文件：

```text
docs/sql/invite_attribution_20260612.sql
```

已在本地数据库创建：

```text
cmf_invite_click
cmf_invite_bind
```

这两张表只存真实点击、真实候选和真实绑定结果；不允许长期保留测试数据。

## 验证记录

已验证：

- `Invite.resolve&ref=29VJ35` 可以解析到真实邀请人 `uid=10000`。
- `/invite?ref=29VJ35` 可以展示下载页、设置 cookie、写入点击记录。
- `Invite.resolve&click_id=...` 可以按点击 ID 解析候选。
- Android 使用 JDK 8 执行 `./gradlew :app:assembleDebug` 编译通过。

验证后已清理测试点击和测试绑定记录，数据库未保留测试数据。

限制：

- iOS 工程当前缺少 Pods 生成文件 `Pods-KSTiSDKdemo.debug.xcconfig`，`xcodebuild` 无法进入源码编译阶段；需要补齐 Podfile/Pods 后再做完整编译。
