package scraper

import "time"

func mergeSourceConfig(base, override SourceConfig) SourceConfig {
	if override.Name != "" {
		base.Name = override.Name
	}
	if override.DisplayName != "" {
		base.DisplayName = override.DisplayName
	}
	if override.BaseURL != "" {
		base.BaseURL = override.BaseURL
	}
	base.Enabled = override.Enabled
	if override.MaxPerHour > 0 {
		base.MaxPerHour = override.MaxPerHour
	}
	if override.BaseDelay > 0 {
		base.BaseDelay = override.BaseDelay
	}
	if override.Jitter > 0 {
		base.Jitter = override.Jitter
	}
	if override.RequestTimeout > 0 {
		base.RequestTimeout = override.RequestTimeout
	}
	if override.CircuitThreshold > 0 {
		base.CircuitThreshold = override.CircuitThreshold
	}
	if override.CircuitCooldown > 0 {
		base.CircuitCooldown = override.CircuitCooldown
	}
	base.UseUTLS = override.UseUTLS
	if base.RequestTimeout <= 0 {
		base.RequestTimeout = 10 * time.Second
	}
	return base
}
