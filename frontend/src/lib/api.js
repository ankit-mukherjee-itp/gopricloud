import { getAccessToken, getRefreshToken, setTokens, clearSession } from "@/lib/tokens";

export class ApiError extends Error {
  constructor(message, status, body) {
    super(message);
    this.status = status;
    this.body = body;
  }
}

// Dedupe concurrent refresh attempts: if several requests 401 at once, only
// one POST /refresh should fire and everyone else should await it.
let refreshPromise = null;

function refreshAccessToken() {
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

      const data = await res.json();
      setTokens(data);
      return data;
    })().finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
}

async function parseBody(res) {
  const text = await res.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

async function request(path, { method = "GET", body, auth = true, retry = true } = {}) {
  const headers = { "Content-Type": "application/json" };
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
    return request(path, { method, body, auth, retry: false });
  }

  const data = await parseBody(res);
  if (!res.ok) {
    throw new ApiError(data?.error || `Request failed (${res.status})`, res.status, data);
  }
  return data;
}

export const api = {
  signup: (payload) => request("/signup", { method: "POST", body: payload, auth: false }),
  signin: (payload) => request("/signin", { method: "POST", body: payload, auth: false }),
  listInstances: () => request("/instances"),
  createInstance: (payload) => request("/instances", { method: "POST", body: payload }),
  getInstance: (id) => request(`/instances/${id}`),
  deleteInstance: (id) => request(`/instances/${id}`, { method: "DELETE" }),
};
