package profile

type ConnectionSetting struct {
	KernelName  string `json:"kernel_name"`
	APIEndpoint string `json:"api_endpoint"`
	Timezone    string `json:"timezone"` // IANA timezone name e.g. "Europe/London", "UTC"
}
