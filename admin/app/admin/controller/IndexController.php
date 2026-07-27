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

use cmf\controller\AdminBaseController;
use think\facade\Db;
use app\admin\model\AdminMenuModel;
use app\admin\service\AdminMenuService;

class IndexController extends AdminBaseController
{

    public function initialize()
    {
        $adminSettings = cmf_get_option('admin_settings');
        if (empty($adminSettings['admin_password']) || $this->request->pathinfo() == $adminSettings['admin_password'] || true) {
            $adminId = cmf_get_current_admin_id();
            if (empty($adminId)) {
                session("__LOGIN_BY_CMF_ADMIN_PW__", 1);//设置后台登录加密码
            }
        }

        parent::initialize();
    }

    /**
     * 后台首页
     */
    public function index(AdminMenuService $service)
    {
        $content = hook_one('admin_index_index_view');

        if (!empty($content)) {
            return $content;
        }

        $adminMenuModel = new AdminMenuModel();
        $menus          = cache('admin_menus_' . cmf_get_current_admin_id(), '', null, 'admin_menus');

        if (empty($menus)) {
            $menus = $adminMenuModel->menuTree();
            cache('admin_menus_' . cmf_get_current_admin_id(), $menus, null, 'admin_menus');
        }

        $this->assign("menus", $menus);
        $adminMenuTree = $this->normalizeMenuTree($menus);


        $result   = $service->getAll();
        $menusTmp = array();
        foreach ($result as $item) {
            //去掉/ _ 全部小写。作为索引。
            $indexTmp            = $item['app'] . $item['controller'] . $item['action'];
            $indexTmp            = preg_replace("/[\\/|_]/", "", $indexTmp);
            $indexTmp            = strtolower($indexTmp);
            $menusTmp[$indexTmp] = $item;
        }
        $menusJsVar = json_encode($menusTmp, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES);

        $admin = Db::name("user")->where('id', cmf_get_current_admin_id())->find();
        $this->assign([
            'admin'            => $admin,
            'adminMenuTree'    => $adminMenuTree,
            'adminMenuTreeJson'=> json_encode($adminMenuTree, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES),
            'menus_js_var'     => $menusJsVar,
        ]);
        return $this->fetch();
    }

    /**
     * 将 ThinkCMF 菜单树整理成模板和前端都可以直接消费的数据。
     */
    protected function normalizeMenuTree($menus): array
    {
        if (empty($menus) || !is_array($menus)) {
            return [];
        }

        $result = [];
        foreach ($menus as $menu) {
            if (!is_array($menu)) {
                $menu = $menu->toArray();
            }

            $result[] = [
                'id'       => (string)($menu['id'] ?? ''),
                'title'    => $this->menuTitle($menu),
                'url'      => (string)($menu['url'] ?? ''),
                'icon'     => empty($menu['icon']) ? 'desktop' : (string)$menu['icon'],
                'children' => $this->normalizeMenuTree($menu['items'] ?? []),
            ];
        }

        return $result;
    }

    protected function menuTitle(array $menu): string
    {
        $lang = (string)($menu['lang'] ?? '');
        $name = (string)($menu['name'] ?? '');

        if ($lang !== '') {
            $langName = lang($lang);
            if ($langName !== $lang) {
                return $langName;
            }
        }

        return $name;
    }
}
