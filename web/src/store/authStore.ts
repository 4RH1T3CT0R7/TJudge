import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User } from '../types';
import api from '../api/client';

interface AuthState {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  isInitialized: boolean;
  login: (username: string, password: string) => Promise<void>;
  register: (username: string, email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  fetchUser: () => Promise<void>;
  updateProfile: (updates: { email?: string; password?: string }) => Promise<void>;
  initialize: () => Promise<void>;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      isLoading: false,
      isAuthenticated: false,
      isInitialized: false,

      login: async (username: string, password: string) => {
        set({ isLoading: true });
        try {
          const response = await api.login(username, password);
          set({ user: response.user, isAuthenticated: true, isInitialized: true });
        } finally {
          set({ isLoading: false });
        }
      },

      register: async (username: string, email: string, password: string) => {
        set({ isLoading: true });
        try {
          const response = await api.register(username, email, password);
          set({ user: response.user, isAuthenticated: true, isInitialized: true });
        } finally {
          set({ isLoading: false });
        }
      },

      logout: async () => {
        set({ isLoading: true });
        try {
          await api.logout();
        } finally {
          // Reset all auth state including isInitialized to ensure clean state for next login
          set({ user: null, isAuthenticated: false, isLoading: false, isInitialized: false });
        }
      },

      fetchUser: async () => {
        set({ isLoading: true });
        try {
          const user = await api.getMe();
          set({ user, isAuthenticated: true });
        } catch {
          set({ user: null, isAuthenticated: false });
        } finally {
          set({ isLoading: false });
        }
      },

      updateProfile: async (updates: { email?: string; password?: string }) => {
        set({ isLoading: true });
        try {
          const user = await api.updateProfile(updates);
          set({ user });
        } finally {
          set({ isLoading: false });
        }
      },

      // Initialize auth state on app start
      initialize: async () => {
        const state = get();
        if (state.isInitialized) return;

        // Check if we have a token in localStorage
        const hasToken = localStorage.getItem('access_token');
        if (hasToken) {
          set({ isLoading: true });
          try {
            const user = await api.getMe();
            set({ user, isAuthenticated: true, isInitialized: true });
          } catch {
            // Token is invalid, clear it
            api.clearTokens();
            set({ user: null, isAuthenticated: false, isInitialized: true });
          } finally {
            set({ isLoading: false });
          }
        } else {
          set({ isInitialized: true });
        }
      },
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({
        user: state.user,
        isAuthenticated: state.isAuthenticated
      }),
    }
  )
);
