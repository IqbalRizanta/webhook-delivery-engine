'use client'

import React, { useState, useEffect } from 'react'
import {
  Activity,
  Send,
  Radio,
  RefreshCw,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Clock,
  Shield,
  Layers,
  ExternalLink,
  Plus,
  Trash2,
  Copy
} from 'lucide-react'

interface Endpoint {
  id: string
  url: string
  secret: string
  description: string
  event_types: string[]
  is_active: boolean
  created_at: string
}

interface DeliveryAttempt {
  id: string
  event_id: string
  event_type: string
  endpoint_id: string
  endpoint_url: string
  payload: any
  headers_sent?: Record<string, string>
  attempt_number: number
  max_attempts: number
  status_code: number
  response_body: string
  error_message?: string
  latency_ms: number
  status: 'pending' | 'success' | 'failed' | 'retrying' | 'dead_letter'
  next_retry_at?: string
  created_at: string
}

interface SystemStats {
  total_endpoints: number
  total_events: number
  total_attempts: number
  successful_attempts: number
  failed_attempts: number
  retrying_attempts: number
  average_latency_ms: number
}

const API_BASE = 'http://localhost:8080'

export default function WebhookDashboard() {
  const [stats, setStats] = useState<SystemStats | null>(null)
  const [endpoints, setEndpoints] = useState<Endpoint[]>([])
  const [attempts, setAttempts] = useState<DeliveryAttempt[]>([])
  const [selectedAttempt, setSelectedAttempt] = useState<DeliveryAttempt | null>(null)
  
  // New Endpoint form state
  const [newUrl, setNewUrl] = useState('http://localhost:8080/api/v1/test-receiver')
  const [newDesc, setNewDesc] = useState('My Local Webhook Consumer')
  const [newEventTypes, setNewEventTypes] = useState('*')

  // Dispatch Simulator form state
  const [simEventType, setSimEventType] = useState('payment.succeeded')
  const [simPayload, setSimPayload] = useState(
    JSON.stringify(
      {
        id: 'evt_998124',
        amount: 250000,
        currency: 'IDR',
        customer: { id: 'cust_01', email: 'user@example.com' },
        timestamp: new Date().toISOString()
      },
      null,
      2
    )
  )
  const [isDispatching, setIsDispatching] = useState(false)
  const [activeTab, setActiveTab] = useState<'attempts' | 'endpoints' | 'simulator'>('attempts')

  const fetchData = async () => {
    try {
      const [resStats, resEndpoints, resAttempts] = await Promise.all([
        fetch(`${API_BASE}/api/v1/stats`),
        fetch(`${API_BASE}/api/v1/endpoints`),
        fetch(`${API_BASE}/api/v1/attempts?limit=50`)
      ])
      if (resStats.ok) setStats(await resStats.json())
      if (resEndpoints.ok) {
        const json = await resEndpoints.json()
        setEndpoints(json.data || [])
      }
      if (resAttempts.ok) {
        const json = await resAttempts.json()
        setAttempts(json.data || [])
      }
    } catch (err) {
      console.warn('Backend server not connected yet on :8080', err)
    }
  }

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 3000)

    // Optional SSE connection for sub-second updates
    let eventSource: EventSource | null = null
    try {
      eventSource = new EventSource(`${API_BASE}/api/v1/events/stream`)
      eventSource.addEventListener('attempt', (e) => {
        const updatedAttempt: DeliveryAttempt = JSON.parse(e.data)
        setAttempts((prev) => {
          const index = prev.findIndex((a) => a.id === updatedAttempt.id)
          if (index >= 0) {
            const next = [...prev]
            next[index] = updatedAttempt
            return next
          }
          return [updatedAttempt, ...prev]
        })
      })
    } catch (e) {
      // Fallback to polling
    }

    return () => {
      clearInterval(interval)
      if (eventSource) eventSource.close()
    }
  }, [])

  const handleRegisterEndpoint = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      const res = await fetch(`${API_BASE}/api/v1/endpoints`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          url: newUrl,
          description: newDesc,
          event_types: newEventTypes.split(',').map((s) => s.trim())
        })
      })
      if (res.ok) {
        setNewUrl('')
        setNewDesc('')
        fetchData()
        setActiveTab('endpoints')
      }
    } catch (err) {
      alert('Failed to connect to backend on :8080')
    }
  }

  const handleDeleteEndpoint = async (id: string) => {
    if (!confirm('Are you sure you want to delete this endpoint?')) return
    await fetch(`${API_BASE}/api/v1/endpoints/${id}`, { method: 'DELETE' })
    fetchData()
  }

  const handleDispatchEvent = async () => {
    setIsDispatching(true)
    try {
      let parsedPayload: any
      try {
        parsedPayload = JSON.parse(simPayload)
      } catch {
        alert('Invalid JSON in payload')
        setIsDispatching(false)
        return
      }

      const res = await fetch(`${API_BASE}/api/v1/events`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          event_type: simEventType,
          payload: parsedPayload
        })
      })
      if (res.ok) {
        setTimeout(fetchData, 400)
        setActiveTab('attempts')
      }
    } catch (err) {
      alert('Error contacting backend server')
    } finally {
      setIsDispatching(false)
    }
  }

  const handleManualRetry = async (attemptId: string) => {
    try {
      await fetch(`${API_BASE}/api/v1/attempts/${attemptId}/retry`, { method: 'POST' })
      fetchData()
    } catch (err) {
      alert('Retry trigger failed')
    }
  }

  const getStatusBadge = (status: DeliveryAttempt['status'], code: number) => {
    switch (status) {
      case 'success':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-emerald-950/80 text-emerald-400 border border-emerald-800/60">
            <CheckCircle2 className="w-3.5 h-3.5" /> 2xx OK ({code})
          </span>
        )
      case 'retrying':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-amber-950/80 text-amber-400 border border-amber-800/60 animate-pulse">
            <Clock className="w-3.5 h-3.5" /> Retrying...
          </span>
        )
      case 'dead_letter':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-red-950/80 text-red-400 border border-red-800/60">
            <XCircle className="w-3.5 h-3.5" /> DLQ Exhausted
          </span>
        )
      default:
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-blue-950/80 text-blue-400 border border-blue-800/60">
            <RefreshCw className="w-3.5 h-3.5 animate-spin" /> In Flight
          </span>
        )
    }
  }

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans">
      {/* Top Navbar */}
      <header className="border-b border-slate-800/80 bg-slate-900/60 backdrop-blur px-6 py-4 flex items-center justify-between sticky top-0 z-20">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-indigo-600/20 border border-indigo-500/40 rounded-lg text-indigo-400">
            <Radio className="w-5 h-5 animate-pulse" />
          </div>
          <div>
            <h1 className="text-lg font-bold tracking-tight text-white flex items-center gap-2">
              Webhook Delivery Engine
              <span className="text-[10px] px-2 py-0.5 rounded-full bg-indigo-950 text-indigo-400 border border-indigo-800 font-mono">
                v1.0 Go + Next.js
              </span>
            </h1>
            <p className="text-xs text-slate-400">
              High-throughput dispatcher with HMAC-SHA256 & Exponential Backoff
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={fetchData}
            className="flex items-center gap-2 text-xs font-medium px-3 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            Refresh
          </button>
          <button
            onClick={() => {
              setActiveTab('simulator')
              handleDispatchEvent()
            }}
            disabled={isDispatching}
            className="flex items-center gap-2 text-xs font-semibold px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white shadow-lg shadow-indigo-600/20 transition"
          >
            <Send className="w-3.5 h-3.5" />
            Dispatch Demo Event
          </button>
        </div>
      </header>

      {/* Main Container */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-6 py-8 space-y-8">
        {/* Metrics Grid */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <div className="p-5 rounded-xl border border-slate-800 bg-slate-900/40 backdrop-blur">
            <div className="flex items-center justify-between text-slate-400 text-xs">
              <span>Active Endpoints</span>
              <Layers className="w-4 h-4 text-indigo-400" />
            </div>
            <div className="text-2xl font-bold mt-2 text-white">
              {stats?.total_endpoints ?? 0}
            </div>
            <div className="text-[11px] text-slate-500 mt-1">Configured webhook targets</div>
          </div>

          <div className="p-5 rounded-xl border border-slate-800 bg-slate-900/40 backdrop-blur">
            <div className="flex items-center justify-between text-slate-400 text-xs">
              <span>Total Deliveries</span>
              <Send className="w-4 h-4 text-sky-400" />
            </div>
            <div className="text-2xl font-bold mt-2 text-white">
              {stats?.total_attempts ?? 0}
            </div>
            <div className="text-[11px] text-slate-500 mt-1">Across all events & retries</div>
          </div>

          <div className="p-5 rounded-xl border border-slate-800 bg-slate-900/40 backdrop-blur">
            <div className="flex items-center justify-between text-slate-400 text-xs">
              <span>Success Rate</span>
              <CheckCircle2 className="w-4 h-4 text-emerald-400" />
            </div>
            <div className="text-2xl font-bold mt-2 text-emerald-400">
              {stats && stats.total_attempts > 0
                ? `${Math.round((stats.successful_attempts / stats.total_attempts) * 100)}%`
                : '100%'}
            </div>
            <div className="text-[11px] text-slate-500 mt-1">
              {stats?.successful_attempts ?? 0} successful / {stats?.failed_attempts ?? 0} failed
            </div>
          </div>

          <div className="p-5 rounded-xl border border-slate-800 bg-slate-900/40 backdrop-blur">
            <div className="flex items-center justify-between text-slate-400 text-xs">
              <span>Average Latency</span>
              <Activity className="w-4 h-4 text-amber-400" />
            </div>
            <div className="text-2xl font-bold mt-2 text-white">
              {stats?.average_latency_ms ? `${stats.average_latency_ms.toFixed(1)} ms` : '< 5 ms'}
            </div>
            <div className="text-[11px] text-slate-500 mt-1">End-to-end HTTP response time</div>
          </div>
        </div>

        {/* Navigation Tabs */}
        <div className="flex border-b border-slate-800 text-sm font-medium gap-8">
          <button
            onClick={() => setActiveTab('attempts')}
            className={`pb-3 border-b-2 flex items-center gap-2 transition ${
              activeTab === 'attempts'
                ? 'border-indigo-500 text-indigo-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <Activity className="w-4 h-4" /> Live Delivery Feed ({attempts.length})
          </button>
          <button
            onClick={() => setActiveTab('endpoints')}
            className={`pb-3 border-b-2 flex items-center gap-2 transition ${
              activeTab === 'endpoints'
                ? 'border-indigo-500 text-indigo-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <Layers className="w-4 h-4" /> Subscribers ({endpoints.length})
          </button>
          <button
            onClick={() => setActiveTab('simulator')}
            className={`pb-3 border-b-2 flex items-center gap-2 transition ${
              activeTab === 'simulator'
                ? 'border-indigo-500 text-indigo-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <Send className="w-4 h-4" /> Dispatch Simulator
          </button>
        </div>

        {/* Tab 1: Delivery Attempts Table */}
        {activeTab === 'attempts' && (
          <div className="space-y-4">
            <div className="rounded-xl border border-slate-800 bg-slate-900/30 overflow-hidden">
              <table className="w-full text-left text-xs border-collapse">
                <thead>
                  <tr className="bg-slate-900/80 border-b border-slate-800 text-slate-400 font-semibold uppercase tracking-wider">
                    <th className="py-3 px-4">Status</th>
                    <th className="py-3 px-4">Event Type</th>
                    <th className="py-3 px-4">Endpoint Target</th>
                    <th className="py-3 px-4">Attempt</th>
                    <th className="py-3 px-4">Latency</th>
                    <th className="py-3 px-4">Timestamp</th>
                    <th className="py-3 px-4 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60 font-mono">
                  {attempts.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="py-12 text-center text-slate-500 font-sans">
                        No delivery attempts yet. Click{' '}
                        <button
                          onClick={handleDispatchEvent}
                          className="text-indigo-400 underline font-semibold"
                        >
                          Dispatch Demo Event
                        </button>{' '}
                        to test the engine!
                      </td>
                    </tr>
                  ) : (
                    attempts.map((att) => (
                      <tr key={att.id} className="hover:bg-slate-800/40 transition">
                        <td className="py-3 px-4">
                          {getStatusBadge(att.status, att.status_code)}
                        </td>
                        <td className="py-3 px-4 font-semibold text-slate-200">
                          {att.event_type}
                        </td>
                        <td className="py-3 px-4 text-slate-400 truncate max-w-xs">
                          {att.endpoint_url}
                        </td>
                        <td className="py-3 px-4 text-slate-300">
                          #{att.attempt_number} of {att.max_attempts}
                        </td>
                        <td className="py-3 px-4 text-slate-300">
                          {att.latency_ms > 0 ? `${att.latency_ms}ms` : '-'}
                        </td>
                        <td className="py-3 px-4 text-slate-400 text-[11px]">
                          {new Date(att.created_at).toLocaleTimeString()}
                        </td>
                        <td className="py-3 px-4 text-right space-x-2">
                          <button
                            onClick={() => setSelectedAttempt(att)}
                            className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-sans transition"
                          >
                            Inspect
                          </button>
                          {(att.status === 'failed' || att.status === 'dead_letter') && (
                            <button
                              onClick={() => handleManualRetry(att.id)}
                              className="px-2.5 py-1 rounded bg-indigo-900/60 hover:bg-indigo-800 text-indigo-300 text-xs font-sans transition"
                            >
                              Replay
                            </button>
                          )}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* Tab 2: Endpoints Management */}
        {activeTab === 'endpoints' && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
            {/* Endpoints list */}
            <div className="lg:col-span-2 space-y-4">
              <h2 className="text-base font-semibold text-white">Registered Endpoints</h2>
              <div className="space-y-3">
                {endpoints.map((ep) => (
                  <div
                    key={ep.id}
                    className="p-5 rounded-xl border border-slate-800 bg-slate-900/40 flex flex-col gap-3"
                  >
                    <div className="flex items-start justify-between">
                      <div>
                        <h3 className="font-semibold text-white text-sm">{ep.description || 'Custom Webhook Endpoint'}</h3>
                        <p className="font-mono text-xs text-indigo-400 mt-1 flex items-center gap-1">
                          {ep.url}
                        </p>
                      </div>
                      <button
                        onClick={() => handleDeleteEndpoint(ep.id)}
                        className="text-slate-500 hover:text-red-400 p-1.5 rounded transition"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>

                    <div className="flex flex-wrap items-center gap-2 text-xs">
                      <span className="text-slate-400">Events:</span>
                      {ep.event_types.map((t) => (
                        <span
                          key={t}
                          className="px-2 py-0.5 rounded bg-slate-800 text-slate-300 font-mono text-[11px]"
                        >
                          {t}
                        </span>
                      ))}
                    </div>

                    <div className="pt-2 border-t border-slate-800 flex items-center justify-between text-xs text-slate-400">
                      <div className="flex items-center gap-1 font-mono">
                        <Shield className="w-3.5 h-3.5 text-emerald-400" />
                        <span>Secret: {ep.secret.slice(0, 16)}...</span>
                      </div>
                      <button
                        onClick={() => {
                          navigator.clipboard.writeText(ep.secret)
                          alert('Copied HMAC Secret to clipboard!')
                        }}
                        className="text-indigo-400 hover:text-indigo-300 flex items-center gap-1"
                      >
                        <Copy className="w-3.5 h-3.5" /> Copy Secret
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* Create endpoint form */}
            <div className="p-6 rounded-xl border border-slate-800 bg-slate-900/50 space-y-4 h-fit">
              <h2 className="text-base font-semibold text-white flex items-center gap-2">
                <Plus className="w-4 h-4 text-indigo-400" /> Add New Subscriber
              </h2>
              <form onSubmit={handleRegisterEndpoint} className="space-y-4 text-xs">
                <div>
                  <label className="block text-slate-400 mb-1 font-medium">Destination URL</label>
                  <input
                    type="url"
                    required
                    value={newUrl}
                    onChange={(e) => setNewUrl(e.target.value)}
                    placeholder="https://api.example.com/webhook"
                    className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-slate-200 focus:outline-none focus:border-indigo-500 font-mono"
                  />
                  <p className="text-[11px] text-slate-500 mt-1">
                    Tip: Use <code className="text-indigo-400">http://localhost:8080/api/v1/test-receiver?status=500</code> to test auto-retry!
                  </p>
                </div>

                <div>
                  <label className="block text-slate-400 mb-1 font-medium">Description</label>
                  <input
                    type="text"
                    value={newDesc}
                    onChange={(e) => setNewDesc(e.target.value)}
                    placeholder="Order service webhook"
                    className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-slate-200 focus:outline-none focus:border-indigo-500 font-sans"
                  />
                </div>

                <div>
                  <label className="block text-slate-400 mb-1 font-medium">Subscribed Events (comma-separated)</label>
                  <input
                    type="text"
                    value={newEventTypes}
                    onChange={(e) => setNewEventTypes(e.target.value)}
                    placeholder="* or payment.succeeded, order.created"
                    className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-slate-200 focus:outline-none focus:border-indigo-500 font-mono"
                  />
                </div>

                <button
                  type="submit"
                  className="w-full py-2.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-semibold shadow-lg shadow-indigo-600/20 transition"
                >
                  Create Endpoint & Generate Secret
                </button>
              </form>
            </div>
          </div>
        )}

        {/* Tab 3: Dispatch Simulator */}
        {activeTab === 'simulator' && (
          <div className="max-w-2xl mx-auto p-6 rounded-xl border border-slate-800 bg-slate-900/40 space-y-5">
            <div>
              <h2 className="text-base font-semibold text-white">Simulate Outgoing Webhook Event</h2>
              <p className="text-xs text-slate-400 mt-1">
                Ingest an event into the engine. The dispatcher worker pool will immediately compute HMAC signatures and fan-out delivery.
              </p>
            </div>

            <div className="space-y-4 text-xs">
              <div>
                <label className="block text-slate-400 mb-1 font-medium">Event Type Name</label>
                <input
                  type="text"
                  value={simEventType}
                  onChange={(e) => setSimEventType(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-slate-200 font-mono focus:border-indigo-500 focus:outline-none"
                />
              </div>

              <div>
                <label className="block text-slate-400 mb-1 font-medium">JSON Event Payload</label>
                <textarea
                  rows={8}
                  value={simPayload}
                  onChange={(e) => setSimPayload(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-slate-200 font-mono text-xs focus:border-indigo-500 focus:outline-none"
                />
              </div>

              <button
                onClick={handleDispatchEvent}
                disabled={isDispatching}
                className="w-full py-3 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-semibold flex items-center justify-center gap-2 shadow-lg shadow-indigo-600/20 transition"
              >
                <Send className="w-4 h-4" />
                {isDispatching ? 'Dispatching...' : 'Dispatch to All Matching Endpoints'}
              </button>
            </div>
          </div>
        )}
      </main>

      {/* Inspection Modal */}
      {selectedAttempt && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl max-w-2xl w-full p-6 space-y-5 max-h-[90vh] overflow-y-auto">
            <div className="flex items-center justify-between border-b border-slate-800 pb-4">
              <div>
                <h3 className="text-base font-bold text-white flex items-center gap-2 font-mono">
                  Delivery Attempt #{selectedAttempt.attempt_number}
                </h3>
                <p className="text-xs text-slate-400 truncate mt-0.5">{selectedAttempt.endpoint_url}</p>
              </div>
              <button
                onClick={() => setSelectedAttempt(null)}
                className="text-slate-400 hover:text-white text-sm"
              >
                ✕ Close
              </button>
            </div>

            <div className="space-y-4 text-xs font-mono">
              <div>
                <span className="text-slate-400 uppercase text-[10px] tracking-wider font-semibold block mb-1">
                  Sent HTTP Headers (Signed)
                </span>
                <div className="p-3 bg-slate-950 border border-slate-800 rounded-lg text-slate-300 space-y-1">
                  {selectedAttempt.headers_sent &&
                    Object.entries(selectedAttempt.headers_sent).map(([k, v]) => (
                      <div key={k} className="flex gap-2">
                        <span className="text-indigo-400">{k}:</span>
                        <span className="text-slate-200 truncate">{v}</span>
                      </div>
                    ))}
                </div>
              </div>

              <div>
                <span className="text-slate-400 uppercase text-[10px] tracking-wider font-semibold block mb-1">
                  Request Payload Body
                </span>
                <pre className="p-3 bg-slate-950 border border-slate-800 rounded-lg text-slate-300 overflow-x-auto">
                  {typeof selectedAttempt.payload === 'string'
                    ? selectedAttempt.payload
                    : JSON.stringify(selectedAttempt.payload, null, 2)}
                </pre>
              </div>

              <div>
                <span className="text-slate-400 uppercase text-[10px] tracking-wider font-semibold block mb-1">
                  Receiver HTTP Response (Status {selectedAttempt.status_code})
                </span>
                <pre className="p-3 bg-slate-950 border border-slate-800 rounded-lg text-emerald-400 overflow-x-auto">
                  {selectedAttempt.response_body || selectedAttempt.error_message || 'No response body'}
                </pre>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
