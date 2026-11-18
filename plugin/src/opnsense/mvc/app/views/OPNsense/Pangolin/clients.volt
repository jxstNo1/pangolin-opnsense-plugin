<script>
$(document).ready(function() {
    mapDataToFormUI({'frmClient': '/api/pangolin/settings/get'}).done(function() {
        $('.selectpicker').selectpicker('refresh');
    });

    function refreshClient() {
        ajaxGet('/api/pangolin/status/client', {}, function(st) {
            $('#client-status').text(st.connected ? 'connected' : (st.running ? 'running, not connected' : 'stopped'));
            $('#client-tunnel').text(st.tunnelIp || '-');
            $('#client-error').text(st.lastError || '-');
        });
    }
    refreshClient();
    setInterval(refreshClient, 5000);

    $('#saveClient').click(function() {
        saveFormToEndpoint('/api/pangolin/settings/set', 'frmClient', function() {
            ajaxCall('/api/pangolin/service/reconfigure', {}, function() {
                ajaxCall('/api/pangolin/service/restart/client', {}, function() {
                    setTimeout(refreshClient, 1500);
                });
            });
        }, true, function() {});
    });

    $('#testClient').click(function() {
        ajaxCall('/api/pangolin/service/test/client', {}, function(data) {
            $('#client-test').text(data.output || data.message || JSON.stringify(data));
        });
    });
});
</script>

<div class="content-box">
    <div class="page-header">
        <h1>Machine Client</h1>
    </div>
    <div class="alert alert-info">
        The machine client connects this firewall to Pangolin and is the identity used by the native subnet router. It is independent of the site connector.
    </div>
    <div class="panel panel-default">
        <div class="panel-heading"><h3 class="panel-title">Status</h3></div>
        <div class="panel-body">
            <table class="table table-condensed">
                <tr><th>State</th><td id="client-status">-</td></tr>
                <tr><th>Tunnel IP</th><td id="client-tunnel">-</td></tr>
                <tr><th>Last error</th><td id="client-error">-</td></tr>
                <tr><th>Endpoint test</th><td id="client-test">-</td></tr>
            </table>
            <button class="btn btn-default btn-xs" id="testClient">Test endpoint</button>
        </div>
    </div>
    {{ partial("layout_partials/base_form",['fields':clientForm,'id':'frmClient']) }}
    <div class="alert content-box" style="display:flex; align-items:center; margin-bottom:0;">
        <button class="btn btn-primary" id="saveClient" type="button">Save and restart client</button>
        <div style="margin-left:1em;">Saving writes validated configuration, reconciles DNS/firewall state and restarts the machine client.</div>
    </div>
</div>
