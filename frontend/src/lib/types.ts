// Shared domain types mirroring the backend wire shapes.
//
// Sources of truth (do not drift from these without updating both sides):
//   - gopricloud/internal/delivery/http/dto/auth_dto.go
//   - gopricloud/internal/delivery/http/dto/compute_dto.go

/** dto.UserResponse */
export interface User {
  id: string;
  name: string;
  email: string;
}

/** dto.AuthResponse — response body of POST /signup and POST /signin. */
export interface AuthResponse {
  user: User;
  access_token: string;
  access_token_expires_at: string;
  refresh_token: string;
  refresh_token_expires_at: string;
}

/** dto.ErrorResponse */
export interface ErrorResponse {
  error: string;
}

/** dto.ComputeResponse — an instance record as tracked in our DB (list/create). */
export interface Instance {
  id: string;
  compute_service_id: string;
  name: string;
  status: string;
  created_at: string;
}

/** A single address entry within dto.ComputeServerResponse.Addresses. */
export interface InstanceAddress {
  addr: string;
  version: number;
}

/**
 * dto.ComputeServerResponse — an instance's live state as reported by Nova
 * (GET /instances/{id}). `addresses` is a map of network name -> addresses.
 */
export interface InstanceServer {
  id: string;
  name: string;
  status: string;
  addresses: Record<string, InstanceAddress[]>;
}

/** dto.SignupRequest */
export interface SignupPayload {
  name: string;
  email: string;
  password: string;
}

/** dto.SigninRequest */
export interface SigninPayload {
  email: string;
  password: string;
}

/** dto.CreateComputeRequest */
export interface CreateInstancePayload {
  name: string;
  image_ref: string;
  flavor_ref: string;
  network_id?: string;
}
