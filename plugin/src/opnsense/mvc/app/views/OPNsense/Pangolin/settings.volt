<script>
$(document).ready(function() {
    mapDataToFormUI({'frmSettings': '/api/pangolin/settings/get'}).done(function() {
        $('.selectpicker').selectpicker('refresh');
    });

    function refreshSettingsStatus() {
        ajaxGet('/api/pangolin/status/overview', {}, function(data) {
            var c = data.client || {};
            var s = data.site || {};
            $('#settings-client').text(c.running ? (c.connected ? 'running, connected' : 'running') : 'stopped');
            $('#settings-site').text(s.running ? (s.connected ? 'running, connected' : 'running') : 'stopped');
            $('#settings-ha').text(c.haNote || s.haNote || '-');
        });
    }
    refreshSettingsStatus();
    setInterval(refreshSettingsStatus, 8000);

    $('#saveSettings').click(function() {
        saveFormToEndpoint('/api/pangolin/settings/set', 'frmSettings', function() {
            ajaxCall('/api/pangolin/service/reconfigure', {}, function() {
                ajaxGet('/api/pangolin/status/overview', {}, function(data) {
                    var c = data.client || {};
                    var s = data.site || {};
                    var pending = 0;
                    function finish() {
                        pending--;
                        if (pending <= 0) { setTimeout(refreshSettingsStatus, 1500); }
                    }
                    if (c.running) { pending++; ajaxCall('/api/pangolin/service/restart/client', {}, finish); }
                    if (s.running) { pending++; ajaxCall('/api/pangolin/service/restart/site', {}, finish); }
                    if (pending === 0) { refreshSettingsStatus(); }
                });
            });
        }, true, function() {});
    });
});
</script>

<div class="content-box">
    <div class="page-header">
        <h1>Settings</h1>
    </div>
    <div class="panel panel-default">
        <div class="panel-heading"><h3 class="panel-title">Agent state</h3></div>
        <div class="panel-body">
            <table class="table table-condensed">
                <tr><th>Machine client</th><td id="settings-client">-</td></tr>
                <tr><th>Site connector</th><td id="settings-site">-</td></tr>
                <tr><th>HA gate</th><td id="settings-ha">-</td></tr>
            </table>
        </div>
    </div>
    {{ partial("layout_partials/base_form",['fields':settingsForm,'id':'frmSettings']) }}
    <div class="alert content-box" style="display:flex; align-items:center; margin-bottom:0;">
        <button class="btn btn-primary" id="saveSettings" type="button">Save and apply</button>
        <div style="margin-left:1em;">Running services are restarted only when they are currently active. Stopped or disabled services are never started implicitly.</div>
    </div>
    <div class="panel panel-default">
        <div class="panel-heading"><h3 class="panel-title">About</h3></div>
        <div class="panel-body">
            <p><strong>os-pangolin</strong> is an AGPL-3.0 native FreeBSD/OPNsense integration for Pangolin. The agent embeds the official <code>fosrl/newt</code> and <code>fosrl/olm</code> runtime with a FreeBSD network backend; it does not use Linux containers, Linux VMs or Linux compatibility layers.</p>
            <p>Secrets are stored in config.xml (root-readable) and in 0600 agent JSON files. The service logs are under <code>/var/log/pangolin/</code>. See <code>/ui/pangolin/logs</code>.</p>
        </div>
    </div>
</div>
