package schema

// SSLSettingsResponse mirrors BotSettingsResponse's own shape/convention --
// paths are returned in full (unlike BotToken, a filesystem path is not a
// secret an admin needs masked) so the settings page can show the admin
// exactly what's currently configured.
type SSLSettingsResponse struct {
	Domain          string `json:"domain"`
	CertificatePath string `json:"certificate_path"`
	PrivateKeyPath  string `json:"private_key_path"`
	Enabled         bool   `json:"enabled"`
}

type UpdateSSLSettingsRequest struct {
	Domain          *string `json:"domain"`
	CertificatePath *string `json:"certificate_path"`
	PrivateKeyPath  *string `json:"private_key_path"`
	Enabled         *bool   `json:"enabled"`
}
