/**
 * Android bridge — calls Kotlin via window.AndroidBridge.call()
 * and receives results through window.__bridgeCallback().
 */

import type { TunnelInfo, StatsInfo, ManagedSettings, ManagedLoginResult, ServerInfo } from './types'

// Call a method on the Android bridge and return a Promise for the result.
function call<T>(method: string, args: unknown[] = []): Promise<T> {
  return new Promise((resolve, reject) => {
    const id = Date.now().toString(36) + Math.random().toString(36).slice(2)
    ;(window as any).__callbacks = (window as any).__callbacks || {}
    ;(window as any).__callbacks[id] = { resolve, reject }
    ;(window as any).AndroidBridge.call(method, JSON.stringify(args), id)
  })
}

// Subscribe to push events from Kotlin.
export function onEvent(name: string, fn: (data: any) => void): void {
  ;(window as any).__eventListeners = (window as any).__eventListeners || {}
  const listeners = (window as any).__eventListeners
  if (!listeners[name]) listeners[name] = []
  listeners[name].push(fn)
}

// --- Standard tunnel API ---

export async function isDaemonRunning(): Promise<boolean> {
  try {
    return await call<boolean>('isDaemonRunning')
  } catch {
    return true // always true on Android
  }
}

export async function listTunnels(): Promise<TunnelInfo[]> {
  const result = await call<TunnelInfo[]>('listTunnels')
  return (result as unknown as TunnelInfo[]) ?? []
}

export async function importTunnel(name: string, configContent: string): Promise<TunnelInfo> {
  return call<TunnelInfo>('importTunnel', [name, configContent])
}

export async function deleteTunnel(id: string): Promise<void> {
  await call('deleteTunnel', [id])
}

export async function connectTunnel(id: string): Promise<void> {
  await call('connectTunnel', [id])
}

export async function disconnectTunnel(id: string): Promise<void> {
  await call('disconnectTunnel', [id])
}

export async function getStats(id: string): Promise<StatsInfo> {
  return call<StatsInfo>('getStats', [id])
}

// --- Managed mode API ---

export async function managedGetSettings(): Promise<ManagedSettings> {
  return call<ManagedSettings>('managedGetSettings')
}

export async function managedSaveServerURL(serverURL: string): Promise<void> {
  await call('managedSaveServerURL', [serverURL])
}

export async function managedLogin(username: string, password: string, totpCode: string): Promise<ManagedLoginResult> {
  return call<ManagedLoginResult>('managedLogin', [username, password, totpCode])
}

export async function managedLogout(): Promise<void> {
  await call('managedLogout')
}

export async function managedListServers(): Promise<ServerInfo[]> {
  const result = await call<ServerInfo[]>('managedListServers')
  return (result as unknown as ServerInfo[]) ?? []
}

export async function managedConnectServer(serverID: string, serverName: string, totpCode: string = ''): Promise<TunnelInfo> {
  return call<TunnelInfo>('managedConnectServer', [serverID, serverName, totpCode])
}

export async function managedDisconnectServer(serverID: string): Promise<void> {
  await call('managedDisconnectServer', [serverID])
}

export async function managedActiveServers(): Promise<string[]> {
  const result = await call<string[]>('managedActiveServers')
  return (result as unknown as string[]) ?? []
}

export async function managedCheckSetup(): Promise<boolean> {
  return call<boolean>('managedCheckSetup')
}

export async function managedGetSetupStep(): Promise<string> {
  return call<string>('managedGetSetupStep')
}

export async function managedDisconnectByTunnelID(tunnelID: string): Promise<void> {
  await call('managedDisconnectByTunnelID', [tunnelID])
}

export async function managedSetMode(mode: string): Promise<void> {
  await call('managedSetMode', [mode])
}

export async function managedDefaultDeviceName(): Promise<string> {
  return call<string>('managedDefaultDeviceName')
}

export async function managedRegisterDevice(serverURL: string, deviceName: string): Promise<void> {
  await call('managedRegisterDevice', [serverURL, deviceName])
}

export async function managedCompleteSetup(): Promise<void> {
  await call('managedCompleteSetup')
}

export async function pickFile(): Promise<{ name: string; content: string }> {
  return call<{ name: string; content: string }>('pickFile')
}
