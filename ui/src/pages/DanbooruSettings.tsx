import React, { useState, useEffect } from 'react';

const DanbooruSettings: React.FC = () => {
  const [username, setUsername] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [message, setMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null);

  useEffect(() => {
    fetchSettings();
  }, []);

  const fetchSettings = async () => {
    try {
      setIsLoading(true);
      const res = await fetch('/api/settings/danbooru');
      if (res.ok) {
        const data = await res.json();
        setUsername(data.username || '');
        setApiKey(data.api_key || '');
      }
    } catch (err) {
      console.error('Failed to fetch Danbooru settings:', err);
    } finally {
      setIsLoading(false);
    }
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSaving(true);
    setMessage(null);

    try {
      const res = await fetch('/api/settings/danbooru', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: username.trim(),
          api_key: apiKey.trim(),
        }),
      });

      if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        throw new Error(errData.error || 'Failed to save credentials.');
      }

      setMessage({
        text: 'Credentials saved successfully! Database and .env updated.',
        type: 'success',
      });
    } catch (err: any) {
      setMessage({
        text: err.message || 'An error occurred while saving.',
        type: 'error',
      });
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="h-full flex flex-col">
      <h1 className="text-2xl font-bold text-gray-200 mb-6">Danbooru Configuration</h1>

      <div className="max-w-4xl">
        <div className="bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-6 mb-8">
          <div className="mb-6">
            <h2 className="text-lg font-bold text-gray-200">API Credentials</h2>
            <p className="text-sm text-gray-500 mt-1">
              Configure your Danbooru account login and API key. These credentials are required for automatic metadata matching, IQDB queries, and downloading source images directly from Danbooru.
            </p>
          </div>

          {isLoading ? (
            <div className="py-8 text-center text-sm text-gray-400 font-mono animate-pulse">
              Loading current settings...
            </div>
          ) : (
            <form onSubmit={handleSave} className="space-y-6">
              <div>
                <label htmlFor="danbooru-username" className="block text-xs font-semibold text-gray-400 uppercase tracking-wide mb-2">
                  Username (Login)
                </label>
                <input
                  id="danbooru-username"
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="e.g. your_danbooru_username"
                  className="w-full max-w-md bg-[#111115] border border-[#2a2a35] text-gray-200 rounded-lg px-4 py-2.5 text-sm outline-none focus:border-[#60a5fa] transition-colors"
                />
              </div>

              <div>
                <label htmlFor="danbooru-apikey" className="block text-xs font-semibold text-gray-400 uppercase tracking-wide mb-2">
                  API Key
                </label>
                <input
                  id="danbooru-apikey"
                  type="password"
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  placeholder="e.g. AbCdEfGhIjKlMnOpQrStUvWx"
                  className="w-full max-w-md bg-[#111115] border border-[#2a2a35] text-gray-200 rounded-lg px-4 py-2.5 text-sm outline-none focus:border-[#60a5fa] transition-colors font-mono"
                />
                <p className="text-xs text-gray-500 mt-2">
                  You can generate or view your API key on Danbooru under <span className="text-gray-400 font-medium">My Account &rarr; Profile &rarr; API Keys</span>.
                </p>
              </div>

              {message && (
                <div
                  className={`p-4 rounded-lg border text-sm flex items-center gap-2 max-w-md ${
                    message.type === 'success'
                      ? 'bg-emerald-500/10 border-emerald-500/20 text-emerald-400'
                      : 'bg-red-500/10 border-red-500/20 text-red-400'
                  }`}
                >
                  {message.text}
                </div>
              )}

              <div className="pt-2">
                <button
                  type="submit"
                  disabled={isSaving}
                  className="px-6 py-2.5 bg-[#2a2a35] border border-[#3a3a45] hover:border-[#60a5fa] hover:text-[#60a5fa] text-gray-200 rounded-lg transition-colors font-medium text-sm disabled:opacity-50 cursor-pointer"
                >
                  {isSaving ? 'Saving...' : 'Save Credentials'}
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  );
};

export default DanbooruSettings;
