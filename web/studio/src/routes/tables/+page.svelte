<script lang="ts">
    import { getTables } from "$lib/api";
    import { onMount } from "svelte";
    import * as Card from "$lib/components/ui/card";
    import { Button } from "$lib/components/ui/button";
    import { Input } from "$lib/components/ui/input";

    let tables = $state<string[]>([]);
    let loading = $state(true);
    let search = $state("");

    const filteredTables = $derived(
        tables.filter((t) => t.toLowerCase().includes(search.toLowerCase())),
    );

    onMount(async () => {
        try {
            tables = await getTables();
        } catch (e) {
            console.error("Failed to load:", e);
        } finally {
            loading = false;
        }
    });
</script>

<div class="p-6">
    <div class="flex items-center justify-between mb-6">
        <h1 class="text-2xl font-bold">Tables</h1>
        <Input
            type="text"
            placeholder="Search..."
            bind:value={search}
            class="w-64"
        />
    </div>

    {#if loading}
        <p class="text-muted-foreground">Loading...</p>
    {:else if filteredTables.length === 0}
        <Card.Root>
            <Card.Content class="py-8 text-center text-muted-foreground">
                {search ? `No tables match "${search}"` : "No tables found"}
            </Card.Content>
        </Card.Root>
    {:else}
        <div class="grid grid-cols-2 gap-3">
            {#each filteredTables as table}
                <Button
                    variant="outline"
                    href="/tables/{table}"
                    class="justify-start font-mono h-12"
                >
                    📋 {table}
                </Button>
            {/each}
        </div>
    {/if}
</div>
