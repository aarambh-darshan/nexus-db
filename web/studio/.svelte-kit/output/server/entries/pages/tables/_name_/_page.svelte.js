import { a5 as store_get, a6 as unsubscribe_stores } from "../../../../chunks/index2.js";
import { g as getContext, e as escape_html } from "../../../../chunks/context.js";
import "clsx";
import "@sveltejs/kit/internal";
import "../../../../chunks/exports.js";
import "../../../../chunks/utils.js";
import "@sveltejs/kit/internal/server";
import "../../../../chunks/state.svelte.js";
import { B as Button } from "../../../../chunks/button.js";
import "../../../../chunks/badge.js";
const getStores = () => {
  const stores$1 = getContext("__svelte__");
  return {
    /** @type {typeof page} */
    page: {
      subscribe: stores$1.page.subscribe
    },
    /** @type {typeof navigating} */
    navigating: {
      subscribe: stores$1.navigating.subscribe
    },
    /** @type {typeof updated} */
    updated: stores$1.updated
  };
};
const page = {
  subscribe(fn) {
    const store = getStores().page;
    return store.subscribe(fn);
  }
};
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    const tableName = store_get($$store_subs ??= {}, "$page", page).params.name;
    $$renderer2.push(`<div class="p-6"><div class="flex items-center gap-3 mb-6">`);
    Button($$renderer2, {
      variant: "ghost",
      href: "/tables",
      children: ($$renderer3) => {
        $$renderer3.push(`<!---->← Back`);
      },
      $$slots: { default: true }
    });
    $$renderer2.push(`<!----> <h1 class="text-2xl font-bold font-mono">${escape_html(tableName)}</h1> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div> `);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<p class="text-muted-foreground">Loading...</p>`);
    }
    $$renderer2.push(`<!--]--></div>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
