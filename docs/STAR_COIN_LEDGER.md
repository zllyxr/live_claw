# 星币单一账本规则

更新时间：2026-06-12

## 统一规则

- 全站只保留一个账面单位：星币。
- 1 星币 = 1 美元 / 1 USDT 的支付口径。
- 不再允许出现第二套到账数量、赠送数量、云票比例、钻石比例、提现抽成比例。
- 旧字段名暂时保留，避免大面积破坏接口和数据库结构；字段语义统一改为星币。

## 核心字段语义

- `cmf_user.coin`：用户可消费星币余额。
- `cmf_user.votes`：主播可提现星币余额，旧名 votes。
- `cmf_user.votestotal`：主播累计收到星币，旧名 votestotal。
- `cmf_charge_rules.money`：充值星币数量。
- `cmf_charge_rules.coin`：到账星币数量，必须等于 `money`。
- `cmf_charge_rules.coin_ios`：兼容旧客户端字段，必须等于 `coin`。
- `cmf_charge_rules.coin_paypal`：兼容旧客户端字段，必须等于 `coin`。
- `cmf_charge_rules.give`：固定为 0，不参与到账。
- `cmf_charge_user.money`：订单星币数量。
- `cmf_charge_user.coin`：订单到账星币数量。
- `cmf_charge_user.coin_give`：兼容旧字段，固定为 0，不参与到账。
- `cmf_cash_record.votes`：提现扣减星币。
- `cmf_cash_record.money`：提现到账星币。

## 已落地配置

- `site_info.name_coin/name_votes/name_score` 固定为 `星币`。
- 英文字段固定为 `Star Coin`。
- `configpri.cash_rate` 固定为 `1`。
- `configpri.cash_take` 固定为 `0`。
- `configpri.bepusdt_fiat` 固定为 `USD`。
- Docker 环境变量 `BEPUSDT_FIAT=USD`。

## 本地充值包

- 普通充值：10、50、100、500、1000 星币。
- 首充礼包：1、10、100 星币。
- 所有充值包 `money = coin = coin_ios = coin_paypal`，`give = 0`。

## 代码兜底

- 后端公共配置读取时会强制返回星币名称、提现 1:1、抽成 0。
- 后端充值规则读取和下单时会归一为 `money = coin = coin_ios = coin_paypal`。
- 支付成功入账只增加 `charge_user.coin`，不再叠加 `coin_give`。
- 提现只按输入星币数扣减和记录，不再按 `cash_rate/cash_take` 换算。
- Android/iOS 充值、收益、游戏余额展示已改为星币。
