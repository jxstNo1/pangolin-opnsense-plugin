<?php

/*
 * Copyright (C) 2026 os-pangolin contributors
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are met:
 *
 * 1. Redistributions of source code must retain the above copyright notice,
 *    this list of conditions and the following disclaimer.
 *
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED ``AS IS'' AND ANY EXPRESS OR IMPLIED WARRANTIES,
 * INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY
 * AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE
 * AUTHOR BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY,
 * OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
 * SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
 * INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
 * CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
 * ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
 * POSSIBILITY OF SUCH DAMAGE.
 */

use OPNsense\Pangolin\Pangolin;

function pangolin_enabled()
{
    try {
        return (new Pangolin())->isEnabled();
    } catch (\Throwable $e) {
        return false;
    }
}

function pangolin_devices()
{
    return [
        [
            'pattern' => '^pangolin[0-9]+$',
            'configurable' => false,
            'spoofmac' => false,
            'volatile' => true,
        ],
        [
            'pattern' => '^newt[0-9]+$',
            'configurable' => false,
            'spoofmac' => false,
            'volatile' => true,
        ],
    ];
}

function pangolin_services()
{
    $services = [];
    try {
        $mdl = new Pangolin();
    } catch (\Throwable $e) {
        return $services;
    }

    if ($mdl->clientEnabled()) {
        $services[] = [
            'description' => gettext('Pangolin Machine Client'),
            'configd' => [
                'restart' => ['pangolin_client restart'],
                'start' => ['pangolin_client start'],
                'stop' => ['pangolin_client stop'],
            ],
            'name' => 'pangolin_client',
            'pidfile' => '/var/run/pangolin_client.pid',
        ];
    }
    if ($mdl->siteEnabled()) {
        $services[] = [
            'description' => gettext('Pangolin Site Connector'),
            'configd' => [
                'restart' => ['pangolin_site restart'],
                'start' => ['pangolin_site start'],
                'stop' => ['pangolin_site stop'],
            ],
            'name' => 'pangolin_site',
            'pidfile' => '/var/run/pangolin_site.pid',
        ];
    }
    return $services;
}

function pangolin_configure()
{
    return [
        'bootup' => ['pangolin_configure_do'],
        'newwanip' => ['pangolin_configure_do'],
        'pangolin_sync' => ['pangolin_configure_do'],
        'pangolin_dns_sync' => ['pangolin_dns_sync_do'],
        'pangolin_firewall_sync' => ['pangolin_firewall_sync_do'],
        'pangolin_uninstall' => ['pangolin_uninstall_do'],
    ];
}

function pangolin_uninstall_do()
{
    try {
        $mdl = new Pangolin();
        pangolin_sync_unbound_dots($mdl, [], false);
    } catch (\Throwable $e) {
        syslog(LOG_WARNING, 'pangolin: uninstall DNS cleanup failed: ' . $e->getMessage());
    }
    @unlink('/usr/local/etc/unbound.opnsense.d/os-pangolin.conf');
    pangolin_flush_firewall_anchors();
    pangolin_restart_unbound();
    foreach (['client', 'site'] as $mode) {
        mwexecfb('/usr/local/etc/rc.d/pangolin_%s stop', [$mode]);
    }
    @unlink(Pangolin::AGENT_CONFIG_DIR . '/client.json');
    @unlink(Pangolin::AGENT_CONFIG_DIR . '/site.json');
    @unlink(Pangolin::STATE_DIR . '/unbound-dots.json');
    syslog(LOG_NOTICE, 'pangolin: runtime state cleaned before package removal');
}

function pangolin_configure_do($verbose = false)
{
    service_log('Synchronising Pangolin configuration...', $verbose);

    if (!is_dir(Pangolin::AGENT_CONFIG_DIR)) {
        @mkdir(Pangolin::AGENT_CONFIG_DIR, 0700, true);
    }
    @chmod(Pangolin::AGENT_CONFIG_DIR, 0700);
    foreach ([Pangolin::STATE_DIR, '/var/log/pangolin', '/var/etc/pangolin'] as $dir) {
        if (!is_dir($dir)) {
            @mkdir($dir, 0755, true);
        }
    }

    try {
        $mdl = new Pangolin();
    } catch (\Throwable $e) {
        service_log("failed to load model: {$e->getMessage()}\n", $verbose);
        return;
    }

    try {
        $mdl->syncAgentConfigs();
    } catch (\Throwable $e) {
        service_log("failed to write agent configuration: {$e->getMessage()}\n", $verbose);
        syslog(LOG_ERR, 'pangolin: failed to write agent configuration: ' . $e->getMessage());
    }

    try {
        pangolin_dns_sync_do($verbose);
    } catch (\Throwable $e) {
        syslog(LOG_ERR, 'pangolin: DNS sync failed: ' . $e->getMessage());
    }
    try {
        pangolin_firewall_sync_do($verbose);
    } catch (\Throwable $e) {
        syslog(LOG_ERR, 'pangolin: firewall sync failed: ' . $e->getMessage());
    }

    syslog(
        LOG_DEBUG,
        sprintf(
            'pangolin: service loop client=%d site=%d',
            $mdl->clientEnabled() ? 1 : 0,
            $mdl->siteEnabled() ? 1 : 0
        )
    );
    foreach (['client' => $mdl->clientEnabled(), 'site' => $mdl->siteEnabled()] as $mode => $enabled) {
        if ($enabled) {
            if (!pangolin_service_running($mode)) {
                service_log("starting pangolin_{$mode}...\n", $verbose);
                mwexecfb('/usr/local/etc/rc.d/pangolin_%s start', [$mode]);
                syslog(LOG_NOTICE, "pangolin: start {$mode} requested");
            }
        } else {
            if (pangolin_service_running($mode)) {
                service_log("stopping disabled pangolin_{$mode}...\n", $verbose);
                mwexecfb('/usr/local/etc/rc.d/pangolin_%s stop', [$mode]);
                syslog(LOG_NOTICE, "pangolin: stop {$mode} requested");
            }
        }
    }

    service_log("done.\n", $verbose);
}

function pangolin_dns_sync_do($verbose = false)
{
    try {
        $mdl = new Pangolin();
    } catch (\Throwable $e) {
        return;
    }
    $include = '/usr/local/etc/unbound.opnsense.d/os-pangolin.conf';
    $changed = false;

    $status = Pangolin::readAgentStatus('client');
    $records = Pangolin::dnsRecords();
    $active = $mdl->dnsEnabled() && !empty($status['connected']);

    $lines = [
        '# Managed by os-pangolin. Do not edit; changes are overwritten.',
        '# Alias records are learned from the running Pangolin machine client.',
    ];
    $seen = [];
    if ($active) {
        $conflicts = Pangolin::unboundConflicts($records);
        if ($conflicts === null) {
            syslog(LOG_ERR, 'pangolin: cannot inspect manual Unbound entries; refusing to publish aliases');
            return;
        }
        $blocked = [];
        foreach ($conflicts as $conflict) {
            $blocked[strtok($conflict, ' ')] = true;
        }
        foreach ($records as $record) {
            $domain = strtolower(trim((string)($record['domain'] ?? ''), '.'));
            $ip = trim((string)($record['ip'] ?? ''));
            $type = strtoupper(trim((string)($record['type'] ?? 'A')));
            if ($domain === '' || !in_array($type, ['A', 'AAAA'], true)) {
                continue;
            }
            if (isset($blocked[$domain])) {
                syslog(LOG_WARNING, "pangolin: DNS alias {$domain} skipped: manual Unbound entry exists");
                continue;
            }
            if (!filter_var($ip, FILTER_VALIDATE_IP)) {
                syslog(LOG_WARNING, "pangolin: DNS alias {$domain} skipped: invalid address {$ip}");
                continue;
            }
            if ($type === 'A' && !filter_var($ip, FILTER_VALIDATE_IP, FILTER_FLAG_IPV4)) {
                continue;
            }
            if ($type === 'AAAA' && !filter_var($ip, FILTER_VALIDATE_IP, FILTER_FLAG_IPV6)) {
                continue;
            }
            $isWildcard = str_starts_with($domain, '*.');
            if ($isWildcard) {
                $zone = substr($domain, 2);
                if (!pangolin_valid_domain($zone)) {
                    continue;
                }
                $key = 'zone:' . $zone;
                if (!isset($seen[$key])) {
                    $lines[] = 'local-zone: "' . $zone . '." redirect';
                    $seen[$key] = true;
                }
                $lines[] = 'local-data: "' . $zone . '. ' . (int)(string)$mdl->dns->ttl . ' IN ' . $type . ' ' . $ip . '"';
            } else {
                if (!pangolin_valid_domain($domain)) {
                    continue;
                }
                $key = 'exact:' . $domain . ':' . $type . ':' . $ip;
                if (!isset($seen[$key])) {
                    $lines[] = 'local-data: "' . $domain . '. ' . (int)(string)$mdl->dns->ttl . ' IN ' . $type . ' ' . $ip . '"';
                    $seen[$key] = true;
                }
            }
        }
    }

    $content = implode("\n", $lines) . "\n";
    if ($active && count($records) > 0) {
        if (is_file($include)) {
            if (file_get_contents($include) !== $content) {
                pangolin_write_atomic($include, $content, 0644);
                $changed = true;
            }
        } else {
            pangolin_write_atomic($include, $content, 0644);
            $changed = true;
        }
    } elseif (is_file($include)) {
        /* Tunnel down or feature disabled: remove only our include so stale
         * aliases cannot point at unreachable addresses. */
        @unlink($include);
        $changed = true;
    }

    /* Conditional forwarding is managed through the official Unbound
     * domain-override model. Only UUIDs recorded in our own state file are
     * ever touched; manually managed entries are never modified. */
    $domainChanged = pangolin_sync_unbound_dots($mdl, $status, $active);
    $changed = $changed || $domainChanged;

    if ($changed) {
        syslog(LOG_NOTICE, 'pangolin: Unbound alias configuration changed, restarting Unbound');
        pangolin_restart_unbound();
    }
    service_log("Unbound DNS sync " . ($changed ? "updated" : "unchanged") . ".\n", $verbose);
}

function pangolin_restart_unbound()
{
    /* Never invoke pluginctl/configctl or unbound_service_stop() from inside a
     * configd hook: both make nested configd requests and deadlock while the
     * current action is still running. Kill PID 1:1 and let start.sh copy the
     * include and start Unbound. */
    $pid = (int)trim((string)@file_get_contents('/var/run/unbound.pid'));
    syslog(LOG_NOTICE, "pangolin: restarting Unbound (pid={$pid})");
    if ($pid > 1) {
        mwexecf('/bin/kill -TERM %s', [$pid]);
        for ($i = 0; $i < 25; $i++) {
            if (!isvalidpid('/var/run/unbound.pid')) {
                break;
            }
            usleep(200000);
        }
    }
    mwexecfb('/usr/local/opnsense/scripts/unbound/start.sh');
    syslog(LOG_DEBUG, 'pangolin: Unbound start script invoked');
}

function pangolin_sync_unbound_dots(Pangolin $mdl, array $status, bool $active): bool
{
    $stateFile = Pangolin::STATE_DIR . '/unbound-dots.json';
    $stored = [];
    if (is_file($stateFile)) {
        $decoded = json_decode((string)file_get_contents($stateFile), true);
        if (is_array($decoded)) {
            $stored = $decoded;
        }
    }

    $desired = [];
    $proxyIP = trim((string)($status['dnsProxyIp'] ?? ''));
    if (
        $active &&
        (string)$mdl->dns->conditionalForwarding === '1' &&
        filter_var($proxyIP, FILTER_VALIDATE_IP, FILTER_FLAG_IPV4)
    ) {
        foreach ($mdl->csvList((string)$mdl->dns->forwardDomains) as $domain) {
            $domain = strtolower(trim($domain, '.'));
            if (pangolin_valid_domain($domain)) {
                $desired[$domain] = true;
            }
        }
    }

    try {
        $unbound = new \OPNsense\Unbound\Unbound();
    } catch (\Throwable $e) {
        return false;
    }

    /* Never add a forward zone that overlaps a manually managed one. */
    $foreignDots = [];
    foreach ($unbound->dots->dot->iterateItems() as $dot) {
        if (str_starts_with((string)$dot->description, 'os-pangolin')) {
            continue;
        }
        $foreign = strtolower(trim((string)$dot->domain, '.'));
        if ($foreign !== '') {
            $foreignDots[$foreign] = true;
        }
    }
    foreach (array_keys($desired) as $domain) {
        foreach (array_keys($foreignDots) as $foreign) {
            if ($domain === $foreign || str_ends_with($domain, '.' . $foreign)) {
                syslog(LOG_WARNING, "pangolin: conditional forward {$domain} skipped: manual Unbound domain override {$foreign} exists");
                unset($desired[$domain]);
                break;
            }
        }
    }

    $changed = false;
    $newStored = [];
    foreach ($stored as $uuid => $domain) {
        $node = $unbound->getNodeByReference('dots.dot.' . $uuid);
        if ($node === null || !isset($desired[$domain])) {
            if ($node !== null) {
                $unbound->dots->dot->del($uuid);
                $changed = true;
            }
            continue;
        }
        $node->enabled = '1';
        $node->type = 'forward';
        if ((string)$node->server !== $proxyIP) {
            $node->server = $proxyIP;
            $changed = true;
        }
        if ((string)$node->port !== '53') {
            $node->port = '53';
            $changed = true;
        }
        $node->forward_first = '0';
        $node->description = 'os-pangolin conditional forward';
        $newStored[$uuid] = $domain;
    }
    foreach (array_keys($desired) as $domain) {
        if (in_array($domain, $newStored, true)) {
            continue;
        }
        $node = $unbound->dots->dot->Add();
        $node->enabled = '1';
        $node->type = 'forward';
        $node->domain = $domain;
        $node->server = $proxyIP;
        $node->port = '53';
        $node->forward_first = '0';
        $node->description = 'os-pangolin conditional forward';
        $newStored[$node->getAttribute('uuid')] = $domain;
        $changed = true;
    }

    if ($changed) {
        $config = \OPNsense\Core\Config::getInstance();
        $config->lock();
        try {
            $unbound->serializeToConfig();
        } finally {
            $config->unlock();
        }
    }

    $oldJson = is_file($stateFile) ? (string)file_get_contents($stateFile) : '';
    $newJson = json_encode($newStored, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES);
    if ($oldJson !== $newJson) {
        pangolin_write_atomic($stateFile, ($newJson === false ? '{}' : $newJson) . "\n", 0600);
    }
    return $changed;
}

function pangolin_firewall_sync_do($verbose = false)
{
    try {
        $mdl = new Pangolin();
    } catch (\Throwable $e) {
        return;
    }
    if (!$mdl->subnetRouterEnabled() || (string)$mdl->subnetRouter->backend !== 'opnsense') {
        syslog(LOG_DEBUG, 'pangolin: firewall sync skipped (subnet router disabled or backend is not opnsense)');
        pangolin_flush_firewall_anchors();
        return;
    }

    $status = Pangolin::readAgentStatus('client');
    syslog(
        LOG_DEBUG,
        sprintf(
            'pangolin: firewall sync state connected=%s tunnel=%s iface=%s',
            $status['connected'] ? 'yes' : 'no',
            $status['tunnelIp'] ?? '',
            $status['interface'] ?? ''
        )
    );
    if (empty($status['connected']) || empty($status['tunnelIp'])) {
        pangolin_flush_firewall_anchors();
        service_log("Subnet router inactive; anchors flushed.\n", $verbose);
        return;
    }
    $tunnelIP = (string)$status['tunnelIp'];
    $iface = (string)($status['interface'] ?? '');
    if (!filter_var($tunnelIP, FILTER_VALIDATE_IP, FILTER_FLAG_IPV4) || !preg_match('/^[A-Za-z0-9_]{1,15}$/', $iface)) {
        syslog(LOG_ERR, 'pangolin: subnet router has no valid tunnel address/interface');
        pangolin_flush_firewall_anchors();
        return;
    }

    $local = [];
    foreach ($mdl->csvList((string)$mdl->subnetRouter->localNetworks) as $cidr) {
        if (pangolin_valid_ipv4_cidr($cidr, 8)) {
            $local[] = $cidr;
        }
    }
    if (empty($local)) {
        syslog(LOG_ERR, 'pangolin: no valid local networks for subnet router');
        pangolin_flush_firewall_anchors();
        return;
    }
    $remote = [];
    foreach ((array)($status['routes'] ?? []) as $cidr) {
        $cidr = trim((string)$cidr);
        if (!pangolin_valid_ipv4_cidr($cidr, 8)) {
            continue;
        }
        /* Never install a default-route-equivalent. Full-tunnel/gateway mode
         * is the machine client's job, not the subnet router's. */
        if (in_array($cidr, ['0.0.0.0/0', '0.0.0.0/1', '128.0.0.0/1'], true)) {
            continue;
        }
        if (pangolin_cidr_overlaps_any($cidr, $local)) {
            syslog(LOG_WARNING, "pangolin: remote route {$cidr} overlaps a local network; skipped");
            continue;
        }
        $remote[] = $cidr;
    }
    $remote = array_values(array_unique($remote));
    if (empty($remote)) {
        syslog(LOG_NOTICE, 'pangolin: tunnel has no remote IPv4 routes; subnet router filter set is empty');
    }
    $allowedRemote = [];
    foreach ($mdl->csvList((string)$mdl->subnetRouter->remoteNetworks) as $cidr) {
        if (pangolin_valid_ipv4_cidr($cidr, 8)) {
            $allowedRemote[] = $cidr;
        }
    }
    if (!empty($allowedRemote)) {
        $filtered = [];
        foreach ($remote as $cidr) {
            foreach ($allowedRemote as $allowed) {
                if (pangolin_cidr_within($cidr, $allowed)) {
                    $filtered[] = $cidr;
                    break;
                }
            }
        }
        $remote = array_values(array_unique($filtered));
        syslog(LOG_DEBUG, sprintf('pangolin: remote allow-list active, %d route(s) remain', count($remote)));
    }
    syslog(
        LOG_DEBUG,
        sprintf(
            'pangolin: firewall sync locals=[%s] remotes=[%s]',
            implode(',', $local),
            implode(',', $remote)
        )
    );

    $nat = "# managed by os-pangolin\n";
    if ((string)$mdl->subnetRouter->nat === '1' && !empty($remote)) {
        $nat .= 'nat on ' . $iface . ' inet from { ' . implode(', ', $local) . ' } to { '
            . implode(', ', $remote) . ' } -> ' . $tunnelIP . "\n";
    }
    $fw = "# managed by os-pangolin\n";
    if ((string)$mdl->subnetRouter->firewall === '1' && !empty($remote)) {
        $fw .= 'pass in quick inet from { ' . implode(', ', $local) . ' } to { '
            . implode(', ', $remote) . ' } keep state' . "\n";
        $fw .= 'pass out quick on ' . $iface . ' inet from { ' . implode(', ', $local) . ' } to { '
            . implode(', ', $remote) . ' } keep state' . "\n";
    }

    $dir = '/var/etc/pangolin';
    if (!is_dir($dir)) {
        @mkdir($dir, 0755, true);
    }
    $natFile = $dir . '/pf_nat.conf';
    $fwFile = $dir . '/pf_fw.conf';
    $changed = pangolin_write_if_changed($natFile, $nat, 0600);
    $changed = pangolin_write_if_changed($fwFile, $fw, 0600) || $changed;

    /* Load only into our own anchor. The global ruleset is never replaced. */
    if ($changed || !pangolin_pf_anchor_loaded()) {
        mwexecf('/sbin/pfctl -a %s -f %s', ['os-pangolin/nat', $natFile]);
        mwexecf('/sbin/pfctl -a %s -f %s', ['os-pangolin/fw', $fwFile]);
        syslog(LOG_NOTICE, "pangolin: subnet router rules applied for {$iface} ({$tunnelIP})");
    }
    service_log("Subnet router rules " . ($changed ? "updated" : "unchanged") . ".\n", $verbose);
}

function pangolin_flush_firewall_anchors()
{
    mwexecf('/sbin/pfctl -a %s -F rules', ['os-pangolin/fw']);
    mwexecf('/sbin/pfctl -a %s -F nat', ['os-pangolin/nat']);
    @unlink('/var/etc/pangolin/pf_nat.conf');
    @unlink('/var/etc/pangolin/pf_fw.conf');
}

function pangolin_pf_anchor_loaded()
{
    $out = [];
    exec('/sbin/pfctl -a os-pangolin/fw -sr 2>/dev/null', $out);
    return count($out) > 0;
}

function pangolin_firewall($fw)
{
    if (!pangolin_enabled()) {
        return;
    }
    $fw->registerAnchor('os-pangolin/nat', 'nat');
    $fw->registerAnchor('os-pangolin/fw', 'fw');
}

function pangolin_cron()
{
    $jobs = [];
    try {
        $mdl = new Pangolin();
    } catch (\Throwable $e) {
        return $jobs;
    }
    if (!$mdl->isEnabled()) {
        return $jobs;
    }
    $seconds = max(60, $mdl->dnsIntervalSeconds());
    $minutes = (int)round($seconds / 60);
    if ($minutes >= 60) {
        $schedule = '0';
    } else {
        $schedule = '*/' . max(1, $minutes);
    }
    $jobs[]['autocron'] = ['/usr/local/sbin/configctl pangolin sync', $schedule];
    return $jobs;
}

function pangolin_xmlrpc_sync()
{
    return [[
        'description' => gettext('Pangolin'),
        'section' => 'OPNsense.pangolin',
        'services' => ['pangolin_client', 'pangolin_site'],
    ]];
}

function pangolin_service_running(string $mode): bool
{
    $pidfile = '/var/run/pangolin_' . $mode . '.pid';
    if (!is_file($pidfile)) {
        return false;
    }
    $pid = (int)trim((string)@file_get_contents($pidfile));
    if ($pid <= 1) {
        return false;
    }
    $command = trim((string)shell_safe('/bin/ps -p %s -o command=', [$pid]));
    if (strpos($command, 'pangolin-agent') === false && strpos($command, 'daemon:') !== 0) {
        return false;
    }
    return isvalidpid($pidfile);
}

function pangolin_valid_domain(string $domain): bool
{
    if ($domain === '' || strlen($domain) > 253) {
        return false;
    }
    return (bool)preg_match('/^(?=.{1,253}$)(?:[A-Za-z0-9_](?:[A-Za-z0-9_\-]{0,61}[A-Za-z0-9_])?\.)+[A-Za-z0-9_](?:[A-Za-z0-9_\-]{0,61}[A-Za-z0-9_])?$/', $domain);
}

function pangolin_valid_ipv4_cidr(string $cidr, int $minPrefix = 0): bool
{
    if (!preg_match('/^(\d{1,3}(?:\.\d{1,3}){3})\/(\d{1,2})$/', $cidr, $m)) {
        return false;
    }
    $ip = ip2long($m[1]);
    $prefix = (int)$m[2];
    if ($ip === false || $prefix < $minPrefix || $prefix > 32) {
        return false;
    }
    if ($prefix === 0) {
        return false;
    }
    $first = ($ip >> 24) & 0xff;
    $second = ($ip >> 16) & 0xff;
    if ($first === 0 || $first === 127 || $first >= 224) {
        return false;
    }
    if ($first === 169 && $second === 254) {
        return false;
    }
    return true;
}

function pangolin_cidr_overlaps_any(string $cidr, array $others): bool
{
    $a = pangolin_cidr_range($cidr);
    if ($a === null) {
        return true;
    }
    foreach ($others as $other) {
        $b = pangolin_cidr_range($other);
        if ($b !== null && $a[0] <= $b[1] && $b[0] <= $a[1]) {
            return true;
        }
    }
    return false;
}

function pangolin_cidr_range(string $cidr): ?array
{
    if (!preg_match('/^(\d{1,3}(?:\.\d{1,3}){3})\/(\d{1,2})$/', $cidr, $m)) {
        return null;
    }
    $ip = ip2long($m[1]);
    $prefix = (int)$m[2];
    if ($ip === false || $prefix < 0 || $prefix > 32) {
        return null;
    }
    $mask = $prefix === 0 ? 0 : (0xffffffff << (32 - $prefix)) & 0xffffffff;
    $start = $ip & $mask;
    return [$start, $start | (~$mask & 0xffffffff)];
}

function pangolin_cidr_within(string $cidr, string $parent): bool
{
    $child = pangolin_cidr_range($cidr);
    $outer = pangolin_cidr_range($parent);
    if ($child === null || $outer === null) {
        return false;
    }
    return $child[0] >= $outer[0] && $child[1] <= $outer[1];
}

function pangolin_write_atomic(string $path, string $content, int $mode): bool
{
    $dir = dirname($path);
    if (!is_dir($dir)) {
        @mkdir($dir, 0755, true);
    }
    $tmp = $path . '.tmp.' . getmypid();
    if (file_put_contents($tmp, $content, LOCK_EX) === false) {
        @unlink($tmp);
        return false;
    }
    @chmod($tmp, $mode);
    if (!rename($tmp, $path)) {
        @unlink($tmp);
        return false;
    }
    @chmod($path, $mode);
    return true;
}

function pangolin_write_if_changed(string $path, string $content, int $mode): bool
{
    if (is_file($path) && file_get_contents($path) === $content) {
        return false;
    }
    return pangolin_write_atomic($path, $content, $mode);
}
