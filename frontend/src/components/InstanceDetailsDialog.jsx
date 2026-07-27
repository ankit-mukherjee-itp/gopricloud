import { useEffect, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { InstanceStatusBadge } from "@/components/InstanceStatusBadge";
import { api } from "@/lib/api";

export function InstanceDetailsDialog({ instance, open, onOpenChange }) {
  const [data, setData] = useState(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open || !instance) return;

    let cancelled = false;
    setIsLoading(true);
    setError("");
    setData(null);

    api
      .getInstance(instance.compute_service_id)
      .then((result) => {
        if (!cancelled) setData(result);
      })
      .catch((err) => {
        if (!cancelled) setError(err.message);
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [open, instance]);

  const addressEntries = Object.entries(data?.addresses ?? {});

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Instance details</DialogTitle>
          <DialogDescription>Live status fetched from OpenStack Nova.</DialogDescription>
        </DialogHeader>

        {isLoading ? (
          <div className="space-y-3 py-2">
            <Skeleton className="h-5 w-2/3" />
            <Skeleton className="h-5 w-1/2" />
            <Skeleton className="h-20 w-full" />
          </div>
        ) : error ? (
          <p className="text-sm text-destructive">{error}</p>
        ) : data ? (
          <div className="space-y-4 py-2">
            <dl className="grid grid-cols-3 gap-y-2.5 text-sm">
              <dt className="text-muted-foreground">Name</dt>
              <dd className="col-span-2 font-medium">{data.name}</dd>

              <dt className="text-muted-foreground">Status</dt>
              <dd className="col-span-2">
                <InstanceStatusBadge status={data.status} />
              </dd>

              <dt className="text-muted-foreground">Instance ID</dt>
              <dd className="col-span-2 break-all font-mono text-xs">{data.id}</dd>
            </dl>

            <div>
              <p className="mb-2 text-sm font-medium">Addresses</p>
              {addressEntries.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  No addresses assigned yet — the instance may still be booting.
                </p>
              ) : (
                <div className="space-y-2">
                  {addressEntries.map(([network, addrs]) => (
                    <div key={network} className="rounded-lg border p-2.5">
                      <p className="mb-1.5 text-sm font-medium">{network}</p>
                      <ul className="space-y-1.5">
                        {addrs.map((addr, i) => (
                          <li
                            key={i}
                            className="flex items-center gap-2 font-mono text-xs text-muted-foreground"
                          >
                            <Badge variant="secondary" className="font-sans">
                              v{addr.version}
                            </Badge>
                            {addr.addr}
                          </li>
                        ))}
                      </ul>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <details className="group rounded-lg border p-2.5">
              <summary className="cursor-pointer text-sm font-medium text-muted-foreground select-none">
                Raw response
              </summary>
              <pre className="mt-2 max-h-64 overflow-auto rounded bg-muted p-3 text-xs">
                {JSON.stringify(data, null, 2)}
              </pre>
            </details>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
