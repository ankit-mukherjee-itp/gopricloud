import { useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { api } from "@/lib/api";
import type { Instance } from "@/lib/types";

interface DeleteInstanceDialogProps {
  instance: Instance | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeleted: (instance: Instance) => void;
}

export function DeleteInstanceDialog({
  instance,
  open,
  onOpenChange,
  onDeleted,
}: DeleteInstanceDialogProps) {
  const [isDeleting, setIsDeleting] = useState(false);

  async function handleConfirm() {
    setIsDeleting(true);
    try {
      // `instance` is guaranteed to be set whenever this dialog can be
      // confirmed: the parent only renders it `open` while an instance is
      // selected (see DashboardPage).
      await api.deleteInstance(instance!.compute_service_id);
      toast.success(`Instance "${instance!.name}" deleted`);
      onDeleted(instance!);
      onOpenChange(false);
    } catch (err) {
      toast.error((err as Error).message);
    } finally {
      setIsDeleting(false);
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={(next) => !isDeleting && onOpenChange(next)}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete instance?</AlertDialogTitle>
          <AlertDialogDescription>
            This will permanently destroy <strong>{instance?.name}</strong> in OpenStack
            and remove it from your dashboard. This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={isDeleting}>Cancel</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            onClick={handleConfirm}
            disabled={isDeleting}
          >
            {isDeleting ? <Loader2 className="size-4 animate-spin" /> : null}
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
