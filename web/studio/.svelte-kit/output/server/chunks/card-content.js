import { Y as attributes, $ as clsx, Z as bind_props } from "./index2.js";
import { c as cn } from "./button.js";
const API_BASE = "/api";
async function executeQuery(query) {
  const res = await fetch(`${API_BASE}/query`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query })
  });
  return res.json();
}
async function getMigrations() {
  const res = await fetch(`${API_BASE}/migrations`);
  const data = await res.json();
  return data.migrations || [];
}
async function runMigration(action) {
  const res = await fetch(`${API_BASE}/migrations`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action })
  });
  return res.json();
}
function Card($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let {
      ref = null,
      class: className,
      children,
      $$slots,
      $$events,
      ...restProps
    } = $$props;
    $$renderer2.push(`<div${attributes({
      "data-slot": "card",
      class: clsx(cn("bg-card text-card-foreground flex flex-col gap-6 rounded-xl border py-6 shadow-sm", className)),
      ...restProps
    })}>`);
    children?.($$renderer2);
    $$renderer2.push(`<!----></div>`);
    bind_props($$props, { ref });
  });
}
function Card_content($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let {
      ref = null,
      class: className,
      children,
      $$slots,
      $$events,
      ...restProps
    } = $$props;
    $$renderer2.push(`<div${attributes({
      "data-slot": "card-content",
      class: clsx(cn("px-6", className)),
      ...restProps
    })}>`);
    children?.($$renderer2);
    $$renderer2.push(`<!----></div>`);
    bind_props($$props, { ref });
  });
}
export {
  Card as C,
  Card_content as a,
  executeQuery as e,
  getMigrations as g,
  runMigration as r
};
