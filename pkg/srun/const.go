package srun

import "time"

const (
	// Portal
	PortalDomain   = "https://portal.nwafu.edu.cn"
	PortalFallback = "172.26.8.11"

	// Self-service
	SelfServiceDomain   = "https://service.nwafu.edu.cn"
	SelfServiceFallback = "172.26.8.13"

	// DNS
	CampusDNS1 = "210.27.82.101"
	CampusDNS2 = "210.27.82.102"

	// HTTP
	DefaultUserAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	PortalTimeout      = 5 * time.Second
	SelfServiceTimeout = 15 * time.Second
	ProbeTimeout       = 3 * time.Second
	DNSTimeout         = 3 * time.Second

	// Delays
	LoginSettleDelay  = 1 * time.Second
	LogoutSettleDelay = 3 * time.Second
	BypassCheckDelay  = 2 * time.Second
	LoginInfoRetry    = 5
	LoginInfoRetryGap = 500 * time.Millisecond

	// Kick retry: useful when a TUN-mode VPN / proxy drops the first
	// connection; a fast retry usually goes through. Does NOT help when the
	// proxy outright blackholes campus traffic.
	KickRetry    = 2
	KickRetryGap = 800 * time.Millisecond

	// Env
	EnvUsername = "NWAFU_SRUN_USERNAME"
	EnvPassword = "NWAFU_SRUN_PASSWORD"
)

// CampusDNSServers returns campus DNS servers for fallback resolution.
func CampusDNSServers() []string {
	return []string{CampusDNS1, CampusDNS2}
}
