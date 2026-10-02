package model

import "time"

// SSLSettings is a singleton row (always ID 1) holding the panel's manual
// TLS certificate configuration -- mirrors BotSettings' own singleton
// convention exactly. This is deliberately the "bring your own cert" model,
// not an ACME/Let's Encrypt integration: the admin runs Certbot (or any
// other ACME client) on the host themselves, outside this panel entirely,
// and just tells the panel where the resulting certificate/key files live
// on disk -- the panel only ever READS these two paths, it never issues,
// renews, or writes to them. Renewal (e.g. Certbot's own cron/systemd timer)
// stays entirely the admin's/host's responsibility; the panel picks up a
// renewed file automatically on its next restart.
type SSLSettings struct {
	Model
	Domain          string `gorm:"type:varchar(255)"`
	CertificatePath string `gorm:"type:varchar(500)"` // e.g. /etc/letsencrypt/live/<domain>/fullchain.pem
	PrivateKeyPath  string `gorm:"type:varchar(500)"` // e.g. /etc/letsencrypt/live/<domain>/privkey.pem
	Enabled         bool   `gorm:"not null;default:false"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
