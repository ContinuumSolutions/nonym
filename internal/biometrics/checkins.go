package biometrics

type Checkin struct {
	Feeling      int    `json:"feeling"`
	StressLevel  int    `json:"stress_level"`
	Sleep        int    `json:"sleep"`
	Energy       int    `json:"energy"`
	ExtraContext string `json:"extra_context"`
}
