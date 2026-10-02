package model

// BotExtraAdminChatID is one additional Telegram chat that should receive
// every admin-facing bot notification alongside BotSettings.AdminChatID --
// a confirmed, reported request: the admin wanted to add a second (and
// later, removable/editable) recipient for their own notifications rather
// than being limited to the single AdminChatID field. Kept as its own
// table (not a comma-separated list on BotSettings) specifically so each
// entry can be added/edited/removed individually with plain CRUD, per the
// admin's own explicit request ("بعدا بتونم حالا اون ایدی رو حذف یا
// ویرایشش کنم").
type BotExtraAdminChatID struct {
	Model
	ChatID string  `gorm:"type:varchar(64);uniqueIndex;not null"` // Telegram chat_id (numeric, as a string)
	Label  *string `gorm:"type:varchar(255)"`                     // optional human-readable name, e.g. "Ali's phone"
}
