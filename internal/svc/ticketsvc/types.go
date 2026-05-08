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
