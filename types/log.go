package types

type Log struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	EntryPoint string `json:"entrypoint"`
	Ident      string `json:"ident"`
	Data       string `json:"data"`
	Datetime   string `json:"datetime"`
}
