package mongodb

var ColNames = struct {
	Tenants     string
	Connectors  string
	Credentials string
	Users       string
	BotProfiles string
	HttpTools   string
}{
	Tenants:     "tenants",
	Connectors:  "connectors",
	Credentials: "credentials",
	Users:       "users",
	BotProfiles: "bot_profiles",
	HttpTools:   "http_tools",
}
