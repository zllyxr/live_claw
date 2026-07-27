<?php

/**
 * 礼物
 */
namespace app\admin\controller;

use cmf\controller\AdminBaseController;
use think\facade\Db;

class GiftController extends AdminBaseController {
    protected function getTypes($k=''){
        $type=[
            '0'=>'普通礼物',
            '1'=>'豪华礼物',
			'3'=>'手绘礼物',
        ];
        if($k===''){
            return $type;
        }
        return $type[$k] ?? '';
    }
    protected function getMark($k=''){
        $mark=[
            '0'=>'普通',
            '1'=>'热门',
            '2'=>'守护',
            '3'=>'幸运',
        ];
        if($k===''){
            return $mark;
        }
        return $mark[$k] ?? '';
    }
    
    protected function getSwftype($k=''){
        $swftype=[
            '0'=>'GIF',
            '1'=>'SVGA',
        ];
        if($k===''){
            return $swftype;
        }
        return $swftype[$k] ?? '';
    }
    
    function index(){

    	$lists = Db::name("gift")
            ->where('type!=2')
			->order("list_order asc,id desc")
			->paginate(20);
        
        $lists->each(function($v,$k){
			$v['gifticon']=get_upload_path($v['gifticon']);
			$v['swf']=get_upload_path($v['swf']);
            $swfPath=parse_url($v['swf'], PHP_URL_PATH);
            $v['swf_name']=$swfPath ? basename($swfPath) : basename($v['swf']);
            return $v;           
        });
        
        $page = $lists->render();

    	$this->assign('lists', $lists);
    	$this->assign("page", $page);
    	$this->assign("type", $this->getTypes());
    	$this->assign("mark", $this->getMark());
    	$this->assign("swftype", $this->getSwftype());
    	
    	return $this->fetch();
    }
    
	function del(){
        
        $id = $this->request->param('id', 0, 'intval');

        //判断该礼物是不是被设置为了大转盘奖品
        $turntable_gift = Db::name("turntable")->where(['type'=>2,'type_val'=>$id])->find();

        if($turntable_gift){
            $this->error("请先将该礼物在大转盘-->奖品管理中删除");
        }

        //判断该礼物是不是被设置为了星球探宝奖品
        $xqtb_gift = Db::name("xqtb_gift")->where(['type'=>1,'giftid'=>$id])->find();
        
        if($xqtb_gift){
            $this->error("请先将该礼物在游戏管理-->星球探宝-->奖品列表中删除");
        }

        //判断该礼物是不是被设置为了幸运大转盘奖品
        $xydzp_gift = Db::name("xydzp_gift")->where(['type'=>1,'giftid'=>$id])->find();
        
        if($xydzp_gift){
            $this->error("请先将该礼物在游戏管理-->幸运大转盘-->奖品列表中删除");
        }


        $rs = DB::name('gift')->where("id={$id}")->delete();
        if(!$rs){
            $this->error("删除失败！");
        }

        //将首充规则里的热门礼物改为0
        Db::name("charge_rules")->where(['giftid'=>$id])->update(['giftid'=>0,'gift_num'=>0]);
        $key='getFirstChargeRules';
        delcache($key);

        $action="删除礼物：{$id}";
        setAdminLog($action);
                    
        $this->resetcache();
        $this->success("删除成功！");
        
	}
    
    /* 全站飘屏 */
    function plat(){
        
        $id = $this->request->param('id', 0, 'intval');
        $isplatgift = $this->request->param('isplatgift', 0, 'intval');
        
        $rs = DB::name('gift')->where("id={$id}")->update(['isplatgift'=>$isplatgift]);
        if(!$rs){
            $this->error("操作失败！");
        }
        
        $action="修改礼物：{$id}";
        setAdminLog($action);
                    
        $this->resetcache();
        $this->success("操作成功！");
	}
    
    //排序
    public function listOrder() {
        $model = DB::name('gift');
        parent::listOrders($model);
        
        $action="更新礼物排序";
        setAdminLog($action);
        
        $this->resetcache();
        $this->success("排序更新成功！");
    }

    function add(){
        
        $this->assign("type", $this->getTypes());
    	$this->assign("mark", $this->getMark());
    	$this->assign("swftype", $this->getSwftype());
        return $this->fetch();				
    }

	function addPost(){
		if ($this->request->isPost()) {
            
            $data = $this->request->param();
            
            $giftname=$data['giftname'];
            $giftname_en=$data['giftname_en'];

            if($giftname == ''){
                $this->error('请输入中文名称');
            }else{
                $check = Db::name('gift')->where("giftname='{$giftname}'")->find();
                if($check){
                    $this->error('中文名称已存在');
                }
            }

            if($giftname_en == ''){
                $this->error('请输入英文名称');
            }else{
                $check = Db::name('gift')->where("giftname_en='{$giftname_en}'")->find();
                if($check){
                    $this->error('英文名称已存在');
                }
            }
            
            $needcoin=$data['needcoin'];
            $gifticon=$data['gifticon'];
            
            if($needcoin==''){
                $this->error('请输入价格');
            }

            if(!is_numeric($needcoin)){
                $this->error('价格必须为数字');
            }

            if($needcoin<1){
                $this->error('价格必须为大于1的整数');
            }

            if(!$needcoin){
                $this->error('价格必须为大于1的整数');
            }

            if(floor($needcoin)!=$needcoin){
                $this->error('价格必须为大于1的整数');
            }

            if($gifticon==''){
                $this->error('请上传图片');
            }
            
            $swftype=$data['swftype'];
            $data['swf']=$data['gif'];
            if($swftype==1){
                $data['swf']=$data['svga'];
            }
            
            if($data['type']==1 && $data['swf']==''){
                $this->error('请上传动画效果');
            }

            $data['gifticon']=set_upload_path($data['gifticon']);
            if($data['gif']){
                $data['gif']=set_upload_path($data['gif']);
            }
            if($data['svga']){
                $data['svga']=set_upload_path($data['svga']);
            }
            
            $data['addtime']=time();
            unset($data['gif']);
            unset($data['svga']);
            
			$id = DB::name('gift')->insertGetId($data);
            if(!$id){
                $this->error("添加失败！");
            }
            
            $action="添加礼物：{$id}";
            setAdminLog($action);
            
            $this->resetcache();
            $this->success("添加成功！");
		}			
	}
    
    function edit(){

        $id  = $this->request->param('id', 0, 'intval');
        
        $data=Db::name('gift')
            ->where("id={$id}")
            ->find();
        if(!$data){
            $this->error("信息错误");
        }
        
        $this->assign("type", $this->getTypes());
    	$this->assign("mark", $this->getMark());
    	$this->assign("swftype", $this->getSwftype());
        $this->assign('data', $data);
        return $this->fetch();            
    }
    
	function editPost(){
		if ($this->request->isPost()) {
            
            $data = $this->request->param();

            $id=$data['id'];
            $giftname=$data['giftname'];
            $giftname_en=$data['giftname_en'];
            if($giftname == ''){
                $this->error('请输入中文名称');
            }else{
                $check = Db::name('gift')->where("giftname='{$giftname}' and id!={$id}")->find();
                if($check){
                    $this->error('中文名称已存在');
                }
            }
            
            if($giftname_en == ''){
                $this->error('请输入英文名称');
            }else{
                $check = Db::name('gift')->where("giftname_en='{$giftname_en}' and id!={$id}")->find();
                if($check){
                    $this->error('英文名称已存在');
                }
            }
            
            $needcoin=$data['needcoin'];
            $gifticon=$data['gifticon'];
            
            if($needcoin==''){
                $this->error('请输入价格');
            }

            if(!is_numeric($needcoin)){
                $this->error('价格必须为数字');
            }

            if($needcoin<1){
                $this->error('价格必须为大于1的整数');
            }

            if(!$needcoin){
                $this->error('价格必须为大于1的整数');
            }

            if(floor($needcoin)!=$needcoin){
                $this->error('价格必须为大于1的整数');
            }

            if($gifticon==''){
                $this->error('请上传图片');
            }

            $gifticon_old=$data['gifticon_old'];
            if($gifticon!=$gifticon_old){
                $data['gifticon']=set_upload_path($gifticon);
            }

            $gif=$data['gif'];
            if($gif){
                $gif_old=$data['gif_old'];
                if($gif!=$gif_old){
                    $data['gif']=set_upload_path($gif);
                }
            }

            $svga=$data['svga'];
            if($svga){
                $svga_old=$data['svga_old'];
                if($svga!=$svga_old){
                    $data['svga']=set_upload_path($svga);
                }
            }
            
            $swftype=$data['swftype'];
            $data['swf']=$data['gif'];
            if($swftype==1){
                $data['swf']=$data['svga'];
            }
            if($data['type']==1 && $data['swf']==''){
                $this->error('请上传动画效果');
            }
            unset($data['gif']);
            unset($data['svga']);
            unset($data['gifticon_old']);
            unset($data['gif_old']);
            unset($data['svga_old']);
            
            if($data['type']!=1){
                $data['isplatgift']=0;
            }
            
			$rs = DB::name('gift')->update($data);
            if($rs===false){
                $this->error("修改失败！");
            }
            
            $action="修改礼物：{$data['id']}";
            setAdminLog($action);
            
            $this->resetcache();
            $this->success("修改成功！");
		}	
	}
        
    function resetcache(){
        $key='getGiftList';
        
		$rs=DB::name('gift')
			->field("id,type,mark,giftname,giftname_en,needcoin,gifticon,sticker_id,swftime,isplatgift")
            ->where('type!=2')
			->order("list_order asc,id desc")
			->select();
        if($rs){
            setcaches($key,$rs);
        }else{
			delcache($key);
		}
        return 1;
    }
}
