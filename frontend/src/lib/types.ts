export interface Peer {
  name: string
  publicKey: string
  presharedKey: string
  clientToken: string
  address: string
  allowedIPs: string[]
  clientRoutes: string[]
  dns: string[]
  endpoint: string
  persistentKeepalive: number
  description: string
  enabled: boolean
  connected: boolean
  latestHandshake: string
  transferRx: number
  transferTx: number
  createdAt: string
  updatedAt: string
}

export interface WireGuardInterface {
  name: string
  publicKey: string
  listenPort: number
  addresses: string[]
  mtu: number
  dns: string[]
  up: boolean
  running: boolean
  dryRun: boolean
  totalPeers: number
  connectedPeers: number
  transferRx: number
  transferTx: number
  peers: Peer[]
  createdAt: string
  updatedAt: string
}

export interface Settings {
  publicEndpoint: string
}

export interface InterfacePatch {
  listenPort?: number | null
  addresses?: string[] | null
  mtu?: number | null
  dns?: string[] | null
  up?: boolean | null
}

export interface PeerInput {
  name: string
  address: string
  publicKey: string
  generateKeys: boolean
  presharedKey: string
  withPreshared: boolean
  allowedIPs: string[]
  clientRoutes: string[]
  dns: string[]
  endpoint: string
  persistentKeepalive: number
  description: string
  enabled: boolean
}