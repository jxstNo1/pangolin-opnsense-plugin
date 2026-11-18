<script>
$(document).ready(function() {
    mapDataToFormUI({'frmSubnet': '/api/pangolin/settings/get'}).done(function() {
        $('.selectpicker').selectpicker('refresh');
    });

    function refreshSubnet() {
        ajaxGet('/api/pangolin/status/client', {}, function(st) {
            var sr = st.subnetRouter || {};
            $('#sr-state').text(sr.enabled ? (sr.applied ? 'active' : 'enabled, waiting for tunnel') : 'disabled');
            $('#sr-tunnel').text(sr.tunnelIp || st.tunnelIp || '-');
            $('#sr-backend').text(sr.backend || '-');
            $('#sr-error').text(sr.lastError || '-');
            var routes = st.routes || [];
            $('#sr-routes').text(routes.length ? routes.join(', ') : '-');
        });
    }
    refreshSubnet();
    setInterval(refreshSubnet, 5000);

    $('#saveSubnet').click(function() {
        saveFormToEndpoint('/api/pangolin/settings/set', 'frmSubnet', function() {
            ajaxCall('/api/pangolin/service/reconfigure', {}, function() {
                ajaxCall('/api/pangolin/service/restart/client', {}, function() {
                    setTimeout(refreshSubnet, 1500);
                });
            });
        }, true, function() {});
    });
});
</script>

<div class="content-box">
    <div class="page-header">
        <h1>Subnet Routing</h1>
    </div>
    <div class="alert alert-warning">
        Only the selected source networks are allowed through the tunnel. The plugin never installs a default route, never opens all VLANs, never creates inter-VLAN pass rules, and never flushes the global pf ruleset. IPv6 is not routed by this feature.
    </div>
    <div class="panel panel-default">
        <div class="panel-heading"><h3 class="panel-title">Agent state</h3></div>
        <div class="panel-body">
            <table class="table table-condensed">
                <tr><th>Subnet router</th><td id="sr-state">-</td></tr>
                <tr><th>Backend</th><td id="sr-backend">-</td></tr>
                <tr><th>Tunnel address</th><td id="sr-tunnel">-</td></tr>
                <tr><th>Remote routes</th><td id="sr-routes">-</td></tr>
                <tr><th>Last error</th><td id="sr-error">-</td></tr>
            </table>
        </div>
    </div>
    {{ partial("layout_partials/base_form",['fields':subnetForm,'id':'frmSubnet']) }}
    <div class="alert content-box" style="display:flex; align-items:center; margin-bottom:0;">
        <button class="btn btn-primary" id="saveSubnet" type="button">Save and apply subnet routing</button>
        <div style="margin-left:1em;">Changes are validated, written to the agent configuration and applied to the dedicated os-pangolin pf anchors.</div>
    </div>
</div>
