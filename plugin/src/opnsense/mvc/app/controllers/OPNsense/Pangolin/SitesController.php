<?php

namespace OPNsense\Pangolin;

class SitesController extends \OPNsense\Base\IndexController
{
    public function indexAction()
    {
        $this->view->siteForm = $this->getForm('site');
        $this->view->pick('OPNsense/Pangolin/sites');
    }
}
