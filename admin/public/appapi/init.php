<?php


defined('API_ROOT') || define('API_ROOT', dirname(__FILE__) . '/../../phalapi');
defined('API_MODE') || define('API_MODE', 'prod');
require_once API_ROOT . '/vendor/autoload.php';
date_default_timezone_set(getenv('APP_DEFAULT_TIMEZONE') ?: 'Asia/Shanghai');
include API_ROOT . '/config/di.php';
if (\PhalApi\DI()->debug) {
    \PhalApi\DI()->tracer->mark('PHALAPI_INIT');
    error_reporting(E_ALL);
    ini_set('display_errors', 'On'); 
}
//语言包
if(isset($_REQUEST['language'])){
    \PhalApi\DI()->language=$_REQUEST['language'];
    if($_REQUEST['language']=='en'){
        \PhalApi\SL('en');
    }else{
        \PhalApi\SL('zh-cn');
    }
}else{
    \PhalApi\SL('zh-cn');
}
