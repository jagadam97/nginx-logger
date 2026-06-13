import { defineComponent, computed } from "vue";
import { fmtNum } from "../lib/format.js";

// Top-N table with an inline proportional bar (top hosts / top IPs).
export default defineComponent({
  name: "MiniTable",
  props: {
    title: { type: String, required: true },
    rows: { type: Array, default: () => [] }, // [{ label, count }]
  },
  setup(props) {
    const max = computed(() =>
      Math.max(1, ...props.rows.map((r) => r.count))
    );
    const pct = (count) => (count / max.value) * 100;
    return { max, pct, fmtNum };
  },
  template: `
    <div class="card">
      <div class="card-title">{{ title }}</div>
      <table class="mini-table">
        <tbody>
          <tr v-if="!rows.length">
            <td class="empty-row">No data in range</td>
          </tr>
          <tr v-for="r in rows" :key="r.label">
            <td class="label-cell" :title="r.label">{{ r.label || '—' }}</td>
            <td class="bar-cell">
              <div class="bar-wrap">
                <div class="bar-track">
                  <div class="bar-fill" :style="{ width: pct(r.count) + '%' }"></div>
                </div>
                <span class="bar-count">{{ fmtNum(r.count) }}</span>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  `,
});
