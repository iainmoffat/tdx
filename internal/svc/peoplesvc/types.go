package peoplesvc

// wireUser matches GET /TDWebApi/api/people/{uid}.
type wireUser struct {
	UID                string `json:"UID"`
	ID                 int    `json:"ID"`
	FullName           string `json:"FullName"`
	PrimaryEmail       string `json:"PrimaryEmail"`
	AlternateEmail     string `json:"AlternateEmail"`
	IsActive           bool   `json:"IsActive"`
	DefaultAccountName string `json:"DefaultAccountName"`
	ReportsToUid       string `json:"ReportsToUid"`
	ReportsToId        int    `json:"ReportsToId"`
	ReportsToFullName  string `json:"ReportsToFullName"`
	ReportsToEmail     string `json:"ReportsToEmail"`
}

// wireUserSearch is the request body for POST /TDWebApi/api/people/search.
type wireUserSearch struct {
	NameLike   string `json:"NameLike,omitempty"`
	IsActive   *bool  `json:"IsActive,omitempty"`
	AccountIDs []int  `json:"AccountIDs,omitempty"`
	UserType   string `json:"UserType,omitempty"`
	MaxResults int    `json:"MaxResults,omitempty"`
}
