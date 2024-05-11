package notify

type Telegram struct {
	APIUrl *string `hcl:"api_url"`

	Token  string `hcl:"token"`
	ChatID string `hcl:"chat_id"`

	Events []Event `hcl:"events"`
}

func (c Telegram) GetAPIUrl() string {
	if c.APIUrl == nil {
		return "https://api.telegram.org"
	}

	return *c.APIUrl
}
