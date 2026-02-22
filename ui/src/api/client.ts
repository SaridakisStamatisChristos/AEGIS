import axios, { AxiosError } from 'axios';
import { useAuthStore } from '../stores/authStore';

const client = axios.create({
  baseURL: '/api/v1',
  headers: {
    'Content-Type': 'application/json',
  },
});

// Dynamically attach auth token on every request from the Zustand store
// (falls back to localStorage for backward compat).
client.interceptors.request.use((config) => {
  const storeToken = useAuthStore.getState().token;
  const token = storeToken || localStorage.getItem('auth_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Response error interceptor — centralised error handling so every caller
// gets consistent behaviour (401 redirect, structured errors, etc.)
client.interceptors.response.use(
  (response) => response,
  (error: AxiosError<{ message?: string; error?: string }>) => {
    if (error.response) {
      const { status } = error.response;

      if (status === 401) {
        // Token expired or invalid — clear local credentials and redirect.
        localStorage.removeItem('auth_token');
        localStorage.removeItem('auth_user');
        if (window.location.pathname !== '/login') {
          window.location.href = '/login';
        }
      }

      // Attach a human-readable message so callers don't have to parse raw data.
      const data = error.response.data;
      const msg =
        data?.message ?? data?.error ?? `Request failed with status ${status}`;
      (error as any).displayMessage = msg;
    } else if (error.request) {
      (error as any).displayMessage =
        'Network error — please check your connection.';
    }

    return Promise.reject(error);
  },
);

export default client;
