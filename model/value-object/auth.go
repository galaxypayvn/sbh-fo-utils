package valueobject

import "github.com/google/uuid"

type Auth struct {
	BusinessID uint64    `json:"business_id" valid:"required"`
	UserID     uuid.UUID `json:"user_id" valid:"required"`
	RequestID  string    `json:"request_id" valid:"required"`
	OrgID      uint64    `json:"org_id" valid:"required"`
	Locale     string    `json:"locale"`
	Timezone   string    `json:"timezone"`
	BranchID   uuid.UUID `json:"branch_id"`
	IsOnly     bool      `json:"is_only"`
	RealIP     string    `json:"real_ip"`
}
