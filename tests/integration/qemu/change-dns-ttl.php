<?php
/* Change the DNS TTL so the generated client config differs, without
 * touching any credential. Used by sync-restart.sh. */
require_once 'config.inc';
require_once 'util.inc';

use OPNsense\Pangolin\Pangolin;

$mdl = new Pangolin();
$current = (int)(string)$mdl->dns->ttl;
$mdl->dns->ttl = (string)($current === 60 ? 61 : 60);
$mdl->serializeToConfig();
\OPNsense\Core\Config::getInstance()->save(['description' => 'pangolin DNS TTL fixture']);
echo 'dns.ttl=' . (string)$mdl->dns->ttl . "\n";
