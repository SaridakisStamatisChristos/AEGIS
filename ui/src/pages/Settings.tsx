import React, { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { Save, Key, Shield, Bell, Database } from 'lucide-react';
import client from '../api/client';

interface SettingsForm {
  apiEndpoint: string;
  defaultPolicy: string;
  notificationsEnabled: boolean;
  autoApprove: boolean;
  retentionDays: number;
}

export default function Settings() {
  const [settings, setSettings] = useState<SettingsForm>({
    apiEndpoint: 'http://localhost:8080',
    defaultPolicy: '',
    notificationsEnabled: true,
    autoApprove: false,
    retentionDays: 90,
  });

  const [saved, setSaved] = useState(false);

  const saveMutation = useMutation({
    mutationFn: async (newSettings: SettingsForm) => {
      await client.put('/settings', newSettings);
    },
    onSuccess: () => {
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    saveMutation.mutate(settings);
  };

  return (
    <div className="max-w-3xl">
      <div className="sm:flex sm:items-center mb-8">
        <div className="sm:flex-auto">
          <h1 className="text-2xl font-semibold text-gray-900">Settings</h1>
          <p className="mt-2 text-sm text-gray-700">
            Configure your AegisRun instance
          </p>
        </div>
      </div>

      <form onSubmit={handleSubmit} className="space-y-8">
        {/* API Configuration */}
        <div className="bg-white shadow rounded-lg p-6">
          <div className="flex items-center mb-4">
            <Key className="h-5 w-5 text-gray-400 mr-2" />
            <h2 className="text-lg font-medium text-gray-900">API Configuration</h2>
          </div>

          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700">
                API Endpoint
              </label>
              <input
                type="url"
                value={settings.apiEndpoint}
                onChange={(e) => setSettings({ ...settings, apiEndpoint: e.target.value })}
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
          </div>
        </div>

        {/* Policy Settings */}
        <div className="bg-white shadow rounded-lg p-6">
          <div className="flex items-center mb-4">
            <Shield className="h-5 w-5 text-gray-400 mr-2" />
            <h2 className="text-lg font-medium text-gray-900">Policy Settings</h2>
          </div>

          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700">
                Default Policy ID
              </label>
              <input
                type="text"
                value={settings.defaultPolicy}
                onChange={(e) => setSettings({ ...settings, defaultPolicy: e.target.value })}
                placeholder="e.g., pol_default"
                className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
              <p className="mt-1 text-xs text-gray-500">
                Policy to use when none is specified
              </p>
            </div>

            <div className="flex items-center">
              <input
                type="checkbox"
                id="autoApprove"
                checked={settings.autoApprove}
                onChange={(e) => setSettings({ ...settings, autoApprove: e.target.checked })}
                className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              <label htmlFor="autoApprove" className="ml-2 block text-sm text-gray-700">
                Auto-approve low-risk tool calls
              </label>
            </div>
          </div>
        </div>

        {/* Notifications */}
        <div className="bg-white shadow rounded-lg p-6">
          <div className="flex items-center mb-4">
            <Bell className="h-5 w-5 text-gray-400 mr-2" />
            <h2 className="text-lg font-medium text-gray-900">Notifications</h2>
          </div>

          <div className="flex items-center">
            <input
              type="checkbox"
              id="notifications"
              checked={settings.notificationsEnabled}
              onChange={(e) => setSettings({ ...settings, notificationsEnabled: e.target.checked })}
              className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
            />
            <label htmlFor="notifications" className="ml-2 block text-sm text-gray-700">
              Enable notifications for pending approvals
            </label>
          </div>
        </div>

        {/* Data Retention */}
        <div className="bg-white shadow rounded-lg p-6">
          <div className="flex items-center mb-4">
            <Database className="h-5 w-5 text-gray-400 mr-2" />
            <h2 className="text-lg font-medium text-gray-900">Data Retention</h2>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700">
              Retention Period (days)
            </label>
            <input
              type="number"
              min="1"
              max="365"
              value={settings.retentionDays}
              onChange={(e) => setSettings({ ...settings, retentionDays: parseInt(e.target.value) })}
              className="mt-1 block w-32 rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            />
            <p className="mt-1 text-xs text-gray-500">
              How long to retain run data and evidence bundles
            </p>
          </div>
        </div>

        {/* Submit */}
        <div className="flex items-center justify-end space-x-3">
          {saved && (
            <span className="text-sm text-green-600">Settings saved successfully!</span>
          )}
          <button
            type="submit"
            disabled={saveMutation.isPending}
            className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50"
          >
            <Save className="h-4 w-4 mr-2" />
            {saveMutation.isPending ? 'Saving...' : 'Save Settings'}
          </button>
        </div>
      </form>
    </div>
  );
}
