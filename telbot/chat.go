package telbot

const (
	ChatTypePrivate    string = "private"
	ChatTypeGroup      string = "group"
	ChatTypeSuperGroup string = "supergroup"
	ChatTypeChannel    string = "channel"
)

type Chat struct {
	Id        int    `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// type ChatFullInfo struct {
// 	Chat
// 	AccentColorId int `json:"accent_color_id"`
// 	MaxReactionCount int `json:"max_reaction_count"`
// }

// type BaseChatParams struct {
// 	ChatId int `json:"chat_id"`
// }
