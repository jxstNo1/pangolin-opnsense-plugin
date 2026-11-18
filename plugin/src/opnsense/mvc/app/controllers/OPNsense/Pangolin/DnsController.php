<?php

namespace OPNsense\Pangolin;

class DnsController extends \OPNsense\Base\IndexController
{
    public function indexAction()
    {
        $this->view->dnsForm = $this->getForm('dns');
        $this->view->pick('OPNsense/Pangolin/dns');
    }
}
