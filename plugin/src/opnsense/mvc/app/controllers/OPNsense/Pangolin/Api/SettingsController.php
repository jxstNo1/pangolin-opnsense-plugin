<?php

namespace OPNsense\Pangolin\Api;

use OPNsense\Base\ApiMutableModelControllerBase;

class SettingsController extends ApiMutableModelControllerBase
{
    protected static $internalModelName = 'pangolin';
    protected static $internalModelClass = '\OPNsense\Pangolin\Pangolin';

    protected function setActionHook()
    {
        $this->getModel()->validateSecurity();
    }
}
