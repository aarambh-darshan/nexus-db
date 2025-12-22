<script lang="ts">
    import { getMigrations, runMigration, type MigrationInfo } from "$lib/api";
    import { onMount } from "svelte";
    import * as Card from "$lib/components/ui/card";
    import { Button } from "$lib/components/ui/button";
    import { Badge } from "$lib/components/ui/badge";
    import { Separator } from "$lib/components/ui/separator";

    let migrations = $state<MigrationInfo[]>([]);
    let loading = $state(true);
    let actionLoading = $state(false);

    async function loadMigrations() {
        try {
            migrations = await getMigrations();
        } catch (e) {
            console.error("Failed to load:", e);
        } finally {
            loading = false;
        }
    }

    async function handleAction(action: "up" | "down") {
        actionLoading = true;
        try {
            await runMigration(action);
            await loadMigrations();
        } catch (e) {
            console.error("Migration failed:", e);
        } finally {
            actionLoading = false;
        }
    }

    onMount(loadMigrations);

    const appliedCount = $derived(migrations.filter((m) => m.applied).length);
    const pendingCount = $derived(migrations.filter((m) => !m.applied).length);
</script>

<div class="p-6">
    <div class="flex items-center justify-between mb-6">
        <h1 class="text-2xl font-bold">Migrations</h1>
        <div class="flex gap-2">
            <Button
                variant="outline"
                onclick={() => handleAction("down")}
                disabled={actionLoading || appliedCount === 0}
            >
                Rollback
            </Button>
            <Button
                onclick={() => handleAction("up")}
                disabled={actionLoading || pendingCount === 0}
            >
                Migrate
            </Button>
        </div>
    </div>

    <div class="flex gap-4 mb-6">
        <Badge variant="secondary">{appliedCount} applied</Badge>
        <Badge variant="outline">{pendingCount} pending</Badge>
    </div>

    {#if loading}
        <p class="text-muted-foreground">Loading...</p>
    {:else}
        <Card.Root>
            {#if migrations.length === 0}
                <Card.Content class="py-8 text-center text-muted-foreground">
                    No migrations. Run <code class="bg-muted px-1 rounded"
                        >nexus migrate new init</code
                    >
                </Card.Content>
            {:else}
                <div class="divide-y">
                    {#each migrations as m}
                        <div class="flex items-center gap-4 px-4 py-3">
                            <div
                                class="w-2 h-2 rounded-full {m.applied
                                    ? 'bg-green-500'
                                    : 'bg-muted'}"
                            ></div>
                            <div class="flex-1">
                                <div class="font-mono text-sm">{m.id}</div>
                                <div class="text-xs text-muted-foreground">
                                    {m.name}
                                </div>
                            </div>
                            <Badge
                                variant={m.applied ? "secondary" : "outline"}
                            >
                                {m.applied ? "Applied" : "Pending"}
                            </Badge>
                        </div>
                    {/each}
                </div>
            {/if}
        </Card.Root>
    {/if}
</div>
