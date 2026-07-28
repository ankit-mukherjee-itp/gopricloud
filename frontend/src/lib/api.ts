import { getAccessToken, getRefreshToken, setTokens, clearSession } from "@/lib/tokens";
import type {
  AuthResponse,
  CreateInstancePayload,
  Instance,
  InstanceServer,
  SigninPayload,
  SignupPayload,
} from "@/lib/types";

export class ApiError extends Error {
  status?: number;
  body?: unknown;

  constructor(message: string, status?: number, body?: unknown) {
    super(message);
    this.status = status;
    this.body = body;
  }
}

// Dedupe concurrent refresh attempts: if several requests 401 at once, only
// one POST /refresh should fire and everyone else should await it.
let refreshPromise: Promise<AuthResponse> | null = null;

function refreshAccessToken(): Promise<AuthResponse> {
  if (!refreshPromise) {
    refreshPromise = (async () => {
      const refreshToken = getRefreshToken();
      if (!refreshToken) throw new ApiError("No refresh token", 401);

      const res = await fetch("/refresh", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refreshToken }),
      });
      if (!res.ok) throw new ApiError("Session expired", res.status);

      const data: AuthResponse = await res.json();
      setTokens(data);
      return data;
    })().finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
}

async function parseBody(res: Response): Promise<any> {
  const text = await res.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

interface RequestOptions {
  method?: string;
  body?: unknown;
  auth?: boolean;
  retry?: boolean;
}

async function request<T>(
  path: string,
  { method = "GET", body, auth = true, retry = true }: RequestOptions = {},
): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (auth) {
    const token = getAccessToken();
    if (token) headers.Authorization = `Bearer ${token}`;
  }

  const res = await fetch(path, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (res.status === 401 && auth && retry) {
    try {
      await refreshAccessToken();
    } catch {
      clearSession();
      window.dispatchEvent(new Event("auth:unauthorized"));
      throw new ApiError("Your session has expired. Please sign in again.", 401);
    }
    return request<T>(path, { method, body, auth, retry: false });
  }

  const data = await parseBody(res);
  if (!res.ok) {
    throw new ApiError(data?.error || `Request failed (${res.status})`, res.status, data);
  }
  return data as T;
}

export const api = {
  signup: (payload: SignupPayload) =>
    request<AuthResponse>("/signup", { method: "POST", body: payload, auth: false }),
  signin: (payload: SigninPayload) =>
    request<AuthResponse>("/signin", { method: "POST", body: payload, auth: false }),
  listInstances: () => request<Instance[]>("/instances"),
  createInstance: (payload: CreateInstancePayload) =>
    request<Instance>("/instances", { method: "POST", body: payload }),
  getInstance: (id: string) => request<InstanceServer>(`/instances/${id}`),
  deleteInstance: (id: string) => request<unknown>(`/instances/${id}`, { method: "DELETE" }),
};
