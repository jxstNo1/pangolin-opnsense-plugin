<?php
/*
 * Configure the Pangolin model in the isolated QEMU test VM.
 *
 * This is a test fixture, not a production tool. It uses the real OPNsense
 * model/config API so the integration fixtures exercise the same paths as the
 * web UI. It contains no credentials.
 */

require_once 'config.inc';
require_once 'util.inc';
require_once 'plugins.inc.d/pangolin.inc';

use OPNsense\Pangolin\Pangolin;

$mdl = new Pangolin();
$mdl->general->enabled = '1';
$mdl->general->logLevel = 'debug';

$mdl->client->enabled = '1';
$mdl->client->endpoint = 'https://pangolin-test.example.invalid';
$mdl->client->id = 'qemu-fixture-client';
$mdl->client->secret = 'qemu-fixture-secret-not-real';
$mdl->client->interfaceName = 'pangolin0';
$mdl->client->mtu = '1280';
$mdl->client->dns = '1.1.1.1';
$mdl->client->upstreamDns = '1.1.1.1:53,8.8.8.8:53';
$mdl->client->pingInterval = '5s';
$mdl->client->pingTimeout = '5s';
$mdl->client->holepunch = '1';
$mdl->client->preferLocalRoutes = '0';

$mdl->subnetRouter->enabled = '1';
$mdl->subnetRouter->backend = 'opnsense';
$mdl->subnetRouter->localNetworks = '192.168.1.0/24';
$mdl->subnetRouter->remoteNetworks = '192.168.50.0/24';
$mdl->subnetRouter->nat = '1';
$mdl->subnetRouter->firewall = '1';

$mdl->dns->enabled = '1';
$mdl->dns->syncInterval = 'm5';
$mdl->dns->ttl = '60';
$mdl->dns->conditionalForwarding = '0';

$mdl->ha->enabled = '0';

try {
    $mdl->validateSecurity();
} catch (Throwable $e) {
    fwrite(STDERR, 'validateSecurity failed: ' . $e->getMessage() . "\n");
    exit(1);
}

$mdl->serializeToConfig();
/* Config::save is what ApiMutableModelControllerBase uses. write_config()
 * would rebuild the singleton from the legacy $config array and discard the
 * model changes. */
\OPNsense\Core\Config::getInstance()->save(['description' => 'pangolin qemu fixture']);

echo "model written: client=" . (string)$mdl->client->enabled
    . " subnet=" . (string)$mdl->subnetRouter->enabled
    . " dns=" . (string)$mdl->dns->enabled . "\n";
