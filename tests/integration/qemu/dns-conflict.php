<?php
/* Regression checks for manual Unbound parent-zone and wildcard conflicts. */
require_once 'config.inc';
require_once 'plugins.inc.d/pangolin.inc';

use OPNsense\Pangolin\Pangolin;

$cases = [
    ['db.corp.example.com', [], ['corp.example.com'], true],
    ['db.corp.example.com', [], ['other.example.com'], false],
    ['db.corp.example.com', ['db.corp.example.com'], [], true],
    ['db.corp.example.com', ['x.corp.example.com'], [], false],
    ['*.corp.example.com', ['db.corp.example.com'], [], true],
    ['*.corp.example.com', [], ['sub.corp.example.com'], true],
    ['db.corp.example.com', [], ['sub.corp.example.com'], false],
];

$failed = 0;
foreach ($cases as [$domain, $exact, $zones, $want]) {
    $got = Pangolin::domainConflicts($domain, $exact, $zones);
    if ($got !== $want) {
        $failed++;
        printf(
            "FAIL domain=%s exact=[%s] zones=[%s] want=%s got=%s\n",
            $domain,
            implode(',', $exact),
            implode(',', $zones),
            $want ? 'true' : 'false',
            $got ? 'true' : 'false'
        );
    }
}
if ($failed > 0) {
    exit(1);
}
echo "domain conflict cases passed: " . count($cases) . "\n";
