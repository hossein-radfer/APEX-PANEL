package model

type Admin struct {
	Model
	Username string `gorm:"type:varchar(64);uniqueIndex;not null;"`
	Password string `gorm:"type:varchar(128);not null;"`
	IsActive bool   `gorm:"not null;default:true;"`
	Role      string `gorm:"type:varchar(32);not null;default:'admin'"`
	ResellerID *uint `gorm:"index"`
}
