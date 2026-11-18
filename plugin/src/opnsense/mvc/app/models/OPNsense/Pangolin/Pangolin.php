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

namespace OPNsense\Pangolin;

use OPNsense\Base\BaseModel;
use OPNsense\Base\UserException;

class Pangolin extends BaseModel
{
    public const AGENT_CONFIG_DIR = '/usr/local/etc/pangolin';
    public const STATE_DIR = '/var/db/pangolin';
    public const HOOK_SCRIPT = '/usr/local/etc/pangolin/hook.sh';
    /** Maximum age of a status heartbeat before the agent is considered dead. */
    public const STATUS_MAX_AGE = 45;

    public function isEnabled(): bool
    {
        return $this->masterEnabled() &&
            ((string)$this->client->enabled === '1' || (string)$this->site->enabled === '1');
    }

    private function masterEnabled(): bool
    {
        return (string)$this->general->enabled === '1';
    }

    public function clientEnabled(): bool
    {
        return $this->masterEnabled() && (string)$this->client->enabled === '1';
    }

    public function siteEnabled(): bool
    {
        return $this->masterEnabled() && (string)$this->site->enabled === '1';
    }

    public function dnsEnabled(): bool
    {
        return $this->masterEnabled() && $this->clientEnabled() && (string)$this->dns->enabled === '1';
    }

    public function dnsIntervalSeconds(): int
    {
        $map = ['m1' => 60, 'm5' => 300, 'm15' => 900, 'm30' => 1800, 'm60' => 3600];
        $value = (string)$this->dns->syncInterval;
        return $map[$value] ?? 300;
    }

    public function subnetRouterEnabled(): bool
    {
        return $this->masterEnabled() && $this->clientEnabled() && (string)$this->subnetRouter->enabled === '1';
    }

    public function haEnabled(): bool
    {
        return $this->masterEnabled() && (string)$this->ha->enabled === '1';
    }

    /**
     * Cross-field validation that cannot be expressed in the XML model.
     * Throws UserException so ApiMutableModelControllerBase aborts the save.
     */
    public function validateSecurity(): void
    {
        if (!$this->masterEnabled()) {
            return;
        }
        $errors = [];
        $allowHttp = (string)$this->advanced->allowInsecureHttp === '1';

        foreach (['client' => 'Machine client', 'site' => 'Site connector'] as $section => $label) {
            if ((string)$this->{$section}->enabled !== '1') {
                continue;
            }
            $endpoint = trim((string)$this->{$section}->endpoint);
            if ($endpoint === '') {
                $errors[] = sprintf('%s: endpoint is required.', $label);
            } else {
                $parts = parse_url($endpoint);
                if ($parts === false || empty($parts['host'])) {
                    $errors[] = sprintf('%s: endpoint is not a valid URL.', $label);
                } elseif (($parts['scheme'] ?? '') !== 'https' && !$allowHttp) {
                    $errors[] = sprintf('%s: endpoint must use https (or enable the advanced insecure-HTTP override).', $label);
                }
            }
            if (trim((string)$this->{$section}->id) === '') {
                $errors[] = sprintf('%s: ID is required.', $label);
            }
            if (trim((string)$this->{$section}->secret) === '') {
                $errors[] = sprintf('%s: secret is required.', $label);
            }
            if (strlen((string)$this->{$section}->secret) > 4096) {
                $errors[] = sprintf('%s: secret is implausibly long.', $label);
            }
        }

        if (
            $this->clientEnabled() && $this->siteEnabled() &&
            (string)$this->client->interfaceName === (string)$this->site->interfaceName
        ) {
            $errors[] = 'Client and site interfaces must have different names.';
        }

        if ((string)$this->subnetRouter->enabled === '1' && !$this->clientEnabled()) {
            $errors[] = 'Subnet routing requires the machine client to be enabled.';
        }
        if ((string)$this->dns->enabled === '1' && !$this->clientEnabled()) {
            $errors[] = 'DNS synchronisation requires the machine client to be enabled.';
        }

        if ($this->subnetRouterEnabled()) {
            $networks = $this->csvList((string)$this->subnetRouter->localNetworks);
            if (empty($networks)) {
                $errors[] = 'Subnet routing requires at least one local network.';
            }
            $ranges = [];
            foreach ($networks as $network) {
                $range = $this->parseCIDR($network);
                if ($range === null) {
                    $errors[] = sprintf('Subnet routing: "%s" is not a valid IPv4 CIDR.', $network);
                    continue;
                }
                [$start, $end, $prefix] = $range;
                if ($prefix < 8) {
                    $errors[] = sprintf('Subnet routing: %s is too broad; use /8 or narrower.', $network);
                }
                if ($start === 0 && $end === 0xffffffff) {
                    $errors[] = 'Subnet routing: the default route 0.0.0.0/0 is never allowed.';
                }
                if (!$this->isUnicastIPv4($start)) {
                    $errors[] = sprintf('Subnet routing: %s is not a unicast network.', $network);
                }
                foreach ($ranges as [$otherStart, $otherEnd, $otherNetwork]) {
                    if ($start <= $otherEnd && $otherStart <= $end) {
                        $errors[] = sprintf('Subnet routing: %s overlaps %s.', $network, $otherNetwork);
                    }
                }
                $ranges[] = [$start, $end, $network];
            }

            foreach ($this->csvList((string)$this->subnetRouter->remoteNetworks) as $network) {
                $range = $this->parseCIDR($network);
                if ($range === null) {
                    $errors[] = sprintf('Subnet routing: remote network "%s" is not a valid IPv4 CIDR.', $network);
                    continue;
                }
                [$start, $end, $prefix] = $range;
                if ($prefix < 8) {
                    $errors[] = sprintf('Subnet routing: remote network %s is too broad; use /8 or narrower.', $network);
                }
                if ($start === 0 && $end === 0xffffffff) {
                    $errors[] = 'Subnet routing: the default route 0.0.0.0/0 is never allowed.';
                }
                if (!$this->isUnicastIPv4($start)) {
                    $errors[] = sprintf('Subnet routing: remote network %s is not a unicast network.', $network);
                }
                foreach ($ranges as [$localStart, $localEnd, $localNetwork]) {
                    if ($start <= $localEnd && $localStart <= $end) {
                        $errors[] = sprintf('Subnet routing: remote network %s overlaps local network %s.', $network, $localNetwork);
                    }
                }
            }
            if ((string)$this->subnetRouter->backend === 'pf') {
                $errors[] = 'The standalone pf backend is not available in the OPNsense plugin; use the OPNsense backend.';
            }
        }

        if ($this->dnsEnabled()) {
            if ((string)$this->dns->conditionalForwarding === '1' && empty($this->csvList((string)$this->dns->forwardDomains))) {
                $errors[] = 'Conditional forwarding requires at least one private DNS suffix.';
            }
        }

        if ($this->haEnabled() && trim((string)$this->ha->interface) === '') {
            $errors[] = 'HA integration requires a CARP interface.';
        }

        if (!empty($errors)) {
            throw new UserException(implode("\n", $errors), gettext('Pangolin validation failed'));
        }
    }

    /**
     * Write the two agent JSON documents. Called from configd as root.
     * Returns the paths actually written. $changedModes receives the modes
     * whose on-disk configuration differs, so a running agent can be
     * restarted without bouncing it on every harmless sync.
     */
    public function syncAgentConfigs(?array &$changedModes = null): array
    {
        $changedModes = [];
        if (!is_dir(self::AGENT_CONFIG_DIR)) {
            mkdir(self::AGENT_CONFIG_DIR, 0700, true);
        }
        @chmod(self::AGENT_CONFIG_DIR, 0700);

        $written = [];
        if ($this->clientEnabled()) {
            $written[] = $this->writeJsonIfChanged(
                self::AGENT_CONFIG_DIR . '/client.json',
                $this->agentConfig('client'),
                $changedModes,
                'client'
            );
        } else {
            if (is_file(self::AGENT_CONFIG_DIR . '/client.json')) {
                @unlink(self::AGENT_CONFIG_DIR . '/client.json');
                $changedModes[] = 'client';
            }
        }
        if ($this->siteEnabled()) {
            $written[] = $this->writeJsonIfChanged(
                self::AGENT_CONFIG_DIR . '/site.json',
                $this->agentConfig('site'),
                $changedModes,
                'site'
            );
        } else {
            if (is_file(self::AGENT_CONFIG_DIR . '/site.json')) {
                @unlink(self::AGENT_CONFIG_DIR . '/site.json');
                $changedModes[] = 'site';
            }
        }
        return $written;
    }

    private function agentConfig(string $mode): array
    {
        $common = [
            'mode' => $mode,
            'logLevel' => (string)$this->general->logLevel,
            'stateDir' => self::STATE_DIR,
            'hookScript' => self::HOOK_SCRIPT,
            'allowInsecureHttp' => (string)$this->advanced->allowInsecureHttp === '1',
        ];

        if ($mode === 'site') {
            $common['site'] = [
                'endpoint' => trim((string)$this->site->endpoint),
                'id' => trim((string)$this->site->id),
                'secret' => (string)$this->site->secret,
                'name' => trim((string)$this->site->name),
                'mtu' => (int)(string)$this->site->mtu,
                'interfaceName' => (string)$this->site->interfaceName,
                'dns' => trim((string)$this->site->dns),
                'useNativeMainInterface' => (string)$this->site->useNativeMainInterface === '1',
                'useNativeClientInterface' => false,
                'disableSsh' => (string)$this->site->disableSsh === '1',
                'disableClients' => (string)$this->site->disableClients === '1',
                'pingInterval' => (string)$this->site->pingInterval,
                'pingTimeout' => (string)$this->site->pingTimeout,
                'logLevel' => (string)$this->general->logLevel,
            ];
            return $common;
        }

        $common['client'] = [
            'endpoint' => trim((string)$this->client->endpoint),
            'id' => trim((string)$this->client->id),
            'secret' => (string)$this->client->secret,
            'orgId' => trim((string)$this->client->orgId),
            'mtu' => (int)(string)$this->client->mtu,
            'interfaceName' => (string)$this->client->interfaceName,
            'dns' => trim((string)$this->client->dns),
            'upstreamDns' => $this->csvList((string)$this->client->upstreamDns),
            'pingInterval' => (string)$this->client->pingInterval,
            'pingTimeout' => (string)$this->client->pingTimeout,
            'logLevel' => (string)$this->general->logLevel,
            'holepunch' => (string)$this->client->holepunch === '1',
            'preferLocalRoutes' => (string)$this->client->preferLocalRoutes === '1',
            'disableRoutesAndAliasesOnExitNode' => (string)$this->client->disableRoutesAndAliasesOnExitNode === '1',
            'overrideDns' => false,
        ];
        $common['subnetRouter'] = [
            'enabled' => $this->subnetRouterEnabled(),
            'backend' => (string)$this->subnetRouter->backend,
            'localNetworks' => $this->csvList((string)$this->subnetRouter->localNetworks),
            'allowedRemoteNetworks' => $this->csvList((string)$this->subnetRouter->remoteNetworks),
            'nat' => (string)$this->subnetRouter->nat === '1',
            'firewall' => (string)$this->subnetRouter->firewall === '1',
        ];
        $common['dns'] = [
            'enabled' => $this->dnsEnabled(),
            'syncInterval' => $this->dnsIntervalSeconds() . 's',
            'outputFile' => self::STATE_DIR . '/dns-records.json',
            'forwardDomains' => $this->csvList((string)$this->dns->forwardDomains),
            'ttl' => (int)(string)$this->dns->ttl,
        ];
        $common['ha'] = [
            'enabled' => $this->haEnabled(),
            'interface' => trim((string)$this->ha->interface),
            'requiredState' => (string)$this->ha->requiredState,
        ];
        return $common;
    }

    private function writeJsonIfChanged(string $path, array $data, array &$changedModes, string $mode): string
    {
        $json = json_encode($data, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES);
        if ($json === false) {
            throw new \RuntimeException('failed to encode Pangolin agent configuration: ' . json_last_error_msg());
        }
        $json .= "\n";
        $existing = @file_get_contents($path);
        if ($existing === $json) {
            @chmod($path, 0600);
            return $path;
        }
        $tmp = $path . '.tmp.' . getmypid();
        $old = umask(0077);
        $ok = file_put_contents($tmp, $json, LOCK_EX);
        umask($old);
        if ($ok === false) {
            @unlink($tmp);
            throw new \RuntimeException('failed to write ' . $path);
        }
        @chmod($tmp, 0600);
        if (!rename($tmp, $path)) {
            @unlink($tmp);
            throw new \RuntimeException('failed to replace ' . $path);
        }
        @chmod($path, 0600);
        if (!in_array($mode, $changedModes, true)) {
            $changedModes[] = $mode;
        }
        return $path;
    }

    public function csvList(string $value): array
    {
        $out = [];
        foreach (explode(',', $value) as $item) {
            $item = trim($item);
            if ($item !== '') {
                $out[] = $item;
            }
        }
        return $out;
    }

    /**
     * Read a status document written by the agent. The file is world-readable
     * but contains no secrets. A stale or dead child process must not keep
     * firewall/NAT/DNS state alive just because the daemon(8) supervisor is
     * still present.
     */
    public static function readAgentStatus(string $mode): array
    {
        $path = self::STATE_DIR . '/' . $mode . '-status.json';
        $status = @file_get_contents($path);
        $data = $status === false ? [] : json_decode($status, true);
        if (!is_array($data)) {
            $data = [];
        }
        $data['connected'] = !empty($data['connected']);
        $data['registered'] = !empty($data['registered']);

        $agentPid = (int)($data['pid'] ?? 0);
        $updatedAt = (string)($data['updatedAt'] ?? '');
        $age = self::statusAge($updatedAt);
        $agentAlive = $agentPid > 1 && self::pidIsAgent($agentPid);
        $fresh = $age !== null && $age <= self::STATUS_MAX_AGE;
        $supervisor = self::serviceRunning($mode);

        $data['supervisorRunning'] = $supervisor;
        $data['statusAge'] = $age;
        $data['running'] = $agentAlive && $fresh;

        if (!$data['running']) {
            $data['connected'] = false;
            $data['registered'] = false;
            $data['pid'] = 0;
            if ($supervisor && !$agentAlive) {
                $data['statusError'] = 'agent process is not running';
            } elseif ($agentAlive && !$fresh) {
                $data['statusError'] = 'agent status is stale';
            } else {
                $data['statusError'] = 'agent is not running';
            }
        }
        return $data;
    }

    private static function statusAge(string $updatedAt): ?int
    {
        if ($updatedAt === '') {
            return null;
        }
        $timestamp = strtotime($updatedAt);
        if ($timestamp === false) {
            return null;
        }
        return max(0, time() - $timestamp);
    }

    private static function pidIsAgent(int $pid): bool
    {
        if ($pid <= 1) {
            return false;
        }
        $output = [];
        $rc = 0;
        @exec('/bin/ps -ww -p ' . $pid . ' -o command= 2>/dev/null', $output, $rc);
        if ($rc !== 0) {
            return false;
        }
        $command = trim(implode(' ', $output));
        /* The daemon supervisor has "daemon:" in its command and must never
         * satisfy the agent-liveness check. */
        return $command !== '' &&
            str_contains($command, 'pangolin-agent') &&
            !str_starts_with($command, 'daemon:');
    }

    public static function serviceRunning(string $mode): bool
    {
        $pidfile = '/var/run/pangolin_' . $mode . '.pid';
        if (!is_file($pidfile)) {
            return false;
        }
        $pid = (int)trim((string)@file_get_contents($pidfile));
        if ($pid <= 1) {
            return false;
        }
        $output = [];
        $rc = 0;
        @exec('/bin/ps -ww -p ' . $pid . ' -o command= 2>/dev/null', $output, $rc);
        if ($rc !== 0) {
            return false;
        }
        $command = implode(' ', $output);
        /* The pidfile belongs to daemon(8); its command line contains the
         * agent path. A recycled PID without that path is not our service. */
        return str_contains($command, 'pangolin-agent') && isvalidpid($pidfile);
    }

    public static function dnsRecords(): array
    {
        $path = self::STATE_DIR . '/dns-records.json';
        $raw = @file_get_contents($path);
        $records = $raw === false ? [] : json_decode($raw, true);
        return is_array($records) ? $records : [];
    }

    /**
     * Return a human-readable list of conflicts with manually managed Unbound
     * entries. The plugin must never override or delete those entries. A
     * manual host/alias protects its exact name; a manual domain override or
     * wildcard protects the whole zone, including subdomains.
     */
    public static function unboundConflicts(array $records): ?array
    {
        $conflicts = [];
        try {
            $unbound = new \OPNsense\Unbound\Unbound();
        } catch (\Throwable $e) {
            return null;
        }

        $manualExact = [];
        $manualZones = [];
        try {
            foreach ($unbound->hosts->host->iterateItems() as $host) {
                $fqdn = strtolower(trim((string)$host->hostname . '.' . (string)$host->domain, '.'));
                if ($fqdn !== '') {
                    $manualExact[] = $fqdn;
                }
            }
            foreach ($unbound->aliases->alias->iterateItems() as $alias) {
                $a = strtolower(trim((string)$alias->hostname . '.' . (string)$alias->domain, '.'));
                if ($a === '') {
                    continue;
                }
                if (str_starts_with($a, '*.')) {
                    $manualZones[] = substr($a, 2);
                } else {
                    $manualExact[] = $a;
                }
            }
            foreach ($unbound->dots->dot->iterateItems() as $dot) {
                if (str_starts_with((string)$dot->description, 'os-pangolin')) {
                    continue;
                }
                $d = strtolower(trim((string)$dot->domain, '.'));
                if ($d !== '') {
                    $manualZones[] = $d;
                }
            }
        } catch (\Throwable $e) {
            return null;
        }

        foreach ($records as $record) {
            $domain = strtolower(trim((string)($record['domain'] ?? ''), '.'));
            if ($domain === '') {
                continue;
            }
            if (self::domainConflicts($domain, $manualExact, $manualZones)) {
                $conflicts[] = sprintf(
                    '%s conflicts with a manually managed Unbound host, alias or zone',
                    $record['domain']
                );
            }
        }
        return array_values(array_unique($conflicts));
    }

    /**
     * Match a learned alias against manual exact names and manual zones.
     * Exposed for the DNS conflict regression fixture.
     */
    public static function domainConflicts(string $domain, array $manualExact, array $manualZones): bool
    {
        $domain = strtolower(trim($domain, '.'));
        if ($domain === '') {
            return false;
        }
        $isWildcard = str_starts_with($domain, '*.');
        $zone = $isWildcard ? substr($domain, 2) : $domain;

        foreach ($manualExact as $entry) {
            $entry = strtolower(trim((string)$entry, '.'));
            if ($entry === '') {
                continue;
            }
            if ($entry === $domain) {
                return true;
            }
            if ($isWildcard && self::domainWithin($entry, $zone)) {
                return true;
            }
        }
        foreach ($manualZones as $entry) {
            $entry = strtolower(trim((string)$entry, '.'));
            if ($entry === '') {
                continue;
            }
            if (self::domainWithin($zone, $entry) || self::domainWithin($entry, $zone)) {
                return true;
            }
        }
        return false;
    }

    private static function domainWithin(string $domain, string $zone): bool
    {
        return $domain === $zone || str_ends_with($domain, '.' . $zone);
    }

    public static function tailLog(string $service, int $lines = 200): array
    {
        if ($service !== 'client' && $service !== 'site') {
            return [];
        }
        $path = '/var/log/pangolin/' . $service . '.log';
        if (!is_file($path) || !is_readable($path)) {
            return [];
        }
        $lines = max(1, min($lines, 2000));
        $result = [];
        $fh = fopen($path, 'rb');
        if ($fh === false) {
            return [];
        }
        $buffer = '';
        $chunk = 4096;
        fseek($fh, 0, SEEK_END);
        $pos = ftell($fh);
        while ($pos > 0 && substr_count($buffer, "\n") <= $lines) {
            $read = min($chunk, $pos);
            $pos -= $read;
            fseek($fh, $pos);
            $buffer = fread($fh, $read) . $buffer;
        }
        fclose($fh);
        $all = explode("\n", rtrim($buffer, "\n"));
        return array_slice($all, -$lines);
    }

    /**
     * Parse an IPv4 CIDR into [start, end, prefix] or null.
     */
    public function parseCIDR(string $cidr): ?array
    {
        if (!preg_match('/^(\d{1,3}(?:\.\d{1,3}){3})\/(\d{1,2})$/', trim($cidr), $m)) {
            return null;
        }
        $ip = ip2long($m[1]);
        $prefix = (int)$m[2];
        if ($ip === false || $prefix < 0 || $prefix > 32) {
            return null;
        }
        $mask = $prefix === 0 ? 0 : (0xffffffff << (32 - $prefix)) & 0xffffffff;
        $start = $ip & $mask;
        $end = $start | (~$mask & 0xffffffff);
        return [$start, $end, $prefix];
    }

    private function isUnicastIPv4(int $ip): bool
    {
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
}
