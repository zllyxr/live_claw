<?php

/**
 * 直播列表
 */
namespace app\admin\controller;

use cmf\controller\AdminBaseController;
use think\facade\Db;

class LiveingController extends AdminBaseController {
    protected function getLiveClass(){

        $liveclass=Db::name("live_class")->order('list_order asc, id desc')->column('id,name');

        return $liveclass;
    }
    
    protected function getTypes($k=''){
        $type=[
            '0'=>'普通房间',
            '1'=>'密码房间',
            '2'=>'门票房间',
            '3'=>'计时房间',
        ];
        
        if($k===''){
            return $type;
        }
        return $type[$k];
    }
    
    function index(){
        $data = $this->request->param();
        $map=[];
        $map[]=['islive','=',1];
        $start_time= $data['start_time'] ?? '';
        $end_time= $data['end_time'] ?? '';
        
        if($start_time!=""){
           $map[]=['starttime','>=',strtotime($start_time)];
        }

        if($end_time!=""){
           $map[]=['starttime','<=',strtotime($end_time) + 60*60*24];
        }

        $uid=$data['uid'] ?? '';
        if($uid!=''){
            $lianguid=getLianguser($uid);
            if($lianguid){
                
                array_push($lianguid,$uid);
                $map[]=['uid','in',$lianguid];
            }else{
                $map[]=['uid','=',$uid];
            }
        } 

        $lists = Db::name("live")
                ->where($map)
                ->order("starttime DESC")
                ->paginate(20);

        $liveclass=$this->getLiveClass();

        array_push($liveclass,['id'=>0,'name'=>'默认分类']);
        
        $lists->each(function($v,$k) use ($liveclass){

             $v['userinfo']=getUserInfo($v['uid']);
             
             /* 本场总收益 */
             $totalcoin=$v['gift_totalcoin'];
             if(!$totalcoin){
                $totalcoin=0;
             }
             /* 送礼物总人数 */
             $total_nums=$v['gift_user_total'];
             if(!$total_nums){
                $total_nums=0;
             }
             /* 人均 */
             $total_average=0;
             if($totalcoin && $total_nums){
                $total_average=round($totalcoin/$total_nums,2);
             }
             
             /* 人数 */
            $nums=zSize('user_'.$v['stream']);
            
            $v['total_average']=$total_average;
            $v['nums']=$nums;
            
            if($v['isvideo']==0){
                $v['pull']=PrivateKeyA('rtmp',$v['stream'],0);
            }

            foreach ($liveclass as $k1 => $v1) {
                if($v['liveclassid']==$v1['id']){
                    $v['liveclassname']=$v1['name'];
                    break;
                }
            }
                
            return $v;           
        });

        
        $lists->appends($data);
        $page = $lists->render();


        $this->assign('lists', $lists);
        $this->assign("page", $page);
        $this->assign("type", $this->getTypes());
        
        return $this->fetch();
    }

    function del(){
        
        $uid = $this->request->param('uid', 0, 'intval');
        
        $rs = DB::name('live')->where("uid={$uid}")->delete();
        if(!$rs){
            $this->error("删除失败！");
        }
		
		$action="直播管理-直播列表删除UID：".$uid;
		setAdminLog($action);
        
        $this->success("删除成功！",url("liveing/index"));
            
    }
    
    function add(){

        $this->assign("liveclass", $this->getLiveClass());
        $this->assign("type", $this->getTypes());
        return $this->fetch();
    }
    
    function addPost(){
        if ($this->request->isPost()) {
            
            $data = $this->request->param();
            
            $nowtime=time();
            $uid=$data['uid'];
            
            $userinfo=DB::name('user')->field("ishot,isrecommend")->where(["id"=>$uid,"user_type"=>2])->find();
            if(!$userinfo){
                $this->error('用户不存在');
            }
            
            $liveinfo=DB::name('live')->field('uid,islive')->where(["uid"=>$uid])->find();
            if($liveinfo && $liveinfo['islive']==1){
                $this->error('该用户正在直播');
            }
            
            $pull=urldecode($data['pull']);
            $type=$data['type'];
            $type_val=$data['type_val'];
            $anyway=$data['anyway'];
            $liveclassid=$data['liveclassid'];
            $live_type=$data['live_type'];
            $voice_type=$data['voice_type'];
            $stream=$uid.'_'.$nowtime;
            $title='';

            if($live_type==1){
                $liveclassid=0;
                $type=0;
                $type_val='';
            }
            
            $data2=array(
                "uid"           =>$uid,
                "ishot"         =>$userinfo['ishot'],
                "isrecommend"   =>$userinfo['isrecommend'],
                "showid"        =>$nowtime,
                "starttime"     =>$nowtime,
                "title"         =>$title,
                "province"      =>'',
                "city"          =>'好像在火星',
                "stream"        =>$stream,
                "thumb"         =>'',
                "pull"          =>$pull,
                "lng"           =>'',
                "lat"           =>'',
                "type"          =>$type,
                "type_val"      =>$type_val,
                "isvideo"       =>1,
                "islive"        =>1,
                "anyway"        =>$anyway,
                "liveclassid"   =>$liveclassid,
                "live_type"     =>$live_type,
                "voice_type"     =>$voice_type
            );

            if($liveinfo){
                $rs = DB::name('live')->update($data2);
            }else{
                $rs = DB::name('live')->insertGetId($data2);
            }

            if($rs===false){
                $this->error("添加失败！");
            }

			$action="直播管理-直播列表添加UID：".$uid;
			setAdminLog($action);
            
            $this->success("添加成功！");
        }           
    }
    
    function edit(){
        $uid   = $this->request->param('uid', 0, 'intval');
        
        $data=Db::name('live')
            ->where("uid={$uid}")
            ->find();
        if(!$data){
            $this->error("信息错误");
        }

        $this->assign('data', $data);
        $this->assign("liveclass", $this->getLiveClass());
        $this->assign("type", $this->getTypes());
        
        return $this->fetch();
    }
    
    function editPost(){
        if ($this->request->isPost()) {
            
            $data  = $this->request->param();
            
            $data['pull']=urldecode($data['pull']);

            $live_type=$data['live_type'];
            if($live_type==1){
                $data['liveclassid']=0;
                $data['type']=0;
                $data['type_val']='';
            }
            $uid=$data['uid'];

            $rs = DB::name('live')->update($data);
            if($rs===false){
                $this->error("修改失败！");
            }
			
			$action="直播管理-直播列表修改UID：".$data['uid'];
			setAdminLog($action);
            $this->success("修改成功！");
        }
    }

}
