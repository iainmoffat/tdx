package domain

// UserFilter constrains a SearchUsers call. Zero-valued fields mean
// "no constraint". A pointer-typed bool (Active) distinguishes
// "no filter" (nil) from "explicitly false" (false).
type UserFilter struct {
	Active      *bool  // nil = no filter; non-nil = filter to this value
	UserType    string // default "User"
	AccountID   int    // 0 = no filter
	AccountName string // "" = no filter
	NameLike    string // "" = no filter
	Limit       int    // 0 = client default (100)
}
