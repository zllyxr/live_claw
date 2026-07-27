<?php

/**
 * 短视频--评论
 */
namespace app\admin\controller;

use cmf\controller\AdminBaseController;
use think\facade\Db;

class VideocomController extends AdminBaseController {


    public function index(){
    	
        $data = $this->request->param();
        $map=[];
		
        $videoid=isset($data['videoid']) ? $data['videoid']: '';
        if($videoid!=''){
            $map[]=['videoid','=',$videoid];
        }
        
        $lists = DB::name("video_comments")
            ->where($map)
            ->order('id desc')
            ->paginate(20);
        
        $lists->each(function($v,$k){
            $v['userinfo']=getUserInfo($v['uid']);
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
            $videoid=Db::name("video_comments")->where(['id'=>$id])->value("videoid");
            $result=DB::name("video_comments")->delete($id);                
            if($result){
                //删除评论喜欢
                Db::name("video_comments_like")->where("commentid={$id}")->delete();
                //更新视频评论数
                Db::name("video")->where("id={$videoid} and comments >0")->setDec("comments");
                //删除相关子评论
                $lists=Db::name("video_comments")
                ->where("commentid='{$id}' or parentid='{$id}'")
                ->select();

                foreach ($lists as $k => $v) {
                    Db::name("video_comments")->where("id={$v['id']}")->delete();
                    Db::name("video_comments_like")->where("commentid={$v['id']}")->delete();
                    Db::name("video")->where("id={$v['videoid']} and comments>0")->setDec("comments");
                }

                $this->success('删除成功');
             }else{
                $this->error('删除失败');
             }
        }else{              
            $this->error('数据传入失败！');
        }               
    }	
}
