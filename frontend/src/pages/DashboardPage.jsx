import { useEffect, useState } from "react";
import { Plus, RefreshCw, ServerOff } from "lucide-react";
import { Navbar } from "@/components/Navbar";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { InstanceCard } from "@/components/InstanceCard";
import { CreateInstanceDialog } from "@/components/CreateInstanceDialog";
import { DeleteInstanceDialog } from "@/components/DeleteInstanceDialog";
import { InstanceDetailsDialog } from "@/components/InstanceDetailsDialog";
import { api } from "@/lib/api";

export default function DashboardPage() {
  const [instances, setInstances] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState("");

  const [createOpen, setCreateOpen] = useState(false);
  const [viewingInstance, setViewingInstance] = useState(null);
  const [deletingInstance, setDeletingInstance] = useState(null);

  async function loadInstances() {
    setIsLoading(true);
    setError("");
    try {
      const result = await api.listInstances();
      setInstances(result ?? []);
    } catch (err) {
      setError(err.message);
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    loadInstances();
  }, []);

  function handleCreated(instance) {
    setInstances((prev) => [instance, ...prev]);
  }

  function handleDeleted(instance) {
    setInstances((prev) => prev.filter((i) => i.compute_service_id !== instance.compute_service_id));
  }

  async function handleRefresh() {
    try {
      await loadInstances();
    } catch {
      // error state is already surfaced by loadInstances
    }
  }

  return (
    <div className="min-h-svh bg-muted/30">
      <Navbar />

      <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
        <div className="mb-6 flex items-center justify-between gap-4">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">Instances</h1>
            <p className="text-sm text-muted-foreground">
              Compute instances provisioned under your account.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon"
              onClick={handleRefresh}
              disabled={isLoading}
              aria-label="Refresh instances"
            >
              <RefreshCw className={isLoading ? "size-4 animate-spin" : "size-4"} />
            </Button>
            <Button onClick={() => setCreateOpen(true)}>
              <Plus className="size-4" />
              Create instance
            </Button>
          </div>
        </div>

        {isLoading ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="rounded-xl border bg-card p-4">
                <Skeleton className="mb-3 h-9 w-9 rounded-lg" />
                <Skeleton className="mb-2 h-4 w-2/3" />
                <Skeleton className="h-4 w-1/3" />
              </div>
            ))}
          </div>
        ) : error ? (
          <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center">
            <p className="text-sm text-destructive">{error}</p>
            <Button variant="outline" size="sm" onClick={handleRefresh}>
              Try again
            </Button>
          </div>
        ) : instances.length === 0 ? (
          <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center">
            <div className="flex size-12 items-center justify-center rounded-full bg-muted">
              <ServerOff className="size-6 text-muted-foreground" />
            </div>
            <div>
              <p className="font-medium">No instances yet</p>
              <p className="text-sm text-muted-foreground">
                Create your first instance to get started.
              </p>
            </div>
            <Button onClick={() => setCreateOpen(true)}>
              <Plus className="size-4" />
              Create instance
            </Button>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {instances.map((instance) => (
              <InstanceCard
                key={instance.id}
                instance={instance}
                onView={setViewingInstance}
                onDelete={setDeletingInstance}
              />
            ))}
          </div>
        )}
      </main>

      <CreateInstanceDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={handleCreated}
      />

      <InstanceDetailsDialog
        instance={viewingInstance}
        open={Boolean(viewingInstance)}
        onOpenChange={(open) => !open && setViewingInstance(null)}
      />

      <DeleteInstanceDialog
        instance={deletingInstance}
        open={Boolean(deletingInstance)}
        onOpenChange={(open) => !open && setDeletingInstance(null)}
        onDeleted={handleDeleted}
      />
    </div>
  );
}
