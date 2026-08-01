-- Production recharge must only expose orderable database-backed products.
-- Seed the fixed RMB tiers used by the client; each insert is idempotent by
-- amount so an administrator-owned product is never duplicated or replaced.

INSERT INTO recharge_products
    (name,fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin,status,sort_order)
SELECT '10 星币','CNY',2,1000,10,0,1,60
WHERE NOT EXISTS (
    SELECT 1 FROM recharge_products
    WHERE fiat_currency='CNY' AND currency_scale=2 AND amount_minor=1000
);

INSERT INTO recharge_products
    (name,fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin,status,sort_order)
SELECT '100 星币','CNY',2,10000,100,0,1,50
WHERE NOT EXISTS (
    SELECT 1 FROM recharge_products
    WHERE fiat_currency='CNY' AND currency_scale=2 AND amount_minor=10000
);

INSERT INTO recharge_products
    (name,fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin,status,sort_order)
SELECT '3000 星币','CNY',2,300000,3000,0,1,40
WHERE NOT EXISTS (
    SELECT 1 FROM recharge_products
    WHERE fiat_currency='CNY' AND currency_scale=2 AND amount_minor=300000
);

INSERT INTO recharge_products
    (name,fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin,status,sort_order)
SELECT '9800 星币','CNY',2,980000,9800,0,1,30
WHERE NOT EXISTS (
    SELECT 1 FROM recharge_products
    WHERE fiat_currency='CNY' AND currency_scale=2 AND amount_minor=980000
);

INSERT INTO recharge_products
    (name,fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin,status,sort_order)
SELECT '38800 星币','CNY',2,3880000,38800,0,1,20
WHERE NOT EXISTS (
    SELECT 1 FROM recharge_products
    WHERE fiat_currency='CNY' AND currency_scale=2 AND amount_minor=3880000
);

INSERT INTO recharge_products
    (name,fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin,status,sort_order)
SELECT '58800 星币','CNY',2,5880000,58800,0,1,10
WHERE NOT EXISTS (
    SELECT 1 FROM recharge_products
    WHERE fiat_currency='CNY' AND currency_scale=2 AND amount_minor=5880000
);
