package model

type Server struct {
	Model
	Comment   *string `gorm:"type:varchar(255)"`
	Name      string  `gorm:"type:varchar(64);uniqueIndex;not null;"`
	IPAddress string  `gorm:"type:varchar(64);uniqueIndex;not null;"`
	APIPort   int     `gorm:"not null;default:80;"`
	Username  string  `gorm:"type:varchar(64);not null;"`
	Password  string  `gorm:"type:varchar(64);not null;"`
	IsActive  bool    `gorm:"not null;default:true;"`
	// IsSSL persists the create-time SSL choice (schema.CreateServerRequest.
	// IsSSL) so it survives a panel restart. Before this field existed,
	// MwpClients.InitClient -- the code path that rebuilds every server's
	// HTTP client on startup, as opposed to SetClient's create/update path --
	// had nowhere to read this from and silently hardcoded http://+
	// InsecureSkipVerify:true for every server, regardless of how it was
	// originally configured.
	IsSSL bool `gorm:"not null;default:false;"`
}
