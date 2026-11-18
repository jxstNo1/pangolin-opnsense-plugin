<?php

namespace OPNsense\Pangolin\Api;

use OPNsense\Base\ApiControllerBase;
use OPNsense\Core\Backend;
use OPNsense\Pangolin\Pangolin;

class ServiceController extends ApiControllerBase
{
    private function target(string $service): ?string
    {
        if ($service === 'client') {
            return 'pangolin_client';
        }
        if ($service === 'site') {
            return 'pangolin_site';
        }
        return null;
    }

    public function statusAction(string $service = 'client')
    {
        if (!$this->request->isGet()) {
            return ['status' => 'failed'];
        }
        $target = $this->target($service);
        if ($target === null) {
            return ['status' => 'failed', 'message' => 'unknown service'];
        }
        return [
            'status' => Pangolin::serviceRunning($service) ? 'running' : 'stopped',
            'service' => $target,
            'agent' => Pangolin::readAgentStatus($service),
        ];
    }

    public function startAction(string $service = 'client')
    {
        return $this->serviceCommand($service, 'start');
    }

    public function stopAction(string $service = 'client')
    {
        return $this->serviceCommand($service, 'stop');
    }

    public function restartAction(string $service = 'client')
    {
        return $this->serviceCommand($service, 'restart');
    }

    private function serviceCommand(string $service, string $command): array
    {
        if (!$this->request->isPost()) {
            return ['status' => 'failed', 'message' => 'POST required'];
        }
        $target = $this->target($service);
        if ($target === null) {
            return ['status' => 'failed', 'message' => 'unknown service'];
        }
        try {
            $backend = new Backend();
            $output = trim($backend->configdRun($target . ' ' . $command));
            return ['status' => 'ok', 'output' => $output];
        } catch (\Throwable $e) {
            return ['status' => 'failed', 'message' => $e->getMessage()];
        }
    }

    /**
     * Write agent configuration, reconcile DNS/firewall and start enabled
     * services. Called by the Apply button.
     */
    public function reconfigureAction()
    {
        if (!$this->request->isPost()) {
            return ['status' => 'failed', 'message' => 'POST required'];
        }
        try {
            $backend = new Backend();
            $output = trim($backend->configdRun('pangolin sync'));
            return ['status' => 'ok', 'output' => $output];
        } catch (\Throwable $e) {
            return ['status' => 'failed', 'message' => $e->getMessage()];
        }
    }

    public function testAction(string $service = 'client')
    {
        if (!$this->request->isPost()) {
            return ['status' => 'failed', 'message' => 'POST required'];
        }
        $target = $this->target($service);
        if ($target === null) {
            return ['status' => 'failed', 'message' => 'unknown service'];
        }
        try {
            $backend = new Backend();
            return ['status' => 'ok', 'output' => trim($backend->configdRun($target . ' test'))];
        } catch (\Throwable $e) {
            return ['status' => 'failed', 'message' => $e->getMessage()];
        }
    }
}
