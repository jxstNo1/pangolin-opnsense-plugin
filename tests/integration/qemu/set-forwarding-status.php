<?php
/* Point the simulated agent status at the fixture DNS server and refresh it. */
$path = '/var/db/pangolin/client-status.json';
$data = json_decode((string)@file_get_contents($path), true);
if (!is_array($data)) {
    fwrite(STDERR, "status file missing or invalid\n");
    exit(1);
}
$data['dnsProxyIp'] = '127.0.0.2';
$data['updatedAt'] = gmdate('Y-m-d\TH:i:s\Z');
file_put_contents($path, json_encode($data, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . "\n", LOCK_EX);
@chmod($path, 0644);
echo "dnsProxyIp=127.0.0.2 updatedAt={$data['updatedAt']}\n";
