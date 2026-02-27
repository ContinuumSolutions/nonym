package profile

import (
	"time"
)

type ConnectionSetting struct {
	KernelName	string `json:"kernel_name"`
	APIEndpoint	string	`json:"api_endpoint"`
	TimeZone	time.Location `json:"time_zone"`
}