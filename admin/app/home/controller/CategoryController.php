<?php
// +----------------------------------------------------------------------
// | ThinkCMF [ WE CAN DO IT MORE SIMPLE ]
// +----------------------------------------------------------------------
// | Copyright (c) 2013-2014 http://www.thinkcmf.com All rights reserved.
// +----------------------------------------------------------------------
// | Author: Dean <zxxjjforever@163.com>
// +----------------------------------------------------------------------
namespace app\home\controller;

use app\common\controller\HomeBaseController;
use think\facade\Db;
/**
 * 分类页
 */
class CategoryController extends HomebaseController {
	
    //分类页
	public function index() {

        
		/* 主播列表 */
        $data = $this->request->param();
        
        $cat= $data['cat'] ?? '';

		switch($cat){
			case "1":
					$where="u.sex='1' and l.islive='1' ";
					$site_seo_title='国民男神';
					$current=1;	
					break;
			case "2":
					$where="u.sex='2' and l.islive='1' ";
					$site_seo_title='女神驾到';
					$current=2;
					break;

			default :
					$where="l.islive='1'";
					$site_seo_title='';
                    $current='0';
					break;			
			
			
		}
			
		$this->assign("current",$current);
		
		$count = Db::name("live")
                ->alias('l')
				->leftjoin("user u","u.id=l.uid")
				->where($where)
				->count();// 查询满足要求的总记录数
			

		// 进行分页数据查询 注意limit方法的参数要使用Page类的属性
		$lists=Db::name("live")
                ->alias('l')
				->field("u.user_nickname,u.avatar,l.thumb,l.uid,l.stream,l.title,l.city,l.islive")
				->leftjoin("user u","u.id=l.uid")
				->where($where)
				->order("l.showid desc")
				->paginate(20);

		$lists->each(function($v,$k){
            $v['avatar']=get_upload_path($v['avatar']);
            $v['thumb']=get_upload_path($v['thumb']);
            if($v['thumb']==''){
                $v['thumb']=$v['avatar'];
            }
            return $v;
        }); 
        $lists->appends($data);
        $page = $lists->render();
            
		$this->assign('lists',$lists);// 赋值数据集
		$this->assign('page',$page);// 赋值分页输出		
		$this->assign('site_seo_title',$site_seo_title);	
        return $this->fetch();
    }

    //分类直播
    public function classlive(){
    	$data = $this->request->param();
        
        $cat= $data['cat'] ?? '';
        $this->assign("current",$cat);


        $live_class=Db::name("live_class")
            ->order("list_order")
            ->select();

        $this->assign("live_class",$live_class);

        $classid=$data['classid'] ?? '';
        $this->assign("classid",$classid);

        $where=['islive'=>1,'live_type'=>0];

        if($classid){
        	$where['liveclassid']=$classid;
        }

        // 进行分页数据查询 注意limit方法的参数要使用Page类的属性

		$lists=Db::name("live")
            ->field("uid,thumb,stream,title,city,islive,type")
            ->where($where)
            ->order("starttime desc")
            ->paginate(5);

		$lists->each(function($v,$k){
            $userinfo=getUserInfo($v['uid']);
            
            $v['avatar']=$userinfo['avatar'];
			$v['avatar_thumb']=$userinfo['avatar_thumb'];
			$v['user_nickname']=$userinfo['user_nickname'];
			$v['signature']=$userinfo['signature'];
            
			$nums=zSize('user_'.$v['stream']);
			$v['nums']=(string)$nums;

			if($v['thumb']==""){
				$v['thumb']=$v['avatar'];
			}

            return $v;
        });

        $lists->appends($data);
        $page = $lists->render();
            
		$this->assign('lists',$lists);// 赋值数据集
		$this->assign('page',$page);// 赋值分页输出		
		$this->assign('site_seo_title','分类');	
        return $this->fetch();

    }

    public function hotlive(){
    	$data = $this->request->param();

    	$type=$data['type'];

    	if(!in_array($type, ['1','2'])){
    		$this->error('参数错误');
    	}
        
        $this->assign("current",'index');

        $where=['islive'=>1,'live_type'=>0];

        // 进行分页数据查询 注意limit方法的参数要使用Page类的属性

        //热门直播
        if($type==1){
        	$lists=Db::name("live")
	            ->field("uid,thumb,stream,title,city,islive,type,hotvotes,isrecommend,recommend_time")
	            ->where($where)
	            ->where(['ishot'=>1])
	            ->order("isrecommend desc,recommend_time desc,hotvotes desc,starttime desc")
	            ->paginate(20);

	        $site_seo_title='热门直播';
        }

        if($type==2){
        	$lists=Db::name("live")
	            ->field("uid,thumb,stream,title,city,islive,type")
	            ->where($where)
	            ->order("starttime desc")
	            ->paginate(20);


	        $site_seo_title='最新直播';
        }

		

		$lists->each(function($v,$k){
            $userinfo=getUserInfo($v['uid']);
            
            $v['avatar']=$userinfo['avatar'];
			$v['avatar_thumb']=$userinfo['avatar_thumb'];
			$v['user_nickname']=$userinfo['user_nickname'];
			$v['signature']=$userinfo['signature'];
            
			$nums=zSize('user_'.$v['stream']);
			$v['nums']=(string)$nums;

			if($v['thumb']==""){
				$v['thumb']=$v['avatar'];
			}

            return $v;
        });

        $lists->appends($data);
        $page = $lists->render();
            
		$this->assign('lists',$lists);// 赋值数据集
		$this->assign('page',$page);// 赋值分页输出		
		$this->assign('site_seo_title',$site_seo_title);	
        return $this->fetch();
    }


}


