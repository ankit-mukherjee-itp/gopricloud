import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const STATUS_STYLES: Record<string, string> = {
  ACTIVE: "border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
  BUILD: "border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400",
  BUILDING: "border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400",
  ERROR: "border-destructive/30 bg-destructive/10 text-destructive",
  SHUTOFF: "border-border bg-muted text-muted-foreground",
  DELETED: "border-border bg-muted text-muted-foreground",
};

interface InstanceStatusBadgeProps {
  status?: string;
  className?: string;
}

export function InstanceStatusBadge({ status, className }: InstanceStatusBadgeProps) {
  const key = status ? status.toUpperCase() : undefined;
  const style = (key ? STATUS_STYLES[key] : undefined) ?? "border-border bg-muted text-muted-foreground";
  return (
    <Badge variant="outline" className={cn(style, className)}>
      {status || "UNKNOWN"}
    </Badge>
  );
}
