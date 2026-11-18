<?php
/* Diagnostic fixture: show what the installed plugin sees. */
require_once 'config.inc';
require_once 'util.inc';
require_once 'plugins.inc.d/pangolin.inc';

$mdl = new OPNsense\Pangolin\Pangolin();
var_dump([
    'master' => (string)$mdl->general->enabled,
    'client' => (string)$mdl->client->enabled,
    'subnet' => (string)$mdl->subnetRouter->enabled,
    'backend' => (string)$mdl->subnetRouter->backend,
    'dns' => (string)$mdl->dns->enabled,
    'subnetRouterEnabled' => $mdl->subnetRouterEnabled(),
    'dnsEnabled' => $mdl->dnsEnabled(),
]);
echo 'status: ' . json_encode(OPNsense\Pangolin\Pangolin::readAgentStatus('client')) . "\n";
echo "--- firewall sync ---\n";
pangolin_firewall_sync_do(true);
echo "--- dns sync ---\n";
pangolin_dns_sync_do(true);
echo 'pf fw: ' . shell_exec('/sbin/pfctl -a os-pangolin/fw -sr 2>&1') . "\n";
echo 'pf nat: ' . shell_exec('/sbin/pfctl -a os-pangolin/nat -sn 2>&1') . "\n";
