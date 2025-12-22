<script lang="ts">
    import { executeQuery, type QueryResult } from "$lib/api";
    import * as Card from "$lib/components/ui/card";
    import * as Table from "$lib/components/ui/table";
    import { Button } from "$lib/components/ui/button";
    import { Badge } from "$lib/components/ui/badge";

    let query = $state("SELECT * FROM sqlite_master LIMIT 10;");
    let result = $state<QueryResult | null>(null);
    let loading = $state(false);

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

    function handleKeydown(e: KeyboardEvent) {
        if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
            runQuery();
        }
    }
</script>

<div class="p-6 h-full flex flex-col">
    <h1 class="text-2xl font-bold mb-4">Query</h1>

    <Card.Root class="mb-4">
        <Card.Content class="pt-4">
            <textarea
                bind:value={query}
                onkeydown={handleKeydown}
                placeholder="SELECT * FROM ..."
                rows="4"
                class="w-full p-3 bg-muted rounded-md font-mono text-sm resize-none focus:outline-none focus:ring-2 focus:ring-ring"
            ></textarea>
            <div class="flex items-center justify-between mt-3">
                <span class="text-xs text-muted-foreground"
                    >Ctrl+Enter to run</span
                >
                <Button onclick={runQuery} disabled={loading || !query.trim()}>
                    {loading ? "Running..." : "Run Query"}
                </Button>
            </div>
        </Card.Content>
    </Card.Root>

    {#if result}
        <Card.Root class="flex-1 overflow-hidden flex flex-col">
            <Card.Header class="py-3">
                <div class="flex items-center gap-2">
                    {#if result.error}
                        <Badge variant="destructive">Error</Badge>
                    {:else if result.data}
                        <Badge variant="secondary"
                            >{result.data.length} rows</Badge
                        >
                    {:else}
                        <Badge variant="secondary"
                            >{result.rowsAffected} affected</Badge
                        >
                    {/if}
                    {#if result.duration !== undefined}
                        <span class="text-xs text-muted-foreground"
                            >{result.duration}ms</span
                        >
                    {/if}
                </div>
            </Card.Header>
            <Card.Content class="flex-1 overflow-auto p-0">
                {#if result.error}
                    <pre
                        class="p-4 text-destructive font-mono text-sm">{result.error}</pre>
                {:else if result.data && result.columns}
                    <Table.Root>
                        <Table.Header>
                            <Table.Row>
                                {#each result.columns as col}
                                    <Table.Head class="font-mono text-xs"
                                        >{col}</Table.Head
                                    >
                                {/each}
                            </Table.Row>
                        </Table.Header>
                        <Table.Body>
                            {#each result.data as row}
                                <Table.Row>
                                    {#each result.columns as col}
                                        <Table.Cell
                                            class="font-mono text-xs max-w-xs truncate"
                                        >
                                            {row[col] === null
                                                ? "NULL"
                                                : row[col]}
                                        </Table.Cell>
                                    {/each}
                                </Table.Row>
                            {/each}
                        </Table.Body>
                    </Table.Root>
                {:else}
                    <p class="p-4 text-muted-foreground">
                        Query executed. {result.rowsAffected} row(s) affected.
                    </p>
                {/if}
            </Card.Content>
        </Card.Root>
    {/if}
</div>
