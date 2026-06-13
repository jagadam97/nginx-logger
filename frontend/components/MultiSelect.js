import { defineComponent, ref, computed, onMounted, onBeforeUnmount } from "vue";

// Searchable multi-select dropdown (checkbox list). v-model is an array of values.
// Handles large option lists (e.g. hundreds of client IPs) via a search box.
export default defineComponent({
  name: "MultiSelect",
  props: {
    label: { type: String, required: true },
    options: { type: Array, default: () => [] },
    modelValue: { type: Array, default: () => [] },
    allText: { type: String, default: "All" },
  },
  emits: ["update:modelValue"],
  setup(props, { emit }) {
    const open = ref(false);
    const search = ref("");
    const root = ref(null);

    const filtered = computed(() => {
      const q = search.value.toLowerCase();
      if (!q) return props.options;
      return props.options.filter((o) => String(o).toLowerCase().includes(q));
    });

    const summary = computed(() => {
      const n = props.modelValue.length;
      if (n === 0) return props.allText;
      if (n === 1) return String(props.modelValue[0]);
      return `${n} selected`;
    });

    function isSelected(opt) {
      return props.modelValue.includes(opt);
    }
    function toggle(opt) {
      const next = isSelected(opt)
        ? props.modelValue.filter((v) => v !== opt)
        : [...props.modelValue, opt];
      emit("update:modelValue", next);
    }
    function clear() {
      emit("update:modelValue", []);
    }

    function onDocClick(e) {
      if (root.value && !root.value.contains(e.target)) open.value = false;
    }
    onMounted(() => document.addEventListener("click", onDocClick));
    onBeforeUnmount(() => document.removeEventListener("click", onDocClick));

    return { open, search, root, filtered, summary, isSelected, toggle, clear };
  },
  template: `
    <div class="field ms" ref="root">
      <label>{{ label }}</label>
      <button type="button" class="ms-trigger" :class="{ active: modelValue.length }" @click="open = !open">
        <span class="ms-summary">{{ summary }}</span>
        <span class="ms-caret">▾</span>
      </button>
      <div class="ms-panel" v-show="open">
        <input type="text" class="ms-search" v-model="search" placeholder="Search…" @click.stop>
        <div class="ms-actions" v-if="modelValue.length">
          <button type="button" @click="clear">Clear ({{ modelValue.length }})</button>
        </div>
        <div class="ms-list">
          <label v-for="opt in filtered" :key="opt" class="ms-option">
            <input type="checkbox" :checked="isSelected(opt)" @change="toggle(opt)">
            <span>{{ opt }}</span>
          </label>
          <div v-if="!filtered.length" class="ms-empty">No matches</div>
        </div>
      </div>
    </div>
  `,
});
