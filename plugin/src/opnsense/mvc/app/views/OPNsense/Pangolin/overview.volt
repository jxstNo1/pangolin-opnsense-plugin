<script>
function pangolinLabel(ok, yes, no) {
    return ok
        ? '<span class="label label-success">' + yes + '</span>'
        : '<span class="label label-default">' + no + '</span>';
}
function pangolinEscape(value) {
    return $('<div>').text(value === undefined || value === null ? '' : value).html();
}
function pangolinRefresh() {
    ajaxGet('/api/pangolin/status/overview', {}, function(data) {
        var c = data.client || {};
        var s = data.site || {};
        $('#client-state').html(pangolinLabel(c.connected, 'Connected', c.running ? 'Running' : 'Stopped'));
        $('#client-tunnel').text(c.tunnelIp || '-');
        $('#client-iface').text(c.interface || '-');
        $('#client-error').text(c.lastError || '-');
        $('#client-ha').text(c.haNote || (c.haActive ? 'active' : 'inactive'));
        $('#site-state').html(pangolinLabel(s.connected, 'Connected', s.running ? 'Running' : 'Stopped'));
        $('#site-iface').text(s.interface || '-');
        $('#site-error').text(s.lastError || '-');
        $('#site-ha').text(s.haNote || (s.haActive ? 'active' : 'inactive'));
        var d = data.dns || {};
        $('#dns-count').text(d.records || 0);
        $('#dns-include').html(pangolinLabel(d.include_exists, 'present', 'absent'));
        $('#dns-conflicts').text((d.conflicts && d.conflicts.length) ? d.conflicts.join('; ') : 'none');
    });
}
function pangolinService(action, service, done) {
    ajaxCall('/api/pangolin/service/' + action + '/' + service, {}, function() {
        if (done) { setTimeout(done, 1200); }
        setTimeout(pangolinRefresh, 1200);
    });
}
$(document).ready(function() {
    pangolinRefresh();
    setInterval(pangolinRefresh, 5000);
    $('#client-start').click(function() { pangolinService('start', 'client'); });
    $('#client-stop').click(function() { pangolinService('stop', 'client'); });
    $('#client-restart').click(function() { pangolinService('restart', 'client'); });
    $('#site-start').click(function() { pangolinService('start', 'site'); });
    $('#site-stop').click(function() { pangolinService('stop', 'site'); });
    $('#site-restart').click(function() { pangolinService('restart', 'site'); });
});
</script>

<div class="content-box">
    <div class="page-header">
        <h1>Pangolin <small>native FreeBSD integration</small></h1>
    </div>
    <div class="row">
        <div class="col-sm-6">
            <div class="panel panel-default">
                <div class="panel-heading">
                    <h3 class="panel-title">Machine Client <span id="client-state"></span></h3>
                </div>
                <div class="panel-body">
                    <table class="table table-condensed">
                        <tr><th>Tunnel IP</th><td id="client-tunnel">-</td></tr>
                        <tr><th>Interface</th><td id="client-iface">-</td></tr>
                        <tr><th>HA gate</th><td id="client-ha">-</td></tr>
                        <tr><th>Last error</th><td id="client-error">-</td></tr>
                    </table>
                    <button class="btn btn-success btn-xs" id="client-start">Start</button>
                    <button class="btn btn-warning btn-xs" id="client-restart">Restart</button>
                    <button class="btn btn-danger btn-xs" id="client-stop">Stop</button>
                </div>
            </div>
        </div>
        <div class="col-sm-6">
            <div class="panel panel-default">
                <div class="panel-heading">
                    <h3 class="panel-title">Site Connector <span id="site-state"></span></h3>
                </div>
                <div class="panel-body">
                    <table class="table table-condensed">
                        <tr><th>Interface</th><td id="site-iface">-</td></tr>
                        <tr><th>HA gate</th><td id="site-ha">-</td></tr>
                        <tr><th>Last error</th><td id="site-error">-</td></tr>
                    </table>
                    <button class="btn btn-success btn-xs" id="site-start">Start</button>
                    <button class="btn btn-warning btn-xs" id="site-restart">Restart</button>
                    <button class="btn btn-danger btn-xs" id="site-stop">Stop</button>
                </div>
            </div>
        </div>
    </div>
    <div class="panel panel-default">
        <div class="panel-heading"><h3 class="panel-title">DNS / Unbound</h3></div>
        <div class="panel-body">
            <table class="table table-condensed">
                <tr><th>Published aliases</th><td id="dns-count">0</td></tr>
                <tr><th>Managed include</th><td id="dns-include">-</td></tr>
                <tr><th>Conflicts</th><td id="dns-conflicts">none</td></tr>
            </table>
            <a class="btn btn-default btn-xs" href="/ui/pangolin/dns">Open DNS settings</a>
        </div>
    </div>
    <div class="alert alert-info">
        Public HTTPS resources are never rewritten. DNS and firewall cleanup is driven by the agent state file and a periodic reconciliation job.
    </div>
</div>
