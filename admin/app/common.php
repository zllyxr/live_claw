<?php
    use think\facade\Db;
    use cmf\lib\Storage;

    error_reporting(E_ALL);
    require_once dirname(__FILE__).'/redis.php';

    /* 密码检查 */
    function passcheck($user_pass) {
        $preg='/^(?=.*[A-Za-z])(?=.*[0-9])[a-zA-Z0-9~!@&%#_.]{6,20}$/';
        return preg_match($preg,$user_pass) ? 1 : 0;
    }

    /* 检验中国大陆手机号 */
    function checkMobile($mobile){
        return preg_match("/^1[3|4|5|6|7|8|9]\d{9}$/",$mobile) ? 1 : 0;
    }

    /* 去除NULL 判断空处理 主要针对字符串类型 */
    function checkNull($checkstr){
        $checkstr=urldecode((string)$checkstr);
        $checkstr=htmlspecialchars($checkstr);
        $checkstr=trim($checkstr);

        if(strstr($checkstr,'null') || (!$checkstr && $checkstr != 0)){
            $str='';
        }else{
            $str=$checkstr;
        }

        return htmlspecialchars($str);
    }

    /* 判断token */
    function checkToken($uid,$token){
        $userinfo=getcaches("token_".$uid);

        if(!$userinfo){
            $userinfo=Db::name('user_token')
                ->field('token,expire_time')
                ->where('user_id',$uid)
                ->find();
            if($userinfo){
                setcaches("token_".$uid,$userinfo);
            }
        }

        if((!$userinfo) || ($userinfo['token']!=$token) || ($userinfo['expire_time']<time())){
            return 700;
        }

        $info=Db::name('user')
            ->field('user_status,end_bantime')
            ->where('id',$uid)
            ->where('user_type','2')
            ->find();

        if(!$info || $info['user_status']==0 || $info['end_bantime']>time()){
            return 700;
        }

        return 0;
    }

    /* 判断是否拉黑 */
    function isBlack($uid,$touid){
        $isexist=Db::name('user_black')
            ->where('uid',$uid)
            ->where('touid',$touid)
            ->find();

        return $isexist ? 1 : 0;
    }

    /* 生成邀请码 */
    function createCode($len=6,$format='ALL2'){
        $is_abc=$is_numer=0;
        $password=$tmp='';
        switch($format){
            case 'ALL':
                $chars='ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
                break;
            case 'CHAR':
                $chars='ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz';
                break;
            case 'NUMBER':
                $chars='0123456789';
                break;
            case 'ALL2':
            default:
                $chars='ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
                break;
        }

        mt_srand((double)microtime()*1000000*getmypid());
        while(strlen($password)<$len){
            $tmp=substr($chars,(mt_rand()%strlen($chars)),1);
            if(($is_numer <> 1 && is_numeric($tmp) && $tmp > 0) || $format == 'CHAR'){
                $is_numer=1;
            }
            if(($is_abc <> 1 && preg_match('/[a-zA-Z]/',$tmp)) || $format == 'NUMBER'){
                $is_abc=1;
            }
            $password.=$tmp;
        }

        if($is_numer <> 1 || $is_abc <> 1 || empty($password)){
            $password=createCode($len,$format);
        }

        return $password;
    }

    /* 获取用户VIP */
    function getUserVip($uid){
        $rs=[
            'type'=>'0',
            'endtime'=>'',
        ];

        if($uid<1){
            return $rs;
        }

        $isexist=Db::name('vip_user')->where('uid',$uid)->find();
        if($isexist && $isexist['endtime']>time()){
            $rs['type']='1';
            $rs['endtime']=date('Y-m-d',$isexist['endtime']);
        }

        return $rs;
    }

    /* 获取用户靓号 */
    function getUserLiang($uid){
        $rs=[
            'name'=>'0',
        ];

        if($uid<1){
            return $rs;
        }

        $isexist=Db::name('liang')
            ->where('uid',$uid)
            ->where('status',1)
            ->where('state',1)
            ->find();
        if($isexist){
            $rs['name']=$isexist['name'];
        }

        return $rs;
    }

    /* 用户基本信息 */
    function getUserInfo($uid,$type=0){
        if(!is_numeric($uid)){
            $info=[
                'id'=>(string)$uid,
                'user_nickname'=>'用户不存在',
                'avatar'=>'/default.jpg',
                'avatar_thumb'=>'/default_thumb.jpg',
                'coin'=>'0',
                'sex'=>'1',
                'signature'=>'',
                'province'=>'',
                'city'=>'城市未填写',
                'birthday'=>'',
                'issuper'=>'0',
                'votestotal'=>'0',
                'consumption'=>'0',
                'location'=>'',
                'user_status'=>'1',
                'praise_num'=>'0',
                'bg_img'=>'/default.jpg',
                'age'=>'0',
            ];

            return $info;
        }

        $info=Db::name('user')
            ->field('id,user_login,user_nickname,avatar,avatar_thumb,sex,signature,coin,consumption,votestotal,votes,province,city,birthday,user_status,end_bantime,issuper,location,praise_num,bg_img,balance,balance_total,balance_consumption')
            ->where('id',$uid)
            ->where('user_type','2')
            ->find();

        if(!$info){
            if($type==1){
                return $info;
            }

            $info=[
                'id'=>(string)$uid,
                'user_nickname'=>$uid==1 ? '系统账号' : '用户不存在',
                'avatar'=>'/default.jpg',
                'avatar_thumb'=>'/default_thumb.jpg',
                'user_login'=>'',
                'coin'=>'0',
                'sex'=>'0',
                'signature'=>'',
                'consumption'=>'0',
                'votestotal'=>'0',
                'votes'=>'0',
                'province'=>'',
                'city'=>'',
                'birthday'=>'',
                'issuper'=>'0',
                'user_status'=>'1',
                'end_bantime'=>'0',
                'location'=>'',
                'praise_num'=>'0',
                'bg_img'=>'/default.jpg',
                'balance'=>'0',
                'balance_total'=>'0',
                'balance_consumption'=>'0',
                'age'=>'0',
            ];
        }

        $info=array_merge([
            'id'=>(string)$uid,
            'user_login'=>'',
            'user_nickname'=>'',
            'avatar'=>'/default.jpg',
            'avatar_thumb'=>'/default_thumb.jpg',
            'sex'=>'0',
            'signature'=>'',
            'coin'=>'0',
            'consumption'=>'0',
            'votestotal'=>'0',
            'votes'=>'0',
            'province'=>'',
            'city'=>'',
            'birthday'=>'',
            'user_status'=>'1',
            'end_bantime'=>'0',
            'issuper'=>'0',
            'location'=>'',
            'praise_num'=>'0',
            'bg_img'=>'/default.jpg',
            'balance'=>'0',
            'balance_total'=>'0',
            'balance_consumption'=>'0',
        ],$info);
        $info['id']=(string)$info['id'];
        $info['sex']=(string)$info['sex'];
        $info['level']=getLevel($info['consumption']);
        $info['level_anchor']=getLevelAnchor($info['votestotal']);
        $info['avatar']=get_upload_path($info['avatar']);
        $info['avatar_thumb']=get_upload_path($info['avatar_thumb']);
        $info['bg_img']=get_upload_path($info['bg_img']);
        $info['vip']=getUserVip($uid);
        $info['liang']=getUserLiang($uid);
        $info['consumption']=(string)$info['consumption'];
        $info['votestotal']=(string)$info['votestotal'];
        $info['coin']=(string)$info['coin'];
        // 统一钱包后，旧 votes 展示字段兼容为 coin。
        $info['votes']=(string)$info['coin'];
        $info['user_status']=(string)$info['user_status'];
        $info['end_bantime']=(string)$info['end_bantime'];
        $info['issuper']=(string)$info['issuper'];
        $info['praise_num']=(string)$info['praise_num'];
        $info['balance']=(string)$info['balance'];
        $info['balance_total']=(string)$info['balance_total'];
        $info['balance_consumption']=(string)$info['balance_consumption'];

        if($info['birthday']){
            $now=time();
            $nowYear=date('Y',$now);
            $month=date('m',$info['birthday']);
            $nowMonth=date('m',$now);
            $cha=$nowMonth>=$month ? 0 : 1;
            $birthdayYear=date('Y',$info['birthday']);
            $info['age']=(string)($nowYear-$birthdayYear-$cha);
            $info['birthday']=date('Y-m-d',$info['birthday']);
        }else{
            $info['birthday']='';
            $info['age']='0';
        }

        return $info;
    }

    /* H5登录态恢复 */
    function LogIn(){
        $uid=(int)session('uid');
        $token=(string)session('token');

        if(!$uid){
            $uid=(int)cookie('uid');
            $token=(string)cookie('token');
        }

        $paramUid=(int)request()->param('uid',0,'intval');
        $paramToken=checkNull((string)request()->param('token',''));
        if($paramUid>0){
            $uid=$paramUid;
            if($paramToken !== ''){
                $token=$paramToken;
            }
        }

        if($uid<1){
            return 0;
        }

        if($token !== '' && checkToken($uid,$token)==700){
            session('uid',null);
            session('token',null);
            session('user',null);
            cookie('uid',null);
            cookie('token',null);
            return 0;
        }

        $user=getUserInfo($uid,1);
        if(!$user){
            return 0;
        }

        session('uid',$uid);
        if($token !== ''){
            session('token',$token);
            cookie('token',$token);
        }
        session('user',$user);
        cookie('uid',$uid);

        return $uid;
    }

    /* 检测用户是否存在 */
    function checkUser($where){
        if($where===''){
            return 0;
        }

        return Db::name('user')->where($where)->find() ? 1 : 0;
    }

    /* 数字格式化 */
    function NumberFormat($num){
        $num=(float)$num;
        if($num<10000){
            return (string)(int)$num;
        }else if($num<1000000){
            return round($num/10000,2).'万';
        }else if($num<100000000){
            return round($num/10000,1).'万';
        }else if($num<10000000000){
            return round($num/100000000,2).'亿';
        }

        return round($num/100000000,1).'亿';
    }

    /* 关注数 */
    function getFollownums($uid){
        return (int)Db::name('user_attention')->where('uid',(int)$uid)->count();
    }

    /* 粉丝数 */
    function getFansnums($uid){
        return (int)Db::name('user_attention')->where('touid',(int)$uid)->count();
    }

    /* 判断是否关注 */
    function isAttention($uid,$touid){
        $uid=(int)$uid;
        $touid=(int)$touid;
        if($uid<1 || $touid<1){
            return '0';
        }

        $isexist=Db::name('user_attention')
            ->where('uid',$uid)
            ->where('touid',$touid)
            ->find();

        return $isexist ? '1' : '0';
    }

    /* 用户私有信息 */
    function getUserPrivateInfo($uid){
        $uid=(int)$uid;
        $info=getUserInfo($uid,1);
        if(!$info){
            return getUserInfo($uid);
        }

        $rawAvatar=$info['avatar'] ?? '';
        $rawAvatarThumb=$info['avatar_thumb'] ?? '';
        $info['avatar_s']=$rawAvatar;
        $info['avatar_t']=$rawAvatar;
        $info['avatar_thumb_s']=$rawAvatarThumb;

        return $info;
    }

    /* 个人中心礼物统计 */
    function getgif($uid){
        $uid=(int)$uid;
        $send=0;
        $receive=0;

        try{
            $send=(int)Db::name('user_coinrecord')->where('uid',$uid)->sum('giftcount');
            $receive=(int)Db::name('user_coinrecord')->where('touid',$uid)->sum('giftcount');
        }catch(\Throwable $e){
            $send=0;
            $receive=0;
        }

        return [
            [
                'tsc'=>(string)$send,
                'tsd'=>(string)$receive,
            ],
        ];
    }

    /* 主播送出总额 */
    function getSendCoins($uid){
        try{
            return (string)(int)Db::name('user_coinrecord')->where('uid',(int)$uid)->sum('totalcoin');
        }catch(\Throwable $e){
            return '0';
        }
    }

    /* 用户印象标签 */
    function getMyLabel($uid){
        $uid=(int)$uid;
        $rs=[];
        if($uid<1){
            return $rs;
        }

        $list=Db::name('label_user')
            ->field('label')
            ->where('touid',$uid)
            ->select()
            ->toArray();

        $label=[];
        foreach($list as $v){
            $items=preg_split('/,|，/',(string)($v['label'] ?? ''));
            $items=array_filter($items);
            if($items){
                $label=array_merge($label,$items);
            }
        }

        if(!$label){
            return $rs;
        }

        $labelNums=array_count_values($label);
        $labelIds=array_keys($labelNums);
        $labels=Db::name('label')->order('list_order asc')->select()->toArray();
        $orderNums=[];

        foreach($labels as $v){
            $id=(string)$v['id'];
            if(in_array($id,$labelIds,true)){
                $v['nums']=(string)$labelNums[$id];
                $orderNums[]=$v['nums'];
                $rs[]=$v;
            }
        }

        if($rs){
            array_multisort($orderNums,SORT_DESC,$rs);
        }

        return $rs;
    }

    /* 后台操作日志 */
    function setAdminLog($action){
        $adminId=function_exists('cmf_get_current_admin_id') ? (int)cmf_get_current_admin_id() : (int)session('ADMIN_ID');
        $adminName=(string)(session('name') ?: '');
        if($adminName === '' && $adminId){
            $adminName=(string)Db::name('user')->where('id',$adminId)->value('user_login');
        }

        $ip=0;
        if(function_exists('get_client_ip')){
            $clientIp=get_client_ip(0,true);
            $ipLong=ip2long($clientIp);
            if($ipLong !== false){
                $ip=sprintf('%u',$ipLong);
            }
        }

        return Db::name('admin_log')->insert([
            'adminid'=>$adminId,
            'admin'=>$adminName,
            'action'=>(string)$action,
            'ip'=>$ip,
            'addtime'=>time(),
        ]);
    }

    /* 信息脱敏 */
    function m_s($str){
        $str=(string)$str;
        if($str === ''){
            return $str;
        }

        if(strpos($str,'@') !== false){
            $parts=explode('@',$str,2);
            $name=$parts[0];
            $domain=$parts[1];
            $nameLen=strlen($name);

            if($nameLen <= 1){
                return $name.'***@'.$domain;
            }

            return substr($name,0,min(2,$nameLen)).'***@'.$domain;
        }

        $len=strlen($str);
        if($len <= 4){
            return $str;
        }

        if($len <= 7){
            return substr($str,0,1).'***'.substr($str,-1);
        }

        return substr($str,0,3).'****'.substr($str,-4);
    }

    /* 会员等级 */
    function getLevelList(){
        $key='level';
        $level=getcaches($key);
        if(!$level){
            $level=Db::name('level')->order('level_up asc')->select()->toArray();
            if($level){
                setcaches($key,$level);
            }
        }

        if(!$level){
            return [];
        }

        foreach($level as $k=>$v){
            $v['thumb']=get_upload_path($v['thumb']);
            $v['thumb_mark']=get_upload_path($v['thumb_mark']);
            $v['bg']=get_upload_path($v['bg']);
            $v['colour']=$v['colour'] ? '#'.$v['colour'] : '#ffdd00';
            $level[$k]=$v;
        }

        return $level;
    }

    function getLevel($experience){
        $levelid=1;
        $level_a=1;
        $level=getLevelList();

        foreach($level as $v){
            if($v['level_up'] >= $experience){
                $levelid=$v['levelid'];
                break;
            }else{
                $level_a=$v['levelid'];
            }
        }

        $levelid=$levelid < $level_a ? $level_a : $levelid;
        return (string)$levelid;
    }

    /* 主播等级 */
    function getLevelAnchorList(){
        $key='levelanchor';
        $level=getcaches($key);
        if(!$level){
            $level=Db::name('level_anchor')->order('level_up asc')->select()->toArray();
            if($level){
                setcaches($key,$level);
            }
        }

        if(!$level){
            return [];
        }

        foreach($level as $k=>$v){
            $v['thumb']=get_upload_path($v['thumb']);
            $v['thumb_mark']=get_upload_path($v['thumb_mark']);
            $v['bg']=get_upload_path($v['bg']);
            $level[$k]=$v;
        }

        return $level;
    }

    function getLevelAnchor($experience){
        $levelid=1;
        $level_a=1;
        $level=getLevelAnchorList();

        foreach($level as $v){
            if($v['level_up'] >= $experience){
                $levelid=$v['levelid'];
                break;
            }else{
                $level_a=$v['levelid'];
            }
        }

        $levelid=$levelid < $level_a ? $level_a : $levelid;
        return (string)$levelid;
    }


    /**
     * @desc 判断文件的后缀是否在指定范围内
     * @param $file_name 文件名称
     * @param $allow_type 允许范围
     * @return bool
     */
    function get_file_suffix($file_name, $allow_type = array()){

        $fnarray=explode('.', $file_name);

        $file_suffix = strtolower(end($fnarray));

        if (empty($allow_type)){
            return true;
        }else{
            if (in_array($file_suffix, $allow_type)){
                return true;
            }else{
                return false;
            }
        }
    }

    /**
     * @desc 直播间封禁规则
     * @return array
     */
    function getLiveBanRules(){
        $rules=[
            [
                'id'=>'1',
                'name'=>'30分钟',
                'type'=>'30min'
            ],
            [
                'id'=>'2',
                'name'=>'1天',
                'type'=>'1day'
            ],
            [
                'id'=>'3',
                'name'=>'7天',
                'type'=>'7day'
            ],
            [
                'id'=>'4',
                'name'=>'15天',
                'type'=>'15day'
            ],
            [
                'id'=>'5',
                'name'=>'30天',
                'type'=>'30day'
            ],
            [
                'id'=>'6',
                'name'=>'90天',
                'type'=>'90day'
            ],
            [
                'id'=>'7',
                'name'=>'180天',
                'type'=>'180day'
            ],
            [
                'id'=>'8',
                'name'=>'永久',
                'type'=>'all'
            ]
        ];

        return $rules;
    }

    //随机生成存数字字符串
    function get_str($length){
        $str = '0123456789';
        $len = strlen($str)-1;
        $randstr = '';
        for ($i=0;$i<$length;$i++) {
         $num=mt_rand(0,$len);
         $randstr .= $str[$num];
        }
        return $randstr;
        
    }

    /**
    * 腾讯云TPNS移动推送
    * @param  string  $title 推送标题
    * @param  string  $msg   推送消息内容
    * @param  string  $type  推送类型 all 全员推送 single 单账号推送 account_list 账号列表推送
    * @param  integer $uid   单账号用户id
    * @url https://cloud.tencent.com/document/product/548/39064
    */
   function txMessageTpns($title,$msg,$type,$uid=0,$account_list=[],$json_str='',$language='zh-cn'){
        
        require_once CMF_ROOT.'sdk/tencentTpns/tpns.php';
        $configpri=getConfigPri();
        $area=$configpri['tencentTpns_area'];
        $accessid_android=$configpri['tencentTpns_accessid_android'];
        $secretkey_android=$configpri['tencentTpns_secretkey_android'];
        $accessid_ios=$configpri['tencentTpns_accessid_ios'];
        $secretkey_ios=$configpri['tencentTpns_secretkey_ios'];
        $ios_environment=$configpri['tencentTpns_ios_environment'];


        if(
            !in_array($area,['guangzhou','shanghai','hongkong','singapore']) || 
            !$accessid_android || 
            !$secretkey_android || 
            !$accessid_ios || 
            !$secretkey_ios
        ){
            return;
        }


        if($area=='guangzhou'){
            $stub_android = new tpns\Stub($accessid_android, $secretkey_android, tpns\GUANGZHOU);
            $stub_ios = new tpns\Stub($accessid_ios, $secretkey_ios, tpns\GUANGZHOU);
        }else if($area=='shanghai'){
            $stub_android = new tpns\Stub($accessid_android, $secretkey_android, tpns\SHANGHAI);
            $stub_ios = new tpns\Stub($accessid_ios, $secretkey_ios, tpns\SHANGHAI);
        }else if($area=='hongkong'){
            $stub_android = new tpns\Stub($accessid_android, $secretkey_android, tpns\HONGKONG);
            $stub_ios = new tpns\Stub($accessid_ios, $secretkey_ios, tpns\HONGKONG);
        }else if($area=='singapore'){
            $stub_android = new tpns\Stub($accessid_android, $secretkey_android, tpns\SINGAPORE);
            $stub_ios = new tpns\Stub($accessid_ios, $secretkey_ios, tpns\SINGAPORE);
        }else{
            return;
        }


        if($type=='account_list' && count($account_list)==1){
            $type='single';
            $uid=$account_list[0];
        }


        

        
        if($type=='all'){

            //Android推送
            $android = new tpns\AndroidMessage;
            if($json_str){
                $android->custom_content = $json_str;   
            }

            //控制通知点击时乱转到指定页面
            $action=[
                "action_type"=> 1,// 动作类型，1，打开activity或app本身；2，打开浏览器；3，打开Intent
                "activity"=> "com.yunbao.im.activity.ImMsgNotifyActivity"
            ];

            $tagItem = new tpns\TagItem;
            $tagItem->tags = array($language);
            $tagItem->tag_type = "xg_user_define";
            

            $tagRule = new tpns\TagRule;
            $tagRule->tag_items = array($tagItem);

            $android->action=(object)$action;

            $req_android = tpns\NewRequest(
                tpns\WithAudienceType(tpns\AUDIENCE_TAG),
                tpns\WithMessageType(tpns\MESSAGE_NOTIFY),
                tpns\WithTitle($title),
                tpns\WithContent($msg),
                tpns\WithTagRules(array($tagRule)),
                tpns\WithAndroidMessage($android),
                tpns\WithEnvironment(tpns\ENVIRONMENT_PROD)
            );

            $result_android = $stub_android->Push($req_android);
            //var_dump($result_android);

            //iOS推送
            $ios = new tpns\iOSMessage;
            if($json_str){
                $ios->custom = $json_str;   
            }


            if($ios_environment==0){ //开发
                $req_ios = tpns\NewRequest(
                    tpns\WithAudienceType(tpns\AUDIENCE_TAG),
                    tpns\WithMessageType(tpns\MESSAGE_NOTIFY),
                    tpns\WithTitle($title),
                    tpns\WithContent($msg),
                    tpns\WithTagRules(array($tagRule)),
                    tpns\WithIOSMessage($ios),
                    tpns\WithEnvironment(tpns\ENVIRONMENT_DEV)
                );
            }else{

                $req_ios = tpns\NewRequest(
                    tpns\WithAudienceType(tpns\AUDIENCE_TAG),
                    tpns\WithMessageType(tpns\MESSAGE_NOTIFY),
                    tpns\WithTitle($title),
                    tpns\WithContent($msg),
                    tpns\WithTagRules(array($tagRule)),
                    tpns\WithIOSMessage($ios),
                    tpns\WithEnvironment(tpns\ENVIRONMENT_PROD)
                );
            }

            $result_ios = $stub_ios->Push($req_ios);
            //var_dump($result_ios);

        }else if($type=='single'){

            if(!$uid){
                return;
            }

            $uid=(string)$uid;

            $tagItem1 = new tpns\TagItem;
            $tagItem1->tags = array($language);
            $tagItem1->tag_type = "xg_user_define";


            $tagItem2 = new tpns\TagItem;
            $tagItem2->tags = array($uid);
            $tagItem2->items_operator = tpns\TAG_OPERATOR_AND; //tagItem2与tagItem1之间的逻辑关系
            $tagItem2->tag_type = "xg_user_define";
            

            $tagRule = new tpns\TagRule;
            $tagRule->tag_items = array($tagItem1,$tagItem2);

            //Android推送
            $android = new tpns\AndroidMessage;
            if($json_str){
                $android->custom_content = $json_str;   
            }

            $action=[
                "action_type"=> 1,// 动作类型，1，打开activity或app本身；2，打开浏览器；3，打开Intent
                "activity"=> "com.yunbao.im.activity.ImMsgNotifyActivity"
            ];

            $android->action=(object)$action;

            $req_android = tpns\NewRequest(
                tpns\WithAudienceType(tpns\AUDIENCE_TAG),
                tpns\WithMessageType(tpns\MESSAGE_NOTIFY),
                tpns\WithTitle($title),
                tpns\WithContent($msg),
                tpns\WithAndroidMessage($android),
                tpns\WithTagRules(array($tagRule)),
                tpns\WithEnvironment(tpns\ENVIRONMENT_PROD)
            );

            $result_android = $stub_android->Push($req_android);
            //var_dump($result_android);

            //iOS推送
            $ios = new tpns\iOSMessage;
            if($json_str){
                $ios->custom = $json_str;   
            }
            

            if($ios_environment==0){ //开发

                $req_ios = tpns\NewRequest(
                    tpns\WithAudienceType(tpns\AUDIENCE_TAG),
                    tpns\WithMessageType(tpns\MESSAGE_NOTIFY),
                    tpns\WithTitle($title),
                    tpns\WithContent($msg),
                    tpns\WithIOSMessage($ios),
                    tpns\WithTagRules(array($tagRule)),
                    tpns\WithEnvironment(tpns\ENVIRONMENT_DEV)
                );

            }else{
                $req_ios = tpns\NewRequest(
                    tpns\WithAudienceType(tpns\AUDIENCE_TAG),
                    tpns\WithMessageType(tpns\MESSAGE_NOTIFY),
                    tpns\WithTitle($title),
                    tpns\WithContent($msg),
                    tpns\WithIOSMessage($ios),
                    tpns\WithTagRules(array($tagRule)),
                    tpns\WithEnvironment(tpns\ENVIRONMENT_PROD)
                );
            }
            

            $result_ios = $stub_ios->Push($req_ios);
            //var_dump($result_ios);

        }else if($type=='account_list'){

            if(empty($account_list)){
                return;
            }


            $tagItem1 = new tpns\TagItem;
            $tagItem1->tags = array($language);
            $tagItem1->tag_type = "xg_user_define";


            $tagItem2 = new tpns\TagItem;
            $tagItem2->tags = $account_list;
            $tagItem2->tags_operator = tpns\TAG_OPERATOR_OR; //tagItem2内部标签之间的逻辑关系
            $tagItem2->items_operator = tpns\TAG_OPERATOR_AND; //tagItem2与tagItem1之间的逻辑关系
            $tagItem2->tag_type = "xg_user_define";
            

            $tagRule = new tpns\TagRule;
            $tagRule->tag_items = array($tagItem1,$tagItem2);


            //Android推送
            $android = new tpns\AndroidMessage;
            if($json_str){
                $android->custom_content = $json_str;   
            }

            $action=[
                "action_type"=> 1,// 动作类型，1，打开activity或app本身；2，打开浏览器；3，打开Intent
                "activity"=> "com.yunbao.im.activity.ImMsgNotifyActivity"
            ];

            $android->action=(object)$action;

            $req_android = tpns\NewRequest(
                tpns\WithAudienceType(tpns\AUDIENCE_TAG),
                tpns\WithMessageType(tpns\MESSAGE_NOTIFY),
                tpns\WithTitle($title),
                tpns\WithContent($msg),
                tpns\WithAndroidMessage($android),
                tpns\WithTagRules(array($tagRule)),
                tpns\WithEnvironment(tpns\ENVIRONMENT_PROD)
            );

            $result_android = $stub_android->Push($req_android);
            //var_dump($result_android);

            //iOS推送
            $ios = new tpns\iOSMessage;
            if($json_str){
                $ios->custom = $json_str;   
            }

            if($ios_environment==0){ //开发
                $req_ios = tpns\NewRequest(
                    tpns\WithAudienceType(tpns\AUDIENCE_TAG),
                    tpns\WithMessageType(tpns\MESSAGE_NOTIFY),
                    tpns\WithTitle($title),
                    tpns\WithContent($msg),
                    tpns\WithIOSMessage($ios),
                    tpns\WithTagRules(array($tagRule)),
                    tpns\WithEnvironment(tpns\ENVIRONMENT_DEV)
                );
            }else{
                $req_ios = tpns\NewRequest(
                    tpns\WithAudienceType(tpns\AUDIENCE_TAG),
                    tpns\WithMessageType(tpns\MESSAGE_NOTIFY),
                    tpns\WithTitle($title),
                    tpns\WithContent($msg),
                    tpns\WithIOSMessage($ios),
                    tpns\WithTagRules(array($tagRule)),
                    tpns\WithEnvironment(tpns\ENVIRONMENT_PROD)
                );
            }
            

            $result_ios = $stub_ios->Push($req_ios);
            //var_dump($result_ios);

        }
   
   }

    function claw_config_cache_get($key){
        if(!function_exists('getcaches')){
            return false;
        }

        try{
            if(!isset($GLOBALS['redisdb']) && function_exists('connectionRedis')){
                connectionRedis();
            }
            if(!isset($GLOBALS['redisdb'])){
                return false;
            }

            return getcaches($key);
        }catch(\Throwable $e){
            return false;
        }
    }

    function claw_config_cache_set($key,$info,$time=0){
        if(!function_exists('setcaches')){
            return false;
        }

        try{
            if(!isset($GLOBALS['redisdb']) && function_exists('connectionRedis')){
                connectionRedis();
            }
            if(!isset($GLOBALS['redisdb'])){
                return false;
            }

            return setcaches($key,$info,$time);
        }catch(\Throwable $e){
            return false;
        }
    }

    function claw_get_option($name){
        $config=[];
        if(function_exists('cmf_get_option')){
            $config=cmf_get_option($name);
        }

        if(!$config){
            $optionValue=Db::name('option')->where('option_name',$name)->value('option_value');
            if($optionValue){
                $config=json_decode($optionValue,true);
            }
        }

        return is_array($config) ? $config : [];
    }

    function claw_split_config_value($value,$splitNested=false){
        if(is_array($value)){
            return $value;
        }
        if($value === null || $value === ''){
            return [];
        }

        $items=preg_split('/,|，/',(string)$value);
        if($splitNested){
            foreach($items as $k=>$v){
                $items[$k]=preg_split('/;|；/',$v);
            }
        }

        return $items;
    }

    function claw_star_coin_name(){
        return '星币';
    }

    function claw_star_coin_name_en(){
        return 'Star Coin';
    }

    function claw_normalize_public_currency_config($config){
        $config=is_array($config) ? $config : [];
        $config['name_coin']=claw_star_coin_name();
        $config['name_votes']=claw_star_coin_name();
        $config['name_score']=claw_star_coin_name();
        $config['name_coin_en']=claw_star_coin_name_en();
        $config['name_votes_en']=claw_star_coin_name_en();
        $config['name_score_en']=claw_star_coin_name_en();

        return $config;
    }

    function claw_normalize_private_currency_config($config){
        $config=is_array($config) ? $config : [];
        $config['cash_rate']='1';
        $config['cash_take']='0';
        $config['bepusdt_fiat']='USD';

        return $config;
    }

    function claw_star_coin_amount_from_charge($charge){
        $money=(float)($charge['money'] ?? 0);
        if($money<=0){
            $money=(float)($charge['coin'] ?? 0);
        }

        $coin=(int)round($money);
        return $coin > 0 ? $coin : 0;
    }

    function claw_normalize_charge_rule($charge){
        if(!$charge || !is_array($charge)){
            return $charge;
        }

        $coin=claw_star_coin_amount_from_charge($charge);
        if($coin<=0){
            return $charge;
        }

        $charge['coin']=$coin;
        $charge['coin_ios']=$coin;
        $charge['coin_paypal']=$coin;
        $charge['give']=0;
        $charge['money']=number_format($coin,2,'.','');

        return $charge;
    }

    function claw_normalize_charge_rules($rules){
        if(!is_iterable($rules)){
            return $rules;
        }

        foreach($rules as $k=>$rule){
            $rules[$k]=claw_normalize_charge_rule(is_array($rule) ? $rule : $rule->toArray());
        }

        return $rules;
    }

    /* 公共配置 */
    function getConfigPub() {
        $key='getConfigPub';
        $config=claw_config_cache_get($key);
        if(!$config){
            $config=claw_get_option('site_info');
            if($config){
                claw_config_cache_set($key,$config);
            }
        }

        $config=is_array($config) ? $config : [];
        $config['live_time_coin']=claw_split_config_value($config['live_time_coin'] ?? '');
        $config['login_type']=claw_split_config_value($config['login_type'] ?? '');
        $config['share_type']=claw_split_config_value($config['share_type'] ?? '');
        $config['live_type']=claw_split_config_value($config['live_type'] ?? '',true);

        return claw_normalize_public_currency_config($config);
    }

    /* 私密配置 */
    function getConfigPri() {
        $key='getConfigPri';
        $config=claw_config_cache_get($key);
        if(!$config){
            $config=claw_get_option('configpri');
            if($config){
                claw_config_cache_set($key,$config);
            }
        }

        $config=is_array($config) ? $config : [];
        $config['game_switch']=claw_split_config_value($config['game_switch'] ?? '');
        $config['cloudtype']=$config['cloudtype'] ?? '3';
        $config['minio_region']=$config['minio_region'] ?? 'us-east-1';
        $config['usdt_switch']=$config['usdt_switch'] ?? (claw_env_value('BEPUSDT_API_TOKEN') ? '1' : '0');
        $config['bepusdt_api_url']=$config['bepusdt_api_url'] ?? claw_env_value('BEPUSDT_API_URL','');
        $config['bepusdt_api_token']=$config['bepusdt_api_token'] ?? claw_env_value('BEPUSDT_API_TOKEN','');
        $config['bepusdt_trade_type']=$config['bepusdt_trade_type'] ?? claw_env_value('BEPUSDT_TRADE_TYPE','usdt.trc20');
        $config['bepusdt_fiat']=$config['bepusdt_fiat'] ?? claw_env_value('BEPUSDT_FIAT','USD');
        $config['bepusdt_timeout']=$config['bepusdt_timeout'] ?? claw_env_value('BEPUSDT_TIMEOUT','1200');

        return claw_normalize_private_currency_config($config);
    }

    function claw_env_value($key,$default=''){
        $value=getenv($key);
        if($value===false && isset($_ENV[$key])){
            $value=$_ENV[$key];
        }

        return ($value===false || $value===null) ? $default : $value;
    }

    function claw_get_bepusdt_config($configpri=null){
        if($configpri===null){
            $configpri=getConfigPri();
        }

        $apiUrl=trim($configpri['bepusdt_api_url'] ?? '') ?: trim(claw_env_value('BEPUSDT_API_URL',''));
        if($apiUrl!=='' && file_exists('/.dockerenv')){
            $apiUrl=preg_replace('#^(https?://)(127\.0\.0\.1|localhost)(:\d+)?#i','$1host.docker.internal$3',$apiUrl);
        }

        $token=trim($configpri['bepusdt_api_token'] ?? '') ?: trim(claw_env_value('BEPUSDT_API_TOKEN',''));
        $enabled=(string)($configpri['usdt_switch'] ?? ($token!=='' ? '1' : '0')) === '1';
        $timeout=(int)(trim($configpri['bepusdt_timeout'] ?? '') ?: claw_env_value('BEPUSDT_TIMEOUT','1200'));
        if($timeout<120){
            $timeout=1200;
        }

        return [
            'enabled'=>$enabled,
            'api_url'=>rtrim($apiUrl,'/'),
            'api_token'=>$token,
            'trade_type'=>trim($configpri['bepusdt_trade_type'] ?? '') ?: claw_env_value('BEPUSDT_TRADE_TYPE','usdt.trc20'),
            'fiat'=>strtoupper(trim($configpri['bepusdt_fiat'] ?? '') ?: claw_env_value('BEPUSDT_FIAT','USD')),
            'timeout'=>$timeout,
        ];
    }

    function claw_bepusdt_sign($params,$token){
        unset($params['signature']);
        ksort($params,SORT_STRING);

        $pairs=[];
        foreach($params as $key=>$value){
            if($value===null || $value===''){
                continue;
            }
            if(is_bool($value)){
                $value=$value ? 'true' : 'false';
            }
            $pairs[]=$key.'='.$value;
        }

        return md5(implode('&',$pairs).$token);
    }

    function claw_bepusdt_request($path,$params,$config=null){
        if($config===null){
            $config=claw_get_bepusdt_config();
        }

        if(empty($config['enabled'])){
            return ['status_code'=>0,'message'=>'USDT支付未开启'];
        }

        if(empty($config['api_url']) || empty($config['api_token'])){
            return ['status_code'=>0,'message'=>'BEpusdt网关未配置'];
        }

        $params['signature']=claw_bepusdt_sign($params,$config['api_token']);
        $ch=curl_init($config['api_url'].$path);
        curl_setopt($ch,CURLOPT_RETURNTRANSFER,true);
        curl_setopt($ch,CURLOPT_POST,true);
        curl_setopt($ch,CURLOPT_POSTFIELDS,json_encode($params,JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES));
        curl_setopt($ch,CURLOPT_HTTPHEADER,['Content-Type: application/json']);
        curl_setopt($ch,CURLOPT_CONNECTTIMEOUT,10);
        curl_setopt($ch,CURLOPT_TIMEOUT,15);
        $body=curl_exec($ch);
        $errno=curl_errno($ch);
        $error=curl_error($ch);
        curl_close($ch);

        if($errno){
            return ['status_code'=>0,'message'=>'BEpusdt请求失败：'.$error];
        }

        $result=json_decode($body,true);
        if(!is_array($result)){
            return ['status_code'=>0,'message'=>'BEpusdt响应解析失败','raw'=>$body];
        }

        return $result;
    }

    function claw_bepusdt_create_transaction($orderid,$money,$name,$notifyUrl,$redirectUrl=''){
        $config=claw_get_bepusdt_config();
        $amount=(float)$money;
        if($amount<=0){
            return ['status_code'=>0,'message'=>'充值金额错误'];
        }

        $params=[
            'order_id'=>(string)$orderid,
            'amount'=>$amount,
            'fiat'=>$config['fiat'],
            'trade_type'=>$config['trade_type'],
            'name'=>(string)$name,
            'notify_url'=>(string)$notifyUrl,
            'redirect_url'=>(string)$redirectUrl,
            'timeout'=>$config['timeout'],
        ];

        return claw_bepusdt_request('/api/v1/order/create-transaction',$params,$config);
    }

    function handelCharge($where,$data=[]){
        if(empty($where) || !is_array($where)){
            return 0;
        }

        $paid=Db::name('charge_user')
            ->where($where)
            ->where('status',1)
            ->find();
        if($paid){
            return 1;
        }

        Db::startTrans();
        try{
            $order=Db::name('charge_user')
                ->where($where)
                ->where('status',0)
                ->lock(true)
                ->find();

            if(!$order){
                Db::rollback();
                $paid=Db::name('charge_user')
                    ->where($where)
                    ->where('status',1)
                    ->find();
                return $paid ? 1 : 0;
            }

            if(!empty($data['trade_no'])){
                $exist=Db::name('charge_user')
                    ->where('trade_no',$data['trade_no'])
                    ->where('type',$order['type'])
                    ->where('id','<>',$order['id'])
                    ->find();
                if($exist){
                    Db::rollback();
                    return 0;
                }
            }

            $uid=(int)$order['touid'];
            $coin=(int)$order['coin'];
            if($coin>0){
                Db::name('user')->where('id',$uid)->inc('coin',$coin)->update();
            }

            $update=array_merge($data,['status'=>1]);
            Db::name('charge_user')
                ->where('id',$order['id'])
                ->where('status',0)
                ->update($update);

            if((int)$order['is_first']===1){
                $now=time();

                if((int)$order['score']>0){
                    Db::name('user')->where('id',$uid)->inc('score',(int)$order['score'])->update();
                    Db::name('user_scorerecord')->insert([
                        'type'=>1,
                        'action'=>'22',
                        'uid'=>$order['uid'],
                        'touid'=>$uid,
                        'giftid'=>$order['id'],
                        'giftcount'=>1,
                        'totalcoin'=>$order['score'],
                        'addtime'=>$now,
                    ]);
                }

                if((int)$order['vip_length']>0){
                    $endtime=60*60*24*(int)$order['vip_length'];
                    $vipInfo=Db::name('vip_user')->where('uid',$uid)->find();
                    if(!$vipInfo){
                        $endtime+=$now;
                        Db::name('vip_user')->insert(['uid'=>$uid,'addtime'=>$now,'endtime'=>$endtime]);
                    }else{
                        $endtime+=((int)$vipInfo['endtime']>$now) ? (int)$vipInfo['endtime'] : $now;
                        Db::name('vip_user')->where('uid',$uid)->update(['endtime'=>$endtime]);
                    }

                    $vipInfo=Db::name('vip_user')->where('uid',$uid)->find();
                    if($vipInfo){
                        setcaches('vip_'.$uid,$vipInfo);
                    }
                }

                if((int)$order['giftid']>0 && (int)$order['gift_num']>0){
                    $backpack=Db::name('backpack')
                        ->where('uid',$uid)
                        ->where('giftid',$order['giftid'])
                        ->find();
                    if(!$backpack){
                        Db::name('backpack')->insert([
                            'uid'=>$uid,
                            'giftid'=>$order['giftid'],
                            'nums'=>$order['gift_num'],
                        ]);
                    }else{
                        Db::name('backpack')
                            ->where('uid',$uid)
                            ->where('giftid',$order['giftid'])
                            ->inc('nums',(int)$order['gift_num'])
                            ->update();
                    }
                }

                Db::name('user')->where('id',$uid)->update(['firstcharge_used'=>1]);
            }

            delcache('userinfo_'.$uid);
            Db::commit();
            return 1;
        }catch(\Throwable $e){
            Db::rollback();
            return 0;
        }
    }

    /**
     * 返回带协议的域名
     */
    function get_host(){
        $config=getConfigPub();
        if(!empty($config['site'])){
            return rtrim($config['site'],'/');
        }

        try{
            return rtrim(request()->domain(),'/');
        }catch(\Throwable $e){
            $scheme=(!empty($_SERVER['HTTPS']) && $_SERVER['HTTPS'] !== 'off') ? 'https' : 'http';
            $host=$_SERVER['HTTP_HOST'] ?? '127.0.0.1:18080';
            return $scheme.'://'.$host;
        }
    }

    function getStorageTypeByCloudtype($cloudtype = null){
        if($cloudtype === null || $cloudtype === ''){
            $configpri=getConfigPri();
            $cloudtype=$configpri['cloudtype'] ?? '3';
        }

        $cloudtype=strtolower((string)$cloudtype);
        switch ($cloudtype) {
            case '4':
            case 'minio':
                return 'minio';
            case '3':
            case 'local':
            default:
                return 'local';
        }
    }

    function getStorageType(){
        $configpri=getConfigPri();
        return getStorageTypeByCloudtype($configpri['cloudtype'] ?? '3');
    }

    function buildStorageUrl($baseUrl,$file){
        $file=str_replace('\\','/',ltrim((string)$file,'/'));
        $parts=array_map('rawurlencode',explode('/',$file));
        return rtrim($baseUrl,'/').'/'.implode('/',$parts);
    }

    function getLocalUploadUrl($file){
        return buildStorageUrl(rtrim(get_host(),'/').'/upload',$file);
    }

    function getMinioBaseUrl($configpri = null){
        if($configpri === null){
            $configpri=getConfigPri();
        }

        $publicUrl=trim($configpri['minio_public_url'] ?? '');
        if($publicUrl !== ''){
            return rtrim($publicUrl,'/');
        }

        $endpoint=trim($configpri['minio_endpoint'] ?? '');
        $bucket=trim($configpri['minio_bucket'] ?? '');
        if($endpoint === ''){
            return '';
        }

        return rtrim($endpoint,'/').($bucket !== '' ? '/'.$bucket : '');
    }

    function getMinioUploadUrl($file,$configpri = null){
        $baseUrl=getMinioBaseUrl($configpri);
        if($baseUrl === ''){
            return '';
        }

        return buildStorageUrl($baseUrl,$file);
    }

    function buildUploadFileKey($filename,$dir = 'admin'){
        $pathinfo=pathinfo((string)$filename);
        $suffix=strtolower($pathinfo['extension'] ?? 'dat');
        $suffix=preg_replace('/[^a-z0-9]/','',$suffix);
        if($suffix === ''){
            $suffix='dat';
        }

        return trim($dir,'/').'/'.date('Ymd').'/'.time().mt_rand(10000,99999).'.'.$suffix;
    }

    function moveUploadFileToLocal($tmpName,$fileKey){
        $target=WEB_ROOT.'upload/'.ltrim($fileKey,'/');
        $dir=dirname($target);
        if(!is_dir($dir)){
            mkdir($dir,0777,true);
        }

        if(is_uploaded_file($tmpName)){
            return move_uploaded_file($tmpName,$target);
        }

        return rename($tmpName,$target);
    }

    function uploadFileToMinio($tmpName,$fileKey,$contentType = ''){
        $configpri=getConfigPri();
        $endpoint=trim($configpri['minio_endpoint'] ?? '');
        $bucket=trim($configpri['minio_bucket'] ?? '');
        $accessKey=trim($configpri['minio_access_key'] ?? '');
        $secretKey=trim($configpri['minio_secret_key'] ?? '');
        $region=trim($configpri['minio_region'] ?? 'us-east-1');

        if($endpoint === '' || $bucket === '' || $accessKey === '' || $secretKey === ''){
            return false;
        }

        $path=CMF_ROOT.'sdk/aws/aws-autoloader.php';
        if(!file_exists($path)){
            return false;
        }
        require_once($path);

        try{
            $sdk = new \Aws\Sdk([
                'region' => $region ?: 'us-east-1',
                'version' => 'latest',
                'endpoint' => rtrim($endpoint,'/'),
                'use_path_style_endpoint' => true,
                'credentials' => [
                    'key' => $accessKey,
                    'secret' => $secretKey,
                ],
            ]);
            $s3Client = $sdk->createS3();

            $params=[
                'Bucket' => $bucket,
                'Key' => ltrim($fileKey,'/'),
                'ACL' => 'public-read',
                'Body' => fopen($tmpName,'r'),
            ];
            if($contentType !== ''){
                $params['ContentType']=$contentType;
            }

            $s3Client->putObject($params);
            return true;
        }catch(\Throwable $e){
            return false;
        }
    }

    function adminUploadFiles($file,$cloudtype = null,$dir = 'admin'){
        if(empty($file) || !empty($file['error'])){
            return false;
        }

        $tmpName=$file['tmp_name'] ?? '';
        if($tmpName === '' || !file_exists($tmpName)){
            return false;
        }

        $fileKey=buildUploadFileKey($file['name'] ?? 'upload.dat',$dir);
        $storageType=getStorageTypeByCloudtype($cloudtype);

        if($storageType === 'minio'){
            $ok=uploadFileToMinio($tmpName,$fileKey,$file['type'] ?? '');
            if(!$ok){
                return false;
            }

            return [
                'url'=>getMinioUploadUrl($fileKey),
                'storage_path'=>'minio_'.$fileKey,
            ];
        }

        if(!moveUploadFileToLocal($tmpName,$fileKey)){
            return false;
        }

        return [
            'url'=>getLocalUploadUrl($fileKey),
            'storage_path'=>'local_'.$fileKey,
        ];
    }

    function set_upload_path($file){
        $file=trim((string)$file);
        if($file === ''){
            return $file;
        }
        if(preg_match('/^(local|minio)_/i',$file)){
            return $file;
        }

        $decoded=html_entity_decode(htmlspecialchars_decode($file));
        $host=rtrim(get_host(),'/');
        $localPrefix=$host.'/upload/';
        if(strpos($decoded,$localPrefix) === 0){
            return 'local_'.ltrim(substr($decoded,strlen($localPrefix)),'/');
        }
        if(strpos($decoded,'/upload/') === 0){
            return 'local_'.ltrim(substr($decoded,strlen('/upload/')),'/');
        }
        if(strpos($decoded,'upload/') === 0){
            return 'local_'.ltrim(substr($decoded,strlen('upload/')),'/');
        }

        $configpri=getConfigPri();
        $minioBase=getMinioBaseUrl($configpri);
        if($minioBase !== '' && strpos($decoded,rtrim($minioBase,'/').'/') === 0){
            return 'minio_'.ltrim(substr($decoded,strlen(rtrim($minioBase,'/').'/')),'/');
        }

        if(strpos($decoded,'http://') === 0 || strpos($decoded,'https://') === 0){
            return $decoded;
        }

        return getStorageType().'_'.ltrim($decoded,'/');
    }

    /**
     * 转化数据库保存的文件路径，为可以访问的url
     */
    function get_upload_path($file){
        if($file==''){
            return $file;
        }
        if(strpos($file,"http")===0){
            return html_entity_decode(htmlspecialchars_decode($file));
        }else if(strpos($file,"/")===0){
            $filepath=get_host().$file;
            return html_entity_decode(htmlspecialchars_decode($filepath));
        }else{
            $fileinfo=explode("_",$file,2);
            $storage_type=$fileinfo[0] ?? '';
            $path=$fileinfo[1] ?? $file;

            if($storage_type=='minio'){
                $filepath=getMinioUploadUrl($path);
                if($filepath === ''){
                    $filepath=getLocalUploadUrl($path);
                }
            }else{
                $filepath=getLocalUploadUrl($path);
            }

            return html_entity_decode(htmlspecialchars_decode($filepath));
        }
    }

    //声网云端播放器生成Header Authorization字段
    function getSwHttpAuthorization(){
        $configpri=getConfigPri();
        // 客户 ID
        // 需要设置环境变量 AGORA_CUSTOMER_KEY
        $customerKey = $configpri['sw_key_id'];
        // 客户密钥
        // 需要设置环境变量 AGORA_CUSTOMER_SECRET
        $customerSecret = $configpri['sw_key_secret'];
        // 拼接客户 ID 和客户密钥
        $credentials = $customerKey . ":" . $customerSecret;

        // 使用 base64 进行编码
        $base64Credentials = base64_encode($credentials);
        // 创建 authorization header
        $arr_header = "Authorization: Basic " . $base64Credentials;

        return $arr_header;
    }

    //生成声网通配Token
    function getShengWangRtcToken($uid,$stream){

        $key=$uid.'_'.$stream.'_swtoken';
        $token=getcaches($key);

        if(!$token){
            require_once CMF_ROOT.'sdk/shengwang/src/RtcTokenBuilder2.php';
            $configpri=getConfigPri();
            $appid=$configpri['sw_app_id'];
            $appCertificate=$configpri['sw_app_certificate'];
            $tokenExpirationInSeconds = 24*60*60; //24小时
            $privilegeExpirationInSeconds = 24*60*60; //24小时

            $channelName=$stream;
            $token = RtcTokenBuilder2::buildTokenWithUid($appid, $appCertificate, $channelName, $uid, RtcTokenBuilder2::ROLE_PUBLISHER, $tokenExpirationInSeconds, $privilegeExpirationInSeconds);

            if(!$token){
                $token='';
            }

            if($token){
            
                setcaches($key,$token,24*60*60-10*60);
            }
        }   
        
        return $token;

    }

    /**
     * 生成直播推拉流地址.
     * @param string $host 协议: http, rtmp, trtc
     * @param string $stream 流名, 可包含 .flv 或 .m3u8
     * @param int $type 0 播流, 1 推流
     */
    function PrivateKeyA($host,$stream,$type){
        $configpri=getConfigPri();
        $cdn_switch=$configpri['cdn_switch'] ?? '';

        switch((string)$cdn_switch){
            case '1':
                return PrivateKey_tx($host,$stream,$type);
            case '2':
                return PrivateKey_sw($host,$stream,$type);
            default:
                return '';
        }
    }

    /**
     * 声网融合 CDN 推拉流地址.
     */
    function PrivateKey_sw($host,$stream,$type){
        $configpri=getConfigPri();

        $now=time();
        $stream_arr=explode('.',$stream);
        $streamKey=$stream_arr[0] ?? '';
        $ext=$stream_arr[1] ?? 'flv';

        if((int)$type===1){
            $push_url=$configpri['sw_push_url'] ?? '';
            $push_url_key=$configpri['sw_push_key'] ?? '';
            $push_url_key_time=(int)($configpri['sw_push_length'] ?? 5);

            $url="rtmp://{$push_url}/live/{$streamKey}";
            if($push_url_key!==''){
                $txTime=$now + $push_url_key_time * 60;
                $url.="?ts={$txTime}&sign=".strtolower(md5("{$push_url_key}/live/{$streamKey}{$txTime}"));
            }

            return $url;
        }

        $pull_url=$configpri['sw_pull_url'] ?? '';
        $pull_url_key=$configpri['sw_pull_key'] ?? '';
        $pull_url_key_time=(int)($configpri['sw_pull_length'] ?? 5);

        if($host==='rtmp'){
            $url="rtmp://{$pull_url}/live/{$streamKey}";
        }else{
            $url="https://{$pull_url}/live/{$streamKey}.{$ext}";
        }

        if($pull_url_key!==''){
            $txTime=$now + $pull_url_key_time * 60;
            $url.="?ts={$txTime}&sign=".strtolower(md5("{$pull_url_key}/live/{$streamKey}.{$ext}{$txTime}"));
        }

        return $url;
    }

    /**
     * 腾讯云推拉流地址.
     */
    function PrivateKey_tx($host,$stream,$type){
        $configpri=getConfigPri();

        $stream_arr=explode('.',$stream);
        $streamKey=$stream_arr[0] ?? '';
        $ext=$stream_arr[1] ?? '';

        $streamkey_arr=explode('_',$streamKey);
        $uid=$streamkey_arr[0] ?? 0;

        if((int)$type===1){
            return getTxTrtcUrl($uid,$streamKey,1);
        }

        $pull=$configpri['tx_pull'] ?? '';
        $play_url_key=$configpri['tx_play_key'] ?? '';
        $play_safe_url='';
        $live_code=$streamKey;

        if(!empty($configpri['tx_play_key_switch'])){
            $play_auth_time=time() + (int)($configpri['tx_play_time'] ?? 0);
            $txPlayTime=dechex($play_auth_time);
            $txPlaySecret=md5($play_url_key.$live_code.$txPlayTime);
            $play_safe_url="?txSecret=".$txPlaySecret."&txTime=".$txPlayTime;
        }

        $url="http://{$pull}/live/".$live_code.".flv".$play_safe_url;
        if($ext){
            $url="http://{$pull}/live/".$live_code.".".$ext.$play_safe_url;
        }

        $configpub=getConfigPub();
        if(!empty($configpub['site']) && strstr($configpub['site'],'https')){
            $url=str_replace('http:','https:',$url);
        }

        return $url;
    }

    function txRtcUserSign($identifier,$appId,$appKey,$expire=86400){
        $now=time();
        $content="TLS.identifier:{$identifier}\nTLS.sdkappid:{$appId}\nTLS.time:{$now}\nTLS.expire:{$expire}\n";
        $payload=array(
            'TLS.ver'=>'2.0','TLS.identifier'=>(string)$identifier,'TLS.sdkappid'=>(int)$appId,
            'TLS.expire'=>(int)$expire,'TLS.time'=>$now,
            'TLS.sig'=>base64_encode(hash_hmac('sha256',$content,$appKey,true)),
        );
        return str_replace(array('+','/','='),array('*','-','_'),base64_encode(gzcompress(json_encode($payload),6)));
    }

    function getTxTrtcUrl($uid,$stream,$type=0){
        $configpri=getConfigPri();
        $appId=$configpri['tencentRTC_appid'] ?? '';
        $appKey=$configpri['tencentRTC_appkey'] ?? '';
        if($appId==='' || $appKey===''){
            return '';
        }
        $streamType=((int)$type===0) ? 'play' : 'push';
        $userSign=txRtcUserSign($uid,$appId,$appKey);
        return 'trtc://cloud.tencent.com/'.$streamType.'/'.$stream.'?sdkappid='.$appId.'&userId='.$uid.'&usersig='.$userSign.'&appscene=live';
    }

    //声网CurlPost提交
    function swCurlPost($sw_url,$sw_header,$sw_params){
        $ch = curl_init();    // 启动一个CURL会话
        curl_setopt($ch, CURLOPT_URL, $sw_url);     // 要访问的地址
        curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, false);  // 对认证证书来源的检查   // https请求 不验证证书和hosts
        curl_setopt($ch, CURLOPT_SSL_VERIFYHOST, false);  // 从证书中检查SSL加密算法是否存在
        //curl_setopt($ch, CURLOPT_USERAGENT, $_SERVER['HTTP_USER_AGENT']); // 模拟用户使用的浏览器
        //curl_setopt($ch, CURLOPT_FOLLOWLOCATION, 1); // 使用自动跳转
        //curl_setopt($ch, CURLOPT_AUTOREFERER, 1); // 自动设置Referer
        curl_setopt($ch, CURLOPT_POST, true); // 发送一个常规的Post请求
        curl_setopt($ch, CURLOPT_POSTFIELDS, json_encode($sw_params));     // Post提交的数据包
        curl_setopt($ch, CURLOPT_CONNECTTIMEOUT, 10);     // 设置超时限制防止死循环
        curl_setopt($ch, CURLOPT_TIMEOUT, 10);
        curl_setopt($ch, CURLOPT_HTTPHEADER, $sw_header); // 设置请求头部信息
        //curl_setopt($ch, CURLOPT_HEADER, 0); // 显示返回的Header区域内容
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);     // 获取的信息以文件流的形式返回 
        $curl_response = curl_exec($ch);
        
        curl_close($ch);
        $curl_response = json_decode($curl_response,true);
        return $curl_response;
    }

    //声网curl delte提交
    function swCurlDelete($sw_url,$sw_header,$sw_params){
        $ch = curl_init();

        curl_setopt ($ch,CURLOPT_URL,$sw_url);

        curl_setopt ($ch, CURLOPT_HTTPHEADER, $sw_header);

        curl_setopt ($ch, CURLOPT_RETURNTRANSFER, 1);

        curl_setopt ($ch, CURLOPT_CUSTOMREQUEST, "DELETE");

        curl_setopt($ch, CURLOPT_POSTFIELDS,$sw_params);

        $output = curl_exec($ch);

        curl_close($ch);

        $output = json_decode($output,true);
        return $output;
    }
