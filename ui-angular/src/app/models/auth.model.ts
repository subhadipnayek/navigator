export interface AuthUser {
  id: number;
  username: string;
  email: string;
  role: 'Admin' | 'User' | string;
}

export interface AuthResponse {
  token: string;
  expiresAtUtc: string;
  user: AuthUser;
}

export interface LoginRequest {
  usernameOrEmail: string;
  password: string;
}

export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
}

export interface UpdateUserRoleRequest {
  role: 'Admin' | 'User';
}
