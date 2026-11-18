<?php

namespace OPNsense\Pangolin;

class OverviewController extends \OPNsense\Base\IndexController
{
    public function indexAction()
    {
        $this->view->pick('OPNsense/Pangolin/overview');
    }
}
