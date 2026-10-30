//go:build unit

package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadTurnstileTrustedPeerCIDRsDefaultEmpty(t *testing.T) {
	resetViperWithJWTSecret(t)
	cfg, err := Load()
	require.NoError(t, err)
	require.Empty(t, cfg.Turnstile.TrustedPeerCIDRs)
	require.False(t, cfg.Turnstile.IsTrustedPeer("172.30.10.1:40000"))
}

func TestLoadTurnstileTrustedPeerCIDRsFromEnv(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("TURNSTILE_TRUSTED_PEER_CIDRS", " 172.30.10.1/32, 10.9.0.0/16 ,, 192.0.2.7 ")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, []string{"172.30.10.1/32", "10.9.0.0/16", "192.0.2.7"}, cfg.Turnstile.TrustedPeerCIDRs)
}

func TestLoadTurnstileTrustedPeerCIDRsRejectsInvalidEntry(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("TURNSTILE_TRUSTED_PEER_CIDRS", "172.30.10.1/32,not-a-cidr")
	_, err := Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "turnstile.trusted_peer_cidrs")
}

func TestTurnstileConfigIsTrustedPeer(t *testing.T) {
	cfg := TurnstileConfig{TrustedPeerCIDRs: []string{"172.30.10.1/32", "10.9.0.0/16", "192.0.2.7", "2001:db8::/32"}}
	cases := map[string]bool{
		"172.30.10.1:40000":          true,
		"[::ffff:172.30.10.1]:40000": true,
		"10.9.200.3:1":               true,
		"192.0.2.7:443":              true,
		"[2001:db8::1]:8080":         true,
		"172.30.10.20:40000":         false,
		"192.0.2.8:443":              false,
		"172.30.10.1":                false, // RemoteAddr always carries a port
		"":                           false,
		"garbage":                    false,
	}
	for remoteAddr, want := range cases {
		require.Equal(t, want, cfg.IsTrustedPeer(remoteAddr), remoteAddr)
	}
	require.False(t, TurnstileConfig{}.IsTrustedPeer("172.30.10.1:40000"))
}
