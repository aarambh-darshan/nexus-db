<script lang="ts">
  import "../app.css";
  import {
    getTables,
    getTableData,
    type TableDataResult,
    type ColumnInfo,
    getTableSchema,
    insertRecord,
    isForeignKey,
    getRecordById,
  } from "$lib/api";
  import { onMount } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { Badge } from "$lib/components/ui/badge";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as Dialog from "$lib/components/ui/dialog";

  // State
  let tables = $state<string[]>([]);
  let openTabs = $state<string[]>([]);
  let activeTab = $state<string | null>(null);
  let tabData = $state<
    Record<string, { columns: ColumnInfo[]; data: TableDataResult }>
  >({});
  let loading = $state(true);

  // Add record dialog
  let showAddDialog = $state(false);
  let newRecord = $state<Record<string, string>>({});
  let addLoading = $state(false);

  // Relation preview dialog
  let showRelationDialog = $state(false);
  let relationTable = $state<string | null>(null);
  let relationData = $state<Record<string, unknown> | null>(null);
  let relationLoading = $state(false);

  onMount(async () => {
    try {
      tables = await getTables();
      if (tables.length > 0) {
        openTable(tables[0]);
      }
    } catch (e) {
      console.error("Failed to load:", e);
    } finally {
      loading = false;
    }
  });

  async function openTable(name: string) {
    if (!openTabs.includes(name)) {
      openTabs = [...openTabs, name];
    }
    activeTab = name;
    await loadTableData(name);
  }

  async function loadTableData(name: string) {
    try {
      const [schema, data] = await Promise.all([
        getTableSchema(name),
        getTableData(name, 1, 100),
      ]);
      tabData[name] = { columns: schema.columns || [], data };
    } catch (e) {
      console.error("Failed to load table:", e);
    }
  }

  function closeTab(name: string) {
    openTabs = openTabs.filter((t) => t !== name);
    delete tabData[name];
    if (activeTab === name) {
      activeTab = openTabs[0] || null;
    }
  }

  function openAddDialog() {
    if (!currentData) return;
    newRecord = {};
    currentData.columns.forEach((col) => {
      if (!col.primaryKey) {
        newRecord[col.name] = "";
      }
    });
    showAddDialog = true;
  }

  async function handleAddRecord() {
    if (!activeTab) return;
    addLoading = true;
    try {
      const result = await insertRecord(activeTab, newRecord);
      if (result.error) {
        alert("Error: " + result.error);
      } else {
        showAddDialog = false;
        await loadTableData(activeTab);
      }
    } catch (e) {
      alert("Error: " + e);
    } finally {
      addLoading = false;
    }
  }

  async function handleCellClick(colName: string, value: unknown) {
    const ref = isForeignKey(colName, tables);
    if (ref && value !== null && tables.includes(ref)) {
      relationTable = ref;
      relationData = null;
      showRelationDialog = true;
      relationLoading = true;
      try {
        relationData = await getRecordById(ref, value as number);
      } catch (e) {
        console.error("Failed to load relation:", e);
      } finally {
        relationLoading = false;
      }
    }
  }

  function formatCellValue(value: unknown): string {
    if (value === null) return "";
    if (value === undefined) return "";
    if (typeof value === "boolean") return value ? "true" : "false";
    if (typeof value === "object") return JSON.stringify(value);
    return String(value);
  }

  const currentData = $derived(activeTab ? tabData[activeTab] : null);
</script>

<div class="flex h-screen dark">
  <!-- Sidebar -->
  <aside class="w-48 border-r bg-card flex flex-col">
    <div class="p-3 font-semibold text-sm border-b">All Models</div>
    <div class="flex-1 overflow-auto p-1">
      {#each tables as table}
        <button
          onclick={() => openTable(table)}
          class="w-full text-left px-3 py-2 text-sm rounded hover:bg-accent transition-colors
            {activeTab === table ? 'bg-accent font-medium' : ''}"
        >
          {table}
        </button>
      {/each}
    </div>
  </aside>

  <!-- Main -->
  <main class="flex-1 flex flex-col bg-background overflow-hidden">
    <!-- Tabs -->
    {#if openTabs.length > 0}
      <div class="flex border-b bg-card">
        {#each openTabs as tab}
          <div
            class="flex items-center gap-2 px-4 py-2 text-sm border-r transition-colors cursor-pointer
              {activeTab === tab
              ? 'bg-background font-medium'
              : 'hover:bg-accent'}"
          >
            <span onclick={() => (activeTab = tab)}>{tab}</span>
            <span
              role="button"
              tabindex="0"
              onclick={() => closeTab(tab)}
              onkeydown={(e) => e.key === "Enter" && closeTab(tab)}
              class="text-muted-foreground hover:text-foreground ml-1">×</span
            >
          </div>
        {/each}
      </div>
    {/if}

    <!-- Table View -->
    {#if currentData}
      <div class="flex items-center justify-between p-3 border-b">
        <div class="flex items-center gap-3">
          <span class="font-semibold">{activeTab}</span>
          <Badge variant="secondary">{currentData.data.total} records</Badge>
        </div>
        <Button size="sm" onclick={openAddDialog}>+ Add record</Button>
      </div>

      <!-- Spreadsheet -->
      <div class="flex-1 overflow-auto">
        <table class="w-full text-sm border-collapse">
          <thead class="sticky top-0 bg-muted z-10">
            <tr>
              {#each currentData.columns as col}
                {@const ref = isForeignKey(col.name, tables)}
                <th
                  class="text-left px-3 py-2 border-b border-r font-medium text-muted-foreground text-xs"
                >
                  {col.name}
                  {#if col.primaryKey}<Badge
                      variant="outline"
                      class="ml-1 text-xs">PK</Badge
                    >{/if}
                  {#if ref && tables.includes(ref)}<Badge
                      variant="secondary"
                      class="ml-1 text-xs">→{ref}</Badge
                    >{/if}
                </th>
              {/each}
            </tr>
          </thead>
          <tbody>
            {#each currentData.data.data as row, i}
              <tr class="hover:bg-muted/50 {i % 2 === 0 ? '' : 'bg-muted/20'}">
                {#each currentData.columns as col}
                  {@const ref = isForeignKey(col.name, tables)}
                  {@const isLink =
                    ref && tables.includes(ref) && row[col.name] !== null}
                  <td
                    class="px-3 py-1.5 border-b border-r font-mono text-xs whitespace-nowrap
                      {isLink
                      ? 'text-blue-500 cursor-pointer hover:underline'
                      : ''}"
                    onclick={() =>
                      isLink && handleCellClick(col.name, row[col.name])}
                  >
                    {#if row[col.name] === null}
                      <span class="text-muted-foreground italic">null</span>
                    {:else if col.type?.toLowerCase().includes("bool")}
                      <input
                        type="checkbox"
                        checked={Boolean(row[col.name])}
                        disabled
                      />
                    {:else}
                      {formatCellValue(row[col.name])}
                    {/if}
                  </td>
                {/each}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      <div class="p-2 border-t bg-card text-xs text-muted-foreground">
        {currentData.data.data.length} of {currentData.data.total} records
      </div>
    {:else if loading}
      <div
        class="flex-1 flex items-center justify-center text-muted-foreground"
      >
        Loading...
      </div>
    {:else}
      <div
        class="flex-1 flex items-center justify-center text-muted-foreground"
      >
        Select a model
      </div>
    {/if}
  </main>
</div>

<!-- Add Record Dialog -->
<Dialog.Root bind:open={showAddDialog}>
  <Dialog.Content
    class="max-w-md !bg-neutral-900 border-neutral-700 text-white"
  >
    <Dialog.Header>
      <Dialog.Title class="text-lg font-semibold text-white"
        >Add {activeTab} record</Dialog.Title
      >
      <Dialog.Description class="text-sm text-neutral-400">
        Fill in the fields to create a new record.
      </Dialog.Description>
    </Dialog.Header>
    <div class="space-y-4 py-4 max-h-96 overflow-auto">
      {#if currentData}
        {#each currentData.columns.filter((c) => !c.primaryKey) as col}
          <div class="space-y-2">
            <Label class="text-sm font-medium text-white">
              {col.name}
              <span
                class="ml-2 text-xs font-normal px-1.5 py-0.5 rounded bg-neutral-800 text-neutral-400"
                >{col.type}</span
              >
              {#if !col.nullable}
                <span class="ml-1 text-xs text-red-400">*</span>
              {/if}
            </Label>
            <Input
              bind:value={newRecord[col.name]}
              placeholder={col.nullable ? "(optional)" : "Enter value..."}
              class="bg-neutral-800 border-neutral-700 text-white placeholder:text-neutral-500"
            />
          </div>
        {/each}
      {/if}
    </div>
    <Dialog.Footer class="gap-2">
      <Button variant="outline" onclick={() => (showAddDialog = false)}
        >Cancel</Button
      >
      <Button onclick={handleAddRecord} disabled={addLoading}>
        {addLoading ? "Adding..." : "Add Record"}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<!-- Relation Preview Dialog -->
<Dialog.Root bind:open={showRelationDialog}>
  <Dialog.Content
    class="max-w-lg !bg-neutral-900 border-neutral-700 text-white"
  >
    <Dialog.Header>
      <Dialog.Title
        class="text-lg font-semibold text-white flex items-center gap-2"
      >
        <span>{relationTable}</span>
        <Badge variant="secondary">Related Record</Badge>
      </Dialog.Title>
    </Dialog.Header>
    <div class="py-4">
      {#if relationLoading}
        <p class="text-neutral-400">Loading...</p>
      {:else if relationData}
        <div class="rounded-lg border border-neutral-700 overflow-hidden">
          <table class="w-full text-sm">
            <tbody>
              {#each Object.entries(relationData) as [key, value]}
                <tr class="border-b border-neutral-700 last:border-0">
                  <td
                    class="px-3 py-2 font-medium text-neutral-400 bg-neutral-800 w-1/3"
                    >{key}</td
                  >
                  <td class="px-3 py-2 font-mono text-white">
                    {#if value === null}
                      <span class="text-neutral-500 italic">null</span>
                    {:else}
                      {formatCellValue(value)}
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else}
        <p class="text-neutral-400">Record not found</p>
      {/if}
    </div>
    <Dialog.Footer>
      <Button variant="outline" onclick={() => (showRelationDialog = false)}
        >Close</Button
      >
      <Button
        onclick={() => {
          showRelationDialog = false;
          if (relationTable) openTable(relationTable);
        }}
      >
        Open Full Table
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
