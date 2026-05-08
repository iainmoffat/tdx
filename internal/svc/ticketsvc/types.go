package ticketsvc

// wireApp matches one row in the response from
// GET /TDWebApi/api/applications. The endpoint returns all platform
// applications; ListApps filters to ticketing apps via AppClass containing
// "Ticket".
type wireApp struct {
	ID          int    `json:"AppID"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	Active      bool   `json:"Active"`
	Type        string `json:"Type"`
	AppClass    string `json:"AppClass"`
}

// wireTicketStatus matches GET /TDWebApi/api/{appId}/tickets/statuses rows.
type wireTicketStatus struct {
	ID          int     `json:"ID"`
	Name        string  `json:"Name"`
	IsActive    bool    `json:"IsActive"`
	Order       float64 `json:"Order"`
	IsDefault   bool    `json:"IsDefault"`
	StatusClass int     `json:"StatusClass"` // TD enum; 6 = Closed (verify live in Task 19)
}

// wireTicketType matches GET /TDWebApi/api/{appId}/tickets/types rows.
type wireTicketType struct {
	ID          int    `json:"ID"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	IsActive    bool   `json:"IsActive"`
}
