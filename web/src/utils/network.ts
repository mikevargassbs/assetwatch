function ipToInt(ip: string): number | null {
  const parts = ip.trim().split('.')
  if (parts.length !== 4) return null
  let n = 0
  for (const part of parts) {
    const v = Number(part)
    if (!Number.isInteger(v) || v < 0 || v > 255) return null
    n = (n << 8) | v
  }
  return n >>> 0
}

function intToIp(n: number): string {
  return [(n >>> 24) & 255, (n >>> 16) & 255, (n >>> 8) & 255, n & 255].join('.')
}

// Caps the suggestion list so very large subnets (e.g. a /16) don't produce
// a datalist with tens of thousands of entries.
const MAX_SUGGESTIONS = 512

// Enumerates usable host addresses in the subnet containing `gateway`, given
// `subnetMask` — excludes the network address, the broadcast address, and
// the gateway itself (since that's already taken).
export function subnetAddresses(gateway: string, subnetMask: string): string[] {
  const gatewayInt = ipToInt(gateway)
  const maskInt = ipToInt(subnetMask)
  if (gatewayInt === null || maskInt === null || maskInt === 0) return []

  const network = gatewayInt & maskInt
  const broadcast = network | (~maskInt >>> 0)

  const addresses: string[] = []
  for (let i = network + 1; i < broadcast && addresses.length < MAX_SUGGESTIONS; i++) {
    if (i === gatewayInt) continue
    addresses.push(intToIp(i))
  }
  return addresses
}
