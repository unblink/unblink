import { createSignal } from "solid-js";
import { authClient } from "./rpc";
import { type User } from "../../gen/unblink/auth/v1/auth_pb";

// Re-export User type from proto
export type { User };

import posthog from "./posthog";

const TOKEN_KEY = 'auth_token';

// Token management
export const setToken = (token: string) => {
  console.log('[setToken] Saving token to localStorage (first 20 chars):', token.substring(0, 20) + '...');
  localStorage.setItem(TOKEN_KEY, token);
};

export const getToken = () => {
  const token = localStorage.getItem(TOKEN_KEY);
  console.log('[getToken] Token loaded from localStorage:', token ? `${token.substring(0, 20)}...` : 'null');
  return token;
};

export const clearToken = () => {
  console.log('[clearToken] Removing token from localStorage');
  localStorage.removeItem(TOKEN_KEY);
};


export interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}

// Auth state
export const [authState, setAuthState] = createSignal<AuthState>({
  user: null,
  isAuthenticated: false,
  isLoading: true,
});

// RPC calls
async function rpcRegister(email: string, password: string, name: string): Promise<{ user: User }> {
  const res = await authClient.register({ email, password, name });
  if (!res.success || !res.user) {
    throw new Error('Registration failed');
  }
  setToken(res.token);
  return { user: res.user };
}

async function rpcLogin(email: string, password: string): Promise<{ user: User }> {
  const res = await authClient.login({ email, password });
  if (!res.success || !res.user) {
    throw new Error('Login failed');
  }
  setToken(res.token);
  return { user: res.user };
}

async function rpcGetMe(): Promise<{ user: User }> {
  const res = await authClient.getUser({});
  if (!res.success || !res.user) {
    throw new Error('Failed to get user');
  }
  return { user: res.user };
}

async function rpcLogout() {
  localStorage.removeItem(TOKEN_KEY);
  return { success: true, message: 'Logged out' };
}

// High-level auth functions
export const login = async (email: string, password: string): Promise<{ success: boolean; message?: string; user?: User }> => {
  try {
    const { user } = await rpcLogin(email, password);
    console.log('[login] Login successful, saving token');
    setAuthState({
      user,
      isAuthenticated: true,
      isLoading: false,
    });
    posthog.identify(user.id);
    posthog.capture('user_logged_in', { email: user.email });
    return { success: true, user };
  } catch (error: any) {
    console.error('Login error:', error);
    posthog.capture('login_failed', { email });
    return { success: false, message: error?.message || 'Login failed' };
  }
};

export const register = async (email: string, password: string, name: string): Promise<{ success: boolean; message?: string; user?: User }> => {
  try {
    const { user } = await rpcRegister(email, password, name);
    console.log('[register] Registration successful');
    posthog.capture('user_registered', { email, name });
    return { success: true, message: 'Registration successful', user };
  } catch (error: any) {
    console.error('Register error:', error);
    return { success: false, message: error?.message || 'Registration failed' };
  }
};

export const logout = async () => {
  clearToken();
  setAuthState({
    user: null,
    isAuthenticated: false,
    isLoading: false,
  });
  posthog.capture('user_logged_out');
  posthog.reset();
  await rpcLogout();
};

export const getMe = async () => {
  const { user } = await rpcGetMe();
  return { success: true, user };
};

export const initAuth = async () => {
  console.log('[initAuth] Starting auth check...');
  try {
    const data = await getMe();
    console.log('[initAuth] Response data:', data);
    if (data.success && data.user) {
      setAuthState({
        user: data.user,
        isAuthenticated: true,
        isLoading: false,
      });
      console.log('[initAuth] User authenticated:', data.user);
    } else {
      setAuthState({
        user: null,
        isAuthenticated: false,
        isLoading: false,
      });
      console.log('[initAuth] Response not successful');
    }
  } catch (error: any) {
    console.error('[initAuth] Auth check failed:', error);
    if (!error?.message?.includes('not authenticated')) {
      console.error('[initAuth] Unexpected error:', error);
    }
    setAuthState({
      user: null,
      isAuthenticated: false,
      isLoading: false,
    });
  }
};