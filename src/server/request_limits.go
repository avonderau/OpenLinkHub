package server

// Inherited device-setting handlers commonly decode JSON directly from
// r.Body without an independent size limit. Keep their maximum well above
// ordinary settings payloads, but below arbitrary multi-megabyte requests.
// Device upload endpoints are separately validated and use a larger quota.
const (
	maxLegacyRequestBodyBytes int64 = 4 << 20
	maxLegacyUploadBodyBytes  int64 = 6 << 20
)

func legacyRequestBodyLimit(route string) int64 {
	switch route {
	case "/api/restore", "/api/lcd/upload":
		// A 5 MiB file plus multipart framing is supported by these endpoints.
		return maxLegacyUploadBodyBytes
	default:
		return maxLegacyRequestBodyBytes
	}
}
