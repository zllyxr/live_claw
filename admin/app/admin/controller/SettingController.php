<?php
// +----------------------------------------------------------------------
// | ThinkCMF [ WE CAN DO IT MORE SIMPLE ]
// +----------------------------------------------------------------------
// | Copyright (c) 2013-present http://www.thinkcmf.com All rights reserved.
// +----------------------------------------------------------------------
// | Licensed ( http://www.apache.org/licenses/LICENSE-2.0 )
// +----------------------------------------------------------------------
// | Author: 小夏 < 449134904@qq.com>
// +----------------------------------------------------------------------
namespace app\admin\controller;

use app\admin\model\RouteModel;
use app\admin\model\UserModel;
use cmf\controller\AdminBaseController;

/**
 * Class SettingController
 * @package app\admin\controller
 * @adminMenuRoot(
 *     'name'   =>'设置',
 *     'action' =>'default',
 *     'parent' =>'',
 *     'display'=> true,
 *     'order'  => 0,
 *     'icon'   =>'cogs',
 *     'remark' =>'系统设置入口'
 * )
 */
class SettingController extends AdminBaseController
{

    /**
     * 网站信息
     * @adminMenu(
     *     'name'   => '网站信息',
     *     'parent' => 'default',
     *     'display'=> true,
     *     'hasView'=> true,
     *     'order'  => 0,
     *     'icon'   => '',
     *     'remark' => '网站信息',
     *     'param'  => ''
     * )
     */
    public function site()
    {
        $content = hook_one('admin_setting_site_view');

        if (!empty($content)) {
            return $content;
        }

        $noNeedDirs     = [".", "..", ".svn", 'fonts'];
        $adminThemesDir = WEB_ROOT . config('template.cmf_admin_theme_path') . config('template.cmf_admin_default_theme') . '/public/assets/themes/';
        $adminStyles    = cmf_scan_dir($adminThemesDir . '*', GLOB_ONLYDIR);
        $adminStyles    = array_diff($adminStyles, $noNeedDirs);
        $cdnSettings    = cmf_get_option('cdn_settings');
        $cmfSettings    = cmf_get_option('cmf_settings');
        $adminSettings  = cmf_get_option('admin_settings');

        $adminThemes = [];
        $themes      = cmf_scan_dir(WEB_ROOT . config('template.cmf_admin_theme_path') . '/*', GLOB_ONLYDIR);

        foreach ($themes as $theme) {
            if (strpos($theme, 'admin_') === 0) {
                array_push($adminThemes, $theme);
            }
        }

        $this->assign('site_info', cmf_get_option('site_info'));
        $this->assign("admin_styles", $adminStyles);
        $this->assign("templates", []);
        $this->assign("admin_themes", $adminThemes);
        $this->assign("cdn_settings", $cdnSettings);
        $this->assign("admin_settings", $adminSettings);
        $this->assign("cmf_settings", $cmfSettings);

        return $this->fetch();
    }

    /**
     * 网站信息设置提交
     * @adminMenu(
     *     'name'   => '网站信息设置提交',
     *     'parent' => 'site',
     *     'display'=> false,
     *     'hasView'=> false,
     *     'order'  => 10000,
     *     'icon'   => '',
     *     'remark' => '网站信息设置提交',
     *     'param'  => ''
     * )
     */
    public function sitePost(){

        if ($this->request->isPost()) {
            $result = $this->validate($this->request->param(), 'SettingSite');
            if ($result !== true) {
                $this->error($result);
            }
            $oldconfig=cmf_get_option('site_info');
            $options = $this->request->param('options/a');
            $options['name_coin']='星币';
            $options['name_coin_en']='Star Coin';
            $options['name_score']='星币';
            $options['name_score_en']='Star Coin';
            $options['name_votes']='星币';
            $options['name_votes_en']='Star Coin';

            $login_type=isset($_POST['login_type'])?$_POST['login_type']:'';
            $share_type=isset($_POST['share_type'])?$_POST['share_type']:'';
            $live_type=isset($_POST['live_type'])?$_POST['live_type']:'';

            $options['login_type']='';
            $options['share_type']='';
            $options['live_type']='';

            if($login_type){
                $options['login_type']=implode(',',$login_type);
            }

            if($share_type){
                $options['share_type']=implode(',',$share_type);
            }
            if($live_type){
                $options['live_type']=implode(',',$live_type);
            }

            if($options['login_img']!=$options['login_img_old']){
                $options['login_img']=set_upload_path($options['login_img']);
            }

            if($options['login_img_en']!=$options['login_img_en_old']){
                $options['login_img_en']=set_upload_path($options['login_img_en']);
            }

            if($options['apk_ewm']!=$options['apk_ewm_old']){
                $options['apk_ewm']=set_upload_path($options['apk_ewm']);
            }

            if($options['ipa_ewm']!=$options['ipa_ewm_old']){
                $options['ipa_ewm']=set_upload_path($options['ipa_ewm']);
            }

            if($options['wechat_ewm']!=$options['wechat_ewm_old']){
                $options['wechat_ewm']=set_upload_path($options['wechat_ewm']);
            }

            if($options['voicelive_icon']!=$options['voicelive_icon_old']){
                $options['voicelive_icon']=set_upload_path($options['voicelive_icon']);
            }

            if($options['qr_url']!=$options['qr_url_old']){
                $options['qr_url']=set_upload_path($options['qr_url']);
            }

            unset($options['apk_ewm_old']);
            unset($options['ipa_ewm_old']);
            unset($options['wechat_ewm_old']);
            unset($options['voicelive_icon_old']);
            unset($options['qr_url_old']);
            unset($options['login_img_old']);
            unset($options['login_img_en_old']);

            cmf_set_option('site_info', $options);
            $this->resetcache('getConfigPub',$options);

            $cdnSettings = $this->request->param('cdn_settings/a');
            cmf_set_option('cdn_settings', $cdnSettings);

            $adminSettings = $this->request->param('admin_settings/a');

            $routeModel = new RouteModel();
            if (!empty($adminSettings['admin_password'])) {
                $routeModel->setRoute($adminSettings['admin_password'] . '$', 'admin/Index/index', [], 2, 5000);
            } else {
                $routeModel->deleteRoute('admin/Index/index', []);
            }

            $routeModel->getRoutes(true);

            if (!empty($adminSettings['admin_theme'])) {
                $result = cmf_set_dynamic_config([
                    'template' => [
                        'cmf_admin_default_theme' => $adminSettings['admin_theme']
                    ]
                ]);

                if ($result === false) {
                    $this->error('配置写入失败!');
                }
            }

            cmf_set_option('admin_settings', $adminSettings);

            $action="修改公共配置 ";

            if($options['maintain_switch'] !=$oldconfig['maintain_switch']){
                $maintain_switch=$options['maintain_switch']?'开':'关';
                $action.='网站维护 '.$maintain_switch.' ';
            }

            if($options['site_name'] !=$oldconfig['site_name']){
                $action.='网站名称 '.$options['site_name'].' ';
            }

            if($options['site'] !=$oldconfig['site']){
                $action.='网站域名 '.$options['site'].' ';
            }

            if($options['name_coin'] !=$oldconfig['name_coin']){
                $action.='星币名称 '.$options['name_coin'].' ';
            }

            if($options['name_score'] !=$oldconfig['name_score']){
                $action.='游戏余额名称 '.$options['name_score'].' ';
            }

            if($options['name_votes'] !=$oldconfig['name_votes']){
                $action.='收益名称 '.$options['name_votes'].' ';
            }


            if($options['isup'] !=$oldconfig['isup']){
                $isup=$options['isup']?'开':'关';
                $action.='修改强制更新 '.$isup.' ';
            }
            if($options['apk_ver'] !=$oldconfig['apk_ver']){
                $action.='修改APK版本号 '.$options['apk_ver'].' ';
            }
            if($options['apk_url'] !=$oldconfig['apk_url']){
                $action.='修改APK下载链接 ';
            }
            if($options['ipa_ver'] !=$oldconfig['ipa_ver']){
                $action.='修改IPA版本号 '.$options['ipa_ver'].' ';
            }
            if($options['ios_shelves'] !=$oldconfig['ios_shelves']){
                $action.='修改IPA上架版本号 '.$options['ios_shelves'].' ';
            }

            if($options['ipa_url'] !=$oldconfig['ipa_url']){
                $action.='修改IPA下载链接 '.$options['ipa_url'].' ';
            }

            if($options['login_type'] !=$oldconfig['login_type']){
                $action.='修改登录方式 ';
                $old_l=explode(',',$oldconfig['login_type']);
                $new_l=explode(',',$options['login_type']);
                foreach($old_l as $k=>$v){
                    if(!in_array($v,$new_l)){
                        $action.='关闭'.$v.' ';
                    }
                }

                foreach($new_l as $k=>$v){
                    if(!in_array($v,$old_l)){
                        $action.='开启'.$v.' ';
                    }
                }
            }
            if($options['share_type'] !=$oldconfig['share_type']){
                $action.='修改分享方式 ';

                $old_l=explode(',',$oldconfig['share_type']);
                $new_l=explode(',',$options['share_type']);
                foreach($old_l as $k=>$v){
                    if(!in_array($v,$new_l)){
                        $action.='关闭'.$v.' ';
                    }
                }

                foreach($new_l as $k=>$v){
                    if(!in_array($v,$old_l)){
                        $action.='开启'.$v.' ';
                    }
                }
            }


            if($options['wx_siteurl'] !=$oldconfig['wx_siteurl']){
                $action.='修改微信推广域名 '.$options['wx_siteurl'].' ';
            }

            if($options['share_title'] !=$oldconfig['share_title']){
                $action.='修改直播分享标题 '.$options['share_title'].' ';
            }

            if($options['share_des'] !=$oldconfig['share_des']){
                $action.='修改直播分享话术 '.$options['share_des'].' ';
            }

            if($options['app_android'] !=$oldconfig['app_android']){
                $action.='修改AndroidAPP下载链接 '.$options['app_android'].' ';
            }

            if($options['app_ios'] !=$oldconfig['app_ios']){
                $action.='修改IOSAPP下载链接 '.$options['app_ios'].' ';
            }

            if($options['video_share_title'] !=$oldconfig['video_share_title']){
                $action.='修改短视频分享标题 '.$options['video_share_title'].' ';
            }

            if($options['video_share_des'] !=$oldconfig['video_share_des']){
                $action.='修改短视频分享话术 '.$options['video_share_des'].' ';
            }

            if($options['live_type'] !=$oldconfig['live_type']){
                $action.='修改房间类型 ';

                $old_l=explode(',',$oldconfig['live_type']);
                $new_l=explode(',',$options['live_type']);
                foreach($old_l as $k=>$v){
                    if(!in_array($v,$new_l)){
                        $action.='关闭'.$v.' ';
                    }
                }

                foreach($new_l as $k=>$v){
                    if(!in_array($v,$old_l)){
                        $action.='开启'.$v.' ';
                    }
                }
            }
            if($options['live_time_coin'] !=$oldconfig['live_time_coin']){
                $action.='修改计时直播收费 ';
            }

            if($options['sprout_key'] !=$oldconfig['sprout_key']){
                $action.='修改萌颜授权码-Andriod '.$options['sprout_key'].' ';
            }

            if($options['sprout_key_ios'] !=$oldconfig['sprout_key_ios']){
                $action.='修改萌颜授权码-IOS '.$options['sprout_key_ios'].' ';
            }

            if($options['skin_whiting'] !=$oldconfig['skin_whiting']){
                $action.='修改美颜-美白 '.$options['skin_whiting'].' ';
            }

            if($options['skin_smooth'] !=$oldconfig['skin_smooth']){
                $action.='修改美颜-磨皮 '.$options['skin_smooth'].' ';
            }

            if($options['skin_tenderness'] !=$oldconfig['skin_tenderness']){
                $action.='修改美颜-红润 '.$options['skin_tenderness'].' ';
            }

            if($options['eye_brow'] !=$oldconfig['eye_brow']){
                $action.='修改磨皮默认值-眉毛 '.$options['eye_brow'].' ';
            }
            if($options['big_eye'] !=$oldconfig['big_eye']){
                $action.='修改磨皮默认值-大眼 '.$options['big_eye'].' ';
            }
            if($options['eye_length'] !=$oldconfig['eye_length']){
                $action.='修改磨皮默认值-眼距 '.$options['eye_length'].' ';
            }

            if($options['eye_corner'] !=$oldconfig['eye_corner']){
                $action.='修改磨皮默认值-眼角 '.$options['eye_corner'].' ';
            }

            if($options['eye_alat'] !=$oldconfig['eye_alat']){
                $action.='修改磨皮默认值-开眼角 '.$options['eye_alat'].' ';
            }

            if($options['face_lift'] !=$oldconfig['face_lift']){
                $action.='修改磨皮默认值-瘦脸 '.$options['face_lift'].' ';
            }

            if($options['face_shave'] !=$oldconfig['face_shave']){
                $action.='修改磨皮默认值-削脸 '.$options['face_shave'].' ';
            }

            if($options['mouse_lift'] !=$oldconfig['mouse_lift']){
                $action.='修改磨皮默认值-嘴形 '.$options['mouse_lift'].' ';
            }
            if($options['nose_lift'] !=$oldconfig['nose_lift']){
                $action.='修改磨皮默认值-瘦鼻 '.$options['nose_lift'].' ';
            }
            if($options['chin_lift'] !=$oldconfig['chin_lift']){
                $action.='修改磨皮默认值-下巴 '.$options['chin_lift'].' ';
            }
            if($options['forehead_lift'] !=$oldconfig['forehead_lift']){
                $action.='修改磨皮默认值-额头 '.$options['forehead_lift'].' ';
            }
            if($options['lengthen_noseLift'] !=$oldconfig['lengthen_noseLift']){
                $action.='修改磨皮默认值-长鼻 '.$options['lengthen_noseLift'].' ';
            }
            if($options['login_alert_title'] !=$oldconfig['login_alert_title']){
                $action.='修改弹框标题 '.$options['login_alert_title'].' ';
            }
            if($options['login_alert_content'] !=$oldconfig['login_alert_content']){
                $action.='修改弹框内容 '.$options['login_alert_content'].' ';
            }
            if($options['login_clause_title'] !=$oldconfig['login_clause_title']){
                $action.='修改APP登录界面底部协议标题 '.$options['login_clause_title'].' ';
            }
            if($options['login_private_title'] !=$oldconfig['login_private_title']){
                $action.='修改隐私政策名称 '.$options['login_private_title'].' ';
            }
            if($options['login_private_url'] !=$oldconfig['login_private_url']){
                $action.='修改隐私政策跳转链接 '.$options['login_private_url'].' ';
            }
            if($options['login_service_title'] !=$oldconfig['login_service_title']){
                $action.='修改服务协议名称 '.$options['login_service_title'].' ';
            }
            if($options['login_service_url'] !=$oldconfig['login_service_url']){
                $action.='修改服务协议跳转链接 '.$options['login_service_url'].' ';
            }

            if($action!='修改公共配置 '){
                setAdminLog($action);
            }

            $this->success("保存成功！", '');
        }
    }

    /**
     * 密码修改
     * @adminMenu(
     *     'name'   => '密码修改',
     *     'parent' => 'default',
     *     'display'=> false,
     *     'hasView'=> true,
     *     'order'  => 10000,
     *     'icon'   => '',
     *     'remark' => '密码修改',
     *     'param'  => ''
     * )
     */
    public function password()
    {
        return $this->fetch();
    }

    /**
     * 密码修改提交
     * @adminMenu(
     *     'name'   => '密码修改提交',
     *     'parent' => 'password',
     *     'display'=> false,
     *     'hasView'=> false,
     *     'order'  => 10000,
     *     'icon'   => '',
     *     'remark' => '密码修改提交',
     *     'param'  => ''
     * )
     */
    public function passwordPost(){

        if ($this->request->isPost()) {

            $data = $this->request->param();
            if (empty($data['old_password'])) {
                $this->error("原始密码不能为空！");
            }
            if (empty($data['password'])) {
                $this->error("新密码不能为空！");
            }

            $userId = cmf_get_current_admin_id();

            $admin = UserModel::where("id", $userId)->find();

            $oldPassword = $data['old_password'];
            $password    = $data['password'];
            $rePassword  = $data['re_password'];

            if (cmf_compare_password($oldPassword, $admin['user_pass'])) {
                if ($password == $rePassword) {

                    if (cmf_compare_password($password, $admin['user_pass'])) {
                        $this->error("新密码不能和原始密码相同！");
                    } else {
                        UserModel::where('id', $userId)->update(['user_pass' => cmf_password($password)]);
                        $this->success("密码修改成功！");
                    }
                } else {
                    $this->error("密码输入不一致！");
                }

            } else {
                $this->error("原始密码不正确！");
            }
        }
    }

    /**
     * 上传限制设置界面
     * @adminMenu(
     *     'name'   => '上传设置',
     *     'parent' => 'default',
     *     'display'=> true,
     *     'hasView'=> true,
     *     'order'  => 10000,
     *     'icon'   => '',
     *     'remark' => '上传设置',
     *     'param'  => ''
     * )
     */
    public function upload()
    {
        $uploadSetting = cmf_get_upload_setting();
        $this->assign('upload_setting', $uploadSetting);
        return $this->fetch();
    }

    /**
     * 上传限制设置界面提交
     * @adminMenu(
     *     'name'   => '上传设置提交',
     *     'parent' => 'upload',
     *     'display'=> false,
     *     'hasView'=> false,
     *     'order'  => 10000,
     *     'icon'   => '',
     *     'remark' => '上传设置提交',
     *     'param'  => ''
     * )
     */
    public function uploadPost()
    {
        if ($this->request->isPost()) {
            //TODO 非空验证
            $uploadSetting = $this->request->post();

            cmf_set_option('upload_setting', $uploadSetting);
            $this->success('保存成功！');
        }

    }

    /**
     * 清除缓存
     * @adminMenu(
     *     'name'   => '清除缓存',
     *     'parent' => 'default',
     *     'display'=> false,
     *     'hasView'=> true,
     *     'order'  => 10000,
     *     'icon'   => '',
     *     'remark' => '清除缓存',
     *     'param'  => ''
     * )
     */
    public function clearCache(){
        $content = hook_one('admin_setting_clear_cache_view');

        if (!empty($content)) {
            return $content;
        }

        cmf_clear_cache();
        return $this->fetch();
    }

    /**
     * 私密设置
     */
    public function configpri(){
        $siteinfo=getConfigPub();
        $name_coin=$siteinfo['name_coin'] ?? '星币';
        $this->assign('name_coin',$name_coin);
        $this->assign('config', getConfigPri());

        return $this->fetch();
    }

    /**
     * 私密设置提交
     */
    public function configpriPost(){

        if ($this->request->isPost()) {


            $oldconfigpri=cmf_get_option('configpri');

            $options = $this->request->param('options/a');
            $options['cash_rate']='1';
            $options['cash_take']='0';
            $options['bepusdt_fiat']='USD';
            $options['cloudtype']=isset($options['cloudtype']) && $options['cloudtype']=='4' ? '4' : '3';
            $options['minio_endpoint']=trim($options['minio_endpoint'] ?? '');
            $options['minio_public_url']=trim($options['minio_public_url'] ?? '');
            $options['minio_bucket']=trim($options['minio_bucket'] ?? '');
            $options['minio_access_key']=trim($options['minio_access_key'] ?? '');
            $options['minio_secret_key']=trim($options['minio_secret_key'] ?? '');
            $options['minio_region']=trim($options['minio_region'] ?? 'us-east-1');
            $options['usdt_switch']=$options['usdt_switch'] ?? '0';
            $options['bepusdt_api_url']=trim($options['bepusdt_api_url'] ?? '');
            $options['bepusdt_api_token']=trim($options['bepusdt_api_token'] ?? '');
            $options['bepusdt_trade_type']=trim($options['bepusdt_trade_type'] ?? 'usdt.trc20');
            $options['bepusdt_fiat']='USD';
            $options['bepusdt_timeout']=trim($options['bepusdt_timeout'] ?? '1200');

            if($options['cloudtype']=='4'){
                if($options['minio_endpoint']=='' || $options['minio_bucket']=='' || $options['minio_access_key']=='' || $options['minio_secret_key']==''){
                    $this->error("MinIO存储请填写Endpoint、Bucket、Access Key和Secret Key");
                }
                if($options['minio_region']==''){
                    $options['minio_region']='us-east-1';
                }
            }

            $bepusdtConfig=claw_get_bepusdt_config($options);
            if($options['usdt_switch']=='1'){
                if($bepusdtConfig['api_url']=='' || $bepusdtConfig['api_token']==''){
                    $this->error("USDT支付请填写BEpusdt网关地址和API对接令牌");
                }
                if(!preg_match('#^https?://#i',$bepusdtConfig['api_url'])){
                    $this->error("BEpusdt网关地址必须以http://或https://开头");
                }
                if(!is_numeric($options['bepusdt_timeout']) || (int)$options['bepusdt_timeout']<120){
                    $this->error("BEpusdt订单超时时间不能小于120秒");
                }
                if($options['bepusdt_trade_type']==''){
                    $this->error("USDT支付请填写交易类型");
                }
            }

            if($options['reg_reward']==''){
                $this->error("登录配置请填写注册奖励");
            }

            if(!is_numeric($options['reg_reward'])){
                $this->error("注册奖励必须为数字");
            }

            if(floor($options['reg_reward']) !=$options['reg_reward']){
                $this->error("注册奖励必须为整数");
            }

            if($options['iplimit_times']==''){
                $this->error("登录配置请填写短信验证码IP限制次数");
            }

            if(!is_numeric($options['iplimit_times'])){
                $this->error("短信验证码IP限制次数必须为数字");
            }

            if(floor($options['iplimit_times']) !=$options['iplimit_times']){
                $this->error("短信验证码IP限制次数必须为整数");
            }

            if($options['level_limit']==''){
                $this->error("直播配置请填写直播限制等级");
            }

            if(!is_numeric($options['level_limit'])){
                $this->error("直播限制等级必须为数字");
            }

            if(floor($options['level_limit']) !=$options['level_limit']){
                $this->error("直播限制等级必须为整数");
            }

            if($options['speak_limit']==''){
                $this->error("直播配置请填写发言等级限制");
            }

            if(!is_numeric($options['speak_limit'])){
                $this->error("发言等级限制必须为数字");
            }

            if(floor($options['speak_limit']) !=$options['speak_limit']){
                $this->error("发言等级限制必须为整数");
            }

            if($options['barrage_limit']==''){
                $this->error("直播配置请填写弹幕等级限制");
            }

            if(!is_numeric($options['barrage_limit'])){
                $this->error("弹幕等级限制必须为数字");
            }

            if(floor($options['barrage_limit']) !=$options['barrage_limit']){
                $this->error("弹幕等级限制必须为整数");
            }

            if($options['barrage_fee']==''){
                $this->error("直播配置请填写弹幕费用");
            }

            if(!is_numeric($options['barrage_fee'])){
                $this->error("弹幕费用必须为数字");
            }

            if(floor($options['barrage_fee']) !=$options['barrage_fee']){
                $this->error("弹幕费用必须为整数");
            }


            if($options['distribut1']>100 || $options['distribut1']<0){
                $this->error("邀请一级分成在0-100之间！");
            }

            if($options['userlist_time']==''){
                $this->error("直播配置请填写用户列表请求间隔");
            }

            if(!is_numeric($options['userlist_time'])){
                $this->error("用户列表请求间隔必须为数字");
            }

            if(floor($options['userlist_time']) !=$options['userlist_time']){
                $this->error("用户列表请求间隔必须为整数");
            }

            if($options['userlist_time']<5){
                $this->error("用户列表请求间隔不能小于5秒");
            }

            if($options['mic_limit']==''){
                $this->error("直播配置请填写连麦等级限制");
            }

            if(!is_numeric($options['mic_limit'])){
                $this->error("连麦等级限制必须为数字");
            }

            if(floor($options['mic_limit']) !=$options['mic_limit']){
                $this->error("连麦等级限制必须为整数");
            }

            $game_switch=isset($_POST['game_switch'])?$_POST['game_switch']:'';

            $options['game_switch']='';

            if($game_switch){
                $options['game_switch']=implode(',',$game_switch);
            }

            $options['sensitive_words']=str_replace("+","",$options['sensitive_words']);

            $action="修私密配置 ";
            if($options['family_switch'] !=$oldconfigpri['family_switch']){
                $family_switch=$options['family_switch']?'开':'关';
                $action.='家族控制开关 '.$family_switch.' ';
            }

            if($options['family_member_divide_switch'] !=$oldconfigpri['family_member_divide_switch']){
                $family_member_divide_switch=$options['family_member_divide_switch']?'开':'关';
                $action.='家族长修改成员分成比例是否管理员审核 '.$family_member_divide_switch.' ';
            }

            if($options['service_switch'] !=$oldconfigpri['service_switch']){
                $service_switch=$options['service_switch']?'开':'关';
                $action.='客服 '.$service_switch.' ';
            }

            if($options['service_url'] !=$oldconfigpri['service_url']){
                $action.='客服链接 ';
            }

            if($options['sensitive_words'] !=$oldconfigpri['sensitive_words']){
                $action.='敏感词 ';
            }

            if($options['reg_reward'] !=$oldconfigpri['reg_reward']){
                $action.='注册奖励 '.$options['reg_reward'].' ';
            }

            if($options['bonus_switch'] !=$oldconfigpri['bonus_switch']){
                $bonus_switch=$options['bonus_switch']?'开':'关';
                $action.='登录奖励开关 '.$bonus_switch.' ';
            }

            if($options['sendcode_switch'] !=$oldconfigpri['sendcode_switch']){
                $sendcode_switch=$options['sendcode_switch']?'开':'关';
                $action.='短信验证码开关 '.$sendcode_switch.' ';
            }

            if($options['typecode_switch'] !=$oldconfigpri['typecode_switch']){
                $typecode_switch=$options['typecode_switch']==1?'阿里云':'容联云';
                $action.='短信接口平台 '.$typecode_switch.' ';
            }

            if($options['iplimit_switch'] !=$oldconfigpri['iplimit_switch']){
                $iplimit_switch=$options['iplimit_switch']?'开':'关';
                $action.='短信验证码IP限制开关 '.$iplimit_switch.' ';
            }

            if($options['iplimit_times'] !=$oldconfigpri['iplimit_times']){
                $action.='短信验证码IP限制次数 '.$options['iplimit_times'].' ';
            }

            if($options['auth_islimit'] !=$oldconfigpri['auth_islimit']){
                $auth_islimit=$options['auth_islimit']?'开':'关';
                $action.='认证限制 '.$auth_islimit.' ';
            }

            if($options['level_islimit'] !=$oldconfigpri['level_islimit']){
                $level_islimit=$options['level_islimit']?'开':'关';
                $action.='直播等级控制 '.$level_islimit.' ';
            }

            if($options['level_limit'] !=$oldconfigpri['level_limit']){
                $action.='直播限制等级 '.$options['level_limit'].' ';
            }

            if($options['speak_limit'] !=$oldconfigpri['speak_limit']){
                $action.='发言等级限制 '.$options['speak_limit'].' ';
            }


            if($options['barrage_limit'] !=$oldconfigpri['barrage_limit']){
                $action.='弹幕等级限制 '.$options['barrage_limit'].' ';
            }

            if($options['barrage_fee'] !=$oldconfigpri['barrage_fee']){
                $action.='弹幕费用 '.$options['barrage_fee'].' ';
            }

            if($options['userlist_time'] !=$oldconfigpri['userlist_time']){
                $action.='用户列表请求间隔(秒) '.$options['userlist_time'].' ';
            }

            if($options['mic_limit'] !=$oldconfigpri['mic_limit']){
                $action.='连麦等级限制 '.$options['mic_limit'].' ';
            }

            if($options['cdn_switch'] !=$oldconfigpri['cdn_switch']){
                $live_sdk=[
                    '1'=>'声网',
                    '2'=>'腾讯云'
                ];
                $action.='直播CDN '.$live_sdk[$options['cdn_switch']].' ';
            }

            if($options['sw_app_id'] !=$oldconfigpri['sw_app_id']){
                $action.='声网appid '.$options['sw_app_id'].' ';
            }

            if($options['sw_key_id'] !=$oldconfigpri['sw_key_id']){
                $action.='声网Key '.$options['sw_key_id'].' ';
            }

            if($options['sw_key_secret'] !=$oldconfigpri['sw_key_secret']){
                $action.='声网Secret '.$options['sw_key_secret'].' ';
            }

            if($options['sw_push_url'] !=$oldconfigpri['sw_push_url']){
                $action.='声网推流域名 '.$options['sw_push_url'].' ';
            }

            if($options['sw_push_key'] !=$oldconfigpri['sw_push_key']){
                $action.='声网推流防盗链 '.$options['sw_push_key'].' ';
            }

            if($options['sw_push_length'] !=$oldconfigpri['sw_push_length']){
                $action.='声网推流鉴权时长 '.$options['sw_push_length'].' ';
            }

            if($options['tx_play_key_switch'] !=$oldconfigpri['tx_play_key_switch']){
                $tx_play_key_switch=$options['tx_play_key_switch']?'开':'关';
                $action.='是否开启腾讯云播流鉴权 '.$tx_play_key_switch.' ';
            }

            if($options['tx_push'] !=$oldconfigpri['tx_push']){
                $action.='腾讯云直播推流域名 '.$options['tx_push'].' ';
            }

            if($options['tx_pull'] !=$oldconfigpri['tx_pull']){
                $action.='腾讯云直播播流域名 '.$options['tx_pull'].' ';
            }

            if($options['cash_rate'] !=$oldconfigpri['cash_rate']){
                $action.='星币提现比例 '.$options['cash_rate'].' ';
            }

            if($options['cash_take'] !=$oldconfigpri['cash_take']){
                $action.='星币提现抽成'.$options['cash_take'].' ';
            }

            if($options['cash_min'] !=$oldconfigpri['cash_min']){
                $action.='提现最低额度'.$options['cash_min'].'星币 ';
            }

            if($options['cash_start'] !=$oldconfigpri['cash_start'] || $options['cash_end'] !=$oldconfigpri['cash_end']){
                $action.='每月提现期 '.$options['cash_start'].'-'.$options['cash_end'].' ';
            }

            if($options['cash_max_times'] !=$oldconfigpri['cash_max_times']){
                $action.='每月提现次数'.$options['cash_max_times'].' ';
            }

            if($options['letter_switch'] !=$oldconfigpri['letter_switch']){
                $letter_switch=$options['letter_switch']?'开':'关';
                $action.='私信开关 '.$letter_switch.' ';
            }

            if($options['aliapp_switch'] !=$oldconfigpri['aliapp_switch']){
                $aliapp_switch=$options['aliapp_switch']?'开':'关';
                $action.='支付宝APP开关 '.$aliapp_switch.' ';
            }

            if($options['aliapp_pc'] !=$oldconfigpri['aliapp_pc']){
                $aliapp_pc=$options['aliapp_pc']?'开':'关';
                $action.='支付宝PC开关 '.$aliapp_pc.' ';
            }

            if($options['wx_switch_pc'] !=$oldconfigpri['wx_switch_pc']){
                $wx_switch_pc=$options['wx_switch_pc']?'开':'关';
                $action.='微信PC开关 '.$wx_switch_pc.' ';
            }

            if($options['wx_switch'] !=$oldconfigpri['wx_switch']){
                $wx_switch=$options['wx_switch']?'开':'关';
                $action.='微信APP开关 '.$wx_switch.' ';
            }

            if(($options['usdt_switch'] ?? '') !=($oldconfigpri['usdt_switch'] ?? '')){
                $usdt_switch=$options['usdt_switch']?'开':'关';
                $action.='USDT支付开关 '.$usdt_switch.' ';
            }

            if($options['agent_switch'] !=$oldconfigpri['agent_switch']){
                $agent_switch=$options['agent_switch']?'开':'关';
                $action.='邀请开关 '.$agent_switch.' ';
            }

            if($options['distribut1'] !=$oldconfigpri['distribut1']){
                $action.='一级分成 '.$options['distribut1'].' ';
            }

            if($options['video_audit_switch'] !=$oldconfigpri['video_audit_switch']){
                $video_audit_switch=$options['video_audit_switch']?'开':'关';
                $action.='视频审核开关 '.$video_audit_switch.' ';
            }

            if($options['dynamic_auth'] !=$oldconfigpri['dynamic_auth']){
                $dynamic_auth=$options['dynamic_auth']?'开':'关';
                $action.='动态认证开关 '.$dynamic_auth.' ';
            }

            if($options['dynamic_switch'] !=$oldconfigpri['dynamic_switch']){
                $dynamic_switch=$options['dynamic_switch']?'开':'关';
                $action.='动态审核 '.$dynamic_switch.' ';
            }

            if($options['comment_weight'] !=$oldconfigpri['comment_weight']){
                $action.='评论权重值 '.$options['comment_weight'].' ';
            }

            if($options['like_weight'] !=$oldconfigpri['like_weight']){
                $action.='点赞权重值 '.$options['like_weight'].' ';
            }

            if($options['game_switch'] !=$oldconfigpri['game_switch']){
                $action.='游戏开关 '.$options['game_switch'].' ';
            }

            if($options['game_banker_limit'] !=$oldconfigpri['game_banker_limit']){
                $action.='上庄限制 '.$options['game_banker_limit'].' ';
            }

            if($options['game_odds'] !=$oldconfigpri['game_odds']){
                $action.='普通游戏赔率 '.$options['game_odds'].' ';
            }

            if($options['game_odds_p'] !=$oldconfigpri['game_odds_p']){
                $action.='系统坐庄游戏赔率 '.$options['game_odds_p'].' ';
            }

            if($options['game_odds_u'] !=$oldconfigpri['game_odds_u']){
                $action.='用户坐庄游戏赔率 '.$options['game_odds_u'].' ';
            }

            if($options['game_pump'] !=$oldconfigpri['game_pump']){
                $action.='游戏抽水 '.$options['game_pump'].' ';
            }

            if($options['turntable_switch'] !=$oldconfigpri['turntable_switch']){
                $turntable_switch=$options['turntable_switch']?'开':'关';
                $action.='直播间大转盘开关 '.$turntable_switch.' ';
            }

            if($options['watch_live_term'] !=$oldconfigpri['watch_live_term'] || $options['watch_live_coin'] !=$oldconfigpri['watch_live_coin']){

                $action.='观看直播 条件(分钟)：'.$options['watch_live_term'].'奖励(星币)：'.$options['watch_live_coin'].' ';
            }

            if($options['watch_video_term'] !=$oldconfigpri['watch_video_term'] || $options['watch_video_coin'] !=$oldconfigpri['watch_video_coin']){

                $action.='观看视频 条件(分钟)：'.$options['watch_video_term'].'奖励(星币)：'.$options['watch_video_coin'].' ';
            }

            if($options['open_live_term'] !=$oldconfigpri['open_live_term'] || $options['open_live_coin'] !=$oldconfigpri['open_live_coin']){

                $action.='直播奖励 条件(分钟)：'.$options['open_live_term'].'奖励(星币)：'.$options['open_live_coin'].' ';
            }

            if($options['award_live_term'] !=$oldconfigpri['award_live_term'] || $options['award_live_coin'] !=$oldconfigpri['award_live_coin']){

                $action.='打赏奖励 条件(分钟)：'.$options['award_live_term'].'奖励(星币)：'.$options['award_live_coin'].' ';
            }

            if($options['share_live_term'] !=$oldconfigpri['share_live_term'] || $options['share_live_coin'] !=$oldconfigpri['share_live_coin']){

                $action.='分享奖励 条件(分钟)：'.$options['share_live_term'].'奖励(星币)：'.$options['share_live_coin'].' ';
            }

            if($options['video_watermark']!=$options['video_watermark_old']){
                $options['video_watermark']=set_upload_path($options['video_watermark']);
            }

            if(isset($oldconfigpri['dailytask_switch']) && ($options['dailytask_switch']!=$oldconfigpri['dailytask_switch']) && $options['dailytask_switch']==0){ //关闭每日任务开关

                //直播奖励
                $live_key="open_live_daily_tasks_";
                $live_arr=blurrySearch($live_key);
                if(!empty($live_arr)){
                    foreach ($live_arr as $k => $v) {

                        $starttime=getcaches($v);
                        $user_arr=explode($live_key, $v);
                        $uid=$user_arr[1];

                        if($starttime){
                            $endtime=time();  //当前时间
                            $data=[
                                'type'=>'3',
                                'starttime'=>$starttime,
                                'endtime'=>$endtime,
                            ];
                            dailyTasks($uid,$data);
                            //删除当前存入的时间
                            delcache($v);
                        }
                        $starttime=0;
                    }
                }

                //观看视频奖励
                $video_key="watch_video_daily_tasks_";
                $video_arr=blurrySearch($video_key);
                if(!empty($video_arr)){
                    foreach ($video_arr as $k => $v) {

                        $starttime=getcaches($v);
                        $user_arr=explode($video_key, $v);
                        $uid=$user_arr[1];

                        if($starttime){
                            $endtime=time();  //当前时间
                            $data=[
                                'type'=>'2',
                                'starttime'=>$starttime,
                                'endtime'=>$endtime,
                            ];
                            dailyTasks($uid,$data);
                            //删除当前存入的时间
                            delcache($v);
                        }

                        $starttime=0;

                    }
                }

                //观看直播奖励
                $watchlive_key="watch_live_daily_tasks_";
                $watchlive_arr=blurrySearch($watchlive_key);

                if(!empty($watchlive_arr)){
                    foreach ($watchlive_arr as $k => $v) {

                        $starttime=getcaches($v);
                        $user_arr=explode($watchlive_key, $v);
                        $uid=$user_arr[1];

                        if($starttime){
                            $endtime=time();  //当前时间
                            $data=[
                                'type'=>'1',
                                'starttime'=>$starttime,
                                'endtime'=>$endtime,
                            ];

                            dailyTasks($uid,$data);

                            //删除当前存入的时间
                            delcache($v);
                        }

                        $starttime=0;
                    }
                }

            }

            unset($options['video_watermark_old']);

            cmf_set_option('storage', [
                'type'=>'Local',
                'storages'=>[
                    'Local'=>['name'=>'本地']
                ],
            ], true);
            cmf_set_option('configpri', $options,true);
            $this->resetcache('getConfigPri',$options);

            setcaches('sensitive_words',explode(',',$options['sensitive_words']));

            if($action!="修私密配置 "){
                setAdminLog($action);
            }

            $this->success("保存成功！", '');
        }
    }

    protected function resetcache($key='',$info=[]){
        if($key!='' && $info){
            delcache($key);
            setcaches($key,$info);
        }
    }


}
