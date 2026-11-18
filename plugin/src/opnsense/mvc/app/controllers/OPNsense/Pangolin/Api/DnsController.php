<?php

namespace OPNsense\Pangolin\Api;

use OPNsense\Base\ApiControllerBase;
use OPNsense\Core\Backend;
use OPNsense\Pangolin\Pangolin;

class DnsController extends ApiControllerBase
{
    public function recordsAction()
    {
        $records = Pangolin::dnsRecords();
        return [
            'records' => $records,
            'conflicts' => Pangolin::unboundConflicts($records),
            'include_exists' => is_file('/usr/local/etc/unbound.opnsense.d/os-pangolin.conf'),
        ];
    }

    public function syncAction()
    {
        if (!$this->request->isPost()) {
            return ['status' => 'failed', 'message' => 'POST required'];
        }
        try {
            $backend = new Backend();
            return ['status' => 'ok', 'output' => trim($backend->configdRun('pangolin dns-sync'))];
        } catch (\Throwable $e) {
            return ['status' => 'failed', 'message' => $e->getMessage()];
        }
    }

    public function testAction(string $domain = '')
    {
        if (!$this->request->isPost()) {
            return ['status' => 'failed', 'message' => 'POST required'];
        }
        $domain = trim($domain);
        if ($domain === '' || !preg_match('/^[A-Za-z0-9_.\-]{1,253}$/', $domain)) {
            return ['status' => 'failed', 'message' => 'invalid domain'];
        }
        try {
            $backend = new Backend();
            return [
                'status' => 'ok',
                'output' => trim($backend->configdpRun('pangolin dig', [$domain])),
            ];
        } catch (\Throwable $e) {
            return ['status' => 'failed', 'message' => $e->getMessage()];
        }
    }
}
