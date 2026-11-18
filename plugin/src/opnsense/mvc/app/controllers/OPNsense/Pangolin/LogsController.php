<?php

namespace OPNsense\Pangolin;

class LogsController extends \OPNsense\Base\IndexController
{
    public function indexAction()
    {
        $this->view->pick('OPNsense/Pangolin/logs');
    }
}
