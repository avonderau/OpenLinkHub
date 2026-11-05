# Live system metrics on Deck LCD keys

The Deck Editor can display the same Linux telemetry sources already used by
the Nexus touchscreen, using the shared Nexus metric registry and sensor readers.

For any key:

1. Select a **Live metric**, e.g. CPU temperature, GPU utilization, memory,
   fan RPM, network transfer or disk utilization.
2. Choose the **Refresh** interval (2, 5, 10, 15, 30 or 60 seconds).
3. Choose a **Visualization**: Value only, Line graph, Bar graph,
   Fuel gauge, or Ring gauge. Line and bar graphs use up to 24 recent
   samples, with gaps for missing readings. Fuel and ring gauges use an
   absolute 0–100% range for utilization metrics, 0–100 C for temperatures,
   and a relative 120% of the recent maximum for other sensor readings.
   The numeric reading remains visible with each visualization.
4. Optionally upload artwork or set a custom display label. Existing
   button actions still run if assigned to a metric key.
5. Save & Apply. The first sampling interval updates the LCD; the editor
   receives the current values through the existing Deck status API.

Metrics not present on a host display `--` rather than invented readings.
Available metric values are finite, validated against the same Nexus registry,
and copied before being included in the JSON status API.

## Performance and safety

- The Mirabox metrics task runs a one-second scheduler but samples only when
  at least one active key is due for its own configured interval.
- Sensor IO occurs *outside* the HID device mutex. The display lock is held
  only while applying the measured values to the correct active page.
- Only updated metric keys are uploaded, followed by one LCD flush.
  Static icons, navigation and action keys are not continuously retransmitted.
- Graph buffers retain at most 24 samples per metric. Previous layouts with
  `showGraph: true` continue to render a Line graph without migration. No untrusted URLs,
  filesystem paths or commands are accepted as metric names.
- CPU/GPU/temp/memory/hwmon/sysfs and optional NVIDIA values are provided by
  the same code as the existing Nexus display. Linux network/disk rates use
  an independent Nexus counter sampler per Deck.
- On unplug/replug, the saved key assignment reappears and the refresh
  scheduler resumes when the Mirabox reconnects.
- Hardware models without an implemented LCD driver remain discovery-only.

## Manual test

Configure CPU temperature and memory utilization keys with 2-second updates.
Optionally choose GPU temperature and a fan metric. Test each visualization,
confirm the physical numbers refresh and that unplug/replug restores the
layout. Confirm a static custom icon and a page-navigation or media button
continue to work. A missing sensor should display `--`.

Automated tests: `go test ./src/devices/deck ./src/devices/nexus -count=1`
and `node --test static/js/deck-editor.test.cjs`.
