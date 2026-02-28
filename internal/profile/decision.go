package profile

type DecisionPreference struct {
	TimeSovereignty    int `json:"time_sovereignty"`    //Weight given to protecting your time
	FinacialGrowth     int `json:"financial_growth"`    //Prioritise revenue-generating decisions
	HealthRecovery     int `json:"health_recovery"`     //Respect sleep, stress and energy limits
	ReputationBuilding int `json:"reputation_building"` //Long-term trust over short-term gain
	PrivacyProtection  int `json:"privacy_protection"`  //Refuse data sharing with low-trust parties
	Autonomy           int `json:"autonomy"`            //Minimise dependence on third parties
}
