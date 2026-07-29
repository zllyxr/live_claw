-- Public copy rendered by the native client. Administrators can update these
-- pages through the existing system-settings editor without redeploying H5.
INSERT INTO system_settings
    (setting_key, setting_value, is_secret, version, updated_by)
VALUES
    ('content.pages',
     JSON_OBJECT(
         'recharge_agreement', JSON_OBJECT(
             'title', '充值协议',
             'content', CAST(
                 CONCAT(
                     '1. 充值前请确认账号、充值金额和支付方式，充值成功后星币将计入当前账号。',
                     CHAR(10), CHAR(10),
                     '2. 星币仅用于平台已开放的服务，不得用于违法交易、套现或其他违规用途。',
                     CHAR(10), CHAR(10),
                     '3. 如遇支付成功但余额未到账，请保留支付凭证并联系平台客服处理。',
                     CHAR(10), CHAR(10),
                     '4. 支付渠道、充值档位及到账状态均以平台实时接口返回为准。'
                 )
                 AS CHAR CHARACTER SET utf8mb4
             )
         )
     ),
     0, 1, 0)
ON DUPLICATE KEY UPDATE setting_key = VALUES(setting_key);
