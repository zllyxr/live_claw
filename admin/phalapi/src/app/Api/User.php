<?php
namespace App\Api;

use PhalApi\Api;
use App\Domain\User as Domain_User;
use App\Domain\Guard as Domain_Guard;
use App\Domain\Video as Domain_Video;
use App\Domain\Cdnrecord as Domain_Cdnrecord;

/**
 * 用户信息
 */
if (!session_id()) session_start();

class User extends Api {
	public function getRules() {
		return array(
			'iftoken' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
			),
			
			'getBaseInfo' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'version_ios' => array('name' => 'version_ios', 'type' => 'string', 'desc' => 'IOS版本号'),
			),
			
			'updateAvatar' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				/*'file' => array('name' => 'file','type' => 'file', 'min' => 0, 'max' => 1024 * 1024 * 30, 'range' => array('image/jpg', 'image/jpeg', 'image/png'), 'ext' => array('jpg', 'jpeg', 'png')),*/
				'avatar' => array('name' => 'avatar', 'type' => 'string',  'desc' => '用户头像地址'),
			),
			
			'updateFields' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'fields' => array('name' => 'fields', 'type' => 'string', 'require' => true, 'desc' => '修改信息，json字符串'),
			),
			
			'updatePass' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'oldpass' => array('name' => 'oldpass', 'type' => 'string', 'require' => true, 'desc' => '旧密码'),
				'pass' => array('name' => 'pass', 'type' => 'string', 'require' => true, 'desc' => '新密码'),
				'pass2' => array('name' => 'pass2', 'type' => 'string', 'require' => true, 'desc' => '确认密码'),
			),
			
			'getBalance' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
                'type' => array('name' => 'type', 'type' => 'string', 'desc' => '设备类型，0android，1IOS'),
                'version_ios' => array('name' => 'version_ios', 'type' => 'string', 'desc' => 'IOS版本号'),
			),
			
			'getProfit' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
			),
			
			'setCash' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'accountid' => array('name' => 'accountid', 'type' => 'int', 'require' => true, 'desc' => '账号ID'),
				'cashvote' => array('name' => 'cashvote', 'type' => 'int', 'require' => true, 'desc' => '提现星币数'),
			),
			
			'setAttent' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'touid' => array('name' => 'touid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '对方ID'),
			),
			
			'isAttent' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'touid' => array('name' => 'touid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '对方ID'),
			),
			
			'isBlacked' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'touid' => array('name' => 'touid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '对方ID'),
			),
			'checkBlack' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'touid' => array('name' => 'touid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '对方ID'),
			),

			'setBlack' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'touid' => array('name' => 'touid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '对方ID'),
			),
			
			'getBindCode' => array(
				'mobile' => array('name' => 'mobile', 'type' => 'string', 'min' => 1, 'require' => true,  'desc' => '手机号'),
			),
			
			'setMobile' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'mobile' => array('name' => 'mobile', 'type' => 'string', 'min' => 1, 'require' => true,  'desc' => '手机号'),
				'code' => array('name' => 'code', 'type' => 'string', 'min' => 1, 'require' => true,   'desc' => '验证码'),
			),
			
			'getFollowsList' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'touid' => array('name' => 'touid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '对方ID'),
				'p' => array('name' => 'p', 'type' => 'int', 'min' => 1, 'default'=>1,'desc' => '页数'),
			),
			
			'getFansList' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'touid' => array('name' => 'touid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '对方ID'),
				'p' => array('name' => 'p', 'type' => 'int', 'min' => 1, 'default'=>1,'desc' => '页数'),
			),
			
			'getBlackList' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'touid' => array('name' => 'touid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '对方ID'),
				'p' => array('name' => 'p', 'type' => 'int', 'min' => 1, 'default'=>1,'desc' => '页数'),
			),
			
			'getLiverecord' => array(
				'touid' => array('name' => 'touid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '对方ID'),
				'p' => array('name' => 'p', 'type' => 'int', 'min' => 1, 'default'=>1,'desc' => '页数'),
			),
			
			'getAliCdnRecord' => array(
                'id' => array('name' => 'id', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '直播记录ID'),
            ),
			
			'getUserHome' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'require' => true, 'desc' => '用户ID'),
				'touid' => array('name' => 'touid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '对方ID'),
			),
			
			'getContributeList' => array(
				'touid' => array('name' => 'touid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '对方ID'),
				'p' => array('name' => 'p', 'type' => 'int', 'default'=>'1' ,'desc' => '页数'),
			),
			
			'getPmUserInfo' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'touid' => array('name' => 'touid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '对方ID'),
			),
			
			'getMultiInfo' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'uids' => array('name' => 'uids', 'type' => 'string', 'min' => 1,'require' => true, 'desc' => '用户ID，多个以逗号分割'),
				'type' => array('name' => 'type', 'type' => 'int', 'require' => true, 'desc' => '关注类型，0 未关注 1 已关注'),
			),
            
            'getUidsInfo' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'uids' => array('name' => 'uids', 'type' => 'string', 'min' => 1,'require' => true, 'desc' => '用户ID，多个以逗号分割'),
			),
			'Bonus' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
			),
            'getBonus' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
			),
			'setDistribut' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'code' => array('name' => 'code', 'type' => 'string', 'require' => true, 'desc' => '邀请码'),
			),

			'getUserLabel' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'touid' => array('name' => 'touid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '对方ID'),
			),
            
            'setUserLabel' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
                'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'touid' => array('name' => 'touid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '对方ID'),
                'labels' => array('name' => 'labels', 'type' => 'string', 'require' => true, 'desc' => '印象标签ID，多个以逗号分割'),
			),

            'getMyLabel' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
                'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
			),

            'getUserAccountList' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
                'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
			),

            'setUserAccount' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
                'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
                'type' => array('name' => 'type', 'type' => 'int', 'require' => true, 'desc' => '账号类型，1表示支付宝，2表示微信，3表示银行卡，4表示USDT.TRC20'),
                'account_bank' => array('name' => 'account_bank', 'type' => 'string', 'default' => '', 'desc' => '银行名称/USDT网络'),
                'account' => array('name' => 'account', 'type' => 'string', 'require' => true, 'desc' => '账号'),
                'name' => array('name' => 'name', 'type' => 'string', 'default' => '', 'desc' => '姓名'),
			),
            
            'delUserAccount' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
                'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
                'id' => array('name' => 'id', 'type' => 'int', 'require' => true, 'desc' => '账号ID'),
			),

			'getAuthInfo'=>array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
			),
			
			'seeDailyTasks'=>array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'liveuid' => array('name' => 'liveuid', 'type' => 'int', 'default' => '0', 'desc' => '主播ID'),
				'islive' => array('name' => 'islive', 'type' => 'int', 'default' => '0',  'desc' => '是否在直播间 0不在 1在'),
			),
			'receiveTaskReward'=>array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'taskid' => array('name' => 'taskid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '任务ID'),
			),

			'setBeautyParams'=>array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'params' => array('name' => 'params', 'type' => 'string', 'require' => true, 'desc' => '用户设置的美颜参数'),
			),

			'getBeautyParams'=>array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
			),

			'getBraintreeToken' => array( 
				'uid' => array('name' => 'uid', 'type' => 'int', 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'desc' => '用户token'),
			),

			'BraintreeCallback'=>array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'ordertype' => array('name' => 'ordertype', 'type' => 'string', 'require' => true, 'desc' => 'order_type 订单类型；coin_charge：星币充值'),
				'orderno' => array('name' => 'orderno', 'type' => 'string',  'require' => true, 'desc' => '系统的订单编号'),
				'nonce' => array('name' => 'nonce', 'type' => 'string', 'require' => true, 'desc' => 'braintree返回的三方订单编号'),
				'money' => array('name' => 'money', 'type' => 'string', 'require' => true, 'desc' => '充值金额'),
				'time' => array('name' => 'time', 'type' => 'string', 'desc' => '时间戳'),
                'sign' => array('name' => 'sign', 'type' => 'string', 'desc' => '签名字符串'),

			),

			'getTurntableWinLists'=>array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'p' => array('name' => 'p', 'type' => 'int', 'default'=>'1' ,'desc' => '页数'),
			),

			'clearTurntableWinLists'=>array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
			),

			'checkTeenager'=>array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
			),

			'setTeenagerPassword'=>array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'password'=>array('name' => 'password', 'type' => 'string', 'require' => true, 'desc' => '青少年模式密码'),
				'type'=>array('name' => 'type', 'type' => 'int', 'require' => true, 'desc' => '操作类型 0 设置密码 1开启青少年模式'),
			),

			'updateTeenagerPassword'=>array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'oldpassword'=>array('name' => 'oldpassword', 'type' => 'string', 'require' => true, 'desc' => '青少年模式旧密码'),
				'password'=>array('name' => 'password', 'type' => 'string', 'require' => true, 'desc' => '青少年模式新密码'),
			),

			'closeTeenager'=>array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'password'=>array('name' => 'password', 'type' => 'string', 'require' => true, 'desc' => '青少年模式密码'),
			),

			'addTeenagerTime'=>array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
			),

			'updateBgImg' => array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
				'img' => array('name' => 'img','type' => 'string','require' => true, 'desc' => '背景图' ),
			),

			'setLiveWindow'=>array(
				'uid' => array('name' => 'uid', 'type' => 'int', 'min' => 1, 'require' => true, 'desc' => '用户ID'),
				'token' => array('name' => 'token', 'type' => 'string', 'require' => true, 'desc' => '用户token'),
			),
			
		);
	}
	/**
	 * 判断token
	 * @desc 用于判断token
	 * @return int code 操作码，0表示成功， 1表示用户不存在
	 * @return array info 
	 * @return string msg 提示信息
	 */
	public function iftoken() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$checkToken=\App\checkToken($this->uid,$this->token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
		return $rs;
	}
	/**
	 * 获取用户信息
	 * @desc 用于获取单个用户基本信息
	 * @return int code 操作码，0表示成功， 1表示用户不存在
	 * @return array info 
	 * @return array info[0] 用户信息
	 * @return int info[0].id 用户ID
	 * @return string info[0].level 等级
	 * @return string info[0].lives 直播数量
	 * @return string info[0].follows 关注数
	 * @return string info[0].fans 粉丝数
	 * @return string info[0].agent_switch 分销开关
	 * @return string info[0].family_switch 家族开关
	 * @return string info[0].live_window 直播小窗状态  0  关闭  1  打开
	 * @return string msg 提示信息
	 */
	public function getBaseInfo() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

		$domain = new Domain_User();
		$info = $domain->getBaseInfo($uid);
        if(!$info){
            $rs['code'] = 700;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
        }
		
		$configpri=\App\getConfigPri();

		$configpub=\App\getConfigPub();
		$agent_switch=$configpri['agent_switch'];
		$family_switch=$configpri['family_switch'];
		$service_switch=$configpri['service_switch'];
		$service_url=$configpri['service_url'];
		$ios_shelves=$configpub['ios_shelves'];
		$dailytask_switch=$configpri['dailytask_switch'];
		
		$info['agent_switch']=$agent_switch;
		$info['family_switch']=$family_switch;

        $info['isauth'] = \App\isAuth($uid);


		/* 个人中心菜单 */
		$version_ios=$this->version_ios;
		$list=array();
		$list1=array();
		$list2=array();
		$shelves=1;
		if($version_ios && $version_ios==$ios_shelves){
			$agent_switch=0;
			$family_switch=0;
			$shelves=0;
		}

		$checkTeenager=$domain->checkTeenager($uid);
		//开启了青少年模式
		if($checkTeenager['info'][0]['is_setpassword']==1 && $checkTeenager['info'][0]['status']==1){

			$list1[]=array(
				'id'=>'23',
				'name'=>\PhalApi\T('动态'),
				'thumb'=>\App\get_upload_path("/static/appapi/images/personal/dynamic.png"),
				'href'=>''
			);

	        if($service_switch && $service_url){
	           $list1[]=array(
	           	'id'=>'21',
	           	'name'=>\PhalApi\T('在线客服'),
	           	'thumb'=>\App\get_upload_path("/static/appapi/images/personal/kefu.png"),
	           	'href'=>$service_url
	           ); 
	        }

			$list[0]['title']=\PhalApi\T('更多服务');
	        $list[0]['list']=$list1;

		}else{

			$list1[]=array(
				'id'=>'19',
				'name'=>\PhalApi\T('视频'),
				'thumb'=>\App\get_upload_path("/static/appapi/images/personal/video.png"),
				'href'=>''
			);

	        $list1[]=array(
	        	'id'=>'23',
	        	'name'=>\PhalApi\T('动态'),
	        	'thumb'=>\App\get_upload_path("/static/appapi/images/personal/dynamic.png"),
	        	'href'=>''
	        );

		        $list1[]=array(
		        	'id'=>'2',
		        	'name'=>\PhalApi\T('道具'),
		        	'thumb'=>\App\get_upload_path("/static/appapi/images/personal/props.png"),
		        	'href'=>''
		        );

	        if($shelves){
				$list1[]=array(
					'id'=>'1',
					'name'=>\PhalApi\T('收益'),
					'thumb'=>\App\get_upload_path("/static/appapi/images/personal/votes.png"),
					'href'=>''
				);
			}

	        //转为app固定
	        //$list1[]=array('id'=>'11','name'=>'认证','thumb'=>\App\get_upload_path("/static/appapi/images/personal/auth.png") ,'href'=>\App\get_upload_path("/appapi/Auth/index"));

			if($dailytask_switch){
	        	$list1[]=array(
	        		'id'=>'25',
	        		'name'=>\PhalApi\T('每日任务'),
	        		'thumb'=>\App\get_upload_path("/static/appapi/images/personal/task.png"),
	        		'href'=>''
	        	);
	        }

		        $list1[]=array('id'=>'20','name'=>\PhalApi\T('房间管理'),'thumb'=>\App\get_upload_path("/static/appapi/images/personal/room.png") ,'href'=>'');


	        
	        
	        
			
			
			if($agent_switch){
				$list2[]=array('id'=>'8','name'=>\PhalApi\T('邀请奖励'),'thumb'=>\App\get_upload_path("/static/appapi/images/personal/agent.png") ,'href'=>\App\get_upload_path("/appapi/Agent/index"));
			}

			$list2[]=array('id'=>'26','name'=>\PhalApi\T('中奖记录'),'thumb'=>\App\get_upload_path("/static/appapi/images/personal/turntable.png") ,'href'=>'');

	        if($service_switch && $service_url){
	           $list2[]=array('id'=>'21','name'=>\PhalApi\T('在线客服'),'thumb'=>\App\get_upload_path("/static/appapi/images/personal/kefu.png") ,'href'=>$service_url); 
	        }
			
			$list[0]['title']=\PhalApi\T('我的服务');
	        $list[0]['list']=$list1;
	        $list[1]['title']=\PhalApi\T('更多服务');
	        $list[1]['list']=$list2;

		}

        

		$info['list']=$list;
		$rs['info'][0] = $info;

		return $rs;
	}

	/**
	 * 头像上传 
	 * @desc 用于用户修改头像
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string list[0].avatar 用户主头像
	 * @return string list[0].avatar_thumb 用户头像缩略图
	 * @return string msg 提示信息
	 */
	public function updateAvatar() {
		$rs = array('code' => 0 , 'msg' => \PhalApi\T('设置头像成功'), 'info' => array());

		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$avatar_str=\App\checkNull($this->avatar);

		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}


		//APP原生上传存储到数据库start

		if(!$avatar_str){
			$rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T('请上传头像');
			return $rs;
		}

		$configpri=\App\getConfigPri();
		$cloudtype=$configpri['cloudtype'];
		if($cloudtype==1){ //七牛云存储
			$avatar= $avatar_str.'?imageView2/2/w/600/h/600'; //600 X 600
			$avatar_thumb= $avatar_str.'?imageView2/2/w/200/h/200'; // 200 X 200
		
		}else{ //亚马逊存储
			$avatar=$avatar_str;
			$avatar_thumb=$avatar_str;
		}

		$data=array(
			"avatar"=>\App\get_upload_path($avatar),
			"avatar_thumb"=>\App\get_upload_path($avatar_thumb),
		);


		$data2=array(
			"avatar"=>$avatar,
			"avatar_thumb"=>$avatar_thumb,
		);

		//APP原生上传存储到数据库end


		// 清除缓存
		\App\delCache("userinfo_".$uid);
		
		$domain = new Domain_User();
		$info = $domain->userUpdate($uid,$data2);

		$rs['info'][0] = $data;


		return $rs;

	}
	
	/**
	 * 修改用户信息
	 * @desc 用于修改用户信息
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string list[0].msg 修改成功提示信息 
	 * @return string msg 提示信息
	 */
	public function updateFields() {
		$rs = array('code' => 0, 'msg' => \PhalApi\T('修改成功'), 'info' => array());
		
		$checkToken=\App\checkToken($this->uid,$this->token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
		$fields=json_decode($this->fields,true);
		
        $allow=['user_nickname','sex','signature','birthday','location'];
		$domain = new Domain_User();
		foreach($fields as $k=>$v){
            if(in_array($k,$allow)){
                $fields[$k]=\App\checkNull($v);
            }else{
                unset($fields[$k]);
            }
			
		}
		
		if(array_key_exists('user_nickname', $fields)){
			if($fields['user_nickname']==''){
				$rs['code'] = 1002;
				$rs['msg'] = \PhalApi\T('昵称不能为空');
				return $rs;
			}
			$isexist = $domain->checkName($this->uid,$fields['user_nickname']);
			if(!$isexist){
				$rs['code'] = 1002;
				$rs['msg'] = \PhalApi\T('昵称重复,请修改');
				return $rs;
			}



			if(strstr($fields['user_nickname'], '已注销')!==false){ //昵称包含已注销三个字
				$rs['code'] = 10011;
				$rs['msg'] = \PhalApi\T('输入非法，请重新输入');
				return $rs;
			}

			if(mb_substr($fields['user_nickname'], 0,1)=='='){
				$rs['code'] = 10011;
				$rs['msg'] = \PhalApi\T('输入非法，请重新输入');
				return $rs;
			}

            $sensitivewords=\App\sensitiveField($fields['user_nickname']);
			if($sensitivewords==1001){
				$rs['code'] = 10011;
				$rs['msg'] = \PhalApi\T('输入非法，请重新输入');
				return $rs;
			}
		}
		if(array_key_exists('signature', $fields)){
			$sensitivewords==\App\sensitiveField($fields['signature']);
			if($sensitivewords==1001){
				$rs['code'] = 10011;
				$rs['msg'] = \PhalApi\T('输入非法，请重新输入');
				return $rs;
			}
		}
        
        if(array_key_exists('birthday', $fields)){
			$fields['birthday']=strtotime($fields['birthday']);
		}
        
		$info = $domain->userUpdate($this->uid,$fields);
	 
		if($info===false){
			$rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T('修改失败');
			return $rs;
		}
		/* 清除缓存 */
		\App\delCache("userinfo_".$this->uid);
		$rs['info'][0]['msg']=\PhalApi\T('修改成功');
		return $rs;
	}

	/**
	 * 修改密码
	 * @desc 用于修改用户密码
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string list[0].msg 修改成功提示信息
	 * @return string msg 提示信息
	 */
	public function updatePass() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$oldpass=\App\checkNull($this->oldpass);
		$pass=\App\checkNull($this->pass);
		$pass2=\App\checkNull($this->pass2);
		
		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
		
		if($pass != $pass2){
			$rs['code'] = 1002;
			$rs['msg'] = \PhalApi\T('两次新密码不一致');
			return $rs;
		}
		
		$check = \App\passcheck($pass);
		if(!$check ){
			$rs['code'] = 1004;
			$rs['msg'] = \PhalApi\T('密码为6-20位字母数字组合');
			return $rs;										
		}
		
		$domain = new Domain_User();
		$info = $domain->updatePass($uid,$oldpass,$pass);
	 
		if($info==1003){
			$rs['code'] = 1003;
			$rs['msg'] = \PhalApi\T('旧密码错误');
			return $rs;
		}else if($info===false){
			$rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T('修改失败');
			return $rs;
		}

		$rs['info'][0]['msg']=\PhalApi\T('修改成功');
		return $rs;
	}	
	
	/**
	 * 我的星币
	 * @desc 用于获取用户余额,充值规则 支付方式信息
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[0].coin 用户星币余额
	 * @return array info[0].rules 充值规则
	 * @return string info[0].rules[].id 充值规则
	 * @return string info[0].rules[].coin 星币
	 * @return string info[0].rules[].money 价格
	 * @return string info[0].rules[].money_ios 兼容字段，等于星币价格
	 * @return string info[0].rules[].product_id 苹果项目ID
	 * @return string info[0].rules[].give 兼容字段，固定为0
	 * @return string info[0].aliapp_switch 支付宝开关，0表示关闭，1表示开启
	 * @return string info[0].aliapp_partner 支付宝合作者身份ID
	 * @return string info[0].aliapp_seller_id 支付宝帐号	
	 * @return string info[0].aliapp_key_android 支付宝安卓密钥
	 * @return string info[0].aliapp_key_ios 支付宝苹果密钥
	 * @return string info[0].wx_switch 微信支付开关，0表示关闭，1表示开启
	 * @return string info[0].wx_appid 开放平台账号AppID
	 * @return string info[0].wx_appsecret 微信应用appsecret
	 * @return string info[0].wx_mchid 微信商户号mchid
	 * @return string info[0].wx_key 微信密钥key
	 * @return string msg 提示信息
	 */
	public function getBalance() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
        
        $uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);
        $type=\App\checkNull($this->type);
        $version_ios=\App\checkNull($this->version_ios);
		
		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
		
		$domain = new Domain_User();
		$info = $domain->getBalance($uid);
		
		$key='getChargeRules';
		$rules=\App\getcaches($key);


		if(!$rules){
			$rules= $domain->getChargeRules();
			\App\setcaches($key,$rules);
		}
		$info['rules'] =$rules;
		
		$configpub=\App\getConfigPub();
		$configpri=\App\getConfigPri();
		
		$aliapp_switch=$configpri['aliapp_switch'];
		
		$info['aliapp_switch']=$aliapp_switch;
		$info['aliapp_partner']=$aliapp_switch==1?$configpri['aliapp_partner']:'';
		$info['aliapp_seller_id']=$aliapp_switch==1?$configpri['aliapp_seller_id']:'';
		$info['aliapp_key_android']=$aliapp_switch==1?$configpri['aliapp_key_android']:'';
		$info['aliapp_key_ios']=$aliapp_switch==1?$configpri['aliapp_key_ios']:'';

        $wx_switch=$configpri['wx_switch'];
		$info['wx_switch']=$wx_switch;
		$info['wx_appid']=$wx_switch==1?$configpri['wx_appid']:'';
		$info['wx_appsecret']=$wx_switch==1?$configpri['wx_appsecret']:'';
		$info['wx_mchid']=$wx_switch==1?$configpri['wx_mchid']:'';
		$info['wx_key']=$wx_switch==1?$configpri['wx_key']:'';

		/*【原Paypal支付因无法使用已废弃】
		$paypal_switch=$configpri['paypal_switch'];
		$info['paypal_switch']=$paypal_switch;
		$info['paypal_sandbox']=$configpri['paypal_sandbox'];
		$info['sandbox_clientid']=$configpri['sandbox_clientid'];//Paypal支付沙盒客户端ID
		$info['product_clientid']=$configpri['product_clientid'];//Paypal支付生产客户端ID
		*/

		$braintree_paypal_switch=$configpri['braintree_paypal_switch'];
		$bepusdt=\App\getBepusdtConfig($configpri);
		$usdt_switch=!empty($bepusdt['enabled']) ? '1' : '0';
		$info['usdt_switch']=$usdt_switch;

        $wx_mini_switch=$configpri['wx_mini_switch'];
        $info['wx_mini_switch']=$wx_mini_switch;

        /* 支付列表 */
        $shelves=1;
        $ios_shelves=$configpub['ios_shelves'];
        if($version_ios && $version_ios==$ios_shelves){
			$shelves=0;
		}
        
        $paylist=[];
        
        if($aliapp_switch && $shelves){
            $paylist[]=[
                'id'=>'ali',
                'name'=>\PhalApi\T('支付宝支付'),
                'thumb'=>\App\get_upload_path("/static/app/pay/ali.png"),
                'href'=>'',
            ];
        }
        
        if($wx_switch && $shelves){
            $paylist[]=[
                'id'=>'wx',
                'name'=>\PhalApi\T('微信支付'),
                'thumb'=>\App\get_upload_path("/static/app/pay/wx.png"),
                'href'=>'',
            ];
        }

        /*【原Paypal支付因无法使用已废弃】
        if($paypal_switch && $shelves){
            $paylist[]=[
                'id'=>'paypal',
                'name'=>\PhalApi\T('Paypal支付'),
                'thumb'=>\App\get_upload_path("/static/app/pay/paypal.png"),
                'href'=>'',
            ];
        }*/

        if($braintree_paypal_switch && $shelves){
            $paylist[]=[
                'id'=>'paypal',
                'name'=>\PhalApi\T('Paypal支付'),
                'thumb'=>\App\get_upload_path("/static/app/pay/paypal.png"),
                'href'=>'',
            ];
        }

        if($usdt_switch && $shelves){
            $paylist[]=[
                'id'=>'usdt',
                'name'=>'USDT',
                'thumb'=>\App\get_upload_path("/static/app/pay/coin.png"),
                'href'=>'',
            ];
        }

        $info['paylist'] =$paylist;
        $info['tip_t'] =$configpub['name_coin'].'/'.$configpub['name_score'].\PhalApi\T('说明:');
        $info['tip_d'] =\PhalApi\T('{coin}可通过平台提供的支付方式进行充值获得，{coin}适用于平台内所有消费；{score} 可通过直播间内游戏奖励获得，所得{score}可用于平台内兑换会员、坐骑、靓号等服务，不可提现。',['coin'=>$configpub['name_coin'],'score'=>$configpub['name_score']]);
        
        
     
		$rs['info'][0]=$info;
		return $rs;
	}
	
	/**
	 * 我的收益
	 * @desc 用于获取用户收益，包括可体现金额，今日可提现金额
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[0].votes 可提取星币数
	 * @return string info[0].votestotal 总星币
	 * @return string info[0].cash_rate 兼容字段，固定为1
	 * @return string info[0].total 可提现星币
	 * @return string info[0].tips 温馨提示
	 * @return string msg 提示信息
	 */
	public function getProfit() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);

		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		} 
		
		$domain = new Domain_User();
		$info = $domain->getProfit($uid);
	 
		$rs['info'][0]=$info;
		return $rs;
	}	
	
	/**
	 * 用户提现
	 * @desc 用于进行用户提现
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[0].msg 提现成功信息
	 * @return string msg 提示信息
	 */
	public function setCash() {
		$rs = array('code' => 0, 'msg' => \PhalApi\T('提现成功'), 'info' => array());
        
        $uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);		
        $accountid=\App\checkNull($this->accountid);		
        $cashvote=\App\checkNull($this->cashvote);		
        
		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
        
        if(!$accountid){
            $rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T('请选择提现账号');
			return $rs;
        }
        
        if(!$cashvote){
            $rs['code'] = 1002;
			$rs['msg'] = \PhalApi\T('请输入有效的提现星币数');
			return $rs;
        }
		
        $data=array(
            'uid'=>$uid,
            'accountid'=>$accountid,
            'cashvote'=>$cashvote,
        );
        $config=\App\getConfigPri();
		$domain = new Domain_User();
		$info = $domain->setCash($data);
		if($info==1001){
			$rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T('您输入的金额大于可提现金额');
			return $rs;
		}else if($info==1003){
			$rs['code'] = 1003;
			$rs['msg'] = \PhalApi\T('请先进行身份认证');
			return $rs;
		}else if($info==1004){
			$rs['code'] = 1004;
			$rs['msg'] = \PhalApi\T('提现最低额度为{num}星币',['num'=>$config['cash_min']]);
			return $rs;
		}else if($info==1005){
			$rs['code'] = 1005;
			$rs['msg'] = \PhalApi\T('不在提现期限内，不能提现');
			return $rs;
		}else if($info==1006){
			$rs['code'] = 1006;
			$rs['msg'] = \PhalApi\T('每月只可提现{num}次,已达上限',['num'=>$config['cash_max_times']]);
			return $rs;
		}else if($info==1007){
			$rs['code'] = 1007;
			$rs['msg'] = \PhalApi\T('提现账号信息不正确');
			return $rs;
		}else if(!$info){
			$rs['code'] = 1002;
			$rs['msg'] = \PhalApi\T('提现失败，请重试');
			return $rs;
		}
	 
		$rs['info'][0]['msg']=\PhalApi\T('提现成功');
		return $rs;
	}		
	/**
	 * 判断是否关注
	 * @desc 用于判断是否关注
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[0].isattent 关注信息，0表示未关注，1表示已关注
	 * @return string msg 提示信息
	 */
	public function isAttent() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$touid=\App\checkNull($this->touid);
		$info = \App\isAttention($uid,$touid);
	 
		$rs['info'][0]['isattent']=(string)$info;
		return $rs;
	}			
	
	/**
	 * 关注/取消关注
	 * @desc 用于关注/取消关注
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[0].isattent 关注信息，0表示未关注，1表示已关注
	 * @return string msg 提示信息
	 */
	public function setAttent() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$touid=\App\checkNull($this->touid);

		if($uid==$touid){
			$rs['code']=1001;
			$rs['msg']=\PhalApi\T('不能关注自己');
			return $rs;	
		}
		$domain = new Domain_User();
		$info = $domain->setAttent($uid,$touid);
	 
		$rs['info'][0]['isattent']=(string)$info;
		return $rs;
	}			
	
	/**
	 * 判断是否拉黑
	 * @desc 用于判断是否拉黑
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[0].isattent  拉黑信息,0表示未拉黑，1表示已拉黑
	 * @return string msg 提示信息
	 */
	public function isBlacked() {
			$rs = array('code' => 0, 'msg' => '', 'info' => array());
			
			$uid=\App\checkNull($this->uid);
			$touid=\App\checkNull($this->touid);

			$info = \App\isBlack($uid,$touid);
		 
			$rs['info'][0]['isblack']=(string)$info;
			return $rs;
	}	

	/**
	 * 检测拉黑状态
	 * @desc 用于私信聊天时判断私聊双方的拉黑状态
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[0].u2t  是否拉黑对方,0表示未拉黑，1表示已拉黑
	 * @return string info[0].t2u  是否被对方拉黑,0表示未拉黑，1表示已拉黑
	 * @return string msg 提示信息
	 */
	public function checkBlack() {
			$rs = array('code' => 0, 'msg' => '', 'info' => array());

			$uid=\App\checkNull($this->uid);
			$touid=\App\checkNull($this->touid);

			//判断对方是否已注销
			$is_destroy=\App\checkIsDestroyByUid($touid);
			if($is_destroy){
				$rs['code']=1001;
				$rs['msg']=\PhalApi\T('对方已注销');
				return $rs;
			}
			
			$u2t = \App\isBlack($uid,$touid);
			$t2u = \App\isBlack($touid,$uid);
		 
			$rs['info'][0]['u2t']=(string)$u2t;
			$rs['info'][0]['t2u']=(string)$t2u;
			return $rs;
	}			
		
	/**
	 * 拉黑/取消拉黑
	 * @desc 用于拉黑/取消拉黑
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[0].isblack 拉黑信息,0表示未拉黑，1表示已拉黑
	 * @return string msg 提示信息
	 */
	public function setBlack() {
			$rs = array('code' => 0, 'msg' => '', 'info' => array());
			
			$uid=\App\checkNull($this->uid);
			$touid=\App\checkNull($this->touid);

			$domain = new Domain_User();
			$info = $domain->setBlack($uid,$touid);
		 
			$rs['info'][0]['isblack']=(string)$info;
			return $rs;
	}		
	
	/**
	 * 获取找回密码短信验证码
	 * @desc 用于找回密码获取短信验证码
	 * @return int code 操作码，0表示成功,2发送失败
	 * @return array info 
	 * @return array info[0]  
	 * @return string msg 提示信息
	 */
	 
	public function getBindCode() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$mobile = \App\checkNull($this->mobile);
		
		$ismobile=\App\checkMobile($mobile);
		if(!$ismobile){
			$rs['code']=1001;
			$rs['msg']=\PhalApi\T('请输入正确的手机号');
			return $rs;	
		}

		if($_SESSION['set_mobile']==$mobile && $_SESSION['set_mobile_expiretime']> time() ){
			$rs['code']=1002;
			$rs['msg']=\PhalApi\T('验证码5分钟有效，请勿多次发送');
			return $rs;
		}

		$mobile_code = \App\random(6,1);
		
		/* 发送验证码 */
		$result=\App\sendCode($mobile,$mobile_code);
		if($result['code']===0){
			$_SESSION['set_mobile'] = $mobile;
			$_SESSION['set_mobile_code'] = $mobile_code;
			$_SESSION['set_mobile_expiretime'] = time() +60*5;	
		}else if($result['code']==667){
			$_SESSION['set_mobile'] = $mobile;
            $_SESSION['set_mobile_code'] = $result['msg'];
            $_SESSION['set_mobile_expiretime'] = time() +60*5;
            
            $rs['code']=1002;
			$rs['msg']=\PhalApi\T('验证码为：').$result['msg'];
            
		}else{
			$rs['code']=1002;
			$rs['msg']=$result['msg'];
		}

		
		return $rs;
	}		

	/**
	 * 绑定手机号
	 * @desc 用于用户绑定手机号
	 * @return int code 操作码，0表示成功，非0表示有错误
	 * @return array info 
	 * @return object info[0].msg 绑定成功提示
	 * @return string msg 提示信息
	 */
	public function setMobile() {

		$rs = array('code' => 0, 'msg' => '', 'info' => array());

		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$mobile=\App\checkNull($this->mobile);
		$code=\App\checkNull($this->code);

		if($mobile!=$_SESSION['set_mobile']){
			$rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T('手机号码不一致');
			return $rs;					
		}

		if($code!=$_SESSION['set_mobile_code']){
			$rs['code'] = 1002;
			$rs['msg'] = \PhalApi\T('验证码错误');
			return $rs;					
		}	

		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
			
		$domain = new Domain_User();

		//更新数据库
		$data=array("mobile"=>$mobile);
		$result = $domain->userUpdate($uid,$data);
		if($result===false){
			$rs['code'] = 1003;
			$rs['msg'] = \PhalApi\T('绑定失败');
			return $rs;
		}
	
		$rs['info'][0]['msg'] = \PhalApi\T('绑定成功');

		return $rs;
	}		
	
	/**
	 * 关注列表
	 * @desc 用于获取用户的关注列表
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[].isattent 是否关注,0表示未关注，1表示已关注
	 * @return string msg 提示信息
	 */
	public function getFollowsList() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$touid=\App\checkNull($this->touid);
		$p=\App\checkNull($this->p);

		$domain = new Domain_User();
		$info = $domain->getFollowsList($uid,$touid,$p);
	 
		$rs['info']=$info;
		return $rs;
	}		
	
	/**
	 * 粉丝列表
	 * @desc 用于获取用户的粉丝列表
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[].isattent 是否关注,0表示未关注，1表示已关注
	 * @return string msg 提示信息
	 */
	public function getFansList() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$touid=\App\checkNull($this->touid);
		$p=\App\checkNull($this->p);

		$domain = new Domain_User();
		$info = $domain->getFansList($uid,$touid,$p);
	 
		$rs['info']=$info;
		return $rs;
	}	

	/**
	 * 黑名单列表
	 * @desc 用于获取用户的黑名单列表
	 * @return int code 操作码，0表示成功
	 * @return array info 用户基本信息
	 * @return string msg 提示信息
	 */
	public function getBlackList() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$touid=\App\checkNull($this->touid);
		$p=\App\checkNull($this->p);

		$domain = new Domain_User();
		$info = $domain->getBlackList($uid,$touid,$p);
	 
		$rs['info']=$info;
		return $rs;
	}		
	
	/**
	 * 直播记录
	 * @desc 用于获取用户的直播记录
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[].nums 观看人数
	 * @return string info[].datestarttime 格式化的开播时间
	 * @return string info[].dateendtime 格式化的结束时间
	 * @return string info[].video_url 回放地址
	 * @return string info[].file_id 回放标示
	 * @return string msg 提示信息
	 */
	public function getLiverecord() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$touid=\App\checkNull($this->touid);
		$p=\App\checkNull($this->p);

		$domain = new Domain_User();
		$info = $domain->getLiverecord($touid,$p);
	 
		$rs['info']=$info;
		return $rs;
	}	

    /**
     *获取阿里云cdn录播地址
     *@desc 如果使用的阿里云cdn，则使用该接口获取录播地址
     *@return int code 操作码，0表示成功
     *@return string info[0].url 录播视频地址
	 * @return string msg 提示信息
    */		
    public function getAliCdnRecord(){
        $rs = array('code' => 0,'msg' => '', 'info' => array());

        $id=\App\checkNull($this->id);
        $domain = new Domain_Cdnrecord();
        $info = $domain->getCdnRecord($id);
        
        if(!$info['video_url']){
            $rs['code']=1002;
            $rs['msg']=\PhalApi\T('直播回放不存在');
            return $rs;
        }

        $rs['info'][0]['url']=$info['video_url'];

        return $rs;
    }	


	/**
	 * 个人主页 
	 * @desc 用于获取个人主页数据
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[0].follows 关注数
	 * @return string info[0].fans 粉丝数
	 * @return string info[0].isattention 是否关注，0表示未关注，1表示已关注
	 * @return string info[0].isblack 我是否拉黑对方，0表示未拉黑，1表示已拉黑
	 * @return string info[0].isblack2 对方是否拉黑我，0表示未拉黑，1表示已拉黑
	 * @return array info[0].contribute 贡献榜前三
	 * @return array info[0].contribute[].avatar 头像
	 * @return string info[0].islive 是否正在直播，0表示未直播，1表示直播
	 * @return string info[0].videonums 视频数
	 * @return string info[0].livenums 直播数
	 * @return array info[0].liverecord 直播记录
	 * @return array info[0].label 印象标签
		 * @return string msg 提示信息
	 */
	public function getUserHome() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
        $uid=\App\checkNull($this->uid);
        $touid=\App\checkNull($this->touid);
        
		$domain = new Domain_User();
		$info=$domain->getUserHome($uid,$touid);
        
        /* 守护 */
        $data=array(
			"liveuid"=>$touid,
		);

		$domain_guard = new Domain_Guard();
		$guardlist = $domain_guard->getGuardList($data);
        
        $info['guardlist']=array_slice($guardlist,0,3);
        
        /* 标签 */
        $key="getMyLabel_".$touid;
        $label=\App\getcaches($key);
        if(!$label){
            $label = $domain->getMyLabel($touid);
            \App\setcaches($key,$label); 
        }
        
        $labels=array_slice($label,0,3);

        //语言包
        $language=\PhalApi\DI()->language;
        foreach ($labels as $k => $v) {
        	if($language=='en'){
        		$v['name']=$v['name_en'];
        	}

        	$labels[$k]=$v;
        }
        
        $info['label']=$labels;
        
        /* 视频 */
        $domain_video = new Domain_Video();
		$video = $domain_video->getHomeVideo($uid,$touid,1);
        
        $info['videolist']=$video;
        
			$rs['info'][0]=$info;
		return $rs;
	}		

	/**
	 * 贡献榜 
	 * @desc 用于获取贡献榜
	 * @return int code 操作码，0表示成功
	 * @return array info 排行榜列表
	 * @return string info[].total 贡献总数
	 * @return string info[].userinfo 用户信息
	 * @return string msg 提示信息
	 */
	public function getContributeList() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$touid=\App\checkNull($this->touid);
		$p=\App\checkNull($this->p);

		$domain = new Domain_User();
		$info=$domain->getContributeList($touid,$p);
		
		$rs['info']=$info;
		return $rs;
	}	
	
	/**
     * 私信用户信息
     * @desc 用于获取其他用户基本信息
     * @return int code 操作码，0表示成功，1表示用户不存在
     * @return array info   
     * @return string info[0].id 用户ID
     * @return string info[0].isattention 我是否关注对方，0未关注，1已关注
     * @return string info[0].isattention2 对方是否关注我，0未关注，1已关注
     * @return string msg 提示信息
     */
    public function getPmUserInfo() {
        $rs = array('code' => 0, 'msg' => '', 'info' => array());

        $uid=\App\checkNull($this->uid);
		$touid=\App\checkNull($this->touid);

        $info = \App\getUserInfo($touid);
		 if (empty($info)) {
            $rs['code'] = 1001;
            $rs['msg'] = \PhalApi\T('用户不存在');
            return $rs;
        }
        $info['isattention2']= (string)\App\isAttention($touid,$uid);
        $info['isattention']= (string)\App\isAttention($uid,$touid);
       
        $rs['info'][0] = $info;

        return $rs;
    }		

	/**
	 * 获取多用户信息 
	 * @desc 用于获取获取多用户信息
	 * @return int code 操作码，0表示成功
	 * @return array info 排行榜列表
	 * @return string info[].utot 是否关注，0未关注，1已关注
	 * @return string info[].ttou 对方是否关注我，0未关注，1已关注
	 * @return string msg 提示信息
	 */
	public function getMultiInfo() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());

		$uid=\App\checkNull($this->uid);
		$uids=\App\checkNull($this->uids);
		$type=\App\checkNull($this->type);
        
        $configpri=\App\getConfigPri();
        
        if($configpri['letter_switch']!=1){
            return $rs;
        }
		
		$uids=explode(",",$uids);

		foreach ($uids as $k=>$userId) {
			if($userId){
				$userinfo= \App\getUserInfo($userId);
				if($userinfo){
					$userinfo['utot']= \App\isAttention($uid,$userId);
					
					$userinfo['ttou']= \App\isAttention($userId,$uid);
					
					if($userinfo['utot']==$type){						
						$rs['info'][]=$userinfo;
					}												
				}					
			}
		}

		return $rs;
	}	

	/**
	 * 获取多用户信息(不区分是否关注)
	 * @desc 用于获取多用户信息
	 * @return int code 操作码，0表示成功
	 * @return array info 排行榜列表
	 * @return string info[].utot 是否关注，0未关注，1已关注
	 * @return string info[].ttou 对方是否关注我，0未关注，1已关注
	 * @return string msg 提示信息
	 */
	public function getUidsInfo() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$uids=\App\checkNull($this->uids);
		$uids=explode(",",$uids);

		foreach ($uids as $k=>$userId) {
			if($userId){
				$userinfo= \App\getUserInfo($userId);
				if($userinfo){
					$userinfo['utot']= \App\isAttention($uid,$userId);
					
					$userinfo['ttou']= \App\isAttention($userId,$uid);					
                    
                    $rs['info'][]=$userinfo;
											
				}					
			}
		}

		return $rs;
	}	

	/**
	 * 登录奖励
	 * @desc 用于用户登录奖励
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[0].bonus_switch 登录开关，0表示未开启
	 * @return string info[0].bonus_day 登录天数,0表示已奖励
	 * @return string info[0].count_day 连续登陆天数
	 * @return string info[0].bonus_list 登录奖励列表
	 * @return string info[0].bonus_list[].day 登录天数
	 * @return string info[0].bonus_list[].coin 登录奖励
	 * @return string msg 提示信息
	 */
	public function Bonus() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);

        $checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
		$domain = new Domain_User();
		$info=$domain->LoginBonus($uid);

		$rs['info'][0]=$info;

		return $rs;
	}		
    
	/**
	 * 登录奖励
	 * @desc 用于用户登录奖励
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[0].bonus_switch 登录开关，0表示未开启
	 * @return string info[0].bonus_day 登录天数,0表示已奖励
	 * @return string msg 提示信息
	 */
	public function getBonus() {
		$rs = array('code' => 0, 'msg' => \PhalApi\T('领取成功'), 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
        
        $checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
		$domain = new Domain_User();
		$info=$domain->getLoginBonus($uid);

		if(!$info){
            $rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T('领取失败');
			return $rs;
        }

		return $rs;
	}
	
	/**
	 * 设置分销上级 
	 * @desc 用于用户首次登录设置分销关系
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[0].msg 提示信息
	 * @return string msg 提示信息
	 */
	public function setDistribut() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$code=\App\checkNull($this->code);
		
		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
		
		if($code==''){
			$rs['code']=1001;
			$rs['msg']=\PhalApi\T('请输入邀请码');
			return $rs;
		}
		
		$domain = new Domain_User();
		$info=$domain->setDistribut($uid,$code);
		if($info==1004){
			$rs['code']=1004;
			$rs['msg']=\PhalApi\T('已设置，不能更改');
			return $rs;
		}
        
		if($info==1002){
			$rs['code']=1002;
			$rs['msg']=\PhalApi\T('邀请码错误');
			return $rs;
		}
        
        if($info==1003){
			$rs['code']=1003;
			$rs['msg']=\PhalApi\T('不能填写自己下级的邀请码');
			return $rs;
		}
		
		$rs['info'][0]['msg']=\PhalApi\T('设置成功');

		return $rs;
	}	

	/**
	 * 获取用户间印象标签 
	 * @desc 用于获取用户间印象标签
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[].id 标签ID
	 * @return string info[].name 名称
	 * @return string info[].colour 色值
	 * @return string info[].ifcheck 是否选择
	 * @return string msg 提示信息
	 */
	public function getUserLabel() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
        
        $uid=\App\checkNull($this->uid);
        $touid=\App\checkNull($this->touid);
        
        $key="getUserLabel_".$uid.'_'.$touid;
		$label=\App\getcaches($key);

		if(!$label){
            $domain = new Domain_User();
			$info = $domain->getUserLabel($uid,$touid);
            $label=$info['label'];
			\App\setcaches($key,$label); 
		}
        
        $label_check=preg_split('/,|，/',$label);
		
        $label_check=array_filter($label_check);
        
        $label_check=array_values($label_check);
        
        
        $key2="getImpressionLabel";
		$label_list=\App\getcaches($key2);
		if(!$label_list){
            $domain = new Domain_User();
			$label_list = $domain->getImpressionLabel();
		}
        
        //语言包
        $language=\PhalApi\DI()->language;
        foreach($label_list as $k=>$v){
            $ifcheck='0';
            if(in_array($v['id'],$label_check)){
                $ifcheck='1';
            }
            $label_list[$k]['ifcheck']=$ifcheck;

            if($language=='en'){
            	$label_list[$k]['name']=$v['name_en'];
            }
        }
        
		$rs['info']=$label_list;

		return $rs;
	}	


	/**
	 * 获取用户间印象标签 
	 * @desc 用于获取用户间印象标签
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[].id 标签ID
	 * @return string info[].name 名称
	 * @return string info[].colour 色值
	 * @return string msg 提示信息
	 */
	public function setUserLabel() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
        
        $uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);
        $touid=\App\checkNull($this->touid);
        $labels=\App\checkNull($this->labels);
        
        $checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
        
        if($uid==$touid){
            $rs['code'] = 1003;
			$rs['msg'] = \PhalApi\T('不能给自己设置标签');
			return $rs;
        }
        
        if($labels==''){
            $rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T('请选择印象');
			return $rs;
        }
        
        $labels_a=preg_split('/,|，/',$labels);
        $labels_a=array_filter($labels_a);
        $nums=count($labels_a);
        if($nums>3){
            $rs['code'] = 1002;
			$rs['msg'] = \PhalApi\T('最多只能选择{num}个印象',['num'=>3]);
			return $rs;
        }
        

        $domain = new Domain_User();
        $result = $domain->setUserLabel($uid,$touid,$labels);

        if($result){
            $key="getUserLabel_".$uid.'_'.$touid;
            \App\setcaches($key,$labels); 
            
            $key2="getMyLabel_".$touid;
            \App\delCache($key2);
        }

		
		$rs['msg']=\PhalApi\T('设置成功');

		return $rs;
	}	


	/**
	 * 获取自己所有的印象标签 
	 * @desc 用于获取自己所有的印象标签
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[].id 标签ID
	 * @return string info[].name 名称
	 * @return string info[].colour 色值
	 * @return string info[].nums 数量
	 * @return string msg 提示信息
	 */
	public function getMyLabel() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
        
        $uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);

        
        $checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
    
        $key="getMyLabel_".$uid;
		$info=\App\getcaches($key);
		
		if(!$info){
            $domain = new Domain_User();
            $info = $domain->getMyLabel($uid);
			

			\App\setcaches($key,$info); 
		}

		//语言包
		$language=\PhalApi\DI()->language;
		foreach ($info as $k => $v) {
			if($language=='en'){
				$v['name']=$v['name_en'];
			}

			$info[$k]=$v;
		}

		$rs['info']=$info;

		return $rs;
	}	
    

	/**
	 * 获取个性设置列表 
	 * @desc 用于获取个性设置列表
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string msg 提示信息
	 */
	public function getPerSetting() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());

        $domain = new Domain_User();
        $info = $domain->getPerSetting();

        $info[]=array('id'=>'17','name'=>\PhalApi\T('意见反馈'),'thumb'=>'' ,'href'=>\App\get_upload_path('/appapi/feedback/index'));
        $info[]=array('id'=>'15','name'=>\PhalApi\T('修改密码'),'thumb'=>'' ,'href'=>'');
        $info[]=array('id'=>'18','name'=>\PhalApi\T('清除缓存'),'thumb'=>'' ,'href'=>'');
        $info[]=array('id'=>'19','name'=>\PhalApi\T('注销账号'),'thumb'=>'' ,'href'=>\App\get_upload_path('/portal/page/index?id=44'));
        $info[]=array('id'=>'16','name'=>\PhalApi\T('检查更新'),'thumb'=>'' ,'href'=>'');
        

		$rs['info']=$info;

		return $rs;
	}	

	/**
	 * 获取用户提现账号 
	 * @desc 用于获取用户提现账号
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string info[].id 账号ID
	 * @return string info[].type 账号类型
	 * @return string info[].account_bank 银行名称/USDT网络
	 * @return string info[].account 账号
	 * @return string info[].name 姓名
	 * @return string msg 提示信息
	 */
	public function getUserAccountList() {
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
        
        $uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);

        
        $checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}        
    

        $domain = new Domain_User();
        $info = $domain->getUserAccountList($uid);

		$rs['info']=$info;

		return $rs;
	}	

	/**
	 * 添加提现账号 
	 * @desc 用于添加提现账号
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string msg 提示信息
	 */
	public function setUserAccount() {
		$rs = array('code' => 0, 'msg' => \PhalApi\T('添加成功'), 'info' => array());
        
        $uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);
        
        $type=\App\checkNull($this->type);
        $account_bank=\App\checkNull($this->account_bank);
        $account=\App\checkNull($this->account);
        $name=\App\checkNull($this->name);

        $type=(int)$type;
        if(!in_array($type,[1,2,3,4],true)){
            $rs['code'] = 1001;
            $rs['msg'] = \PhalApi\T('账号类型不正确');
            return $rs;
        }

        if($type==3){
            if($account_bank==''){
                $rs['code'] = 1001;
                $rs['msg'] = \PhalApi\T('银行名称不能为空');
                return $rs;
            }
        }

        if($type==4){
            $account_bank='USDT.TRC20';
            $name='';
        }
        
        if($account==''){
            $rs['code'] = 1002;
            $rs['msg'] = \PhalApi\T('账号不能为空');
            return $rs;
        }
        
        
        if(mb_strlen($account)>40){
            $rs['code'] = 1002;
            $rs['msg'] = \PhalApi\T('账号长度不能超过{num}个字符',['num'=>40]);
            return $rs;
        }

        if($type==4 && !preg_match('/^T[1-9A-HJ-NP-Za-km-z]{33}$/',$account)){
            $rs['code'] = 1002;
            $rs['msg'] = \PhalApi\T('TRON地址格式不正确');
            return $rs;
        }
        
        $checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}        
        
        $data=array(
            'uid'=>$uid,
            'type'=>$type,
            'account_bank'=>$account_bank,
            'account'=>$account,
            'name'=>$name,
            'addtime'=>time(),
        );
        
        $domain = new Domain_User();
        $where=[
            'uid'=>$uid,
            'type'=>$type,
            'account_bank'=>$account_bank,
            'account'=>$account,
        ];
        $isexist=$domain->getUserAccount($where);
        if($isexist){
            $rs['code'] = 1004;
            $rs['msg'] = \PhalApi\T('账号已存在');
            return $rs;
        }
        
        $result = $domain->setUserAccount($data);

        if(!$result){
            $rs['code'] = 1003;
            $rs['msg'] = \PhalApi\T('添加失败，请重试');
            return $rs;
        }
        
        $rs['info'][0]=$result;

		return $rs;
	}	


	/**
	 * 删除用户提现账号 
	 * @desc 用于删除用户提现账号
	 * @return int code 操作码，0表示成功
	 * @return array info 
	 * @return string msg 提示信息
	 */
	public function delUserAccount() {
		$rs = array('code' => 0, 'msg' => \PhalApi\T('删除成功'), 'info' => array());
        
        $uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);
        
        $id=\App\checkNull($this->id);
        
        $checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}        
        
        $data=array(
            'uid'=>$uid,
            'id'=>$id,
        );
        
        $domain = new Domain_User();
        $result = $domain->delUserAccount($data);

        if(!$result){
            $rs['code'] = 1003;
            $rs['msg'] = \PhalApi\T('删除失败，请重试');
            return $rs;
        }

		return $rs;
	}	


    /**
     * 获取用户的认证信息
     * @desc 用于获取用户的认证信息
     * @return int code 状态码，0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
    public function getAuthInfo(){
    	$rs = array('code' => 0, 'msg' => '', 'info' => array());
        
        $uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);

        $checkToken=\App\checkToken($uid,$token);

		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

		$isauth=\App\isAuth($uid);
		if(!$isauth){
			$rs['code']=1001;
			$rs['msg']=\PhalApi\T('请先进行实名认证');
			return $rs;
		}

		$domain=new Domain_User();
		$res=$domain->getAuthInfo($uid);

		$rs['info'][0]=$res;
		return $rs;

    }
    

    /**
     * 查看每日任务
     * @desc 用于用户查看每日任务的进度
     * @return int code 状态码，0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
    public function seeDailyTasks(){
    	$rs = array('code' => 0, 'msg' => '', 'info' => array());
        
        $uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);
        $liveuid=\App\checkNull($this->liveuid);
        $islive=\App\checkNull($this->islive);

        $checkToken=\App\checkToken($uid,$token);

		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
		
		$configpri=\App\getConfigPri();
		$dailytask_switch=$configpri['dailytask_switch'];
		
		if($islive==1){   //判断请求是否在直播间
			if($uid==$liveuid){ //主播访问
				//主播直播计时---每日任务--取出用户时间
				$key='open_live_daily_tasks_'.$uid;
				$starttime=\App\getcaches($key);
				if($starttime){ 
					$endtime=time();  //当前时间
					$data=[
						'type'=>'3',
						'starttime'=>$starttime,
						'endtime'=>$endtime,
					];
					\App\dailyTasks($uid,$data);
					//删除当前存入的时间
					\App\delCache($key);
				}

				if($dailytask_switch){
					//主播直播计时---用于每日任务--记录主播请求时间
					$enterRoom_time=time();
					\App\setcaches($key,$enterRoom_time);
				}
				
				
			}else{  //用户访问
			
				//观看直播计时---每日任务--取出用户进入时间
				$key='watch_live_daily_tasks_'.$uid;
				$starttime=\App\getcaches($key);
				if($starttime){ 
					$endtime=time();  //当前时间
					$data=[
						'type'=>'1',
						'starttime'=>$starttime,
						'endtime'=>$endtime,
					];
					\App\dailyTasks($uid,$data);
					//删除当前存入的时间
					\App\delCache($key);
				}
				
				if($dailytask_switch){
					//观看直播计时---用于每日任务--记录用户进入时间
					$enterRoom_time=time();
					\App\setcaches($key,$enterRoom_time);
				}

			}
		}
		
		$domain=new Domain_User();
		$info=$domain->seeDailyTasks($uid);

		$configpub=\App\getConfigPub();
		$name_coin=$configpub['name_coin']; //星币名称

		$rs['info'][0]['tip_m']= \PhalApi\T("温馨提示：当您某个任务达成时就会获得平台奖励给您的{coin}，获得的奖励需要您手动领取才可放入余额中，当日不领取次日系统会自动清零，亲爱的您一定要记得领取当日奖励哦",['coin'=>$name_coin]);
		$rs['info'][0]['list']=$info;
		return $rs;

    }
	
	
	/**
     * 领取每日任务奖励
     * @desc 用于用户领取每日任务奖励
     * @return int code 状态码，0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
    public function receiveTaskReward(){
    	$rs = array('code' => 0, 'msg' => '', 'info' => array());
        
        $uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);
        $taskid=\App\checkNull($this->taskid);

        $checkToken=\App\checkToken($uid,$token);

		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}
		
		$domain=new Domain_User();
		$info=$domain->receiveTaskReward($uid,$taskid);

		
		return $info;

    }


    /**
     * 获取七牛上传Token
     * @desc 用于获取七牛上传Token
     * @return int code 操作码，0表示成功
     * @return string msg 提示信息
     */
	public function getQiniuToken(){
	   	$rs = array('code' => 0, 'msg' => '', 'info' =>array());

	   	//获取后台配置的腾讯云存储信息
		$Qiniu=\PhalApi\DI()->config->get('app.Qiniu');

		
		require_once API_ROOT.'/../sdk/qiniu/autoload.php';
		
		// 需要填写你的 Access Key 和 Secret Key
		// 需要填写你的 Access Key 和 Secret Key
		$accessKey =$Qiniu['accessKey'];// $configpri['qiniu_accesskey'];
		
		$secretKey = $Qiniu['secretKey'];//$configpri['qiniu_secretkey'];
		$bucket =$Qiniu['space_bucket'];// $configpri['qiniu_bucket'];
		$qiniu_domain_url = $Qiniu['space_host'];
		// 构建鉴权对象
		$auth = new \Qiniu\Auth($accessKey, $secretKey);
		// 生成上传 Token
		$token = $auth->uploadToken($bucket);
		$rs['info'][0]['token']=$token ; 
		return $rs; 
		
	}

	/**
     * 用户设置美颜参数
     * @desc 用于用户设置美颜参数
     * @return int code 状态码，0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
    public function setBeautyParams(){
    	$rs = array('code' => 0, 'msg' => \PhalApi\T('设置成功'), 'info' => array());
        
        $uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);
        $params=$this->params;

        $checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}else if($checkToken==10020){
			$rs['code'] = 700;
			$rs['msg'] = \PhalApi\T('该账号已被禁用');
			return $rs;
		}


		$domain=new Domain_User();
		$res=$domain->setBeautyParams($uid,$params);
		if(!$res){
			$rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T('设置失败');
			return $rs;
		}

		return $rs;
    }

    /**
     * 获取用户设置的美颜参数
     * @desc 用于获取用户设置的美颜参数
     * @return int code 状态码，0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
    public function getBeautyParams(){

    	$rs = array('code' => 0, 'msg' => '', 'info' => array());
        
        $uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);
        $checkToken=\App\checkToken($uid,$token);

		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}else if($checkToken==10020){
			$rs['code'] = 700;
			$rs['msg'] = \PhalApi\T('该账号已被禁用');
			return $rs;
		}

		$domain=new Domain_User();
		$res=$domain->getBeautyParams($uid);
		$rs['info'][0]=$res;

		return $rs;
    }

    /**
     * 用于APP端调用Braintree支付时的token验证
     * @desc 用于APP端调用Braintree支付时的token验证
     * @return int code 状态码,0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
    public function getBraintreeToken(){
    	$rs = array('code' => 0, 'msg' => '', 'info' => array());
    	$uid=\App\checkNull($this->uid);
        $token=\App\checkNull($this->token);

        $checkToken = \App\checkToken($uid,$token);

		if($checkToken==700){
			$rs['code']=700;
			$rs['msg']=\PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}else if($checkToken==10020){
			$rs['code'] = 700;
			$rs['msg'] = \PhalApi\T('该账号已被禁用');
			return $rs;
		}

		$getway_back=$this->getBrainTreeGateway();
		

        if($getway_back['code']!=0){
        	return $getway_back;
        }else{
        	$gateway=$getway_back['info'];
        }

		$clientToken = $gateway->clientToken()->generate();

		$rs['info'][0]['braintreeToken']=$clientToken;
		return $rs;
    }

    /**
     * BrainTree支付回调
     * @desc BrainTree支付回调
     * @return int code 状态码，0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
    public function BraintreeCallback(){
    	$rs = array('code' => 0, 'msg' => \PhalApi\T('回调成功'), 'info' => array());

    	$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$orderno=\App\checkNull($this->orderno);
		$ordertype=\App\checkNull($this->ordertype);
		$nonce=\App\checkNull($this->nonce);
		$money=\App\checkNull($this->money);
		$time=\App\checkNull($this->time);
		$sign=\App\checkNull($this->sign);


		//file_put_contents('./111111.txt',date('y-m-d H:i:s').' 提交参数信息:'.json_encode($nonce)."\r\n",FILE_APPEND);

		if(!in_array($ordertype, ['coin_charge'])){
			$rs['code'] = 1001;
			$rs['msg'] = \PhalApi\T('参数错误');
			return $rs;
		}

		if(!$nonce){
			$rs['code'] = 1002;
			$rs['msg'] = \PhalApi\T('三方订单编号错误');
			return $rs;
		}

		if(!$money){
			$rs['code'] = 1002;
			$rs['msg'] = \PhalApi\T('金额错误');
			return $rs;
		}

		$checkToken=\App\checkToken($uid,$token);

		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

		$now=time();
        if($now-$time>300){
            $rs['code']=1001;
            $rs['msg']=\PhalApi\T('参数错误');
            return $rs;
        }

        $checkdata=array(
            'uid'=>$uid,
            'ordertype'=>$ordertype,
            'orderno'=>$orderno,
            'time'=>$time,
			'nonce'=>$nonce
        );

        $issign=\App\checkSign($checkdata,$sign);
        if(!$issign){
            $rs['code']=1002;
            $rs['msg']=\PhalApi\T('签名错误');
            return $rs; 
        }

        $getway_back=$this->getBrainTreeGateway();

        if($getway_back['code']!=0){
        	return $getway_back;
        }else{
        	$gateway=$getway_back['info'];
        }

		$result = $gateway->transaction()->sale([
		    'amount' => $money,
		    'paymentMethodNonce' => $nonce,
		    'options' => [ 'submitForSettlement' => true ]
		]);

		if($result->success){

			$domain=new Domain_User();
	        $res=$domain->BraintreeCallback($uid,$orderno,$ordertype,$nonce,$money);
	        if($res==1001){
	        	$rs['code']=1001;
	            $rs['msg']=\PhalApi\T('订单不存在');
	            return $rs; 
	        }

	        if($res==1002){
	        	$rs['code']=1002;
	            $rs['msg']=\PhalApi\T('订单已支付');
	            return $rs; 
	        }

	        return $rs;

		}else{

			$rs['code']=1002;
            $rs['msg']=\PhalApi\T('订单回调验证失败');
            return $rs;

		}

    }

    /**
     * 获取BrainTreeGateway
     */
    private function getBrainTreeGateway(){
		//Braintree支付专用--start
		include API_ROOT.'/../vendor/Braintree/vendor/autoload.php';
		//use Braintree\ClientToken;
		//Braintree支付专用--start

    	$rs = array('code' => 0, 'msg' => '', 'info' => array());

    	$configpri=\App\getConfigPri();

		$environment=$configpri['braintree_paypal_environment'];

		$merchantId='';
		$publicKey='';
		$privateKey='';

		if($environment==0){ //沙盒
			$merchantId=$configpri['braintree_merchantid_sandbox'];
			$publicKey=$configpri['braintree_publickey_sandbox'];
			$privateKey=$configpri['braintree_privatekey_sandbox'];
			$environment='sandbox';
			
		}else{ //生产

			$merchantId=$configpri['braintree_merchantid_product'];
			$publicKey=$configpri['braintree_publickey_product'];
			$privateKey=$configpri['braintree_privatekey_product'];
			$environment='production';
			
		}

		if(!$merchantId || !$publicKey ||!$privateKey){
			$rs['code']=1001;
			$rs['msg']=\PhalApi\T('BraintreePaypal未配置');
			return $rs;
		}

		$gateway = new \Braintree\Gateway([
			'environment' => $environment,
			'merchantId' => $merchantId,
			'publicKey' => $publicKey,
			'privateKey' => $privateKey
		]);

		$rs['info']=$gateway;
		return $rs;
    }

    /**
     * 获取转盘中奖记录
     * @desc 获取转盘中奖记录
     * @return int code 状态码，0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
    public function getTurntableWinLists(){
    	$rs = array('code' => 0, 'msg' => '', 'info' => array());

    	$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$p=\App\checkNull($this->p);

		$checkToken=\App\checkToken($uid,$token);

		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

		if($p<1){
			$p=1;
		}

		$domain=new Domain_User();
		$res=$domain->getTurntableWinLists($uid,$p);
		$rs['info']=$res;

		return $rs;
    }

    /**
     * 清除转盘中奖记录
     * @desc 清除转盘中奖记录
     * @return int code 状态码，0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
    public function clearTurntableWinLists(){
		$rs = array('code' => 0, 'msg' =>  \PhalApi\T('清空成功'), 'info' => array());

    	$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);

		$checkToken=\App\checkToken($uid,$token);

		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

		$domain=new Domain_User();
		$res=$domain->clearTurntableWinLists($uid);

		return $rs;
    }

    /**
     * 检查用户是否开启了青少年模式
     * @desc 检查用户是否开启了青少年模式
     * @return int code 状态码，0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     * @return array info[0].is_setpassword 是否设置过密码 0 否 1 是
     * @return array info[0].status 是否开启青少年模式 0 否 1 是
     * @return array info[0].is_tip 是否提示用户弹窗显示青少年模式下不能继续使用app   0 否  1 是
     * @return array info[0].tips  弹窗显示青少年模式下不能继续使用app的提示语
     * @return array info[0].teenager_des  青少年模式提示语
     */
    public function checkTeenager(){
    	$rs = array('code' => 0, 'msg' => '', 'info' => array());

    	$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);

		$checkToken=\App\checkToken($uid,$token);

		/*if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}*/

		$domain = new Domain_User();
		$res = $domain->checkTeenager($uid);

		$configpub=\App\getConfigPub();

		$res['info'][0]['is_tip']='0';
		$res['info'][0]['tips']='';
		$res['info'][0]['teenager_des']=$configpub['teenager_des'];

		//开启了青少年模式
		if($res['info'][0]['is_setpassword'] && $res['info'][0]['status']){
			$overtime = $domain->checkTeenagerIsOvertime($uid);

			if($overtime['code']!=0){
				$res['info'][0]['is_tip']='1';
				$res['info'][0]['tips']=$overtime['msg'];
			}
		}

		return $res;
    }

    /**
     * 用户开启青少年模式/初次设置密码后重新设置密码
     * @desc 用户开启青少年模式/初次设置密码后重新设置密码
     * @return int code 状态码，0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
    public function setTeenagerPassword(){
    	$rs = array('code' => 0, 'msg' => \PhalApi\T('青少年模式开启成功'), 'info' => array());

    	$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$password=\App\checkNull($this->password);
		$type=\App\checkNull($this->type);

		$checkToken=\App\checkToken($uid,$token);

		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

		if(mb_strlen($password)!=4){
			$rs['code'] = 1001;
			$rs['msg'] =\PhalApi\T('密码必须为{num}位',['num'=>4]);
			return $rs;
		}

		$domain=new Domain_User();
		$res=$domain->setTeenagerPassword($uid,$password,$type);

		if($res==1001){
			$rs['code'] = 1002;
			$rs['msg'] =\PhalApi\T('密码错误');
			return $rs;
		}

		if($res==1002){
			$rs['code'] = 1003;
			$rs['msg'] =\PhalApi\T('密码设置失败,请稍后重试');
			return $rs;
		}

		return $rs;
    }

    /**
     * 用户修改青少年模式密码
     * @desc 用户修改青少年模式密码
     * @return int code 状态码,0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
    public function updateTeenagerPassword(){
    	$rs = array('code' => 0, 'msg' => \PhalApi\T('密码修改成功'), 'info' => array());

    	$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$oldpassword=\App\checkNull($this->oldpassword);
		$password=\App\checkNull($this->password);

		if(!$oldpassword){
			$rs['code']=1001;
			$rs['msg']=\PhalApi\T('请输入原密码');
			return $rs;
		}

		if(mb_strlen($oldpassword)!=4){
			$rs['code']=1002;
			$rs['msg']=\PhalApi\T('原密码长度必须为{num}位',['num'=>4]);
			return $rs;
		}

		if(!$password){
			$rs['code']=1001;
			$rs['msg']=\PhalApi\T('请输入新密码');
			return $rs;
		}

		if(mb_strlen($password)!=4){
			$rs['code']=1002;
			$rs['msg']='新密码长度必须为4位';
			return $rs;
		}

		$checkToken=\App\checkToken($uid,$token);

		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

		$domain = new Domain_User();
		$res=$domain->updateTeenagerPassword($uid,$oldpassword,$password);

		if($res==1001){
			$rs['code']=1003;
			$rs['msg']=\PhalApi\T('你还未设置密码');
			return $rs;
		}

		if($res==1002){
			$rs['code']=1004;
			$rs['msg']=\PhalApi\T('原密码错误');
			return $rs;
		}

		if(!$res){
			$rs['code']=1005;
			$rs['msg']=\PhalApi\T('密码修改失败');
			return $rs;
		}

		return $rs;
    }

    /**
     * 用户关闭青少年模式
     * @desc 用户关闭青少年模式
     * @return int code 状态码，0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
    public function closeTeenager(){

    	$rs = array('code' => 0, 'msg' => \PhalApi\T('青少年模式关闭成功'), 'info' => array());

    	$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$password=\App\checkNull($this->password);

		if(!$password){
			$rs['code']=1001;
			$rs['msg']=\PhalApi\T('请输入密码');
			return $rs;
		}

		if(mb_strlen($password)!=4){
			$rs['code']=1002;
			$rs['msg']=\PhalApi\T('密码长度必须为{num}位',['num'=>4]);
			return $rs;
		}

		$checkToken=\App\checkToken($uid,$token);

		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

		$domain = new Domain_User();
		$res=$domain->closeTeenager($uid,$password);
		if($res==1001){
			$rs['code'] = 1003;
			$rs['msg'] = \PhalApi\T('你还未开启青少年模式');
			return $rs;
		}

		if($res==1002){
			$rs['code'] = 1004;
			$rs['msg'] = \PhalApi\T('青少年模式未开启');
			return $rs;
		}

		if($res==1003){
			$rs['code'] = 1005;
			$rs['msg'] = \PhalApi\T('密码错误');
			return $rs;
		}

		if(!$res){
			$rs['code']=1006;
			$rs['msg']=\PhalApi\T('青少年模式关闭失败');
			return $rs;
		}

		return $rs;
    }

    /**
     * 定时增加用户使用青少年模式时间
     * @desc 定时增加用户使用青少年模式时间
     * @return int code 状态码，0表示成功
     * @return string msg 提示信息
     * @return array info 返回信息
     */
    public function addTeenagerTime(){
    	$rs = array('code' => 0, 'msg' => '', 'info' => array());

    	$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);

		$checkToken=\App\checkToken($uid,$token);

		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

		$domain = new Domain_User();
		$res=$domain->addTeenagerTime($uid);

		return $res;
    }


    /**
	 * 更换个人中心背景图
	 * @desc 更换个人中心背景图
	 * @retun int code 状态码,0表示成功
	 * @retun string msg 返回信息
	 * @retun array info 返回信息
	 * @retun array info[0]['bg_img'] 返回上传的背景图
	 * */
	public function updateBgImg(){
		$rs = array('code' => 0, 'msg' => \PhalApi\T('背景图更换成功'), 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);
		$img=\App\checkNull($this->img);

		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

		if(!$img){
			$rs['code']=1001;
			$rs['msg']=\PhalApi\T('请上传背景图');
			return $rs;
		}

		$domain=new Domain_User();
		$res=$domain->updateBgImg($uid,$img);

		if($res==1001){
			$rs['code']=1002;
			$rs['msg']=\PhalApi\T('背景图更换失败');
			return $rs;
		}

		\App\delCache("userinfo_".$uid);
		$userinfo=\App\getUserInfo($uid);
		$rs['info'][0]['bg_img']=$userinfo['bg_img'];
		return $rs;
	}

	/**
	 * 关闭/打开直播小窗开关
	 * @desc 关闭/打开直播小窗开关
	 * @retun int code 状态码,0表示成功
	 * @retun string msg 返回信息
	 * @retun array info 返回信息
	 * @retun array info[0]['status'] 直播小窗开关状态 0 关闭 1 打开
	 * */
	public function setLiveWindow(){
		$rs = array('code' => 0, 'msg' => '', 'info' => array());
		
		$uid=\App\checkNull($this->uid);
		$token=\App\checkNull($this->token);

		$checkToken=\App\checkToken($uid,$token);
		if($checkToken==700){
			$rs['code'] = $checkToken;
			$rs['msg'] = \PhalApi\T('您的登陆状态失效，请重新登陆！');
			return $rs;
		}

		$domain = new Domain_User();
		$res=$domain->setLiveWindow($uid);

		return $res;
	}
    
}
