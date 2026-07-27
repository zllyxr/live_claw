-- Remove shop / mall / paid content data structures.
-- Generated for local development cleanup on 2026-06-06.

DROP TABLE IF EXISTS
  cmf_apply_goods_class,
  cmf_paidprogram,
  cmf_paidprogram_apply,
  cmf_paidprogram_class,
  cmf_paidprogram_comment,
  cmf_paidprogram_order,
  cmf_seller_goods_class,
  cmf_seller_platform_goods,
  cmf_shop_address,
  cmf_shop_apply,
  cmf_shop_bond,
  cmf_shop_express,
  cmf_shop_goods,
  cmf_shop_goods_class,
  cmf_shop_order,
  cmf_shop_order_comments,
  cmf_shop_order_message,
  cmf_shop_order_refund,
  cmf_shop_order_refund_list,
  cmf_shop_platform_reason,
  cmf_shop_points,
  cmf_shop_refund_reason,
  cmf_shop_refuse_reason,
  cmf_user_goods_collect,
  cmf_user_goods_visit;

ALTER TABLE cmf_dynamic
  DROP COLUMN goodsid,
  DROP COLUMN goods_isxiajia;

ALTER TABLE cmf_live
  DROP COLUMN isshop;

ALTER TABLE cmf_video
  DROP COLUMN goodsid;

DELETE FROM cmf_admin_menu
WHERE id IN (462, 572)
   OR parent_id IN (462, 572)
   OR controller IN (
      'Shop', 'Shopapply', 'shopbond', 'shopgoods', 'Goodsclass', 'Buyeraddress',
      'Express', 'Refundreason', 'Goodsorder', 'Refundlist', 'Shopcash',
      'Shopcategory', 'paidprogram', 'Paidprogramclass', 'Paidprogram'
   );

DELETE FROM cmf_auth_rule
WHERE name REGEXP '^Admin/(Shop|Shopapply|shopbond|shopgoods|Goodsclass|Buyeraddress|Express|Refundreason|Goodsorder|Refundlist|Shopcash|Shopcategory|paidprogram|Paidprogramclass|Paidprogram)'
   OR name REGEXP '^Appapi/(Shop|Mall|Shoporder|Shoppay|Goodsorderrefund|Shopcash|Express|Paidprogram|Paidprogrampay)';
