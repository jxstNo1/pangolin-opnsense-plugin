<?php

namespace OPNsense\Pangolin;

class ClientsController extends \OPNsense\Base\IndexController
{
    public function indexAction()
    {
        $this->view->clientForm = $this->getForm('client');
        $this->view->pick('OPNsense/Pangolin/clients');
    }
}
