<?php
// +----------------------------------------------------------------------
// | ThinkCMF [ WE CAN DO IT MORE SIMPLE ]
// +----------------------------------------------------------------------
// | Copyright (c) 2013-present http://www.thinkcmf.com All rights reserved.
// +----------------------------------------------------------------------
// | Licensed ( http://www.apache.org/licenses/LICENSE-2.0 )
// +----------------------------------------------------------------------
// | Author: Powerless < wzxaini9@gmail.com>
// +----------------------------------------------------------------------

namespace app\user\controller;

use app\user\model\UserModel;
use cmf\controller\AdminBaseController;
use think\facade\Db;

/**
 * Class AdminIndexController
 * @package app\user\controller
 *
 * @adminMenuRoot(
 *     'name'   =>'用户管理',
 *     'action' =>'default',
 *     'parent' =>'',
 *     'display'=> true,
 *     'order'  => 10,
 *     'icon'   =>'group',
 *     'remark' =>'用户管理'
 * )
 *
 * @adminMenuRoot(
 *     'name'   =>'用户组',
 *     'action' =>'default1',
 *     'parent' =>'user/AdminIndex/default',
 *     'display'=> true,
 *     'order'  => 10000,
 *     'icon'   =>'',
 *     'remark' =>'用户组'
 * )
 */
class AdminIndexController extends AdminBaseController{

    //读取国家代号
    public function getCountrys(){

        $key='getCountrys';
        $info=getcaches($key);
        //$info=false;
        if(!$info){

            $country=CMF_ROOT.'data/config/country.json';
            // 从文件中读取数据到PHP变量
            $json_string = file_get_contents($country);
            // 用参数true把JSON字符串强制转成PHP数组
            $data = json_decode($json_string, true);

            $info=$data['country']; //国家

            setcaches($key,$info);
        }

        $country_list=[];
        foreach ($info as $k => $v) {
            $arr=$v['lists'];
            foreach ($arr as $k1 => $v1) {
                $country_list[]=$v1;
            }
        }

        return $country_list;
    }

    protected function isIndexAjaxRequest($data){

        return isset($data['is_ajax']) && (string)$data['is_ajax'] === '1';
    }

    protected function buildIndexMap($data){

        $map=[];
        $map[]=['user_type','=',2];

        $start_time= $data['start_time'] ?? '';
        $end_time= $data['end_time'] ?? '';

        if($start_time!=""){
            $map[]=['create_time','>=',strtotime($start_time)];
        }

        if($end_time!=""){
            $map[]=['create_time','<=',strtotime($end_time) + 60*60*24];
        }

        $iszombie= $data['iszombie'] ?? '';
        if($iszombie!=''){
            $map[]=['iszombie','=',$iszombie];
        }

        $isban= $data['isban'] ?? '';
        if($isban!=''){
            if($isban==1){
                $map[]=['user_status','=',0];
            }else{
                $map[]=['user_status','<>',0];
            }
        }

        $disable= $data['disable'] ?? '';
        if($disable!=''){
            if($disable==1){
                $map[]=['end_bantime','>',time()];
            }else{
                $map[]=['end_bantime','=',0];
            }
        }

        $issuper= $data['issuper'] ?? '';
        if($issuper!=''){
            $map[]=['issuper','=',$issuper];
        }

        $source= $data['source'] ?? '';
        if($source!=''){
            $map[]=['source','=',$source];
        }

        $ishot= $data['ishot'] ?? '';
        if($ishot!=''){
            $map[]=['ishot','=',$ishot];
        }

        $iszombiep= $data['iszombiep'] ?? '';
        if($iszombiep!=''){
            $map[]=['iszombiep','=',$iszombiep];
        }

        $keyword= $data['keyword'] ?? '';
        if($keyword!=''){
            $map[]=['user_login|user_nickname','like','%'.$keyword.'%'];
        }

        $uid= $data['uid'] ?? '';
        if($uid!=''){

            $lianguid=getLianguser($uid);

            if($lianguid){

                array_push($lianguid,$uid);

                $map[]=['id','in',$lianguid];

            }else{
                $map[]=['id','=',$uid];
            }
        }

        return $map;
    }

    protected function getIndexOrder($data){

        $order = "id desc";
        $field = $data['field'] ?? '';
        $direction = strtolower($data['order'] ?? '');
        $sortFields = [
            'id',
            'coin',
            'consumption',
            'votes',
            'votestotal',
            'balance',
            'balance_total',
            'balance_consumption',
            'create_time',
            'last_login_time'
        ];

        if(in_array($field, $sortFields, true) && in_array($direction, ['asc', 'desc'], true)){
            $order = $field . ' ' . $direction;
        }

        return $order;
    }

    protected function formatIndexUserRows($rows){

        if(is_object($rows) && method_exists($rows, 'toArray')){
            $rows = $rows->toArray();
        }

        $country_list=$this->getCountrys();
        $nowtime=time();
        $list=[];

        foreach ($rows as $row) {
            $list[]=$this->formatIndexUserRow($row, $country_list, $nowtime);
        }

        return $list;
    }

    protected function formatIndexUserRow($row, $country_list, $nowtime){

        if(is_object($row) && method_exists($row, 'toArray')){
            $row = $row->toArray();
        }

        if(!is_array($row)){
            $row = (array)$row;
        }

        $uid = isset($row['id']) ? (int)$row['id'] : 0;
        $row['code']=Db::name("agent_code")->where('uid', $uid)->value('code');
        $row['user_login']=m_s($row['user_login'] ?? '');
        $row['mobile']=m_s($row['mobile'] ?? '');
        $row['user_email']=m_s($row['user_email'] ?? '');
        $row['avatar']=get_upload_path($row['avatar'] ?? '');

        $row['live_ban']=0;
        $row['ban_addtime']='--';
        $row['ban_type']='';
        $row['ban_endtime']='--';
        $row['nowtime']=$nowtime;
        $live_ban=Db::name("live_ban")->field("addtime,type,endtime")->where(['liveuid'=>$uid])->find();
        if($live_ban){

            if( ($live_ban['endtime'] > 0) && ($live_ban['endtime']<=$nowtime) ){
                Db::name("live_ban")->where(['liveuid'=>$uid])->delete();
            }else{
                $row['live_ban']=1;
                $row['ban_addtime']=date("Y-m-d H:i",$live_ban['addtime']);
                $row['ban_type']=$live_ban['type']!='all'?$live_ban['type']:'永久';
                if($live_ban['endtime']>0){
                    $row['ban_endtime']=date("Y-m-d H:i",$live_ban['endtime']);
                }
            }
        }

        $row['country_name']='';
        foreach ($country_list as $k1 => $v1) {
            if(isset($row['country_code']) && $row['country_code']==$v1['tel']){
                $row['country_name']=$v1['name'];
                break;
            }
        }

        return $row;
    }

    protected function getIndexJson($map, $nums, $data){

        $page = $this->request->param('page', 1, 'intval');
        $limit = $this->request->param('limit', 20, 'intval');
        $page = max(1, $page);
        $limit = max(1, min(100, $limit));

        $rows = UserModel::where($map)
            ->order($this->getIndexOrder($data))
            ->page($page, $limit)
            ->select();

        return json([
            'code'  => 0,
            'msg'   => '',
            'count' => (int)$nums,
            'data'  => $this->formatIndexUserRows($rows),
            'nums'  => (int)$nums,
        ]);
    }

    /**
     * @desc 后台本站用户列表
     * @return mixed
     */
    public function index(){

        $data = $this->request->param();
        $is_ajax=$this->isIndexAjaxRequest($data);

        if(!$is_ajax){
            $content = hook_one('user_admin_index_view');

            if (!empty($content)) {
                return $content;
            }
        }

        $map=$this->buildIndexMap($data);

        $nums=UserModel::where($map)->count();

        if($is_ajax){
            return $this->getIndexJson($map, $nums, $data);
        }

        $configpub=getConfigPub();

        $list = UserModel::where($map)
            ->order("id desc")
            ->paginate(20);

        $country_list=$this->getCountrys();
        $nowtime=time();
        $list->each(function($v,$k) use ($country_list, $nowtime){
            $row=$this->formatIndexUserRow($v, $country_list, $nowtime);
            foreach ($row as $field => $value) {
                $v[$field]=$value;
            }

            return $v;
        });

        $list->appends($data);
        // 获取分页显示
        $page = $list->render();
        $this->assign('list', $list);
        $this->assign('page', $page);
        $this->assign('nums', $nums);
        $this->assign('nowtime', time());
        $this->assign('name_coin', $configpub['name_coin']);
        $this->assign('name_votes', $configpub['name_votes']);
        $this->assign('country_list', json_encode($this->getCountrys()));
        // 渲染模板输出
        return $this->fetch();
    }

    /**
     * @desc 添加用户
     * @return void
     */
    public function add(){
        $this->assign('country_list', $this->getCountrys());
        return $this->fetch();
    }

    /**
     * @desc 用户添加保存
     * @return void
     */
    public function addPost(){
        if ($this->request->isPost()) {

            $data = $this->request->param();

            $country_code=$data['country_code'];

            if(!$country_code){
                $this->error(lang("请选择国家/地区"));
            }

            $user_login=$data['user_login'];

            if($user_login==""){
                $this->error("请填写手机号");
            }

            if($country_code=='86'){
                if(!checkMobile($user_login)){
                    $this->error("请填写正确手机号");
                }
            }

            $isexist=UserModel::where(['user_login'=>$user_login,'country_code'=>$country_code])->value('id');
            if($isexist){
                $this->error("该账号已存在，请更换");
            }

            $data['mobile']=$user_login;

            $user_pass=$data['user_pass'];
            if($user_pass==""){
                $this->error("请填写密码");
            }

            if(!passcheck($user_pass)){
                $this->error("密码为6-20位字母数字组合");
            }

            $data['user_pass']=cmf_password($user_pass);

            $user_nickname=$data['user_nickname'];
            if($user_nickname==""){
                $this->error("请填写昵称");
            }

            $avatar=$data['avatar'];
            $avatar_thumb=$data['avatar_thumb'];
            if( ($avatar=="" || $avatar_thumb=='' ) && ($avatar!="" || $avatar_thumb!='' )){
                $this->error("请同时上传头像 和 头像小图  或 都不上传");
            }

            if($avatar=='' && $avatar_thumb==''){
                $data['avatar']='/default.jpg';
                $data['avatar_thumb']='/default_thumb.jpg';
            }

            $data['avatar']=set_upload_path($data['avatar']);
            $data['avatar_thumb']=set_upload_path($data['avatar_thumb']);

            $data['user_type']=2;
            $data['create_time']=time();

            $id = UserModel::insertGetId($data);
            if(!$id){
                $this->error("添加失败！");
            }

            $action="添加会员：{$id}";
            setAdminLog($action);

            $this->success("添加成功！");

        }
    }

    /**
     * @desc 编辑用户信息
     */
    public function edit(){

        $id   = $this->request->param('id', 0, 'intval');

        $data=UserModel::where("id={$id}")
            ->find();
        if(!$data){
            $this->error("信息错误");
        }

        $data['user_login']=m_s($data['user_login']);
        $this->assign('country_list', $this->getCountrys());
        $this->assign('data', $data);
        return $this->fetch();
    }

    /**
     * @desc 用户信息编辑提交
     * @return void
     */
    public function editPost(){
        if ($this->request->isPost()) {

            $data = $this->request->param();

            //获取用户的状态
            $user_status=UserModel::where("id={$data['id']}")->value("user_status");
            $login_type=UserModel::where("id={$data['id']}")->value("login_type");

            /*if($login_type=='phone'){

                $country_code=$data['country_code'];
                if(!$country_code){
                    $this->error(lang("请选择国家/地区"));
                }

                if($country_code=='86'){
                    if(!checkMobile($data['user_login'])){
                        $this->error("中国大陆手机号应为11位");
                    }
                }   
            }*/

            $user_pass=$data['user_pass'];
            if($user_pass!=""){
                if(!passcheck($user_pass)){
                    $this->error("密码为6-20位字母数字组合");
                }

                $data['user_pass']=cmf_password($user_pass);
            }else{
                unset($data['user_pass']);
            }

            $user_nickname=$data['user_nickname'];
            if($user_nickname==""){
                $this->error("请填写昵称");
            }

            if($user_status!=3){
                if(strstr($user_nickname,'已注销')!==false){
                    $this->error("非注销用户昵称不能包含已注销");
                }
            }

            if(mb_substr($user_nickname, 0,1)=="="){
                $this->error("昵称内容非法");
            }

            $avatar=$data['avatar'];
            $avatar_thumb=$data['avatar_thumb'];

            if( ($avatar=="" || $avatar_thumb=='' ) && ($avatar!="" || $avatar_thumb!='' )){
                $this->error("请同时上传头像 和 头像小图  或 都不上传");
            }

            if($avatar=='' && $avatar_thumb==''){
                $data['avatar']='/default.jpg';
                $data['avatar_thumb']='/default_thumb.jpg';
            }

            $avatar_old=$data['avatar_old'];
            if($avatar_old!=$avatar){
                $data['avatar']=set_upload_path($data['avatar']);
            }

            $avatar_thumb_old=$data['avatar_thumb_old'];
            if($avatar_thumb_old!=$avatar_thumb){
                $data['avatar_thumb']=set_upload_path($data['avatar_thumb']);
            }

            unset($data['avatar_old']);
            unset($data['avatar_thumb_old']);
            unset($data['user_login']);

            $rs = UserModel::update($data);
            if($rs===false){
                $this->error("修改失败！");
            }

            $action="修改会员信息：{$data['id']}";
            setAdminLog($action);

            //查询用户信息存入缓存中
            $info=UserModel::field('id,user_nickname,avatar,avatar_thumb,sex,signature,consumption,votestotal,province,city,birthday,user_status,issuper,location')
                ->where("id={$data['id']} and user_type=2")
                ->find();


            if($info){
                setcaches("userinfo_".$data['id'],$info);
            }

            $this->success("修改成功！");
        }
    }

    /**
     * @desc 删除用户
     * @return void
     */
    public function del(){

        $id = $this->request->param('id', 0, 'intval');

        $user_login = UserModel::where(["id"=>$id,"user_type"=>2])->value('user_login');
        $rs = UserModel::where(["id"=>$id,"user_type"=>2])->delete();
        if(!$rs){
            $this->error("删除失败！");
        }

        $action="删除会员：{$id} - {$user_login}";
        setAdminLog($action);

        // 删除认证
        DB::name("user_auth")->where("uid='{$id}'")->delete();
        // 删除直播记录
        DB::name("live")->where("uid='{$id}'")->delete();
        DB::name("live_record")->where("uid='{$id}'")->delete();
        // 删除房间管理员
        DB::name("live_manager")->where("uid='{$id}' or liveuid='{$id}'")->delete();

        //  删除黑名单
        DB::name("user_black")->where("uid='{$id}' or touid='{$id}'")->delete();
        // 删除关注记录
        DB::name("user_attention")->where("uid='{$id}' or touid='{$id}'")->delete();

        // 删除僵尸
        DB::name("user_zombie")->where("uid='{$id}'")->delete();
        // 删除超管
        DB::name("user_super")->where("uid='{$id}'")->delete();
        // 删除会员
        DB::name("vip_user")->where("uid='{$id}'")->delete();

        // 删除分销关系
        DB::name("agent")->where("uid='{$id}' or one_uid={$id}")->delete();
        // 删除分销邀请码
        DB::name("agent_code")->where("uid='{$id}'")->delete();
        // 删除分销收益
        DB::name("agent_profit")->where("uid='{$id}'")->delete();
        // 删除分销收益记录
        DB::name("agent_profit_recode")->where("one_uid='{$id}'")->delete();

        // 删除坐骑
        DB::name("car_user")->where("uid='{$id}'")->delete();


        // 删除钱包账号
        DB::name("cash_account")->where("uid='{$id}'")->delete();
        // 删除自己的标签
        DB::name("label_user")->where("touid='{$id}'")->delete();

        // 删除背包
        DB::name("backpack")->where("uid='{$id}'")->delete();

        // 删除动态 相关
        $dynamicids=DB::name("dynamic")->where("uid='{$id}'")->column('id');
        DB::name("dynamic")->where("uid='{$id}'")->delete();

        DB::name("dynamic_comments")->where('dynamicid','in',$dynamicids)->delete();
        DB::name("dynamic_comments_like")->where('dynamicid','in',$dynamicids)->delete();
        DB::name("dynamic_like")->where('dynamicid','in',$dynamicids)->delete();
        DB::name("dynamic_report")->where('dynamicid','in',$dynamicids)->delete();
        DB::name("dynamic_report")->where('touid','=',$id)->delete();

        // 删除反馈
        DB::name("feedback")->where('uid','=',$id)->delete();

        // 删除守护
        DB::name("guard_user")->where('uid','=',$id)->delete();
        DB::name("guard_user")->where('liveuid','=',$id)->delete();

        // 删除靓号
        DB::name("liang")->where('uid','=',$id)->delete();

        // 删除踢人
        DB::name("live_kick")->where('uid','=',$id)->delete();

        // 删除踢人
        DB::name("live_kick")->where('liveuid','=',$id)->delete();

        // 删除禁言
        DB::name("live_shut")->where('uid','=',$id)->delete();

        // 删除音乐收藏
        DB::name("music_collection")->where('uid','=',$id)->delete();

        // 删除举报
        DB::name("report")->where('touid','=',$id)->delete();

        // 删除禁用
        DB::name("user_banrecord")->where("uid='{$id}'")->delete();

        // 删除登录
        DB::name("user_sign")->where("uid='{$id}'")->delete();

        // 删除映票记录
        DB::name("user_voterecord")->where("uid='{$id}'")->delete();

        // 删除积分
        DB::name("user_scorerecord")->where("touid='{$id}'")->delete();

        // 删除视频 相关
        $videoids=DB::name("video")->where("uid='{$id}'")->column('id');
        DB::name("video")->where("uid='{$id}'")->delete();

        DB::name("video_black")->where('videoid','in',$videoids)->delete();
        DB::name("video_black")->where('uid','=',$id)->delete();

        DB::name("video_comments")->where('videoid','in',$videoids)->delete();
        DB::name("video_comments_like")->where('videoid','in',$videoids)->delete();

        DB::name("video_like")->where('videoid','in',$videoids)->delete();
        DB::name("video_like")->where('uid','=',$id)->delete();

        DB::name("video_report")->where('videoid','in',$videoids)->delete();
        DB::name("video_report")->where('touid','=',$id)->delete();
        // 删除视频 相关


        //删除家族关系
        DB::name("family_user")->where("uid='{$id}'")->delete();
        // 家族长处理
        $isexist=DB::name("family")->field("id")->where("uid={$id}")->find();
        if($isexist){
            $data=array(
                'state'=>3,
                'signout'=>2,
                'signout_istip'=>2,
            );
            DB::name("family_user")->where("familyid={$isexist['id']}")->update($data);
            DB::name("family_profit")->where("familyid={$isexist['id']}")->delete();
            DB::name("family_profit")->where("id={$isexist['id']}")->delete();
        }



        //删除用户美颜参数设置
        Db::name("user_beauty_params")->where("uid={$id}")->delete();
        //删除用户语音聊天室上麦申请
        Db::name("voicelive_applymic")->where("uid={$id} or liveuid={$id}")->delete();
        //删除用户语音聊天室上麦表
        Db::name("voicelive_mic")->where("uid={$id} or liveuid={$id}")->delete();
        //删除粉丝关注信息
        Db::name("user_attention_messages")->where("uid={$id} or touid={$id}")->delete();

        //

        delcache("userinfo_".$id,"token_".$id);

        //删除极光IM用户id
        //delIMUser($id);

        $this->success("删除成功！");

    }

    /**
     * @desc 禁用时间
     * @return void
     */
    public function setBan(){

        $id = $this->request->param('id', 0, 'intval');
        $reason = $this->request->param('reason');
        $ban_long = $this->request->param('ban_long');

        if(!$id){
            $this->error('数据传入失败！');
        }

        if($ban_long){
            $ban_long=strtotime($ban_long);
        }else{
            $ban_long=0;
        }

        $data=[
            'uid'=>$id,
            'ban_long'=>$ban_long,
            'ban_reason'=>$reason,
            'addtime'=>time(),
        ];

        $result = Db::name("user_banrecord")->where(["uid" => $id])->update($data);
        if(!$result){
            $result=Db::name("user_banrecord")->insert($data);
        }
        if(!$result){
            $this->error('操作失败！');
        }

        UserModel::where(["id" => $id])->update(['end_bantime'=>$ban_long]);

        $action="禁用会员：{$id}";
        setAdminLog($action);

        $live=Db::name("live")->field("uid")->where("islive='1'")->select()->toArray();
        foreach($live as $k=>$v){
            hSet($v['uid'] . 'shutup',$id,1);
        }

        $this->success("操作成功！");
    }

    /**
     * @desc 本站用户启用
     * @return void
     */
    public function cancelBan(){

        $id = input('param.id', 0, 'intval');
        if ($id) {
            UserModel::where(["id" => $id, "user_type" => 2])->update(['user_status' => 1,'end_bantime'=>0]);
            $action="启用会员：{$id}";
            setAdminLog($action);
            $this->success("会员启用成功！", '');
        } else {
            $this->error('数据传入失败！');
        }
    }

    /**
     * @desc 本站用户拉黑
     * @return void
     */
    public function ban(){

        $id = input('param.id', 0, 'intval');
        if ($id) {
            $result = UserModel::where(["id" => $id, "user_type" => 2])->update(['user_status' => 0]);
            if ($result) {
                $this->success("会员拉黑成功！", "adminIndex/index");
            } else {
                $this->error('会员拉黑失败,会员不存在,或者是管理员！');
            }
        } else {
            $this->error('数据传入失败！');
        }
    }

    /**
     * @desc 解封直播间
     * @return void
     */
    public function cancelLiveBan(){

        $id = $this->request->param('id', 0, 'intval');
        if ($id) {

            Db::name("live_ban")->where(["liveuid" => $id])->delete();

            $action="解封直播会员：{$id}";
            setAdminLog($action);

            $this->success("会员解封成功！");
        } else {
            $this->error('数据传入失败！');
        }
    }

    /**
     * @desc 设置超管
     * @return void
     */
    public function setsuper(){

        $id = $this->request->param('id', 0, 'intval');
        $issuper = $this->request->param('issuper', 0, 'intval');

        $rs = UserModel::where("id={$id}")->update(['issuper'=>$issuper]);
        if(!$rs){
            $this->error("操作失败！");
        }

        if($issuper==1){
            $action="设置超管会员：{$id}";
            $isexist=DB::name("user_super")->where("uid={$id}")->find();
            if(!$isexist){
                DB::name("user_super")->insert(array("uid"=>$id,'addtime'=>time()));
            }

            hSet('super',$id,'1');
        }else{
            $action="取消超管会员：{$id}";

            DB::name("user_super")->where("uid='{$id}'")->delete();
            hDel('super',$id);
        }

        setAdminLog($action);

        $this->success("操作成功！");

    }

    /**
     * @desc 设置热门
     * @return void
     */
    public function sethot(){

        $id = $this->request->param('id', 0, 'intval');
        $ishot = $this->request->param('ishot', 0, 'intval');

        $rs = UserModel::where("id={$id}")->update(['ishot'=>$ishot]);
        if(!$rs){
            $this->error("操作失败！");
        }
        DB::name("live")->where(array("uid"=>$id))->update(['ishot'=>$ishot]);
        if($ishot==1){
            $action="设置热门会员：{$id}";
        }else{
            $action="取消热门会员：{$id}";
        }

        setAdminLog($action);

        $this->success("操作成功！");

    }

    /**
     * @desc 推荐
     * @return void
     */
    public function setrecommend(){

        $id = $this->request->param('id', 0, 'intval');
        $isrecommend = $this->request->param('isrecommend', 0, 'intval');

        $data=[
            'isrecommend'=>$isrecommend,
            'recommend_time'=>time(),
        ];

        $rs = UserModel::where("id={$id}")->update($data);
        if(!$rs){
            $this->error("操作失败！");
        }
        DB::name("live")->where(array("uid"=>$id))->update($data);
        if($isrecommend==1){
            $action="设置推荐会员：{$id}";
        }else{
            $action="取消推荐会员：{$id}";
        }

        setAdminLog($action);

        $this->success("操作成功！");

    }

    /**
     * @desc 开启/关闭僵尸粉
     * @return void
     */
    public function setzombie(){

        $id = $this->request->param('id', 0, 'intval');
        $iszombie = $this->request->param('iszombie', 0, 'intval');

        $rs = UserModel::where("id={$id}")->update(['iszombie'=>$iszombie]);
        if(!$rs){
            $this->error("操作失败！");
        }

        if($iszombie==1){
            $action="开启会员僵尸粉：{$id}";
        }else{
            $action="关闭会员僵尸粉：{$id}";
        }

        setAdminLog($action);

        $this->success("操作成功！");

    }

    /**
     * @desc 设置僵尸粉
     * @return void
     */
    function setzombiep(){

        $id = $this->request->param('id', 0, 'intval');
        $iszombiep = $this->request->param('iszombiep', 0, 'intval');

        $rs = UserModel::where("id={$id}")->update(['iszombiep'=>$iszombiep]);
        if(!$rs){
            $this->error("操作失败！");
        }

        if($iszombiep==1){
            $action="开启僵尸粉会员：{$id}";
            $isexist=DB::name("user_zombie")->where("uid={$id}")->find();
            if(!$isexist){
                DB::name("user_zombie")->insert(array("uid"=>$id));
            }
        }else{
            $action="关闭僵尸粉会员：{$id}";

            DB::name("user_zombie")->where("uid='{$id}'")->delete();
        }

        setAdminLog($action);

        $this->success("操作成功！");

    }
    /**
     * @desc 一键开启/关闭僵尸粉
     * @return void
     */
    function setzombieall(){

        $iszombie = $this->request->param('iszombie', 0, 'intval');

        $rs = DB::name('user')->where('user_type=2')->update(['iszombie'=>$iszombie]);
        if(!$rs){
            $this->error("操作失败！");
        }

        if($iszombie==1){
            $action="开启全部会员僵尸粉";
        }else{
            $action="关闭全部会员僵尸粉";
        }

        setAdminLog($action);

        $this->success("操作成功！");

    }

    /**
     * @desc 批量设置/取消僵尸粉
     * @return void
     */
    function setzombiepall(){
        $data = $this->request->param();
        $ids = $data['ids'];
        if(!$ids){
            $this->error("信息错误！");
        }

        $tids=join(",",$ids);
        $iszombiep = $this->request->param('iszombiep', 0, 'intval');

        $rs = UserModel::where('id', 'in', $ids)->update(['iszombiep'=>$iszombiep]);
        if(!$rs){
            $this->error("操作失败！");
        }

        if($iszombiep==1){
            $action="开启僵尸粉会员：{$tids}";
            foreach($ids as $k=>$v){
                $isexist=DB::name("user_zombie")->where("uid={$v}")->find();
                if(!$isexist){
                    DB::name("user_zombie")->insert(array("uid"=>$v));
                }
            }

        }else{
            $action="关闭僵尸粉会员：{$tids}";

            DB::name("user_zombie")->where('uid', 'in', $ids)->delete();
        }

        setAdminLog($action);

        $this->success("操作成功！");

    }
}
