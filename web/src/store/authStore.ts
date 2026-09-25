import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import axios from 'axios';
import type { User } from '../types';
import api from '../api/client';
import { queryClient } from '../api/queryClient';

interface AuthState {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  isInitialized: boolean;
  login: (username: string, password: string) => Promise<void>;
  register: (username: string, email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  fetchUser: () => Promise<void>;
  updateProfile: (updates: { email?: string; password?: string; current_password?: string }) => Promise<void>;
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
          // isInitialized остаётся true: initialize() зовётся один раз при старте,
          // и с false защищённые маршруты висели бы на «Загрузка...».
          // Кэш запросов чистится, чтобы следующий пользователь (общий компьютер
          // в аудитории) не увидел чужую команду и программы.
          queryClient.clear();
          set({ user: null, isAuthenticated: false, isLoading: false, isInitialized: true });
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

        // Register auth failure callback so the API client can notify us
        // when token refresh fails (instead of doing window.location.href)
        api.setOnAuthFailure(() => {
          queryClient.clear();
          set({ user: null, isAuthenticated: false, isInitialized: true, isLoading: false });
        });

        // Check if we have a token in localStorage
        const hasToken = localStorage.getItem('access_token');
        if (hasToken) {
          set({ isLoading: true });
          try {
            const user = await api.getMe();
            set({ user, isAuthenticated: true, isInitialized: true });
          } catch (err) {
            // Сессию сбрасывает только 401: интерцептор уже попробовал refresh,
            // и сервер его отклонил. Сеть и 5xx на старте (деплой) токены не
            // трогают: остаётся сохранённый пользователь, запросы пойдут,
            // когда API оживёт.
            if (axios.isAxiosError(err) && err.response?.status === 401) {
              api.clearTokens();
              set({ user: null, isAuthenticated: false });
            }
            set({ isInitialized: true });
          } finally {
            set({ isLoading: false });
          }
        } else {
          set({ user: null, isAuthenticated: false, isInitialized: true });
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
