<?php

namespace app\common\initializer;

use think\App;
use think\initializer\Error;

class PhpCompatError extends Error
{
    public function init(App $app)
    {
        parent::init($app);

        if (PHP_VERSION_ID >= 80400) {
            error_reporting(E_ALL & ~E_DEPRECATED & ~E_USER_DEPRECATED);
        }
    }
}
