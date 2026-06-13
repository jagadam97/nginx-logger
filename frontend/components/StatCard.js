import { defineComponent, computed } from "vue";
import ChartCanvas from "./ChartCanvas.js";

// A single stat card: big value + label + a sparkline of the windowed series.
export default defineComponent({
  name: "StatCard",
  components: { ChartCanvas },
  props: {
    title: { type: String, required: true },
    value: { type: [String, Number], required: true },
    unit: { type: String, default: "" },
    label: { type: String, default: "" },
    spark: { type: Array, default: () => [] }, // numeric series
    color: { type: String, default: "#4fc3f7" },
  },
  setup(props) {
    const sparkData = computed(() => ({
      labels: props.spark.map(() => ""),
      datasets: [
        {
          data: props.spark,
          borderColor: props.color,
          backgroundColor: props.color + "22",
          fill: true,
          tension: 0.4,
          pointRadius: 0,
          borderWidth: 1.5,
        },
      ],
    }));

    const sparkOptions = {
      responsive: true,
      maintainAspectRatio: false,
      // Inset the plot area a couple of px so the line/area at zero-value windows
      // isn't drawn flush against the canvas edge (which clips its lower stroke,
      // very visible on high-DPI phone screens).
      layout: { padding: { top: 2, bottom: 2 } },
      plugins: { legend: { display: false }, tooltip: { enabled: false } },
      scales: { x: { display: false }, y: { display: false, beginAtZero: true } },
    };

    return { sparkData, sparkOptions };
  },
  template: `
    <div class="card stat">
      <div class="card-title">{{ title }}</div>
      <div>
        <span class="stat-value">{{ value }}</span>
        <span class="stat-unit" v-if="unit">{{ unit }}</span>
      </div>
      <div class="stat-label">{{ label }}</div>
      <div class="stat-spark">
        <ChartCanvas type="line" :data="sparkData" :options="sparkOptions" />
      </div>
    </div>
  `,
});
