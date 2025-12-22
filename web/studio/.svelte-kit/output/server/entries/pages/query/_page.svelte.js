import { Y as attributes, $ as clsx, Z as bind_props, a0 as ensure_array_like } from "../../../chunks/index2.js";
import { C as Card, a as Card_content, e as executeQuery } from "../../../chunks/card-content.js";
import "clsx";
import { c as cn, B as Button } from "../../../chunks/button.js";
import { B as Badge } from "../../../chunks/badge.js";
import { e as escape_html } from "../../../chunks/context.js";
function Card_header($$renderer, $$props) {
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
      "data-slot": "card-header",
      class: clsx(cn("@container/card-header grid auto-rows-min grid-rows-[auto_auto] items-start gap-1.5 px-6 has-data-[slot=card-action]:grid-cols-[1fr_auto] [.border-b]:pb-6", className)),
      ...restProps
    })}>`);
    children?.($$renderer2);
    $$renderer2.push(`<!----></div>`);
    bind_props($$props, { ref });
  });
}
function Table($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let {
      ref = null,
      class: className,
      children,
      $$slots,
      $$events,
      ...restProps
    } = $$props;
    $$renderer2.push(`<div data-slot="table-container" class="relative w-full overflow-x-auto"><table${attributes({
      "data-slot": "table",
      class: clsx(cn("w-full caption-bottom text-sm", className)),
      ...restProps
    })}>`);
    children?.($$renderer2);
    $$renderer2.push(`<!----></table></div>`);
    bind_props($$props, { ref });
  });
}
function Table_body($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let {
      ref = null,
      class: className,
      children,
      $$slots,
      $$events,
      ...restProps
    } = $$props;
    $$renderer2.push(`<tbody${attributes({
      "data-slot": "table-body",
      class: clsx(cn("[&_tr:last-child]:border-0", className)),
      ...restProps
    })}>`);
    children?.($$renderer2);
    $$renderer2.push(`<!----></tbody>`);
    bind_props($$props, { ref });
  });
}
function Table_cell($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let {
      ref = null,
      class: className,
      children,
      $$slots,
      $$events,
      ...restProps
    } = $$props;
    $$renderer2.push(`<td${attributes({
      "data-slot": "table-cell",
      class: clsx(cn("bg-clip-padding p-2 align-middle whitespace-nowrap [&:has([role=checkbox])]:pe-0", className)),
      ...restProps
    })}>`);
    children?.($$renderer2);
    $$renderer2.push(`<!----></td>`);
    bind_props($$props, { ref });
  });
}
function Table_head($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let {
      ref = null,
      class: className,
      children,
      $$slots,
      $$events,
      ...restProps
    } = $$props;
    $$renderer2.push(`<th${attributes({
      "data-slot": "table-head",
      class: clsx(cn("text-foreground h-10 bg-clip-padding px-2 text-start align-middle font-medium whitespace-nowrap [&:has([role=checkbox])]:pe-0", className)),
      ...restProps
    })}>`);
    children?.($$renderer2);
    $$renderer2.push(`<!----></th>`);
    bind_props($$props, { ref });
  });
}
function Table_header($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let {
      ref = null,
      class: className,
      children,
      $$slots,
      $$events,
      ...restProps
    } = $$props;
    $$renderer2.push(`<thead${attributes({
      "data-slot": "table-header",
      class: clsx(cn("[&_tr]:border-b", className)),
      ...restProps
    })}>`);
    children?.($$renderer2);
    $$renderer2.push(`<!----></thead>`);
    bind_props($$props, { ref });
  });
}
function Table_row($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let {
      ref = null,
      class: className,
      children,
      $$slots,
      $$events,
      ...restProps
    } = $$props;
    $$renderer2.push(`<tr${attributes({
      "data-slot": "table-row",
      class: clsx(cn("hover:[&,&>svelte-css-wrapper]:[&>th,td]:bg-muted/50 data-[state=selected]:bg-muted border-b transition-colors", className)),
      ...restProps
    })}>`);
    children?.($$renderer2);
    $$renderer2.push(`<!----></tr>`);
    bind_props($$props, { ref });
  });
}
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let query = "SELECT * FROM sqlite_master LIMIT 10;";
    let result = null;
    let loading = false;
    async function runQuery() {
      if (!query.trim()) return;
      loading = true;
      try {
        result = await executeQuery(query);
      } catch (e) {
        result = { error: String(e) };
      } finally {
        loading = false;
      }
    }
    $$renderer2.push(`<div class="p-6 h-full flex flex-col"><h1 class="text-2xl font-bold mb-4">Query</h1> <!---->`);
    Card($$renderer2, {
      class: "mb-4",
      children: ($$renderer3) => {
        $$renderer3.push(`<!---->`);
        Card_content($$renderer3, {
          class: "pt-4",
          children: ($$renderer4) => {
            $$renderer4.push(`<textarea placeholder="SELECT * FROM ..." rows="4" class="w-full p-3 bg-muted rounded-md font-mono text-sm resize-none focus:outline-none focus:ring-2 focus:ring-ring">`);
            const $$body = escape_html(query);
            if ($$body) {
              $$renderer4.push(`${$$body}`);
            }
            $$renderer4.push(`</textarea> <div class="flex items-center justify-between mt-3"><span class="text-xs text-muted-foreground">Ctrl+Enter to run</span> `);
            Button($$renderer4, {
              onclick: runQuery,
              disabled: loading || !query.trim(),
              children: ($$renderer5) => {
                $$renderer5.push(`<!---->${escape_html(loading ? "Running..." : "Run Query")}`);
              },
              $$slots: { default: true }
            });
            $$renderer4.push(`<!----></div>`);
          },
          $$slots: { default: true }
        });
        $$renderer3.push(`<!---->`);
      },
      $$slots: { default: true }
    });
    $$renderer2.push(`<!----> `);
    if (result) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<!---->`);
      Card($$renderer2, {
        class: "flex-1 overflow-hidden flex flex-col",
        children: ($$renderer3) => {
          $$renderer3.push(`<!---->`);
          Card_header($$renderer3, {
            class: "py-3",
            children: ($$renderer4) => {
              $$renderer4.push(`<div class="flex items-center gap-2">`);
              if (result.error) {
                $$renderer4.push("<!--[-->");
                Badge($$renderer4, {
                  variant: "destructive",
                  children: ($$renderer5) => {
                    $$renderer5.push(`<!---->Error`);
                  },
                  $$slots: { default: true }
                });
              } else {
                $$renderer4.push("<!--[!-->");
                if (result.data) {
                  $$renderer4.push("<!--[-->");
                  Badge($$renderer4, {
                    variant: "secondary",
                    children: ($$renderer5) => {
                      $$renderer5.push(`<!---->${escape_html(result.data.length)} rows`);
                    },
                    $$slots: { default: true }
                  });
                } else {
                  $$renderer4.push("<!--[!-->");
                  Badge($$renderer4, {
                    variant: "secondary",
                    children: ($$renderer5) => {
                      $$renderer5.push(`<!---->${escape_html(result.rowsAffected)} affected`);
                    },
                    $$slots: { default: true }
                  });
                }
                $$renderer4.push(`<!--]-->`);
              }
              $$renderer4.push(`<!--]--> `);
              if (result.duration !== void 0) {
                $$renderer4.push("<!--[-->");
                $$renderer4.push(`<span class="text-xs text-muted-foreground">${escape_html(result.duration)}ms</span>`);
              } else {
                $$renderer4.push("<!--[!-->");
              }
              $$renderer4.push(`<!--]--></div>`);
            },
            $$slots: { default: true }
          });
          $$renderer3.push(`<!----> <!---->`);
          Card_content($$renderer3, {
            class: "flex-1 overflow-auto p-0",
            children: ($$renderer4) => {
              if (result.error) {
                $$renderer4.push("<!--[-->");
                $$renderer4.push(`<pre class="p-4 text-destructive font-mono text-sm">${escape_html(result.error)}</pre>`);
              } else {
                $$renderer4.push("<!--[!-->");
                if (result.data && result.columns) {
                  $$renderer4.push("<!--[-->");
                  $$renderer4.push(`<!---->`);
                  Table($$renderer4, {
                    children: ($$renderer5) => {
                      $$renderer5.push(`<!---->`);
                      Table_header($$renderer5, {
                        children: ($$renderer6) => {
                          $$renderer6.push(`<!---->`);
                          Table_row($$renderer6, {
                            children: ($$renderer7) => {
                              $$renderer7.push(`<!--[-->`);
                              const each_array = ensure_array_like(result.columns);
                              for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
                                let col = each_array[$$index];
                                $$renderer7.push(`<!---->`);
                                Table_head($$renderer7, {
                                  class: "font-mono text-xs",
                                  children: ($$renderer8) => {
                                    $$renderer8.push(`<!---->${escape_html(col)}`);
                                  },
                                  $$slots: { default: true }
                                });
                                $$renderer7.push(`<!---->`);
                              }
                              $$renderer7.push(`<!--]-->`);
                            },
                            $$slots: { default: true }
                          });
                          $$renderer6.push(`<!---->`);
                        },
                        $$slots: { default: true }
                      });
                      $$renderer5.push(`<!----> <!---->`);
                      Table_body($$renderer5, {
                        children: ($$renderer6) => {
                          $$renderer6.push(`<!--[-->`);
                          const each_array_1 = ensure_array_like(result.data);
                          for (let $$index_2 = 0, $$length = each_array_1.length; $$index_2 < $$length; $$index_2++) {
                            let row = each_array_1[$$index_2];
                            $$renderer6.push(`<!---->`);
                            Table_row($$renderer6, {
                              children: ($$renderer7) => {
                                $$renderer7.push(`<!--[-->`);
                                const each_array_2 = ensure_array_like(result.columns);
                                for (let $$index_1 = 0, $$length2 = each_array_2.length; $$index_1 < $$length2; $$index_1++) {
                                  let col = each_array_2[$$index_1];
                                  $$renderer7.push(`<!---->`);
                                  Table_cell($$renderer7, {
                                    class: "font-mono text-xs max-w-xs truncate",
                                    children: ($$renderer8) => {
                                      $$renderer8.push(`<!---->${escape_html(row[col] === null ? "NULL" : row[col])}`);
                                    },
                                    $$slots: { default: true }
                                  });
                                  $$renderer7.push(`<!---->`);
                                }
                                $$renderer7.push(`<!--]-->`);
                              },
                              $$slots: { default: true }
                            });
                            $$renderer6.push(`<!---->`);
                          }
                          $$renderer6.push(`<!--]-->`);
                        },
                        $$slots: { default: true }
                      });
                      $$renderer5.push(`<!---->`);
                    },
                    $$slots: { default: true }
                  });
                  $$renderer4.push(`<!---->`);
                } else {
                  $$renderer4.push("<!--[!-->");
                  $$renderer4.push(`<p class="p-4 text-muted-foreground">Query executed. ${escape_html(result.rowsAffected)} row(s) affected.</p>`);
                }
                $$renderer4.push(`<!--]-->`);
              }
              $$renderer4.push(`<!--]-->`);
            },
            $$slots: { default: true }
          });
          $$renderer3.push(`<!---->`);
        },
        $$slots: { default: true }
      });
      $$renderer2.push(`<!---->`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div>`);
  });
}
export {
  _page as default
};
