package domain

type TenantStatus string

const (
	TACTIVE    TenantStatus = "ACTIVE"
	TSUSPENDED TenantStatus = "SUSPENDED"
	TDELETED   TenantStatus = "DELETED"
)

type Tenant struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Status TenantStatus `json:"status"`
}
