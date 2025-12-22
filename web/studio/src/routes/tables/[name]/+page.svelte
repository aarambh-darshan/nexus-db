<script lang="ts">
    import { page } from "$app/stores";
    import {
        getTableSchema,
        getTableData,
        type TableDataResult,
        type ColumnInfo,
    } from "$lib/api";
    import { onMount } from "svelte";
    import * as Card from "$lib/components/ui/card";
    import * as Table from "$lib/components/ui/table";
    import { Button } from "$lib/components/ui/button";
    import { Badge } from "$lib/components/ui/badge";

    const tableName = $derived($page.params.name);

    let columns = $state<ColumnInfo[]>([]);
    let data = $state<TableDataResult | null>(null);
    let loading = $state(true);
    let currentPage = $state(1);

    async function loadData(pageNum = 1) {
        loading = true;
        try {
            const [schema, tableData] = await Promise.all([
                getTableSchema(tableName),
                getTableData(tableName, pageNum),
            ]);
            columns = schema.columns || [];
            data = tableData;
            currentPage = pageNum;
        } catch (e) {
            console.error("Failed to load:", e);
        } finally {
            loading = false;
        }
    }

    onMount(() => loadData());

    function formatValue(value: unknown): string {
        if (value === null) return "NULL";
        if (value === undefined) return "";
        if (typeof value === "object") return JSON.stringify(value);
        return String(value);
    }
</script>

<div class="p-6">
    <div class="flex items-center gap-3 mb-6">
        <Button variant="ghost" href="/tables">← Back</Button>
        <h1 class="text-2xl font-bold font-mono">{tableName}</h1>
        {#if data}
            <Badge variant="secondary">{data.total} rows</Badge>
        {/if}
    </div>

    {#if loading && !data}
        <p class="text-muted-foreground">Loading...</p>
    {:else if data}
        <Card.Root>
            <Table.Root>
                <Table.Header>
                    <Table.Row>
                        {#each columns as col}
                            <Table.Head>
                                <div class="font-mono text-xs">{col.name}</div>
                                <div class="text-xs text-muted-foreground">
                                    {col.type}
                                </div>
                            </Table.Head>
                        {/each}
                    </Table.Row>
                </Table.Header>
                <Table.Body>
                    {#each data.data as row}
                        <Table.Row>
                            {#each columns as col}
                                <Table.Cell class="font-mono text-xs">
                                    <span
                                        class={row[col.name] === null
                                            ? "text-muted-foreground italic"
                                            : ""}
                                    >
                                        {formatValue(row[col.name])}
                                    </span>
                                </Table.Cell>
                            {/each}
                        </Table.Row>
                    {/each}
                </Table.Body>
            </Table.Root>
        </Card.Root>

        <!-- Pagination -->
        {#if data.pages > 1}
            <div class="flex items-center justify-center gap-2 mt-4">
                <Button
                    variant="outline"
                    disabled={currentPage <= 1}
                    onclick={() => loadData(currentPage - 1)}
                >
                    Previous
                </Button>
                <span class="text-sm text-muted-foreground">
                    {currentPage} / {data.pages}
                </span>
                <Button
                    variant="outline"
                    disabled={currentPage >= data.pages}
                    onclick={() => loadData(currentPage + 1)}
                >
                    Next
                </Button>
            </div>
        {/if}
    {/if}
</div>
