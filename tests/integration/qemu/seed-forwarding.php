<?php
/* Seed/remove forwarding entries for the conditional-forwarding fixture:
 * - old.internal: existing plugin-owned entry, must be removed by the plugin
 * - manual.internal: manual entry, must never be touched
 */
require_once 'config.inc';
require_once 'util.inc';

use OPNsense\Core\Config;
use OPNsense\Unbound\Unbound;

$mode = $argv[1] ?? '';
$u = new Unbound();
$fixtureDomains = ['old.internal', 'manual.internal'];
$fixtureDescriptions = ['os-pangolin conditional forward', 'manual forwarding fixture'];

if ($mode === 'add' || $mode === 'remove') {
    foreach ($u->dots->dot->iterateItems() as $uuid => $node) {
        $domain = strtolower(trim((string)$node->domain, '.'));
        $description = (string)$node->description;
        if (in_array($domain, $fixtureDomains, true) || in_array($description, $fixtureDescriptions, true)) {
            $u->dots->dot->del($uuid);
        }
    }
}

if ($mode === 'add') {
    $plugin = $u->dots->dot->Add();
    $plugin->enabled = '1';
    $plugin->type = 'forward';
    $plugin->domain = 'old.internal';
    $plugin->server = '127.0.0.2';
    $plugin->port = '53';
    $plugin->forward_first = '0';
    $plugin->description = 'os-pangolin conditional forward';
    $pluginUuid = (string)$plugin->getAttribute('uuid');

    $manual = $u->dots->dot->Add();
    $manual->enabled = '1';
    $manual->type = 'forward';
    $manual->domain = 'manual.internal';
    $manual->server = '127.0.0.2';
    $manual->port = '53';
    $manual->forward_first = '0';
    $manual->description = 'manual forwarding fixture';

    $u->serializeToConfig();
    Config::getInstance()->save(['description' => 'seed forwarding fixture']);

    if (!is_dir('/var/db/pangolin')) {
        @mkdir('/var/db/pangolin', 0755, true);
    }
    $state = json_encode([$pluginUuid => 'old.internal'], JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . "\n";
    file_put_contents('/var/db/pangolin/unbound-dots.json', $state, LOCK_EX);
    @chmod('/var/db/pangolin/unbound-dots.json', 0600);
    echo "seeded old.internal ({$pluginUuid}) and manual.internal\n";
    exit(0);
}

if ($mode === 'remove') {
    $u->serializeToConfig();
    Config::getInstance()->save(['description' => 'remove forwarding fixture']);
    @unlink('/var/db/pangolin/unbound-dots.json');
    echo "forwarding fixture removed\n";
    exit(0);
}

fwrite(STDERR, "usage: seed-forwarding.php add|remove\n");
exit(2);
