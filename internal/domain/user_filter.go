package domain

// UserFilter constrains a SearchUsers call. Zero-valued fields mean
// "no constraint". A pointer-typed bool (Active) distinguishes
// "no filter" (nil) from "explicitly false" (false).
type UserFilter struct {
	Active     *bool  // nil = no filter; non-nil = filter to this value
	Employee   *bool  // nil = no filter; non-nil = filter by IsEmployee
	UserType   string // default "User"
	AccountIDs []int  // nil/empty = no filter; otherwise restrict to these accounts
	NameLike   string // "" = no filter
	Limit      int    // 0 = client default (100)
}
