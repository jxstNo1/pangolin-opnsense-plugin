<script>
function pangolinDnsRefresh() {
    ajaxGet('/api/pangolin/status/dns', {}, function(data) {
        var body = $('#dns-records').empty();
        var records = data.records || [];
        if (!records.length) {
            body.append('<tr><td colspan="4" class="text-muted">No aliases reported by the machine client.</td></tr>');
        }
        records.forEach(function(r) {
            body.append(
                '<tr><td>' + $('<div>').text(r.domain).html() + '</td>' +
                '<td>' + $('<div>').text(r.type).html() + '</td>' +
                '<td>' + $('<div>').text(r.ip).html() + '</td>' +
                '<td>' + $('<div>').text(r.siteId === undefined ? '' : r.siteId).html() + '</td></tr>'
            );
        });
        var conflicts = data.conflicts || [];
        $('#dns-conflicts').text(conflicts.length ? conflicts.join('; ') : 'none');
        $('#dns-include').text(data.include_exists ? 'present' : 'absent');
    });
}
$(document).ready(function() {
    mapDataToFormUI({'frmDns': '/api/pangolin/settings/get'}).done(function() {
        $('.selectpicker').selectpicker('refresh');
    });
    pangolinDnsRefresh();
    setInterval(pangolinDnsRefresh, 10000);

    $('#saveDns').click(function() {
        saveFormToEndpoint('/api/pangolin/settings/set', 'frmDns', function() {
            ajaxCall('/api/pangolin/service/reconfigure', {}, function() {
                ajaxCall('/api/pangolin/dns/sync', {}, function(data) {
                    $('#dns-result').text(data.output || data.message || '');
                    pangolinDnsRefresh();
                });
            });
        }, true, function() {});
    });

    $('#syncDns').click(function() {
        ajaxCall('/api/pangolin/dns/sync', {}, function(data) {
            $('#dns-result').text(data.output || data.message || '');
            pangolinDnsRefresh();
        });
    });

    $('#testDns').click(function() {
        var domain = $('#dns-test-domain').val();
        ajaxCall('/api/pangolin/dns/test/' + encodeURIComponent(domain), {}, function(data) {
            $('#dns-result').text(data.output || data.message || JSON.stringify(data));
        });
    });
});
</script>

<div class="content-box">
    <div class="page-header">
        <h1>DNS / Unbound</h1>
    </div>
    <div class="alert alert-info">
        The plugin writes only <code>/usr/local/etc/unbound.opnsense.d/os-pangolin.conf</code>. It never edits or deletes manually managed Unbound host/domain overrides. Public domains are never rewritten; aliases are learned from the machine client and therefore refer to private tunnel-accessible addresses.
    </div>
    <div class="panel panel-default">
        <div class="panel-heading"><h3 class="panel-title">Discovered aliases</h3></div>
        <div class="panel-body">
            <p>Managed include: <strong id="dns-include">-</strong></p>
            <table class="table table-condensed table-striped">
                <thead><tr><th>Domain</th><th>Type</th><th>Address</th><th>Site</th></tr></thead>
                <tbody id="dns-records"></tbody>
            </table>
            <p>Conflicts: <span id="dns-conflicts">none</span></p>
            <button class="btn btn-default btn-xs" id="syncDns">Sync now</button>
            <div class="input-group input-group-sm" style="max-width:32em; margin-top:0.75em;">
                <input type="text" class="form-control" id="dns-test-domain" placeholder="hostname.example.com"/>
                <span class="input-group-btn"><button class="btn btn-default" id="testDns" type="button">Resolve</button></span>
            </div>
            <pre id="dns-result" style="margin-top:0.75em;"></pre>
        </div>
    </div>
    {{ partial("layout_partials/base_form",['fields':dnsForm,'id':'frmDns']) }}
    <div class="alert content-box" style="display:flex; align-items:center; margin-bottom:0;">
        <button class="btn btn-primary" id="saveDns" type="button">Save and synchronise DNS</button>
        <div style="margin-left:1em;">Unbound is restarted only when the generated include or conditional-forwarding entries actually change.</div>
    </div>
</div>
