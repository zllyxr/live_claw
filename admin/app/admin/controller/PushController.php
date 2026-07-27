<?php

/**
 * 推送管理
 */
namespace app\admin\controller;

use cmf\controller\AdminBaseController;
use think\facade\Db;


class PushController extends AdminBaseController {

    function index(){
        $data = $this->request->param();
        $map=[];
        $map[]=['type','=','0'];
		
        $start_time= $data['start_time'] ?? '';
        $end_time= $data['end_time'] ?? '';
        
        if($start_time!=""){
           $map[]=['addtime','>=',strtotime($start_time)];
        }

        if($end_time!=""){
           $map[]=['addtime','<=',strtotime($end_time) + 60*60*24];
        }
        
        $keyword= $data['keyword'] ?? '';
        if($keyword!=''){
            $map[]=['touid|adminid','like',"%".$keyword."%"];
        }
        
    	$lists = DB::name("pushrecord")
            ->where($map)
            ->order('id desc')
            ->paginate(20);
        
        $lists->each(function($v,$k){
            $v['ip']=long2ip($v['ip']);
            return $v;
        });
        
        $lists->appends($data);
        $page = $lists->render();
        
        $this->assign('lists', $lists);
        
    	$this->assign("page", $page);
    	
    	return $this->fetch();
    }

    function del(){
        $id = $this->request->param('id', 0, 'intval');
        if($id){
            $result=DB::name("pushrecord")->delete($id);				
            if($result){
                $action="删除推送信息：{$id}";
                setAdminLog($action);
                
                $this->success('删除成功');
             }else{
                $this->error('删除失败');
             }
        }else{				
            $this->error('数据传入失败！');
        }				
    }	
    
	function add(){
        return $this->fetch();			
	}

	function addPost(){
        if ($this->request->isPost()) {
            
            $data      = $this->request->param();
            
			$content=$data['content'];
            $content_en=$data['content_en'];
			$touid=$data['touid'];
            
            $content=str_replace("\r","", $content);
            $content=str_replace("\n","", $content);

            $content_en=str_replace("\r","", $content_en);
            $content_en=str_replace("\n","", $content_en);
            
            $touid=str_replace("\r","", $touid);
            $touid=str_replace("\n","", $touid);
            $touid=preg_replace("/,|，/",",", $touid);
            
            if($content==''){
                $this->error('中文推送内容不能为空');
            }

            if($content_en==''){
                $this->error('英文推送内容不能为空');
            }

            $tpns_title=[
                'zh-cn'=>'系统消息',
                'en'=>'system information',
            ];
            $tpns_arr=[
                'zh-cn'=>$content,
                'en'=>$content_en
            ];

            if($touid!=''){
                $uids=preg_split('/,|，/',$touid);

                $new_uids=[];
                //查询靓号
                foreach ($uids as $k => $v) {
                    array_push($new_uids,$v);
                    $lianguid=getLianguser($v);
                    if($lianguid){
                        for ($i=0; $i <count($lianguid); $i++) { 
                           array_push($new_uids,$lianguid[$i]); 
                        }
                    }
                }

                //var_dump($new_uids);
                
                $send_user=implode(',', $new_uids);

                // var_dump($send_user);
                // die;

                $nums=count($new_uids);


                if($nums==1){

                    txMessageTpns('系统消息',$content,'single',$new_uids[0],[],json_encode(['type'=>2]),'zh-cn');
                    sleep(2);
                    txMessageTpns('system information',$content_en,'single',$new_uids[0],[],json_encode(['type'=>2]),'en');

                }else{
                    for($i=0;$i<$nums;){
                        $alias=array_slice($new_uids,$i,900);
                        $i+=900;
                        //type=2 非直播开播消息
                        txMessageTpns('系统消息',$content,'account_list',0,$alias,json_encode(['type'=>2]),'zh-cn');
                        sleep(2);
                        txMessageTpns('system information',$content_en,'account_list',0,$alias,json_encode(['type'=>2]),'en');

                    }
                }


            }else{
                $send_user='';
                txMessageTpns('系统消息',$content,'all',0,[],json_encode(['type'=>2]),'zh-cn');
                sleep(2);
                txMessageTpns('system information',$content_en,'all',0,[],json_encode(['type'=>2]),'en');
            }
            
            //写入记录
            $id=addSysytemInfo($send_user,$content,$content_en,0);
            if(!$id){
                $this->error("推送失败！");
            }
            
            $action="推送信息ID：{$id}";
            setAdminLog($action);
            
            $this->success("推送成功！");
		}
        
	}		
    
    function export(){
        
        $data = $this->request->param();
        $map=[];
        $map[]=['type','=','0'];
		
        $start_time= $data['start_time'] ?? '';
        $end_time= $data['end_time'] ?? '';
        
        if($start_time!=""){
           $map[]=['addtime','>=',strtotime($start_time)];
        }

        if($end_time!=""){
           $map[]=['addtime','<=',strtotime($end_time) + 60*60*24];
        }
        
        $keyword= $data['keyword'] ?? '';
        if($keyword!=''){
            $map[]=['touid|adminid','like',"%".$keyword."%"];
        }
        
    	$xlsData = DB::name("pushrecord")
            ->where($map)
            ->order('id desc')
            ->select()
            ->toArray();

        if(empty($xlsData)){
            $this->error("数据为空");
        }
            
        foreach ($xlsData as $k => $v)
        {
            if(!$v['touid']){
                $xlsData[$k]['touid']='所有会员';
                
            }  
			$xlsData[$k]['ip']=long2ip($v['ip']);
			$xlsData[$k]['addtime']=date("Y-m-d H:i:s",$v['addtime']);
        }
        
        $action="导出推送信息：".DB::name("pushrecord")->getLastSql();
        setAdminLog($action);
        $xlsName='推送记录';
        $cellName = array('A','B','C','D','E','F');
        $xlsCell  = array(
            array('id','序号'),
            array('admin','管理员'),
            array('ip','IP'),
            array('touid','推送对象'),
            array('content','推送内容'),
            array('addtime','提交时间'),
        );
        exportExcel($xlsName,$xlsCell,$xlsData,$cellName);
    }
    
}
