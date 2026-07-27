<?php

namespace app\common\controller;

use cmf\controller\HomeBaseController as CmfHomeBaseController;

class HomeBaseController extends CmfHomeBaseController
{
    protected $configpub = [];
    protected $configpri = [];
    protected $language = 'zh-cn';
    protected $language_type = 'zh-cn';

    protected function initialize()
    {
        parent::initialize();
        $this->initializeLegacyViewGlobals();
        $this->initializeH5Language();
        $this->initializeCountryList();
        $this->initializeLevelList();
    }

    protected function initializeLegacyViewGlobals()
    {
        $this->configpub = function_exists('getConfigPub') ? getConfigPub() : [];
        $this->configpri = function_exists('getConfigPri') ? getConfigPri() : [];

        $siteInfo = function_exists('cmf_get_site_info') ? cmf_get_site_info() : [];
        if (!is_array($siteInfo)) {
            $siteInfo = [];
        }

        $configpub = array_merge($siteInfo, is_array($this->configpub) ? $this->configpub : []);
        $siteName = (string)($configpub['site_name'] ?? $siteInfo['site_name'] ?? '');

        $this->assign('configpub', $configpub);
        $this->assign('configpri', is_array($this->configpri) ? $this->configpri : []);
        $this->assign('site_name', $siteName);
        $this->assign('site_seo_title', (string)($configpub['site_seo_title'] ?? $siteName));
        $this->assign('site_seo_keywords', (string)($configpub['site_seo_keywords'] ?? ''));
        $this->assign('site_seo_description', (string)($configpub['site_seo_description'] ?? ''));
        $user = session('user') ?: null;
        $this->assign('current', '');
        $this->assign('user', $user);
        $this->assign('userinfo', $user ? json_encode($user, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES) : 'null');
    }

    protected function initializeH5Language()
    {
        $language = (string)$this->request->param('language', $this->request->param('lang', ''));
        $language = strtolower(trim($language));

        if ($language === '') {
            $language = strtolower(cmf_current_lang() ?: 'zh-cn');
        }

        $language = strpos($language, 'en') === 0 ? 'en' : 'zh-cn';

        $this->language      = $language;
        $this->language_type = $language;

        $langFile = $this->app->getAppPath() . 'appapi' . DIRECTORY_SEPARATOR . 'lang' . DIRECTORY_SEPARATOR . $language . DIRECTORY_SEPARATOR . 'common.php';
        $langMap  = [];
        if (is_file($langFile)) {
            $langMap = $this->app->lang->load($langFile);
        }

        $this->assign('language', $language);
        $this->assign('language_type', $language);
        $this->assign('language_json', json_encode($language, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES));
        $this->assign('lang_json', json_encode($langMap ?: new \stdClass(), JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES));
    }

    protected function initializeCountryList()
    {
        $countries = getcaches('h5_country_list');
        if (!$countries) {
            $countries = [];
            $countryFile = CMF_ROOT . 'data/config/country.json';
            if (is_file($countryFile)) {
                $countryData = json_decode(file_get_contents($countryFile), true);
                foreach (($countryData['country'] ?? []) as $group) {
                    foreach (($group['lists'] ?? []) as $country) {
                        $countries[] = $country;
                    }
                }
            }

            if (!$countries) {
                $countries = [
                    ['name' => '中国', 'name_en' => 'China', 'tel' => '86'],
                ];
            }

            setcaches('h5_country_list', $countries);
        }

        $this->assign('countrys', $countries);
        $this->assign('country_list', json_encode($countries, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES));
    }

    protected function initializeLevelList()
    {
        $levelList = [];
        if (function_exists('getLevelList')) {
            foreach (getLevelList() as $level) {
                $levelList[$level['levelid']] = $level;
            }
        }

        $levelAnchorList = [];
        if (function_exists('getLevelAnchorList')) {
            foreach (getLevelAnchorList() as $level) {
                $levelAnchorList[$level['levelid']] = $level;
            }
        }

        $this->assign('levellist', $levelList);
        $this->assign('levellistj', json_encode($levelList, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES));
        $this->assign('levelanchorlist', $levelAnchorList);
        $this->assign('levelanchorlistj', json_encode($levelAnchorList, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES));
    }
}
