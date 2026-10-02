package model

// SystemConfig is a small, independent key-value table used to track
// panel/schema version and other one-off system metadata across upgrades.
// It must never be modified in a way that alters or deletes existing rows in
// other tables — it exists purely so future versions can detect what schema
// state the database is in before applying additive changes.
type SystemConfig struct {
	Model
	Key   string `gorm:"type:varchar(64);uniqueIndex;not null"`
	Value string `gorm:"type:varchar(255);not null"`
}
