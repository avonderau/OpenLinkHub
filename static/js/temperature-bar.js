$(document).ready(function () {
    // Keep existing API/DOM IDs, but render temperature units separately so the
    // value follows the compact Nexus telemetry visual hierarchy.
    function updateTemperature(id, raw) {
        const target = document.getElementById(id);
        if (!target) return;
        const text = raw == null ? "—" : String(raw).trim();
        const match = /^(-?\d+(?:[.,]\d+)?)\s*(°[CF])$/i.exec(text);
        if (!match) {
            target.textContent = text;
            return;
        }
        const number = document.createElement("span");
        number.className = "olh-telemetry-number";
        number.textContent = match[1];
        const unit = document.createElement("span");
        unit.className = "olh-telemetry-unit";
        unit.textContent = match[2];
        target.replaceChildren(number, unit);
        target.setAttribute("aria-label", match[1] + " " + match[2]);
    }

    // Style the server-rendered values immediately, before the first API poll.
    document.querySelectorAll(".olh-telemetry-temperature").forEach(function (element) {
        updateTemperature(element.id, element.textContent);
    });

    function getTemperatures() {
        $.ajax({
            url:'/api/cpuTemp',
            type:'get',
            success:function(result){
                updateTemperature("cpu_temp", result.data);
            }
        });
        $.ajax({
            url:'/api/gpuTemps',
            type:'get',
            success:function(result){
                $.each(result.data, function(index, value) {
                    updateTemperature("gpu_temp_" + index, value);
                });
            }
        });

        $.ajax({
            url:'/api/storageTemp',
            type:'get',
            success:function(result){
                $.each(result.data, function(index, value) {
                    updateTemperature("storage_temp-" + value.Key, value.TemperatureString);
                });
            }
        });
    }

    function autoRefresh() {
        getTemperatures();
        setInterval(function(){
            getTemperatures();
        },3000);
    }
    autoRefresh();
});
