import { useState } from "react";
import type { ChangeEvent, FormEvent } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";
import type { CreateInstancePayload, Instance } from "@/lib/types";

const EMPTY_FORM = { name: "", image_ref: "", flavor_ref: "", network_id: "" };
type FormState = typeof EMPTY_FORM;

interface CreateInstanceDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (instance: Instance) => void;
}

export function CreateInstanceDialog({ open, onOpenChange, onCreated }: CreateInstanceDialogProps) {
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState("");

  function handleOpenChange(next: boolean) {
    if (!isSubmitting) {
      onOpenChange(next);
      if (!next) {
        setForm(EMPTY_FORM);
        setError("");
      }
    }
  }

  function updateField(field: keyof FormState) {
    return (e: ChangeEvent<HTMLInputElement>) =>
      setForm((prev) => ({ ...prev, [field]: e.target.value }));
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    setIsSubmitting(true);
    try {
      const payload: CreateInstancePayload = {
        name: form.name.trim(),
        image_ref: form.image_ref.trim(),
        flavor_ref: form.flavor_ref.trim(),
      };
      if (form.network_id.trim()) payload.network_id = form.network_id.trim();

      const instance = await api.createInstance(payload);
      toast.success(`Instance "${instance.name}" is being provisioned`);
      onCreated(instance);
      handleOpenChange(false);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Create instance</DialogTitle>
            <DialogDescription>
              Boots a new server via OpenStack Nova using the given image and flavor.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="name">Name</Label>
              <Input
                id="name"
                required
                value={form.name}
                onChange={updateField("name")}
                placeholder="my-instance"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="image_ref">Image ID</Label>
              <Input
                id="image_ref"
                required
                value={form.image_ref}
                onChange={updateField("image_ref")}
                placeholder="Glance image ID"
                className="font-mono text-sm"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="flavor_ref">Flavor ID</Label>
              <Input
                id="flavor_ref"
                required
                value={form.flavor_ref}
                onChange={updateField("flavor_ref")}
                placeholder="Nova flavor ID"
                className="font-mono text-sm"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="network_id">Network ID (optional)</Label>
              <Input
                id="network_id"
                value={form.network_id}
                onChange={updateField("network_id")}
                placeholder="Neutron network ID"
                className="font-mono text-sm"
              />
            </div>
          </div>

          {error ? <p className="mb-4 text-sm text-destructive">{error}</p> : null}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => handleOpenChange(false)}
              disabled={isSubmitting}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? <Loader2 className="size-4 animate-spin" /> : null}
              Create instance
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
