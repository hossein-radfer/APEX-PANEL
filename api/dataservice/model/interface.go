package model

type Interface struct {
	Model
	// ServerID identifies which physical Mikrotik router this interface
	// lives on. InterfaceID and Name used to be globally unique on their
	// own, which implicitly assumed a single-server install -- two servers
	// each running their own "wg0" would have collided. Both are now scoped
	// to (ServerID, value) instead, so the same interface name is allowed
	// to exist independently on different servers (see
	// backfillServerIDForSingleServerInstall in dataservice/db.go for how
	// existing rows get a ServerID on upgrade).
	ServerID    uint    `gorm:"uniqueIndex:idx_interface_server_interface_id;uniqueIndex:idx_interface_server_name;not null"`
	InterfaceID string  `gorm:"type:varchar(255);uniqueIndex:idx_interface_server_interface_id;not null"`
	Disabled    bool    `gorm:"type:boolean;not null;default:false"`
	Comment     *string `gorm:"type:varchar(255)"`
	Name        string  `gorm:"type:varchar(255);uniqueIndex:idx_interface_server_name;not null"`
	PrivateKey  string  `gorm:"type:varchar(255);not null"`
	PublicKey   string  `gorm:"type:varchar(255);not null"`
	ListenPort  string  `gorm:"type:varchar(10);not null"`

	IPPool *IPPool `gorm:"foreignKey:InterfaceID"`
}
