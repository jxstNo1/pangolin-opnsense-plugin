<?php
/* Enable/disable the plugin conditional-forwarding fixture. */
require_once 'config.inc';
require_once 'util.inc';

use OPNsense\Pangolin\Pangolin;

$mode = $argv[1] ?? '';
$mdl = new Pangolin();
if ($mode === 'on') {
    $mdl->dns->enabled = '1';
    $mdl->dns->conditionalForwarding = '1';
    $mdl->dns->forwardDomains = 'corp.internal,manual.internal';
} elseif ($mode === 'off') {
    $mdl->dns->conditionalForwarding = '0';
    $mdl->dns->forwardDomains = '';
} else {
    fwrite(STDERR, "usage: conditional.php on|off\n");
    exit(2);
}
$mdl->serializeToConfig();
\OPNsense\Core\Config::getInstance()->save(['description' => 'pangolin conditional forwarding fixture']);
echo "conditionalForwarding=" . (string)$mdl->dns->conditionalForwarding
    . " forwardDomains=" . (string)$mdl->dns->forwardDomains . "\n";
