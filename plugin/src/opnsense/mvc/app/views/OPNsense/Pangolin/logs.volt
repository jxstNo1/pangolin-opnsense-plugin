<script>
function pangolinLogsRefresh() {
    var service = $('#log-service').val();
    ajaxGet('/api/pangolin/status/logs/' + service, {}, function(data) {
        var lines = data.lines || [];
        $('#log-output').text(lines.length ? lines.join('\n') : 'No log lines available.');
        $('#log-output').scrollTop($('#log-output')[0].scrollHeight);
    });
}
$(document).ready(function() {
    $('#log-service').change(pangolinLogsRefresh);
    $('#log-refresh').click(pangolinLogsRefresh);
    pangolinLogsRefresh();
    setInterval(pangolinLogsRefresh, 10000);
});
</script>

<div class="content-box">
    <div class="page-header">
        <h1>Logs</h1>
    </div>
    <div class="row">
        <div class="col-sm-4">
            <select class="form-control" id="log-service">
                <option value="client">Machine client</option>
                <option value="site">Site connector</option>
            </select>
        </div>
        <div class="col-sm-8">
            <button class="btn btn-default" id="log-refresh" type="button">Refresh</button>
            <span class="text-muted" style="margin-left:1em;">Last 300 lines, refreshed every 10 seconds.</span>
        </div>
    </div>
    <pre id="log-output" style="max-height:65vh; overflow:auto; margin-top:1em; white-space:pre-wrap;"></pre>
</div>
