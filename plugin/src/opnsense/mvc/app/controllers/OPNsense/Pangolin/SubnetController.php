<?php

namespace OPNsense\Pangolin;

class SubnetController extends \OPNsense\Base\IndexController
{
    public function indexAction()
    {
        $this->view->subnetForm = $this->getForm('subnet');
        $this->view->pick('OPNsense/Pangolin/subnet');
    }
}
