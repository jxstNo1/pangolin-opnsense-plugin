<?php
/* Add or remove a manual Unbound domain override for the conflict fixture. */
require_once 'config.inc';
require_once 'util.inc';

use OPNsense\Core\Config;

$mode = $argv[1] ?? '';
$domain = 'corp.example.com';
$u = new \OPNsense\Unbound\Unbound();

if ($mode === 'add') {
    $node = $u->dots->dot->Add();
    $node->enabled = '1';
    $node->domain = $domain;
    $node->server = '1.1.1.1';
    $node->description = 'manual regression fixture';
    $u->serializeToConfig();
    Config::getInstance()->save(['description' => 'manual Unbound zone fixture']);
    echo "manual zone added: $domain\n";
    exit(0);
}

if ($mode === 'remove') {
    $removed = 0;
    foreach ($u->dots->dot->iterateItems() as $uuid => $node) {
        if ((string)$node->description === 'manual regression fixture') {
            $u->dots->dot->del($uuid);
            $removed++;
        }
    }
    $u->serializeToConfig();
    Config::getInstance()->save(['description' => 'manual Unbound zone fixture removed']);
    echo "manual zones removed: $removed\n";
    exit(0);
}

fwrite(STDERR, "usage: manual-zone.php add|remove\n");
exit(2);
