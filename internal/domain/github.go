package domain

// AuthToken represents the stored GitHub OAuth token.
type AuthToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	User        string `json:"user"`
}
