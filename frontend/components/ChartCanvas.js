import { defineComponent, ref, onMounted, onBeforeUnmount, watch } from "vue";

// Generic Chart.js wrapper. Chart is loaded globally via the UMD <script> tag.
// Props: type, data, options. Rebuilds the dataset reactively when `data` changes.
export default defineComponent({
  name: "ChartCanvas",
  props: {
    type: { type: String, required: true },
    data: { type: Object, required: true },
    options: { type: Object, default: () => ({}) },
  },
  setup(props) {
    const el = ref(null);
    let chart = null;

    onMounted(() => {
      chart = new window.Chart(el.value, {
        type: props.type,
        data: props.data,
        options: props.options,
      });
    });

    // Swap in the new data object and redraw without animating from scratch.
    watch(
      () => props.data,
      (next) => {
        if (!chart) return;
        chart.data = next;
        chart.update();
      },
      { deep: true }
    );

    onBeforeUnmount(() => {
      if (chart) chart.destroy();
    });

    return { el };
  },
  template: `<canvas ref="el"></canvas>`,
});
