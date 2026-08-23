import { createApp, defineComponent, ref, reactive, computed, watch, onMounted } from "vue";
import { api, DURATIONS, resolveRange } from "./lib/api.js";
import { fmtBytes, fmtNum, fmtMs, fmtTime, statusClass, toLocalInput } from "./lib/format.js";
import StatCard from "./components/StatCard.js";
import MiniTable from "./components/MiniTable.js";
import ChartCanvas from "./components/ChartCanvas.js";
import MultiSelect from "./components/MultiSelect.js";

// Match Chart.js to the dark theme once, globally.
const Chart = window.Chart;
Chart.defaults.color = "#8b91a1";
Chart.defaults.font.size = 11;
Chart.defaults.borderColor = "#1e2130";

const STATUS_COLORS = { "2": "#4caf50", "3": "#42a5f5", "4": "#ff7043", "5": "#ef5350" };

const App = defineComponent({
  name: "App",
  components: { StatCard, MiniTable, ChartCanvas, MultiSelect },
  setup() {
    // ---- Range / filter state ----
    const durationMs = ref(DURATIONS["24h"]); // null when a custom range is active
    const customFrom = ref("");
    const customTo = ref("");
    // Selected filter values (multi). Empty array = no filter (all).
    const selHosts = ref([]);
    const selStatuses = ref([]);
    const selClients = ref([]);

    // ---- Filter option lists (from /api/filters) ----
    const hostOptions = ref([]);
    const statusOptions = ref([]);
    const clientOptions = ref([]);

    // ---- Data state ----
    const healthy = ref(true);
    const loading = ref(false);
    const stats = reactive({
      total_requests: 0,
      data_sent_bytes: 0,
      avg_request_time_s: 0,
      avg_upstream_response_time_s: 0,
      by_status_code: {},
      top_hosts: [],
      top_ips: [],
    });
    const series = ref([]);
    const allLogs = ref([]);
    // stub_status series; stays empty when the collector runs without STUB_STATUS_URL.
    const stubSeries = ref([]);

    // ---- Log table controls ----
    const uriFilter = ref("");
    const limit = ref("500");

    // Mobile-only: collapse the filter bar behind a toggle (no effect on desktop).
    const filtersOpen = ref(false);

    const presets = ["1h", "6h", "24h", "2d", "7d"];

    function currentRange() {
      return resolveRange({
        durationMs: durationMs.value,
        customFrom: customFrom.value,
        customTo: customTo.value,
      });
    }

    const filterArgs = computed(() => ({
      host: selHosts.value,
      status: selStatuses.value,
      client_ip: selClients.value,
    }));

    // Status options: range buckets first, then the exact codes present in the data.
    const STATUS_RANGES = ["2xx", "3xx", "4xx", "5xx"];

    // ---- Loaders ----
    async function loadFilters() {
      try {
        const f = await api.filters();
        hostOptions.value = f.hosts || [];
        statusOptions.value = [...STATUS_RANGES, ...(f.statuses || [])];
        clientOptions.value = f.clients || [];
      } catch (e) {
        toast("Failed to load filters: " + e.message);
      }
    }

    async function loadStats(range) {
      const s = await api.stats(range, filterArgs.value);
      Object.assign(stats, {
        total_requests: s.total_requests || 0,
        data_sent_bytes: s.data_sent_bytes || 0,
        avg_request_time_s: s.avg_request_time_s || 0,
        avg_upstream_response_time_s: s.avg_upstream_response_time_s || 0,
        by_status_code: s.by_status_code || {},
        top_hosts: s.top_hosts || [],
        top_ips: s.top_ips || [],
      });
    }

    async function loadTimeSeries(range) {
      series.value = await api.timeseries(range, filterArgs.value);
    }

    // Swallows its own errors: stub_status is optional, so a missing or failing
    // endpoint hides the two panels rather than blanking the whole dashboard.
    async function loadStub(range) {
      try {
        stubSeries.value = await api.stub(range);
      } catch (e) {
        stubSeries.value = [];
      }
    }

    async function loadLogs() {
      const range = currentRange();
      allLogs.value = await api.logs(range, filterArgs.value, limit.value);
    }

    async function refreshAll() {
      loading.value = true;
      const range = currentRange();
      // Reflect the resolved range into the custom inputs for visibility.
      if (durationMs.value !== null) {
        customFrom.value = toLocalInput(new Date(range.from));
        customTo.value = toLocalInput(new Date(range.to));
      }
      try {
        const [, , , , h] = await Promise.all([
          loadStats(range),
          loadTimeSeries(range),
          loadLogs(),
          loadStub(range),
          api.health(),
        ]);
        healthy.value = h;
      } catch (e) {
        toast("Fetch failed: " + e.message);
      } finally {
        loading.value = false;
      }
    }

    // ---- Range / filter handlers ----
    function setPreset(key) {
      durationMs.value = DURATIONS[key];
      refreshAll();
    }
    function onCustomChange() {
      durationMs.value = null; // switch off presets, use explicit range
      refreshAll();
    }
    function isActive(key) {
      return durationMs.value === DURATIONS[key];
    }

    watch([selHosts, selStatuses, selClients], refreshAll, { deep: true });
    watch(limit, loadLogs);

    // ---- Derived view data ----
    const dataSent = computed(() => fmtBytes(stats.data_sent_bytes));

    const sparks = computed(() => ({
      requests: series.value.map((p) => p.requests),
      bytes: series.value.map((p) => p.bytes_sent),
      reqtime: series.value.map((p) => p.avg_request_time_s * 1000),
      uptime: series.value.map((p) => p.avg_upstream_response_time_s * 1000),
    }));

    // ---- stub_status (proxy-wide; unaffected by the host/status/client filters) ----
    const stubEnabled = computed(() => stubSeries.value.length > 0);

    const activeConns = computed(() => {
      const s = stubSeries.value;
      return s.length ? Math.round(s[s.length - 1].active) : 0;
    });

    const avgRps = computed(() => {
      const s = stubSeries.value;
      if (!s.length) return "0.0";
      const total = s.reduce((a, p) => a + (p.requests_per_sec || 0), 0);
      return (total / s.length).toFixed(1);
    });

    const stubSparks = computed(() => ({
      active: stubSeries.value.map((p) => p.active),
      rps: stubSeries.value.map((p) => p.requests_per_sec),
    }));

    const connChartData = computed(() => {
      const labels = stubSeries.value.map((p) => fmtTime(p.time));
      const area = (label, key, color) => ({
        label,
        data: stubSeries.value.map((p) => p[key]),
        borderColor: color,
        backgroundColor: color + "44",
        fill: true,
        tension: 0.3,
        pointRadius: 0,
        borderWidth: 1.5,
      });
      return {
        labels,
        datasets: [
          area("Waiting", "waiting", "#5b6172"),
          area("Reading", "reading", "#4fc3f7"),
          area("Writing", "writing", "#ffb300"),
        ],
      };
    });

    const connChartOptions = {
      responsive: true,
      maintainAspectRatio: false,
      plugins: { legend: { position: "bottom", labels: { boxWidth: 12, padding: 12 } } },
      scales: {
        x: { grid: { display: false }, ticks: { maxTicksLimit: 8, maxRotation: 0 } },
        y: { stacked: true, grid: { color: "#1e2130" }, beginAtZero: true },
      },
      interaction: { intersect: false, mode: "index" },
    };

    const tsChartData = computed(() => ({
      labels: series.value.map((p) => fmtTime(p.time)),
      datasets: [
        {
          label: "Requests",
          data: series.value.map((p) => p.requests),
          borderColor: "#4fc3f7",
          backgroundColor: "rgba(79,195,247,0.12)",
          fill: true,
          tension: 0.3,
          pointRadius: 0,
          borderWidth: 2,
        },
      ],
    }));

    const tsChartOptions = {
      responsive: true,
      maintainAspectRatio: false,
      plugins: { legend: { display: false } },
      scales: {
        x: { grid: { display: false }, ticks: { maxTicksLimit: 8, maxRotation: 0 } },
        y: { grid: { color: "#1e2130" }, beginAtZero: true },
      },
      interaction: { intersect: false, mode: "index" },
    };

    const statusChartData = computed(() => {
      const entries = Object.entries(stats.by_status_code).sort((a, b) => b[1] - a[1]);
      return {
        labels: entries.map((e) => e[0]),
        datasets: [
          {
            data: entries.map((e) => e[1]),
            backgroundColor: entries.map((e) => STATUS_COLORS[e[0][0]] || "#5b6172"),
            borderColor: "#161922",
            borderWidth: 2,
          },
        ],
      };
    });

    const statusChartOptions = {
      responsive: true,
      maintainAspectRatio: false,
      cutout: "62%",
      plugins: { legend: { position: "right", labels: { boxWidth: 12, padding: 12 } } },
    };

    // ---- Log table (client-side URI filter on fetched rows) ----
    const filteredLogs = computed(() => {
      const q = uriFilter.value.toLowerCase();
      if (!q) return allLogs.value;
      return allLogs.value.filter((l) => (l.request_uri || "").toLowerCase().includes(q));
    });

    // Two adjacent rows are "the same log" if everything but the timestamp matches.
    function sig(l) {
      return [l.http_host, l.request_method, l.request_uri, l.status, l.remote_addr].join("");
    }

    // Collapse consecutive identical rows into one with a repeat count (×N),
    // keeping the most recent timestamp of the run. Rows are already time-sorted desc.
    const displayLogs = computed(() => {
      const out = [];
      for (const l of filteredLogs.value) {
        const prev = out[out.length - 1];
        if (prev && sig(prev) === sig(l)) {
          prev.count++;
        } else {
          out.push({ ...l, count: 1 });
        }
      }
      return out;
    });

    // ---- Toast ----
    const toastMsg = ref("");
    const toastShown = ref(false);
    let toastTimer = null;
    function toast(msg) {
      toastMsg.value = msg;
      toastShown.value = true;
      clearTimeout(toastTimer);
      toastTimer = setTimeout(() => (toastShown.value = false), 4000);
    }

    onMounted(async () => {
      await loadFilters();
      await refreshAll();
    });

    return {
      durationMs, customFrom, customTo,
      selHosts, selStatuses, selClients, hostOptions, statusOptions, clientOptions,
      healthy, loading, stats, allLogs, uriFilter, limit, presets, filtersOpen,
      setPreset, onCustomChange, isActive, refreshAll, loadLogs,
      dataSent, sparks, tsChartData, tsChartOptions,
      stubEnabled, activeConns, avgRps, stubSparks, connChartData, connChartOptions,
      statusChartData, statusChartOptions, filteredLogs, displayLogs,
      toastMsg, toastShown,
      fmtNum, fmtMs, fmtTime, statusClass,
    };
  },
  template: `
  <header>
    <div class="header-inner">
      <div class="topbar">
        <div class="brand">
          <span class="dot" :class="{ down: !healthy }"></span> Nginx&nbsp;Logger
        </div>
        <button class="filters-toggle" :class="{ open: filtersOpen }"
                @click="filtersOpen = !filtersOpen" :aria-expanded="filtersOpen">
          Filters <span class="ft-caret">▾</span>
        </button>
      </div>

      <div class="filters" :class="{ open: filtersOpen }">
        <div class="presets">
          <button v-for="p in presets" :key="p"
                  :class="{ active: isActive(p) }"
                  @click="setPreset(p)">{{ p }}</button>
        </div>

        <div class="field">
          <label>From</label>
          <input type="datetime-local" v-model="customFrom" @change="onCustomChange">
        </div>
        <div class="field">
          <label>To</label>
          <input type="datetime-local" v-model="customTo" @change="onCustomChange">
        </div>

        <div class="spacer"></div>

        <MultiSelect label="Host" all-text="All hosts" :options="hostOptions" v-model="selHosts" />
        <MultiSelect label="Status" all-text="All" :options="statusOptions" v-model="selStatuses" />
        <MultiSelect label="Client IP" all-text="All clients" :options="clientOptions" v-model="selClients" />

        <button class="btn-refresh" @click="refreshAll">Refresh</button>
      </div>
    </div>
  </header>

  <main :class="{ loading }">
    <div class="row stats">
      <StatCard title="Total Requests" :value="fmtNum(stats.total_requests)"
                label="in selected range" :spark="sparks.requests" color="#4fc3f7" />
      <StatCard title="Data Sent" :value="dataSent.val" :unit="dataSent.unit"
                label="total response bytes" :spark="sparks.bytes" color="#4caf50" />
      <StatCard title="Avg Request Time" :value="fmtMs(stats.avg_request_time_s)" unit="ms"
                label="mean across requests" :spark="sparks.reqtime" color="#ffb300" />
      <StatCard title="Avg Upstream Time" :value="fmtMs(stats.avg_upstream_response_time_s)" unit="ms"
                label="mean upstream response" :spark="sparks.uptime" color="#ff7043" />
      <StatCard v-if="stubEnabled" title="Active Connections" :value="fmtNum(activeConns)"
                label="current · proxy-wide" :spark="stubSparks.active" color="#ab47bc" />
      <StatCard v-if="stubEnabled" title="Requests/sec" :value="avgRps"
                label="mean over range · proxy-wide" :spark="stubSparks.rps" color="#26c6da" />
    </div>

    <div class="row split">
      <MiniTable title="Top Hosts" :rows="stats.top_hosts" />
      <MiniTable title="Top Client IPs" :rows="stats.top_ips" />
    </div>

    <div class="row split">
      <div class="card">
        <div class="card-title">Requests Over Time</div>
        <div class="chart-wrap">
          <ChartCanvas type="line" :data="tsChartData" :options="tsChartOptions" />
        </div>
      </div>
      <div class="card">
        <div class="card-title">Status Code Distribution</div>
        <div class="chart-wrap">
          <ChartCanvas type="doughnut" :data="statusChartData" :options="statusChartOptions" />
        </div>
      </div>
    </div>

    <div v-if="stubEnabled" class="card">
      <div class="card-title">Connection States <span class="log-count">proxy-wide · not affected by filters</span></div>
      <div class="chart-wrap">
        <ChartCanvas type="line" :data="connChartData" :options="connChartOptions" />
      </div>
    </div>

    <div class="card logs-card">
      <div class="logs-header">
        <div class="card-title">Access Logs</div>
        <span class="log-count">{{ displayLogs.length }} shown · {{ filteredLogs.length }} of {{ allLogs.length }} rows</span>
        <div class="logs-controls">
          <input type="text" v-model="uriFilter" placeholder="Filter URI…" style="width:220px">
          <select v-model="limit">
            <option value="100">100 rows</option>
            <option value="500">500 rows</option>
            <option value="1000">1000 rows</option>
          </select>
        </div>
      </div>
      <div class="log-scroll">
        <table class="log-table">
          <thead><tr>
            <th>Time</th><th>Host</th><th>Method</th><th>URI</th>
            <th>Status</th><th>Client IP</th><th>Req Time</th>
          </tr></thead>
          <tbody>
            <tr v-for="(l, i) in displayLogs" :key="i">
              <td class="time-cell">
                {{ fmtTime(l.timestamp) }}
                <span v-if="l.count > 1" class="repeat" :title="l.count + ' identical requests'">×{{ l.count }}</span>
              </td>
              <td>{{ l.http_host || '—' }}</td>
              <td class="method">{{ l.request_method || '—' }}</td>
              <td class="uri-cell" :title="l.request_uri">{{ l.request_uri || '—' }}</td>
              <td><span class="chip" :class="statusClass(l.status)">{{ l.status }}</span></td>
              <td>{{ l.remote_addr || '—' }}</td>
              <td>{{ (l.request_time * 1000).toFixed(0) }}ms</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </main>

  <div class="toast" :class="{ show: toastShown }">{{ toastMsg }}</div>
  `,
});

createApp(App).mount("#app");
