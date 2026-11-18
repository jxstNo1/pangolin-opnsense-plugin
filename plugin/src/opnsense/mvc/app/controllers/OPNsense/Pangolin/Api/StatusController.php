<?php

namespace OPNsense\Pangolin\Api;

use OPNsense\Base\ApiControllerBase;
use OPNsense\Pangolin\Pangolin;

class StatusController extends ApiControllerBase
{
    public function overviewAction()
    {
        if (!$this->request->isGet()) {
            return [];
        }
        $records = Pangolin::dnsRecords();
        return [
            'client' => Pangolin::readAgentStatus('client'),
            'site' => Pangolin::readAgentStatus('site'),
            'dns' => [
                'records' => count($records),
                'conflicts' => Pangolin::unboundConflicts($records) ?? [],
                'include_exists' => is_file('/usr/local/etc/unbound.opnsense.d/os-pangolin.conf'),
            ],
        ];
    }

    public function clientAction()
    {
        return Pangolin::readAgentStatus('client');
    }

    public function siteAction()
    {
        return Pangolin::readAgentStatus('site');
    }

    public function dnsAction()
    {
        $records = Pangolin::dnsRecords();
        return [
            'records' => $records,
            'conflicts' => Pangolin::unboundConflicts($records) ?? [],
            'include_exists' => is_file('/usr/local/etc/unbound.opnsense.d/os-pangolin.conf'),
        ];
    }

    public function logsAction(string $service = 'client')
    {
        if (!$this->request->isGet()) {
            return [];
        }
        return ['lines' => Pangolin::tailLog($service, 300)];
    }
}
