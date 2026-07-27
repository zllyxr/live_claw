# App API 服务索引

整理日期: 2026-06-05

本索引记录 `admin/phalapi/src/app/Api` 下的公开服务模块。客户端调用通常使用 `Module.Method`，例如 `Live.createRoom`。后台实现路径通常为:

- 入口: `admin/public/appapi/index.php`
- API 参数层: `admin/phalapi/src/app/Api/<Module>.php`
- 业务层: `admin/phalapi/src/app/Domain/<Module>.php`
- 数据层: `admin/phalapi/src/app/Model/<Module>.php`

## 核心服务

| 模块 | 公开方法 |
| --- | --- |
| `Home` | `getConfig`, `getLogin`, `getHot`, `getFollow`, `search`, `getNearby`, `getRecommend`, `attentRecommend`, `profitList`, `consumeList`, `getClassLive`, `getFilterField`, `getShopList`, `getShopThreeClass`, `getShopClassList`, `searchShop`, `getVoiceLiveList`, `updateCity` |
| `Login` | `userLogin`, `userReg`, `userFindPass`, `userLoginByThird`, `getCode`, `getForgetCode`, `getUnionid`, `logout`, `getCancelCondition`, `cancelAccount`, `getCountrys` |
| `User` | `iftoken`, `getBaseInfo`, `updateAvatar`, `updateFields`, `updatePass`, `getBalance`, `getProfit`, `setCash`, `isAttent`, `setAttent`, `isBlacked`, `checkBlack`, `setBlack`, `getBindCode`, `setMobile`, `getFollowsList`, `getFansList`, `getBlackList`, `getLiverecord`, `getAliCdnRecord`, `getUserHome`, `getContributeList`, `getPmUserInfo`, `getMultiInfo`, `getUidsInfo`, `Bonus`, `getBonus`, `setDistribut`, `getUserLabel`, `setUserLabel`, `getMyLabel`, `getPerSetting`, `getUserAccountList`, `setUserAccount`, `delUserAccount`, `setShopCash`, `getAuthInfo`, `seeDailyTasks`, `receiveTaskReward`, `getQiniuToken`, `setBeautyParams`, `getBeautyParams`, `getBraintreeToken`, `BraintreeCallback`, `getTurntableWinLists`, `clearTurntableWinLists`, `checkTeenager`, `setTeenagerPassword`, `updateTeenagerPassword`, `closeTeenager`, `addTeenagerTime`, `updateBgImg`, `setLiveWindow` |
| `Live` | `getSDK`, `createRoom`, `changeLive`, `changeLiveType`, `stopRoom`, `stopInfo`, `checkLive`, `roomCharge`, `timeCharge`, `enterRoom`, `showVideo`, `getZombie`, `getUserLists`, `getPop`, `getGiftList`, `sendGift`, `sendBarrage`, `setAdmin`, `getAdminList`, `getReportClass`, `setReport`, `getVotes`, `setShutUp`, `kicking`, `superStopRoom`, `getCoin`, `checkLiveing`, `getLiveInfo`, `setLiveGoodsIsShow`, `signOutWatchLive`, `shareLiveRoom`, `applyVoiceLiveMic`, `cancelVoiceLiveMicApply`, `handleVoiceMicApply`, `getVoiceMicApplyList`, `changeVoiceEmptyMicStatus`, `anchorGetVoiceMicList`, `changeVoiceMicStatus`, `userCloseVoiceMic`, `closeUserVoiceMic`, `getVoiceMicStream`, `getVoiceLivePullStreams`, `getLiveBanRules`, `getLiveBanInfo`, `checkUserRedis`, `getMicPullUrl`, `getUserRank` |
| `Linkmic` | `setMic`, `isMic`, `RequestLVBAddrForLinkMic`, `RequestPlayUrlWithSignForLinkMic`, `getSwRtcToken`, `getSwRtcPKToken` |
| `Livepk` | `getLiveList`, `search`, `checkLive`, `changeLive`, `setPK`, `endPK`, `getPkUid` |
| `Livemanage` | `getManageList`, `cancelManage`, `getRoomList`, `getShutList`, `cancelShut`, `getKickList`, `cancelKick` |
| `Video` | `getCon`, `setVideo`, `setComment`, `addView`, `addLike`, `addShare`, `setBlack`, `addCommentLike`, `getVideoList`, `getAttentionVideo`, `getVideo`, `getComments`, `getReplys`, `getMyVideo`, `del`, `report`, `getHomeVideo`, `getQiniuToken`, `getNearby`, `getReportContentlist`, `setConversion`, `getClassVideo`, `startWatchVideo`, `endWatchVideo`, `delComments`, `getLikeVideos` |
| `Music` | `classify_list`, `music_list`, `searchMusic`, `collectMusic`, `getCollectMusicLists`, `hotLists` |
| `Livemusic` | `searchMusic`, `getDownurl` |
| `Dynamic` | `setDynamic`, `setComment`, `addLike`, `addCommentLike`, `getAttentionDynamic`, `getNewDynamic`, `getHomeDynamic`, `getRecommendDynamics`, `getDynamic`, `getComments`, `getReplys`, `del`, `report`, `getQiniuToken`, `getReportlist`, `getDynamicLabels`, `getHotDynamicLabels`, `getLabelDynamic`, `searchHotLabels`, `searchLabels`, `delComments` |
| `Message` | `getList`, `getShopOrderList`, `fansLists`, `praiseLists`, `atLists`, `commentLists` |

## 商城与付费内容

| 模块 | 公开方法 |
| --- | --- |
| `Shop` | `getBond`, `deductBond`, `getOneGoodsClass`, `shopApply`, `getShopApplyInfo`, `getShop`, `getSale`, `setSale`, `getShopInfo`, `getGoodsInfo`, `getGoodsCommentList`, `searchShopGoods`, `setCollect`, `getGoodsCollect`, `getBusinessCategory`, `getApplyBusinessCategory`, `applyBusinessCategory`, `getGoodExistence`, `setPlatformGoodsSale`, `searchOnsalePlatformGoods`, `getOnsalePlatformGoods`, `delGoodsCollect` |
| `Buyer` | `getHome`, `addAddress`, `editAddress`, `addressList`, `delAddress`, `addGoodsVisitRecord`, `delGoodsVisitRecord`, `getGoodsVisitRecord`, `createGoodsOrder`, `getBalance`, `goodsOrderPay`, `getGoodsOrderList`, `cancelGoodsOrder`, `receiveGoodsOrder`, `delGoodsOrder`, `getGoodsOrderInfo`, `evaluateGoodsOrder`, `appendEvaluateGoodsOrder`, `getRefundReason`, `applyRefundGoodsOrder`, `cancelRefundGoodsOrder`, `getGoodsOrderRefundInfo`, `reapplyRefundGoodsOrder`, `getPlatformReasonList`, `applyPlatformInterpose`, `getRefundList` |
| `Seller` | `getHome`, `getGoodsClass`, `setGoods`, `getGoodsNums`, `getGoodsList`, `getReceiverAddress`, `upReceiverAddress`, `upGoodsSpecs`, `upGoods`, `delGoods`, `upStatus`, `getExpressList`, `getGoodsOrderList`, `setExpressInfo`, `delGoodsOrder`, `getGoodsOrderInfo`, `getGoodsOrderRefundInfo`, `getRefundRefuseReason`, `setGoodsOrderRefund`, `getSettlementList`, `setOutsideGoods`, `upOutsideGoods`, `getPlatformGoodsLists`, `setPlatformGoods`, `getOnsalePlatformGoods` |
| `Paidprogram` | `getApplyStatus`, `apply`, `getPaidprogramClassList`, `addPaidProgram`, `getPaidProgramInfo`, `getMyPaidProgram`, `getBalance`, `getAliOrder`, `getWxOrder`, `balancePay`, `getPaidProgramList`, `setComment`, `searchPaidProgram`, `getBraintreePaypalOrder`, `getHomePaidprogram` |

## 支付、游戏与运营

| 模块 | 公开方法 |
| --- | --- |
| `Charge` | `getWxOrder`, `getAliOrder`, `getIosOrder`, `getWxMiniOrder`, `getBraintreePaypalOrder`, `getFirstChargeRules` |
| `Game` | `settleGame`, `checkGame`, `Jinhua`, `endGame`, `JinhuaBet`, `Dial`, `Dial_end`, `Dial_Bet`, `getGameRecord`, `getBankerProfit`, `getXqtbRandList`, `xqtbPlay`, `getXqtbWinList`, `getXqtbTotalList`, `getXydzpRandList`, `xydzpPlay`, `getXydzpWinList`, `getXydzpTotalList` |
| `LotteryGame` | 由 Go Core 提供：首页、详情、当前期、下注、注单、开奖记录；开奖完全在本地定时完成 |
| `Sports` / `SportsBet` | 由 Go Core 提供：数据库赛程/比分、盘口、下注和注单；请求过程不访问体育上游 |
| `Turntable` | `getTurntable`, `turn`, `getWin` |
| `Jackpot` | `getJackpot` |
| `Guard` | `getGuardList`, `getList`, `buyGuard` |
| `Red` | `sendRed`, `getRedList`, `robRed`, `getRedRobList` |
| `Backpack` | `getBackpack` |
| `Agent` | `getCode`, `checkAgent` |
| `Family` | `createFamily` |
| `Auth` | `getAuth`, `setAuth` |
| `Guide` | `getGuide` |
| `Upload` | `getCosInfo`, `uploadFile` |
| `Wxmini` | `createM3u8Pull`, `getAuth`, `userAuth`, `profitList`, `goodsOrderRefundConsult`, `getOrderExpressInfo`, `getShopCashRecord`, `getSwRtcToken` |

## 使用约定

- 表中的方法省略了每个模块都有的 `getRules`。
- Android 对应调用位置通常在 `android/<module>/src/main/java/com/yunbao/<module>/http/*HttpUtil.java`。
- iOS 对应调用可以用 `rg "Module.Method" ios/YBLive` 查找。
- 如果客户端调用存在但表中没有，先检查大小写差异、旧接口、`admin/app/appapi/controller` 下的 H5/回调控制器。

## 上传与存储接口

当前主 App 上传链路已改为服务端中转:

1. App 调 `Upload.getCosInfo`。
2. 服务端返回 `cloudtype=local|minio` 和 `storageInfo.upload_url`。
3. App 用 multipart 表单把文件字段 `file` 上传到 `Upload.uploadFile`。
4. 服务端写入本地 `public/upload` 或 MinIO，并返回 `local_...` / `minio_...` 文件 key 和可访问 URL。

主接口位置:

- `admin/phalapi/src/app/Api/Upload.php`
- `admin/phalapi/src/app/functions.php`
- `admin/app/common.php`

历史遗留:

- `Video.getQiniuToken`, `Dynamic.getQiniuToken`, 部分 `Wxmini` 上传接口仍保留旧七牛实现，当前主 Android/iOS 上传入口不再使用它们。
- 后端 `get_upload_path()` 仍兼容 `qiniu_...` 和 `aws_...` 前缀，用于历史数据展示。
