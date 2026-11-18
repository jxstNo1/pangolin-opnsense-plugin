<script>
$(document).ready(function() {
    mapDataToFormUI({'frmSite': '/api/pangolin/settings/get'}).done(function() {
        $('.selectpicker').selectpicker('refresh');
    });

    function refreshSite() {
        ajaxGet('/api/pangolin/status/site', {}, function(st) {
            $('#site-status').text(st.connected ? 'connected' : (st.running ? 'running, not connected' : 'stopped'));
            $('#site-error').text(st.lastError || '-');
        });
    }
    refreshSite();
    setInterval(refreshSite, 5000);

    $('#saveSite').click(function() {
        saveFormToEndpoint('/api/pangolin/settings/set', 'frmSite', function() {
            ajaxCall('/api/pangolin/service/reconfigure', {}, function() {
                ajaxCall('/api/pangolin/service/restart/site', {}, function() {
                    setTimeout(refreshSite, 1500);
                });
            });
        }, true, function() {});
    });

    $('#testSite').click(function() {
        ajaxCall('/api/pangolin/service/test/site', {}, function(data) {
            $('#site-test').text(data.output || data.message || JSON.stringify(data));
        });
    });
});
</script>

<div class="content-box">
    <div class="page-header">
        <h1>Site Connector</h1>
    </div>
    <div class="alert alert-info">
        The site connector embeds Newt and exposes local resources through Pangolin's site/resource model. The default user-space netstack needs no TUN device. A native TUN mode is available for advanced deployments.
    </div>
    <div class="panel panel-default">
        <div class="panel-heading"><h3 class="panel-title">Status</h3></div>
        <div class="panel-body">
            <table class="table table-condensed">
                <tr><th>State</th><td id="site-status">-</td></tr>
                <tr><th>Last error</th><td id="site-error">-</td></tr>
                <tr><th>Endpoint test</th><td id="site-test">-</td></tr>
            </table>
            <button class="btn btn-default btn-xs" id="testSite">Test endpoint</button>
        </div>
    </div>
    {{ partial("layout_partials/base_form",['fields':siteForm,'id':'frmSite']) }}
    <div class="alert content-box" style="display:flex; align-items:center; margin-bottom:0;">
        <button class="btn btn-primary" id="saveSite" type="button">Save and restart site connector</button>
        <div style="margin-left:1em;">Saving writes validated configuration, reconciles DNS/firewall state and restarts the site connector.</div>
    </div>
</div>
