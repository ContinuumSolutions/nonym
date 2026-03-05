package profile

type DecisionPreference struct {
	TimeSovereignty    int     `json:"time_sovereignty"`    // Weight given to protecting your time (1–10)
	FinacialGrowth     int     `json:"financial_growth"`    // Prioritise revenue-generating decisions (1–10)
	HealthRecovery     int     `json:"health_recovery"`     // Respect sleep, stress and energy limits (1–10)
	ReputationBuilding int     `json:"reputation_building"` // Long-term trust over short-term gain (1–10)
	PrivacyProtection  int     `json:"privacy_protection"`  // Refuse data sharing with low-trust parties (1–10)
	Autonomy           int     `json:"autonomy"`            // Minimise dependence on third parties (1–10)
	BaseHourlyRate     float64 `json:"base_hourly_rate"`    // Your target USD value of one hour of attention (e.g. 50, 150, 500)
}
