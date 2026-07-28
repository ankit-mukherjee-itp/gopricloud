import { Server, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { InstanceStatusBadge } from "@/components/InstanceStatusBadge";
import type { Instance } from "@/lib/types";

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString(undefined, {
      dateStyle: "medium",
      timeStyle: "short",
    });
  } catch {
    return iso;
  }
}

interface InstanceCardProps {
  instance: Instance;
  onView: (instance: Instance) => void;
  onDelete: (instance: Instance) => void;
}

export function InstanceCard({ instance, onView, onDelete }: InstanceCardProps) {
  return (
    <Card className="transition-shadow hover:shadow-md">
      <CardHeader className="flex-row items-start justify-between gap-2 space-y-0">
        <div className="flex items-start gap-3">
          <div className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted">
            <Server className="size-4.5 text-muted-foreground" />
          </div>
          <div className="min-w-0">
            <p className="truncate font-medium leading-tight">{instance.name}</p>
            <p className="mt-1 truncate font-mono text-xs text-muted-foreground">
              {instance.compute_service_id}
            </p>
          </div>
        </div>
        <InstanceStatusBadge status={instance.status} />
      </CardHeader>

      <CardContent className="flex items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">
          Created {formatDate(instance.created_at)}
        </p>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => onView(instance)}>
            View details
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="text-destructive hover:bg-destructive/10 hover:text-destructive"
            onClick={() => onDelete(instance)}
            aria-label={`Delete ${instance.name}`}
          >
            <Trash2 className="size-4" />
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
