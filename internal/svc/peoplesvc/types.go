package peoplesvc

// wireUser matches GET /TDWebApi/api/people/{uid} and rows in the
// POST /TDWebApi/api/people/search response.
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
	ResourcePoolID     int    `json:"ResourcePoolID"`
	ResourcePoolName   string `json:"ResourcePoolName"`
}

// wireResourcePool matches rows in POST /TDWebApi/api/resourcepools/search.
type wireResourcePool struct {
	ID               int    `json:"ID"`
	Name             string `json:"Name"`
	IsActive         bool   `json:"IsActive"`
	RequiresApproval bool   `json:"RequiresApproval"`
	ManagerUID       string `json:"ManagerUID"`
	ManagerFullName  string `json:"ManagerFullName"`
}

// wireUserSearch is the request body for POST /TDWebApi/api/people/search.
type wireUserSearch struct {
	NameLike   string `json:"NameLike,omitempty"`
	IsActive   *bool  `json:"IsActive,omitempty"`
	IsEmployee *bool  `json:"IsEmployee,omitempty"`
	AccountIDs []int  `json:"AccountIDs,omitempty"`
	UserType   string `json:"UserType,omitempty"`
	MaxResults int    `json:"MaxResults,omitempty"`
}
